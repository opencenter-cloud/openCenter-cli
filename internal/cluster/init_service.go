package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	configdefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	configflags "github.com/opencenter-cloud/opencenter-cli/internal/config/flags"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"gopkg.in/yaml.v3"
)

// InitOptions contains options for cluster initialization
type InitOptions struct {
	ClusterName  string
	Organization string
	// OrganizationExplicit is true when the caller explicitly provided an
	// organization (via --org, an embedded "org/cluster" name, or the
	// deprecated org flag), as opposed to Organization merely holding a
	// resolved default. Only an explicit organization may override a value
	// already present in a loaded --config-file.
	OrganizationExplicit bool
	Region               string
	RegionExplicit       bool
	Provider             string
	DeploymentMethod     string
	ConfigFile           string
	ConfigMap            map[string]any
	Force                bool
	Strict               bool
	NoKeyGen             bool
	NoSOPSKeyGen         bool
	RegenerateKeys       bool
	NoGitInit            bool
	FullSchema           bool
	SchemaVersion        string
	ServerPools          []string
	FlagOverrides        []string
	// BaseDomain is the normalized --base-domain value. When set, the
	// base_domain override (which parseInitOptions injects into
	// FlagOverrides) is tagged managed.base_domain: default so the cluster
	// keeps following cluster_defaults.base_domain from the settings file.
	BaseDomain            string
	KindDisableDefaultCNI *bool
	// DryRun performs every read, validation and path-resolution step and
	// reports what would be created, but writes nothing: no directories, no
	// config file, no SOPS/SSH keys, no git repository, and no remote
	// application credential. The global --dry-run flag maps here.
	DryRun bool
}

// InitResult contains the result of cluster initialization
type InitResult struct {
	ConfigPath     string
	ClusterPaths   *paths.ClusterPaths
	Config         *v2.Config
	ConfigMap      map[string]any
	KeysGenerated  bool
	GitInitialized bool
	Message        string
}

// InitService handles cluster initialization business logic
type InitService struct {
	pathResolver     *paths.PathResolver
	validationEngine *validation.ValidationEngine
	configManager    *config.ConfigManager
	configurationMgr *config.ConfigurationManager
	fileSystem       fs.FileSystem
}

// NewInitService creates a new InitService
func NewInitService(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configManager *config.ConfigManager,
) *InitService {
	return NewInitServiceWithConfigMgr(pathResolver, validationEngine, configManager, nil, nil)
}

// NewInitServiceWithConfigMgr creates a new InitService with optional ConfigurationManager
func NewInitServiceWithConfigMgr(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configManager *config.ConfigManager,
	configurationMgr *config.ConfigurationManager,
	fileSystem fs.FileSystem,
) *InitService {
	// Create ConfigurationManager if not provided
	if configurationMgr == nil {
		// Create ConfigurationManager with the provided validation engine
		errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
		fs := fs.NewDefaultFileSystem(errorHandler)

		// Use the pathResolver's base directory
		configurationMgr = config.NewConfigurationManagerWithDeps(
			v2.NewConfigIOHandler(fs),
			validationEngine, // Use the validation engine from DI
			v2.NewConfigCache(),
			pathResolver,
			fs,
		)
	}

	// Create FileSystem if not provided
	if fileSystem == nil {
		errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
		fileSystem = fs.NewDefaultFileSystem(errorHandler)
	}

	return &InitService{
		pathResolver:     pathResolver,
		validationEngine: validationEngine,
		configManager:    configManager,
		configurationMgr: configurationMgr,
		fileSystem:       fileSystem,
	}
}

// Initialize performs cluster initialization
func (s *InitService) Initialize(ctx context.Context, opts InitOptions) (*InitResult, error) {
	// Validate cluster name
	if err := s.validateClusterName(ctx, opts.ClusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// Validate organization
	if opts.Organization == "" {
		if s.configManager != nil {
			if cliConfig := s.configManager.GetConfig(); cliConfig != nil && cliConfig.ClusterDefaults.Organization != "" {
				opts.Organization = cliConfig.ClusterDefaults.Organization
			}
		}
	}
	if opts.Organization == "" {
		opts.Organization = "opencenter"
	}
	if err := s.validateOrganization(ctx, opts.Organization); err != nil {
		return nil, fmt.Errorf("invalid organization: %w", err)
	}

	// Use PathResolver to resolve paths for the cluster
	// For new clusters, this will use the organization-based strategy
	strategy := s.pathResolver.GetStrategies()[0]
	clusterPaths, err := strategy.Resolve(ctx, opts.ClusterName, opts.Organization)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster paths: %w", err)
	}

	// Check if cluster exists and handle force flag
	if err := s.checkExistingCluster(clusterPaths, opts.ClusterName, opts.Organization, opts.Force); err != nil {
		return nil, err
	}

	// Load or create configuration
	cfg, configMap, err := s.loadOrCreateConfig(opts)
	if err != nil {
		return nil, fmt.Errorf("loading/creating config: %w", err)
	}

	// Apply configuration overrides
	if err := s.applyOverrides(cfg, configMap, opts); err != nil {
		return nil, fmt.Errorf("applying overrides: %w", err)
	}

	// Generate enabled-service credentials only during actual initialization.
	if err := s.generateHarborSecrets(cfg, configMap); err != nil {
		return nil, fmt.Errorf("generating Harbor secrets: %w", err)
	}
	if err := s.generateKeycloakSecrets(cfg, configMap); err != nil {
		return nil, fmt.Errorf("generating Keycloak secrets: %w", err)
	}

	// Update configuration with resolved paths
	s.updateConfigPaths(cfg, configMap, clusterPaths, opts)

	// Validate configuration if strict mode
	if opts.Strict {
		if err := s.validateConfig(cfg); err != nil {
			return nil, fmt.Errorf("config validation failed: %w", err)
		}
	}

	// Create directory structure. A dry run stops here: everything above is
	// read-only and already proved the request is valid, and the remaining
	// steps are the ones that leave state behind.
	if opts.DryRun {
		dryResult := &InitResult{
			ConfigPath:     clusterPaths.ConfigPath,
			ClusterPaths:   clusterPaths,
			Config:         cfg,
			ConfigMap:      configMap,
			KeysGenerated:  !opts.NoKeyGen,
			GitInitialized: !opts.NoGitInit,
		}
		dryResult.Message = s.buildDryRunMessage(clusterPaths, opts.Organization, cfg.OpenCenter.GitOps.Repository.LocalDir, opts)
		return dryResult, nil
	}

	if err := s.createDirectories(ctx, clusterPaths, opts.Organization); err != nil {
		return nil, fmt.Errorf("creating directories: %w", err)
	}

	// Initialize result
	result := &InitResult{
		ConfigPath:   clusterPaths.ConfigPath,
		ClusterPaths: clusterPaths,
		Config:       cfg,
		ConfigMap:    configMap,
	}

	// Generate keys if requested
	if !opts.NoKeyGen {
		keysGenerated, err := s.generateKeys(clusterPaths, cfg, opts)
		if err != nil {
			return nil, fmt.Errorf("generating keys: %w", err)
		}
		result.KeysGenerated = keysGenerated
	}

	// Save configuration
	if err := s.saveConfig(ctx, cfg, clusterPaths.ConfigPath, opts.FullSchema); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	// Initialize git repository if requested
	if !opts.NoGitInit {
		if err := s.initGitRepo(clusterPaths); err != nil {
			return nil, fmt.Errorf("initializing git repo: %w", err)
		}
		result.GitInitialized = true
	}

	// Build result message
	result.Message = s.buildResultMessage(clusterPaths, opts.Organization, cfg.OpenCenter.GitOps.Repository.LocalDir, result.KeysGenerated, opts)

	return result, nil
}

