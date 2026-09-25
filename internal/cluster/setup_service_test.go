package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
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
	cache := v2.NewConfigCache()
	loader := v2.NewConfigIOHandler(fileSystem)
	configMgr := config.NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	service := NewSetupServiceWithConfigMgr(pathResolver, validator, configMgr)
	service.overlayEncryptor = overlayFileEncryptorFunc(func(context.Context, string, *v2.Config) error {
		return nil
	})
	return service
}

func TestNewSetupService(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := validation.NewValidationEngine()

	service := NewSetupService(pathResolver, validationEngine)

	if service == nil {
		t.Fatal("NewSetupService returned nil")
		return
	}

	if service.pathResolver == nil {
		t.Error("pathResolver is nil")
	}

	if service.validationEngine == nil {
		t.Error("validationEngine is nil")
	}
}

func TestSetupService_generateGitOpsManifests_EncryptsBeforePromotion(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	pathResolver := paths.NewPathResolver(tmpDir)
	if err := pathResolver.CreateClusterDirectories(ctx, "encryption-boundary", "test-org"); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	clusterPaths, err := pathResolver.Resolve(ctx, "encryption-boundary", "test-org")
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}

	cfg := mustNewClusterTestConfig("encryption-boundary", "kind")
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir
	cfg.Secrets.Headlamp.OIDCClientSecret = "fixture-headlamp-secret"

	var calls int
	service := NewSetupService(pathResolver, validation.NewValidationEngine())
	service.overlayEncryptor = overlayFileEncryptorFunc(func(_ context.Context, overlayPath string, gotCfg *v2.Config) error {
		calls++
		sensitiveRel := filepath.Join("services", "headlamp", "helm-values", "override-values.yaml")
		targetPath := filepath.Join(gotCfg.GitDir(), "applications", "overlays", gotCfg.ClusterName(), sensitiveRel)
		if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
			t.Fatalf("target sensitive file exists before live encryption: %v", err)
		}

		files, err := filepath.Glob(filepath.Join(overlayPath, "services", "*", "helm-values", "override-values.yaml"))
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("no generated sensitive override values")
		}
		for _, path := range files {
			plain, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(plain), "fixture-headlamp-secret") {
				if err := os.WriteFile(path, []byte("sops:\n  mac: fixture\nenc: encrypted\n"), 0o644); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if _, err := service.generateGitOpsManifests(ctx, cfg, clusterPaths, false); err != nil {
		t.Fatalf("generateGitOpsManifests() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("live generation encryption calls = %d, want 1", calls)
	}

	sensitivePath := filepath.Join(clusterPaths.GitOpsDir, "applications", "overlays", cfg.ClusterName(), "services", "headlamp", "helm-values", "override-values.yaml")
	data, err := os.ReadFile(sensitivePath)
	if err != nil {
		t.Fatalf("read promoted sensitive override values: %v", err)
	}
	if !strings.Contains(string(data), "sops:") || strings.Contains(string(data), "fixture-headlamp-secret") {
		t.Fatalf("live generation promoted plaintext sensitive values: %q", data)
	}
}

// overlayFileEncryptorFunc adapts a test function to the production seam.
type overlayFileEncryptorFunc func(context.Context, string, *v2.Config) error

func (f overlayFileEncryptorFunc) EncryptServiceOverrideValues(ctx context.Context, overlayPath string, cfg *v2.Config) error {
	return f(ctx, overlayPath, cfg)
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
	service.overlayEncryptor = overlayFileEncryptorFunc(func(context.Context, string, *v2.Config) error { return nil })

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
	makeSetupConfigGenerationReady(&cfg)

	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validation.NewValidationEngine()
	if err := validator.Register(validators.NewConfigValidator()); err != nil {
		t.Fatalf("register config validator: %v", err)
	}
	cache := v2.NewConfigCache()
	loader := v2.NewConfigIOHandler(fileSystem)
	configMgr := config.NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	service := NewSetupServiceWithConfigMgr(pathResolver, validator, configMgr)
	service.overlayEncryptor = overlayFileEncryptorFunc(func(context.Context, string, *v2.Config) error { return nil })
	validatedTofu := false
	service.manifestValidator = func(root string) error {
		for _, name := range []string{"provider.tf", "terraform.tfvars"} {
			path := filepath.Join(root, "infrastructure", "clusters", clusterName, name)
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("staged OpenTofu file %s missing during validation: %w", name, err)
			}
		}
		validatedTofu = true
		return nil
	}
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
	if !validatedTofu {
		t.Fatal("dry-run setup did not validate staged OpenTofu files")
	}
	for _, name := range []string{"provider.tf", "terraform.tfvars"} {
		rel := filepath.ToSlash(filepath.Join("infrastructure", "clusters", clusterName, name))
		if !containsString(result.Promotion.Added, rel) {
			t.Fatalf("dry-run promotion missing staged OpenTofu path %q: %+v", rel, result.Promotion)
		}
		if _, err := os.Stat(filepath.Join(gitDir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote live OpenTofu file %q: %v", rel, err)
		}
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
		return
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
	makeSetupConfigGenerationReady(&cfg)

	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)
	service.overlayEncryptor = overlayFileEncryptorFunc(func(context.Context, string, *v2.Config) error {
		return nil
	})
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

func TestSetupService_Setup_MagnumDoesNotGenerateOpenTofuArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "gitops")
	pathResolver := paths.NewPathResolver(tmpDir)
	ctx := context.Background()
	clusterName := "magnum-cluster"
	organization := "opencenter"

	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("failed to create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "magnum")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	// Keep this enabled deliberately so the test also proves that Magnum's
	// generation path does not materialize OpenTofu based on the config flag.
	cfg.OpenTofu.Enabled = true
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.AuthURL = "https://keystone.example.test/v3"
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.ProjectID = "project-id"
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.ApplicationCredentialID = "application-credential-id"
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.ApplicationCredentialSecret = "application-credential-secret"
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.ClusterTemplate = "kubernetes-template"
	makeSetupConfigGenerationReady(&cfg)
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)
	result, err := service.Setup(ctx, SetupOptions{
		ClusterName:    clusterName,
		Organization:   organization,
		SkipValidation: true,
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if result == nil || result.ManifestsCreated == 0 {
		t.Fatalf("Setup result = %#v, want generated manifests", result)
	}

	clusterInfrastructureDir := filepath.Join(gitDir, "infrastructure", "clusters", clusterName)
	if _, err := os.Stat(clusterInfrastructureDir); !os.IsNotExist(err) {
		t.Fatalf("expected Magnum infrastructure directory to be absent, got %v", err)
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

func TestSetupService_SetupBlocksGenerationValidationBeforeTargetMutation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v2.Config)
	}{
		{
			name: "missing dependency",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig).Enabled = false
			},
		},
		{
			name: "placeholder required secret",
			mutate: func(cfg *v2.Config) {
				cfg.Secrets.Keycloak.AdminPassword = v2.PlaceholderSecret
			},
		},
		{
			name: "static provider incompatibility",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			ctx := context.Background()
			clusterName := "invalid-generation"
			organization := "opencenter"
			target := filepath.Join(tmpDir, "target")
			pathResolver := paths.NewPathResolver(tmpDir)
			if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
				t.Fatalf("create cluster directories: %v", err)
			}

			cfg := generationReadySetupConfig(t, clusterName, target)
			tt.mutate(&cfg)
			testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)
			writeTargetSentinel(t, target)
			before := snapshotTargetTree(t, target)

			service := createTestSetupService(pathResolver)
			service.overlayEncryptor = overlayFileEncryptorFunc(func(context.Context, string, *v2.Config) error {
				t.Fatal("generation reached encryption after failed offline validation")
				return nil
			})

			_, err := service.Setup(ctx, SetupOptions{ClusterName: clusterName, Organization: organization})
			if err == nil || !strings.Contains(err.Error(), "offline generation validation") {
				t.Fatalf("Setup() error = %v, want offline generation validation failure", err)
			}
			assertTargetTreeEquals(t, target, before)
		})
	}
}

