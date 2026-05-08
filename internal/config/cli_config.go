// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	corePaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"gopkg.in/yaml.v3"
)

// CLIConfig represents the global CLI configuration system that controls
// CLI behavior, logging, and default paths.
type CLIConfig struct {
	Logging         LoggingConfig         `yaml:"logging"`
	Paths           PathsConfig           `yaml:"paths"`
	Behavior        BehaviorConfig        `yaml:"behavior"`
	ClusterDefaults ClusterDefaultsConfig `yaml:"cluster_defaults"`
}

// LoggingConfig controls logging behavior including level, format, output, and file rotation.
type LoggingConfig struct {
	Level  string     `yaml:"level"`  // debug, info, warn, error
	Format string     `yaml:"format"` // text, json, yaml
	Output string     `yaml:"output"` // stdout, stderr, or file path
	File   FileConfig `yaml:"file"`
}

// FileConfig controls file logging rotation settings.
type FileConfig struct {
	MaxSize    int  `yaml:"maxSize"` // MB
	MaxBackups int  `yaml:"maxBackups"`
	MaxAge     int  `yaml:"maxAge"` // days
	Compress   bool `yaml:"compress"`
}

// PathsConfig controls default paths for configuration and clusters.
type PathsConfig struct {
	SettingsDir     string `yaml:"settingsDir"`
	ClustersDir     string `yaml:"clustersDir"`
	BlueprintsDir   string `yaml:"blueprintsDir"`
	GitOpsDir       string `yaml:"gitopsDir"`
	ClusterStateDir string `yaml:"clusterStateDir"`
	SecretsDir      string `yaml:"secretsDir"`
	PluginsDir      string `yaml:"pluginsDir"`
	StateDir        string `yaml:"stateDir"`
}

// BehaviorConfig controls CLI behavior settings.
type BehaviorConfig struct {
	AutoConfirm bool   `yaml:"autoConfirm"`
	DryRun      bool   `yaml:"dryRun"`
	Validation  string `yaml:"validation"`
}

const (
	ValidationModeOffline = "offline"
	ValidationModeOnline  = "online"

	GitopsAuthMethodSSH   = "ssh"
	GitopsAuthMethodToken = "token"
)

// ClusterDefaultsConfig contains default values applied when generating new cluster
// configurations via "opencenter cluster init". These values are injected into the
// cluster config YAML when the corresponding field is not already set.
type ClusterDefaultsConfig struct {
	Organization      string   `yaml:"organization,omitempty"`
	Provider          string   `yaml:"provider"`
	Region            string   `yaml:"region"`
	Environment       string   `yaml:"environment"`
	GitopsAuthMethod  string   `yaml:"gitops_auth_method"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys,omitempty"`
	BaseDomain        string   `yaml:"base_domain,omitempty"`
	AdminEmail        string   `yaml:"admin_email,omitempty"`
	KubernetesVersion string   `yaml:"kubernetes_version,omitempty"`
	CNI               string   `yaml:"cni,omitempty"`
	SSHUser           string   `yaml:"ssh_user,omitempty"`
}

// ConfigManager handles CLI configuration loading, validation, and merging.
type ConfigManager struct {
	mu         sync.RWMutex
	configPath string
	config     *CLIConfig
	defaults   *CLIConfig
	validator  *ConfigValidator
	fileSystem fs.FileSystem
}

// ConfigValidator validates configuration values and structure.
type ConfigValidator struct {
	autoRepair bool
}

// ConfigError represents configuration-related errors with actionable messages.
type ConfigError struct {
	Type     string // validation, permission, path, format
	Field    string // dot notation path to problematic field
	Value    interface{}
	Message  string
	Repaired bool // indicates if the error was automatically repaired
}

// ValidationResult contains the results of configuration validation.
type ValidationResult struct {
	Valid    bool
	Errors   []*ConfigError
	Warnings []*ConfigError
	Repaired []*ConfigError
}

func (ce *ConfigError) Error() string {
	prefix := ""
	if ce.Repaired {
		prefix = "[AUTO-REPAIRED] "
	}

	if ce.Field != "" {
		return fmt.Sprintf("%s%s error in field '%s': %s", prefix, ce.Type, ce.Field, ce.Message)
	}
	return fmt.Sprintf("%s%s error: %s", prefix, ce.Type, ce.Message)
}

// Suggestions returns actionable suggestions for fixing the configuration error.
func (ce *ConfigError) Suggestions() []string {
	if ce.Repaired {
		return []string{"Configuration was automatically repaired with default values"}
	}

	switch ce.Type {
	case "validation":
		switch ce.Field {
		case "logging.level":
			return []string{"Use one of: debug, info, warn, error"}
		case "logging.format":
			return []string{"Use one of: text, json, yaml"}
		case "logging.output":
			return []string{"Use 'stdout', 'stderr', or a valid file path"}
		case "logging.file.maxSize":
			return []string{"Use a positive integer value (MB)"}
		case "logging.file.maxBackups":
			return []string{"Use a non-negative integer value"}
		case "logging.file.maxAge":
			return []string{"Use a non-negative integer value (days)"}
		case "paths.settingsDir", "paths.clustersDir", "paths.gitopsDir", "paths.clusterStateDir", "paths.secretsDir":
			return []string{"Use an absolute path or path starting with ~"}
		case "paths.stateDir":
			return []string{"Use an absolute path or path starting with ~"}
		}
	case "permission":
		return []string{
			"Check file/directory permissions",
			"Ensure the directory is writable",
			"Run with appropriate user permissions",
			"Try running: chmod 755 <directory>",
		}
	case "path":
		return []string{
			"Ensure the path exists or can be created",
			"Check for typos in the path",
			"Verify parent directories exist",
			"Try creating the directory manually: mkdir -p <path>",
		}
	case "dependency":
		return []string{
			"Ensure required dependencies are installed",
			"Check system requirements",
			"Verify environment setup",
		}
	}
	return []string{"Check the configuration documentation for valid values"}
}

// NewConfigManager creates a new configuration manager with the specified config path.
// If configPath is empty, it uses the default CLI configuration location.
func NewConfigManager(configPath string) (*ConfigManager, error) {
	if configPath == "" {
		var err error
		configPath, err = DefaultCLIConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to determine default config path: %w", err)
		}
	}

	// Expand environment variables and tilde
	configPath = corePaths.ExpandPath(configPath)

	// Create FileSystem with error handler
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	cm := &ConfigManager{
		configPath: configPath,
		defaults:   DefaultCLIConfig(),
		validator:  &ConfigValidator{autoRepair: true},
		fileSystem: fileSystem,
	}

	// Load configuration
	if err := cm.Load(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return cm, nil
}