// validateClusterName validates the cluster name using the validation engine
func (s *InitService) validateClusterName(ctx context.Context, name string) error {
	result, err := s.validationEngine.Validate(ctx, "cluster-name", name)
	if err != nil {
		return err
	}

	if !result.Valid {
		return fmt.Errorf("validation failed: %v", result.Errors)
	}

	return nil
}

// validateOrganization validates the organization name
func (s *InitService) validateOrganization(ctx context.Context, organization string) error {
	result, err := s.validationEngine.Validate(ctx, "organization-name", organization)
	if err != nil {
		return err
	}

	if !result.Valid {
		return fmt.Errorf("validation failed: %v", result.Errors)
	}

	return nil
}

func (s *InitService) generateHarborSecrets(cfg *v2.Config, configMap map[string]any) error {
	harbor, ok := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
	if !ok || !harbor.IsEnabled() {
		return nil
	}

	candidate := cfg.Secrets.Harbor
	keyManager := sops.NewKeyManager("")
	used := map[string]struct{}{}
	generate := func(current *string) error {
		if !isMissingHarborSecret(*current) {
			used[*current] = struct{}{}
			return nil
		}
		for {
			value, err := keyManager.GenerateRandomPassword(32)
			if err != nil {
				return err
			}
			if _, exists := used[value]; exists {
				continue
			}
			*current = value
			used[value] = struct{}{}
			return nil
		}
	}

	if err := generate(&candidate.AdminPassword); err != nil {
		return err
	}
	if err := generate(&candidate.RegistryPassword); err != nil {
		return err
	}
	if err := generate(&candidate.DatabasePassword); err != nil {
		return err
	}

	cfg.Secrets.Harbor = candidate
	setNestedConfigValue(configMap, candidate.AdminPassword, "secrets", "harbor", "admin_password")
	setNestedConfigValue(configMap, candidate.RegistryPassword, "secrets", "harbor", "registry_password")
	setNestedConfigValue(configMap, candidate.DatabasePassword, "secrets", "harbor", "database_password")
	return nil
}

func (s *InitService) generateKeycloakSecrets(cfg *v2.Config, configMap map[string]any) error {
	keycloak, ok := cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig)
	keycloakEnabled := ok && keycloak.IsEnabled()
	oidcKeycloak := cfg.OpenCenter.Identity.OIDC.Enabled &&
		strings.EqualFold(strings.TrimSpace(cfg.OpenCenter.Identity.OIDC.Provider), v2.OIDCProviderKeycloak)

	if !keycloakEnabled && !oidcKeycloak {
		return nil
	}

	keyManager := sops.NewKeyManager("")

	// Generate the `opencenter` OIDC client secret if missing.
	if isMissingSecret(cfg.Secrets.Keycloak.ClientSecret) {
		clientSecret, err := keyManager.GenerateRandomPassword(32)
		if err != nil {
			return fmt.Errorf("generating Keycloak client secret: %w", err)
		}
		cfg.Secrets.Keycloak.ClientSecret = clientSecret
		setNestedConfigValue(configMap, clientSecret, "secrets", "keycloak", "client_secret")
	}

	// Generate the dedicated `grafana-oauth` client secret if missing. It must
	// not reuse ClientSecret: Keycloak authenticates the token exchange against
	// the secret of the client named in client_id.
	if isMissingSecret(cfg.Secrets.Keycloak.GrafanaClientSecret) {
		grafanaSecret, err := keyManager.GenerateRandomPassword(32)
		if err != nil {
			return fmt.Errorf("generating Keycloak grafana-oauth client secret: %w", err)
		}
		cfg.Secrets.Keycloak.GrafanaClientSecret = grafanaSecret
		setNestedConfigValue(configMap, grafanaSecret, "secrets", "keycloak", "grafana_client_secret")
	}

	// Propagate client secret to Headlamp if Headlamp secret is missing
	if isMissingSecret(cfg.Secrets.Headlamp.OIDCClientSecret) {
		cfg.Secrets.Headlamp.OIDCClientSecret = cfg.Secrets.Keycloak.ClientSecret
		setNestedConfigValue(configMap, cfg.Secrets.Keycloak.ClientSecret, "secrets", "headlamp", "oidc_client_secret")
	}

	return nil
}

func isMissingSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.EqualFold(trimmed, v2.PlaceholderSecret)
}

func isMissingHarborSecret(value string) bool {
	return isMissingSecret(value)
}

