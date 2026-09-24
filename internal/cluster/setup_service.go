package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/tofu"
)

// SetupOptions contains options for cluster setup
type SetupOptions struct {
	ClusterName      string
	Organization     string
	DryRun           bool
	SkipValidation   bool
	Force            bool
	Prune            *bool
	AdoptGenerated   bool
	GitopsAuthMethod string // "ssh" or "token"; when set, overrides BaseRepo URL scheme
}

// SetupResult contains the result of cluster setup
type SetupResult struct {
	GitOpsPath       string
	ManifestsCreated int
	ValidationPassed bool
	Promotion        *gitops.PromoteResult
	Warnings         []string
}

type overlayFileEncryptor interface {
	EncryptServiceOverrideValues(ctx context.Context, overlayPath string, cfg *v2.Config) error
}

// SetupService handles cluster setup business logic
type SetupService struct {
	pathResolver      *paths.PathResolver
	validationEngine  *validation.ValidationEngine
	configurationMgr  *config.ConfigurationManager
	overlayEncryptor  overlayFileEncryptor
	manifestValidator func(string) error
}

// NewSetupService creates a new SetupService
func NewSetupService(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
) *SetupService {
	return NewSetupServiceWithConfigMgr(pathResolver, validationEngine, nil)
}

// NewSetupServiceWithConfigMgr creates a new SetupService with optional ConfigurationManager
func NewSetupServiceWithConfigMgr(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configurationMgr *config.ConfigurationManager,
) *SetupService {
	// Create ConfigurationManager if not provided
	if configurationMgr == nil {
		// Try to create one, but don't fail if it doesn't work
		configurationMgr, _ = config.NewConfigurationManager()
	}

	return &SetupService{
		pathResolver:     pathResolver,
		validationEngine: validationEngine,
		configurationMgr: configurationMgr,
		overlayEncryptor: sops.NewSOPSManager(),
		manifestValidator: func(gitDir string) error {
			return gitops.NewManifestValidator(gitDir).Validate()
		},
	}
}

// Setup performs cluster setup
func (s *SetupService) Setup(ctx context.Context, opts SetupOptions) (*SetupResult, error) {
	// Resolve paths
	clusterPaths, err := s.pathResolver.Resolve(ctx, opts.ClusterName, opts.Organization)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster paths: %w", err)
	}

	// Load configuration using ConfigurationManager
	// Build the full identifier (org/cluster) for config loading when organization is known
	configIdentifier := opts.ClusterName
	if opts.Organization != "" {
		configIdentifier = opts.Organization + "/" + opts.ClusterName
	}

	var cfg v2.Config
	if s.configurationMgr != nil {
		var loadedCfg *v2.Config
		var err error

		// Use LoadWithoutValidation if validation will be skipped anyway
		if opts.SkipValidation {
			loadedCfg, err = s.configurationMgr.LoadWithoutValidation(ctx, configIdentifier)
		} else {
			loadedCfg, err = s.configurationMgr.Load(ctx, configIdentifier)
		}

		if err != nil {
			return nil, fmt.Errorf("loading configuration: %w", err)
		}
		cfg = *loadedCfg
	} else {
		// Fallback: create temporary manager
		tempMgr, err := config.NewConfigurationManager()
		if err != nil {
			return nil, fmt.Errorf("creating configuration manager: %w", err)
		}

		var loadedCfg *v2.Config
		if opts.SkipValidation {
			loadedCfg, err = tempMgr.LoadWithoutValidation(ctx, configIdentifier)
		} else {
			loadedCfg, err = tempMgr.Load(ctx, configIdentifier)
		}

		if err != nil {
			return nil, fmt.Errorf("loading configuration: %w", err)
		}
		cfg = *loadedCfg
	}

	// Check schema version - only v2 is supported
	if cfg.SchemaVersion != "2.0" {
		return nil, fmt.Errorf("invalid schema version for cluster %s: expected 2.0, got %q", opts.ClusterName, cfg.SchemaVersion)
	}

	// Apply the run-only GitOps auth choice to the in-memory config. Setup may
	// also be invoked outside the command layer, so validate it here too.
	if opts.GitopsAuthMethod != "" {
		if err := config.ValidateGitopsAuthMethod(opts.GitopsAuthMethod); err != nil {
			return nil, fmt.Errorf("invalid GitOps auth method: %w", err)
		}
		cfg.OpenCenter.GitOps.ResolvedAuthMethod = opts.GitopsAuthMethod
		switch opts.GitopsAuthMethod {
		case config.GitopsAuthMethodSSH:
			cfg.OpenCenter.GitOps.BaseRepo.URL = v2.DefaultGitBaseRepoURLSSH
		default:
			cfg.OpenCenter.GitOps.BaseRepo.URL = v2.DefaultGitBaseRepoURLHTTPS
		}
	}

	// Validate that git_dir is set
	gitDir := cfg.GitDir()
	if gitDir == "" || strings.HasPrefix(gitDir, "./testdata/test-git-repo-") {
		return nil, fmt.Errorf("opencenter.gitops.git_dir must be set in the configuration")
	}

	result := &SetupResult{
		GitOpsPath: gitDir,
	}

	if !opts.SkipValidation {
		if err := v2.ValidateForGeneration(&cfg); err != nil {
			return nil, fmt.Errorf("validating configuration for generation: %w", err)
		}
		result.ValidationPassed = true
	}

	// Generate GitOps manifests from the same staged tree for dry-run and apply.
	promotion, manifestCount, err := s.generateGitOpsManifestsWithPromotion(ctx, cfg, clusterPaths, opts.DryRun, !opts.SkipValidation, gitops.PromoteOptions{
		Prune:          opts.Prune,
		AdoptGenerated: opts.AdoptGenerated,
	})
	if err != nil {
		return nil, fmt.Errorf("generating manifests: %w", err)
	}
	result.Promotion = promotion
	result.ManifestsCreated = manifestCount
	result.Warnings = append(result.Warnings, formatDryRunPromotion(promotion)...)

	return result, nil
}

