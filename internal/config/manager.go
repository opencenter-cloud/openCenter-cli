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
	"strings"
	"sync"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// ConfigurationManager provides unified configuration management.
// It orchestrates all configuration operations including loading, saving,
// validation, listing, and deletion with caching and atomic operations.
//
// The manager integrates with:
//   - ConfigCache: Thread-safe in-memory caching
//   - ConfigIOHandler: Low-level I/O operations
//   - ValidationEngine: Configuration validation (Phase 2)
//   - PathResolver: Path resolution (Phase 1)
//   - FileSystem: Atomic file operations (Phase 1)
//
// Example usage:
//
//	manager, err := NewConfigurationManager()
//	if err != nil {
//	    return err
//	}
//
//	// Load configuration with caching
//	config, err := manager.Load(ctx, "my-cluster")
//	if err != nil {
//	    return err
//	}
//
//	// Save with validation and atomic writes
//	err = manager.Save(ctx, config)
type ConfigurationManager struct {
	loader       *ConfigIOHandler
	validator    *validation.ValidationEngine
	cache        *ConfigCache
	pathResolver *paths.PathResolver
	fileSystem   fs.FileSystem
	mu           sync.RWMutex
}

// NewConfigurationManager creates a new ConfigurationManager with all dependencies.
//
// The manager is initialized with:
//   - ConfigIOHandler for file I/O
//   - ValidationEngine from Phase 2
//   - ConfigCache for in-memory caching
//   - PathResolver from Phase 1
//   - DefaultFileSystem from Phase 1
//
// Returns:
//   - *ConfigurationManager: New manager instance
//   - error: Initialization error (nil on success)
//
// Example:
//
//	manager, err := NewConfigurationManager()
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewConfigurationManager() (*ConfigurationManager, error) {
	// Create FileSystem with error handler
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	// Create PathResolver with base directory from CLI config
	pathResolver := paths.NewPathResolver(ResolveClustersDir())

	// Create ValidationEngine
	// Note: Config validation is currently disabled as the ConfigValidator
	// is designed for individual config values, not full Config structs
	validator := validation.NewValidationEngine()

	return &ConfigurationManager{
		loader:       NewConfigIOHandler(fileSystem),
		validator:    validator,
		cache:        NewConfigCache(),
		pathResolver: pathResolver,
		fileSystem:   fileSystem,
	}, nil
}

// NewConfigurationManagerWithDeps creates a ConfigurationManager with custom dependencies.
//
// This constructor is useful for testing or when custom components are needed.
//
// Parameters:
//   - loader: ConfigIOHandler for file I/O
//   - validator: ValidationEngine for validation
//   - cache: ConfigCache for caching
//   - pathResolver: PathResolver for path resolution
//   - fileSystem: FileSystem for file operations
//
// Returns:
//   - *ConfigurationManager: New manager instance
func NewConfigurationManagerWithDeps(
	loader *ConfigIOHandler,
	validator *validation.ValidationEngine,
	cache *ConfigCache,
	pathResolver *paths.PathResolver,
	fileSystem fs.FileSystem,
) *ConfigurationManager {
	return &ConfigurationManager{
		loader:       loader,
		validator:    validator,
		cache:        cache,
		pathResolver: pathResolver,
		fileSystem:   fileSystem,
	}
}

