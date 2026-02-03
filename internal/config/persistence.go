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
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	corePaths "github.com/rackerlabs/opencenter-cli/internal/core/paths"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
	"gopkg.in/yaml.v3"
)

// ResolveConfigDir resolves the configuration directory based on the OPENCENTER_CONFIG_DIR
// environment variable. If the variable is not set, it falls back to the user's
// standard config directory (e.g., ~/.config/opencenter on Linux).
// The directory is created if it does not exist.
//
// This is the internal implementation used by internal/core/config.
func ResolveConfigDir() (string, error) {
	var err error
	dir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if dir == "" {
		// Determine OS-specific config directory
		switch runtime.GOOS {
		case "windows":
			base := os.Getenv("APPDATA")
			if base == "" {
				base = os.Getenv("LOCALAPPDATA")
			}
			if base == "" {
				base = os.Getenv("USERPROFILE")
			}
			dir = filepath.Join(base, "opencenter")
		default:
			home, herr := os.UserHomeDir()
			if herr != nil {
				err = herr
				return "", err
			}
			dir = filepath.Join(home, ".config", "opencenter")
		}
	}
	// Ensure absolute path
	if !filepath.IsAbs(dir) {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return "", err
		}
	}
	// Create directory if not exists
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		err = mkErr
		return "", err
	}
	return dir, err
}

// ParseClusterIdentifier parses a cluster identifier which can be in one of two formats:
// 1. "cluster" - just the cluster name (uses default "opencenter" organization)
// 2. "organization/cluster" - organization and cluster name
//
// Inputs:
//   - identifier: The cluster identifier to parse.
//
// Outputs:
//   - organization: The organization name (or "opencenter" if not specified).
//   - clusterName: The cluster name.
//   - error: An error if the identifier is invalid.
func ParseClusterIdentifier(identifier string) (organization string, clusterName string, err error) {
	if identifier == "" {
		return "", "", errors.New("cluster identifier cannot be empty")
	}

	// Check for organization/cluster format
	if strings.Contains(identifier, "/") {
		parts := strings.SplitN(identifier, "/", 2)
		if len(parts) != 2 {
			return "", "", errors.New("invalid cluster identifier format: expected 'organization/cluster'")
		}
		organization = parts[0]
		clusterName = parts[1]

		// Validate both parts using ValidationEngine
		ctx := context.Background()
		validator := validators.NewClusterNameValidator()
		
		result, err := validator.Validate(ctx, organization)
		if err != nil {
			return "", "", fmt.Errorf("organization name validation failed: %w", err)
		}
		if !result.Valid {
			return "", "", fmt.Errorf("invalid organization name: %s", result.Errors[0].Message)
		}
		
		result, err = validator.Validate(ctx, clusterName)
		if err != nil {
			return "", "", fmt.Errorf("cluster name validation failed: %w", err)
		}
		if !result.Valid {
			return "", "", fmt.Errorf("invalid cluster name: %s", result.Errors[0].Message)
		}

		return organization, clusterName, nil
	}

	// Just cluster name, use default organization
	ctx := context.Background()
	validator := validators.NewClusterNameValidator()
	
	result, err := validator.Validate(ctx, identifier)
	if err != nil {
		return "", "", fmt.Errorf("cluster name validation failed: %w", err)
	}
	if !result.Valid {
		return "", "", fmt.Errorf("invalid cluster name: %s", result.Errors[0].Message)
	}

	return "opencenter", identifier, nil
}



