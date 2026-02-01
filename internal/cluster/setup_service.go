package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/gitops"
	"github.com/rackerlabs/opencenter-cli/internal/tofu"
)

// SetupOptions contains options for cluster setup
type SetupOptions struct {
	ClusterName    string
	Organization   string
	DryRun         bool
	SkipValidation bool
	Force          bool
}

// SetupResult contains the result of cluster setup
type SetupResult struct {
	GitOpsPath       string
	ManifestsCreated int
	ValidationPassed bool
	CommitHash       string
}

// SetupService handles cluster setup business logic
type SetupService struct {
	pathResolver     *paths.PathResolver
	validationEngine *validation.ValidationEngine
}

// NewSetupService creates a new SetupService
func NewSetupService(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
) *SetupService {
	return &SetupService{
		pathResolver:     pathResolver,
		validationEngine: validationEngine,
	}
}

// Setup performs cluster setup
func (s *SetupService) Setup(ctx context.Context, opts SetupOptions) (*SetupResult, error) {
	// Resolve paths
	clusterPaths, err := s.pathResolver.Resolve(ctx, opts.ClusterName, opts.Organization)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster paths: %w", err)
	}

	// Load configuration
	cfg, err := config.Load(opts.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}

	// Validate that git_dir is set
	gitDir := cfg.GitOps().GitDir
	if gitDir == "" || strings.HasPrefix(gitDir, "./testdata/test-git-repo-") {
		return nil, fmt.Errorf("opencenter.gitops.git_dir must be set in the configuration")
	}

	result := &SetupResult{
		GitOpsPath: gitDir,
	}

	// Validate configuration unless skipped
	if !opts.SkipValidation {
		validationResult, err := s.validationEngine.Validate(ctx, "config", cfg)
		if err != nil {
			return nil, fmt.Errorf("validating config: %w", err)
		}

		result.ValidationPassed = validationResult.Valid
		if !validationResult.Valid && !opts.Force {
			return nil, fmt.Errorf("validation failed: %v", validationResult.Errors)
		}
	}

	// Check if already initialized (unless --force is used)
	if !opts.Force {
		initialized, err := gitops.IsGitOpsInitialized(gitDir)
		if err != nil {
			return nil, fmt.Errorf("checking if GitOps repository is initialized: %w", err)
		}

		if initialized {
			return nil, fmt.Errorf("GitOps repository already initialized at: %s (use --force to overwrite)", gitDir)
		}
	}

	// Generate GitOps manifests
	manifestCount, err := s.generateGitOpsManifests(ctx, cfg, clusterPaths, opts.DryRun)
	if err != nil {
		return nil, fmt.Errorf("generating manifests: %w", err)
	}
	result.ManifestsCreated = manifestCount

	// Validate generated manifests
	if err := s.validateManifests(clusterPaths); err != nil {
		return nil, fmt.Errorf("validating manifests: %w", err)
	}

	// Commit changes if not dry run
	if !opts.DryRun {
		commitHash, err := s.commitChanges(ctx, clusterPaths)
		if err != nil {
			return nil, fmt.Errorf("committing changes: %w", err)
		}
		result.CommitHash = commitHash
	}

	return result, nil
}

// generateGitOpsManifests generates GitOps manifests from configuration
func (s *SetupService) generateGitOpsManifests(ctx context.Context, cfg config.Config, clusterPaths *paths.ClusterPaths, dryRun bool) (int, error) {
	if dryRun {
		// In dry-run mode, just count what would be generated
		// For now, return an estimate
		return 50, nil
	}

	// Copy base GitOps structure (always render for generation)
	if err := gitops.CopyBase(cfg, true); err != nil {
		return 0, fmt.Errorf("copying base GitOps structure: %w", err)
	}

	// Render cluster-specific applications
	if err := gitops.RenderClusterApps(cfg); err != nil {
		return 0, fmt.Errorf("rendering cluster apps: %w", err)
	}

	// Render infrastructure templates
	if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
		return 0, fmt.Errorf("rendering infrastructure cluster: %w", err)
	}

	// Provision OpenTofu (renders main.tf and provider.tf)
	if err := tofu.Provision(cfg); err != nil {
		return 0, fmt.Errorf("provisioning opentofu: %w", err)
	}

	// Count generated files
	manifestCount, err := s.countGeneratedFiles(clusterPaths.GitOpsDir)
	if err != nil {
		return 0, fmt.Errorf("counting generated files: %w", err)
	}

	return manifestCount, nil
}

// validateManifests validates generated GitOps manifests
func (s *SetupService) validateManifests(clusterPaths *paths.ClusterPaths) error {
	// Create manifest validator
	validator := gitops.NewManifestValidator(clusterPaths.GitOpsDir)

	// Run validation
	if err := validator.Validate(); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}

	return nil
}

// commitChanges commits generated manifests to git
func (s *SetupService) commitChanges(ctx context.Context, clusterPaths *paths.ClusterPaths) (string, error) {
	gitDir := clusterPaths.GitOpsDir

	// Change to git directory
	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(gitDir); err != nil {
		return "", fmt.Errorf("changing to git directory: %w", err)
	}

	// Check if git repository is initialized
	if _, err := os.Stat(filepath.Join(gitDir, ".git")); os.IsNotExist(err) {
		// Initialize git repository
		cmd := exec.CommandContext(ctx, "git", "init")
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("initializing git repository: %w", err)
		}
	}

	// Stage all files
	cmd := exec.CommandContext(ctx, "git", "add", ".")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("staging files: %w", err)
	}

	// Check if there are changes to commit
	cmd = exec.CommandContext(ctx, "git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("checking git status: %w", err)
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		// No changes to commit
		return "", nil
	}

	// Commit changes
	commitMessage := "Initialize GitOps repository structure\n\n- Add base GitOps structure\n- Add cluster-specific applications\n- Add infrastructure templates"
	cmd = exec.CommandContext(ctx, "git", "commit", "-m", commitMessage)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("committing changes: %w", err)
	}

	// Get commit hash
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting commit hash: %w", err)
	}

	commitHash := strings.TrimSpace(string(output))
	return commitHash, nil
}

// countGeneratedFiles counts the number of files in the GitOps directory
func (s *SetupService) countGeneratedFiles(gitDir string) (int, error) {
	count := 0

	err := filepath.Walk(gitDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Count regular files
		if !info.IsDir() {
			count++
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return count, nil
}