// checkExistingCluster checks if cluster already exists and handles force flag
func (s *InitService) checkExistingCluster(clusterPaths *paths.ClusterPaths, clusterName, organization string, force bool) error {
	if _, err := os.Stat(clusterPaths.ConfigPath); err == nil {
		if !force {
			return fmt.Errorf("cluster '%s' already exists in organization '%s' at %s, use --force to overwrite", clusterName, organization, clusterPaths.ConfigPath)
		}
		// If force is true, we'll overwrite the config but preserve keys
	}
	return nil
}

// loadOrCreateConfig loads configuration from file or creates default
func (s *InitService) loadOrCreateConfig(opts InitOptions) (*v2.Config, map[string]any, error) {
	cfg := &v2.Config{}
	var configMap map[string]any

	if opts.ConfigFile != "" {
		// Load from file using the same strict public decode contract as ConfigLoader.
		data, err := s.fileSystem.ReadFile(opts.ConfigFile)
		if err != nil {
			return nil, nil, fmt.Errorf("reading config file: %w", err)
		}

		cfg, err = v2.DecodePublicConfig(data)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing config file: %w", err)
		}
		sanitized, err := v2.SanitizePublicYAML(data)
		if err != nil {
			return nil, nil, fmt.Errorf("sanitizing config file: %w", err)
		}
		configMap = make(map[string]any)
		if err := yaml.Unmarshal(sanitized, &configMap); err != nil {
			return nil, nil, fmt.Errorf("parsing config file to map: %w", err)
		}
	} else if opts.ConfigMap != nil {
		// Use provided config map while retaining its pointer for init merges.
		configMap = opts.ConfigMap
		data, err := yaml.Marshal(configMap)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling config map: %w", err)
		}
		cfg, err = v2.DecodePublicConfig(data)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing config map: %w", err)
		}
		data, err = v2.SanitizePublicYAML(data)
		if err != nil {
			return nil, nil, fmt.Errorf("sanitizing config map: %w", err)
		}
		sanitizedMap := make(map[string]any)
		if err := yaml.Unmarshal(data, &sanitizedMap); err != nil {
			return nil, nil, fmt.Errorf("parsing sanitized config map: %w", err)
		}
		for key := range configMap {
			delete(configMap, key)
		}
		for key, value := range sanitizedMap {
			configMap[key] = value
		}
	} else {
		// Create default configuration
		var err error
		cfg, configMap, err = s.createDefaultConfig(opts)
		if err != nil {
			return nil, nil, fmt.Errorf("creating default config: %w", err)
		}
	}

	return cfg, configMap, nil
}

// createDefaultConfig creates a default configuration based on options
func (s *InitService) createDefaultConfig(opts InitOptions) (*v2.Config, map[string]any, error) {
	var (
		cfg *v2.Config
		err error
	)

	if opts.FullSchema {
		cfg, err = v2.NewV2FullTemplateWithRegion(opts.ClusterName, opts.Provider, opts.Region)
		if err != nil {
			return nil, nil, fmt.Errorf("building v2 full template: %w", err)
		}
	} else {
		cfg, err = v2.NewV2DefaultWithRegion(opts.ClusterName, opts.Provider, opts.Region)
		if err != nil {
			return nil, nil, fmt.Errorf("building v2 defaults: %w", err)
		}
	}

	if opts.Organization != "" {
		cfg.OpenCenter.Meta.Organization = opts.Organization
	}
	if cfg.OpenCenter.Meta.Organization == "" {
		cfg.OpenCenter.Meta.Organization = "opencenter"
	}
	if opts.Provider != "" {
		cfg.OpenCenter.Infrastructure.Provider = opts.Provider
	}

	data, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling default v2 config: %w", err)
	}

	configMap := make(map[string]any)
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return nil, nil, fmt.Errorf("rebuilding default v2 config map: %w", err)
	}

	return cfg, configMap, nil
}

