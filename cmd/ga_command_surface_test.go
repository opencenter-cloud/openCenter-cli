package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newGARootForCommandSurfaceTest() *cobra.Command {
	root := &cobra.Command{Use: "opencenter"}
	addGlobalFlags(root)
	root.AddCommand(NewClusterCmd())
	root.AddCommand(NewSettingsCmd())
	root.AddCommand(NewSecretsCmd())
	root.AddCommand(NewPluginsCmd())
	root.AddCommand(NewShellInitCmd())
	root.AddCommand(NewVersionCmd())
	root.InitDefaultCompletionCmd()
	return root
}

func findCommandPath(root *cobra.Command, parts ...string) *cobra.Command {
	current := root
	for _, part := range parts {
		var next *cobra.Command
		for _, child := range current.Commands() {
			if child.Name() == part || child.HasAlias(part) {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		current = next
	}
	return current
}

func requireCommandPath(t *testing.T, root *cobra.Command, parts ...string) *cobra.Command {
	t.Helper()
	cmd := findCommandPath(root, parts...)
	if cmd == nil {
		t.Fatalf("expected command %q to exist", strings.Join(parts, " "))
	}
	if cmd.Hidden {
		t.Fatalf("expected command %q to be public, but it is hidden", strings.Join(parts, " "))
	}
	return cmd
}

func requireNoCommandPath(t *testing.T, root *cobra.Command, parts ...string) {
	t.Helper()
	cmd := findCommandPath(root, parts...)
	if cmd != nil {
		t.Fatalf("expected command %q to be removed from the public tree", strings.Join(parts, " "))
	}
}

func TestGAClusterCommandSurface(t *testing.T) {
	root := newGARootForCommandSurfaceTest()

	expectedClusterCommands := [][]string{
		{"cluster", "init"},
		{"cluster", "configure"},
		{"cluster", "edit"},
		{"cluster", "set"},
		{"cluster", "normalize"},
		{"cluster", "export"},
		{"cluster", "validate"},
		{"cluster", "doctor"},
		{"cluster", "generate"},
		{"cluster", "deploy"},
		{"cluster", "status"},
		{"cluster", "describe"},
		{"cluster", "list"},
		{"cluster", "use"},
		{"cluster", "active"},
		{"cluster", "env"},
		{"cluster", "destroy"},
		{"cluster", "service"},
		{"cluster", "drift"},
		{"cluster", "backup"},
		{"cluster", "import"},
	}
	for _, path := range expectedClusterCommands {
		requireCommandPath(t, root, path...)
	}
	statusCmd := requireCommandPath(t, root, "cluster", "status")
	if statusCmd.Flags().Lookup("sync") == nil {
		t.Fatal("expected cluster status to expose --sync")
	}
	if statusCmd.Flags().Lookup("sync-timeout") == nil {
		t.Fatal("expected cluster status to expose --sync-timeout")
	}

	removedClusterCommands := [][]string{
		{"cluster", "setup"},
		{"cluster", "render"},
		{"cluster", "bootstrap"},
		{"cluster", "preflight"},
		{"cluster", "info"},
		{"cluster", "select"},
		{"cluster", "current"},
		{"cluster", "update"},
		{"cluster", "config"},
		{"cluster", "sync-status"},
		{"cluster", "validate-manifests"},
		{"cluster", "check-keys"},
		{"cluster", "rotate-keys"},
		{"cluster", "revoke-key"},
		{"cluster", "install-hooks"},
		{"cluster", "credentials"},
	}
	for _, path := range removedClusterCommands {
		requireNoCommandPath(t, root, path...)
	}
}

func TestRemovedClusterCommandsAreRejectedAtRuntime(t *testing.T) {
	removedClusterCommands := [][]string{
		{"cluster", "setup"},
		{"cluster", "render"},
		{"cluster", "bootstrap"},
		{"cluster", "preflight"},
		{"cluster", "info"},
		{"cluster", "select"},
		{"cluster", "current"},
		{"cluster", "update"},
		{"cluster", "config"},
		{"cluster", "sync-status"},
		{"cluster", "validate-manifests"},
		{"cluster", "check-keys"},
		{"cluster", "rotate-keys"},
		{"cluster", "revoke-key"},
		{"cluster", "install-hooks"},
		{"cluster", "credentials"},
	}

	for _, args := range removedClusterCommands {
		root := newGARootForCommandSurfaceTest()
		var out bytes.Buffer
		var errOut bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&errOut)
		root.SetArgs(args)

		err := root.Execute()
		if err == nil {
			t.Fatalf("expected removed command %q to be rejected, got success with stdout:\n%s", strings.Join(args, " "), out.String())
		}
		if !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("expected removed command %q to report unknown command, got: %v", strings.Join(args, " "), err)
		}
	}
}

func TestGAClusterParentHelpShowsWorkflowAndGAExamples(t *testing.T) {
	root := newGARootForCommandSurfaceTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"cluster", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster help failed: %v", err)
	}

	help := out.String()
	expectedWorkflow := `Common Workflow:
  1. Create a cluster config
     opencenter cluster init prod --org acme
  2. Complete provider-specific settings
     opencenter cluster configure acme/prod
  3. Validate config and prerequisites
     opencenter cluster validate acme/prod
     opencenter cluster doctor acme/prod
  4. Generate GitOps assets
     opencenter cluster generate acme/prod
  5. Deploy the cluster
     opencenter cluster deploy acme/prod`
	if !strings.Contains(help, expectedWorkflow) {
		t.Fatalf("expected GA workflow block in cluster help, got:\n%s", help)
	}

	for _, example := range []string{
		"opencenter cluster use acme/prod",
		"opencenter cluster active",
		"opencenter cluster generate acme/prod",
		"opencenter cluster deploy acme/prod",
		"opencenter cluster describe acme/prod",
	} {
		if !strings.Contains(help, example) {
			t.Fatalf("expected help example %q, got:\n%s", example, help)
		}
	}

	clusterCommand := "opencenter cluster "
	for _, oldCommand := range []string{
		clusterCommand + "select",
		clusterCommand + "current",
		clusterCommand + "setup",
		clusterCommand + "bootstrap",
		clusterCommand + "preflight",
		clusterCommand + "info",
		clusterCommand + "render",
	} {
		if strings.Contains(help, oldCommand) {
			t.Fatalf("cluster help still references old command %q:\n%s", oldCommand, help)
		}
	}
}

func TestGAGlobalFlags(t *testing.T) {
	root := newGARootForCommandSurfaceTest()

	expected := []string{"config-dir", "log-level", "output", "quiet", "yes", "dry-run"}
	for _, name := range expected {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("expected global flag --%s to exist", name)
		}
	}

	removed := []string{"config", "set", "show-active", "break-lock"}
	for _, name := range removed {
		if root.PersistentFlags().Lookup(name) != nil {
			t.Fatalf("expected global flag --%s to be removed or command-scoped", name)
		}
	}
}

func TestGASecretsKeyOwnership(t *testing.T) {
	root := newGARootForCommandSurfaceTest()

	requireCommandPath(t, root, "secrets", "keys", "check")
	requireCommandPath(t, root, "secrets", "keys", "rotate")
	requireCommandPath(t, root, "secrets", "keys", "revoke")

	requireNoCommandPath(t, root, "cluster", "check-keys")
	requireNoCommandPath(t, root, "cluster", "rotate-keys")
	requireNoCommandPath(t, root, "cluster", "revoke-key")
}
