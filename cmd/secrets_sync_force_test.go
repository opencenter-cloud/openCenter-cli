package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"gopkg.in/yaml.v3"
)

func TestSecretsSync_ForceManifestRefreshEndToEnd(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	binDir := t.TempDir()
	installFakeGitBinary(t, binDir)
	prependTestPath(t, binDir)

	clusterName := "doritos"
	organization := "opensnack"

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, clusterName, organization)

	// Create and save Age key
	keyManager := sops.NewKeyManager(filepath.Dir(clusterPaths.SOPSKeyPath))
	keyPair, err := keyManager.GenerateAgeKey()
	if err != nil {
		t.Fatalf("generate age key: %v", err)
	}
	keyName := strings.TrimSuffix(filepath.Base(clusterPaths.SOPSKeyPath), ".txt")
	if err := keyManager.SaveAgeKey(keyPair, keyName); err != nil {
		t.Fatalf("save age key: %v", err)
	}
	cfg.Secrets.SopsAgeKeyFile = clusterPaths.SOPSKeyPath
	cfg.Secrets.SOPSConfig.AgeKeyFile = clusterPaths.SOPSKeyPath

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(clusterPaths.ConfigPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 1. Initial generate
	resetCommandStateForTests()
	genCmd := newClusterGenerateCmd()
	genCmd.SetOut(&bytes.Buffer{})
	genCmd.SetErr(&bytes.Buffer{})
	genCmd.SetArgs([]string{organization + "/" + clusterName, "--force", "--skip-validation"})
	if err := genCmd.Execute(); err != nil {
		t.Fatalf("initial cluster generate failed: %v", err)
	}

	// Verify that before secrets sync, headlamp/kustomization.yaml does NOT have secret.yaml
	overlayDir := filepath.Join(clusterPaths.GitOpsDir, "applications", "overlays", clusterName)
	headlampKustBefore, err := os.ReadFile(filepath.Join(overlayDir, "services", "headlamp", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read headlamp kustomization: %v", err)
	}
	if strings.Contains(string(headlampKustBefore), "secret.yaml") {
		t.Fatalf("expected headlamp kustomization to not contain secret.yaml before secrets sync")
	}

	// Verify checkSecretsSynced fails before sync
	if err := checkSecretsSynced(&cfg, clusterName); err == nil {
		t.Fatalf("expected checkSecretsSynced to fail before sync")
	}

	// 2. Simulate human commit modifying root .gitignore in GitOps repository (e.g. adding .omp)
	gitIgnorePath := filepath.Join(clusterPaths.GitOpsDir, ".gitignore")
	existingGitIgnore, err := os.ReadFile(gitIgnorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if err := os.WriteFile(gitIgnorePath, append(existingGitIgnore, []byte("\n.omp\n")...), 0o644); err != nil {
		t.Fatalf("modify .gitignore: %v", err)
	}

	// 3. Run secrets sync with --force
	resetCommandStateForTests()
	syncCmd := newSecretsSyncCmd()
	syncCmd.SetOut(&bytes.Buffer{})
	syncCmd.SetErr(&bytes.Buffer{})
	syncCmd.SetArgs([]string{organization + "/" + clusterName, "--force", "--skip-validation"})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("secrets sync --force failed: %v", err)
	}

	// 4. Verify that headlamp/kustomization.yaml now has secret.yaml wired in!
	headlampKustAfter, err := os.ReadFile(filepath.Join(overlayDir, "services", "headlamp", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read headlamp kustomization after sync: %v", err)
	}
	if !strings.Contains(string(headlampKustAfter), "secret.yaml") {
		t.Fatalf("expected headlamp kustomization to contain secret.yaml after sync, got:\n%s", string(headlampKustAfter))
	}

	// 5. Verify checkSecretsSynced now passes!
	if err := checkSecretsSynced(&cfg, clusterName); err != nil {
		t.Fatalf("checkSecretsSynced failed after sync: %v", err)
	}
}