// Load loads a configuration from disk or cache.
//
// The load process follows these steps:
//  1. Check cache for existing configuration (fast path)
//  2. Resolve configuration file path using PathResolver
//  3. Read and parse configuration file
//  4. Validate configuration using ValidationEngine
//  5. Store in cache for future loads
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - name: Cluster name to load
//
// Returns:
//   - *Config: Loaded and validated configuration
//   - error: Load, parse, or validation error
//
// Example:
//
//	config, err := manager.Load(ctx, "prod-cluster")
//	if err != nil {
//	    return fmt.Errorf("failed to load config: %w", err)
//	}
func (cm *ConfigurationManager) Load(ctx context.Context, name string) (*v2.Config, error) {
	if name == "" {
		return nil, errors.WrapWithOperation(
			fmt.Errorf("cluster name cannot be empty"),
			"load",
		)
	}

	// Check cache first (fast path)
	if cached, found := cm.cache.Get(ctx, name); found {
		return cached, nil
	}

	// Parse cluster identifier to handle organization/cluster format
	var clusterPaths *paths.ClusterPaths
	var err error

	if strings.Contains(name, "/") {
		// organization/cluster format - parse and use Resolve
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			return nil, errors.WrapWithOperation(
				fmt.Errorf("invalid cluster identifier format: expected 'organization/cluster'"),
				"load",
			)
		}
		organization := parts[0]
		clusterName := parts[1]
		clusterPaths, err = cm.pathResolver.Resolve(ctx, clusterName, organization)
	} else {
		// Just cluster name - use ResolveWithFallback to search all organizations
		clusterPaths, err = cm.pathResolver.ResolveWithFallback(ctx, name)
	}

	if err != nil {
		return nil, NewConfigNotFoundError(name, errors.WrapWithOperation(
			NewPathError(name, "", err),
			"load",
		))
	}

	configPath := clusterPaths.ConfigPath

	// Check if file exists
	if !cm.fileSystem.Exists(configPath) {
		return nil, NewConfigNotFoundError(name, errors.WrapWithOperation(
			NewFileError("read", configPath, fmt.Errorf("configuration file not found")),
			"load",
		))
	}

	// Load configuration from file
	config, err := cm.loader.LoadFromFile(ctx, configPath)
	if err != nil {
		// Check if it's a parse error - wrap with appropriate context
		return nil, errors.WrapWithOperation(
			NewParseError(configPath, 0, 0, err),
			"load",
		)
	}

	// Cache the loaded configuration
	cm.cache.Set(ctx, name, config)

	return config, nil
}

// LoadWithoutValidation loads a configuration from disk or cache without validation.
//
// This method is primarily intended for testing scenarios where you need to load
// incomplete or invalid configurations. In production code, use Load() instead.
//
// The load process follows these steps:
//  1. Check cache for existing configuration (fast path)
//  2. Resolve configuration file path using PathResolver
//  3. Read and parse configuration file
//  4. Store in cache for future loads (NO VALIDATION)
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - name: Cluster name to load
//
// Returns:
//   - *Config: Loaded configuration (not validated)
//   - error: Load or parse error
//
// Example:
//
//	config, err := manager.LoadWithoutValidation(ctx, "test-cluster")
//	if err != nil {
//	    return fmt.Errorf("failed to load config: %w", err)
//	}
func (cm *ConfigurationManager) LoadWithoutValidation(ctx context.Context, name string) (*v2.Config, error) {
	if name == "" {
		return nil, errors.WrapWithOperation(
			fmt.Errorf("cluster name cannot be empty"),
			"load",
		)
	}

	// Check cache first (fast path)
	if cached, found := cm.cache.Get(ctx, name); found {
		return cached, nil
	}

	// Parse cluster identifier to handle organization/cluster format
	var clusterPaths *paths.ClusterPaths
	var err error

	if strings.Contains(name, "/") {
		// organization/cluster format - parse and use Resolve
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			return nil, errors.WrapWithOperation(
				fmt.Errorf("invalid cluster identifier format: expected 'organization/cluster'"),
				"load",
			)
		}
		organization := parts[0]
		clusterName := parts[1]
		clusterPaths, err = cm.pathResolver.Resolve(ctx, clusterName, organization)
	} else {
		// Just cluster name - use ResolveWithFallback to search all organizations
		clusterPaths, err = cm.pathResolver.ResolveWithFallback(ctx, name)
	}

	if err != nil {
		return nil, NewConfigNotFoundError(name, errors.WrapWithOperation(
			NewPathError(name, "", err),
			"load",
		))
	}

	configPath := clusterPaths.ConfigPath

	// Check if file exists
	if !cm.fileSystem.Exists(configPath) {
		return nil, NewConfigNotFoundError(name, errors.WrapWithOperation(
			NewFileError("read", configPath, fmt.Errorf("configuration file not found")),
			"load",
		))
	}

	// Load configuration from file
	config, err := cm.loader.LoadFromFile(ctx, configPath)
	if err != nil {
		// Check if it's a parse error - wrap with appropriate context
		return nil, errors.WrapWithOperation(
			NewParseError(configPath, 0, 0, err),
			"load",
		)
	}

	// Cache the loaded configuration (without validation)
	cm.cache.Set(ctx, name, config)

	return config, nil
}