// ConfigPath returns the absolute path to a cluster's configuration file.
// It implements a fallback strategy to support both organization-based and legacy structures.
// The name parameter can be in "cluster" or "organization/cluster" format.
// If no organization is specified, it searches all organizations for the cluster.
//
// Deprecated: Use internal/core/paths.PathResolver.ResolveClusterPaths() instead.
// This function will be removed in v2.0.0.
// Migration: Replace ConfigPath(name) with pathResolver.ResolveClusterPaths(name, org) and access config path.
//
// Inputs:
//   - name: The name of the cluster (can be "cluster" or "organization/cluster").
//
// Outputs:
//   - string: The absolute path to the configuration file.
//   - error: An error if one occurred.
func ConfigPath(name string) (string, error) {
	logDeprecationWarning(
		"config.ConfigPath()",
		"internal/core/paths.PathResolver.ResolveClusterPaths()",
		"v2.0.0",
	)
	// Parse the cluster identifier to extract organization and cluster name
	organization, clusterName, err := ParseClusterIdentifier(name)
	if err != nil {
		return "", fmt.Errorf("invalid cluster identifier: %w", err)
	}

	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory: %w", err)
	}

	// Load CLI configuration to get the configured clustersDir
	cliConfigManager, err := NewConfigManager("")
	var clustersDir string
	if err == nil {
		clustersDir = cliConfigManager.GetConfig().Paths.ClustersDir
		if clustersDir == "" {
			clustersDir = filepath.Join(configDir, "clusters")
		}
		clustersDir = corePaths.ExpandPath(clustersDir)
	} else {
		clustersDir = filepath.Join(configDir, "clusters")
	}

	// Priority 1: If organization was explicitly specified, check organization-based paths
	if strings.Contains(name, "/") {
		// Build organization-based paths directly
		organizationDir := filepath.Join(clustersDir, organization)
		clusterDir := filepath.Join(organizationDir, "infrastructure", "clusters", clusterName)

		// Check for config file at organization level (primary location)
		orgConfigPath := filepath.Join(organizationDir, "."+clusterName+"-config.yaml")
		if _, statErr := os.Stat(orgConfigPath); statErr == nil {
			return orgConfigPath, nil
		}

		// Check for config file at cluster directory level (alternative location)
		clusterConfigPath := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")
		if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
			return clusterConfigPath, nil
		}

		// If explicitly specified org/cluster not found, return error
		return "", fmt.Errorf("cluster configuration file not found for cluster %s", name)
	}

	// Priority 2: No organization specified - search organization-based paths first
	if entries, readErr := os.ReadDir(clustersDir); readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				orgName := entry.Name()

				// Check for config file at organization level (primary location)
				orgConfigPath := filepath.Join(clustersDir, orgName, "."+clusterName+"-config.yaml")
				if _, statErr := os.Stat(orgConfigPath); statErr == nil {
					return orgConfigPath, nil
				}

				// Check for config file at cluster directory level (alternative location)
				clusterConfigPath := filepath.Join(clustersDir, orgName, "infrastructure", "clusters", clusterName, "."+clusterName+"-config.yaml")
				if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
					return clusterConfigPath, nil
				}
			}
		}
	}

	// Priority 3: Check for flat config file (backward compatibility)
	flatConfigPath := filepath.Join(configDir, clusterName+".yaml")
	if _, statErr := os.Stat(flatConfigPath); statErr == nil {
		return flatConfigPath, nil
	}

	// Priority 4: Fall back to legacy directory structure (backward compatibility)
	legacyClusterDir := filepath.Join(clustersDir, clusterName)
	legacyConfigPath := filepath.Join(legacyClusterDir, "."+clusterName+"-config.yaml")
	if _, statErr := os.Stat(legacyConfigPath); statErr == nil {
		return legacyConfigPath, nil
	}

	// Config file not found anywhere
	return "", fmt.Errorf("cluster configuration file not found for cluster %s", name)
}

