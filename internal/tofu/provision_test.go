package tofu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func TestProvisionProviderFile(t *testing.T) {
	dir := t.TempDir()
	cfg := config.NewDefault("dev")
	cfg.OpenCenter.GitOps.GitDir = dir
	cfg.OpenTofu.Enabled = true
	cfg.OpenTofu.Backend.Type = "local"
	cfg.OpenTofu.Backend.Local.Path = "terraform.tfstate"

	if err := Provision(cfg); err != nil {
		t.Fatal(err)
	}

	prov := filepath.Join(dir, "infrastructure", "clusters", "dev", "provider.tf")
	if _, err := os.Stat(prov); os.IsNotExist(err) {
		t.Fatalf("provider.tf not created at %s", prov)
	}
	if b, _ := os.ReadFile(prov); len(b) == 0 {
		t.Error("provider.tf is empty")
	}
}