// DefaultCLIConfig returns the default CLI configuration.
func DefaultCLIConfig() *CLIConfig {
	configDir := DefaultConfigDir()
	clustersDir := filepath.Join(configDir, "clusters")
	blueprintsDir := filepath.Join(clustersDir, "blueprints")
	gitopsDir := filepath.Join(clustersDir, "gitops")
	clusterStateDir := filepath.Join(clustersDir, "state")
	secretsDir := filepath.Join(clustersDir, "secrets")
	pluginsDir := filepath.Join(configDir, "plugins")
	stateDir := DefaultStateDir()

	if envClustersDir := clustersDirFromEnv(); envClustersDir != "" {
		clustersDir = corePaths.ExpandPath(envClustersDir)
		blueprintsDir = filepath.Join(clustersDir, "blueprints")
		gitopsDir = filepath.Join(clustersDir, "gitops")
		clusterStateDir = filepath.Join(clustersDir, "state")
		secretsDir = filepath.Join(clustersDir, "secrets")
	}
	if envBlueprintsDir := os.Getenv("OPENCENTER_BLUEPRINTS_DIR"); envBlueprintsDir != "" {
		blueprintsDir = corePaths.ExpandPath(envBlueprintsDir)
	}
	if envGitOpsDir := os.Getenv("OPENCENTER_GITOPS_DIR"); envGitOpsDir != "" {
		gitopsDir = corePaths.ExpandPath(envGitOpsDir)
	}
	if envClusterStateDir := os.Getenv("OPENCENTER_CLUSTER_STATE_DIR"); envClusterStateDir != "" {
		clusterStateDir = corePaths.ExpandPath(envClusterStateDir)
	}
	if envSecretsDir := os.Getenv("OPENCENTER_SECRETS_DIR"); envSecretsDir != "" {
		secretsDir = corePaths.ExpandPath(envSecretsDir)
	}
	if envPluginsDir := os.Getenv("OPENCENTER_PLUGINS_DIR"); envPluginsDir != "" {
		pluginsDir = corePaths.ExpandPath(envPluginsDir)
	}

	return &CLIConfig{
		Logging: LoggingConfig{
			Level:  "warn",
			Format: "text",
			Output: "stderr",
			File: FileConfig{
				MaxSize:    100,
				MaxBackups: 3,
				MaxAge:     28,
				Compress:   true,
			},
		},
		Paths: PathsConfig{
			SettingsDir:     configDir,
			ClustersDir:     clustersDir,
			BlueprintsDir:   blueprintsDir,
			GitOpsDir:       gitopsDir,
			ClusterStateDir: clusterStateDir,
			SecretsDir:      secretsDir,
			PluginsDir:      pluginsDir,
			StateDir:        stateDir,
		},
		Behavior: BehaviorConfig{
			AutoConfirm: false,
			DryRun:      false,
			Validation:  ValidationModeOffline,
		},
		ClusterDefaults: ClusterDefaultsConfig{
			Organization:     "opencenter",
			Provider:         "openstack",
			Region:           "dfw3",
			Environment:      "dev",
			GitopsAuthMethod: GitopsAuthMethodToken,
		},
	}
}

// DefaultCLIConfigPath returns the default path for the CLI settings file.
func DefaultCLIConfigPath() (string, error) {
	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "settings.yaml"), nil
}

// Load loads the CLI configuration from the file system.
// If the file doesn't exist, it creates it with default values.
func (cm *ConfigManager) Load() error {
	// Ensure the configuration directory exists
	configDir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return &ConfigError{
			Type:    "permission",
			Field:   "configDir",
			Value:   configDir,
			Message: fmt.Sprintf("failed to create configuration directory: %v", err),
		}
	}

	// Check if config file exists
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		// Create default configuration file
		if err := cm.createDefaultConfig(); err != nil {
			return fmt.Errorf("failed to create default configuration: %w", err)
		}
	}

	// Read configuration file using FileSystem
	data, err := cm.fileSystem.ReadFile(cm.configPath)
	if err != nil {
		return &ConfigError{
			Type:    "path",
			Field:   "configFile",
			Value:   cm.configPath,
			Message: fmt.Sprintf("failed to read configuration file: %v", err),
		}
	}

	// Parse YAML
	var config CLIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return &ConfigError{
			Type:    "format",
			Field:   "yaml",
			Value:   string(data),
			Message: fmt.Sprintf("failed to parse YAML: %v", err),
		}
	}

	// Merge with defaults to ensure completeness
	cm.config = cm.mergeWithDefaults(&config)

	// Validate configuration with auto-repair
	result := cm.validator.ValidateWithResult(cm.config)
	if !result.Valid {
		// Apply auto-repairs if any were made
		if len(result.Repaired) > 0 {
			// Save the repaired configuration
			if err := cm.Save(); err != nil {
				return fmt.Errorf("failed to save auto-repaired configuration: %w", err)
			}
		}

		// If there are still errors after repair, return them
		if len(result.Errors) > 0 {
			return result.Errors[0] // Return the first error
		}
	}

	// Expand paths in the loaded configuration
	cm.expandConfigPaths()

	// Initialize logging with the loaded configuration
	if err := InitializeLogging(&cm.config.Logging); err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}

	return nil
}

// createDefaultConfig creates a default configuration file with cluster_defaults
// fields commented out so users can see all available options without them being active.
func (cm *ConfigManager) createDefaultConfig() error {
	content := cm.generateDefaultConfigTemplate()

	if err := cm.fileSystem.WriteFileAtomic(cm.configPath, []byte(content), 0600); err != nil {
		return &ConfigError{
			Type:    "permission",
			Field:   "configFile",
			Value:   cm.configPath,
			Message: fmt.Sprintf("failed to write default configuration: %v", err),
		}
	}

	return nil
}

// generateDefaultConfigTemplate produces the settings.yaml content with cluster_defaults
// fields commented out. Active sections (logging, paths, behavior) are written normally;
// cluster_defaults values are present but commented so users can uncomment what they need.
func (cm *ConfigManager) generateDefaultConfigTemplate() string {
	d := cm.defaults

	var b strings.Builder

	// Logging section (active)
	b.WriteString("logging:\n")
	b.WriteString("  level: " + d.Logging.Level + "\n")
	b.WriteString("  format: " + d.Logging.Format + "\n")
	b.WriteString("  output: " + d.Logging.Output + "\n")
	b.WriteString("  file:\n")
	b.WriteString("    maxSize: " + strconv.Itoa(d.Logging.File.MaxSize) + "\n")
	b.WriteString("    maxBackups: " + strconv.Itoa(d.Logging.File.MaxBackups) + "\n")
	b.WriteString("    maxAge: " + strconv.Itoa(d.Logging.File.MaxAge) + "\n")
	b.WriteString("    compress: " + strconv.FormatBool(d.Logging.File.Compress) + "\n")

	// Paths section (active)
	b.WriteString("paths:\n")
	b.WriteString("  settingsDir: " + d.Paths.SettingsDir + "\n")
	b.WriteString("  clustersDir: " + d.Paths.ClustersDir + "\n")
	b.WriteString("  blueprintsDir: " + d.Paths.BlueprintsDir + "\n")
	b.WriteString("  gitopsDir: " + d.Paths.GitOpsDir + "\n")
	b.WriteString("  clusterStateDir: " + d.Paths.ClusterStateDir + "\n")
	b.WriteString("  secretsDir: " + d.Paths.SecretsDir + "\n")
	b.WriteString("  pluginsDir: " + d.Paths.PluginsDir + "\n")
	b.WriteString("  stateDir: " + d.Paths.StateDir + "\n")

	// Behavior section (active)
	b.WriteString("behavior:\n")
	b.WriteString("  autoConfirm: " + strconv.FormatBool(d.Behavior.AutoConfirm) + "\n")
	b.WriteString("  dryRun: " + strconv.FormatBool(d.Behavior.DryRun) + "\n")
	b.WriteString("  validation: " + d.Behavior.Validation + "\n")

	// Cluster defaults section (all commented out)
	b.WriteString("\n# Default values applied during 'opencenter cluster init'.\n")
	b.WriteString("# Uncomment and modify the values you want to override.\n")
	b.WriteString("cluster_defaults:\n")
	b.WriteString("  # organization: " + d.ClusterDefaults.Organization + "\n")
	b.WriteString("  # provider: " + d.ClusterDefaults.Provider + "\n")
	b.WriteString("  # region: " + d.ClusterDefaults.Region + "\n")
	b.WriteString("  # environment: " + d.ClusterDefaults.Environment + "\n")
	b.WriteString("  # gitops_auth_method: " + d.ClusterDefaults.GitopsAuthMethod + "\n")
	b.WriteString("  # base_domain: " + d.ClusterDefaults.BaseDomain + "\n")
	b.WriteString("  # admin_email: " + d.ClusterDefaults.AdminEmail + "\n")
	b.WriteString("  # kubernetes_version: " + d.ClusterDefaults.KubernetesVersion + "\n")
	b.WriteString("  # cni: " + d.ClusterDefaults.CNI + "\n")
	b.WriteString("  # ssh_user: " + d.ClusterDefaults.SSHUser + "\n")
	b.WriteString("  # ssh_authorized_keys:\n")
	b.WriteString("  #   - ssh-ed25519 AAAAC3... user@example.com\n")

	return b.String()
}