// applyOverrides applies configuration overrides from options
func (s *InitService) applyOverrides(cfg *v2.Config, configMap map[string]any, opts InitOptions) error {
	// Apply organization. Only an explicitly-provided organization (--org, an
	// embedded "org/cluster" name, or the deprecated org flag) may override a
	// value already parsed from --config-file; a merely-resolved default must
	// not clobber it.
	if opts.OrganizationExplicit {
		cfg.OpenCenter.Meta.Organization = opts.Organization
		setNestedConfigValue(configMap, opts.Organization, "opencenter", "meta", "organization")
	} else if cfg.OpenCenter.Meta.Organization == "" {
		// Nothing explicit and the config file left it unset: fall back to
		// the resolved default rather than leaving it blank.
		cfg.OpenCenter.Meta.Organization = opts.Organization
		setNestedConfigValue(configMap, opts.Organization, "opencenter", "meta", "organization")
	}

	// Apply provider
	if opts.Provider != "" {
		cfg.OpenCenter.Infrastructure.Provider = opts.Provider
		setNestedConfigValue(configMap, opts.Provider, "opencenter", "infrastructure", "provider")
	}

	// Apply CLI config defaults
	cliConfig := s.configManager.GetConfig()
	if opts.RegionExplicit {
		cfg.OpenCenter.Meta.Region = opts.Region
		setNestedConfigValue(configMap, opts.Region, "opencenter", "meta", "region")
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
			cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = opts.Region
			setNestedConfigValue(configMap, opts.Region, "opencenter", "infrastructure", "cloud", "openstack", "region")
		}
		if cfg.OpenCenter.Infrastructure.Cloud.Magnum != nil {
			cfg.OpenCenter.Infrastructure.Cloud.Magnum.Region = opts.Region
			setNestedConfigValue(configMap, opts.Region, "opencenter", "infrastructure", "cloud", "magnum", "region")
		}
	} else if cfg.OpenCenter.Meta.Region == "" {
		if cliConfig.ClusterDefaults.Region != "" {
			cfg.OpenCenter.Meta.Region = cliConfig.ClusterDefaults.Region
			setNestedConfigValue(configMap, cliConfig.ClusterDefaults.Region, "opencenter", "meta", "region")
		}
	}
	if cfg.OpenCenter.Meta.Env == "" || cfg.OpenCenter.Meta.Env == "dev" {
		if cliConfig.ClusterDefaults.Environment != "" {
			cfg.OpenCenter.Meta.Env = cliConfig.ClusterDefaults.Environment
			setNestedConfigValue(configMap, cliConfig.ClusterDefaults.Environment, "opencenter", "meta", "env")
		}
	}

	if len(opts.FlagOverrides) > 0 {
		integration, err := configflags.NewCLIIntegration()
		if err != nil {
			return fmt.Errorf("creating flag integration: %w", err)
		}
		if err := integration.ProcessFlags(opts.FlagOverrides, cfg, configMap); err != nil {
			return fmt.Errorf("applying dotted overrides: %w", err)
		}
	}

	// --base-domain: the dotted override above wrote base_domain into both the
	// struct and the config map. Tag it settings-owned (managed.default) so
	// future cluster_defaults.base_domain changes keep propagating at load --
	// seeding a cluster is not freezing it (a deliberate freeze is 'cluster
	// set', which pins), and re-derive cluster_fqdn so the identity fields
	// agree on disk.
	if opts.BaseDomain != "" {
		cfg.OpenCenter.Cluster.Managed.DefaultBaseDomain()
		setNestedConfigValue(configMap, v2.ProvenanceDefault, "opencenter", "cluster", "managed", "base_domain")
		if derived := cfg.OpenCenter.Cluster.DeriveClusterFQDN(cfg.OpenCenter.Meta.Region); derived != "" {
			cfg.OpenCenter.Cluster.ClusterFQDN = derived
			setNestedConfigValue(configMap, derived, "opencenter", "cluster", "cluster_fqdn")
		}
	}

	if opts.KindDisableDefaultCNI != nil {
		if !strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "kind") {
			return fmt.Errorf("--kind-disable-default-cni is only valid for kind clusters")
		}
		if cfg.OpenCenter.Infrastructure.Kind == nil {
			cfg.OpenCenter.Infrastructure.Kind = &v2.KindCompatibilityConfig{}
		}
		cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI = *opts.KindDisableDefaultCNI
		setNestedConfigValue(configMap, *opts.KindDisableDefaultCNI, "opencenter", "infrastructure", "kind", "disable_default_cni")

		// Kind defaults to kindnet with Calico disabled. When the operator opts
		// into managed CNI by disabling the default CNI, enable Calico so the
		// cluster still has a CNI. Leaving the default CNI in place keeps Calico
		// disabled (kindnet provides networking).
		if cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico != nil {
			cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = *opts.KindDisableDefaultCNI
			setNestedConfigValue(configMap, *opts.KindDisableDefaultCNI, "opencenter", "cluster", "kubernetes", "network_plugin", "calico", "enabled")
		}
	}

	return nil
}

// updateConfigPaths updates configuration with resolved paths
func (s *InitService) updateConfigPaths(cfg *v2.Config, configMap map[string]any, clusterPaths *paths.ClusterPaths, opts InitOptions) {
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir
	setNestedConfigValue(configMap, clusterPaths.GitOpsDir, "opencenter", "gitops", "repository", "local_dir")

	// Update SSH key paths (used by non-Kind providers for Git SSH auth).
	sshKeyPath := clusterPaths.SSHKeyPath

	cfg.OpenCenter.Infrastructure.SSH.KeyPath = sshKeyPath
	cfg.Secrets.SSHKey.Private = sshKeyPath
	cfg.Secrets.SSHKey.Public = sshKeyPath + ".pub"
	setNestedConfigValue(configMap, sshKeyPath, "opencenter", "infrastructure", "ssh", "key_path")
	setNestedConfigValue(configMap, sshKeyPath, "secrets", "ssh_key", "private")
	setNestedConfigValue(configMap, sshKeyPath+".pub", "secrets", "ssh_key", "public")

	authMethod := s.effectiveGitopsAuthMethod()
	if hasExplicitConfigValue(configMap, opts, "opencenter", "gitops", "auth", "ssh") {
		authMethod = config.GitopsAuthMethodSSH
	}
	if hasExplicitConfigValue(configMap, opts, "opencenter", "gitops", "auth", "token") {
		authMethod = config.GitopsAuthMethodToken
	}

	if authMethod == config.GitopsAuthMethodSSH {
		if !hasExplicitConfigValue(configMap, opts, "opencenter", "gitops", "repository", "url") {
			cfg.OpenCenter.GitOps.Repository.URL = "ssh://git@example.com/opencenter/cluster-config.git"
			setNestedConfigValue(configMap, cfg.OpenCenter.GitOps.Repository.URL, "opencenter", "gitops", "repository", "url")
		}
		cfg.OpenCenter.GitOps.Auth.SSH = &v2.GitOpsSSHAuth{
			PrivateKey: sshKeyPath,
			PublicKey:  sshKeyPath + ".pub",
		}
		cfg.OpenCenter.GitOps.Auth.Token = nil
		setNestedConfigValue(configMap, sshKeyPath, "opencenter", "gitops", "auth", "ssh", "private_key")
		setNestedConfigValue(configMap, sshKeyPath+".pub", "opencenter", "gitops", "auth", "ssh", "public_key")
		setNestedConfigValue(configMap, nil, "opencenter", "gitops", "auth", "token")
	} else {
		if !hasExplicitConfigValue(configMap, opts, "opencenter", "gitops", "repository", "url") {
			cfg.OpenCenter.GitOps.Repository.URL = "https://github.com/opencenter/cluster-config.git"
			setNestedConfigValue(configMap, cfg.OpenCenter.GitOps.Repository.URL, "opencenter", "gitops", "repository", "url")
		}
		if cfg.OpenCenter.GitOps.Auth.Token == nil {
			cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{}
		}
		if !hasExplicitConfigValue(configMap, opts, "opencenter", "gitops", "auth", "token", "provider") {
			// Preserve a provider already chosen by per-provider behavior
			// defaults (e.g. kind defaults to gitea). Only fall back to github
			// when no provider has been set, so external providers stay opt-in
			// per infrastructure type rather than being forced to github here.
			provider := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Provider)
			if provider == "" {
				provider = "github"
			}
			cfg.OpenCenter.GitOps.Auth.Token.Provider = provider
			setNestedConfigValue(configMap, provider, "opencenter", "gitops", "auth", "token", "provider")
		}
		tokenExplicit := hasExplicitConfigValue(configMap, opts, "opencenter", "gitops", "auth", "token", "token")
		tokenFileExplicit := hasExplicitConfigValue(configMap, opts, "opencenter", "gitops", "auth", "token", "token_file")
		if !tokenExplicit && tokenFileExplicit {
			cfg.OpenCenter.GitOps.Auth.Token.Token = ""
			setNestedConfigValue(configMap, "", "opencenter", "gitops", "auth", "token", "token")
		}
		if !tokenExplicit && !tokenFileExplicit {
			cfg.OpenCenter.GitOps.Auth.Token.Token = v2.PlaceholderSecret
			setNestedConfigValue(configMap, v2.PlaceholderSecret, "opencenter", "gitops", "auth", "token", "token")
		}
		cfg.OpenCenter.GitOps.Auth.SSH = nil
		setNestedConfigValue(configMap, nil, "opencenter", "gitops", "auth", "ssh")
	}

	// Update SOPS key path
	if opts.NoSOPSKeyGen &&
		!hasExplicitConfigValue(configMap, opts, "secrets", "sops_age_key_file") &&
		!hasExplicitConfigValue(configMap, opts, "secrets", "sops", "age_key_file") {
		cfg.Secrets.SopsAgeKeyFile = ""
		cfg.Secrets.SOPSConfig.Enabled = false
		cfg.Secrets.SOPSConfig.AgeKeyFile = ""
		setNestedConfigValue(configMap, "", "secrets", "sops_age_key_file")
		setNestedConfigValue(configMap, false, "secrets", "sops", "enabled")
		setNestedConfigValue(configMap, "", "secrets", "sops", "age_key_file")
		return
	}

	cfg.Secrets.SopsAgeKeyFile = clusterPaths.SOPSKeyPath
	cfg.Secrets.SOPSConfig.Enabled = true
	cfg.Secrets.SOPSConfig.AgeKeyFile = clusterPaths.SOPSKeyPath
	setNestedConfigValue(configMap, clusterPaths.SOPSKeyPath, "secrets", "sops_age_key_file")
	setNestedConfigValue(configMap, clusterPaths.SOPSKeyPath, "secrets", "sops", "age_key_file")
}