// Load reads and unmarshals a YAML configuration file for the given cluster name.
// Default values are applied for any omitted fields.
// It supports both organization-based and legacy directory structures.
// The name parameter can be in "cluster" or "organization/cluster" format.
//
// Metadata Preservation:
//   - If the configuration file contains metadata (created_at, created_by, tags, annotations),
//     it will be preserved when loading.
//   - If metadata is missing (for backward compatibility with old configs), new metadata
//     will be initialized with current timestamps.
//
// Inputs:
//   - name: The name of the cluster (can be "cluster" or "organization/cluster").
//
// Outputs:
//   - Config: The loaded configuration.
//   - error: An error if the file does not exist or cannot be parsed.
//
// Deprecated: This function uses the legacy loading mechanism. New code should use
// internal/core/config.ConfigManager for better version handling and caching.
// This function will be removed in v2.0.0.
func Load(name string) (Config, error) {
	logDeprecationWarning(
		"config.Load()",
		"internal/core/config.ConfigManager",
		"v2.0.0",
	)

	// Parse the cluster identifier - ConfigPath will handle validation
	_, clusterName, err := ParseClusterIdentifier(name)
	if err != nil {
		return Config{}, fmt.Errorf("invalid cluster identifier: %w", err)
	}

	path, err := ConfigPath(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to resolve configuration path for cluster '%s': %w", name, err)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return Config{}, fmt.Errorf("failed to read cluster configuration file '%s': %w", path, readErr)
	}

	// Expand environment variables in the raw YAML data
	// This allows users to use ${VAR} or $VAR in their config file to reference secrets
	// stored in environment variables, avoiding plaintext secrets in the file.
	expandedData := []byte(os.ExpandEnv(string(data)))

	// Unmarshal YAML then overlay onto default config (use actual cluster name, not full identifier)
	cfg := defaultConfig(clusterName)
	if unmarshalErr := yaml.Unmarshal(expandedData, &cfg); unmarshalErr != nil {
		return Config{}, fmt.Errorf("failed to parse YAML configuration from '%s': %w", path, unmarshalErr)
	}

	// Detect schema version mismatch and log warning
	if needsMigration, configVersion, _ := DetectSchemaMigrationNeeded(cfg); needsMigration {
		if configVersion == "" {
			fmt.Fprintf(os.Stderr, "Warning: Configuration file '%s' does not have a schema version. Current schema version is %s. Consider running migration.\n", path, SchemaVersion)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Configuration file '%s' has schema version %s but current version is %s. Migration may be needed.\n", path, configVersion, SchemaVersion)
		}
	}

	// Apply organization-based defaults if not explicitly set
	applyOrganizationDefaults(&cfg)

	// Apply CLI defaults if available
	applyCLIDefaults(&cfg)

	// Initialize metadata if it's missing (for backward compatibility with old configs)
	if cfg.Metadata.CreatedAt.IsZero() {
		cfg.Metadata = NewConfigMetadata()
	}

	return cfg, nil
}

// GenerateCompleteConfig generates a complete configuration by merging schema defaults
// with the actual cluster configuration. The opencenter values take precedence over
// schema defaults.
//
// Deprecated: Use internal/core/config.ConfigManager.Load() with merge options instead.
// This function will be removed in v2.0.0.
// Migration: Replace GenerateCompleteConfig(name) with configManager.Load(path, LoadOptions{MergeDefaults: true})
//
// Inputs:
//   - name: The cluster name to load configuration for.
//
// Outputs:
//   - Config: The complete merged configuration.
//   - error: An error if the configuration cannot be generated.
func GenerateCompleteConfig(name string) (Config, error) {
	logDeprecationWarning(
		"config.GenerateCompleteConfig()",
		"internal/core/config.ConfigManager.Load() with merge options",
		"v2.0.0",
	)
	// Generate schema defaults as YAML
	defaultYAML, err := GenerateDefaultFromSchema(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to generate schema defaults: %w", err)
	}

	// Read the actual cluster configuration file directly as YAML
	path, err := ConfigPath(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get config path: %w", err)
	}
	actualYAML, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read cluster config: %w", err)
	}

	// Parse both as generic maps to preserve all structure
	var schemaDefaults map[string]any
	if err := yaml.Unmarshal(defaultYAML, &schemaDefaults); err != nil {
		return Config{}, fmt.Errorf("failed to parse schema defaults: %w", err)
	}

	var actualConfig map[string]any
	if err := yaml.Unmarshal(actualYAML, &actualConfig); err != nil {
		return Config{}, fmt.Errorf("failed to parse actual config: %w", err)
	}

	// Merge the configurations with actual config taking precedence
	mergedConfig := mergeYAMLMaps(schemaDefaults, actualConfig)

	// Marshal back to YAML then unmarshal into Config struct
	mergedYAML, err := yaml.Marshal(mergedConfig)
	if err != nil {
		return Config{}, fmt.Errorf("failed to marshal merged config: %w", err)
	}

	var completeCfg Config
	if err := yaml.Unmarshal(mergedYAML, &completeCfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse merged config into struct: %w", err)
	}

	return completeCfg, nil
}

