package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/testenv"
)

func TestClusterImportScanReportAndApply(t *testing.T) {
	dirs := testenv.SetIsolatedCLIDirs(t)
	prepareCommandTestEnv(t, dirs.ConfigDir)
	t.Setenv("OPENCENTER_STATE_DIR", dirs.StateDir)

	repoPath, err := filepath.Abs(filepath.Join("..", "testdata", "example-inc"))
	if err != nil {
		t.Fatalf("resolve repo path: %v", err)
	}

	scanCmd := newClusterImportCmd()
	var scanOut bytes.Buffer
	scanCmd.SetOut(&scanOut)
	scanCmd.SetErr(&scanOut)
	scanCmd.SetArgs([]string{"scan", "--repo", repoPath})

	if err := scanCmd.Execute(); err != nil {
		t.Fatalf("scan command failed: %v", err)
	}
	if !strings.Contains(scanOut.String(), "Discovered 5 clusters") {
		t.Fatalf("expected scan summary, got:\n%s", scanOut.String())
	}

	reportCmd := newClusterImportCmd()
	var reportOut bytes.Buffer
	reportCmd.SetOut(&reportOut)
	reportCmd.SetErr(&reportOut)
	reportCmd.SetArgs([]string{"report", "--repo", repoPath, "--output", "json"})

	if err := reportCmd.Execute(); err != nil {
		t.Fatalf("report command failed: %v", err)
	}
	if !strings.Contains(reportOut.String(), "\"clusters_discovered\": 5") {
		t.Fatalf("expected report summary in json, got:\n%s", reportOut.String())
	}

	applyCmd := newClusterImportCmd()
	var applyOut bytes.Buffer
	applyCmd.SetOut(&applyOut)
	applyCmd.SetErr(&applyOut)
	applyCmd.SetArgs([]string{"apply", "--repo", repoPath})

	if err := applyCmd.Execute(); err != nil {
		t.Fatalf("apply command failed: %v", err)
	}
	if !strings.Contains(applyOut.String(), "Created config for") {
		t.Fatalf("expected apply output to mention created configs, got:\n%s", applyOut.String())
	}

	resolver := paths.NewPathResolver(filepath.Join(dirs.ConfigDir, "clusters"))
	clusterPaths, err := resolver.Resolve(context.Background(), "k8s-prod", "example-platform")
	if err != nil {
		t.Fatalf("resolve imported cluster path: %v", err)
	}
	data, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read imported config: %v", err)
	}
	if !strings.Contains(string(data), "worker_count: 3") {
		t.Fatalf("expected imported config to include worker count, got:\n%s", string(data))
	}
}

func TestClusterImportApplyPatchesExistingConfig(t *testing.T) {
	dirs := testenv.SetIsolatedCLIDirs(t)
	prepareCommandTestEnv(t, dirs.ConfigDir)
	t.Setenv("OPENCENTER_STATE_DIR", dirs.StateDir)

	repoPath, err := filepath.Abs(filepath.Join("..", "testdata", "example-inc"))
	if err != nil {
		t.Fatalf("resolve repo path: %v", err)
	}

	scanCmd := newClusterImportCmd()
	scanCmd.SetOut(&bytes.Buffer{})
	scanCmd.SetErr(&bytes.Buffer{})
	scanCmd.SetArgs([]string{"scan", "--repo", repoPath})
	if err := scanCmd.Execute(); err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	resolver := paths.NewPathResolver(filepath.Join(dirs.ConfigDir, "clusters"))
	if err := resolver.CreateClusterDirectories(context.Background(), "k8s-prod", "example-platform"); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	clusterPaths, err := resolver.Resolve(context.Background(), "k8s-prod", "example-platform")
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}

	original := `schema_version: "2.0"
opencenter:
  meta:
    name: k8s-prod
    organization: example-platform
    env: production
    region: ord1
  cluster:
    cluster_name: k8s-prod
    kubernetes:
      version: 1.32.7
  infrastructure:
    provider: vmware
    compute:
      master_count: 3
      worker_count: 1
`
	if err := os.WriteFile(clusterPaths.ConfigPath, []byte(original), 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	applyCmd := newClusterImportCmd()
	var applyOut bytes.Buffer
	applyCmd.SetOut(&applyOut)
	applyCmd.SetErr(&applyOut)
	applyCmd.SetArgs([]string{"apply", "--repo", repoPath})

	if err := applyCmd.Execute(); err != nil {
		t.Fatalf("apply command failed: %v", err)
	}

	updated, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if !strings.Contains(string(updated), "worker_count: 3") {
		t.Fatalf("expected patched worker count in existing config, got:\n%s", string(updated))
	}
	if !strings.Contains(applyOut.String(), "@@") {
		t.Fatalf("expected unified diff in apply output, got:\n%s", applyOut.String())
	}
}