func (s *InitService) effectiveGitopsAuthMethod() string {
	if s != nil && s.configManager != nil {
		method := strings.ToLower(strings.TrimSpace(s.configManager.GetConfig().ClusterDefaults.GitopsAuthMethod))
		if method == config.GitopsAuthMethodSSH || method == config.GitopsAuthMethodToken {
			return method
		}
	}
	return config.GitopsAuthMethodToken
}

// validateConfig validates the configuration
func (s *InitService) validateConfig(cfg *v2.Config) error {
	data, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		return fmt.Errorf("marshal v2 config for validation: %w", err)
	}

	if _, err := s.v2Loader().LoadFromBytes(data); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// createDirectories creates the necessary directory structure using PathResolver
func (s *InitService) createDirectories(ctx context.Context, clusterPaths *paths.ClusterPaths, organization string) error {
	// Extract cluster name from the cluster directory path
	clusterName := filepath.Base(clusterPaths.ClusterDir)

	// Use PathResolver to create all necessary directories
	if err := s.pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		return fmt.Errorf("creating cluster directories: %w", err)
	}

	for _, secureDir := range []string{
		clusterPaths.SecretsDir,
		filepath.Join(clusterPaths.SecretsDir, "kubernetes"),
		filepath.Dir(clusterPaths.KubeconfigPath),
		filepath.Join(clusterPaths.SecretsDir, "age"),
		filepath.Dir(clusterPaths.SOPSKeyPath),
		filepath.Join(clusterPaths.SecretsDir, "ssh"),
		filepath.Dir(clusterPaths.SSHKeyPath),
	} {
		if err := ensureMode(secureDir, 0o700); err != nil {
			return err
		}
	}

	return clusterPaths.Validate()
}

// saveConfig saves the configuration to disk using ConfigurationManager
func (s *InitService) saveConfig(ctx context.Context, cfg *v2.Config, configPath string, fullSchema bool) error {
	_ = ctx
	cfg.Metadata.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	// Mint the generation identifier at creation, where it belongs: 'cluster
	// init' is the only operation that begins a generation, so a recreate after
	// destroy always gets a fresh one. The bootstrap resume state is stored under
	// this value, which is what keeps a new generation from resuming the dead
	// generation's completed steps.
	v2.EnsureGenerationID(cfg)

	loader := s.v2Loader()
	if fullSchema {
		data, err := v2.RenderFullTemplateYAMLFromConfig(cfg)
		if err != nil {
			return fmt.Errorf("rendering v2 full template: %w", err)
		}
		data, err = v2.SanitizePublicYAML(data)
		if err != nil {
			return fmt.Errorf("sanitizing rendered v2 full template: %w", err)
		}
		if _, err := loader.LoadFromBytes(data); err != nil {
			return fmt.Errorf("validating rendered v2 full template: %w", err)
		}
		if err := s.fileSystem.WriteFileAtomic(configPath, data, 0o600); err != nil {
			return fmt.Errorf("writing config file: %w", err)
		}
		return verifyMode(configPath, 0o600)
	}

	if err := loader.SaveToFile(cfg, configPath); err != nil {
		return fmt.Errorf("saving v2 config: %w", err)
	}

	return verifyMode(configPath, 0o600)
}

// buildResultMessage builds a user-friendly result message. The cluster_defaults
// section reports only the defaults that actually shaped the generated config:
// a field overridden by an explicit flag (e.g. --region) is not a "used default"
// and must not be listed, or the output contradicts the written configuration.
func (s *InitService) buildResultMessage(clusterPaths *paths.ClusterPaths, organization, gitDir string, keysGenerated bool, opts InitOptions) string {
	var msg strings.Builder
	fmt.Fprintf(&msg, "Created cluster configuration in organization '%s' at '%s'\n", organization, clusterPaths.ConfigPath)
	fmt.Fprintf(&msg, "GitOps repository root: %s\n", gitDir)
	if keysGenerated {
		fmt.Fprintf(&msg, "SOPS key location: %s\n", clusterPaths.SOPSKeyPath)
	}

	if opts.BaseDomain != "" {
		fmt.Fprintf(&msg, "Base domain: %s (from --base-domain; follows cluster_defaults.base_domain from settings until pinned with 'cluster set')\n", opts.BaseDomain)
	}

	msg.WriteString(clusterDefaultsBlock(opts))

	return msg.String()
}

