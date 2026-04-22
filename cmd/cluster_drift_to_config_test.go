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
