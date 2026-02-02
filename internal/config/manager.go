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
	"sync"
	"time"
)

// ConfigurationManager implements the ConfigManagerInterface with caching and validation.
type ConfigurationManager struct {
	loader       ConfigLoaderInterface
	validator    ConfigValidatorInterface
	pathResolver PathResolverInterface
	cache        ConfigCacheInterface

	// Configuration options
	enableCache  bool
	cacheTimeout time.Duration

	// Thread safety
	mu sync.RWMutex
}

// NewConfigurationManager creates a new configuration manager with the specified components.
func NewConfigurationManager(
	loader ConfigLoaderInterface,
	validator ConfigValidatorInterface,
	pathResolver PathResolverInterface,
	cache ConfigCacheInterface,
) *ConfigurationManager {
	return &ConfigurationManager{
		loader:       loader,
		validator:    validator,
		pathResolver: pathResolver,
		cache:        cache,
		enableCache:  true,
		cacheTimeout: 5 * time.Minute,
	}
}

// NewEnhancedConfigurationManager creates a new configuration manager with enhanced validation.
func NewEnhancedConfigurationManager(
	loader ConfigLoaderInterface,
	pathResolver PathResolverInterface,
	cache ConfigCacheInterface,
	autoRepair bool,
) *ConfigurationManager {
	enhancedValidator := NewEnhancedConfigValidator(autoRepair)

	return &ConfigurationManager{
		loader:       loader,
		validator:    enhancedValidator,
		pathResolver: pathResolver,
		cache:        cache,
		enableCache:  true,
		cacheTimeout: 5 * time.Minute,
	}
}

// LoadConfig loads a cluster configuration by name with caching support.
func (cm *ConfigurationManager) LoadConfig(ctx context.Context, clusterName string) (*Config, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// Check cache first if enabled
	if cm.enableCache {
		if cached, found := cm.cache.Get(ctx, clusterName); found {
			return cached, nil
		}
	}

	// Load configuration using the loader
	config, err := cm.loader.LoadFromFile(ctx, "")
	if err != nil {
		// Try to resolve the config path and load directly
		configPath, pathErr := cm.GetConfigPath(ctx, clusterName)
		if pathErr != nil {
			return nil, fmt.Errorf("failed to resolve config path for cluster '%s': %w", clusterName, pathErr)
		}

		config, err = cm.loader.LoadFromFile(ctx, configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config for cluster '%s': %w", clusterName, err)
		}
	}

	// Validate the loaded configuration
	if result := cm.validator.Validate(ctx, config); !result.Valid {
		// Log validation errors but don't fail loading
		for _, validationErr := range result.Errors {
			fmt.Printf("Warning: Configuration validation error: %s\n", validationErr.Error())
		}
	}

	// Cache the loaded configuration if caching is enabled
	if cm.enableCache {
		if cacheErr := cm.cache.Set(ctx, clusterName, config); cacheErr != nil {
			// Log cache error but don't fail loading
			fmt.Printf("Warning: Failed to cache configuration for cluster '%s': %v\n", clusterName, cacheErr)
		}
	}

	return config, nil
}

// SaveConfig saves a cluster configuration with validation.
func (cm *ConfigurationManager) SaveConfig(ctx context.Context, config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	clusterName := config.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	// Validate configuration before saving
	if result := cm.validator.Validate(ctx, config); !result.Valid {
		return fmt.Errorf("configuration validation failed: %v", result.Errors)
	}

	// Save the configuration using the existing Save function
	// Save handles determining the correct path and creating directories
	if err := Save(*config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Invalidate cache for this cluster
	if cm.enableCache {
		if cacheErr := cm.cache.InvalidateCluster(ctx, clusterName); cacheErr != nil {
			// Log cache error but don't fail saving
			fmt.Printf("Warning: Failed to invalidate cache for cluster '%s': %v\n", clusterName, cacheErr)
		}
	}

	return nil
}

// ValidateConfig validates a cluster configuration.
func (cm *ConfigurationManager) ValidateConfig(ctx context.Context, config *Config) *ConfigValidationResult {
	if config == nil {
		return &ConfigValidationResult{
			Valid: false,
			Errors: []*ConfigValidationError{
				{
					Type:    "validation",
					Field:   "config",
					Message: "configuration cannot be nil",
				},
			},
		}
	}

	return cm.validator.Validate(ctx, config)
}

// ValidateConfigComprehensive performs comprehensive validation using the enhanced validator.
func (cm *ConfigurationManager) ValidateConfigComprehensive(ctx context.Context, config *Config) *ConfigValidationResult {
	if config == nil {
		return &ConfigValidationResult{
			Valid: false,
			Errors: []*ConfigValidationError{
				{
					Type:    "validation",
					Field:   "config",
					Message: "configuration cannot be nil",
				},
			},
		}
	}

	// Use enhanced validator if available
	if enhancedValidator, ok := cm.validator.(*EnhancedConfigValidator); ok {
		result := enhancedValidator.ValidateComprehensive(ctx, config)

		// Convert structured errors to config validation errors
		configResult := &ConfigValidationResult{
			Valid:    result.Valid,
			Errors:   []*ConfigValidationError{},
			Warnings: []*ConfigValidationError{},
		}

		for _, err := range result.Errors {
			configResult.Errors = append(configResult.Errors, &ConfigValidationError{
				Type:        string(err.Type),
				Field:       err.Field,
				Message:     err.Message,
				Suggestions: err.Suggestions,
			})
		}

		for _, warning := range result.Warnings {
			configResult.Warnings = append(configResult.Warnings, &ConfigValidationError{
				Type:        string(warning.Type),
				Field:       warning.Field,
				Message:     warning.Message,
				Suggestions: warning.Suggestions,
			})
		}

		return configResult
	}

	// Fall back to regular validation
	return cm.validator.Validate(ctx, config)
}

// ListConfigs returns a list of available cluster configurations.
func (cm *ConfigurationManager) ListConfigs(ctx context.Context) ([]string, error) {
	// Use the existing List function
	clusters, err := List()
	if err != nil {
		return nil, fmt.Errorf("failed to list configurations: %w", err)
	}

	return clusters, nil
}

// DeleteConfig removes a cluster configuration.
func (cm *ConfigurationManager) DeleteConfig(ctx context.Context, clusterName string) error {
	if err := ValidateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	// Get the configuration file path
	configPath, err := cm.GetConfigPath(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Check if the configuration file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("configuration for cluster '%s' does not exist", clusterName)
	}

	// Remove the configuration file
	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("failed to delete configuration file '%s': %w", configPath, err)
	}

	// Invalidate cache for this cluster
	if cm.enableCache {
		if cacheErr := cm.cache.InvalidateCluster(ctx, clusterName); cacheErr != nil {
			// Log cache error but don't fail deletion
			fmt.Printf("Warning: Failed to invalidate cache for cluster '%s': %v\n", clusterName, cacheErr)
		}
	}

	return nil
}