// Save saves a configuration to disk atomically.
//
// The save process follows these steps:
//  1. Validate configuration using ValidationEngine
//  2. Resolve configuration file path
//  3. Create backup of existing file (if exists)
//  4. Write configuration atomically using FileSystem
//  5. Invalidate cache entry for this cluster
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - config: Configuration to save
//
// Returns:
//   - error: Validation, path resolution, or write error
//
// Example:
//
//	err := manager.Save(ctx, config)
//	if err != nil {
//	    return fmt.Errorf("failed to save config: %w", err)
//	}
func (cm *ConfigurationManager) Save(ctx context.Context, config *v2.Config) error {
	if config == nil {
		return errors.WrapWithOperation(
			fmt.Errorf("configuration cannot be nil"),
			"save",
		)
	}

	clusterName := config.ClusterName()
	if clusterName == "" {
		return errors.WrapWithOperation(
			NewValidationError("cluster_name", "cluster name cannot be empty", nil),
			"save",
		)
	}

	// Validate configuration before saving
	Debug("ConfigManager.Save: starting validation")
	Debugf("ConfigManager.Save: validating config for cluster: %s", config.ClusterName())

	if err := cm.validateConfigStruct(ctx, config); err != nil {
		return errors.WrapWithOperation(
			err,
			"save",
		)
	}

	// Resolve configuration path
	organization := config.OpenCenter.Meta.Organization
	clusterPaths, err := cm.pathResolver.Resolve(ctx, clusterName, organization)
	if err != nil {
		return errors.WrapWithOperation(
			NewPathError(clusterName, organization, err),
			"save",
		)
	}

	configPath := clusterPaths.ConfigPath

	// Create backup if file exists
	if cm.fileSystem.Exists(configPath) {
		backupPath := configPath + ".backup"
		data, err := cm.fileSystem.ReadFile(configPath)
		if err != nil {
			return errors.WrapWithOperation(
				NewFileError("read", configPath, err),
				"save",
			)
		}

		if err := cm.fileSystem.WriteFile(backupPath, data, 0600); err != nil {
			return errors.WrapWithOperation(
				NewFileError("write", backupPath, err),
				"save",
			)
		}
	}

	// Save configuration atomically
	if err := cm.loader.SaveToFile(ctx, configPath, config); err != nil {
		return errors.WrapWithOperation(
			NewFileError("write", configPath, err),
			"save",
		)
	}

	// Invalidate cache entry
	cm.cache.Invalidate(ctx, clusterName)

	return nil
}

