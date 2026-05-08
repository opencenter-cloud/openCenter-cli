package cluster

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// createTestSetupService creates a SetupService with test dependencies
// that uses LoadWithoutValidation for loading configs in tests
func createTestSetupService(pathResolver *paths.PathResolver) *SetupService {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validation.NewValidationEngine()
	cache := config.NewConfigCache()
	loader := config.NewConfigIOHandler(fileSystem)
	configMgr := config.NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	return NewSetupServiceWithConfigMgr(pathResolver, validator, configMgr)
}

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

	cfg := mustNewClusterTestConfig("test-cluster", "openstack")
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir

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

func TestSetupService_countGeneratedFiles(t *testing.T) {
	tests := []struct {
		name        string
		setupBefore func(t *testing.T, gitDir string) // files present before generation
		setupAfter  func(t *testing.T, gitDir string) // files written during generation
		wantCount   int
	}{
		{
			name:        "empty directory, no new files",
			setupBefore: func(t *testing.T, gitDir string) {},
			setupAfter:  func(t *testing.T, gitDir string) {},
			wantCount:   0,
		},
		{
			name:        "all files are new",
			setupBefore: func(t *testing.T, gitDir string) {},
			setupAfter: func(t *testing.T, gitDir string) {
				if err := os.WriteFile(filepath.Join(gitDir, "file1.txt"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create file1: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "file2.txt"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create file2: %v", err)
				}
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
			name: "pre-existing files are not counted",
			setupBefore: func(t *testing.T, gitDir string) {
				if err := os.WriteFile(filepath.Join(gitDir, "old.txt"), []byte("old"), 0o644); err != nil {
					t.Fatalf("failed to create old file: %v", err)
				}
			},
			setupAfter: func(t *testing.T, gitDir string) {
				if err := os.WriteFile(filepath.Join(gitDir, "new.txt"), []byte("new"), 0o644); err != nil {
					t.Fatalf("failed to create new file: %v", err)
				}
			},
			wantCount: 1, // Only new.txt
		},
		{
			name:        ".git directory is skipped",
			setupBefore: func(t *testing.T, gitDir string) {},
			setupAfter: func(t *testing.T, gitDir string) {
				gitSubDir := filepath.Join(gitDir, ".git")
				if err := os.MkdirAll(gitSubDir, 0o755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitSubDir, "config"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create git config: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "file1.txt"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create file1: %v", err)
				}
			},
			wantCount: 1, // Only file1.txt, .git directory is skipped
		},
		{
			name:        "non-generated directories are skipped",
			setupBefore: func(t *testing.T, gitDir string) {},
			setupAfter: func(t *testing.T, gitDir string) {
				// Create directories that should be excluded from the count:
				// .terraform, venv, kubespray, .bin (matches SOPS exclusion list)
				for _, dir := range []string{".terraform", "venv", "kubespray", ".bin"} {
					d := filepath.Join(gitDir, "infrastructure", "clusters", "test", dir)
					if err := os.MkdirAll(d, 0o755); err != nil {
						t.Fatalf("failed to create %s dir: %v", dir, err)
					}
					if err := os.WriteFile(filepath.Join(d, "state.json"), []byte("{}"), 0o644); err != nil {
						t.Fatalf("failed to create file in %s: %v", dir, err)
					}
				}
				// One real generated file
				if err := os.WriteFile(filepath.Join(gitDir, "kustomization.yaml"), []byte("test"), 0o644); err != nil {
					t.Fatalf("failed to create kustomization.yaml: %v", err)
				}
			},
			wantCount: 1, // Only kustomization.yaml; .terraform, venv, kubespray, .bin skipped
		},
		{
			name: "modified files are counted",
			setupBefore: func(t *testing.T, gitDir string) {
				if err := os.WriteFile(filepath.Join(gitDir, "config.yaml"), []byte("old-content"), 0o644); err != nil {
					t.Fatalf("failed to create config: %v", err)
				}
			},
			setupAfter: func(t *testing.T, gitDir string) {
				// Overwrite existing file (simulates re-generation)
				if err := os.WriteFile(filepath.Join(gitDir, "config.yaml"), []byte("new-content"), 0o644); err != nil {
					t.Fatalf("failed to update config: %v", err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "new.yaml"), []byte("brand-new"), 0o644); err != nil {
					t.Fatalf("failed to create new file: %v", err)
				}
			},
			wantCount: 2, // config.yaml (modified) + new.yaml (new)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gitDir := filepath.Join(tmpDir, "gitops")

			if err := os.MkdirAll(gitDir, 0o755); err != nil {
				t.Fatalf("failed to create gitops dir: %v", err)
			}

			// Set up pre-existing files
			if tt.setupBefore != nil {
				tt.setupBefore(t, gitDir)
			}

			pathResolver := paths.NewPathResolver(tmpDir)
			validationEngine := validation.NewValidationEngine()
			service := NewSetupService(pathResolver, validationEngine)

			// Take snapshot before generation
			snapshot, err := service.snapshotFileModTimes(gitDir)
			if err != nil {
				t.Fatalf("snapshotFileModTimes() unexpected error: %v", err)
			}

			// Simulate generation by writing new/modified files
			if tt.setupAfter != nil {
				tt.setupAfter(t, gitDir)
			}

			count, err := service.countGeneratedFiles(gitDir, snapshot)

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

func TestSetupService_OpenStackSetupDoesNotUseLegacyConfigValidator(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "openstack-setup"
	organization := "test-org"
	gitDir := filepath.Join(tmpDir, "gitops-repo")

	pathResolver := paths.NewPathResolver(tmpDir)
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir

	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validation.NewValidationEngine()
	if err := validator.Register(validators.NewConfigValidator()); err != nil {
		t.Fatalf("register config validator: %v", err)
	}
	cache := config.NewConfigCache()
	loader := config.NewConfigIOHandler(fileSystem)
	configMgr := config.NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	service := NewSetupServiceWithConfigMgr(pathResolver, validator, configMgr)
	result, err := service.Setup(ctx, SetupOptions{
		ClusterName:    clusterName,
		Organization:   organization,
		DryRun:         true,
		SkipValidation: false,
	})
	if err != nil {
		t.Fatalf("Setup() unexpected error: %v", err)
	}
	if result == nil || result.ManifestsCreated == 0 {
		t.Fatalf("expected dry-run setup result with manifest count, got %#v", result)
	}
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
	cfg := mustNewClusterTestConfig("test-cluster", "kind")
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir

	// Save config
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)

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

func TestSetupService_Setup_KindProviderRendersKindConfigOnly(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "gitops")

	pathResolver := paths.NewPathResolver(tmpDir)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, "kind-cluster", "opencenter"); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig("kind-cluster", "openstack")
	cfg.OpenCenter.Meta.Organization = "opencenter"
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	if err := applyClusterProviderDefaults(&cfg, "kind"); err != nil {
		t.Fatalf("apply provider defaults: %v", err)
	}

	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)
	result, err := service.Setup(ctx, SetupOptions{
		ClusterName:    "kind-cluster",
		Organization:   "opencenter",
		DryRun:         false,
		SkipValidation: false,
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Setup returned nil result")
	}

	kindConfigPath := filepath.Join(gitDir, "infrastructure", "clusters", "kind-cluster", "kind-config.yaml")
	if _, err := os.Stat(kindConfigPath); err != nil {
		t.Fatalf("expected kind-config.yaml to exist: %v", err)
	}

	mainTFPath := filepath.Join(gitDir, "infrastructure", "clusters", "kind-cluster", "main.tf")
	if _, err := os.Stat(mainTFPath); !os.IsNotExist(err) {
		t.Fatalf("expected main.tf to be absent for kind setup")
	}

	providerTFPath := filepath.Join(gitDir, "infrastructure", "clusters", "kind-cluster", "provider.tf")
	if _, err := os.Stat(providerTFPath); !os.IsNotExist(err) {
		t.Fatalf("expected provider.tf to be absent for kind setup")
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
	cfg := mustNewClusterTestConfig("test-cluster", "kind")
	cfg.OpenCenter.GitOps.Repository.LocalDir = "" // Empty git_dir

	// Save config
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)

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
	cfg := mustNewClusterTestConfig("test-cluster", "kind")
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	cfg.OpenCenter.Infrastructure.Provider = "invalid-provider"

	// Save config
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)

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

	cfg := mustNewClusterTestConfig("test-cluster", "kind")
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir

	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)

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
