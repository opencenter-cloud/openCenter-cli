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

	"gopkg.in/yaml.v3"
)

// CLIConfig represents the global CLI configuration system that controls
// CLI behavior, logging, and default paths.
type CLIConfig struct {
	Logging  LoggingConfig  `yaml:"logging"`
	Paths    PathsConfig    `yaml:"paths"`
	Behavior BehaviorConfig `yaml:"behavior"`
	Defaults DefaultsConfig `yaml:"defaults"`
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
	MaxSize    int  `yaml:"maxSize"`    // MB
	MaxBackups int  `yaml:"maxBackups"`
	MaxAge     int  `yaml:"maxAge"`     // days
	Compress   bool `yaml:"compress"`
}

// PathsConfig controls default paths for configuration and clusters.
type PathsConfig struct {
	ConfigDir   string `yaml:"configDir"`
	ClustersDir string `yaml:"clustersDir"`
}

// BehaviorConfig controls CLI behavior settings.
type BehaviorConfig struct {
	AutoConfirm bool `yaml:"autoConfirm"`
	DryRun      bool `yaml:"dryRun"`
	Verbose     bool `yaml:"verbose"`
}

// DefaultsConfig contains default values for cluster operations.
type DefaultsConfig struct {
	Provider    string `yaml:"provider"`
	Region      string `yaml:"region"`
	Environment string `yaml:"environment"`
}

// ConfigManager handles CLI configuration loading, validation, and merging.
type ConfigManager struct {
	configPath string
	config     *CLIConfig
	defaults   *CLIConfig
	validator  *ConfigValidator
}

// ConfigValidator validates configuration values and structure.
type ConfigValidator struct{}

// ConfigError represents configuration-related errors with actionable messages.
type ConfigError struct {
	Type    string      // validation, permission, path, format
	Field   string      // dot notation path to problematic field
	Value   interface{}
	Message string
}

func (ce *ConfigError) Error() string {
	if ce.Field != "" {
		return fmt.Sprintf("%s error in field '%s': %s", ce.Type, ce.Field, ce.Message)
	}
	return fmt.Sprintf("%s error: %s", ce.Type, ce.Message)
}