// mergeWithDefaults merges the loaded configuration with defaults to ensure completeness.
func (cm *ConfigManager) mergeWithDefaults(config *CLIConfig) *CLIConfig {
	merged := *cm.defaults

	// Merge logging configuration
	if config.Logging.Level != "" {
		merged.Logging.Level = config.Logging.Level
	}
	if config.Logging.Format != "" {
		merged.Logging.Format = config.Logging.Format
	}
	if config.Logging.Output != "" {
		merged.Logging.Output = config.Logging.Output
	}
	if config.Logging.File.MaxSize > 0 {
		merged.Logging.File.MaxSize = config.Logging.File.MaxSize
	}
	if config.Logging.File.MaxBackups > 0 {
		merged.Logging.File.MaxBackups = config.Logging.File.MaxBackups
	}
	if config.Logging.File.MaxAge > 0 {
		merged.Logging.File.MaxAge = config.Logging.File.MaxAge
	}
	merged.Logging.File.Compress = config.Logging.File.Compress

	// Merge paths configuration
	if config.Paths.SettingsDir != "" {
		merged.Paths.SettingsDir = config.Paths.SettingsDir
	}
	if config.Paths.ClustersDir != "" {
		merged.Paths.ClustersDir = config.Paths.ClustersDir
	}
	if config.Paths.BlueprintsDir != "" {
		merged.Paths.BlueprintsDir = config.Paths.BlueprintsDir
	}
	if config.Paths.GitOpsDir != "" {
		merged.Paths.GitOpsDir = config.Paths.GitOpsDir
	}
	if config.Paths.ClusterStateDir != "" {
		merged.Paths.ClusterStateDir = config.Paths.ClusterStateDir
	}
	if config.Paths.SecretsDir != "" {
		merged.Paths.SecretsDir = config.Paths.SecretsDir
	}
	if config.Paths.PluginsDir != "" {
		merged.Paths.PluginsDir = config.Paths.PluginsDir
	}
	if config.Paths.StateDir != "" {
		merged.Paths.StateDir = config.Paths.StateDir
	}

	// Merge behavior configuration
	merged.Behavior.AutoConfirm = config.Behavior.AutoConfirm
	merged.Behavior.DryRun = config.Behavior.DryRun
	if config.Behavior.Validation != "" {
		merged.Behavior.Validation = config.Behavior.Validation
	}

	// Merge cluster defaults configuration
	if config.ClusterDefaults.Organization != "" {
		merged.ClusterDefaults.Organization = config.ClusterDefaults.Organization
	}
	if config.ClusterDefaults.Provider != "" {
		merged.ClusterDefaults.Provider = config.ClusterDefaults.Provider
	}
	if config.ClusterDefaults.Region != "" {
		merged.ClusterDefaults.Region = config.ClusterDefaults.Region
	}
	if config.ClusterDefaults.Environment != "" {
		merged.ClusterDefaults.Environment = config.ClusterDefaults.Environment
	}
	if config.ClusterDefaults.GitopsAuthMethod != "" {
		merged.ClusterDefaults.GitopsAuthMethod = config.ClusterDefaults.GitopsAuthMethod
	}
	if len(config.ClusterDefaults.SSHAuthorizedKeys) > 0 {
		merged.ClusterDefaults.SSHAuthorizedKeys = config.ClusterDefaults.SSHAuthorizedKeys
	}
	if config.ClusterDefaults.BaseDomain != "" {
		merged.ClusterDefaults.BaseDomain = config.ClusterDefaults.BaseDomain
	}
	if config.ClusterDefaults.AdminEmail != "" {
		merged.ClusterDefaults.AdminEmail = config.ClusterDefaults.AdminEmail
	}
	if config.ClusterDefaults.KubernetesVersion != "" {
		merged.ClusterDefaults.KubernetesVersion = config.ClusterDefaults.KubernetesVersion
	}
	if config.ClusterDefaults.CNI != "" {
		merged.ClusterDefaults.CNI = config.ClusterDefaults.CNI
	}
	if config.ClusterDefaults.SSHUser != "" {
		merged.ClusterDefaults.SSHUser = config.ClusterDefaults.SSHUser
	}

	return &merged
}

// expandConfigPaths expands environment variables and tilde in configuration paths.
func (cm *ConfigManager) expandConfigPaths() {
	cm.config.Paths.SettingsDir = corePaths.ExpandPath(cm.config.Paths.SettingsDir)
	cm.config.Paths.ClustersDir = corePaths.ExpandPath(cm.config.Paths.ClustersDir)
	cm.config.Paths.BlueprintsDir = corePaths.ExpandPath(cm.config.Paths.BlueprintsDir)
	cm.config.Paths.GitOpsDir = corePaths.ExpandPath(cm.config.Paths.GitOpsDir)
	cm.config.Paths.ClusterStateDir = corePaths.ExpandPath(cm.config.Paths.ClusterStateDir)
	cm.config.Paths.SecretsDir = corePaths.ExpandPath(cm.config.Paths.SecretsDir)
	cm.config.Paths.PluginsDir = corePaths.ExpandPath(cm.config.Paths.PluginsDir)
	cm.config.Paths.StateDir = corePaths.ExpandPath(cm.config.Paths.StateDir)
}

// Save saves the current configuration to the file system.
func (cm *ConfigManager) Save() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	data, err := yaml.Marshal(cm.config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Use atomic write for configuration file
	if err := cm.fileSystem.WriteFileAtomic(cm.configPath, data, 0600); err != nil {
		return &ConfigError{
			Type:    "permission",
			Field:   "configFile",
			Value:   cm.configPath,
			Message: fmt.Sprintf("failed to write configuration: %v", err),
		}
	}

	return nil
}

// saveLocked persists the current configuration without acquiring the mutex.
// The caller must already hold cm.mu (write lock).
func (cm *ConfigManager) saveLocked() error {
	data, err := yaml.Marshal(cm.config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := cm.fileSystem.WriteFileAtomic(cm.configPath, data, 0600); err != nil {
		return &ConfigError{
			Type:    "permission",
			Field:   "configFile",
			Value:   cm.configPath,
			Message: fmt.Sprintf("failed to write configuration: %v", err),
		}
	}

	return nil
}

// GetConfig returns the current configuration.
func (cm *ConfigManager) GetConfig() *CLIConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.config
}

// SetValue sets a configuration value using dot notation.
func (cm *ConfigManager) SetValue(key string, value interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if err := cm.setValueByPath(cm.config, key, value); err != nil {
		return err
	}

	// Validate the updated configuration
	result := cm.validator.ValidateWithResult(cm.config)
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}

	return nil
}

// GetValue gets a configuration value using dot notation.
func (cm *ConfigManager) GetValue(key string) (interface{}, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.getValueByPath(cm.config, key)
}

// Reset resets the configuration to default values.
func (cm *ConfigManager) Reset() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config = DefaultCLIConfig()
	return cm.saveLocked()
}

// GetConfigPath returns the path to the configuration file.
func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}

// ValidateConfig performs comprehensive validation and returns detailed results.
func (cm *ConfigManager) ValidateConfig() *ValidationResult {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.validator.ValidateWithResult(cm.config)
}

// RepairConfig attempts to repair configuration issues and returns the results.
func (cm *ConfigManager) RepairConfig() (*ValidationResult, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Create a validator with auto-repair enabled
	repairValidator := &ConfigValidator{autoRepair: true}

	// Validate and repair
	result := repairValidator.ValidateWithResult(cm.config)

	// If repairs were made, save the configuration
	if len(result.Repaired) > 0 {
		if err := cm.saveLocked(); err != nil {
			return result, fmt.Errorf("failed to save repaired configuration: %w", err)
		}
	}

	return result, nil
}

