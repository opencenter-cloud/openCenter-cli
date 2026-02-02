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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

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
	cache map[string]*Config

	// mu protects concurrent access to cache
	mu sync.RWMutex
}

// LoadStrategy defines the interface for version-specific configuration loaders.
// Each version (v1, v2, legacy) implements this interface to provide
// version-specific loading logic.
//
// Implementations must:
//   - Detect if they can load a configuration (CanLoad)
//   - Parse and load the configuration (Load)
//   - Return a version identifier (Version)
//
// Example implementation:
//
//	type V2Strategy struct{}
//
//	func (s *V2Strategy) CanLoad(data []byte) (bool, error) {
//	    var meta struct {
//	        SchemaVersion string `yaml:"schema_version"`
//	    }
//	    if err := yaml.Unmarshal(data, &meta); err != nil {
//	        return false, err
//	    }
//	    return meta.SchemaVersion == "2.0", nil
//	}
//
//	func (s *V2Strategy) Load(data []byte, clusterName string) (*Config, error) {
//	    // Parse v2 configuration...
//	}
//
//	func (s *V2Strategy) Version() string {
//	    return "2.0"
//	}
type LoadStrategy interface {
	// CanLoad determines if this strategy can load the given configuration data
	//
	// This method should quickly detect if the configuration matches this
	// strategy's version. It typically checks a version field in the YAML.
	//
	// Parameters:
	//   - data: Raw YAML configuration data
	//
	// Returns:
	//   - bool: true if this strategy can load the configuration
	//   - error: Detection failure (invalid YAML, etc.)
	CanLoad(data []byte) (bool, error)

	// Load loads and parses the configuration data
	//
	// This method performs the actual parsing and returns a fully populated
	// Config structure. It should handle version-specific logic.
	//
	// Parameters:
	//   - data: Raw YAML configuration data
	//   - clusterName: Name of the cluster (for metadata)
	//
	// Returns:
	//   - *Config: Parsed configuration
	//   - error: Parsing or validation failure
	Load(data []byte, clusterName string) (*Config, error)

	// Version returns the version identifier for this strategy
	//
	// The version should match the schema_version field in configurations
	// that this strategy can load (e.g., "1.0", "2.0", "legacy").
	//
	// Returns:
	//   - string: Version identifier
	Version() string
}

// LoadOptions configures the behavior of the Load operation.
//
// Options:
//   - AutoMigrate: Automatically upgrade older versions to current version
//   - Validate: Run validation after loading
//   - SkipCache: Force fresh load, bypassing cache
//
// Example:
//
//	opts := config.LoadOptions{
//	    AutoMigrate: true,  // Upgrade v1 to v2 automatically
//	    Validate: true,     // Validate after loading
//	    SkipCache: false,   // Use cache if available
//	}
//	cfg, err := manager.Load(path, opts)
type LoadOptions struct {
	// AutoMigrate automatically migrates older versions to the current version
	AutoMigrate bool

	// Validate runs validation after loading
	Validate bool

	// SkipCache bypasses the cache and forces a fresh load
	SkipCache bool
}

// NewConfigManager creates a new ConfigManager with all supported strategies registered.
//
// The manager is created with an empty strategy registry and cache.
// Strategies must be registered using RegisterStrategy before loading configurations.
//
// Example:
//
//	manager := config.NewConfigManager()
//	manager.RegisterStrategy(strategies.NewV2Strategy())
//	manager.RegisterStrategy(strategies.NewV1Strategy())
//
//	cfg, err := manager.Load(path, config.LoadOptions{AutoMigrate: true})
func NewConfigManager() *ConfigManager {
	cm := &ConfigManager{
		strategies: make(map[string]LoadStrategy),
		cache:      make(map[string]*Config),
	}

	// Register strategies for each supported version
	// Import strategies package to get concrete implementations
	// Note: We use lazy registration pattern - strategies register themselves
	// when their package is imported. Strategies can also be registered
	// externally using the RegisterStrategy method.

	return cm
}

