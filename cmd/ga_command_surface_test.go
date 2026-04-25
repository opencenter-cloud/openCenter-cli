package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newGARootForCommandSurfaceTest() *cobra.Command {
	root := &cobra.Command{Use: "opencenter"}
	addGlobalFlags(root)
	root.AddCommand(NewClusterCmd())
	root.AddCommand(NewConfigCmd())
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
