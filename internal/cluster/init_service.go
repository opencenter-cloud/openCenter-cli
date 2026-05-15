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
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"gopkg.in/yaml.v3"
)

// InitOptions contains options for cluster initialization
type InitOptions struct {
	ClusterName           string
	Organization          string
	Provider              string
	DeploymentMethod      string
	ConfigFile            string
	ConfigMap             map[string]any
	Force                 bool
	Strict                bool
	NoKeyGen              bool
	NoSOPSKeyGen          bool
	RegenerateKeys        bool
	NoGitInit             bool
	FullSchema            bool
	SchemaVersion         string
	ServerPools           []string
	FlagOverrides         []string
	KindDisableDefaultCNI *bool
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

	// Update configuration with resolved paths
	s.updateConfigPaths(cfg, configMap, clusterPaths, opts)

	// Validate configuration if strict mode
	if opts.Strict {
		if err := s.validateConfig(cfg); err != nil {
			return nil, fmt.Errorf("config validation failed: %w", err)
		}
	}

	// Create directory structure
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
	result.Message = s.buildResultMessage(clusterPaths, opts.Organization, cfg.OpenCenter.GitOps.Repository.LocalDir, result.KeysGenerated)

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
		// Load from file
		data, err := s.fileSystem.ReadFile(opts.ConfigFile)
		if err != nil {
			return nil, nil, fmt.Errorf("reading config file: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, nil, fmt.Errorf("parsing config file: %w", err)
		}

		configMap = make(map[string]any)
		if err := yaml.Unmarshal(data, &configMap); err != nil {
			return nil, nil, fmt.Errorf("parsing config file to map: %w", err)
		}
	} else if opts.ConfigMap != nil {
		// Use provided config map
		configMap = opts.ConfigMap
		data, err := yaml.Marshal(configMap)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling config map: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, nil, fmt.Errorf("parsing config map: %w", err)
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
		cfg, err = v2.NewV2FullTemplate(opts.ClusterName, opts.Provider)
		if err != nil {
			return nil, nil, fmt.Errorf("building v2 full template: %w", err)
		}
	} else {
		cfg, err = v2.NewV2Default(opts.ClusterName, opts.Provider)
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

	data, err := yaml.Marshal(cfg)
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
	// Apply organization
	if opts.Organization != "" {
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
	if cfg.OpenCenter.Meta.Region == "" || cfg.OpenCenter.Meta.Region == "sjc3" {
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

	if opts.KindDisableDefaultCNI != nil {
		if !strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "kind") {
			return fmt.Errorf("--kind-disable-default-cni is only valid for kind clusters")
		}
		if cfg.OpenCenter.Infrastructure.Kind == nil {
			cfg.OpenCenter.Infrastructure.Kind = &v2.KindCompatibilityConfig{}
		}
		cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI = *opts.KindDisableDefaultCNI
		setNestedConfigValue(configMap, *opts.KindDisableDefaultCNI, "opencenter", "infrastructure", "kind", "disable_default_cni")
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
			cfg.OpenCenter.GitOps.Auth.Token.Provider = "github"
			setNestedConfigValue(configMap, "github", "opencenter", "gitops", "auth", "token", "provider")
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
	data, err := yaml.Marshal(cfg)
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
		clusterPaths.ClusterStateDir,
		clusterPaths.InventoryPath,
		clusterPaths.VenvPath,
		clusterPaths.BinPath,
		clusterPaths.SecretsDir,
		filepath.Join(clusterPaths.SecretsDir, "age"),
		filepath.Dir(clusterPaths.SOPSKeyPath),
		filepath.Join(clusterPaths.SecretsDir, "ssh"),
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

	loader := s.v2Loader()
	if fullSchema {
		data, err := v2.RenderFullTemplateYAMLFromConfig(cfg)
		if err != nil {
			return fmt.Errorf("rendering v2 full template: %w", err)
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

// buildResultMessage builds a user-friendly result message
func (s *InitService) buildResultMessage(clusterPaths *paths.ClusterPaths, organization, gitDir string, keysGenerated bool) string {
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("Created cluster configuration in organization '%s' at '%s'\n", organization, clusterPaths.ConfigPath))
	msg.WriteString(fmt.Sprintf("GitOps repository root: %s\n", gitDir))
	if keysGenerated {
		msg.WriteString(fmt.Sprintf("SOPS key location: %s\n", clusterPaths.SOPSKeyPath))
	}

	// Surface which cluster_defaults were used from config.yaml
	if cm, err := config.NewConfigManager(""); err == nil {
		cd := cm.GetConfig().ClusterDefaults
		var defaults []string
		if cd.Provider != "" {
			defaults = append(defaults, fmt.Sprintf("  provider:            %s", cd.Provider))
		}
		if cd.Region != "" {
			defaults = append(defaults, fmt.Sprintf("  region:              %s", cd.Region))
		}
		if cd.Environment != "" {
			defaults = append(defaults, fmt.Sprintf("  environment:         %s", cd.Environment))
		}
		if len(cd.SSHAuthorizedKeys) > 0 {
			defaults = append(defaults, fmt.Sprintf("  ssh_authorized_keys: %d key(s)", len(cd.SSHAuthorizedKeys)))
		}
		if cd.BaseDomain != "" {
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
		if len(defaults) > 0 {
			msg.WriteString("\nUsing cluster_defaults from config.yaml:\n")
			for _, d := range defaults {
				msg.WriteString(d + "\n")
			}
		}
	}

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
	publicKey, err := s.readAgePublicKey(cfg.Secrets.SopsAgeKeyFile)
	if err != nil {
		return err
	}

	content := fmt.Sprintf(`creation_rules:
  - path_regex: 'secrets/age/keys/.*-key\.txt$'
    age: >-
      %s
  - path_regex: 'secrets/ssh/[^.]+$'
    age: >-
      %s
  - path_regex: 'applications/overlays/[^/]+/(managed-services|services)/.*/.*\.ya?ml$'
    encrypted_regex: "^(secret)$"
    age: >-
      %s
  - path_regex: '^infrastructure/clusters/%s/.*\.ya?ml$'
    encrypted_regex: "^(secret)$"
    age: >-
      %s
`, publicKey, publicKey, publicKey, cfg.OpenCenter.Cluster.ClusterName, publicKey)

	if err := s.fileSystem.WriteFileAtomic(clusterPaths.SOPSConfigPath, []byte(content), 0o644); err != nil {
		return err
	}
	return verifyMode(clusterPaths.SOPSConfigPath, 0o644)
}

func (s *InitService) readAgePublicKey(keyPath string) (string, error) {
	if keyPath == "" {
		return "", fmt.Errorf("SOPS age key path cannot be empty")
	}

	data, err := s.fileSystem.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("reading SOPS age key: %w", err)
	}

	privateKey := extractAgePrivateKey(string(data))
	if privateKey == "" {
		return "", fmt.Errorf("no valid age private key found in %s", keyPath)
	}

	keyPair, err := crypto.ParseAgeKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("parsing SOPS age key: %w", err)
	}

	return strings.TrimSpace(keyPair.PublicKey), nil
}

func extractAgePrivateKey(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "AGE-SECRET-KEY-") {
			return trimmed
		}
	}

	return ""
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
`

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