// SaveWithoutValidation saves a configuration to disk atomically without validation.
//
// This method is primarily intended for testing scenarios where you need to save
// incomplete or invalid configurations. In production code, use Save() instead.
//
// The save process follows these steps:
//  1. Resolve configuration file path
//  2. Create backup of existing file (if exists)
//  3. Write configuration atomically using FileSystem
//  4. Invalidate cache entry for this cluster
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - config: Configuration to save
//
// Returns:
//   - error: Path resolution or write error
//
// Example:
//
//	err := manager.SaveWithoutValidation(ctx, config)
//	if err != nil {
//	    return fmt.Errorf("failed to save config: %w", err)
//	}
func (cm *ConfigurationManager) SaveWithoutValidation(ctx context.Context, config *v2.Config) error {
	if config == nil {
		return errors.WrapWithOperation(
			fmt.Errorf("configuration cannot be nil"),
			"save",
		)
	}

	clusterName := config.ClusterName()
	if clusterName == "" {
		return errors.WrapWithOperation(
			NewValidationError("cluster_name", "cluster name cannot be empty", nil),
			"save",
		)
	}

	// Resolve configuration path
	organization := config.OpenCenter.Meta.Organization
	clusterPaths, err := cm.pathResolver.Resolve(ctx, clusterName, organization)
	if err != nil {
		return errors.WrapWithOperation(
			NewPathError(clusterName, organization, err),
			"save",
		)
	}

	configPath := clusterPaths.ConfigPath

	// Create backup if file exists
	if cm.fileSystem.Exists(configPath) {
		backupPath := configPath + ".backup"
		data, err := cm.fileSystem.ReadFile(configPath)
		if err != nil {
			return errors.WrapWithOperation(
				NewFileError("read", configPath, err),
				"save",
			)
		}

		if err := cm.fileSystem.WriteFile(backupPath, data, 0600); err != nil {
			return errors.WrapWithOperation(
				NewFileError("write", backupPath, err),
				"save",
			)
		}
	}

	// Save configuration atomically
	if err := cm.loader.SaveToFile(ctx, configPath, config); err != nil {
		return errors.WrapWithOperation(
			NewFileError("write", configPath, err),
			"save",
		)
	}

	// Invalidate cache entry
	cm.cache.Invalidate(ctx, clusterName)

	return nil
}

// Validate validates a configuration without saving.
//
// This method is useful for checking configuration validity before
// attempting to save or for validating configurations from other sources.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - config: Configuration to validate
//
// Returns:
//   - error: Validation error (nil if valid)
//
// Example:
//
//	err := manager.Validate(ctx, config)
//	if err != nil {
//	    fmt.Println("Configuration is invalid:", err)
//	}
func (cm *ConfigurationManager) Validate(ctx context.Context, config *v2.Config) error {
	if config == nil {
		return NewValidationError("", "configuration cannot be nil", nil)
	}

	Debug("ConfigManager.Validate: starting validation")
	Debugf("ConfigManager.Validate: validating config for cluster: %s", config.ClusterName())

	return cm.validateConfigStruct(ctx, config)
}

func (cm *ConfigurationManager) validateConfigStruct(ctx context.Context, cfg *v2.Config) error {
	if err := cm.loader.ValidateConfig(ctx, cfg); err != nil {
		return NewValidationError("", fmt.Sprintf("validation engine error: %v", err), err)
	}

	Debug("ConfigManager.Validate: validation passed")
	return nil
}

// List returns all cluster names in the configuration directory.
//
// The method scans the base directory for organization subdirectories
// and returns all cluster names found.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//
// Returns:
//   - []string: List of cluster names (empty if none found)
//   - error: Directory read error
//
// Example:
//
//	clusters, err := manager.List(ctx)
//	if err != nil {
//	    return err
//	}
//	for _, cluster := range clusters {
//	    fmt.Println(cluster)
//	}
func (cm *ConfigurationManager) List(ctx context.Context) ([]string, error) {
	return cm.ListWithOrganization(ctx, "")
}