// GetValidationSummary returns a human-readable summary of validation results.
func (cm *ConfigManager) GetValidationSummary() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := cm.validator.ValidateWithResult(cm.config)

	var summary strings.Builder

	if result.Valid {
		summary.WriteString("✓ Configuration is valid\n")
	} else {
		summary.WriteString("✗ Configuration has issues\n")
	}

	if len(result.Errors) > 0 {
		summary.WriteString(fmt.Sprintf("\nErrors (%d):\n", len(result.Errors)))
		for _, err := range result.Errors {
			summary.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
		}
	}

	if len(result.Warnings) > 0 {
		summary.WriteString(fmt.Sprintf("\nWarnings (%d):\n", len(result.Warnings)))
		for _, warning := range result.Warnings {
			summary.WriteString(fmt.Sprintf("  - %s\n", warning.Error()))
		}
	}

	if len(result.Repaired) > 0 {
		summary.WriteString(fmt.Sprintf("\nAuto-repaired (%d):\n", len(result.Repaired)))
		for _, repaired := range result.Repaired {
			summary.WriteString(fmt.Sprintf("  - %s\n", repaired.Error()))
		}
	}

	return summary.String()
}

// LoadWithConfig loads the configuration manager with an existing configuration.
// This is useful for applying runtime overrides without modifying the file.
func (cm *ConfigManager) LoadWithConfig(config *CLIConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config = config
	cm.validator = &ConfigValidator{autoRepair: false} // Don't auto-repair when loading existing config

	// Validate the provided configuration
	result := cm.validator.ValidateWithResult(cm.config)
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}

	// Expand paths in the configuration
	cm.expandConfigPaths()

	// Initialize logging with the loaded configuration
	if err := InitializeLogging(&cm.config.Logging); err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}

	return nil
}

// NewConfigManagerWithConfig creates a new configuration manager with an existing configuration.
// This is useful for creating temporary managers for applying overrides.
func NewConfigManagerWithConfig(config *CLIConfig) (*ConfigManager, error) {
	cm := &ConfigManager{
		config:    config,
		defaults:  DefaultCLIConfig(),
		validator: &ConfigValidator{autoRepair: false}, // Don't auto-repair when using existing config
	}

	// Validate the provided configuration
	result := cm.validator.ValidateWithResult(cm.config)
	if !result.Valid && len(result.Errors) > 0 {
		return nil, result.Errors[0]
	}

	// Expand paths in the configuration
	cm.expandConfigPaths()

	// Initialize logging with the loaded configuration
	if err := InitializeLogging(&cm.config.Logging); err != nil {
		return nil, fmt.Errorf("failed to initialize logging: %w", err)
	}

	return cm, nil
}

// setValueByPath sets a value in the configuration using dot notation.
func (cm *ConfigManager) setValueByPath(config *CLIConfig, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return &ConfigError{
			Type:    "validation",
			Field:   path,
			Value:   value,
			Message: "path must have at least two parts (e.g., 'logging.level')",
		}
	}

	switch parts[0] {
	case "logging":
		return cm.setLoggingValue(&config.Logging, parts[1:], value)
	case "paths":
		return cm.setPathsValue(&config.Paths, parts[1:], value)
	case "behavior":
		return cm.setBehaviorValue(&config.Behavior, parts[1:], value)
	case "cluster_defaults":
		return cm.setClusterDefaultsValue(&config.ClusterDefaults, parts[1:], value)
	default:
		return &ConfigError{
			Type:    "validation",
			Field:   path,
			Value:   value,
			Message: fmt.Sprintf("unknown configuration section: %s", parts[0]),
		}
	}
}

// getValueByPath gets a value from the configuration using dot notation.
func (cm *ConfigManager) getValueByPath(config *CLIConfig, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return nil, &ConfigError{
			Type:    "validation",
			Field:   path,
			Message: "path must have at least two parts (e.g., 'logging.level')",
		}
	}

	switch parts[0] {
	case "logging":
		return cm.getLoggingValue(&config.Logging, parts[1:])
	case "paths":
		return cm.getPathsValue(&config.Paths, parts[1:])
	case "behavior":
		return cm.getBehaviorValue(&config.Behavior, parts[1:])
	case "cluster_defaults":
		return cm.getClusterDefaultsValue(&config.ClusterDefaults, parts[1:])
	default:
		return nil, &ConfigError{
			Type:    "validation",
			Field:   path,
			Message: fmt.Sprintf("unknown configuration section: %s", parts[0]),
		}
	}
}

// setLoggingValue sets a logging configuration value.
func (cm *ConfigManager) setLoggingValue(logging *LoggingConfig, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "logging",
			Value:   value,
			Message: "missing field name in logging section",
		}
	}

	switch parts[0] {
	case "level":
		if str, ok := value.(string); ok {
			logging.Level = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "logging.level",
				Value:   value,
				Message: "level must be a string",
			}
		}
	case "format":
		if str, ok := value.(string); ok {
			logging.Format = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "logging.format",
				Value:   value,
				Message: "format must be a string",
			}
		}
	case "output":
		if str, ok := value.(string); ok {
			logging.Output = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "logging.output",
				Value:   value,
				Message: "output must be a string",
			}
		}
	case "file":
		if len(parts) < 2 {
			return &ConfigError{
				Type:    "validation",
				Field:   "logging.file",
				Value:   value,
				Message: "missing file configuration field",
			}
		}
		return cm.setFileValue(&logging.File, parts[1:], value)
	default:
		return &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("logging.%s", parts[0]),
			Value:   value,
			Message: fmt.Sprintf("unknown logging field: %s", parts[0]),
		}
	}
	return nil
}

// setFileValue sets a file configuration value.
func (cm *ConfigManager) setFileValue(file *FileConfig, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "logging.file",
			Value:   value,
			Message: "missing field name in file section",
		}
	}

	switch parts[0] {
	case "maxSize":
		if intVal, err := convertToInt(value); err == nil {
			file.MaxSize = intVal
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "logging.file.maxSize",
				Value:   value,
				Message: "maxSize must be an integer",
			}
		}
	case "maxBackups":
		if intVal, err := convertToInt(value); err == nil {
			file.MaxBackups = intVal
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "logging.file.maxBackups",
				Value:   value,
				Message: "maxBackups must be an integer",
			}
		}
	case "maxAge":
		if intVal, err := convertToInt(value); err == nil {
			file.MaxAge = intVal
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "logging.file.maxAge",
				Value:   value,
				Message: "maxAge must be an integer",
			}
		}
	case "compress":
		if boolVal, err := convertToBool(value); err == nil {
			file.Compress = boolVal
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "logging.file.compress",
				Value:   value,
				Message: "compress must be a boolean",
			}
		}
	default:
		return &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("logging.file.%s", parts[0]),
			Value:   value,
			Message: fmt.Sprintf("unknown file field: %s", parts[0]),
		}
	}
	return nil
}

// setPathsValue sets a paths configuration value.
func (cm *ConfigManager) setPathsValue(paths *PathsConfig, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "paths",
			Value:   value,
			Message: "missing field name in paths section",
		}
	}

	switch parts[0] {
	case "settingsDir":
		if str, ok := value.(string); ok {
			paths.SettingsDir = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "paths.settingsDir",
				Value:   value,
				Message: "configDir must be a string",
			}
		}
	case "clustersDir":
		if str, ok := value.(string); ok {
			paths.ClustersDir = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "paths.clustersDir",
				Value:   value,
				Message: "clustersDir must be a string",
			}
		}
	case "gitopsDir":
		if str, ok := value.(string); ok {
			paths.GitOpsDir = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "paths.gitopsDir",
				Value:   value,
				Message: "gitopsDir must be a string",
			}
		}
	case "clusterStateDir":
		if str, ok := value.(string); ok {
			paths.ClusterStateDir = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "paths.clusterStateDir",
				Value:   value,
				Message: "clusterStateDir must be a string",
			}
		}
	case "secretsDir":
		if str, ok := value.(string); ok {
			paths.SecretsDir = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "paths.secretsDir",
				Value:   value,
				Message: "secretsDir must be a string",
			}
		}
	case "pluginsDir":
		if str, ok := value.(string); ok {
			paths.PluginsDir = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "paths.pluginsDir",
				Value:   value,
				Message: "pluginsDir must be a string",
			}
		}
	case "stateDir":
		if str, ok := value.(string); ok {
			paths.StateDir = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "paths.stateDir",
				Value:   value,
				Message: "stateDir must be a string",
			}
		}
	default:
		return &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("paths.%s", parts[0]),
			Value:   value,
			Message: fmt.Sprintf("unknown paths field: %s", parts[0]),
		}
	}
	return nil
}