// clusterDefaultsBlock renders the cluster_defaults that config.yaml supplied,
// so a user can see which values came from the CLI config rather than from a
// flag. Shared by the real and dry-run messages so the two cannot drift.
func clusterDefaultsBlock(opts InitOptions) string {
	cm, err := config.NewConfigManager("")
	if err != nil {
		return ""
	}
	cd := cm.GetConfig().ClusterDefaults
	var defaults []string
	if cd.Provider != "" {
		defaults = append(defaults, fmt.Sprintf("  provider:            %s", cd.Provider))
	}
	if cd.Region != "" && !opts.RegionExplicit {
		defaults = append(defaults, fmt.Sprintf("  region:              %s", cd.Region))
	}
	if cd.Environment != "" {
		defaults = append(defaults, fmt.Sprintf("  environment:         %s", cd.Environment))
	}
	if len(cd.SSHAuthorizedKeys) > 0 {
		defaults = append(defaults, fmt.Sprintf("  ssh_authorized_keys: %d key(s)", len(cd.SSHAuthorizedKeys)))
	}
	// --base-domain supersedes the settings default; listing both would
	// contradict the written config (same rule as region above).
	if cd.BaseDomain != "" && opts.BaseDomain == "" {
		defaults = append(defaults, fmt.Sprintf("  base_domain:         %s", cd.BaseDomain))
	}
	if cd.AdminEmail != "" {
		defaults = append(defaults, fmt.Sprintf("  admin_email:         %s", cd.AdminEmail))
	}
	if cd.KubernetesVersion != "" {
		defaults = append(defaults, fmt.Sprintf("  kubernetes_version:  %s", cd.KubernetesVersion))
	}
	if cd.CNI != "" {
		defaults = append(defaults, fmt.Sprintf("  cni:                 %s", cd.CNI))
	}
	if cd.SSHUser != "" {
		defaults = append(defaults, fmt.Sprintf("  ssh_user:            %s", cd.SSHUser))
	}
	if len(defaults) == 0 {
		return ""
	}
	return "\nUsing cluster_defaults from config.yaml:\n" + strings.Join(defaults, "\n") + "\n"
}

// buildDryRunMessage reports what 'cluster init' would create without creating
// it. Every path is named, because the point of the preview is to see where the
// real run would write.
func (s *InitService) buildDryRunMessage(clusterPaths *paths.ClusterPaths, organization, gitDir string, opts InitOptions) string {
	var msg strings.Builder
	fmt.Fprintf(&msg, "[dry-run] Would create cluster configuration in organization '%s' at '%s'\n", organization, clusterPaths.ConfigPath)
	fmt.Fprintf(&msg, "[dry-run] GitOps repository root: %s\n", gitDir)
	if !opts.NoKeyGen {
		fmt.Fprintf(&msg, "[dry-run] Would generate SOPS age key: %s\n", clusterPaths.SOPSKeyPath)
		fmt.Fprintf(&msg, "[dry-run] Would generate SSH key pair: %s\n", clusterPaths.SSHKeyPath)
	}
	if !opts.NoGitInit {
		fmt.Fprintf(&msg, "[dry-run] Would initialize a git repository in the cluster directory\n")
	}
	if opts.BaseDomain != "" {
		fmt.Fprintf(&msg, "[dry-run] Would set base domain: %s (from --base-domain; follows cluster_defaults.base_domain from settings)\n", opts.BaseDomain)
	}
	msg.WriteString(clusterDefaultsBlock(opts))
	fmt.Fprintf(&msg, "\n[dry-run] Nothing was written.\n")
	return msg.String()
}

func setNestedConfigValue(configMap map[string]any, value any, parts ...string) {
	if len(parts) == 0 {
		return
	}

	current := configMap
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}

	current[parts[len(parts)-1]] = value
}

func hasExplicitConfigValue(configMap map[string]any, opts InitOptions, parts ...string) bool {
	dottedPath := strings.Join(parts, ".")
	if hasFlagOverride(opts.FlagOverrides, dottedPath) {
		return true
	}

	if opts.ConfigFile == "" && opts.ConfigMap == nil {
		return false
	}

	current := any(configMap)
	for _, part := range parts {
		next, ok := current.(map[string]any)
		if !ok {
			return false
		}
		value, exists := next[part]
		if !exists {
			return false
		}
		current = value
	}

	return true
}

func hasFlagOverride(overrides []string, key string) bool {
	prefix := "--" + key + "="
	for _, override := range overrides {
		if strings.HasPrefix(override, prefix) {
			return true
		}
	}
	return false
}

// generateKeys generates SSH and SOPS keys for the cluster
func (s *InitService) generateKeys(clusterPaths *paths.ClusterPaths, cfg *v2.Config, opts InitOptions) (bool, error) {
	keysGenerated := false

	// Generate SOPS Age key if not disabled
	if !opts.NoSOPSKeyGen {
		sopsKeyExists := false
		if _, err := os.Stat(clusterPaths.SOPSKeyPath); err == nil {
			sopsKeyExists = true
		}

		if opts.RegenerateKeys || !sopsKeyExists {
			if err := s.generateSOPSKey(clusterPaths); err != nil {
				return false, fmt.Errorf("generating SOPS key: %w", err)
			}
			keysGenerated = true
		}

		if cfg.Secrets.SopsAgeKeyFile != "" {
			if _, err := os.Stat(clusterPaths.SOPSConfigPath); os.IsNotExist(err) || opts.RegenerateKeys || keysGenerated {
				if err := s.ensureSOPSConfig(clusterPaths, cfg); err != nil {
					return false, fmt.Errorf("creating SOPS config: %w", err)
				}
			}
		}
	}

	// Generate SSH key pair
	sshKeyExists := false
	if _, err := os.Stat(clusterPaths.SSHKeyPath); err == nil {
		sshKeyExists = true
	}

	if opts.RegenerateKeys || !sshKeyExists {
		if err := s.generateSSHKey(clusterPaths, cfg); err != nil {
			return false, fmt.Errorf("generating SSH key: %w", err)
		}
		keysGenerated = true
	}

	return keysGenerated, nil
}

