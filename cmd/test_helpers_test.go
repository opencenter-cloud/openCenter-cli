package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
)

func resetCommandStateForTests() {
	resetConfigManagerForTests()
	resetContainerForTests()
}

func prepareCommandTestEnv(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("OPENCENTER_CONFIG_DIR", dir)
	t.Setenv("HOME", dir)
	t.Setenv("OPENCENTER_CLUSTER", "")
	t.Setenv("OPENCENTER_SESSION_FILE", "")
	t.Setenv("OPENCENTER_SESSION_ID", "")
	t.Setenv("OPENCENTER_TEST_MODE", "1")
	resetCommandStateForTests()
}

func createClusterDirectoriesForTest(t *testing.T, dir, clusterName, organization string) (*paths.PathResolver, *paths.ClusterPaths) {
	t.Helper()

	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	if err := resolver.CreateClusterDirectories(context.Background(), clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	clusterPaths, err := resolver.Resolve(context.Background(), clusterName, organization)
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}

	return resolver, clusterPaths
}

func saveKindConfigForCommandTest(t *testing.T, dir, clusterName, organization string) (v2.Config, *paths.ClusterPaths) {
	t.Helper()

	resolver, clusterPaths := createClusterDirectoriesForTest(t, dir, clusterName, organization)

	cfgPtr, err := v2.NewV2Default(clusterName, "kind")
	if err != nil {
		t.Fatalf("create native v2 kind config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir

	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
	return cfg, clusterPaths
}

func installFakeGitBinary(t *testing.T, binDir string) {
	t.Helper()

	writeFakeExecutable(t, filepath.Join(binDir, "git"), `#!/bin/sh
set -eu
if [ "${1:-}" = "-C" ]; then
  cd "${2:?}"
  shift 2
fi

subcommand="${1:-}"
shift || true

case "$subcommand" in
  init)
    mkdir -p .git
    ;;
  remote)
    action="${1:-}"
    shift || true
    case "$action" in
      add)
        if [ "${1:-}" = "origin" ]; then
          printf '%s\n' "${2:-}" > .git/origin-url
        fi
        ;;
      get-url)
        if [ "${1:-}" = "origin" ] && [ -f .git/origin-url ]; then
          cat .git/origin-url
        else
          exit 1
        fi
        ;;
      set-url)
        if [ "${1:-}" = "origin" ]; then
          printf '%s\n' "${2:-}" > .git/origin-url
        fi
        ;;
    esac
    ;;
  config)
    # Accept any git config command (get or set)
    ;;
  add)
    ;;
  status)
    ;;
  commit)
    ;;
  rev-parse)
    echo deadbeef
    ;;
esac
exit 0
`)
}

func installFakeKindBinary(t *testing.T, binDir string) {
	t.Helper()

	writeFakeExecutable(t, filepath.Join(binDir, "kind"), `#!/bin/sh
set -eu
state_dir="${FAKE_KIND_STATE_DIR:?}"
mkdir -p "$state_dir"
echo "kind $*" >> "$state_dir/kind.log"

subcommand="${1:-}"
shift || true

case "$subcommand" in
  get)
    if [ "${1:-}" = "clusters" ]; then
      if [ -f "$state_dir/clusters" ]; then
        cat "$state_dir/clusters"
      fi
      exit 0
    fi
    ;;
  create)
    name=""
    config_path=""
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --name)
          name="$2"
          shift 2
          ;;
        --config)
          config_path="$2"
          shift 2
          ;;
        *)
          shift
          ;;
      esac
    done
    [ -n "$config_path" ] && [ -f "$config_path" ] || {
      echo "missing kind config" >&2
      exit 1
    }
    printf '%s\n' "$name" > "$state_dir/clusters"
    printf '%s\n' "${KIND_EXPERIMENTAL_PROVIDER:-docker}" > "$state_dir/runtime"
    exit 0
    ;;
  export)
    name=""
    kubeconfig_path=""
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --name)
          name="$2"
          shift 2
          ;;
        --kubeconfig)
          kubeconfig_path="$2"
          shift 2
          ;;
        *)
          shift
          ;;
      esac
    done
    mkdir -p "$(dirname "$kubeconfig_path")"
    cat > "$kubeconfig_path" <<EOF
apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: $name
contexts:
- context:
    cluster: $name
    user: $name
  name: $name
current-context: $name
kind: Config
users:
- name: $name
  user:
    token: fake
EOF
    exit 0
    ;;
  delete)
    name=""
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --name)
          name="$2"
          shift 2
          ;;
        *)
          shift
          ;;
      esac
    done
    if [ -f "$state_dir/delete_fail" ]; then
      echo "forced delete failure for $name" >&2
      exit 1
    fi
    rm -f "$state_dir/clusters"
    exit 0
    ;;
esac

echo "unsupported fake kind invocation: $subcommand $*" >&2
exit 1
`)
}

func installFakeKubectlBinary(t *testing.T, binDir string) {
	t.Helper()

	writeFakeExecutable(t, filepath.Join(binDir, "kubectl"), `#!/bin/sh
set -eu
state_dir="${FAKE_KIND_STATE_DIR:?}"
mkdir -p "$state_dir"
echo "kubectl $*" >> "$state_dir/kubectl.log"

kubeconfig_path=""
if [ "${1:-}" = "--kubeconfig" ]; then
  kubeconfig_path="$2"
  shift 2
fi

case "${1:-}" in
  cluster-info)
    [ -n "$kubeconfig_path" ] && [ -f "$kubeconfig_path" ] || exit 1
    if [ -f "$state_dir/not_ready" ]; then
      exit 1
    fi
    echo "Kubernetes control plane is running at https://127.0.0.1:6443"
    exit 0
    ;;
  config)
    echo "https://127.0.0.1:6443"
    exit 0
    ;;
esac

echo "unsupported fake kubectl invocation: $*" >&2
exit 1
`)
}

func prependTestPath(t *testing.T, binDir string) {
	t.Helper()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeFakeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake executable %s: %v", path, err)
	}
}
