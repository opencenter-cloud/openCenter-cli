package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/testenv"
)

func TestClusterDriftReconcileToConfigDryRunUsesLiveKubectlState(t *testing.T) {
	dirs := testenv.SetIsolatedCLIDirs(t)
	prepareCommandTestEnv(t, dirs.ConfigDir)
	t.Setenv("OPENCENTER_STATE_DIR", dirs.StateDir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dirs.ConfigDir, "drift-kind", "opencenter")
	cfg.OpenCenter.Cluster.Kubernetes.Version = "1.32.7"
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	if err := saveConfig(context.Background(), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := os.WriteFile(clusterPaths.KubeconfigPath, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	installFakeKubectlForDriftTest(t, t.TempDir(), map[string]string{
		"nodes": `{"items":[
  {"metadata":{"labels":{"node-role.kubernetes.io/control-plane":""}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.8"}}},
  {"metadata":{"labels":{"node-role.kubernetes.io/control-plane":""}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.8"}}},
  {"metadata":{"labels":{"node-role.kubernetes.io/control-plane":""}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.8"}}},
  {"metadata":{"labels":{}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.8"}}},
  {"metadata":{"labels":{}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.8"}}},
  {"metadata":{"labels":{}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.8"}}}
]}`,
		"namespaces": `{"items":[{"metadata":{"name":"velero"}}]}`,
	})

	cmd := newClusterDriftCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"reconcile", "drift-kind", "--dry-run", "--to-config"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("drift reconcile --to-config failed: %v", err)
	}

	text := output.String()
	if !strings.Contains(text, "worker_count: 2") || !strings.Contains(text, "worker_count: 3") {
		t.Fatalf("expected worker count diff in output, got:\n%s", text)
	}
	if !strings.Contains(text, "version: 1.32.7") || !strings.Contains(text, "version: 1.32.8") {
		t.Fatalf("expected version diff in output, got:\n%s", text)
	}
}

// TestClusterDriftReconcileToConfigWritesConfigAndHonorsField proves the
// folded-in promotion behavior: --to-config actually writes the patch (the
// capability the old `cluster promote` owned), and --field restricts the
// promoted paths so an operator can promote the network identity without
// accepting every other live divergence.
func TestClusterDriftReconcileToConfigWritesConfigAndHonorsField(t *testing.T) {
	dirs := testenv.SetIsolatedCLIDirs(t)
	prepareCommandTestEnv(t, dirs.ConfigDir)
	t.Setenv("OPENCENTER_STATE_DIR", dirs.StateDir)
	t.Setenv("OPENCENTER_TEST_MODE", "1")

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dirs.ConfigDir, "drift-write", "opencenter")
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	if err := saveConfig(context.Background(), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := os.WriteFile(clusterPaths.KubeconfigPath, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	installFakeKubectlForDriftTest(t, t.TempDir(), map[string]string{
		"nodes":      `{"items":[{"metadata":{"labels":{"node-role.kubernetes.io/control-plane":""}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.7"}}},{"metadata":{"labels":{}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.7"}}},{"metadata":{"labels":{}},"status":{"nodeInfo":{"kubeletVersion":"v1.32.7"}}}]}`,
		"namespaces": `{"items":[]}`,
	})

	// --field with a path the live source does not report: nothing is written.
	filtered := newClusterDriftCmd()
	var filteredOut bytes.Buffer
	filtered.SetOut(&filteredOut)
	filtered.SetErr(&filteredOut)
	filtered.SetArgs([]string{"reconcile", "drift-write", "--to-config",
		"--field", "opencenter.infrastructure.cloud.openstack.networking.subnet_id"})
	if err := filtered.Execute(); err != nil {
		t.Fatalf("filtered reconcile failed: %v", err)
	}
	if !strings.Contains(filteredOut.String(), "No promotable live-to-config drift detected") {
		t.Fatalf("expected the --field filter to suppress the unrelated live patch, got:\n%s", filteredOut.String())
	}

	// Without --field the patch is applied and the config file changes. The
	// filtered pass cached the pre-patch config; drop it so the apply reads the
	// file the way a fresh process would.
	if manager, err := getConfigManager(); err == nil {
		if err := manager.ClearCache(context.Background()); err != nil {
			t.Fatalf("clear cache: %v", err)
		}
	}

	// Without --field the patch is applied and the config file changes. The
	// write target is the plan's resolved blueprint path (the one the command
	// reports), not the fixture-time resolver view.
	apply := newClusterDriftCmd()
	var applyOut bytes.Buffer
	apply.SetOut(&applyOut)
	apply.SetErr(&applyOut)
	apply.SetArgs([]string{"reconcile", "drift-write", "--to-config"})
	if err := apply.Execute(); err != nil {
		t.Fatalf("apply reconcile failed: %v", err)
	}
	updatedLine := "Updated config for drift-write at "
	index := strings.Index(applyOut.String(), updatedLine)
	if index < 0 {
		t.Fatalf("apply did not report a written config:\n%s", applyOut.String())
	}
	writtenPath := strings.TrimSpace(applyOut.String()[index+len(updatedLine):])
	written, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("read %s: %v", writtenPath, err)
	}
	if !strings.Contains(string(written), "version: 1.32.7") {
		t.Fatalf("config was not patched by --to-config.\napply output:\n%s", applyOut.String())
	}
	if _, err := os.Stat(writtenPath + ".backup"); err != nil {
		t.Fatalf("write-plan backup missing: %v", err)
	}
}

func TestClusterDriftReconcileDefaultsToProviderPath(t *testing.T) {
	dirs := testenv.SetIsolatedCLIDirs(t)
	prepareCommandTestEnv(t, dirs.ConfigDir)
	t.Setenv("OPENCENTER_STATE_DIR", dirs.StateDir)

	_, _ = saveKindConfigForCommandTest(t, dirs.ConfigDir, "drift-kind-default", "opencenter")

	cmd := newClusterDriftCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"reconcile", "drift-kind-default", "--dry-run"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected default drift reconcile to use provider path and fail for kind")
	}
	if !strings.Contains(err.Error(), "failed to get cloud provider") {
		t.Fatalf("expected provider-path error, got %v", err)
	}
}

func installFakeKubectlForDriftTest(t *testing.T, binDir string, responses map[string]string) {
	t.Helper()

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create fake kubectl bin dir: %v", err)
	}

	script := fmt.Sprintf(`#!/bin/sh
set -eu
for arg in "$@"; do
  case "$arg" in
    nodes)
      cat <<'EOF'
%s
EOF
      exit 0
      ;;
    namespaces|ns)
      cat <<'EOF'
%s
EOF
      exit 0
      ;;
  esac
done
echo "unexpected kubectl invocation: $*" >&2
exit 1
`, responses["nodes"], responses["namespaces"])

	path := filepath.Join(binDir, "kubectl")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake kubectl: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
