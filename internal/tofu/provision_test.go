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
	info, err := os.Stat(prov)
	if os.IsNotExist(err) {
		t.Fatalf("provider.tf not created at %s", prov)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("provider.tf mode = %v, want 0644", info.Mode().Perm())
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

func TestProvisionAtMaterializesOnlyExplicitStagedRoot(t *testing.T) {
	live := t.TempDir()
	stage := t.TempDir()
	cfg, err := v2.NewV2Default("staged", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = live
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "stage-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "stage-secret"
	cfg.OpenTofu.Enabled = true
	cfg.OpenTofu.Backend.Type = "local"
	cfg.OpenTofu.Backend.Local = &v2.LocalBackendConfig{Path: "terraform.tfstate"}

	if err := ProvisionAt(*cfg, stage); err != nil {
		t.Fatalf("ProvisionAt() error = %v", err)
	}
	clusterDir := filepath.Join(stage, "infrastructure", "clusters", "staged")
	for _, name := range []string{"provider.tf", "terraform.tfvars"} {
		if _, err := os.Stat(filepath.Join(clusterDir, name)); err != nil {
			t.Fatalf("staged %s missing: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(live, "infrastructure", "clusters", "staged", name)); !os.IsNotExist(err) {
			t.Fatalf("ProvisionAt wrote live %s: %v", name, err)
		}
	}
	if info, err := os.Stat(filepath.Join(clusterDir, "terraform.tfvars")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("staged terraform.tfvars mode = %v, %v", info, err)
	}
}
