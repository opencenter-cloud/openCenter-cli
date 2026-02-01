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

	"gopkg.in/yaml.v3"

	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/config/migration"
)

// ConfigManager provides unified configuration management with version detection,
// migration, and caching capabilities. It uses a strategy pattern to handle
// different configuration versions (v1, v2, legacy) through a single interface.
//
// Key Features:
//   - Auto-detection of configuration version from YAML
//   - Strategy-based loading for version-specific logic
//   - Integrated migration pipeline for version upgrades
//   - Thread-safe caching with invalidation
//   - Single Load() method handles all versions
//
// Usage:
//
//	manager := NewConfigManager()
//	config, err := manager.Load(path, LoadOptions{
//	    AutoMigrate: true,
//	    Validate: true,
//	})
type ConfigManager struct {
	// strategies maps version identifiers to their loading strategies
	strategies map[string]LoadStrategy

	// cache stores loaded configurations by path
	cache map[string]*internalconfig.Config

	// mu protects concurrent access to cache
	mu sync.RWMutex
}

// LoadStrategy defines the interface for version-specific configuration loaders.
// Each version (v1, v2, legacy) implements this interface to provide
// version-specific loading logic.
type LoadStrategy interface {
	// CanLoad determines if this strategy can load the given configuration data
	CanLoad(data []byte) (bool, error)

	// Load loads and parses the configuration data
	Load(data []byte, clusterName string) (*internalconfig.Config, error)

	// Version returns the version identifier for this strategy
	Version() string
}

// LoadOptions configures the behavior of the Load operation.
type LoadOptions struct {
	// AutoMigrate automatically migrates older versions to the current version
	AutoMigrate bool

	// Validate runs validation after loading
	Validate bool

	// SkipCache bypasses the cache and forces a fresh load
	SkipCache bool
}

// NewConfigManager creates a new ConfigManager with all supported strategies registered.
func NewConfigManager() *ConfigManager {
	cm := &ConfigManager{
		strategies: make(map[string]LoadStrategy),
		cache:      make(map[string]*internalconfig.Config),
	}

	// Register strategies for each supported version
	// Import strategies package to get concrete implementations
	// Note: We'll use lazy registration pattern - strategies register themselves
	// when their package is imported. For now, we provide a method to register
	// strategies externally.

	return cm
}

// RegisterStrategy registers a new loading strategy for a specific version.
func (cm *ConfigManager) RegisterStrategy(strategy LoadStrategy) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.strategies[strategy.Version()] = strategy
}

// Load reads and unmarshals a configuration file with automatic version detection.
// It uses the registered strategies to determine the correct loader based on the
// file content, then applies the appropriate loading logic.
//
// The loading process:
//  1. Check cache (unless SkipCache is true)
//  2. Read file from disk
//  3. Detect version using registered strategies
//  4. Load using the appropriate strategy
//  5. Optionally migrate to current version
//  6. Optionally validate the configuration
//  7. Cache the result
//
// Inputs:
//   - path: Absolute path to the configuration file
//   - opts: Loading options (auto-migrate, validate, skip cache)
//
// Outputs:
//   - *Config: The loaded configuration
//   - error: An error if loading fails
func (cm *ConfigManager) Load(path string, opts LoadOptions) (*internalconfig.Config, error) {
	// Check cache first (unless skip cache is requested)
	if !opts.SkipCache {
		if cached := cm.getFromCache(path); cached != nil {
			return cached, nil
		}
	}

	// Read file from disk
	data, err := cm.readFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Detect version and select appropriate strategy
	strategy, err := cm.detectStrategy(data)
	if err != nil {
		return nil, fmt.Errorf("failed to detect configuration version: %w", err)
	}

	// Extract cluster name from path for loading
	clusterName := cm.extractClusterName(path)

	// Load using the selected strategy
	cfg, err := strategy.Load(data, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Optionally migrate to current version
	if opts.AutoMigrate && cfg.SchemaVersion != CurrentSchemaVersion {
		cfg, err = cm.migrateToCurrentVersion(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate configuration: %w", err)
		}
	}

	// Optionally validate the configuration
	if opts.Validate {
		if err := cm.validateConfig(cfg); err != nil {
			return nil, fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	// Cache the result
	cm.putInCache(path, cfg)

	return cfg, nil
}

// Save writes the configuration to a YAML file with proper permissions (0600).
// The file is saved at the path determined by the configuration's cluster name
// and organization.
//
// Inputs:
//   - path: Absolute path where to save the configuration
//   - config: The configuration to save
//
// Outputs:
//   - error: An error if saving fails
func (cm *ConfigManager) Save(path string, config *internalconfig.Config) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal configuration to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration to YAML: %w", err)
	}

	// Write to file with secure permissions (0600 - owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write configuration file %s: %w", path, err)
	}

	// Invalidate cache to ensure fresh load on next access
	cm.InvalidateCache(path)

	return nil
}

