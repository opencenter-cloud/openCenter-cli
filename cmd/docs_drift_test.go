package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDocsDoNotUseRemovedGACommands(t *testing.T) {
	forbiddenTexts := removedGAForbiddenTexts()
	forbiddenTokens := []string{
		"--config",
		"--git-ssh-key",
		"--git-token",
		"--git-token-provider",
		"--git-url",
		"--organization",
		"--provider",
	}
	secretsCommand := "secrets"
	secretsKeysCommand := secretsCommand + " keys"
	forbiddenLinePatterns := []forbiddenLinePattern{
		{name: "cluster deploy with --force", parts: []string{"cluster deploy", "--force"}},
		{name: "cluster generate with --git-dir", parts: []string{"cluster generate", "--git-dir"}},
		{name: "cluster generate with --all", parts: []string{"cluster generate", "--all"}},
		{name: "cluster list with --org", parts: []string{"cluster list", "--org"}},
		{name: "validate command with unsupported flag", parts: []string{secretsCommand + " validate", "--cluster"}},
		{name: "validate command with unsupported flag", parts: []string{secretsCommand + " validate", "--key-file"}},
		{name: "validate command with unsupported flag", parts: []string{secretsCommand + " validate", "--verbose"}},
		{name: "encrypt command with unsupported flag", parts: []string{secretsCommand + " encrypt", "--cluster"}},
		{name: "encrypt command with unsupported flag", parts: []string{secretsCommand + " encrypt", "--file"}},
		{name: "decrypt command with unsupported flag", parts: []string{secretsCommand + " decrypt", "--cluster"}},
		{name: "decrypt command with unsupported flag", parts: []string{secretsCommand + " decrypt", "--file"}},
		{name: "backup command with unsupported flag", parts: []string{secretsKeysCommand + " backup", "--cluster"}},
		{name: "backup command with unsupported flag", parts: []string{secretsKeysCommand + " backup", "--timestamp"}},
		{name: "rotate command missing type", parts: []string{secretsKeysCommand + " rotate", "--cluster"}, avoid: []string{"--type"}},
		{name: "revoke command with unsupported key id flag", parts: []string{secretsKeysCommand + " revoke", "--key-id"}},
		{name: "revoke command missing cluster value", parts: []string{secretsKeysCommand + " revoke", "--cluster --"}},
	}
	forbiddenTokenSequences := []forbiddenTokenSequence{
		{name: "opencenter cluster config", tokens: []string{"opencenter", "cluster", "config"}},
	}

	repoRoot := testRepoRoot(t)
	paths := []string{
		filepath.Join(repoRoot, "llms.txt"),
		filepath.Join(repoRoot, "README.md"),
		filepath.Join(repoRoot, "docs"),
		filepath.Join(repoRoot, "tests", "features"),
		filepath.Join(repoRoot, "cmd", "cluster_credentials.go"),
		filepath.Join(repoRoot, "cmd", "cluster_credentials_export.go"),
		filepath.Join(repoRoot, "cmd", "cluster_credentials_unset.go"),
		filepath.Join(repoRoot, "cmd", "cluster_install_hooks.go"),
		filepath.Join(repoRoot, "cmd", "cluster_keys.go"),
		filepath.Join(repoRoot, "cmd", "cluster_validate_manifests.go"),
		filepath.Join(repoRoot, "cmd", "root.go"),
		filepath.Join(repoRoot, "cmd", "secrets_validate.go"),
		filepath.Join(repoRoot, "cmd", "secrets_sops_helpers.go"),
		filepath.Join(repoRoot, "internal", "barbican", "client.go"),
		filepath.Join(repoRoot, "internal", "config", "errors.go"),
		filepath.Join(repoRoot, "internal", "config", "suggestions.go"),
		filepath.Join(repoRoot, "internal", "core", "validation", "validators", "sops_key.go"),
		filepath.Join(repoRoot, "internal", "gitops", "gitops-base-dir", "README.md"),
		filepath.Join(repoRoot, "internal", "secrets", "doc.go"),
		filepath.Join(repoRoot, "internal", "secrets", "hooks.go"),
		filepath.Join(repoRoot, "internal", "secrets", "interfaces.go"),
		filepath.Join(repoRoot, "internal", "secrets", "manager.go"),
		filepath.Join(repoRoot, "internal", "sops", "manager.go"),
		filepath.Join(repoRoot, "internal", "template", "FEATURE_FLAG.md"),
		filepath.Join(repoRoot, "internal", "ui", "error_formatter.go"),
		filepath.Join(repoRoot, "internal", "util", "errors", "error_handler.go"),
	}

	var failures []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if !info.IsDir() {
			failures = append(failures, scanDocsFileForForbiddenText(t, repoRoot, path, forbiddenTexts)...)
			failures = append(failures, scanDocsFileForForbiddenTokens(t, repoRoot, path, forbiddenTokens)...)
			failures = append(failures, scanDocsFileForForbiddenLinePatterns(t, repoRoot, path, forbiddenLinePatterns)...)
			failures = append(failures, scanDocsFileForForbiddenTokenSequences(t, repoRoot, path, forbiddenTokenSequences)...)
			continue
		}

		err = filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if filepath.ToSlash(mustRelPath(t, repoRoot, path)) == "docs/superpowers/plans" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".md" && ext != ".feature" {
				return nil
			}
			failures = append(failures, scanDocsFileForForbiddenText(t, repoRoot, path, forbiddenTexts)...)
			failures = append(failures, scanDocsFileForForbiddenTokens(t, repoRoot, path, forbiddenTokens)...)
			failures = append(failures, scanDocsFileForForbiddenLinePatterns(t, repoRoot, path, forbiddenLinePatterns)...)
			failures = append(failures, scanDocsFileForForbiddenTokenSequences(t, repoRoot, path, forbiddenTokenSequences)...)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", path, err)
		}
	}

	if len(failures) > 0 {
		t.Fatalf("docs contain removed GA command text:\n%s", strings.Join(failures, "\n"))
	}
}