// RegisterStrategy registers a new loading strategy for a specific version.
//
// Strategies are identified by their Version() string. Registering a strategy
// with a duplicate version replaces the existing strategy.
//
// Parameters:
//   - strategy: The loading strategy to register
//
// Example:
//
//	manager.RegisterStrategy(strategies.NewV2Strategy())
//	manager.RegisterStrategy(strategies.NewV1Strategy())
func (cm *ConfigManager) RegisterStrategy(strategy LoadStrategy) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.strategies[strategy.Version()] = strategy
}

// Load reads and unmarshals a configuration file with automatic version detection.
// It uses the registered strategies to determine the correct loader based on the
// file content, then applies the appropriate loading logic.
//
// In v2.0.0, this method rejects v1 configurations with a clear error message
// directing users to migrate using v1.x before upgrading.
//
// The loading process:
//  1. Check cache (unless SkipCache is true)
//  2. Read file from disk
//  3. Detect version using registered strategies (rejects v1)
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
func (cm *ConfigManager) Load(path string, opts LoadOptions) (*Config, error) {
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
	// This will reject v1 configurations in v2.0.0
	strategy, err := cm.detectStrategy(data)
	if err != nil {
		// Check if this is a v1 config error and add path information
		var v1Err *V1ConfigError
		if errors.As(err, &v1Err) {
			v1Err.Path = path
			return nil, v1Err
		}
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
func (cm *ConfigManager) Save(path string, config *Config) error {
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
// Call this after:
//   - Modifying a configuration file externally
//   - Saving a configuration with Save()
//   - Detecting configuration changes
//
// Parameters:
//   - path: Path to the configuration file to invalidate
//
// Example:
//
//	manager.Save(path, config)
//	manager.InvalidateCache(path) // Force fresh load next time
func (cm *ConfigManager) InvalidateCache(path string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.cache, path)
}

// getFromCache retrieves a configuration from the cache if it exists.
func (cm *ConfigManager) getFromCache(path string) *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cache[path]
}

// putInCache stores a configuration in the cache.
func (cm *ConfigManager) putInCache(path string, config *Config) {
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
// In v2.0.0, this rejects v1 configurations with a clear error message.
func (cm *ConfigManager) detectStrategy(data []byte) (LoadStrategy, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// First, check if this is a v1 configuration and reject it
	if isV1Config(data) {
		return nil, fmt.Errorf("v1 configuration detected: %w", &V1ConfigError{
			Message: "v1 configurations are not supported in v2.0.0",
			Suggestion: `To upgrade to v2.0.0:
1. Install opencenter v1.x (latest version)
2. Run: opencenter cluster migrate-config <cluster-name>
3. Verify the migrated configuration
4. Upgrade to opencenter v2.0.0

For more information, see: https://docs.opencenter.io/migration/v1-to-v2`,
		})
	}

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

// isV1Config checks if the configuration data is a v1 configuration.
// It checks for schema_version: "1.0" or missing schema_version (which defaults to v1).
func isV1Config(data []byte) bool {
	var versionCheck struct {
		SchemaVersion string `yaml:"schema_version"`
	}

	if err := yaml.Unmarshal(data, &versionCheck); err != nil {
		return false
	}

	// V1 is identified by explicit "1.0" or missing version
	return versionCheck.SchemaVersion == "1.0" || versionCheck.SchemaVersion == ""
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
func (cm *ConfigManager) migrateToCurrentVersion(cfg *Config) (*Config, error) {
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

// validateConfig validates a configuration using the validation system.
// Currently returns nil as validation is performed during unmarshaling.
// Future enhancements will integrate with the validation engine.
func (cm *ConfigManager) validateConfig(cfg *Config) error {
	// Basic validation is performed during unmarshaling
	// Additional validation can be added here as needed
	return nil
}

// CurrentSchemaVersion is the current schema version that configurations
// should be migrated to when AutoMigrate is enabled.
const CurrentSchemaVersion = "1.0"
