package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestClusterNormalizeCommandShape(t *testing.T) {
	cmd := newClusterNormalizeCmd()

	if cmd == nil {
		t.Fatal("normalize command should not be nil")
	}
	if cmd.Use != "normalize [name]" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "normalize [name]")
	}
	if cmd.Flags().Lookup("no-backup") == nil {
		t.Fatal("expected --no-backup flag")
	}
	if cmd.Flags().Lookup("dry-run") != nil {
		t.Fatal("did not expect command-local --dry-run flag")
	}
}

func TestClusterNormalizeDryRunDoesNotWriteOrCreateBackup(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	clusterPaths := writeNormalizeExpandableConfig(t, dir, "normalize-dry-run")
	originalData := readFileForNormalizeTest(t, clusterPaths.ConfigPath)

	root := newClusterNormalizeRootForTest()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"cluster", "normalize", "normalize-dry-run", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster normalize --dry-run failed: %v", err)
	}

	data := readFileForNormalizeTest(t, clusterPaths.ConfigPath)
	if !bytes.Equal(data, originalData) {
		t.Fatalf("dry-run wrote config:\n%s", string(data))
	}
	if backups := normalizeBackupFiles(t, clusterPaths.ConfigPath); len(backups) != 0 {
		t.Fatalf("dry-run created backup files: %v", backups)
	}
	if !strings.Contains(out.String(), "Would normalize configuration:") {
		t.Fatalf("expected dry-run normalize summary, got:\n%s", out.String())
	}
}

func TestClusterNormalizeCreatesBackupByDefault(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	clusterPaths := writeNormalizeExpandableConfig(t, dir, "normalize-backup")
	originalData := readFileForNormalizeTest(t, clusterPaths.ConfigPath)

	cmd := newClusterNormalizeCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"normalize-backup"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster normalize failed: %v", err)
	}

	data := readFileForNormalizeTest(t, clusterPaths.ConfigPath)
	if bytes.Equal(data, originalData) {
		t.Fatalf("expected normalize to write expanded config")
	}
	if !strings.Contains(string(data), "kind:") {
		t.Fatalf("expected normalized config to contain kind defaults, got:\n%s", string(data))
	}

	backups := normalizeBackupFiles(t, clusterPaths.ConfigPath)
	if len(backups) != 1 {
		t.Fatalf("expected one backup file, got %d: %v", len(backups), backups)
	}
	backupData := readFileForNormalizeTest(t, backups[0])
	if !bytes.Equal(backupData, originalData) {
		t.Fatalf("expected backup to contain original config:\n%s", string(backupData))
	}
	if !strings.Contains(out.String(), "Backup created:") {
		t.Fatalf("expected backup summary, got:\n%s", out.String())
	}
}

func TestClusterNormalizeNoBackupWritesWithoutCreatingBackup(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	clusterPaths := writeNormalizeExpandableConfig(t, dir, "normalize-no-backup")
	originalData := readFileForNormalizeTest(t, clusterPaths.ConfigPath)

	cmd := newClusterNormalizeCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"normalize-no-backup", "--no-backup"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster normalize --no-backup failed: %v", err)
	}

	data := readFileForNormalizeTest(t, clusterPaths.ConfigPath)
	if bytes.Equal(data, originalData) {
		t.Fatalf("expected normalize --no-backup to write expanded config")
	}
	if !strings.Contains(string(data), "kind:") {
		t.Fatalf("expected normalized config to contain kind defaults, got:\n%s", string(data))
	}
	if backups := normalizeBackupFiles(t, clusterPaths.ConfigPath); len(backups) != 0 {
		t.Fatalf("expected no backup files with --no-backup, got: %v", backups)
	}
	if strings.Contains(out.String(), "Backup created:") {
		t.Fatalf("did not expect backup summary, got:\n%s", out.String())
	}
}

func writeNormalizeExpandableConfig(t *testing.T, dir, clusterName string) *paths.ClusterPaths {
	t.Helper()

	_, clusterPaths := createClusterDirectoriesForTest(t, dir, clusterName, "opencenter")

	cfgPtr, err := v2.NewV2Default(clusterName, "kind")
	if err != nil {
		t.Fatalf("create native v2 kind config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = "opencenter"
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir
	cfg.OpenCenter.Infrastructure.Kind = nil

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal minimal normalize config: %v", err)
	}
	if err := os.WriteFile(clusterPaths.ConfigPath, data, 0600); err != nil {
		t.Fatalf("write minimal normalize config: %v", err)
	}

	return clusterPaths
}

func readFileForNormalizeTest(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func normalizeBackupFiles(t *testing.T, configPath string) []string {
	t.Helper()

	matches, err := filepath.Glob(configPath + ".backup.*")
	if err != nil {
		t.Fatalf("glob normalize backups: %v", err)
	}
	return matches
}

func newClusterNormalizeRootForTest() *cobra.Command {
	root := &cobra.Command{
		Use: "opencenter",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return applyGlobalOptions(cmd, args)
		},
	}
	addGlobalFlags(root)
	cluster := &cobra.Command{Use: "cluster"}
	cluster.AddCommand(newClusterNormalizeCmd())
	root.AddCommand(cluster)
	return root
}
