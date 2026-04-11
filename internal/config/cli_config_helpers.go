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
	"os"
	"path/filepath"

	configpersistence "github.com/opencenter-cloud/opencenter-cli/internal/config/persistence"
	corePaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

// ResolveClustersDir returns the runtime clusters directory.
// If OPENCENTER_CONFIG_DIR is set, it is treated as the config root and
// the clusters directory is resolved beneath it. Otherwise, the CLI config
// clustersDir value is used, falling back to the default clusters path.
func ResolveClustersDir() string {
	// Prefer an explicit CLI configuration override when one exists, even when
	// OPENCENTER_CONFIG_DIR is only being used to point at the CLI config file.
	if cliConfigManager, err := NewConfigManager(""); err == nil && cliConfigManager != nil {
		clustersDir := cliConfigManager.GetConfig().Paths.ClustersDir
		if clustersDir != "" {
			return normalizeDirectoryPath(clustersDir)
		}
	}

	if dir := os.Getenv("OPENCENTER_CONFIG_DIR"); dir != "" {
		return normalizeDirectoryPath(filepath.Join(dir, "clusters"))
	}

	return normalizeDirectoryPath(filepath.Join(DefaultConfigDir(), "clusters"))
}

// GetClustersDir returns the clusters directory from the CLI config.
// If the CLI config cannot be loaded or clustersDir is not set, it returns the default.
// This function is safe to call from anywhere and will not cause circular dependencies.
func GetClustersDir() string {
	// Try to load CLI config (but don't fail if it doesn't exist)
	cliConfigManager, err := NewConfigManager("")
	if err == nil && cliConfigManager != nil {
		clustersDir := cliConfigManager.GetConfig().Paths.ClustersDir
		if clustersDir != "" {
			return normalizeDirectoryPath(clustersDir)
		}
	}

	// Fallback to default
	return normalizeDirectoryPath(filepath.Join(DefaultConfigDir(), "clusters"))
}

// GetConfigDir returns the configuration directory from the CLI config.
// If the CLI config cannot be loaded or configDir is not set, it returns the default.
func GetConfigDir() string {
	// Try to load CLI config (but don't fail if it doesn't exist)
	cliConfigManager, err := NewConfigManager("")
	if err == nil && cliConfigManager != nil {
		configDir := cliConfigManager.GetConfig().Paths.ConfigDir
		if configDir != "" {
			return normalizeDirectoryPath(configDir)
		}
	}

	// Fallback to default
	return DefaultConfigDir()
}

// GetPluginsDir returns the plugins directory from the CLI config.
// If the CLI config cannot be loaded or pluginsDir is not set, it returns the default.
func GetPluginsDir() string {
	// Try to load CLI config (but don't fail if it doesn't exist)
	cliConfigManager, err := NewConfigManager("")
	if err == nil && cliConfigManager != nil {
		pluginsDir := cliConfigManager.GetConfig().Paths.PluginsDir
		if pluginsDir != "" {
			return normalizeDirectoryPath(pluginsDir)
		}
	}

	// Fallback to default
	return normalizeDirectoryPath(filepath.Join(DefaultConfigDir(), "plugins"))
}

// GetStateDir returns the runtime state directory using the precedence:
// OPENCENTER_STATE_DIR, CLI config paths.stateDir, then the platform default.
func GetStateDir() string {
	if stateDir := os.Getenv("OPENCENTER_STATE_DIR"); stateDir != "" {
		return normalizeDirectoryPath(stateDir)
	}

	if cliConfigManager, err := NewConfigManager(""); err == nil && cliConfigManager != nil {
		stateDir := cliConfigManager.GetConfig().Paths.StateDir
		if stateDir != "" {
			return normalizeDirectoryPath(stateDir)
		}
	}

	return DefaultStateDir()
}

func normalizeDirectoryPath(path string) string {
	expanded := corePaths.ExpandPath(path)
	if normalized, err := configpersistence.NormalizeDir(expanded); err == nil {
		return normalized
	}
	return expanded
}
