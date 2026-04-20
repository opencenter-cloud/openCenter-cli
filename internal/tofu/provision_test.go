package tofu

import (
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
)

func TestProvisionProviderFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := v2.NewV2Default("dev", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = dir
	cfg.OpenTofu.Enabled = true
	cfg.OpenTofu.Backend.Type = "local"
	cfg.OpenTofu.Backend.Local = &v2.LocalBackendConfig{Path: "terraform.tfstate"}

	if err := Provision(*cfg); err != nil {
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

func TestInfrastructureArtifactsAreCoLocated(t *testing.T) {
	dir := t.TempDir()
	cfg, err := v2.NewV2Default("demo", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}

	cfg.OpenCenter.GitOps.Repository.LocalDir = dir
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"
	cfg.OpenTofu.Enabled = true
	cfg.OpenTofu.Backend.Type = "local"
	cfg.OpenTofu.Backend.Local = &v2.LocalBackendConfig{Path: "terraform.tfstate"}

	if err := gitops.RenderInfrastructureCluster(*cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster() error = %v", err)
	}
	if err := Provision(*cfg); err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	clusterDir := filepath.Join(dir, "infrastructure", "clusters", "demo")
	expectedFiles := []string{"main.tf", "variables.tf", "provider.tf", "Makefile"}
	for _, filename := range expectedFiles {
		path := filepath.Join(clusterDir, filename)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected infrastructure file %s: %v", path, err)
		}
	}

	nestedClusterDir := filepath.Join(clusterDir, "infrastructure", "clusters", "demo")
	if _, err := os.Stat(nestedClusterDir); err == nil {
		t.Fatalf("unexpected nested infrastructure directory: %s", nestedClusterDir)
	}
}