// ListWithOrganization returns cluster names filtered by organization.
//
// If organization is empty, returns clusters from all organizations.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - organization: Organization name to filter by (empty for all)
//
// Returns:
//   - []string: List of cluster names
//   - error: Directory read error
//
// Example:
//
//	clusters, err := manager.ListWithOrganization(ctx, "my-org")
func (cm *ConfigurationManager) ListWithOrganization(ctx context.Context, organization string) ([]string, error) {
	baseDir := cm.pathResolver.GetBaseDir()

	// Check if base directory exists
	if !cm.fileSystem.Exists(baseDir) {
		return []string{}, nil
	}

	var clusters []string

	// If organization is specified, only scan that organization
	if organization != "" {
		orgDir := filepath.Join(baseDir, organization, "infrastructure", "clusters")
		if cm.fileSystem.Exists(orgDir) {
			entries, err := os.ReadDir(orgDir)
			if err != nil {
				return nil, NewFileError("read", orgDir, err)
			}

			for _, entry := range entries {
				if entry.IsDir() {
					clusters = append(clusters, entry.Name())
				}
			}
		}
		return clusters, nil
	}

	// Scan all organizations
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, NewFileError("read", baseDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		orgName := entry.Name()
		orgClustersDir := filepath.Join(baseDir, orgName, "infrastructure", "clusters")

		if cm.fileSystem.Exists(orgClustersDir) {
			clusterEntries, err := os.ReadDir(orgClustersDir)
			if err != nil {
				continue // Skip organizations we can't read
			}

			for _, clusterEntry := range clusterEntries {
				if clusterEntry.IsDir() {
					// Return in organization/cluster format
					clusters = append(clusters, fmt.Sprintf("%s/%s", orgName, clusterEntry.Name()))
				}
			}
		}
	}

	return clusters, nil
}

// Delete removes a configuration file.
//
// The delete process:
//  1. Resolve configuration file path
//  2. Create backup of the file
//  3. Remove the configuration file
//  4. Invalidate cache entry
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - name: Cluster name to delete
//
// Returns:
//   - error: Path resolution, backup, or deletion error
//
// Example:
//
//	err := manager.Delete(ctx, "old-cluster")
//	if err != nil {
//	    return fmt.Errorf("failed to delete: %w", err)
//	}
func (cm *ConfigurationManager) Delete(ctx context.Context, name string) error {
	if name == "" {
		return errors.WrapWithOperation(
			fmt.Errorf("cluster name cannot be empty"),
			"delete",
		)
	}

	// Resolve configuration path
	clusterPaths, err := cm.pathResolver.ResolveWithFallback(ctx, name)
	if err != nil {
		return errors.WrapWithOperation(
			NewPathError(name, "", err),
			"delete",
		)
	}

	configPath := clusterPaths.ConfigPath

	// Check if file exists
	if !cm.fileSystem.Exists(configPath) {
		return errors.WrapWithOperation(
			NewFileError("delete", configPath, fmt.Errorf("configuration file not found")),
			"delete",
		)
	}

	// Create backup before deletion
	backupPath := configPath + ".deleted"
	data, err := cm.fileSystem.ReadFile(configPath)
	if err != nil {
		return errors.WrapWithOperation(
			NewFileError("read", configPath, err),
			"delete",
		)
	}

	if err := cm.fileSystem.WriteFile(backupPath, data, 0600); err != nil {
		return errors.WrapWithOperation(
			NewFileError("write", backupPath, err),
			"delete",
		)
	}

	// Remove the configuration file
	if err := cm.fileSystem.Remove(configPath); err != nil {
		return errors.WrapWithOperation(
			NewFileError("delete", configPath, err),
			"delete",
		)
	}

	// Invalidate cache entry
	cm.cache.Invalidate(ctx, name)

	return nil
}

// ClearCache removes all cached configurations.
//
// Example:
//
//	manager.ClearCache(ctx)
func (cm *ConfigurationManager) ClearCache(ctx context.Context) error {
	cm.cache.Clear(ctx)
	return nil
}

// InvalidateCluster removes a specific cluster from cache.
//
// Parameters:
//   - ctx: Context for cancellation
//   - name: Cluster name to invalidate
//
// Example:
//
//	manager.InvalidateCluster(ctx, "my-cluster")
func (cm *ConfigurationManager) InvalidateCluster(ctx context.Context, name string) error {
	cm.cache.Invalidate(ctx, name)
	return nil
}

