package cluster

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

func TestNewSetupService(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := validation.NewValidationEngine()

	service := NewSetupService(pathResolver, validationEngine)

	if service == nil {
		t.Fatal("NewSetupService returned nil")
	}

	if service.pathResolver == nil {
		t.Error("pathResolver is nil")
	}

	if service.validationEngine == nil {
		t.Error("validationEngine is nil")
	}
}

func TestSetupService_generateGitOpsManifests_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "gitops")

	cfg := config.Config{}
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = gitDir
	cfg.OpenCenter.Infrastructure.Provider = "openstack"

	pathResolver := paths.NewPathResolver(tmpDir)

	// Create cluster directories
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "test-org"); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	clusterPaths, err := pathResolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("failed to resolve paths: %v", err)
	}

	validationEngine := validation.NewValidationEngine()
	service := NewSetupService(pathResolver, validationEngine)

	count, err := service.generateGitOpsManifests(ctx, cfg, clusterPaths, true)

	if err != nil {
		t.Errorf("generateGitOpsManifests() unexpected error: %v", err)
		return
	}

	if count == 0 {
		t.Error("generateGitOpsManifests() returned 0 manifests in dry-run")
	}
}

func TestSetupService_validateManifests(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "gitops")

	if err := os.MkdirAll(filepath.Join(gitDir, "applications"), 0o755); err != nil {
		t.Fatalf("failed to create applications dir: %v", err)
	}

	pathResolver := paths.NewPathResolver(tmpDir)

	// Create cluster directories
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "test-org"); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	clusterPaths, err := pathResolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("failed to resolve paths: %v", err)
	}
	clusterPaths.GitOpsDir = gitDir

	validationEngine := validation.NewValidationEngine()
	service := NewSetupService(pathResolver, validationEngine)

	err = service.validateManifests(clusterPaths)
	if err != nil {
		t.Errorf("validateManifests() unexpected error: %v", err)
	}
}

func TestSetupService_commitChanges(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping test")
	}

	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "gitops")

	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("failed to create gitops dir: %v", err)
	}

	// Create a file to commit
	if err := os.WriteFile(filepath.Join(gitDir, "test.txt"), []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	pathResolver := paths.NewPathResolver(tmpDir)

	// Create cluster directories
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "test-org"); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	clusterPaths, err := pathResolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("failed to resolve paths: %v", err)
	}
	clusterPaths.GitOpsDir = gitDir

	validationEngine := validation.NewValidationEngine()
	service := NewSetupService(pathResolver, validationEngine)

	commitHash, err := service.commitChanges(ctx, clusterPaths)

	if err != nil {
		t.Errorf("commitChanges() unexpected error: %v", err)
		return
	}

	// Verify commit hash is not empty
	if commitHash == "" {
		t.Error("commitChanges() returned empty commit hash")
	}
}