// setBehaviorValue sets a behavior configuration value.
func (cm *ConfigManager) setBehaviorValue(behavior *BehaviorConfig, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "behavior",
			Value:   value,
			Message: "missing field name in behavior section",
		}
	}

	switch parts[0] {
	case "autoConfirm":
		if boolVal, err := convertToBool(value); err == nil {
			behavior.AutoConfirm = boolVal
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "behavior.autoConfirm",
				Value:   value,
				Message: "autoConfirm must be a boolean",
			}
		}
	case "dryRun":
		if boolVal, err := convertToBool(value); err == nil {
			behavior.DryRun = boolVal
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "behavior.dryRun",
				Value:   value,
				Message: "dryRun must be a boolean",
			}
		}
	case "validation":
		str, ok := value.(string)
		if !ok {
			return &ConfigError{
				Type:    "validation",
				Field:   "behavior.validation",
				Value:   value,
				Message: "validation must be a string",
			}
		}
		str = strings.ToLower(strings.TrimSpace(str))
		if err := ValidateBehaviorValidationMode(str); err != nil {
			return &ConfigError{
				Type:    "validation",
				Field:   "behavior.validation",
				Value:   value,
				Message: err.Error(),
			}
		}
		behavior.Validation = str
	default:
		return &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("behavior.%s", parts[0]),
			Value:   value,
			Message: fmt.Sprintf("unknown behavior field: %s", parts[0]),
		}
	}
	return nil
}

// setClusterDefaultsValue sets a cluster defaults configuration value.
func (cm *ConfigManager) setClusterDefaultsValue(defaults *ClusterDefaultsConfig, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "cluster_defaults",
			Value:   value,
			Message: "missing field name in cluster_defaults section",
		}
	}

	switch parts[0] {
	case "provider":
		if str, ok := value.(string); ok {
			defaults.Provider = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.provider",
				Value:   value,
				Message: "provider must be a string",
			}
		}
	case "region":
		if str, ok := value.(string); ok {
			defaults.Region = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.region",
				Value:   value,
				Message: "region must be a string",
			}
		}
	case "environment":
		if str, ok := value.(string); ok {
			defaults.Environment = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.environment",
				Value:   value,
				Message: "environment must be a string",
			}
		}
	case "gitops_auth_method":
		str, ok := value.(string)
		if !ok {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.gitops_auth_method",
				Value:   value,
				Message: "gitops_auth_method must be a string",
			}
		}
		str = strings.ToLower(strings.TrimSpace(str))
		if err := ValidateGitopsAuthMethod(str); err != nil {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.gitops_auth_method",
				Value:   value,
				Message: err.Error(),
			}
		}
		defaults.GitopsAuthMethod = str
	case "base_domain":
		if str, ok := value.(string); ok {
			defaults.BaseDomain = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.base_domain",
				Value:   value,
				Message: "base_domain must be a string",
			}
		}
	case "admin_email":
		if str, ok := value.(string); ok {
			defaults.AdminEmail = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.admin_email",
				Value:   value,
				Message: "admin_email must be a string",
			}
		}
	case "kubernetes_version":
		if str, ok := value.(string); ok {
			defaults.KubernetesVersion = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.kubernetes_version",
				Value:   value,
				Message: "kubernetes_version must be a string",
			}
		}
	case "cni":
		if str, ok := value.(string); ok {
			defaults.CNI = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.cni",
				Value:   value,
				Message: "cni must be a string",
			}
		}
	case "ssh_user":
		if str, ok := value.(string); ok {
			defaults.SSHUser = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.ssh_user",
				Value:   value,
				Message: "ssh_user must be a string",
			}
		}
	case "ssh_authorized_keys":
		// Handle both string (single key) and []string (multiple keys)
		switch v := value.(type) {
		case string:
			defaults.SSHAuthorizedKeys = []string{v}
		case []string:
			defaults.SSHAuthorizedKeys = v
		case []interface{}:
			// Convert []interface{} to []string
			keys := make([]string, len(v))
			for i, item := range v {
				if str, ok := item.(string); ok {
					keys[i] = str
				} else {
					return &ConfigError{
						Type:    "validation",
						Field:   "cluster_defaults.ssh_authorized_keys",
						Value:   value,
						Message: "ssh_authorized_keys must be a string or array of strings",
					}
				}
			}
			defaults.SSHAuthorizedKeys = keys
		default:
			return &ConfigError{
				Type:    "validation",
				Field:   "cluster_defaults.ssh_authorized_keys",
				Value:   value,
				Message: "ssh_authorized_keys must be a string or array of strings",
			}
		}
	default:
		return &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("cluster_defaults.%s", parts[0]),
			Value:   value,
			Message: fmt.Sprintf("unknown cluster_defaults field: %s", parts[0]),
		}
	}
	return nil
}

// getLoggingValue gets a logging configuration value.
func (cm *ConfigManager) getLoggingValue(logging *LoggingConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return logging, nil
	}

	switch parts[0] {
	case "level":
		return logging.Level, nil
	case "format":
		return logging.Format, nil
	case "output":
		return logging.Output, nil
	case "file":
		if len(parts) == 1 {
			return logging.File, nil
		}
		return cm.getFileValue(&logging.File, parts[1:])
	default:
		return nil, &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("logging.%s", parts[0]),
			Message: fmt.Sprintf("unknown logging field: %s", parts[0]),
		}
	}
}

// getFileValue gets a file configuration value.
func (cm *ConfigManager) getFileValue(file *FileConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return file, nil
	}

	switch parts[0] {
	case "maxSize":
		return file.MaxSize, nil
	case "maxBackups":
		return file.MaxBackups, nil
	case "maxAge":
		return file.MaxAge, nil
	case "compress":
		return file.Compress, nil
	default:
		return nil, &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("logging.file.%s", parts[0]),
			Message: fmt.Sprintf("unknown file field: %s", parts[0]),
		}
	}
}

// getPathsValue gets a paths configuration value.
func (cm *ConfigManager) getPathsValue(paths *PathsConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return paths, nil
	}

	switch parts[0] {
	case "settingsDir":
		return paths.SettingsDir, nil
	case "clustersDir":
		return paths.ClustersDir, nil
	case "gitopsDir":
		return paths.GitOpsDir, nil
	case "clusterStateDir":
		return paths.ClusterStateDir, nil
	case "secretsDir":
		return paths.SecretsDir, nil
	case "pluginsDir":
		return paths.PluginsDir, nil
	case "stateDir":
		return paths.StateDir, nil
	default:
		return nil, &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("paths.%s", parts[0]),
			Message: fmt.Sprintf("unknown paths field: %s", parts[0]),
		}
	}
}

// getBehaviorValue gets a behavior configuration value.
func (cm *ConfigManager) getBehaviorValue(behavior *BehaviorConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return behavior, nil
	}

	switch parts[0] {
	case "autoConfirm":
		return behavior.AutoConfirm, nil
	case "dryRun":
		return behavior.DryRun, nil
	case "validation":
		return behavior.Validation, nil
	default:
		return nil, &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("behavior.%s", parts[0]),
			Message: fmt.Sprintf("unknown behavior field: %s", parts[0]),
		}
	}
}

// getDefaultsValue gets a defaults configuration value.
func (cm *ConfigManager) getClusterDefaultsValue(defaults *ClusterDefaultsConfig, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return defaults, nil
	}

	switch parts[0] {
	case "provider":
		return defaults.Provider, nil
	case "region":
		return defaults.Region, nil
	case "environment":
		return defaults.Environment, nil
	case "gitops_auth_method":
		return defaults.GitopsAuthMethod, nil
	case "ssh_authorized_keys":
		return defaults.SSHAuthorizedKeys, nil
	case "base_domain":
		return defaults.BaseDomain, nil
	case "admin_email":
		return defaults.AdminEmail, nil
	case "kubernetes_version":
		return defaults.KubernetesVersion, nil
	case "cni":
		return defaults.CNI, nil
	case "ssh_user":
		return defaults.SSHUser, nil
	default:
		return nil, &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("cluster_defaults.%s", parts[0]),
			Message: fmt.Sprintf("unknown cluster_defaults field: %s", parts[0]),
		}
	}
}