// mergeYAMLMaps recursively merges two YAML maps, with values from 'override' taking precedence
func mergeYAMLMaps(base, override map[string]any) map[string]any {
	result := make(map[string]any)

	// Start with all base values
	for k, v := range base {
		result[k] = v
	}

	// Override with values from override map
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			// If both values are maps, merge them recursively
			if baseMap, baseIsMap := baseVal.(map[string]any); baseIsMap {
				if overrideMap, overrideIsMap := v.(map[string]any); overrideIsMap {
					result[k] = mergeYAMLMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		// Otherwise, override value takes precedence
		result[k] = v
	}

	return result
}

// GenerateCompleteConfigYAML generates a complete configuration YAML by merging schema defaults
// with the actual cluster configuration, preserving all YAML structure.
//
// Deprecated: Use internal/core/config.ConfigManager.Load() with merge options and marshal to YAML instead.
// This function will be removed in v2.0.0.
// Migration: Use configManager.Load() then yaml.Marshal() for YAML output.
//
// Inputs:
//   - name: The cluster name to load configuration for.
//
// Outputs:
//   - []byte: The complete merged configuration as YAML.
//   - error: An error if the configuration cannot be generated.
func GenerateCompleteConfigYAML(name string) ([]byte, error) {
	logDeprecationWarning(
		"config.GenerateCompleteConfigYAML()",
		"internal/core/config.ConfigManager.Load() with yaml.Marshal()",
		"v2.0.0",
	)
	// Generate schema defaults as YAML
	defaultYAML, err := GenerateDefaultFromSchema(name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema defaults: %w", err)
	}

	// Read the actual cluster configuration file directly as YAML
	path, err := ConfigPath(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}
	actualYAML, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster config: %w", err)
	}

	// Parse both as generic maps to preserve all structure
	var schemaDefaults map[string]any
	if err := yaml.Unmarshal(defaultYAML, &schemaDefaults); err != nil {
		return nil, fmt.Errorf("failed to parse schema defaults: %w", err)
	}

	var actualConfig map[string]any
	if err := yaml.Unmarshal(actualYAML, &actualConfig); err != nil {
		return nil, fmt.Errorf("failed to parse actual config: %w", err)
	}

	// Merge the configurations with actual config taking precedence
	mergedConfig := mergeYAMLMaps(schemaDefaults, actualConfig)

	// Marshal back to YAML
	mergedYAML, err := yaml.Marshal(mergedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged config: %w", err)
	}

	return mergedYAML, nil
}