func TestSetupService_SetupSkipValidationExplicitlyBypassesGenerationGate(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	clusterName := "skip-generation-validation"
	organization := "opencenter"
	target := filepath.Join(tmpDir, "target")
	pathResolver := paths.NewPathResolver(tmpDir)
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := generationReadySetupConfig(t, clusterName, target)
	cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig).Enabled = false
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)
	service.manifestValidator = func(string) error {
		t.Fatal("explicit --skip-validation bypass still invoked manifest validation")
		return nil
	}
	result, err := service.Setup(ctx, SetupOptions{
		ClusterName:    clusterName,
		Organization:   organization,
		DryRun:         true,
		SkipValidation: true,
	})
	if err != nil {
		t.Fatalf("Setup() with explicit validation bypass error = %v", err)
	}
	if result == nil || result.ManifestsCreated == 0 {
		t.Fatalf("Setup() bypass result = %#v, want generated dry-run plan", result)
	}
}

func TestSetupService_SetupRejectsStagedManifestBeforePromotion(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	clusterName := "invalid-manifest"
	organization := "opencenter"
	target := filepath.Join(tmpDir, "target")
	pathResolver := paths.NewPathResolver(tmpDir)
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := generationReadySetupConfig(t, clusterName, target)
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)
	writeTargetSentinel(t, target)
	before := snapshotTargetTree(t, target)

	service := createTestSetupService(pathResolver)
	encrypted := false
	service.overlayEncryptor = overlayFileEncryptorFunc(func(context.Context, string, *v2.Config) error {
		encrypted = true
		return nil
	})
	service.manifestValidator = func(path string) error {
		if path == target || strings.HasPrefix(path, target+string(os.PathSeparator)) {
			t.Fatalf("manifest validation ran against live target %q", path)
		}
		if !encrypted {
			t.Fatal("manifest validation ran before staged encryption finalized bytes")
		}
		return fmt.Errorf("synthetic invalid manifest")
	}

	_, err := service.Setup(ctx, SetupOptions{ClusterName: clusterName, Organization: organization})
	if err == nil || !strings.Contains(err.Error(), "synthetic invalid manifest") {
		t.Fatalf("Setup() error = %v, want fatal staged manifest validation failure", err)
	}
	assertTargetTreeEquals(t, target, before)
}

