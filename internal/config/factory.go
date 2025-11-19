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
	"time"
)

// ConfigManagerFactory provides factory methods for creating configuration management components.
type ConfigManagerFactory struct {
	cliConfigManager *ConfigManager
}

// NewConfigManagerFactory creates a new configuration manager factory.
func NewConfigManagerFactory(cliConfigManager *ConfigManager) *ConfigManagerFactory {
	return &ConfigManagerFactory{
		cliConfigManager: cliConfigManager,
	}
}

// CreateConfigurationManager creates a fully configured ConfigurationManager with all dependencies.
func (f *ConfigManagerFactory) CreateConfigurationManager() *ConfigurationManager {
	// Create path resolver
	pathResolver := NewPathResolverImpl(f.cliConfigManager)

	// Create loader
	loader := NewConfigLoader(pathResolver)

	// Create validator
	validator := NewConfigValidator(true) // Enable auto-repair

	// Create cache
	cache := NewInMemoryConfigCache(5*time.Minute, 100)

	// Create migrator
	migrator := NewConfigMigrator(pathResolver, loader, validator)

	// Create configuration manager
	return NewConfigurationManager(loader, validator, pathResolver, cache, migrator)
}

// CreatePathResolver creates a path resolver with the CLI configuration manager.
func (f *ConfigManagerFactory) CreatePathResolver() PathResolverInterface {
	return NewPathResolverImpl(f.cliConfigManager)
}

// CreateConfigLoader creates a configuration loader with path resolver.
func (f *ConfigManagerFactory) CreateConfigLoader() ConfigLoaderInterface {
	pathResolver := f.CreatePathResolver()
	return NewConfigLoader(pathResolver)
}

// CreateConfigValidator creates a configuration validator.
func (f *ConfigManagerFactory) CreateConfigValidator(autoRepair bool) ConfigValidatorInterface {
	return NewConfigValidator(autoRepair)
}

// CreateConfigCache creates a configuration cache with specified settings.
func (f *ConfigManagerFactory) CreateConfigCache(defaultTTL time.Duration, maxSize int) ConfigCacheInterface {
	return NewInMemoryConfigCache(defaultTTL, maxSize)
}

// CreateConfigMigrator creates a configuration migrator with all dependencies.
func (f *ConfigManagerFactory) CreateConfigMigrator() ConfigMigratorInterface {
	pathResolver := f.CreatePathResolver()
	loader := f.CreateConfigLoader()
	validator := f.CreateConfigValidator(false) // Don't auto-repair during migration

	return NewConfigMigrator(pathResolver, loader, validator)
}

// CreateMinimalConfigurationManager creates a configuration manager with minimal dependencies.
// This is useful for scenarios where caching or migration is not needed.
func (f *ConfigManagerFactory) CreateMinimalConfigurationManager() *ConfigurationManager {
	// Create path resolver
	pathResolver := NewPathResolverImpl(f.cliConfigManager)

	// Create loader
	loader := NewConfigLoader(pathResolver)

	// Create validator
	validator := NewConfigValidator(false) // Disable auto-repair for minimal setup

	// Create simple cache
	cache := NewInMemoryConfigCache(1*time.Minute, 10)

	// No migrator for minimal setup
	var migrator ConfigMigratorInterface = nil

	// Create configuration manager
	return NewConfigurationManager(loader, validator, pathResolver, cache, migrator)
}

// CreateTestConfigurationManager creates a configuration manager suitable for testing.
func (f *ConfigManagerFactory) CreateTestConfigurationManager() *ConfigurationManager {
	// Create path resolver
	pathResolver := NewPathResolverImpl(f.cliConfigManager)

	// Create loader
	loader := NewConfigLoader(pathResolver)

	// Create validator with auto-repair disabled for predictable testing
	validator := NewConfigValidator(false)

	// Create cache with short TTL for testing
	cache := NewInMemoryConfigCache(10*time.Second, 5)

	// Create migrator
	migrator := NewConfigMigrator(pathResolver, loader, validator)

	// Create configuration manager
	configManager := NewConfigurationManager(loader, validator, pathResolver, cache, migrator)

	// Disable caching for testing to ensure fresh loads
	configManager.SetCacheEnabled(false)

	return configManager
}

// GetDefaultFactory creates a factory with default CLI configuration manager.
func GetDefaultFactory() (*ConfigManagerFactory, error) {
	cliConfigManager, err := NewConfigManager("")
	if err != nil {
		return nil, err
	}

	return NewConfigManagerFactory(cliConfigManager), nil
}

// CreateStandaloneComponents creates individual components without a full configuration manager.
// This is useful when you only need specific functionality.
type StandaloneComponents struct {
	PathResolver PathResolverInterface
	Loader       ConfigLoaderInterface
	Validator    ConfigValidatorInterface
	Cache        ConfigCacheInterface
	Migrator     ConfigMigratorInterface
}

// CreateStandaloneComponents creates individual components that can be used independently.
func (f *ConfigManagerFactory) CreateStandaloneComponents() *StandaloneComponents {
	pathResolver := NewPathResolverImpl(f.cliConfigManager)
	loader := NewConfigLoader(pathResolver)
	validator := NewConfigValidator(true)
	cache := NewInMemoryConfigCache(5*time.Minute, 100)
	migrator := NewConfigMigrator(pathResolver, loader, validator)

	return &StandaloneComponents{
		PathResolver: pathResolver,
		Loader:       loader,
		Validator:    validator,
		Cache:        cache,
		Migrator:     migrator,
	}
}

// ConfigManagerOptions provides options for creating configuration managers.
type ConfigManagerOptions struct {
	EnableCache      bool
	CacheTTL         time.Duration
	CacheMaxSize     int
	EnableAutoRepair bool
	EnableMigration  bool
}

// DefaultConfigManagerOptions returns default options for configuration manager creation.
func DefaultConfigManagerOptions() *ConfigManagerOptions {
	return &ConfigManagerOptions{
		EnableCache:      true,
		CacheTTL:         5 * time.Minute,
		CacheMaxSize:     100,
		EnableAutoRepair: true,
		EnableMigration:  true,
	}
}

// CreateConfigurationManagerWithOptions creates a configuration manager with custom options.
func (f *ConfigManagerFactory) CreateConfigurationManagerWithOptions(opts *ConfigManagerOptions) *ConfigurationManager {
	if opts == nil {
		opts = DefaultConfigManagerOptions()
	}

	// Create path resolver
	pathResolver := NewPathResolverImpl(f.cliConfigManager)

	// Create loader
	loader := NewConfigLoader(pathResolver)

	// Create validator
	validator := NewConfigValidator(opts.EnableAutoRepair)

	// Create cache
	var cache ConfigCacheInterface
	if opts.EnableCache {
		cache = NewInMemoryConfigCache(opts.CacheTTL, opts.CacheMaxSize)
	} else {
		cache = NewInMemoryConfigCache(0, 1) // Minimal cache that effectively disables caching
	}

	// Create migrator
	var migrator ConfigMigratorInterface
	if opts.EnableMigration {
		migrator = NewConfigMigrator(pathResolver, loader, validator)
	}

	// Create configuration manager
	configManager := NewConfigurationManager(loader, validator, pathResolver, cache, migrator)

	// Configure caching
	configManager.SetCacheEnabled(opts.EnableCache)
	if opts.EnableCache {
		configManager.SetCacheTimeout(opts.CacheTTL)
	}

	return configManager
}
