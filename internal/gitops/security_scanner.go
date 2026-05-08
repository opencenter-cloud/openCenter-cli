package gitops

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SecretScanOptions controls GitOps secret scanning.
type SecretScanOptions struct {
	Root   string
	Staged bool
}

// SecretScanFinding describes a secret policy violation in a GitOps tree.
type SecretScanFinding struct {
	Path    string
	Rule    string
	Message string
}

var rawSecretPatterns = []struct {
	rule    string
	message string
	re      *regexp.Regexp
}{
	{
		rule:    "age-private-key",
		message: "Age private key material must not be committed",
		re:      regexp.MustCompile(`AGE-SECRET-KEY-`),
	},
	{
		rule:    "private-key",
		message: "private key material must not be committed",
		re:      regexp.MustCompile(`-----BEGIN (?:OPENSSH |RSA |EC |DSA |ENCRYPTED )?PRIVATE KEY-----`),
	},
	{
		rule:    "git-token",
		message: "inline Git tokens or credential-bearing URLs must not be committed",
		re: regexp.MustCompile(
			`(?:gh[pousr]_[A-Za-z0-9_]{20,}|glpat-[A-Za-z0-9_-]{20,}|https://[^\s/:]+:[^\s@]+@[^\s]+)`,
		),
	},
}

// ScanGitOpsSecrets scans a GitOps worktree for committed secret material.
func ScanGitOpsSecrets(root string) ([]SecretScanFinding, error) {
	return ScanGitOpsSecretsWithOptions(context.Background(), SecretScanOptions{Root: root})
}

// ScanGitOpsSecretsWithOptions scans a worktree or its staged blobs.
func ScanGitOpsSecretsWithOptions(ctx context.Context, opts SecretScanOptions) ([]SecretScanFinding, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	if opts.Staged {
		return scanStagedGitOpsSecrets(ctx, absRoot)
	}
	return scanWorktreeGitOpsSecrets(absRoot)
}

func scanWorktreeGitOpsSecrets(root string) ([]SecretScanFinding, error) {
	var findings []SecretScanFinding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".terraform", "venv", ".bin":
				return filepath.SkipDir
			default:
				return nil
			}
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		var data []byte
		if d.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			data = []byte(target)
		} else {
			data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		findings = append(findings, scanGitOpsFile(filepath.ToSlash(rel), data)...)
		return nil
	})
	return findings, err
}

func scanStagedGitOpsSecrets(ctx context.Context, root string) ([]SecretScanFinding, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--name-only", "-z", "--diff-filter=ACM")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing staged files: %w", err)
	}

	var findings []SecretScanFinding
	for _, rawRel := range bytes.Split(output, []byte{0}) {
		if len(rawRel) == 0 {
			continue
		}
		rel := string(rawRel)
		show := exec.CommandContext(ctx, "git", "show", ":"+rel)
		show.Dir = root
		data, err := show.Output()
		if err != nil {
			return nil, fmt.Errorf("reading staged file %s: %w", rel, err)
		}
		findings = append(findings, scanGitOpsFile(filepath.ToSlash(rel), data)...)
	}
	return findings, nil
}

func scanGitOpsFile(path string, data []byte) []SecretScanFinding {
	var findings []SecretScanFinding
	content := string(data)
	for _, pattern := range rawSecretPatterns {
		if pattern.re.MatchString(content) {
			findings = append(findings, SecretScanFinding{
				Path:    path,
				Rule:    pattern.rule,
				Message: pattern.message,
			})
		}
	}

	if isYAMLPath(path) {
		findings = append(findings, scanYAMLSecrets(path, data)...)
	}
	return findings
}

func scanYAMLSecrets(path string, data []byte) []SecretScanFinding {
	var findings []SecretScanFinding
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc map[string]any
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			// A YAML parse error means we cannot reliably inspect the
			// remaining documents. Record the error and stop scanning this
			// file so we don't miss a Secret hidden behind invalid syntax.
			findings = append(findings, SecretScanFinding{
				Path:    path,
				Rule:    "invalid-yaml",
				Message: fmt.Sprintf("invalid YAML (remaining documents not scanned): %v", err),
			})
			break
		}
		if len(doc) == 0 {
			continue
		}
		kind, _ := doc["kind"].(string)
		if kind != "Secret" {
			continue
		}

		sopsMetadata, ok := doc["sops"]
		if !ok {
			findings = append(findings, SecretScanFinding{
				Path:    path,
				Rule:    "unencrypted-kubernetes-secret",
				Message: "Kubernetes Secret manifests must contain SOPS metadata",
			})
		} else if !hasValidSOPSMetadata(sopsMetadata) {
			findings = append(findings, SecretScanFinding{
				Path:    path,
				Rule:    "invalid-sops-metadata",
				Message: "Kubernetes Secret manifests must contain valid SOPS Age metadata and encrypted MAC",
			})
		}

		for _, field := range []string{"data", "stringData"} {
			for key, value := range yamlStringMap(doc[field]) {
				if !isSOPSEncryptedValue(value) {
					findings = append(findings, SecretScanFinding{
						Path:    path,
						Rule:    "plaintext-secret-field",
						Message: fmt.Sprintf("Secret %s.%s is not SOPS-encrypted", field, key),
					})
				}
			}
		}
	}
	return findings
}

func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func yamlStringMap(value any) map[string]string {
	out := make(map[string]string)
	rawMap, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for key, raw := range rawMap {
		text, ok := raw.(string)
		if !ok {
			out[key] = ""
			continue
		}
		out[key] = text
	}
	return out
}

func hasValidSOPSMetadata(value any) bool {
	metadata, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if !isSOPSEncryptedValue(asYAMLString(metadata["mac"])) {
		return false
	}

	ageRecipients, ok := metadata["age"].([]any)
	if !ok || len(ageRecipients) == 0 {
		return false
	}
	for _, rawRecipient := range ageRecipients {
		recipient, ok := rawRecipient.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(asYAMLString(recipient["recipient"])) == "" {
			continue
		}
		enc := strings.TrimSpace(asYAMLString(recipient["enc"]))
		if isSOPSEncryptedValue(enc) || strings.Contains(enc, "BEGIN AGE ENCRYPTED FILE") {
			return true
		}
	}
	return false
}

func asYAMLString(value any) string {
	text, _ := value.(string)
	return text
}

func isSOPSEncryptedValue(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "ENC[")
}
