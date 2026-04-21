package testenv

import (
	"os"
	"path/filepath"
	"testing"
)

// CLIDirs captures an isolated CLI config/state layout for tests.
type CLIDirs struct {
	Root        string
	ConfigDir   string
	ClustersDir string
	PluginsDir  string
	StateDir    string
}

// SetIsolatedCLIDirs configures writable CLI config/state roots for tests.
func SetIsolatedCLIDirs(t *testing.T) CLIDirs {
	t.Helper()

	root := t.TempDir()
	dirs := CLIDirs{
		Root:        root,
		ConfigDir:   filepath.Join(root, "config"),
		ClustersDir: filepath.Join(root, "config", "clusters"),
		PluginsDir:  filepath.Join(root, "config", "plugins"),
		StateDir:    filepath.Join(root, "state"),
	}

	for _, path := range []string{dirs.ConfigDir, dirs.ClustersDir, dirs.PluginsDir, dirs.StateDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create isolated CLI dir %s: %v", path, err)
		}
	}

	t.Setenv("OPENCENTER_CONFIG_DIR", dirs.ConfigDir)
	t.Setenv("OPENCENTER_STATE_DIR", dirs.StateDir)

	return dirs
}
