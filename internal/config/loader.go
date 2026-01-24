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
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigLoader implements the ConfigLoaderInterface for loading configurations from various sources.
type ConfigLoader struct {
	pathResolver PathResolverInterface
}

// NewConfigLoader creates a new configuration loader.
func NewConfigLoader(pathResolver PathResolverInterface) *ConfigLoader {
	return &ConfigLoader{
		pathResolver: pathResolver,
	}
}

// LoadFromFile loads configuration from a file path.
func (cl *ConfigLoader) LoadFromFile(ctx context.Context, filePath string) (*Config, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file does not exist: %s", filePath)
	}

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file '%s': %w", filePath, err)
	}

	// Extract cluster name from file path for context
	clusterName := cl.extractClusterNameFromPath(filePath)

	return cl.LoadFromBytes(ctx, data, clusterName)
}

// LoadFromBytes loads configuration from byte data.
// It automatically detects the schema version and routes to the appropriate loader.
// Requirements: 13.1, 13.2, 13.3
func (cl *ConfigLoader) LoadFromBytes(ctx context.Context, data []byte, clusterName string) (*Config, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("configuration data cannot be empty")
	}

	if clusterName == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}

	// Detect schema version
	versionInfo, err := DetectSchemaVersionFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to detect schema version: %w", err)
	}

	// Route to appropriate loader based on version
	if versionInfo.IsV2 {
		// v2 configurations are not yet fully supported in the v1 loader
		// Return an error directing users to use the v2 loader
		return nil, fmt.Errorf("v2 configuration detected: use v2.ConfigLoader for loading v2 configurations")
	}

	// v1 configuration loading (existing logic)
	// Start with default configuration
	config := defaultConfig(clusterName)

	// Unmarshal YAML data onto the default configuration
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML configuration: %w", err)
	}

	// Resolve references after parsing
	resolver := NewReferenceResolver()
	if err := resolver.Resolve(&config); err != nil {
		return nil, fmt.Errorf("failed to resolve configuration references: %w", err)
	}

	return &config, nil
}

// LoadDefault creates a default configuration for a cluster.
func (cl *ConfigLoader) LoadDefault(ctx context.Context, clusterName string) (*Config, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	config := defaultConfig(clusterName)
	return &config, nil
}

// GenerateCompleteConfig generates a complete configuration with defaults merged.
func (cl *ConfigLoader) GenerateCompleteConfig(ctx context.Context, clusterName string) (*Config, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// Use the existing GenerateCompleteConfig function
	config, err := GenerateCompleteConfig(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate complete configuration: %w", err)
	}

	return &config, nil
}

// LoadFromPath loads configuration using organization-aware path resolution.
func (cl *ConfigLoader) LoadFromPath(ctx context.Context, clusterName string) (*Config, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// Try organization-aware path resolution first
	if cl.pathResolver != nil {
		if paths, err := cl.pathResolver.ResolveClusterPaths(ctx, clusterName, ""); err == nil {
			configPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
			if _, err := os.Stat(configPath); err == nil {
				return cl.LoadFromFile(ctx, configPath)
			}
		}
	}

	// Fall back to legacy path resolution
	configPath, err := ConfigPath(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve configuration path: %w", err)
	}

	return cl.LoadFromFile(ctx, configPath)
}

// LoadWithDefaults loads configuration and merges with schema defaults.
func (cl *ConfigLoader) LoadWithDefaults(ctx context.Context, clusterName string) (*Config, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// Generate complete configuration with defaults
	return cl.GenerateCompleteConfig(ctx, clusterName)
}

// SaveToFile saves configuration to a file path.
func (cl *ConfigLoader) SaveToFile(ctx context.Context, config *Config, filePath string) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dir, err)
	}

	// Marshal configuration to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration to YAML: %w", err)
	}

	// Write to file with secure permissions
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write configuration file '%s': %w", filePath, err)
	}

	return nil
}

// SaveToPath saves configuration using organization-aware path resolution.
func (cl *ConfigLoader) SaveToPath(ctx context.Context, config *Config) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	clusterName := config.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	// Try organization-aware path resolution first
	if cl.pathResolver != nil {
		if paths, err := cl.pathResolver.ResolveClusterPaths(ctx, clusterName, ""); err == nil {
			configPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
			return cl.SaveToFile(ctx, config, configPath)
		}
	}

	// Fall back to legacy path resolution
	configPath, err := ConfigPath(clusterName)
	if err != nil {
		return fmt.Errorf("failed to resolve configuration path: %w", err)
	}

	return cl.SaveToFile(ctx, config, configPath)
}

// ValidateFile validates that a configuration file exists and is readable.
func (cl *ConfigLoader) ValidateFile(ctx context.Context, filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("configuration file does not exist: %s", filePath)
	}
	if err != nil {
		return fmt.Errorf("failed to access configuration file '%s': %w", filePath, err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", filePath)
	}

	// Check if file is readable
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("configuration file is not readable: %w", err)
	}
	file.Close()

	return nil
}

// extractClusterNameFromPath extracts the cluster name from a configuration file path.
func (cl *ConfigLoader) extractClusterNameFromPath(filePath string) string {
	// Extract filename without extension
	filename := filepath.Base(filePath)

	// Handle organization-based structure: .cluster-name-config.yaml
	if filename[0] == '.' && filepath.Ext(filename) == ".yaml" {
		// Remove leading dot and -config.yaml suffix
		name := filename[1:]
		if suffix := "-config.yaml"; len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			return name[:len(name)-len(suffix)]
		}
	}

	// Handle legacy structure: cluster-name.yaml
	if filepath.Ext(filename) == ".yaml" {
		return filename[:len(filename)-5] // Remove .yaml extension
	}

	return filename
}

// GetSupportedFormats returns the list of supported configuration file formats.
func (cl *ConfigLoader) GetSupportedFormats() []string {
	return []string{"yaml", "yml"}
}

// IsValidFormat checks if a file format is supported.
func (cl *ConfigLoader) IsValidFormat(filePath string) bool {
	ext := filepath.Ext(filePath)
	if len(ext) > 0 {
		ext = ext[1:] // Remove the dot
	}

	supportedFormats := cl.GetSupportedFormats()
	for _, format := range supportedFormats {
		if ext == format {
			return true
		}
	}

	return false
}

// LoadMultiple loads multiple configurations by cluster names.
func (cl *ConfigLoader) LoadMultiple(ctx context.Context, clusterNames []string) (map[string]*Config, error) {
	if len(clusterNames) == 0 {
		return make(map[string]*Config), nil
	}

	configs := make(map[string]*Config)
	var errors []error

	for _, clusterName := range clusterNames {
		config, err := cl.LoadFromPath(ctx, clusterName)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to load config for cluster '%s': %w", clusterName, err))
			continue
		}
		configs[clusterName] = config
	}

	if len(errors) > 0 {
		return configs, fmt.Errorf("failed to load some configurations: %v", errors)
	}

	return configs, nil
}

// CreateFromTemplate creates a new configuration from a template.
func (cl *ConfigLoader) CreateFromTemplate(ctx context.Context, clusterName, templateName string) (*Config, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	if templateName == "" {
		templateName = "default"
	}

	// For now, we only support the default template
	if templateName != "default" {
		return nil, fmt.Errorf("unsupported template: %s", templateName)
	}

	return cl.LoadDefault(ctx, clusterName)
}