// Suggestions returns actionable suggestions for fixing the configuration error.
func (ce *ConfigError) Suggestions() []string {
	switch ce.Type {
	case "validation":
		switch ce.Field {
		case "logging.level":
			return []string{"Use one of: debug, info, warn, error"}
		case "logging.format":
			return []string{"Use one of: text, json, yaml"}
		case "logging.output":
			return []string{"Use 'stdout', 'stderr', or a valid file path"}
		}
	case "permission":
		return []string{
			"Check file/directory permissions",
			"Ensure the directory is writable",
			"Run with appropriate user permissions",
		}
	case "path":
		return []string{
			"Ensure the path exists or can be created",
			"Check for typos in the path",
			"Verify parent directories exist",
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
	configPath = ExpandPath(configPath)

	cm := &ConfigManager{
		configPath: configPath,
		defaults:   DefaultCLIConfig(),
		validator:  &ConfigValidator{},
	}

	// Load configuration
	if err := cm.Load(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return cm, nil
}

// DefaultCLIConfig returns the default CLI configuration.
func DefaultCLIConfig() *CLIConfig {
	configDir, _ := ResolveConfigDir()
	clustersDir := filepath.Join(configDir, "clusters")

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
			ConfigDir:   configDir,
			ClustersDir: clustersDir,
		},
		Behavior: BehaviorConfig{
			AutoConfirm: false,
			DryRun:      false,
			Verbose:     false,
		},
		Defaults: DefaultsConfig{
			Provider:    "openstack",
			Region:      "iad3",
			Environment: "dev",
		},
	}
}

// DefaultCLIConfigPath returns the default path for the CLI configuration file.
func DefaultCLIConfigPath() (string, error) {
	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
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

	// Read configuration file
	data, err := os.ReadFile(cm.configPath)
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

	// Validate configuration
	if err := cm.validator.Validate(cm.config); err != nil {
		return err
	}

	// Expand paths in the loaded configuration
	cm.expandConfigPaths()

	return nil
}

// createDefaultConfig creates a default configuration file.
func (cm *ConfigManager) createDefaultConfig() error {
	data, err := yaml.Marshal(cm.defaults)
	if err != nil {
		return fmt.Errorf("failed to marshal default configuration: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0600); err != nil {
		return &ConfigError{
			Type:    "permission",
			Field:   "configFile",
			Value:   cm.configPath,
			Message: fmt.Sprintf("failed to write default configuration: %v", err),
		}
	}

	return nil
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
	if config.Paths.ConfigDir != "" {
		merged.Paths.ConfigDir = config.Paths.ConfigDir
	}
	if config.Paths.ClustersDir != "" {
		merged.Paths.ClustersDir = config.Paths.ClustersDir
	}

	// Merge behavior configuration
	merged.Behavior.AutoConfirm = config.Behavior.AutoConfirm
	merged.Behavior.DryRun = config.Behavior.DryRun
	merged.Behavior.Verbose = config.Behavior.Verbose

	// Merge defaults configuration
	if config.Defaults.Provider != "" {
		merged.Defaults.Provider = config.Defaults.Provider
	}
	if config.Defaults.Region != "" {
		merged.Defaults.Region = config.Defaults.Region
	}
	if config.Defaults.Environment != "" {
		merged.Defaults.Environment = config.Defaults.Environment
	}

	return &merged
}

// expandConfigPaths expands environment variables and tilde in configuration paths.
func (cm *ConfigManager) expandConfigPaths() {
	cm.config.Paths.ConfigDir = ExpandPath(cm.config.Paths.ConfigDir)
	cm.config.Paths.ClustersDir = ExpandPath(cm.config.Paths.ClustersDir)
}

// Save saves the current configuration to the file system.
func (cm *ConfigManager) Save() error {
	data, err := yaml.Marshal(cm.config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0600); err != nil {
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
	return cm.config
}

// SetValue sets a configuration value using dot notation.
func (cm *ConfigManager) SetValue(key string, value interface{}) error {
	if err := cm.setValueByPath(cm.config, key, value); err != nil {
		return err
	}

	// Validate the updated configuration
	if err := cm.validator.Validate(cm.config); err != nil {
		return err
	}

	return nil
}

// GetValue gets a configuration value using dot notation.
func (cm *ConfigManager) GetValue(key string) (interface{}, error) {
	return cm.getValueByPath(cm.config, key)
}

// Reset resets the configuration to default values.
func (cm *ConfigManager) Reset() error {
	cm.config = DefaultCLIConfig()
	return cm.Save()
}

// GetConfigPath returns the path to the configuration file.
func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}

// LoadWithConfig loads the configuration manager with an existing configuration.
// This is useful for applying runtime overrides without modifying the file.
func (cm *ConfigManager) LoadWithConfig(config *CLIConfig) error {
	cm.config = config
	cm.validator = &ConfigValidator{}
	
	// Validate the provided configuration
	if err := cm.validator.Validate(cm.config); err != nil {
		return err
	}

	// Expand paths in the configuration
	cm.expandConfigPaths()

	return nil
}

// NewConfigManagerWithConfig creates a new configuration manager with an existing configuration.
// This is useful for creating temporary managers for applying overrides.
func NewConfigManagerWithConfig(config *CLIConfig) (*ConfigManager, error) {
	cm := &ConfigManager{
		config:    config,
		defaults:  DefaultCLIConfig(),
		validator: &ConfigValidator{},
	}

	// Validate the provided configuration
	if err := cm.validator.Validate(cm.config); err != nil {
		return nil, err
	}

	// Expand paths in the configuration
	cm.expandConfigPaths()

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
	case "defaults":
		return cm.setDefaultsValue(&config.Defaults, parts[1:], value)
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
	case "defaults":
		return cm.getDefaultsValue(&config.Defaults, parts[1:])
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
	case "configDir":
		if str, ok := value.(string); ok {
			paths.ConfigDir = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "paths.configDir",
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
	case "verbose":
		if boolVal, err := convertToBool(value); err == nil {
			behavior.Verbose = boolVal
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "behavior.verbose",
				Value:   value,
				Message: "verbose must be a boolean",
			}
		}
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

// setDefaultsValue sets a defaults configuration value.
func (cm *ConfigManager) setDefaultsValue(defaults *DefaultsConfig, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "defaults",
			Value:   value,
			Message: "missing field name in defaults section",
		}
	}

	switch parts[0] {
	case "provider":
		if str, ok := value.(string); ok {
			defaults.Provider = str
		} else {
			return &ConfigError{
				Type:    "validation",
				Field:   "defaults.provider",
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
				Field:   "defaults.region",
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
				Field:   "defaults.environment",
				Value:   value,
				Message: "environment must be a string",
			}
		}
	default:
		return &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("defaults.%s", parts[0]),
			Value:   value,
			Message: fmt.Sprintf("unknown defaults field: %s", parts[0]),
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
	case "configDir":
		return paths.ConfigDir, nil
	case "clustersDir":
		return paths.ClustersDir, nil
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
	case "verbose":
		return behavior.Verbose, nil
	default:
		return nil, &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("behavior.%s", parts[0]),
			Message: fmt.Sprintf("unknown behavior field: %s", parts[0]),
		}
	}
}