func removedGAForbiddenTexts() []string {
	cluster := "cluster"
	opencenterCluster := "opencenter " + cluster
	secrets := "secrets"
	clusterConfig := cluster + " config"
	return []string{
		cluster + " setup",
		cluster + " render",
		cluster + " bootstrap",
		cluster + " preflight",
		cluster + " info",
		cluster + " select",
		cluster + " current",
		cluster + " update",
		clusterConfig + " update",
		clusterConfig + " export-effective",
		clusterConfig + " get",
		clusterConfig + " set",
		cluster + " audit-log",
		cluster + " keys list",
		cluster + " schema",
		cluster + " validate-" + secrets,
		cluster + " sync-" + secrets,
		"validate-" + secrets,
		"sync-" + secrets,
		cluster + " upgrade",
		cluster + " validate-manifests",
		cluster + " sync-status",
		cluster + " check-keys",
		cluster + " rotate-keys",
		cluster + " revoke-key",
		cluster + " install-hooks",
		opencenterCluster + " keys",
		opencenterCluster + " credentials",
		opencenterCluster + " template",
		"secrets hooks install",
		"opencenter " + "sops",
		"--set",
		"--opencenter.",
		"--json",
		"--format",
		"--show-active",
		"--connectivity",
		"config file path string",
		"secrets keys " + "generate " + "--cluster",
		"secrets keys " + "generate " + "--org",
	}
}

type forbiddenLinePattern struct {
	name  string
	parts []string
	avoid []string
}

type forbiddenTokenSequence struct {
	name   string
	tokens []string
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test filename")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
}

func scanDocsFileForForbiddenText(t *testing.T, repoRoot string, path string, forbiddenTexts []string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var failures []string
	for lineNumber, line := range strings.Split(string(content), "\n") {
		normalizedLine := strings.ToLower(line)
		for _, forbidden := range forbiddenTexts {
			if strings.Contains(normalizedLine, strings.ToLower(forbidden)) {
				failures = append(failures, mustRelPath(t, repoRoot, path)+":"+itoa(lineNumber+1)+": "+forbidden)
			}
		}
	}
	return failures
}

func scanDocsFileForForbiddenLinePatterns(t *testing.T, repoRoot string, path string, patterns []forbiddenLinePattern) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var failures []string
	for lineNumber, line := range strings.Split(string(content), "\n") {
		for _, pattern := range patterns {
			if lineContainsAll(line, pattern.parts) && !lineContainsAny(line, pattern.avoid) {
				failures = append(failures, mustRelPath(t, repoRoot, path)+":"+itoa(lineNumber+1)+": "+pattern.name)
			}
		}
	}
	return failures
}

func lineContainsAll(line string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(line, part) {
			return false
		}
	}
	return true
}

func lineContainsAny(line string, parts []string) bool {
	for _, part := range parts {
		if strings.Contains(line, part) {
			return true
		}
	}
	return false
}

func scanDocsFileForForbiddenTokenSequences(t *testing.T, repoRoot string, path string, sequences []forbiddenTokenSequence) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var failures []string
	for lineNumber, line := range strings.Split(string(content), "\n") {
		tokens := tokenFields(line)
		for _, sequence := range sequences {
			if containsTokenSequence(tokens, sequence.tokens) {
				failures = append(failures, mustRelPath(t, repoRoot, path)+":"+itoa(lineNumber+1)+": "+sequence.name)
			}
		}
	}
	return failures
}

func containsTokenSequence(tokens []string, sequence []string) bool {
	if len(sequence) == 0 || len(sequence) > len(tokens) {
		return false
	}
	for i := 0; i <= len(tokens)-len(sequence); i++ {
		match := true
		for j, expected := range sequence {
			if tokens[i+j] != expected {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func scanDocsFileForForbiddenTokens(t *testing.T, repoRoot string, path string, forbiddenTokens []string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var failures []string
	for lineNumber, line := range strings.Split(string(content), "\n") {
		for _, token := range tokenFields(line) {
			for _, forbidden := range forbiddenTokens {
				if token == forbidden || strings.HasPrefix(token, forbidden+"=") {
					failures = append(failures, mustRelPath(t, repoRoot, path)+":"+itoa(lineNumber+1)+": "+forbidden)
				}
			}
		}
	}
	return failures
}

func tokenFields(line string) []string {
	fields := strings.Fields(line)
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		tokens = append(tokens, strings.Trim(field, "`'\"|,;:.()[]{}<>"))
	}
	return tokens
}

func mustRelPath(t *testing.T, base string, path string) string {
	t.Helper()

	rel, err := filepath.Rel(base, path)
	if err != nil {
		t.Fatalf("rel %s from %s: %v", path, base, err)
	}
	return filepath.ToSlash(rel)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