// generateSOPSKey generates a SOPS Age key for the cluster
func (s *InitService) generateSOPSKey(clusterPaths *paths.ClusterPaths) error {
	// Create the secrets directory with proper permissions
	secretsKeyDir := filepath.Dir(clusterPaths.SOPSKeyPath)
	if err := os.MkdirAll(secretsKeyDir, 0o700); err != nil {
		return fmt.Errorf("creating secrets directory: %w", err)
	}
	if err := ensureMode(secretsKeyDir, 0o700); err != nil {
		return err
	}

	// Use the SOPS key manager to generate and save an Age key
	km := sops.NewKeyManager(secretsKeyDir)
	keyPair, err := km.GenerateAgeKey()
	if err != nil {
		return fmt.Errorf("generating Age key pair: %w", err)
	}

	// Extract cluster name from the key path
	// Path format: <org>/secrets/age/<cluster>-key.txt
	keyFileName := filepath.Base(clusterPaths.SOPSKeyPath)
	keyName := strings.TrimSuffix(keyFileName, ".txt")

	// Save the key using the key manager
	if err := km.SaveAgeKey(keyPair, keyName); err != nil {
		return fmt.Errorf("saving Age key pair: %w", err)
	}

	return verifyMode(clusterPaths.SOPSKeyPath, 0o600)
}

func (s *InitService) ensureSOPSConfig(clusterPaths *paths.ClusterPaths, cfg *v2.Config) error {
	_, err := sops.EnsureSOPSConfig(clusterPaths.SOPSConfigPath, cfg, s.fileSystem)
	if err != nil {
		return err
	}
	// Re-assert the mode even when the file already existed: init with
	// --regenerate-keys must leave a 0644 config regardless of prior state.
	return verifyMode(clusterPaths.SOPSConfigPath, 0o644)
}

// generateSSHKey generates an SSH key pair for the cluster
func (s *InitService) generateSSHKey(clusterPaths *paths.ClusterPaths, cfg *v2.Config) error {
	// Create SSH directory if it doesn't exist
	sshDir := filepath.Dir(clusterPaths.SSHKeyPath)
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("creating SSH directory: %w", err)
	}
	if err := ensureMode(sshDir, 0o700); err != nil {
		return err
	}

	clusterName := cfg.ClusterName()
	organization := cfg.OpenCenter.Meta.Organization
	if organization == "" {
		organization = filepath.Base(filepath.Dir(clusterPaths.SecretsDir))
	}

	// Get region from config
	region := cfg.OpenCenter.Meta.Region
	if region == "" {
		region = "local"
	}

	// Create SSH key comment
	sshKeyComment := fmt.Sprintf("%s-%s-%s", organization, clusterName, region)

	// Get cipher from config or default to ed25519
	cipher := cfg.Secrets.SSHKey.Cypher
	if cipher == "" {
		cipher = "ed25519"
	}

	// Generate SSH key pair
	keyPair, err := crypto.GenerateSSHKeyWithComment(cipher, sshKeyComment)
	if err != nil {
		return fmt.Errorf("generating SSH key pair: %w", err)
	}

	// Write private key with restrictive permissions
	if err := s.fileSystem.WriteFileAtomic(clusterPaths.SSHKeyPath, keyPair.PrivateKey, 0o600); err != nil {
		return fmt.Errorf("writing SSH private key: %w", err)
	}
	if err := verifyMode(clusterPaths.SSHKeyPath, 0o600); err != nil {
		return err
	}

	// Write public key
	pubKeyPath := clusterPaths.SSHKeyPath + ".pub"
	if err := s.fileSystem.WriteFile(pubKeyPath, keyPair.PublicKey, 0o644); err != nil {
		return fmt.Errorf("writing SSH public key: %w", err)
	}
	if err := verifyMode(pubKeyPath, 0o644); err != nil {
		return err
	}

	if shouldPopulateGeneratedSSHAuthorizedKey(cfg.OpenCenter.Infrastructure.SSH.AuthorizedKeys) {
		cfg.OpenCenter.Infrastructure.SSH.AuthorizedKeys = []string{strings.TrimSpace(string(keyPair.PublicKey))}
	}

	return nil
}

func shouldPopulateGeneratedSSHAuthorizedKey(keys []string) bool {
	if len(keys) == 0 {
		return true
	}

	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" && trimmed != v2.DefaultSSHAuthorizedKeyPlaceholder {
			return false
		}
	}

	return true
}

func (s *InitService) v2Loader() *v2.ConfigLoader {
	return v2.NewConfigLoader(configdefaults.NewRegistry())
}

