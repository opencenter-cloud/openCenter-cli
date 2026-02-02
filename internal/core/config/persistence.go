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
	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
)

// This file provides configuration persistence operations including loading,
// saving, and managing cluster configurations on disk.
//
// The persistence layer handles:
//   - Reading and writing YAML configuration files
//   - Path resolution for cluster configurations
//   - Active cluster tracking and selection
//   - Configuration directory management
//   - Schema version validation
//
// All functions delegate to the internal/config package for the actual
// implementation while providing a clean API for the core config layer.

// Load reads and unmarshals a configuration file for the given cluster name.
// It validates the schema version and returns an error if the config is not v2.
//
// The function enforces v2-only support as part of the v2.0.0 breaking changes.
// Configurations with schema version 1.0 or empty will return a V1ConfigError
// with instructions to upgrade using v1.x first.
//
// Parameters:
//   - name: The cluster name (can be "cluster" or "organization/cluster")
//
// Returns:
//   - *Config: The loaded configuration
//   - error: An error if the file doesn't exist, can't be parsed, or is not v2
func Load(name string) (*Config, error) {
	// Delegate to the internal/config implementation
	oldCfg, err := internalconfig.Load(name)
	if err != nil {
		return nil, err
	}
	
	// Convert to core config type (they're the same via type alias)
	cfg := Config(oldCfg)
	
	// Validate schema version for v2-only support
	if cfg.SchemaVersion != "2.0" {
		// Get the config path for error message
		configPath, _ := internalconfig.ConfigPath(name)
		
		// Return V1ConfigError for v1 configs, UnsupportedVersionError for others
		if cfg.SchemaVersion == "" || cfg.SchemaVersion == "1.0" {
			return nil, NewV1ConfigError(configPath)
		}
		return nil, &UnsupportedVersionError{
			Version: cfg.SchemaVersion,
			Path:    configPath,
		}
	}
	
	return &cfg, nil
}

// Save writes the configuration to a YAML file with 0600 permissions.
// The file is saved to the standard configuration directory based on the
// cluster name and organization structure.
//
// Parameters:
//   - cfg: The configuration to save
//
// Returns:
//   - error: An error if the configuration cannot be saved
func Save(cfg *Config) error {
	// Convert core config to internal config (they're the same via type alias)
	internalCfg := internalconfig.Config(*cfg)
	return internalconfig.Save(internalCfg)
}

// SaveWithOmitEmpty writes the configuration to a YAML file, omitting empty fields.
// The file is saved with 0600 permissions to protect sensitive data.
// This is useful for cleaning up configurations by removing fields with zero values.
//
// Parameters:
//   - cfg: The configuration to save
//
// Returns:
//   - error: An error if the configuration cannot be saved
func SaveWithOmitEmpty(cfg *Config) error {
	// Convert core config to internal config (they're the same via type alias)
	internalCfg := internalconfig.Config(*cfg)
	return internalconfig.SaveWithOmitEmpty(internalCfg)
}

// ResolveConfigDir resolves the configuration directory based on the
// OPENCENTER_CONFIG_DIR environment variable or the user's standard config directory.
// The directory is created if it doesn't exist.
//
// Returns:
//   - string: The absolute path to the configuration directory
//   - error: An error if the directory cannot be resolved or created
func ResolveConfigDir() (string, error) {
	return internalconfig.ResolveConfigDir()
}

// ConfigPath returns the absolute path to a cluster's configuration file.
// The path follows the organization-based structure:
// ~/.config/opencenter/clusters/<organization>/.<cluster>-config.yaml
//
// Parameters:
//   - name: The name of the cluster (can be "cluster" or "organization/cluster")
//
// Returns:
//   - string: The absolute path to the configuration file
//   - error: An error if the path cannot be resolved
func ConfigPath(name string) (string, error) {
	return internalconfig.ConfigPath(name)
}

// List returns a sorted list of cluster names from the configuration directory.
// The list includes all clusters across all organizations.
//
// Returns:
//   - []string: A list of cluster names in "organization/cluster" format
//   - error: An error if the directory cannot be read
func List() ([]string, error) {
	return internalconfig.List()
}

// SetActive writes the given cluster name into the active marker file.
// If the name is empty, the marker file is removed.
// The active cluster is used as the default when no cluster is specified.
//
// Parameters:
//   - name: The name of the cluster to set as active
//
// Returns:
//   - error: An error if the marker file cannot be written
func SetActive(name string) error {
	return internalconfig.SetActive(name)
}

// GetActive reads the active cluster name with precedence:
// 1. OPENCENTER_CLUSTER environment variable (session-scoped)
// 2. Session file (if shell integration is active)
// 3. Persistent selection from marker file
//
// Returns:
//   - string: The active cluster name, or empty string if none is set
//   - error: An error if the marker file cannot be read
func GetActive() (string, error) {
	return internalconfig.GetActive()
}