// GetConfigPath returns the path to a cluster's configuration file.
func (cm *ConfigurationManager) GetConfigPath(ctx context.Context, clusterName string) (string, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	// Try organization-aware path resolution first
	if paths, err := cm.pathResolver.ResolveClusterPaths(ctx, clusterName, ""); err == nil {
		orgConfigPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
		if _, err := os.Stat(orgConfigPath); err == nil {
			return orgConfigPath, nil
		}
	}

	// Fall back to the existing ConfigPath function
	return ConfigPath(clusterName)
}

// SetActiveConfig sets the active cluster configuration.
func (cm *ConfigurationManager) SetActiveConfig(ctx context.Context, clusterName string) error {
	if clusterName != "" {
		if err := ValidateClusterName(clusterName); err != nil {
			return fmt.Errorf("invalid cluster name: %w", err)
		}

		// Verify the cluster configuration exists
		if _, err := cm.LoadConfig(ctx, clusterName); err != nil {
			return fmt.Errorf("cluster '%s' configuration not found: %w", clusterName, err)
		}
	}

	// Use the existing SetActive function
	return SetActive(clusterName)
}

// GetActiveConfig returns the name of the active cluster configuration.
func (cm *ConfigurationManager) GetActiveConfig(ctx context.Context) (string, error) {
	// Use the existing GetActive function
	return GetActive()
}

// SetCacheEnabled enables or disables configuration caching.
func (cm *ConfigurationManager) SetCacheEnabled(enabled bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.enableCache = enabled
}

// SetCacheTimeout sets the cache timeout duration.
func (cm *ConfigurationManager) SetCacheTimeout(timeout time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cacheTimeout = timeout
}

// ClearCache clears all cached configurations.
func (cm *ConfigurationManager) ClearCache(ctx context.Context) error {
	if !cm.enableCache {
		return nil
	}

	return cm.cache.Clear(ctx)
}

// GetValidationSummary returns a human-readable summary of validation results for a cluster.
func (cm *ConfigurationManager) GetValidationSummary(ctx context.Context, clusterName string) (string, error) {
	config, err := cm.LoadConfig(ctx, clusterName)
	if err != nil {
		return "", fmt.Errorf("failed to load configuration: %w", err)
	}

	result := cm.ValidateConfig(ctx, config)
	return cm.formatValidationSummary(result), nil
}

// formatValidationSummary formats a validation result into a human-readable summary.
func (cm *ConfigurationManager) formatValidationSummary(result *ConfigValidationResult) string {
	if result.Valid {
		return "✓ Configuration is valid"
	}

	summary := "✗ Configuration has issues\n"

	if len(result.Errors) > 0 {
		summary += fmt.Sprintf("\nErrors (%d):\n", len(result.Errors))
		for _, err := range result.Errors {
			summary += fmt.Sprintf("  - %s\n", err.Error())
		}
	}

	if len(result.Warnings) > 0 {
		summary += fmt.Sprintf("\nWarnings (%d):\n", len(result.Warnings))
		for _, warning := range result.Warnings {
			summary += fmt.Sprintf("  - %s\n", warning.Error())
		}
	}

	if len(result.Repaired) > 0 {
		summary += fmt.Sprintf("\nAuto-repaired (%d):\n", len(result.Repaired))
		for _, repaired := range result.Repaired {
			summary += fmt.Sprintf("  - %s\n", repaired.Error())
		}
	}

	return summary
}

// GetClusterPaths returns all paths for a cluster.
func (cm *ConfigurationManager) GetClusterPaths(ctx context.Context, clusterName string) (*OrganizationClusterPaths, error) {
	if cm.pathResolver == nil {
		return nil, fmt.Errorf("path resolver not available")
	}

	// Try to determine organization
	organization, err := cm.pathResolver.GetClusterOrganization(ctx, clusterName)
	if err != nil {
		organization = "opencenter" // Default organization
	}

	return cm.pathResolver.ResolveClusterPaths(ctx, clusterName, organization)
}

// CreateClusterDirectories creates all necessary directories for a cluster.
func (cm *ConfigurationManager) CreateClusterDirectories(ctx context.Context, clusterName, organization string) error {
	if cm.pathResolver == nil {
		return fmt.Errorf("path resolver not available")
	}

	return cm.pathResolver.CreateClusterDirectories(ctx, clusterName, organization)
}