// initGitRepo initializes a git repository for the cluster
func (s *InitService) initGitRepo(clusterPaths *paths.ClusterPaths) error {
	// Create the GitOps directory if it doesn't exist
	if err := os.MkdirAll(clusterPaths.GitOpsDir, 0o755); err != nil {
		return fmt.Errorf("creating GitOps directory: %w", err)
	}

	if err := writeGitOpsHygiene(clusterPaths.GitOpsDir); err != nil {
		return err
	}

	gitDir := filepath.Join(clusterPaths.GitOpsDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init")
		cmd.Dir = clusterPaths.GitOpsDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git init failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}

	cmd := exec.Command("git", "config", "core.hooksPath", ".opencenter/hooks")
	cmd.Dir = clusterPaths.GitOpsDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config core.hooksPath failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// gitIgnoreContent is also the seed content used when initializing a brand-new
// GitOps repository. A hygiene file that pre-exists in a shared organization
// repository (for example one created by an earlier cluster's init) belongs to
// that repository: silently replacing it would desynchronize the openCenter
// ownership ledger from the on-disk file and block subsequent `cluster generate`
// runs, so existing files are preserved. The root-only config guards are a hard
// requirement: silently rewriting user patterns could un-ignore tracked files,
// so a file missing them fails with an actionable error. The deploy-time
// exclusions below are machine hygiene, not user content: when an existing file
// lacks them, they are appended (idempotently) so OpenTofu/Kubespray artifacts
// (legacy-mode inventory, module cache, lock file) stop dirtying the worktree.
var gitIgnoreRequiredPatterns = []string{
	"/*-config.yaml",
	"/.*-config.yaml",
}

// gitIgnoreDeployArtifactPatterns keep deploy-time outputs out of the GitOps
// worktree's git status. Single source of truth: gitops.DeployArtifactIgnorePatterns,
// which `cluster generate` also uses to heal a tracked .gitignore that predates
// an exclusion.
var gitIgnoreDeployArtifactPatterns = append([]string(nil), gitops.DeployArtifactIgnorePatterns...)

func writeGitOpsHygiene(gitOpsDir string) error {
	files := map[string]struct {
		content string
		mode    os.FileMode
	}{
		".gitignore": {
			content: gitIgnoreContent,
			mode:    0o644,
		},
		filepath.Join(".opencenter", "hooks", "pre-commit"): {
			content: preCommitHookContent,
			mode:    0o755,
		},
		filepath.Join(".opencenter", "scripts", "scan-secrets"): {
			content: scannerScriptContent,
			mode:    0o755,
		},
		filepath.Join(".github", "workflows", "opencenter-secret-scan.yml"): {
			content: githubSecretScanWorkflow,
			mode:    0o644,
		},
	}

	for rel, spec := range files {
		path := filepath.Join(gitOpsDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating %s parent: %w", rel, err)
		}
		if info, statErr := os.Stat(path); statErr == nil {
			if !info.IsDir() && rel == ".gitignore" {
				existing, readErr := os.ReadFile(path)
				if readErr != nil {
					return fmt.Errorf("reading %s: %w", path, readErr)
				}
				missingGuards := false
				for _, pattern := range gitIgnoreRequiredPatterns {
					if !gitIgnoreContainsLine(existing, pattern) {
						missingGuards = true
						break
					}
				}
				if missingGuards {
					return fmt.Errorf("%s exists but is missing required pattern(s) %v; add them manually or remove the file and re-run", path, gitIgnoreRequiredPatterns)
				}
				if err := appendMissingGitIgnorePatterns(path, existing, gitIgnoreDeployArtifactPatterns); err != nil {
					return err
				}
				continue // user-owned file in a shared repo: never clobber
			}
			// Machine-managed hook/script/workflow (or an unexpected directory):
			// safe to refresh in place.
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("stat %s: %w", path, statErr)
		}
		if err := os.WriteFile(path, []byte(spec.content), spec.mode); err != nil {
			return fmt.Errorf("writing %s: %w", rel, err)
		}
		if err := verifyMode(path, spec.mode); err != nil {
			return err
		}
	}
	return nil
}

const gitIgnoreContent = `# Private keys and secrets should never exist in this GitOps tree.
*.key
*-key.txt
id_rsa*
id_ed25519*
*.pem
*.age

# Cluster config input is local state, not a GitOps artifact.
/*-config.yaml
/.*-config.yaml

# Deploy-time OpenTofu/Kubespray artifacts
infrastructure/clusters/*/inventory/
infrastructure/clusters/*/venv/
infrastructure/clusters/*/kubespray/
infrastructure/clusters/*/.bin/
infrastructure/clusters/*/kubespray-outputs.json
infrastructure/clusters/*/ansible-local-tmp/
infrastructure/clusters/*/.kubeconfig.fetch.tmp
infrastructure/clusters/*/.terraform/
infrastructure/clusters/*/.terraform.lock.hcl
`

// gitIgnoreContainsLine reports whether data has pattern as a whole line.
func gitIgnoreContainsLine(data []byte, pattern string) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimRight(line, "\r") == pattern {
			return true
		}
	}
	return false
}

// appendMissingGitIgnorePatterns appends the patterns missing from the
// existing file content, once, under a marker comment. Idempotent: a file that
// already contains every pattern is left byte-identical.
func appendMissingGitIgnorePatterns(path string, existing []byte, patterns []string) error {
	var missing []string
	for _, pattern := range patterns {
		if !gitIgnoreContainsLine(existing, pattern) {
			missing = append(missing, pattern)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	content := string(existing)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n# Deploy-time OpenTofu/Kubespray artifacts (managed by openCenter).\n"
	for _, pattern := range missing {
		content += pattern + "\n"
	}
	info, err := os.Stat(path)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return fmt.Errorf("updating %s: %w", path, err)
	}
	return nil
}

const preCommitHookContent = `#!/usr/bin/env sh
set -eu

if [ "${OPENCENTER_SKIP_HOOKS:-}" = "1" ]; then
  echo "WARNING: openCenter pre-commit checks skipped by OPENCENTER_SKIP_HOOKS=1" >&2
  exit 0
fi

repo_root=$(git rev-parse --show-toplevel)
opencenter cluster validate-manifests --repo-path "$repo_root" --staged --security-only
`

const scannerScriptContent = `#!/usr/bin/env sh
set -eu

repo_root=${1:-$(git rev-parse --show-toplevel)}
opencenter cluster validate-manifests --repo-path "$repo_root" --security-only
`

const githubSecretScanWorkflow = `name: openCenter secret scan

on:
  pull_request:
  push:

jobs:
  secret-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run openCenter secret scanner
        run: ./.opencenter/scripts/scan-secrets "$GITHUB_WORKSPACE"
`

func ensureMode(path string, want os.FileMode) error {
	if err := os.Chmod(path, want); err != nil {
		return fmt.Errorf("setting mode on %s: %w", path, err)
	}
	return verifyMode(path, want)
}

func verifyMode(path string, want os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		if os.Getenv("OPENCENTER_ALLOW_INSECURE_FILE_MODES") == "1" {
			fmt.Fprintf(os.Stderr, "warning: %s mode is %#o, expected %#o; continuing because OPENCENTER_ALLOW_INSECURE_FILE_MODES=1\n", path, got, want)
			return nil
		}
		return fmt.Errorf("%s mode is %#o, expected %#o", path, got, want)
	}
	return nil
}