// getDefaultsValue gets a defaults configuration value.
func (cm *ConfigManager) getDefaultsValue(defaults *DefaultsConfig, parts []string) (interface{}, error) {
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
	default:
		return nil, &ConfigError{
			Type:    "validation",
			Field:   fmt.Sprintf("defaults.%s", parts[0]),
			Message: fmt.Sprintf("unknown defaults field: %s", parts[0]),
		}
	}
}

// Validate validates the CLI configuration.
func (cv *ConfigValidator) Validate(config *CLIConfig) error {
	// Validate logging configuration
	if err := cv.validateLogging(&config.Logging); err != nil {
		return err
	}

	// Validate paths configuration
	if err := cv.validatePaths(&config.Paths); err != nil {
		return err
	}

	return nil
}

// validateLogging validates the logging configuration.
func (cv *ConfigValidator) validateLogging(logging *LoggingConfig) error {
	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, logging.Level) {
		return &ConfigError{
			Type:    "validation",
			Field:   "logging.level",
			Value:   logging.Level,
			Message: fmt.Sprintf("invalid log level '%s', must be one of: %s", logging.Level, strings.Join(validLevels, ", ")),
		}
	}

	// Validate log format
	validFormats := []string{"text", "json", "yaml"}
	if !contains(validFormats, logging.Format) {
		return &ConfigError{
			Type:    "validation",
			Field:   "logging.format",
			Value:   logging.Format,
			Message: fmt.Sprintf("invalid log format '%s', must be one of: %s", logging.Format, strings.Join(validFormats, ", ")),
		}
	}

	// Validate log output
	validOutputs := []string{"stdout", "stderr"}
	if !contains(validOutputs, logging.Output) {
		// If not a standard output, check if it's a valid file path
		if logging.Output != "" {
			if err := cv.validateFilePath(logging.Output); err != nil {
				return &ConfigError{
					Type:    "validation",
					Field:   "logging.output",
					Value:   logging.Output,
					Message: fmt.Sprintf("invalid output '%s', must be 'stdout', 'stderr', or a valid file path: %v", logging.Output, err),
				}
			}
		}
	}

	// Validate file configuration
	if logging.File.MaxSize <= 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "logging.file.maxSize",
			Value:   logging.File.MaxSize,
			Message: "maxSize must be greater than 0",
		}
	}

	if logging.File.MaxBackups < 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "logging.file.maxBackups",
			Value:   logging.File.MaxBackups,
			Message: "maxBackups must be greater than or equal to 0",
		}
	}

	if logging.File.MaxAge < 0 {
		return &ConfigError{
			Type:    "validation",
			Field:   "logging.file.maxAge",
			Value:   logging.File.MaxAge,
			Message: "maxAge must be greater than or equal to 0",
		}
	}

	return nil
}

// validatePaths validates the paths configuration.
func (cv *ConfigValidator) validatePaths(paths *PathsConfig) error {
	// Validate config directory
	if paths.ConfigDir == "" {
		return &ConfigError{
			Type:    "validation",
			Field:   "paths.configDir",
			Value:   paths.ConfigDir,
			Message: "configDir cannot be empty",
		}
	}

	// Validate clusters directory
	if paths.ClustersDir == "" {
		return &ConfigError{
			Type:    "validation",
			Field:   "paths.clustersDir",
			Value:   paths.ClustersDir,
			Message: "clustersDir cannot be empty",
		}
	}

	return nil
}

// validateFilePath validates that a file path is accessible for writing.
func (cv *ConfigValidator) validateFilePath(path string) error {
	// Expand the path
	expandedPath := ExpandPath(path)

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

// ExpandPath expands environment variables and tilde in a path.
func ExpandPath(path string) string {
	// Expand environment variables
	path = os.ExpandEnv(path)

	// Expand tilde
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		}
	}

	return path
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