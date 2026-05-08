package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretScannerDetectsPrivateKeysTokensAndPlaintextSecrets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "applications/overlays/demo/services/app/plain-secret.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: safe
---
apiVersion: v1
kind: Secret
metadata:
  name: unsafe
stringData:
  password: plaintext
`)
	writeFile(t, root, "notes/key.txt", "AGE-SECRET-KEY-1EXAMPLE")
	writeFile(t, root, "notes/token.txt", "remote=https://ghp_1234567890abcdefghijklmnopqrstuvwx@example.invalid/repo.git")

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}

	assertFinding(t, findings, "age-private-key")
	assertFinding(t, findings, "git-token")
	assertFinding(t, findings, "unencrypted-kubernetes-secret")
	assertFinding(t, findings, "plaintext-secret-field")
}

func TestSecretScannerAcceptsSOPSEncryptedSecret(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "applications/overlays/demo/services/app/encrypted-secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: encrypted
data:
  password: ENC[AES256_GCM,data:abc,iv:def,tag:ghi,type:str]
sops:
  mac: ENC[AES256_GCM,data:abc,iv:def,tag:ghi,type:str]
  age:
    - recipient: age1example
      enc: |
        -----BEGIN AGE ENCRYPTED FILE-----
        example
        -----END AGE ENCRYPTED FILE-----
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("ScanGitOpsSecrets() findings = %+v, want none", findings)
	}
}

func TestSecretScannerRejectsInvalidSOPSMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "applications/overlays/demo/services/app/invalid-sops-secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: invalid
data:
  password: ENC[AES256_GCM,data:abc,iv:def,tag:ghi,type:str]
sops:
  version: fake
`)

	findings, err := ScanGitOpsSecrets(root)
	if err != nil {
		t.Fatalf("ScanGitOpsSecrets() error = %v", err)
	}
	assertFinding(t, findings, "invalid-sops-metadata")
}

func TestSecretScannerScansStagedFilesWithSpaces(t *testing.T) {
	root := t.TempDir()
	runGitForScannerTest(t, root, "init")

	writeFile(t, root, "applications/overlays/demo/services/app/plain secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: unsafe
stringData:
  password: plaintext
`)
	runGitForScannerTest(t, root, "add", ".")

	findings, err := ScanGitOpsSecretsWithOptions(context.Background(), SecretScanOptions{
		Root:   root,
		Staged: true,
	})
	if err != nil {
		t.Fatalf("ScanGitOpsSecretsWithOptions() error = %v", err)
	}
	assertFinding(t, findings, "unencrypted-kubernetes-secret")
	assertFinding(t, findings, "plaintext-secret-field")
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFinding(t *testing.T, findings []SecretScanFinding, rule string) {
	t.Helper()
	for _, finding := range findings {
		if finding.Rule == rule {
			return
		}
	}
	var rules []string
	for _, finding := range findings {
		rules = append(rules, finding.Rule)
	}
	t.Fatalf("missing finding rule %q in %s", rule, strings.Join(rules, ", "))
}

func runGitForScannerTest(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
