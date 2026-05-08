package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
)

func (s *ValidateService) populateOperatorReport(ctx context.Context, cfg *v2.Config, result *ValidationResult) {
	result.ServiceReports = buildServiceReports(cfg, result.Issues)
	result.GitOpsReport = s.buildGitOpsReport(ctx, cfg, result.ValidationMode)
	result.Missing = buildMissing(result.Issues, result.GitOpsReport, result.ServiceReports)
	result.ActionItems = buildActionItems(result.Suggestions, result.GitOpsReport)
	result.CheckSummary = buildCheckSummary(result)
}

func buildServiceReports(cfg *v2.Config, issues []v2.ValidationIssue) []ValidationServiceReport {
	if cfg == nil {
		return nil
	}

	names := enabledServiceNames(cfg)
	reports := make([]ValidationServiceReport, 0, len(names))
	for _, name := range names {
		report := ValidationServiceReport{Name: name, Status: CheckStatusPass}
		for _, issue := range issues {
			if issue.Category != v2.CategoryServices {
				continue
			}
			if serviceIssueMatches(name, issue.Path) {
				report.Status = CheckStatusFail
				report.Missing = append(report.Missing, issue.Path)
				if report.Message == "" {
					report.Message = conciseMissingMessage(issue)
				}
			}
		}
		reports = append(reports, report)
	}
	return reports
}