// Validate validates the CLI configuration and returns the first error found.
func (cv *ConfigValidator) Validate(config *CLIConfig) error {
	result := cv.ValidateWithResult(config)
	if !result.Valid && len(result.Errors) > 0 {
		return result.Errors[0]
	}
	return nil
}

// ValidateWithResult performs comprehensive validation and returns detailed results.
func (cv *ConfigValidator) ValidateWithResult(config *CLIConfig) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []*ConfigError{},
		Warnings: []*ConfigError{},
		Repaired: []*ConfigError{},
	}

	// Validate logging configuration
	cv.validateLoggingWithResult(&config.Logging, result)

	// Validate paths configuration
	cv.validatePathsWithResult(&config.Paths, result)

	// Validate behavior configuration
	cv.validateBehaviorWithResult(&config.Behavior, result)

	// Validate cluster defaults configuration
	cv.validateClusterDefaultsWithResult(&config.ClusterDefaults, result)

	// Validate dependencies
	cv.validateDependenciesWithResult(config, result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	return result
}

// validateLoggingWithResult validates the logging configuration with auto-repair support.
func (cv *ConfigValidator) validateLoggingWithResult(logging *LoggingConfig, result *ValidationResult) {
	defaults := DefaultCLIConfig()

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, logging.Level) {
		if cv.autoRepair {
			logging.Level = defaults.Logging.Level
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "logging.level",
				Value:    logging.Level,
				Message:  fmt.Sprintf("invalid log level, repaired to default '%s'", defaults.Logging.Level),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "logging.level",
				Value:   logging.Level,
				Message: fmt.Sprintf("invalid log level '%s', must be one of: %s", logging.Level, strings.Join(validLevels, ", ")),
			})
		}
	}

	// Validate log format
	validFormats := []string{"text", "json", "yaml"}
	if !contains(validFormats, logging.Format) {
		if cv.autoRepair {
			logging.Format = defaults.Logging.Format
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "logging.format",
				Value:    logging.Format,
				Message:  fmt.Sprintf("invalid log format, repaired to default '%s'", defaults.Logging.Format),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "logging.format",
				Value:   logging.Format,
				Message: fmt.Sprintf("invalid log format '%s', must be one of: %s", logging.Format, strings.Join(validFormats, ", ")),
			})
		}
	}

	// Validate log output
	validOutputs := []string{"stdout", "stderr"}
	if !contains(validOutputs, logging.Output) && logging.Output != "" {
		// Check if it's a valid file path
		if err := cv.validateFilePath(logging.Output); err != nil {
			if cv.autoRepair {
				logging.Output = defaults.Logging.Output
				result.Repaired = append(result.Repaired, &ConfigError{
					Type:     "validation",
					Field:    "logging.output",
					Value:    logging.Output,
					Message:  fmt.Sprintf("invalid output path, repaired to default '%s'", defaults.Logging.Output),
					Repaired: true,
				})
			} else {
				result.Errors = append(result.Errors, &ConfigError{
					Type:    "validation",
					Field:   "logging.output",
					Value:   logging.Output,
					Message: fmt.Sprintf("invalid output '%s', must be 'stdout', 'stderr', or a valid file path: %v", logging.Output, err),
				})
			}
		}
	}

	// Validate file configuration
	if logging.File.MaxSize <= 0 {
		if cv.autoRepair {
			logging.File.MaxSize = defaults.Logging.File.MaxSize
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "logging.file.maxSize",
				Value:    logging.File.MaxSize,
				Message:  fmt.Sprintf("invalid maxSize, repaired to default %d", defaults.Logging.File.MaxSize),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "logging.file.maxSize",
				Value:   logging.File.MaxSize,
				Message: "maxSize must be greater than 0",
			})
		}
	}

	if logging.File.MaxBackups < 0 {
		if cv.autoRepair {
			logging.File.MaxBackups = defaults.Logging.File.MaxBackups
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "logging.file.maxBackups",
				Value:    logging.File.MaxBackups,
				Message:  fmt.Sprintf("invalid maxBackups, repaired to default %d", defaults.Logging.File.MaxBackups),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "logging.file.maxBackups",
				Value:   logging.File.MaxBackups,
				Message: "maxBackups must be greater than or equal to 0",
			})
		}
	}

	if logging.File.MaxAge < 0 {
		if cv.autoRepair {
			logging.File.MaxAge = defaults.Logging.File.MaxAge
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "logging.file.maxAge",
				Value:    logging.File.MaxAge,
				Message:  fmt.Sprintf("invalid maxAge, repaired to default %d", defaults.Logging.File.MaxAge),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "logging.file.maxAge",
				Value:   logging.File.MaxAge,
				Message: "maxAge must be greater than or equal to 0",
			})
		}
	}
}

