package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newOutputRootForCommandTest() *cobra.Command {
	root := &cobra.Command{
		Use:           "opencenter",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return applyGlobalOptions(cmd, args)
		},
	}
	addGlobalFlags(root)
	root.AddCommand(NewClusterCmd())
	root.AddCommand(NewSecretsCmd())
	root.AddCommand(NewPluginsCmd())
	return root
}

func TestClusterListUsesGlobalJSONOutput(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveKindConfigForCommandTest(t, dir, "alpha", "opencenter")
	saveKindConfigForCommandTest(t, dir, "beta", "opencenter")

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"cluster", "list", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster list --output json failed: %v", err)
	}

	var names []string
	if err := json.Unmarshal(out.Bytes(), &names); err != nil {
		t.Fatalf("expected JSON cluster list, got %q: %v", out.String(), err)
	}

	got := strings.Join(names, ",")
	if got != "alpha,beta" {
		t.Fatalf("cluster names = %q, want alpha,beta", got)
	}
}

func TestClusterListRejectsLocalJSONFlag(t *testing.T) {
	cmd := newClusterListCmd()

	if cmd.Flags().Lookup("json") != nil {
		t.Fatal("cluster list must use global --output instead of local --json")
	}
}

func TestClusterListRejectsDryRun(t *testing.T) {
	root := newOutputRootForCommandTest()
	root.SetArgs([]string{"cluster", "list", "--dry-run"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected read-only cluster list to reject --dry-run")
	}
	if !strings.Contains(err.Error(), `--dry-run has no effect for read-only command "opencenter cluster list"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
