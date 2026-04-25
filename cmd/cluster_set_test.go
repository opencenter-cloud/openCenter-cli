package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
	"github.com/spf13/cobra"
)

func TestClusterSetUpdatesExplicitField(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "set-cluster", "opencenter")
	cfg.OpenCenter.Meta.Env = "dev"
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"set-cluster", "opencenter.meta.env=prod"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v", err)
	}

	data, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "env: prod") {
		t.Fatalf("expected config to contain env: prod, got:\n%s", string(data))
	}
	if !strings.Contains(out.String(), "Updated cluster configuration set-cluster") {
		t.Fatalf("expected update summary, got:\n%s", out.String())
	}
}

func TestClusterSetDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, clusterPaths := saveKindConfigForCommandTest(t, dir, "dry-run-set", "opencenter")
	cfg.OpenCenter.Meta.Env = "dev"
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	root := newClusterSetRootForTest()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"cluster", "set", "dry-run-set", "opencenter.meta.env=prod", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster set --dry-run failed: %v", err)
	}

	data, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "env: prod") {
		t.Fatalf("dry-run wrote config:\n%s", string(data))
	}
	if !strings.Contains(out.String(), "Would update cluster configuration dry-run-set") {
		t.Fatalf("expected dry-run summary, got:\n%s", out.String())
	}
}

func TestClusterSetUpdatesKindDisableDefaultCNIByPath(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, _ := saveKindConfigForCommandTest(t, dir, "set-kind-cni", "opencenter")
	resolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterSetCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"set-kind-cni", "opencenter.infrastructure.kind.disable_default_cni=true"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster set failed: %v\nstderr: %s", err, stderr.String())
	}

	resetCommandStateForTests()

	updated, err := loadCanonicalConfig("set-kind-cni")
	if err != nil {
		t.Fatalf("load canonical config: %v", err)
	}
	if updated.OpenCenter.Infrastructure.Kind == nil {
		t.Fatal("expected kind infrastructure config to be present")
	}
	if !updated.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
		t.Fatal("expected disable_default_cni to be true after cluster set")
	}
}

func TestClusterSetRejectsMissingAssignmentAfterClusterName(t *testing.T) {
	cmd := newClusterSetCmd()
	cmd.SetArgs([]string{"set-cluster"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected cluster set to reject missing assignment")
	}
	if !strings.Contains(err.Error(), "at least one path=value assignment is required after cluster name") {
		t.Fatalf("expected missing assignment error, got: %v", err)
	}
}

func newClusterSetRootForTest() *cobra.Command {
	root := &cobra.Command{
		Use: "opencenter",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return applyGlobalOptions(cmd, args)
		},
	}
	addGlobalFlags(root)
	cluster := &cobra.Command{Use: "cluster"}
	cluster.AddCommand(newClusterSetCmd())
	root.AddCommand(cluster)
	return root
}