func promotionChangeCount(result *gitops.PromoteResult) int {
	if result == nil {
		return 0
	}
	return len(result.Added) + len(result.Updated) + len(result.Renamed)
}

func formatDryRunPromotion(result *gitops.PromoteResult) []string {
	if result == nil {
		return nil
	}
	lines := []string{"promotion summary: " + fmt.Sprintf("added=%d updated=%d unchanged=%d seeded=%d pruned=%d prune-candidates=%d renamed=%d adopted=%d", len(result.Added), len(result.Updated), len(result.Unchanged), len(result.Seeded), len(result.Pruned), len(result.PruneCandidates), len(result.Renamed), len(result.Adopted))}
	for _, category := range []struct {
		name  string
		paths []string
	}{
		{name: "pruned", paths: result.Pruned},
		{name: "prune candidate (retained)", paths: result.PruneCandidates},
		{name: "renamed", paths: result.Renamed},
		{name: "adopted", paths: result.Adopted},
	} {
		for _, path := range category.paths {
			lines = append(lines, fmt.Sprintf("  %s: %s", category.name, path))
		}
	}
	for _, path := range result.BackupPaths {
		lines = append(lines, fmt.Sprintf("  backup: %s", path))
	}
	for _, warning := range result.Warnings {
		lines = append(lines, fmt.Sprintf("promotion warning: %s", warning))
	}
	return lines
}

// generateGitOpsManifests generates GitOps manifests from configuration.
// The legacy count-only shape is retained for focused callers.
func (s *SetupService) generateGitOpsManifests(ctx context.Context, cfg v2.Config, clusterPaths *paths.ClusterPaths, dryRun bool) (int, error) {
	promotion, _, err := s.generateGitOpsManifestsWithPromotion(ctx, cfg, clusterPaths, dryRun, false, gitops.PromoteOptions{})
	if err != nil {
		return 0, err
	}
	return promotionChangeCount(promotion), nil
}

func (s *SetupService) generateGitOpsManifestsWithPromotion(ctx context.Context, cfg v2.Config, clusterPaths *paths.ClusterPaths, dryRun, validateManifests bool, promoteOpts gitops.PromoteOptions) (*gitops.PromoteResult, int, error) {
	generationOptions := gitops.StagedGenerationOptions{
		Encrypt:               s.overlayEncryptor.EncryptServiceOverrideValues,
		IncludeInfrastructure: strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) != "magnum",
		IncludeFluxBridge:     true,
		Promote:               promoteOpts,
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	if provider != "kind" && provider != "magnum" {
		generationOptions.Materialize = func(root string) error {
			return tofu.ProvisionAt(cfg, root)
		}
	}
	generationOptions.Promote.DryRun = dryRun || promoteOpts.DryRun
	if validateManifests {
		generationOptions.ValidateManifest = s.manifestValidator
	}

	promotion, manifestCount, err := gitops.GenerateClusterTree(ctx, cfg, generationOptions)
	if err != nil {
		return nil, 0, err
	}
	return promotion, manifestCount, nil
}

// validateManifests validates generated GitOps manifests.
func (s *SetupService) validateManifests(clusterPaths *paths.ClusterPaths) error {
	if err := s.manifestValidator(clusterPaths.GitOpsDir); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}
	return nil
}

// skippedDirs lists directories that are not generated by opencenter and must
// be excluded from both the pre-generation snapshot and the post-generation
// count. These match the exclusions in the SOPS infrastructure path regex
// (see internal/sops/key_manager.go).
var skippedDirs = map[string]bool{
	".git":       true,
	".terraform": true,
	".bin":       true,
	"venv":       true,
	"kubespray":  true,
}

// snapshotFileModTimes walks gitDir and records the modification time of every
// regular file (excluding non-generated directories such as .git, .terraform,
// venv, kubespray, and .bin). The returned map is keyed by absolute path.
// If gitDir does not exist yet the map is empty (first-time generation).
func (s *SetupService) snapshotFileModTimes(gitDir string) (map[string]int64, error) {
	snapshot := make(map[string]int64)

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return snapshot, nil
	}

	err := filepath.Walk(gitDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (skippedDirs[info.Name()] || strings.HasPrefix(info.Name(), ".opentofu-local")) {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			snapshot[path] = info.ModTime().UnixNano()
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// countGeneratedFiles counts files that were created or modified during this
// generation run by comparing the current state against a pre-generation
// snapshot of modification times. Non-generated directories (.git, .terraform,
// venv, kubespray, .bin) are excluded from the walk.
func (s *SetupService) countGeneratedFiles(gitDir string, before map[string]int64) (int, error) {
	count := 0

	err := filepath.Walk(gitDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && (skippedDirs[info.Name()] || strings.HasPrefix(info.Name(), ".opentofu-local")) {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			prevModTime, existed := before[path]
			if !existed || info.ModTime().UnixNano() != prevModTime {
				count++
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return count, nil
}