// SaveDebugConfig saves a complete configuration to the GitOps directory as .opencenter.yaml
// for debugging purposes. This is only called when OPENCENTER_DEBUG environment variable exists.
//
// Deprecated: Use internal/core/config.ConfigManager for debug configuration output.
// This function will be removed in v2.0.0.
// Migration: Use configManager.Load() and save to debug location manually.
//
// Inputs:
//   - clusterName: The cluster name to generate complete config for.
//   - gitDir: The GitOps directory where to save the debug config.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func SaveDebugConfig(clusterName, gitDir string) error {
	logDeprecationWarning(
		"config.SaveDebugConfig()",
		"internal/core/config.ConfigManager",
		"v2.0.0",
	)
	if gitDir == "" {
		return fmt.Errorf("git directory is empty")
	}

	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return fmt.Errorf("failed to create git directory %s: %w", gitDir, err)
	}

	debugPath := filepath.Join(gitDir, ".opencenter.yaml")

	// Generate the complete config YAML
	data, err := GenerateCompleteConfigYAML(clusterName)
	if err != nil {
		return fmt.Errorf("failed to generate complete config: %w", err)
	}

	// Write the debug config file with 0600 permissions
	if err := os.WriteFile(debugPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write debug config to %s: %w", debugPath, err)
	}

	return nil
}

// Save writes the configuration to a YAML file. The file is saved with 0600
// permissions to protect sensitive data.
//
// Deprecated: Use internal/core/config.ConfigManager.Save() instead.
// This function will be removed in v2.0.0.
// Migration: Replace Save(cfg) with configManager.Save(path, config)
//
// Inputs:
//   - cfg: The configuration to save.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func Save(cfg Config) error {
	logDeprecationWarning(
		"config.Save()",
		"internal/core/config.ConfigManager.Save()",
		"v2.0.0",
	)
	return saveConfig(cfg, false)
}

// SaveWithOmitEmpty writes the configuration to a YAML file, omitting empty fields.
// The file is saved with 0600 permissions to protect sensitive data.
// This is useful for cleaning up configurations by removing fields with zero values.
//
// Deprecated: Use internal/core/config.ConfigManager.Save() with save options instead.
// This function will be removed in v2.0.0.
// Migration: Replace SaveWithOmitEmpty(cfg) with configManager.Save(path, config, SaveOptions{OmitEmpty: true})
//
// Inputs:
//   - cfg: The configuration to save.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func SaveWithOmitEmpty(cfg Config) error {
	logDeprecationWarning(
		"config.SaveWithOmitEmpty()",
		"internal/core/config.ConfigManager.Save() with save options",
		"v2.0.0",
	)
	return saveConfig(cfg, true)
}

// saveConfig is the internal implementation for saving configurations.
// It preserves the CreatedAt timestamp and CreatedBy field from the original
// configuration while updating the UpdatedAt timestamp to the current time.
// Tags and Annotations are also preserved during the save operation.
func saveConfig(cfg Config, omitEmpty bool) error {
	if cfg.ClusterName() == "" {
		return errors.New("cluster_name must not be empty")
	}

	// Update the UpdatedAt timestamp before saving
	cfg.Metadata.Touch()

	// Try to get existing config path first
	path, err := ConfigPath(cfg.ClusterName())
	if err != nil {
		// If config doesn't exist, determine where to create it based on organization
		path, err = getConfigPathForSave(cfg)
		if err != nil {
			return err
		}
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var data []byte
	var marshalErr error

	if omitEmpty {
		// Marshal to map first, then clean empty values
		var configMap map[string]any
		tempData, err := yaml.Marshal(&cfg)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(tempData, &configMap); err != nil {
			return err
		}

		// Remove empty values recursively
		cleanEmptyValues(configMap)

		data, marshalErr = yaml.Marshal(configMap)
	} else {
		// Standard marshal
		data, marshalErr = yaml.Marshal(&cfg)
	}

	if marshalErr != nil {
		return marshalErr
	}
	// Write with 0600 permissions
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
		return writeErr
	}
	return nil
}

// cleanEmptyValues recursively removes empty values from a map.
// Empty values include: nil, empty strings, empty slices, empty maps, and zero numbers.
func cleanEmptyValues(m map[string]any) {
	for key, value := range m {
		if isEmpty(value) {
			delete(m, key)
			continue
		}

		// Recursively clean nested maps
		if nestedMap, ok := value.(map[string]any); ok {
			cleanEmptyValues(nestedMap)
			// Remove the nested map if it became empty after cleaning
			if len(nestedMap) == 0 {
				delete(m, key)
			}
		}
	}
}

