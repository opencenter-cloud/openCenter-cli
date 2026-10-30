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
	"gopkg.in/yaml.v3"
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
	// SyncBaseDomain adopts the settings-file cluster_defaults.base_domain
	// into the cluster config (rewriting the derived cluster_fqdn) before
	// generating. It is the explicit remediation for a pinned cluster whose
	// settings domain changed; without it, a pinned domain is never touched.
	SyncBaseDomain bool
}

// SetupResult contains the result of cluster setup
type SetupResult struct {
	GitOpsPath       string
	ManifestsCreated int
	ValidationPassed bool
	Promotion        *gitops.PromoteResult
	Warnings         []string
	// BaseDomainDrift is the settings-vs-cluster base_domain divergence
	// report for this generation run (zero value = aligned).
	BaseDomainDrift BaseDomainDrift
	// BaseDomainSynced is true when --sync-base-domain rewrote the cluster
	// config on disk during this run.
	BaseDomainSynced bool
	// BaseDomainSyncedFrom/To describe the applied rewrite.
	BaseDomainSyncedFrom string
	BaseDomainSyncedTo   string

	// RepairedHostnameFields lists the service hostname fields loader stage 4c
	// healed while loading the config for this generation run. Surfaced because
	// the sweep changes TLS DNS names on a read path: without a line in the
	// generate output an operator would see hostnames move in the GitOps tree
	// with nothing explaining why.
	RepairedHostnameFields []string
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

	// The on-disk values are the drift baseline: load-time hydration rewrites
	// the in-memory config when the settings file owns base_domain, so the
	// pre-load values must be captured before Load to tell "assets were
	// generated with X" from "settings now say Y".
	persistedBaseDomain, persistedFQDN, persistedStamp := persistedClusterIdentity(s, configIdentifier)

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
		// The loader healed any placeholder-derived hostnames while hydrating
		// this config (stage 4c). Carry them so the command layer can say so.
		RepairedHostnameFields: cfg.RepairedHostnameFields,
	}

	if !opts.SkipValidation {
		if err := v2.ValidateForGeneration(&cfg); err != nil {
			return nil, fmt.Errorf("validating configuration for generation: %w", err)
		}
		result.ValidationPassed = true
	}

	// Settings-vs-cluster base_domain divergence. The loader already applied
	// provenance hydration (managed fields follow the settings file); this is
	// the report and the explicit remediation path for pinned clusters.
	settingsBaseDomain := ""
	if cm, err := config.NewConfigManager(""); err == nil {
		if cliConfig := cm.GetConfig(); cliConfig != nil {
			settingsBaseDomain = strings.ToLower(strings.TrimSpace(cliConfig.ClusterDefaults.BaseDomain))
		}
	}
	if settingsBaseDomain == "" {
		settingsBaseDomain = v2.ReadSettingsBaseDomain()
	}

	// A managed field propagated by load-time hydration must land on disk at
	// the same moment the assets that render it are generated; otherwise the
	// on-disk config disagrees with the GitOps tree. Persist through a
	// detached copy: the in-memory cfg carries the run-only GitOps auth
	// override, which must never leak into the cluster config file.
	persistConfig := func() error {
		if s.configurationMgr == nil {
			return fmt.Errorf("persisting the cluster config requires a configuration manager")
		}
		persisted := cfg
		persisted.OpenCenter.GitOps.ResolvedAuthMethod = ""
		// Stamp the rendered domain: the assets written by this run carry
		// this value, and the stamp is the drift baseline for the next run.
		persisted.OpenCenter.Cluster.LastGeneratedBaseDomain = strings.ToLower(
			strings.TrimSpace(cfg.OpenCenter.Cluster.BaseDomain))
		if err := s.configurationMgr.Save(ctx, &persisted); err != nil {
			return err
		}
		s.configurationMgr.InvalidateCluster(ctx, configIdentifier)
		return nil
	}

	// Detect against the persisted config, not the hydrated in-memory one:
	// the loader propagates a managed field before this point, so the file
	// still holds what the last successful generation rendered with. The
	// report then reads "assets were generated with X, settings now say Y".
	persisted := cfg
	if persistedBaseDomain != "" {
		persisted.OpenCenter.Cluster.BaseDomain = persistedBaseDomain
		persisted.OpenCenter.Cluster.ClusterFQDN = persistedFQDN
	}
	persisted.OpenCenter.Cluster.LastGeneratedBaseDomain = persistedStamp
	result.BaseDomainDrift = DetectBaseDomainDrift(&persisted, settingsBaseDomain)
	result.BaseDomainDrift.RenderedBaseDomain = strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Cluster.BaseDomain))

	// A managed field propagated by load-time hydration must land on disk at
	// the same moment the assets that render it are generated; otherwise the
	// on-disk config disagrees with the GitOps tree.
	if result.BaseDomainDrift.Class == BaseDomainFollowsSettings && !opts.DryRun {
		if err := persistConfig(); err != nil {
			return nil, fmt.Errorf("persisting settings-propagated base_domain: %w", err)
		}
	}

	if opts.SyncBaseDomain {
		if settingsBaseDomain == "" {
			return nil, fmt.Errorf("--sync-base-domain requires cluster_defaults.base_domain to be set in the CLI settings file")
		}
		from := cfg.OpenCenter.Cluster.BaseDomain
		changed := v2.SyncBaseDomainFromSettings(&cfg, settingsBaseDomain)
		if changed && !opts.DryRun {
			if err := persistConfig(); err != nil {
				return nil, fmt.Errorf("saving synced base_domain: %w", err)
			}
			result.BaseDomainSynced = true
			result.BaseDomainSyncedFrom = from
			result.BaseDomainSyncedTo = cfg.OpenCenter.Cluster.BaseDomain
		}
	} else if warning := result.BaseDomainDrift.Warning(); warning != "" {
		result.Warnings = append(result.Warnings, warning)
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

// persistedClusterIdentity reads base_domain, cluster_fqdn, and
// last_generated_base_domain straight from the cluster config file, bypassing
// the manager cache (which holds the already-hydrated value). Returns empty
// strings when the file is unreadable.
func persistedClusterIdentity(s *SetupService, configIdentifier string) (baseDomain, fqdn, stamp string) {
	if s == nil || s.pathResolver == nil {
		return "", "", ""
	}
	clusterName := configIdentifier
	organization := ""
	if idx := strings.Index(configIdentifier, "/"); idx >= 0 {
		organization = configIdentifier[:idx]
		clusterName = configIdentifier[idx+1:]
	}
	clusterPaths, err := s.pathResolver.Resolve(context.Background(), clusterName, organization)
	if err != nil {
		return "", "", ""
	}
	data, err := os.ReadFile(clusterPaths.ConfigPath)
	if err != nil {
		return "", "", ""
	}
	var probe struct {
		OpenCenter struct {
			Cluster struct {
				BaseDomain              string `yaml:"base_domain"`
				ClusterFQDN             string `yaml:"cluster_fqdn"`
				LastGeneratedBaseDomain string `yaml:"last_generated_base_domain"`
			} `yaml:"cluster"`
		} `yaml:"opencenter"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return "", "", ""
	}
	return strings.TrimSpace(probe.OpenCenter.Cluster.BaseDomain),
		strings.TrimSpace(probe.OpenCenter.Cluster.ClusterFQDN),
		strings.TrimSpace(probe.OpenCenter.Cluster.LastGeneratedBaseDomain)
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
		IncludeFluxBridge:     cfg.ShouldRenderFluxBridge(),
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