func TestSetupService_countGeneratedFiles(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles func(t *testing.T, gitDir string)
		wantCount  int
	}{
		{
			name: "empty directory",
			setupFiles: func(t *testing.T, gitDir string) {
				// No files
			},
			wantCount: 0,
		},
		{
			name: "with files",
			setupFiles: func(t *testing.T, gitDir string) {
				// Create some files
				if err := os.WriteFile(filepath.Join(gitDir, "file1.txt"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create file1: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "file2.txt"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create file2: %v", err)
				}

				// Create subdirectory with file
				subDir := filepath.Join(gitDir, "subdir")
				if err := os.MkdirAll(subDir, 0o755); err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create file3: %v", err)
				}
			},
			wantCount: 3,
		},
		{
			name: "with .git directory",
			setupFiles: func(t *testing.T, gitDir string) {
				// Create .git directory (should be skipped)
				gitSubDir := filepath.Join(gitDir, ".git")
				if err := os.MkdirAll(gitSubDir, 0o755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitSubDir, "config"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create git config: %v", err)
				}

				// Create regular file
				if err := os.WriteFile(filepath.Join(gitDir, "file1.txt"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create file1: %v", err)
				}
			},
			wantCount: 1, // Only file1.txt, .git directory is skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gitDir := filepath.Join(tmpDir, "gitops")

			if err := os.MkdirAll(gitDir, 0o755); err != nil {
				t.Fatalf("failed to create gitops dir: %v", err)
			}

			if tt.setupFiles != nil {
				tt.setupFiles(t, gitDir)
			}

			pathResolver := paths.NewPathResolver(tmpDir)
			validationEngine := validation.NewValidationEngine()
			service := NewSetupService(pathResolver, validationEngine)

			count, err := service.countGeneratedFiles(gitDir)

			if err != nil {
				t.Errorf("countGeneratedFiles() unexpected error: %v", err)
				return
			}

			if count != tt.wantCount {
				t.Errorf("countGeneratedFiles() = %v, want %v", count, tt.wantCount)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func setupContains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestSetupService_Setup(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "gitops")

	pathResolver := paths.NewPathResolver(tmpDir)

	// Create cluster directories first
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "opencenter"); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	// Create a minimal config
	cfg := config.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = gitDir
	cfg.OpenCenter.Infrastructure.Provider = "kind"

	// Save config
	if err := config.Save(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	validationEngine := validation.NewValidationEngine()
	service := NewSetupService(pathResolver, validationEngine)

	opts := SetupOptions{
		ClusterName:    "test-cluster",
		Organization:   "opencenter",
		DryRun:         true,
		SkipValidation: true,
	}

	result, err := service.Setup(ctx, opts)

	if err != nil {
		t.Errorf("Setup() error = %v", err)
		return
	}

	if result == nil {
		t.Fatal("Setup() returned nil result")
	}

	if result.GitOpsPath != gitDir {
		t.Errorf("Setup() GitOpsPath = %v, want %v", result.GitOpsPath, gitDir)
	}

	if result.ManifestsCreated == 0 {
		t.Error("Setup() ManifestsCreated = 0")
	}
}

func TestSetupService_Setup_MissingGitDir(t *testing.T) {
	tmpDir := t.TempDir()

	pathResolver := paths.NewPathResolver(tmpDir)

	// Create cluster directories first
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "opencenter"); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	// Create a config without git_dir
	cfg := config.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = "" // Empty git_dir
	cfg.OpenCenter.Infrastructure.Provider = "kind"

	// Save config
	if err := config.Save(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	validationEngine := validation.NewValidationEngine()
	service := NewSetupService(pathResolver, validationEngine)

	opts := SetupOptions{
		ClusterName:    "test-cluster",
		Organization:   "opencenter",
		DryRun:         false,
		SkipValidation: true,
	}

	_, err := service.Setup(ctx, opts)

	if err == nil {
		t.Error("Setup() expected error for missing git_dir")
	}
}

func TestSetupService_Setup_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "gitops")

	pathResolver := paths.NewPathResolver(tmpDir)

	// Create cluster directories first
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "opencenter"); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	// Create an invalid config
	cfg := config.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = gitDir
	cfg.OpenCenter.Infrastructure.Provider = "kind"

	// Save config
	if err := config.Save(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	validationEngine := validation.NewValidationEngine()
	service := NewSetupService(pathResolver, validationEngine)

	opts := SetupOptions{
		ClusterName:    "test-cluster",
		Organization:   "opencenter",
		DryRun:         false,
		SkipValidation: false, // Enable validation
		Force:          false,
	}

	result, err := service.Setup(ctx, opts)

	// Should fail validation but not return error if validation result is captured
	if err == nil && result != nil && result.ValidationPassed {
		t.Error("Setup() expected validation to fail")
	}
}

func TestSetupService_Setup_WithForce(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "gitops")

	// Create gitops directory to simulate existing setup
	if err := os.MkdirAll(filepath.Join(gitDir, "applications"), 0o755); err != nil {
		t.Fatalf("failed to create gitops dir: %v", err)
	}

	pathResolver := paths.NewPathResolver(tmpDir)

	// Create cluster directories first
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "test-cluster", "opencenter"); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	cfg := config.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster"
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.GitOps.GitDir = gitDir
	cfg.OpenCenter.Infrastructure.Provider = "kind"

	if err := config.Save(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	validationEngine := validation.NewValidationEngine()
	service := NewSetupService(pathResolver, validationEngine)

	opts := SetupOptions{
		ClusterName:    "test-cluster",
		Organization:   "opencenter",
		DryRun:         true,
		SkipValidation: true,
		Force:          true, // Force overwrite
	}

	result, err := service.Setup(ctx, opts)

	if err != nil {
		t.Errorf("Setup() with force error = %v", err)
	}

	if result == nil {
		t.Fatal("Setup() returned nil result")
	}
}
