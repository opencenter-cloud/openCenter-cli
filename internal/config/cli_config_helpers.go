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
)

// GetClustersDir returns the clusters directory from the CLI config.
// If the CLI config cannot be loaded or clustersDir is not set, it returns the default.
// This function is safe to call from anywhere and will not cause circular dependencies.
func GetClustersDir() string {
	// Try to load CLI config (but don't fail if it doesn't exist)
	cliConfigManager, err := NewConfigManager("")
	if err == nil && cliConfigManager != nil {
		clustersDir := cliConfigManager.GetConfig().Paths.ClustersDir
		if clustersDir != "" {
			return clustersDir
		}
	}

	// Fallback to default
	return filepath.Join(getDefaultConfigDir(), "clusters")
}

// GetConfigDir returns the configuration directory from the CLI config.
// If the CLI config cannot be loaded or configDir is not set, it returns the default.
func GetConfigDir() string {
	// Try to load CLI config (but don't fail if it doesn't exist)
	cliConfigManager, err := NewConfigManager("")
	if err == nil && cliConfigManager != nil {
		configDir := cliConfigManager.GetConfig().Paths.ConfigDir
		if configDir != "" {
			return configDir
		}
	}

	// Fallback to default
	return getDefaultConfigDir()
}

// GetPluginsDir returns the plugins directory from the CLI config.
// If the CLI config cannot be loaded or pluginsDir is not set, it returns the default.
func GetPluginsDir() string {
	// Try to load CLI config (but don't fail if it doesn't exist)
	cliConfigManager, err := NewConfigManager("")
	if err == nil && cliConfigManager != nil {
		pluginsDir := cliConfigManager.GetConfig().Paths.PluginsDir
		if pluginsDir != "" {
			return pluginsDir
		}
	}

	// Fallback to default
	return filepath.Join(getDefaultConfigDir(), "plugins")
}

// getDefaultConfigDir returns the default configuration directory.
// This is a helper function used by the other Get*Dir functions.
func getDefaultConfigDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		// Try to get home directory
		if homeDir, err := os.UserHomeDir(); err == nil {
			home = homeDir
		} else {
			// Last resort fallback
			return "/tmp/opencenter"
		}
	}
	return filepath.Join(home, ".config", "opencenter")
}