func TestSetupService_SetupValidatesStagedManifestOnValidDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	clusterName := "valid-generation"
	organization := "opencenter"
	target := filepath.Join(tmpDir, "target")
	pathResolver := paths.NewPathResolver(tmpDir)
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := generationReadySetupConfig(t, clusterName, target)
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	service := createTestSetupService(pathResolver)
	validated := false
	service.manifestValidator = func(path string) error {
		validated = true
		if path == target {
			t.Fatal("valid generation was validated only after promotion")
		}
		return nil
	}

	result, err := service.Setup(ctx, SetupOptions{ClusterName: clusterName, Organization: organization, DryRun: true})
	if err != nil {
		t.Fatalf("Setup() valid dry-run error = %v", err)
	}
	if !validated {
		t.Fatal("Setup() did not validate staged generated manifests")
	}
	if result == nil || !result.ValidationPassed || result.ManifestsCreated == 0 {
		t.Fatalf("Setup() valid dry-run result = %#v", result)
	}
}

func generationReadySetupConfig(t *testing.T, name, target string) v2.Config {
	t.Helper()
	cfg := mustNewClusterTestConfig(name, "kind")
	cfg.OpenCenter.Meta.Organization = "opencenter"
	cfg.OpenCenter.GitOps.Repository.LocalDir = target
	makeSetupConfigGenerationReady(&cfg)
	return cfg
}

func makeSetupConfigGenerationReady(cfg *v2.Config) {
	cfg.OpenCenter.GitOps.Repository.URL = "https://github.com/example/cluster.git"
	cfg.OpenCenter.GitOps.Auth.SSH = nil
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{Provider: "github", Token: "test-token"}
	cfg.Secrets.Global.AWS.Application.AccessKey = "global-application-access"
	cfg.Secrets.Global.AWS.Application.SecretAccessKey = "global-application-secret"
	cfg.Secrets.Keycloak.ClientSecret = "keycloak-client-secret"
	cfg.Secrets.Keycloak.AdminPassword = "keycloak-admin-password"
	cfg.Secrets.Headlamp.OIDCClientSecret = "headlamp-client-secret"
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "loki-swift-secret"
	cfg.Secrets.Loki.S3AccessKeyID = "loki-s3-access"
	cfg.Secrets.Loki.S3SecretAccessKey = "loki-s3-secret"
	cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = "tempo-swift-secret"
	cfg.Secrets.Tempo.AccessKey = "tempo-s3-access"
	cfg.Secrets.Tempo.SecretKey = "tempo-s3-secret"
	if loki, ok := cfg.OpenCenter.Services["loki"].(*services.LokiConfig); ok {
		loki.S3Endpoint = "https://loki-s3.example"
	}
	if tempo, ok := cfg.OpenCenter.Services["tempo"].(*services.TempoConfig); ok {
		tempo.S3Endpoint = "https://tempo-s3.example"
	}
	if keycloak, ok := cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig); ok {
		keycloak.Instances = 2
	}
	if openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack; openstack != nil {
		openstack.ApplicationCredentialID = "openstack-application-credential-id"
		openstack.ApplicationCredentialSecret = "openstack-application-credential-secret"
	}
}

func writeTargetSentinel(t *testing.T, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(target, "custom"), 0o755); err != nil {
		t.Fatalf("create target sentinel directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "custom", "sentinel.txt"), []byte("unchanged\n"), 0o644); err != nil {
		t.Fatalf("write target sentinel: %v", err)
	}
}

func snapshotTargetTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatalf("snapshot target tree: %v", err)
	}
	return snapshot
}

func assertTargetTreeEquals(t *testing.T, root string, want map[string]string) {
	t.Helper()
	got := snapshotTargetTree(t, root)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("target tree mutated:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFormatDryRunPromotionIncludesPruneCandidates(t *testing.T) {
	lines := formatDryRunPromotion(&gitops.PromoteResult{
		PruneCandidates: []string{"applications/base/stale.yaml", "clusters/demo/old.yaml"},
	})
	output := strings.Join(lines, "\n")
	for _, want := range []string{
		"prune-candidates=2",
		"prune candidate (retained): applications/base/stale.yaml",
		"prune candidate (retained): clusters/demo/old.yaml",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("dry-run promotion output missing %q:\n%s", want, output)
		}
	}
}