// isEmpty checks if a value is considered empty.
func isEmpty(v any) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case bool:
		return false // Keep boolean values even if false
	case int, int8, int16, int32, int64:
		return val == 0
	case uint, uint8, uint16, uint32, uint64:
		return val == 0
	case float32, float64:
		return val == 0
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}

// getConfigPathForSave determines where to save a new cluster configuration.
// It uses organization structure if organization is set, otherwise uses flat file structure.
func getConfigPathForSave(cfg Config) (string, error) {
	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory: %w", err)
	}

	organization := cfg.OpenCenter.Meta.Organization
	clusterName := cfg.ClusterName()

	if organization != "" && organization != "opencenter" {
		// Use organization structure: clusters/<org>/.<cluster>-config.yaml
		return filepath.Join(configDir, "clusters", organization, "."+clusterName+"-config.yaml"), nil
	}

	// Use flat file structure for backward compatibility and default organization
	return filepath.Join(configDir, clusterName+".yaml"), nil
}

// List returns a sorted list of cluster names from the configuration directory.
// It looks for cluster directories within the configured clustersDir.
// It supports both organization-based and legacy directory structures.
//
// Deprecated: Use internal/core/paths.PathResolver for cluster discovery instead.
// This function will be removed in v2.0.0.
// Migration: Use pathResolver methods for discovering clusters.
//
// Outputs:
//   - []string: A list of cluster names.
//   - error: An error if the directory cannot be read.
func List() ([]string, error) {
	logDeprecationWarning(
		"config.List()",
		"internal/core/paths.PathResolver",
		"v2.0.0",
	)
	dir, err := ResolveConfigDir()
	if err != nil {
		Debugf("List: failed to resolve config directory: %v", err)
		return nil, fmt.Errorf("failed to resolve configuration directory: %w", err)
	}
	Debugf("List: resolved config directory: %s", dir)

	// Load CLI configuration to get the configured clustersDir
	configManager, err := NewConfigManager("")
	if err != nil {
		Debugf("List: failed to load CLI config manager: %v", err)
		// Fall back to default behavior if CLI config can't be loaded
	}

	var clustersDir string
	if configManager != nil {
		clustersDir = configManager.GetConfig().Paths.ClustersDir
		Debugf("List: using clustersDir from CLI config: %s", clustersDir)
	} else {
		// Fallback to default
		clustersDir = filepath.Join(dir, "clusters")
		Debugf("List: using default clustersDir: %s", clustersDir)
	}

	// Expand environment variables and tilde in clustersDir
	clustersDir = corePaths.ExpandPath(clustersDir)
	Debugf("List: expanded clustersDir: %s", clustersDir)

	var names []string
	nameSet := make(map[string]bool) // Use set to avoid duplicates

	// First, check for flat YAML files in the config directory (for backward compatibility and tests)
	Debugf("List: checking for flat config files in: %s", dir)
	if flatEntries, flatErr := os.ReadDir(dir); flatErr == nil {
		for _, flatEntry := range flatEntries {
			if !flatEntry.IsDir() && strings.HasSuffix(flatEntry.Name(), ".yaml") {
				// Extract cluster name by removing .yaml extension
				clusterName := strings.TrimSuffix(flatEntry.Name(), ".yaml")
				// Skip the CLI config file itself
				if clusterName != "" && clusterName != "config" && !nameSet[clusterName] {
					Debugf("List: found flat config file: %s (cluster: %s)", flatEntry.Name(), clusterName)
					names = append(names, clusterName)
					nameSet[clusterName] = true
				}
			}
		}
	}
	Debugf("List: found %d flat config clusters", len(names))

	// Check clusters directory for legacy and organization-based structures
	Debugf("List: checking clusters directory: %s", clustersDir)
	entries, readErr := os.ReadDir(clustersDir)
	if readErr != nil {
		// If clusters directory doesn't exist, just return flat config files
		if os.IsNotExist(readErr) {
			Debugf("List: clusters directory does not exist, returning %d flat config clusters", len(names))
			// Sort lexically
			if len(names) > 1 {
				sortStrings(names)
			}
			return names, nil
		}
		Debugf("List: failed to read clusters directory: %v", readErr)
		return nil, fmt.Errorf("failed to read clusters directory: %w", readErr)
	}
	Debugf("List: found %d entries in clusters directory", len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			entryName := entry.Name()
			Debugf("List: processing directory entry: %s", entryName)

			// Check for legacy structure first: clustersDir/clusterName/.clusterName-config.yaml
			// This is for backward compatibility with old flat structure
			legacyConfigFile := filepath.Join(clustersDir, entryName, "."+entryName+"-config.yaml")
			Debugf("List: checking for legacy config file: %s", legacyConfigFile)
			if _, err := os.Stat(legacyConfigFile); err == nil {
				Debugf("List: found legacy config file for: %s", entryName)
				// Check if this is truly legacy (no infrastructure/clusters subdirs OR no applications subdirs)
				infraDir := filepath.Join(clustersDir, entryName, "infrastructure", "clusters")
				appsDir := filepath.Join(clustersDir, entryName, "applications", "overlays")
				hasInfra := false
				hasApps := false
				if _, err := os.Stat(infraDir); err == nil {
					hasInfra = true
					Debugf("List: %s has infrastructure directory", entryName)
				}
				if _, err := os.Stat(appsDir); err == nil {
					hasApps = true
					Debugf("List: %s has applications directory", entryName)
				}

				// If it has neither infrastructure nor applications subdirs, it's legacy flat structure
				if !hasInfra && !hasApps {
					Debugf("List: %s is legacy flat structure (no infra/apps dirs)", entryName)
					if !nameSet[entryName] {
						Debugf("List: adding legacy cluster: %s", entryName)
						names = append(names, entryName)
						nameSet[entryName] = true
					}
					continue // Skip organization check for this entry
				} else {
					Debugf("List: %s has subdirs (infra=%v, apps=%v), treating as organization", entryName, hasInfra, hasApps)
				}
			} else {
				Debugf("List: no legacy config file found for: %s", entryName)
			}

			// Check for organization-based structure
			// Look for clusters in: clustersDir/organization/infrastructure/clusters/<cluster>/.<cluster>-config.yaml
			orgDir := filepath.Join(clustersDir, entryName)
			infraClustersDir := filepath.Join(orgDir, "infrastructure", "clusters")
			Debugf("List: checking organization infrastructure/clusters directory: %s", infraClustersDir)

			if infraEntries, err := os.ReadDir(infraClustersDir); err == nil {
				Debugf("List: found %d entries in infrastructure/clusters directory for org: %s", len(infraEntries), entryName)
				for _, clusterEntry := range infraEntries {
					if clusterEntry.IsDir() {
						clusterName := clusterEntry.Name()
						// Check for config file at cluster directory level
						clusterConfigPath := filepath.Join(infraClustersDir, clusterName, "."+clusterName+"-config.yaml")
						Debugf("List: checking for config file: %s", clusterConfigPath)
						if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
							Debugf("List: found cluster config file for: %s", clusterName)
							// Format as organization/cluster
							fullName := entryName + "/" + clusterName
							if !nameSet[fullName] {
								Debugf("List: adding organization cluster: %s", fullName)
								names = append(names, fullName)
								nameSet[fullName] = true
							} else {
								Debugf("List: skipping duplicate cluster: %s", fullName)
							}
						}
					}
				}
			} else {
				Debugf("List: infrastructure/clusters directory does not exist for org %s: %v", entryName, err)
			}

			// Also check for config files at organization level (alternative location)
			if orgFiles, err := os.ReadDir(orgDir); err == nil {
				Debugf("List: found %d files in organization directory: %s", len(orgFiles), entryName)
				for _, orgFile := range orgFiles {
					if !orgFile.IsDir() && strings.HasPrefix(orgFile.Name(), ".") && strings.HasSuffix(orgFile.Name(), "-config.yaml") {
						Debugf("List: found organization-level config file: %s", orgFile.Name())
						// Extract cluster name from .<cluster>-config.yaml
						clusterName := strings.TrimPrefix(orgFile.Name(), ".")
						clusterName = strings.TrimSuffix(clusterName, "-config.yaml")
						Debugf("List: extracted cluster name: %s from file: %s", clusterName, orgFile.Name())
						if clusterName != "" {
							// Format as organization/cluster
							fullName := entryName + "/" + clusterName
							if !nameSet[fullName] {
								Debugf("List: adding organization cluster: %s", fullName)
								names = append(names, fullName)
								nameSet[fullName] = true
							} else {
								Debugf("List: skipping duplicate cluster: %s", fullName)
							}
						} else {
							Debugf("List: skipping cluster (name is empty)")
						}
					}
				}
			} else {
				Debugf("List: failed to read organization directory %s: %v", orgDir, err)
			}
		} else {
			Debugf("List: skipping non-directory entry: %s", entry.Name())
		}
	}

	// Sort lexically
	Debugf("List: sorting %d cluster names", len(names))
	if len(names) > 1 {
		sortStrings(names)
	}
	Debugf("List: returning %d total clusters", len(names))
	for i, name := range names {
		Debugf("List: final result[%d]: %s", i, name)
	}
	return names, nil
}