// validatePathsWithResult validates the paths configuration with auto-repair support.
func (cv *ConfigValidator) validatePathsWithResult(paths *PathsConfig, result *ValidationResult) {
	defaults := DefaultCLIConfig()

	// Validate config directory
	if paths.SettingsDir == "" {
		if cv.autoRepair {
			paths.SettingsDir = defaults.Paths.SettingsDir
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "paths.settingsDir",
				Value:    paths.SettingsDir,
				Message:  fmt.Sprintf("empty configDir, repaired to default '%s'", defaults.Paths.SettingsDir),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "paths.settingsDir",
				Value:   paths.SettingsDir,
				Message: "configDir cannot be empty",
			})
		}
	} else {
		// Validate that the path is accessible
		expandedPath := corePaths.ExpandPath(paths.SettingsDir)
		if err := cv.validateDirectoryPath(expandedPath); err != nil {
			if cv.autoRepair {
				// Try to create the directory
				if createErr := os.MkdirAll(expandedPath, 0755); createErr != nil {
					result.Errors = append(result.Errors, &ConfigError{
						Type:    "permission",
						Field:   "paths.settingsDir",
						Value:   paths.SettingsDir,
						Message: fmt.Sprintf("cannot create configDir '%s': %v", expandedPath, createErr),
					})
				} else {
					result.Repaired = append(result.Repaired, &ConfigError{
						Type:     "permission",
						Field:    "paths.settingsDir",
						Value:    paths.SettingsDir,
						Message:  fmt.Sprintf("created missing configDir '%s'", expandedPath),
						Repaired: true,
					})
				}
			} else {
				// For non-auto-repair mode, only warn about missing directories
				result.Warnings = append(result.Warnings, &ConfigError{
					Type:    "path",
					Field:   "paths.settingsDir",
					Value:   paths.SettingsDir,
					Message: fmt.Sprintf("configDir path may not be accessible: %v", err),
				})
			}
		}
	}

	// Validate clusters directory
	if paths.ClustersDir == "" {
		if cv.autoRepair {
			paths.ClustersDir = defaults.Paths.ClustersDir
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "paths.clustersDir",
				Value:    paths.ClustersDir,
				Message:  fmt.Sprintf("empty clustersDir, repaired to default '%s'", defaults.Paths.ClustersDir),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "paths.clustersDir",
				Value:   paths.ClustersDir,
				Message: "clustersDir cannot be empty",
			})
		}
	} else {
		// Validate that the path is accessible
		expandedPath := corePaths.ExpandPath(paths.ClustersDir)
		if err := cv.validateDirectoryPath(expandedPath); err != nil {
			if cv.autoRepair {
				// Try to create the directory
				if createErr := os.MkdirAll(expandedPath, 0755); createErr != nil {
					result.Errors = append(result.Errors, &ConfigError{
						Type:    "permission",
						Field:   "paths.clustersDir",
						Value:   paths.ClustersDir,
						Message: fmt.Sprintf("cannot create clustersDir '%s': %v", expandedPath, createErr),
					})
				} else {
					result.Repaired = append(result.Repaired, &ConfigError{
						Type:     "permission",
						Field:    "paths.clustersDir",
						Value:    paths.ClustersDir,
						Message:  fmt.Sprintf("created missing clustersDir '%s'", expandedPath),
						Repaired: true,
					})
				}
			} else {
				// For non-auto-repair mode, only warn about missing directories
				result.Warnings = append(result.Warnings, &ConfigError{
					Type:    "path",
					Field:   "paths.clustersDir",
					Value:   paths.ClustersDir,
					Message: fmt.Sprintf("clustersDir path may not be accessible: %v", err),
				})
			}
		}
	}

	validateZonePath := func(field string, value *string, defaultValue string) {
		if *value == "" {
			if cv.autoRepair {
				*value = defaultValue
				result.Repaired = append(result.Repaired, &ConfigError{
					Type:     "validation",
					Field:    field,
					Value:    *value,
					Message:  fmt.Sprintf("empty %s, repaired to default '%s'", field, defaultValue),
					Repaired: true,
				})
			} else {
				result.Errors = append(result.Errors, &ConfigError{
					Type:    "validation",
					Field:   field,
					Value:   *value,
					Message: fmt.Sprintf("%s cannot be empty", strings.TrimPrefix(field, "paths.")),
				})
			}
			return
		}

		expandedPath := corePaths.ExpandPath(*value)
		if stat, err := os.Stat(expandedPath); err == nil {
			if !stat.IsDir() {
				result.Errors = append(result.Errors, &ConfigError{
					Type:    "path",
					Field:   field,
					Value:   *value,
					Message: fmt.Sprintf("%s path exists but is not a directory", field),
				})
			}
		} else if err != nil && !os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, &ConfigError{
				Type:    "path",
				Field:   field,
				Value:   *value,
				Message: fmt.Sprintf("%s path may not be accessible: %v", field, err),
			})
		}
	}

	validateZonePath("paths.gitopsDir", &paths.GitOpsDir, defaults.Paths.GitOpsDir)
	validateZonePath("paths.clusterStateDir", &paths.ClusterStateDir, defaults.Paths.ClusterStateDir)
	validateZonePath("paths.secretsDir", &paths.SecretsDir, defaults.Paths.SecretsDir)

	if paths.GitOpsDir != "" && paths.ClusterStateDir != "" && paths.SecretsDir != "" {
		samplePaths := corePaths.ClusterPaths{
			GitOpsDir:       paths.GitOpsDir,
			ClusterStateDir: paths.ClusterStateDir,
			SecretsDir:      paths.SecretsDir,
			ConfigPath:      filepath.Join(paths.ClusterStateDir, "sample-config.yaml"),
			SOPSKeyPath:     filepath.Join(paths.SecretsDir, "age", "keys", "sample-key.txt"),
			SSHKeyPath:      filepath.Join(paths.SecretsDir, "ssh", "sample"),
		}
		if err := samplePaths.Validate(); err != nil {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "paths",
				Value:   paths,
				Message: err.Error(),
			})
		}
	}

	// Validate plugins directory
	if paths.PluginsDir == "" {
		if cv.autoRepair {
			paths.PluginsDir = defaults.Paths.PluginsDir
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "paths.pluginsDir",
				Value:    paths.PluginsDir,
				Message:  fmt.Sprintf("empty pluginsDir, repaired to default '%s'", defaults.Paths.PluginsDir),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "paths.pluginsDir",
				Value:   paths.PluginsDir,
				Message: "pluginsDir cannot be empty",
			})
		}
	} else {
		// Validate that the path is accessible
		expandedPath := corePaths.ExpandPath(paths.PluginsDir)
		if err := cv.validateDirectoryPath(expandedPath); err != nil {
			if cv.autoRepair {
				// Try to create the directory
				if createErr := os.MkdirAll(expandedPath, 0755); createErr != nil {
					result.Errors = append(result.Errors, &ConfigError{
						Type:    "permission",
						Field:   "paths.pluginsDir",
						Value:   paths.PluginsDir,
						Message: fmt.Sprintf("cannot create pluginsDir '%s': %v", expandedPath, createErr),
					})
				} else {
					result.Repaired = append(result.Repaired, &ConfigError{
						Type:     "permission",
						Field:    "paths.pluginsDir",
						Value:    paths.PluginsDir,
						Message:  fmt.Sprintf("created missing pluginsDir '%s'", expandedPath),
						Repaired: true,
					})
				}
			} else {
				// For non-auto-repair mode, only warn about missing directories
				result.Warnings = append(result.Warnings, &ConfigError{
					Type:    "path",
					Field:   "paths.pluginsDir",
					Value:   paths.PluginsDir,
					Message: fmt.Sprintf("pluginsDir path may not be accessible: %v", err),
				})
			}
		}
	}

	// Validate state directory
	if paths.StateDir == "" {
		if cv.autoRepair {
			paths.StateDir = defaults.Paths.StateDir
			result.Repaired = append(result.Repaired, &ConfigError{
				Type:     "validation",
				Field:    "paths.stateDir",
				Value:    paths.StateDir,
				Message:  fmt.Sprintf("empty stateDir, repaired to default '%s'", defaults.Paths.StateDir),
				Repaired: true,
			})
		} else {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "validation",
				Field:   "paths.stateDir",
				Value:   paths.StateDir,
				Message: "stateDir cannot be empty",
			})
		}
	} else {
		expandedPath := corePaths.ExpandPath(paths.StateDir)
		if err := cv.validateDirectoryPath(expandedPath); err != nil {
			if cv.autoRepair {
				if createErr := os.MkdirAll(expandedPath, 0755); createErr != nil {
					result.Errors = append(result.Errors, &ConfigError{
						Type:    "permission",
						Field:   "paths.stateDir",
						Value:   paths.StateDir,
						Message: fmt.Sprintf("cannot create stateDir '%s': %v", expandedPath, createErr),
					})
				} else {
					result.Repaired = append(result.Repaired, &ConfigError{
						Type:     "permission",
						Field:    "paths.stateDir",
						Value:    paths.StateDir,
						Message:  fmt.Sprintf("created missing stateDir '%s'", expandedPath),
						Repaired: true,
					})
				}
			} else {
				result.Warnings = append(result.Warnings, &ConfigError{
					Type:    "path",
					Field:   "paths.stateDir",
					Value:   paths.StateDir,
					Message: fmt.Sprintf("stateDir path may not be accessible: %v", err),
				})
			}
		}
	}
}

// validateBehaviorWithResult validates the behavior configuration.
func (cv *ConfigValidator) validateBehaviorWithResult(behavior *BehaviorConfig, result *ValidationResult) {
	// Behavior configuration is mostly boolean values, so validation is minimal
	// We could add warnings for potentially problematic combinations
	if behavior.AutoConfirm && !behavior.DryRun {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "validation",
			Field:   "behavior.autoConfirm",
			Value:   behavior.AutoConfirm,
			Message: "autoConfirm is enabled without dryRun, this may lead to unintended actions",
		})
	}
	if behavior.Validation == "" {
		behavior.Validation = ValidationModeOffline
	}
	if err := ValidateBehaviorValidationMode(behavior.Validation); err != nil {
		result.Errors = append(result.Errors, &ConfigError{
			Type:    "validation",
			Field:   "behavior.validation",
			Value:   behavior.Validation,
			Message: err.Error(),
		})
	}
}

func ValidateBehaviorValidationMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case ValidationModeOffline, ValidationModeOnline:
		return nil
	default:
		return fmt.Errorf("invalid behavior.validation %q; expected offline or online", mode)
	}
}

func ValidateGitopsAuthMethod(method string) error {
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case GitopsAuthMethodSSH, GitopsAuthMethodToken:
		return nil
	default:
		return fmt.Errorf("invalid cluster_defaults.gitops_auth_method %q; expected ssh or token", method)
	}
}

