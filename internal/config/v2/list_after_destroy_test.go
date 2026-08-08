package v2

import (
	"os"
	"path/filepath"
	"testing"
)

// A destroyed cluster must stop being listed.
//
// `cluster destroy --force --remove-files` removes <name>-config.yaml, but
// Delete writes a .backup beside it first, so the directory survives. Discovery
// accepted `Exists(orgDir/clusterName)` as an alternative to finding a config —
// which is the directory already being iterated, always true — so the config
// check never ran and every leftover directory was reported as a cluster.
//
// The symptom was destroy exiting 0 while `cluster list` still showed the
// cluster: two commands disagreeing about whether something exists.

func TestADirectoryHoldingOnlyABackupIsNotACluster(t *testing.T) {
	org := t.TempDir()
	cluster := filepath.Join(org, "gone")
	if err := os.MkdirAll(cluster, 0o755); err != nil {
		t.Fatal(err)
	}
	// Exactly what destroy leaves: the backup, and no configuration.
	if err := os.WriteFile(filepath.Join(cluster, "gone-config.yaml.backup"),
		[]byte("opencenter: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if hasClusterConfig(cluster) {
		t.Error("a directory holding only a backup was treated as a cluster")
	}
}

func TestADirectoryHoldingAConfigIsACluster(t *testing.T) {
	org := t.TempDir()
	cluster := filepath.Join(org, "live")
	if err := os.MkdirAll(cluster, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cluster, "live-config.yaml"),
		[]byte("opencenter: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !hasClusterConfig(cluster) {
		t.Error("a directory holding a configuration was not treated as a cluster")
	}
}

// Tightening discovery must not hide a cluster whose configuration does not
// match its directory name — a renamed cluster, or one from an earlier layout.
func TestAConfigUnderADifferentNameStillCounts(t *testing.T) {
	org := t.TempDir()
	cluster := filepath.Join(org, "renamed-directory")
	if err := os.MkdirAll(cluster, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cluster, "original-name-config.yaml"),
		[]byte("opencenter: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !hasClusterConfig(cluster) {
		t.Error("a cluster whose config does not match its directory name was hidden")
	}
}

func TestAnEmptyDirectoryIsNotACluster(t *testing.T) {
	org := t.TempDir()
	cluster := filepath.Join(org, "empty")
	if err := os.MkdirAll(cluster, 0o755); err != nil {
		t.Fatal(err)
	}
	if hasClusterConfig(cluster) {
		t.Error("an empty directory was treated as a cluster")
	}
}