// simple string sorter to avoid pulling in a larger dependency.
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// activeClusterPath returns the absolute path to the file tracking
// the active cluster. This file stores the cluster name as plain
// text.
func activeClusterPath() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".active"), nil
}

// SetActive writes the given cluster name into the active marker file.
// If the name is empty, the marker file is removed.
//
// Deprecated: Use internal/core/config.ConfigManager for active cluster management.
// This function will be removed in v2.0.0.
// Migration: Use configManager methods for active cluster tracking.
//
// Inputs:
//   - name: The name of the cluster to set as active.
//
// Outputs:
//   - error: An error if the file cannot be written.
func SetActive(name string) error {
	logDeprecationWarning(
		"config.SetActive()",
		"internal/core/config.ConfigManager",
		"v2.0.0",
	)
	path, err := activeClusterPath()
	if err != nil {
		return err
	}
	if name == "" {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(name), 0o600)
}

// GetActive reads the active cluster name with precedence:
// 1. OPENCENTER_CLUSTER environment variable (session-scoped)
// 2. Session file (if shell integration is active)
// 3. Persistent selection from marker file
//
// Deprecated: Use internal/core/config.ConfigManager for active cluster retrieval.
// This function will be removed in v2.0.0.
// Migration: Use configManager methods for active cluster retrieval.
//
// Outputs:
//   - string: The active cluster name.
//   - error: An error if the file cannot be read.
func GetActive() (string, error) {
	logDeprecationWarning(
		"config.GetActive()",
		"internal/core/config.ConfigManager",
		"v2.0.0",
	)
	// Priority 1: Check environment variable (highest priority)
	if cluster := os.Getenv("OPENCENTER_CLUSTER"); cluster != "" {
		return strings.TrimSpace(cluster), nil
	}

	// Priority 2: Check session file (shell integration)
	if sessionFile := os.Getenv("OPENCENTER_SESSION_FILE"); sessionFile != "" {
		if data, err := os.ReadFile(sessionFile); err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
	}

	// Priority 3: Fall back to persistent selection
	path, err := activeClusterPath()
	if err != nil {
		return "", err
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return "", nil
		}
		return "", readErr
	}
	return strings.TrimSpace(string(data)), nil
}