func enabledServiceNames(cfg *v2.Config) []string {
	seen := make(map[string]bool)
	collect := func(services v2.ServiceMap) {
		for name, serviceConfig := range services {
			if svc, ok := serviceConfig.(interface{ IsEnabled() bool }); ok && svc.IsEnabled() {
				seen[name] = true
			}
		}
	}
	collect(cfg.OpenCenter.Services)
	collect(cfg.OpenCenter.ManagedServices)

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func serviceIssueMatches(serviceName, path string) bool {
	key := strings.ReplaceAll(serviceName, "-", "_")
	path = strings.ToLower(path)
	if strings.Contains(path, key) {
		return true
	}
	switch serviceName {
	case "kube-prometheus-stack":
		return strings.Contains(path, "grafana")
	case "vsphere-csi":
		return strings.Contains(path, "vsphere_csi")
	default:
		return false
	}
}

func conciseMissingMessage(issue v2.ValidationIssue) string {
	path := strings.TrimSpace(issue.Path)
	if path != "" {
		parts := strings.Split(path, ".")
		return "missing " + strings.ReplaceAll(parts[len(parts)-1], "_", " ")
	}
	message := strings.TrimSpace(issue.Message)
	if message == "" {
		return "missing required setting"
	}
	return message
}

func (s *ValidateService) buildGitOpsReport(ctx context.Context, cfg *v2.Config, mode string) ValidationGitOpsReport {
	if cfg == nil {
		return ValidationGitOpsReport{}
	}

	repoURL := cfg.ConfiguredGitURL()
	localPath := cfg.GitDir()
	branch := cfg.GitBranchOrDefault()
	report := ValidationGitOpsReport{
		RepositoryURL: repoURL,
		LocalPath:     localPath,
		Branch:        branch,
	}

	if repoURL == "" {
		report.Checks = append(report.Checks, ValidationCheck{Name: "Repository URL", Status: CheckStatusFail, Message: "missing GitOps repository URL"})
	} else {
		report.Checks = append(report.Checks, ValidationCheck{Name: "Repository URL", Status: CheckStatusPass, Detail: repoURL})
	}

	authStatus, authMessage := gitOpsAuthStatus(cfg)
	report.Checks = append(report.Checks, ValidationCheck{Name: "Auth method", Status: authStatus, Message: authMessage})

	if localPath == "" {
		report.Checks = append(report.Checks, ValidationCheck{Name: "Local path", Status: CheckStatusWarn, Message: "not configured"})
	} else {
		report.Checks = append(report.Checks, ValidationCheck{Name: "Local path", Status: CheckStatusPass, Detail: localPath})
		report.Checks = append(report.Checks, s.localGitChecks(ctx, localPath)...)
		report.Checks = append(report.Checks, s.secretEncryptionChecks(ctx, localPath)...)
	}

	if mode != ValidationModeOnline {
		report.Checks = append(report.Checks, ValidationCheck{Name: "Remote checks", Status: CheckStatusSkip, Message: "skipped in offline mode"})
		return report
	}

	report.Checks = append(report.Checks, s.remoteGitChecks(ctx, repoURL, branch)...)
	return report
}

func gitOpsAuthStatus(cfg *v2.Config) (CheckStatus, string) {
	auth := cfg.OpenCenter.GitOps.Auth
	if auth.SSH != nil && auth.Token != nil {
		return CheckStatusFail, "ssh and token auth are both configured"
	}
	if auth.SSH != nil {
		if strings.TrimSpace(auth.SSH.PrivateKey) == "" || strings.EqualFold(strings.TrimSpace(auth.SSH.PrivateKey), v2.PlaceholderSecret) {
			return CheckStatusFail, "missing ssh private key"
		}
		if strings.TrimSpace(auth.SSH.PublicKey) == "" || strings.EqualFold(strings.TrimSpace(auth.SSH.PublicKey), v2.PlaceholderSecret) {
			return CheckStatusFail, "missing ssh public key"
		}
		return CheckStatusPass, "ssh"
	}
	if auth.Token != nil {
		token := strings.TrimSpace(auth.Token.Token)
		tokenFile := strings.TrimSpace(auth.Token.TokenFile)
		if (token == "" || strings.EqualFold(token, v2.PlaceholderSecret)) &&
			(tokenFile == "" || strings.EqualFold(tokenFile, v2.PlaceholderSecret)) {
			return CheckStatusFail, "missing token"
		}
		return CheckStatusPass, "token"
	}
	return CheckStatusFail, "missing GitOps auth"
}

func (s *ValidateService) localGitChecks(ctx context.Context, localPath string) []ValidationCheck {
	var checks []ValidationCheck
	if _, err := os.Stat(localPath); err != nil {
		if os.IsNotExist(err) {
			return []ValidationCheck{{Name: "Local git", Status: CheckStatusWarn, Message: "local path does not exist"}}
		}
		return []ValidationCheck{{Name: "Local git", Status: CheckStatusWarn, Message: fmt.Sprintf("cannot access local path: %v", err)}}
	}

	if _, err := os.Stat(filepath.Join(localPath, ".git")); err != nil {
		if os.IsNotExist(err) {
			return []ValidationCheck{{Name: "Local git", Status: CheckStatusWarn, Message: "local path is not a git repository"}}
		}
		return []ValidationCheck{{Name: "Local git", Status: CheckStatusWarn, Message: fmt.Sprintf("cannot access .git directory: %v", err)}}
	}

	branchOutput, err := runGit(ctx, localPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		checks = append(checks, ValidationCheck{Name: "Local branch", Status: CheckStatusPass, Detail: strings.TrimSpace(branchOutput)})
	}

	statusOutput, err := runGit(ctx, localPath, "status", "--porcelain")
	if err != nil {
		checks = append(checks, ValidationCheck{Name: "Local git", Status: CheckStatusWarn, Message: fmt.Sprintf("git status failed: %v", err)})
		return checks
	}
	statusOutput = strings.TrimSpace(statusOutput)
	if statusOutput == "" {
		checks = append(checks, ValidationCheck{Name: "Local git", Status: CheckStatusPass, Message: "clean"})
		return checks
	}
	checks = append(checks, ValidationCheck{Name: "Local git", Status: CheckStatusWarn, Message: describePorcelainStatus(statusOutput)})
	return checks
}

func (s *ValidateService) remoteGitChecks(ctx context.Context, repoURL, branch string) []ValidationCheck {
	if repoURL == "" {
		return []ValidationCheck{{Name: "Remote checks", Status: CheckStatusSkip, Message: "repository URL is missing"}}
	}
	if branch == "" {
		branch = "main"
	}
	remoteCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := runGit(remoteCtx, "", "ls-remote", "--heads", repoURL, branch)
	if err != nil {
		return []ValidationCheck{{Name: "Remote checks", Status: CheckStatusFail, Message: fmt.Sprintf("failed to reach Git remote: %v", err)}}
	}
	if strings.TrimSpace(output) == "" {
		return []ValidationCheck{{Name: "Remote branch", Status: CheckStatusFail, Message: fmt.Sprintf("branch %q not found", branch)}}
	}
	return []ValidationCheck{{Name: "Remote branch", Status: CheckStatusPass, Detail: branch}}
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func describePorcelainStatus(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	modified := 0
	untracked := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "??") {
			untracked++
		} else {
			modified++
		}
	}
	parts := []string{"dirty"}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", modified))
	}
	if untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", untracked))
	}
	return strings.Join(parts, ", ")
}