// InvalidateCache removes a configuration from the cache, forcing a fresh load
// on the next Load() call.
//
// Inputs:
//   - path: Path to the configuration file to invalidate
func (cm *ConfigManager) InvalidateCache(path string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.cache, path)
}

// getFromCache retrieves a configuration from the cache if it exists.
func (cm *ConfigManager) getFromCache(path string) *internalconfig.Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cache[path]
}

// putInCache stores a configuration in the cache.
func (cm *ConfigManager) putInCache(path string, config *internalconfig.Config) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cache[path] = config
}

// readFile reads the configuration file from disk.
func (cm *ConfigManager) readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return data, nil
}

// detectStrategy determines which loading strategy to use based on file content.
func (cm *ConfigManager) detectStrategy(data []byte) (LoadStrategy, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Try each strategy to see which one can load this configuration
	for _, strategy := range cm.strategies {
		canLoad, err := strategy.CanLoad(data)
		if err != nil {
			// Log error but continue trying other strategies
			continue
		}
		if canLoad {
			return strategy, nil
		}
	}

	return nil, fmt.Errorf("no suitable loading strategy found for configuration")
}

// extractClusterName extracts the cluster name from the configuration file path.
// Expected path format: .../clusters/<org>/<cluster>-config.yaml or .../<cluster>-config.yaml
func (cm *ConfigManager) extractClusterName(path string) string {
	// Get the base filename
	base := filepath.Base(path)

	// Remove file extension
	name := strings.TrimSuffix(base, filepath.Ext(base))

	// Remove common suffixes like "-config" or ".config"
	name = strings.TrimSuffix(name, "-config")
	name = strings.TrimPrefix(name, ".")

	return name
}

// migrateToCurrentVersion migrates a configuration to the current schema version.
func (cm *ConfigManager) migrateToCurrentVersion(cfg *internalconfig.Config) (*internalconfig.Config, error) {
	// Create a migrator with all registered migrations
	migrator := cm.createMigrator()

	// Migrate to current version
	migrated, err := migrator.Migrate(cfg, CurrentSchemaVersion)
	if err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return migrated, nil
}

// createMigrator creates a migrator with all available migrations registered.
func (cm *ConfigManager) createMigrator() *migration.Migrator {
	migrator := migration.NewMigrator()

	// Register all available migrations
	// Legacy -> V1
	if err := migrator.Register(migration.LegacyToV1Migration()); err != nil {
		// Log error but continue - migration may already be registered
		_ = err
	}

	// V1 -> V2
	if err := migrator.Register(migration.V1ToV2Migration()); err != nil {
		// Log error but continue - migration may already be registered
		_ = err
	}

	return migrator
}

// validateConfig validates a configuration using the existing validation system.
func (cm *ConfigManager) validateConfig(cfg *internalconfig.Config) error {
	// Use the existing validator from the config package
	validator := internalconfig.NewConfigValidator(false)
	result := validator.Validate(context.Background(), cfg)

	if !result.Valid {
		// Collect all error messages
		var errMsgs []string
		for _, err := range result.Errors {
			errMsgs = append(errMsgs, err.Message)
		}
		return fmt.Errorf("validation failed: %s", strings.Join(errMsgs, "; "))
	}

	return nil
}

// CurrentSchemaVersion is the current schema version that configurations
// should be migrated to when AutoMigrate is enabled.
const CurrentSchemaVersion = "1.0"
