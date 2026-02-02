package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestConfig(t *testing.T, dir, name, provider, gitDir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if gitDir != "" {
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatalf("mkdir git dir: %v", err)
		}
	}
	content := fmt.Sprintf(`opencenter:
  meta:
    organization: opencenter
  infrastructure:
    provider: %s
  cluster:
    cluster_name: %s
  gitops:
    git_dir: %q
`, provider, name, gitDir)
	// Create cluster directory structure with organization
	orgDir := filepath.Join(dir, "clusters", "opencenter")
	if err := os.MkdirAll(orgDir, 0o755); err != nil {
		t.Fatalf("create organization directory: %v", err)
	}

	path := filepath.Join(orgDir, "."+name+"-config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestClusterBootstrapDryRunMake(t *testing.T) {
	t.Setenv("CONTAINER_RUNTIME", "")

	cfgDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "repo")
	clusterDir := filepath.Join(gitDir, "infrastructure", "clusters", "demo")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	writeTestConfig(t, cfgDir, "demo", "openstack", gitDir)
	t.Setenv("OPENCENTER_CONFIG_DIR", cfgDir)

	cmd := newClusterBootstrapCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("bootstrap command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "$ KUBECONFIG=./kubeconfig.yaml make") {
		t.Fatalf("expected make command in output, got: %s", output)
	}
	if !strings.Contains(output, "Bootstrap complete.") {
		t.Fatalf("expected completion message in output, got: %s", output)
	}
}

func TestClusterBootstrapDryRunKind(t *testing.T) {
	cfgDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "repo")
	writeTestConfig(t, cfgDir, "demo", "kind", gitDir)
	t.Setenv("OPENCENTER_CONFIG_DIR", cfgDir)
	t.Setenv("CONTAINER_RUNTIME", "docker")

	cmd := newClusterBootstrapCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"demo", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("bootstrap command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "kind create cluster --name demo --config=-") {
		t.Fatalf("expected kind create command in output, got: %s", output)
	}
	if !strings.Contains(output, "kind export kubeconfig --name demo") {
		t.Fatalf("expected kind export command in output, got: %s", output)
	}
}