// secretEncryptionChecks scans the local GitOps directory for Kubernetes Secret
// manifests that are not SOPS-encrypted. This catches the same issues that the
// generate command's pre-commit scan would reject, giving operators early
// feedback during validate.
func (s *ValidateService) secretEncryptionChecks(ctx context.Context, localPath string) []ValidationCheck {
	if _, err := os.Stat(localPath); err != nil {
		// Directory doesn't exist yet; nothing to scan.
		return nil
	}

	findings, err := gitops.ScanGitOpsSecretsWithOptions(ctx, gitops.SecretScanOptions{
		Root:   localPath,
		Staged: false,
	})
	if err != nil {
		return []ValidationCheck{{Name: "Secret encryption", Status: CheckStatusWarn, Message: fmt.Sprintf("scan error: %v", err)}}
	}

	if len(findings) == 0 {
		return []ValidationCheck{{Name: "Secret encryption", Status: CheckStatusPass, Message: "all secrets encrypted"}}
	}

	// Group findings by file for a concise summary.
	fileSet := make(map[string]bool)
	for _, f := range findings {
		fileSet[f.Path] = true
	}

	message := fmt.Sprintf("%d unencrypted secret file(s) found; run 'opencenter secrets sync' to encrypt", len(fileSet))
	return []ValidationCheck{{Name: "Secret encryption", Status: CheckStatusFail, Message: message}}
}

func buildMissing(issues []v2.ValidationIssue, gitops ValidationGitOpsReport, services []ValidationServiceReport) []ValidationMissing {
	seen := make(map[string]bool)
	var missing []ValidationMissing
	add := func(path, message string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		missing = append(missing, ValidationMissing{Path: path, Message: message})
	}

	for _, issue := range issues {
		if issue.Severity == v2.SeverityError && issue.Path != "" {
			add(issue.Path, issue.Message)
		}
	}
	for _, service := range services {
		for _, path := range service.Missing {
			add(path, service.Message)
		}
	}
	for _, check := range gitops.Checks {
		if check.Status == CheckStatusFail {
			switch check.Name {
			case "Repository URL":
				add("opencenter.gitops.repository.url", check.Message)
			case "Auth method":
				add("opencenter.gitops.auth", check.Message)
			}
		}
	}
	return missing
}

func buildActionItems(suggestions []string, gitops ValidationGitOpsReport) []string {
	seen := make(map[string]bool)
	var items []string
	add := func(item string) {
		item = strings.TrimSpace(item)
		if item == "" {
			return
		}
		if !strings.HasSuffix(item, ".") {
			item += "."
		}
		if seen[item] {
			return
		}
		seen[item] = true
		items = append(items, item)
	}

	for _, suggestion := range suggestions {
		add(suggestion)
	}
	for _, check := range gitops.Checks {
		if check.Name == "Local git" && check.Status == CheckStatusWarn && strings.Contains(check.Message, "dirty") {
			add("Commit or stash local GitOps repository changes before deploy")
		}
	}
	if len(items) == 0 {
		add("No action required")
	}
	return items
}

func buildCheckSummary(result *ValidationResult) ValidationCheckSummary {
	var summary ValidationCheckSummary
	addStatus := func(status CheckStatus) {
		switch status {
		case CheckStatusPass:
			summary.Passed++
		case CheckStatusFail:
			summary.Failed++
		case CheckStatusWarn:
			summary.Warnings++
		case CheckStatusSkip:
			summary.Skipped++
		}
	}

	for _, valid := range []bool{result.ConfigValid, result.ConnectivityValid, result.ProviderValid} {
		if valid {
			addStatus(CheckStatusPass)
		} else {
			addStatus(CheckStatusFail)
		}
	}
	for _, service := range result.ServiceReports {
		addStatus(service.Status)
	}
	for _, check := range result.GitOpsReport.Checks {
		addStatus(check.Status)
	}
	return summary
}