// validateDefaultsWithResult validates the defaults configuration.
func (cv *ConfigValidator) validateClusterDefaultsWithResult(defaults *ClusterDefaultsConfig, result *ValidationResult) {
	// Validate provider
	validProviders := []string{"openstack", "aws", "azure", "gcp", "kind", "vmware", "baremetal"}
	if defaults.Provider != "" && !contains(validProviders, defaults.Provider) {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "validation",
			Field:   "cluster_defaults.provider",
			Value:   defaults.Provider,
			Message: fmt.Sprintf("unknown provider '%s', supported providers: %s", defaults.Provider, strings.Join(validProviders, ", ")),
		})
	}

	// Validate region format (basic check)
	if defaults.Region != "" && len(defaults.Region) < 2 {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "validation",
			Field:   "cluster_defaults.region",
			Value:   defaults.Region,
			Message: "region appears to be too short, verify it's a valid region identifier",
		})
	}

	// Validate environment
	commonEnvs := []string{"dev", "test", "stage", "staging", "prod", "production"}
	if defaults.Environment != "" && !contains(commonEnvs, defaults.Environment) {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "validation",
			Field:   "cluster_defaults.environment",
			Value:   defaults.Environment,
			Message: fmt.Sprintf("uncommon environment '%s', common environments: %s", defaults.Environment, strings.Join(commonEnvs, ", ")),
		})
	}

	if defaults.GitopsAuthMethod == "" {
		defaults.GitopsAuthMethod = GitopsAuthMethodToken
	}
	if err := ValidateGitopsAuthMethod(defaults.GitopsAuthMethod); err != nil {
		result.Errors = append(result.Errors, &ConfigError{
			Type:    "validation",
			Field:   "cluster_defaults.gitops_auth_method",
			Value:   defaults.GitopsAuthMethod,
			Message: err.Error(),
		})
	}

	// Validate CNI
	validCNIs := []string{"calico", "cilium", "kube-ovn"}
	if defaults.CNI != "" && !contains(validCNIs, defaults.CNI) {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "validation",
			Field:   "cluster_defaults.cni",
			Value:   defaults.CNI,
			Message: fmt.Sprintf("unknown CNI '%s', supported CNIs: %s", defaults.CNI, strings.Join(validCNIs, ", ")),
		})
	}
}

// validateDependenciesWithResult validates system dependencies and requirements.
func (cv *ConfigValidator) validateDependenciesWithResult(config *CLIConfig, result *ValidationResult) {
	// Check if required directories are accessible
	expandedConfigDir := corePaths.ExpandPath(config.Paths.SettingsDir)
	expandedClustersDir := corePaths.ExpandPath(config.Paths.ClustersDir)
	expandedGitOpsDir := corePaths.ExpandPath(config.Paths.GitOpsDir)
	expandedClusterStateDir := corePaths.ExpandPath(config.Paths.ClusterStateDir)
	expandedSecretsDir := corePaths.ExpandPath(config.Paths.SecretsDir)
	expandedPluginsDir := corePaths.ExpandPath(config.Paths.PluginsDir)
	expandedStateDir := corePaths.ExpandPath(config.Paths.StateDir)

	// Check disk space for config directory
	if err := cv.checkDiskSpace(expandedConfigDir); err != nil {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "dependency",
			Field:   "paths.settingsDir",
			Value:   expandedConfigDir,
			Message: fmt.Sprintf("disk space warning for configDir: %v", err),
		})
	}

	// Check disk space for clusters directory
	if err := cv.checkDiskSpace(expandedClustersDir); err != nil {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "dependency",
			Field:   "paths.clustersDir",
			Value:   expandedClustersDir,
			Message: fmt.Sprintf("disk space warning for clustersDir: %v", err),
		})
	}
	if err := cv.checkDiskSpaceForExistingParent(expandedGitOpsDir); err != nil {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "dependency",
			Field:   "paths.gitopsDir",
			Value:   expandedGitOpsDir,
			Message: fmt.Sprintf("disk space warning for gitopsDir: %v", err),
		})
	}
	if err := cv.checkDiskSpaceForExistingParent(expandedClusterStateDir); err != nil {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "dependency",
			Field:   "paths.clusterStateDir",
			Value:   expandedClusterStateDir,
			Message: fmt.Sprintf("disk space warning for clusterStateDir: %v", err),
		})
	}
	if err := cv.checkDiskSpaceForExistingParent(expandedSecretsDir); err != nil {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "dependency",
			Field:   "paths.secretsDir",
			Value:   expandedSecretsDir,
			Message: fmt.Sprintf("disk space warning for secretsDir: %v", err),
		})
	}

	// Check disk space for plugins directory
	if err := cv.checkDiskSpace(expandedPluginsDir); err != nil {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "dependency",
			Field:   "paths.pluginsDir",
			Value:   expandedPluginsDir,
			Message: fmt.Sprintf("disk space warning for pluginsDir: %v", err),
		})
	}
	if err := cv.checkDiskSpace(expandedStateDir); err != nil {
		result.Warnings = append(result.Warnings, &ConfigError{
			Type:    "dependency",
			Field:   "paths.stateDir",
			Value:   expandedStateDir,
			Message: fmt.Sprintf("disk space warning for stateDir: %v", err),
		})
	}

	// Check if logging output file is writable (if it's a file path)
	validOutputs := []string{"stdout", "stderr"}
	if !contains(validOutputs, config.Logging.Output) && config.Logging.Output != "" {
		if err := cv.validateFilePath(config.Logging.Output); err != nil {
			result.Errors = append(result.Errors, &ConfigError{
				Type:    "permission",
				Field:   "logging.output",
				Value:   config.Logging.Output,
				Message: fmt.Sprintf("cannot write to log file: %v", err),
			})
		}
	}
}

// validateDirectoryPath validates that a directory path exists or can be created.
func (cv *ConfigValidator) validateDirectoryPath(path string) error {
	// Check if directory exists
	if stat, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist, check if we can create it
			return fmt.Errorf("directory does not exist: %s", path)
		}
		return fmt.Errorf("cannot access directory: %w", err)
	} else if !stat.IsDir() {
		return fmt.Errorf("path exists but is not a directory: %s", path)
	}

	// Check if directory is writable
	testFile := filepath.Join(path, ".opencenter_write_test")
	if file, err := os.Create(testFile); err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	} else {
		file.Close()
		os.Remove(testFile)
	}

	return nil
}

// validateFilePath validates that a file path is accessible for writing.
func (cv *ConfigValidator) validateFilePath(path string) error {
	// Expand the path
	expandedPath := corePaths.ExpandPath(path)

	// Check if the directory exists or can be created
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}

	// Try to create/open the file for writing
	file, err := os.OpenFile(expandedPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("cannot write to file %s: %w", expandedPath, err)
	}
	file.Close()

	return nil
}

// checkDiskSpace checks if there's sufficient disk space in the given directory.
func (cv *ConfigValidator) checkDiskSpace(path string) error {
	// This is a basic implementation - in a real system you might want to use
	// syscalls to get actual disk space information
	if stat, err := os.Stat(path); err != nil {
		return fmt.Errorf("cannot check disk space: %w", err)
	} else if !stat.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	// For now, just check if we can write a small test file
	testFile := filepath.Join(path, ".opencenter_space_test")
	if file, err := os.Create(testFile); err != nil {
		return fmt.Errorf("insufficient disk space or permissions: %w", err)
	} else {
		file.Close()
		os.Remove(testFile)
	}

	return nil
}

func (cv *ConfigValidator) checkDiskSpaceForExistingParent(path string) error {
	current := filepath.Clean(path)
	for {
		if stat, err := os.Stat(current); err == nil && stat.IsDir() {
			return cv.checkDiskSpace(current)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot check disk space: %w", err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("cannot check disk space: no existing parent for %s", path)
		}
		current = parent
	}
}

// convertToInt converts various types to int.
func convertToInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// convertToBool converts various types to bool.
func convertToBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
