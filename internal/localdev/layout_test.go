package localdev

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLayout_DefaultsToConfigDirLocal(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", configDir)

	layout, err := ResolveLayout("")
	if err != nil {
		t.Fatalf("ResolveLayout() error = %v", err)
	}

	wantRoot := filepath.Join(configDir, "local")
	if layout.Root != wantRoot {
		t.Fatalf("layout.Root = %q, want %q", layout.Root, wantRoot)
	}
	if layout.AdminTokenPath != filepath.Join(layout.Root, "tokens", "gitea-admin.token") {
		t.Fatalf("unexpected admin token path: %s", layout.AdminTokenPath)
	}
	if layout.CACertPath != filepath.Join(layout.Root, "gitea", "gitea", "certs", "ca.pem") {
		t.Fatalf("unexpected ca path: %s", layout.CACertPath)
	}
}

func TestLayoutEnsureCreatesExpectedDirectories(t *testing.T) {
	layout, err := ResolveLayout(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatalf("ResolveLayout() error = %v", err)
	}

	if err := layout.Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	for _, dir := range []string{layout.Root, layout.GiteaConfDir, layout.GiteaCertDir, layout.TokensDir} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("expected directory %s to exist, err=%v", dir, err)
		}
	}
}
