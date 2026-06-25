package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
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
