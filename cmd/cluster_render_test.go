package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
)

func newTestCmd() (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{Use: "test"}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	return cmd, &buf
}

func newTestConfig(dir string) *v2.Config {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.GitOps.Repository.LocalDir = dir
	cfg.OpenCenter.Services = make(v2.ServiceMap)
	return cfg
}

func TestRenderServicesOnly_DryRun(t *testing.T) {
	cfg := newTestConfig(t.TempDir())
	cmd, buf := newTestCmd()

	err := renderServicesOnly(cfg, false, true, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DRY RUN") {
		t.Error("expected DRY RUN in output")
	}
	if !strings.Contains(output, "test-cluster") {
		t.Error("expected cluster name in output")
	}
}

func TestRenderServicesOnly_DryRunForce(t *testing.T) {
	cfg := newTestConfig(t.TempDir())
	cmd, buf := newTestCmd()

	err := renderServicesOnly(cfg, true, true, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Create timestamped backups") {
		t.Error("expected backup notice in force dry-run output")
	}
}

func TestRenderServicesOnly_AlreadyRenderedNoForce(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(dir)

	// Create the kustomization file to simulate already-rendered state
	kustomizationDir := filepath.Join(dir, "applications", "overlays", "test-cluster")
	if err := os.MkdirAll(kustomizationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kustomizationDir, "kustomization.yaml"), []byte("---"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd, _ := newTestCmd()
	err := renderServicesOnly(cfg, false, false, cmd)
	if err == nil {
		t.Fatal("expected error when already rendered without force")
	}
	if !strings.Contains(err.Error(), "already rendered") {
		t.Errorf("expected 'already rendered' error, got: %v", err)
	}
}

func TestRenderSingleService_NotFound(t *testing.T) {
	cfg := newTestConfig(t.TempDir())
	cmd, _ := newTestCmd()

	err := renderSingleService(cfg, "nonexistent", false, false, cmd)
	if err == nil {
		t.Fatal("expected error for non-existent service")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRenderSingleService_Disabled(t *testing.T) {
	cfg := newTestConfig(t.TempDir())
	cfg.OpenCenter.Services["my-svc"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: false},
	}
	cmd, _ := newTestCmd()

	err := renderSingleService(cfg, "my-svc", false, false, cmd)
	if err == nil {
		t.Fatal("expected error for disabled service")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("expected 'disabled' error, got: %v", err)
	}
}

func TestRenderSingleService_DryRun(t *testing.T) {
	cfg := newTestConfig(t.TempDir())
	cfg.OpenCenter.Services["my-svc"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
	}
	cmd, buf := newTestCmd()

	err := renderSingleService(cfg, "my-svc", false, true, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DRY RUN") {
		t.Error("expected DRY RUN in output")
	}
	if !strings.Contains(output, "my-svc") {
		t.Error("expected service name in output")
	}
}

func TestRenderSingleService_AlreadyExistsNoForce(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(dir)
	cfg.OpenCenter.Services["my-svc"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
	}

	// Create the service directory to simulate existing render
	serviceDir := filepath.Join(dir, "applications", "overlays", "test-cluster", "services", "my-svc")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd, _ := newTestCmd()
	err := renderSingleService(cfg, "my-svc", false, false, cmd)
	if err == nil {
		t.Fatal("expected error when service already exists without force")
	}
	if !strings.Contains(err.Error(), "already exist") {
		t.Errorf("expected 'already exist' error, got: %v", err)
	}
}

func TestRenderInfrastructureOnly_DryRun(t *testing.T) {
	cfg := newTestConfig(t.TempDir())
	cmd, buf := newTestCmd()

	err := renderInfrastructureOnly(cfg, true, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DRY RUN") {
		t.Error("expected DRY RUN in output")
	}
	if !strings.Contains(output, "infrastructure") {
		t.Error("expected 'infrastructure' in output")
	}
}

func TestRenderInfrastructureOnly_DryRunWithExistingInfra(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(dir)

	// Create the infrastructure directory
	infraPath := filepath.Join(dir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(infraPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd, buf := newTestCmd()
	err := renderInfrastructureOnly(cfg, true, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Create timestamped backups") {
		t.Error("expected backup notice when existing infra detected in dry-run")
	}
}

func TestCheckRenderStatus_AlreadyRendered(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig(dir)

	// Create the kustomization file
	kustomizationDir := filepath.Join(dir, "applications", "overlays", "test-cluster")
	if err := os.MkdirAll(kustomizationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kustomizationDir, "kustomization.yaml"), []byte("---"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd, buf := newTestCmd()
	err := checkRenderStatus(cfg, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Render complete") {
		t.Error("expected 'Render complete' in output")
	}
	if !strings.Contains(output, "already been rendered") {
		t.Error("expected 'already been rendered' in output")
	}
}

func TestBackupServiceDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create an existing backup file that should be skipped
	if err := os.WriteFile(filepath.Join(dir, "old.yaml.bak-20250101-000000"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd, _ := newTestCmd()
	err := backupServiceDirectory(dir, "test-svc", cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify backups were created for non-backup files
	entries, _ := os.ReadDir(dir)
	backupCount := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak-") && e.Name() != "old.yaml.bak-20250101-000000" {
			backupCount++
		}
	}
	if backupCount != 2 {
		t.Errorf("expected 2 new backup files, got %d", backupCount)
	}
}

// TestBackupInfrastructureDirectory_SkipsTofuTrees guards against the
// regression that broke `cluster destroy`: the force-render backup walker
// copied files inside .terraform/providers and .terraform/modules, and
// OpenTofu then rejected the provider cache ("existing cached package ...
// does not match the content of the downloaded package"), failing
// tofu init during destroy. tfstate under nested .opentofu-* dirs must
// also stay untouched.
func TestBackupInfrastructureDirectory_SkipsTofuTrees(t *testing.T) {
	root := t.TempDir()

	writeAt := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeAt("main.tf")
	writeAt(".terraform.lock.hcl")
	writeAt(".terraform/providers/registry.opentofu.org/hashicorp/local/2.9.1/darwin_arm64/terraform-provider-local")
	writeAt(".terraform/modules/openstack-nova/main.tf")
	writeAt(".opentofu-local-test-cluster/terraform.tfstate")
	writeAt("venv/lib/python3.14/site-packages/ansible/__init__.py")
	writeAt("kubespray/cluster.yml")
	writeAt("inventory/inventory.yaml")

	cmd, _ := newTestCmd()
	if err := backupInfrastructureDirectory(root, "test-cluster", cmd); err != nil {
		t.Fatalf("backupInfrastructureDirectory: %v", err)
	}

	for _, rel := range []string{
		".terraform/providers/registry.opentofu.org/hashicorp/local/2.9.1/darwin_arm64/terraform-provider-local",
		".terraform/modules/openstack-nova/main.tf",
		".opentofu-local-test-cluster/terraform.tfstate",
		"venv/lib/python3.14/site-packages/ansible/__init__.py",
		"kubespray/cluster.yml",
	} {
		matches, err := filepath.Glob(filepath.Join(root, rel)+".bak-*")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Errorf("backup created inside skipped tree: %v", matches)
		}
	}

	for _, rel := range []string{"main.tf", ".terraform.lock.hcl", "inventory/inventory.yaml"} {
		matches, err := filepath.Glob(filepath.Join(root, rel)+".bak-*")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Errorf("expected exactly one backup for %s, got %v", rel, matches)
		}
	}
}

func TestPrintPromotionSummaryIncludesCountsDestructivePathsAndBackups(t *testing.T) {
	cmd, buf := newTestCmd()
	printPromotionSummary(cmd, &gitops.PromoteResult{
		Added:           []string{"added.yaml"},
		Updated:         []string{"updated.yaml"},
		Unchanged:       []string{"same.yaml"},
		Seeded:          []string{"seed.yaml"},
		Pruned:          []string{"stale.yaml"},
		PruneCandidates: []string{"retained.yaml"},
		Renamed:         []string{"old.yaml -> new.yaml"},
		Adopted:         []string{"collision.yaml"},
		BackupPaths:     []string{"/tmp/.opencenter-backup/backup/collision.yaml"},
	})
	output := buf.String()
	for _, want := range []string{
		"added=1 updated=1 unchanged=1 seeded=1 pruned=1 prune-candidates=1 renamed=1 adopted=1",
		"pruned: stale.yaml",
		"prune candidate (retained): retained.yaml",
		"renamed: old.yaml -> new.yaml",
		"adopted: collision.yaml",
		"backup: /tmp/.opencenter-backup/backup/collision.yaml",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("summary missing %q:\n%s", want, output)
		}
	}
}

func TestRenderOnlyValidationGateAndExplicitSkip(t *testing.T) {
	cfg := newTestConfig(t.TempDir())
	if validator, err := renderOnlyManifestValidator(cfg, false); err == nil || validator != nil {
		t.Fatalf("default render-only validation returned validatorNil=%t, err=%v; want blocking error", validator == nil, err)
	}
	validator, err := renderOnlyManifestValidator(cfg, true)
	if err != nil {
		t.Fatalf("skip-validation gate error = %v", err)
	}
	if validator != nil {
		t.Fatal("skip-validation returned a manifest validator")
	}
}

func TestRenderSingleServiceForceBackupWaitsForFinalizationAndOwnershipPreflight(t *testing.T) {
	for _, tt := range []struct {
		name       string
		encryptErr error
		addUnknown bool
		wantError  string
	}{
		{name: "encryption failure", encryptErr: fmt.Errorf("synthetic encryption failure"), wantError: "synthetic encryption failure"},
		{name: "ownership failure", addUnknown: true, wantError: "user-authored files"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			cfg, err := v2.NewV2Default("scoped-backup", "kind")
			if err != nil {
				t.Fatalf("NewV2Default() error = %v", err)
			}
			cfg.OpenCenter.GitOps.Repository.LocalDir = repo
			cfg.OpenCenter.Services["metallb"].(*services.MetalLBConfig).Enabled = true
			if err := gitops.RenderClusterApps(*cfg); err != nil {
				t.Fatalf("initial RenderClusterApps() error = %v", err)
			}
			serviceDir := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "metallb")
			if tt.addUnknown {
				if err := os.WriteFile(filepath.Join(serviceDir, "hand-authored.yaml"), []byte("keep\n"), 0o644); err != nil {
					t.Fatalf("write ownership conflict: %v", err)
				}
			}

			oldEncrypt := encryptRenderedServiceOverrides
			encryptRenderedServiceOverrides = func(context.Context, string, *v2.Config) error { return tt.encryptErr }
			defer func() { encryptRenderedServiceOverrides = oldEncrypt }()

			cmd, _ := newTestCmd()
			err = renderSingleServiceWithOptions(cfg, "metallb", true, false, true, false, cmd)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("renderSingleServiceWithOptions() error = %v, want %q", err, tt.wantError)
			}
			backups, globErr := filepath.Glob(filepath.Join(serviceDir, "*.bak-*"))
			if globErr != nil {
				t.Fatal(globErr)
			}
			if len(backups) != 0 {
				t.Fatalf("force backup created before scoped preflight: %v", backups)
			}
		})
	}
}