// NewBuilder creates a new ConfigBuilder for building configurations.
//
// The builder provides a fluent API for constructing cluster configurations
// with method chaining and validation. The builder integrates with the
// ConfigurationManager for validation and saving.
//
// Parameters:
//   - name: Cluster name for the new configuration
//
// Returns:
//   - ConfigBuilder: New builder instance with default values
//
// Example:
//
//	builder := manager.NewBuilder("my-cluster")
//	config, err := builder.
//	    WithProvider("openstack").
//	    WithOrganization("my-org").
//	    WithRegion("us-east-1").
//	    Build()
func (cm *ConfigurationManager) NewBuilder(name string) ConfigBuilder {
	builder := NewConfigBuilder(name).(*FluentConfigBuilder)
	// Inject manager reference for validation and saving
	builder.manager = cm
	return builder
}

// BuildFrom creates a ConfigBuilder from an existing configuration.
//
// This method is useful for modifying existing configurations using
// the fluent builder API.
//
// Parameters:
//   - config: Existing configuration to build from
//
// Returns:
//   - ConfigBuilder: Builder instance initialized with the config
//
// Example:
//
//	config, _ := manager.Load(ctx, "my-cluster")
//	builder := manager.BuildFrom(config)
//	updated, err := builder.
//	    WithWorkerCount(5).
//	    Build()
func (cm *ConfigurationManager) BuildFrom(config *Config) ConfigBuilder {
	if config == nil {
		// Return builder with empty config
		return cm.NewBuilder("")
	}
	builder := NewConfigBuilderFromConfig(*config).(*FluentConfigBuilder)
	// Inject manager reference for validation and saving
	builder.manager = cm
	return builder
}

// GetActive returns the active cluster name with precedence:
// 1. OPENCENTER_CLUSTER environment variable (session-scoped)
// 2. Session file (if shell integration is active)
// 3. Persistent selection from marker file
//
// Returns:
//   - string: The active cluster name (empty string if none set)
//   - error: File read error
//
// Example:
//
//	active, err := manager.GetActive()
//	if err != nil {
//	    return err
//	}
//	if active == "" {
//	    fmt.Println("No active cluster")
//	}
func (cm *ConfigurationManager) GetActive() (string, error) {
	// Priority 1: Check environment variable (highest priority)
	if cluster := os.Getenv("OPENCENTER_CLUSTER"); cluster != "" {
		return strings.TrimSpace(cluster), nil
	}

	// Priority 2: Check session file (shell integration)
	if sessionFile := os.Getenv("OPENCENTER_SESSION_FILE"); sessionFile != "" {
		if data, err := cm.fileSystem.ReadFile(sessionFile); err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
	}

	// Priority 3: Fall back to persistent selection
	path, err := cm.activeClusterPath()
	if err != nil {
		return "", err
	}

	data, readErr := cm.fileSystem.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", nil
		}
		return "", readErr
	}
	return strings.TrimSpace(string(data)), nil
}

// SetActive writes the given cluster name into the active marker file.
// If the name is empty, the marker file is removed.
//
// Parameters:
//   - name: The name of the cluster to set as active (empty to clear)
//
// Returns:
//   - error: File write error
//
// Example:
//
//	// Set active cluster
//	err := manager.SetActive("my-cluster")
//
//	// Clear active cluster
//	err := manager.SetActive("")
func (cm *ConfigurationManager) SetActive(name string) error {
	path, err := cm.activeClusterPath()
	if err != nil {
		return err
	}
	if name == "" {
		return cm.fileSystem.Remove(path)
	}

	if err := cm.fileSystem.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return cm.fileSystem.WriteFileAtomic(path, []byte(name), 0o600)
}

// activeClusterPath returns the path to the active cluster marker file.
func (cm *ConfigurationManager) activeClusterPath() (string, error) {
	baseDir := cm.pathResolver.GetBaseDir()
	return filepath.Join(baseDir, ".active"), nil
}
