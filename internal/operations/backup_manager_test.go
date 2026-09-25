package operations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

func TestBackupManagerListBackupsFilenameCases(t *testing.T) {
	backupDir := t.TempDir()
	manager, err := NewBackupManager(paths.NewPathResolver(t.TempDir()), backupDir)
	if err != nil {
		t.Fatalf("NewBackupManager() error = %v", err)
	}

	names := []string{
		"cluster-20260924-120000.tar.gz",
		"cluster-20260924-120000.tar.gz.enc",
		"cluster-extra.tar.gz",
		"cluster.tar.gz",
		"clusterish-20260924-120000.tar.gz",
		"cluster-20260924-120000.tar.gz.sha256",
		"other-20260924-120000.tar.gz",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(backupDir, name), []byte("backup"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	backups, err := manager.ListBackups("cluster")
	if err != nil {
		t.Fatalf("ListBackups() error = %v", err)
	}

	got := make(map[string]bool, len(backups))
	for _, backup := range backups {
		got[filepath.Base(backup.StorageLocation)] = true
	}
	want := map[string]bool{
		"cluster-20260924-120000.tar.gz":     true,
		"cluster-20260924-120000.tar.gz.enc": true,
		"cluster-extra.tar.gz":               true,
	}
	if len(got) != len(want) {
		t.Fatalf("ListBackups() returned %v, want %v", got, want)
	}
	for name := range want {
		if !got[name] {
			t.Errorf("ListBackups() missing expected file %q", name)
		}
	}
}
