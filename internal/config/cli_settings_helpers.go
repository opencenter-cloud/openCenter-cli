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
// If OPENCENTER_CLUSTERS_DIR is set, it is used as the cluster storage root.
// Otherwise, the CLI config clustersDir value is used, falling back to
// OPENCENTER_CONFIG_DIR/clusters or the default clusters path.
func ResolveClustersDir() string {
	if dir := clustersDirFromEnv(); dir != "" {
		return normalizeDirectoryPath(dir)
	}

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
// OPENCENTER_CLUSTERS_DIR overrides the CLI config. If the CLI config cannot be
// loaded or clustersDir is not set, it returns the default.
// This function is safe to call from anywhere and will not cause circular dependencies.
func GetClustersDir() string {
	if dir := clustersDirFromEnv(); dir != "" {
		return normalizeDirectoryPath(dir)
	}

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

// GetGitOpsDir returns the GitOps repository root using the precedence:
// OPENCENTER_GITOPS_DIR, CLI settings paths.gitopsDir, then clustersDir/gitops.
func GetGitOpsDir() string {
	if gitopsDir := os.Getenv("OPENCENTER_GITOPS_DIR"); gitopsDir != "" {
		return normalizeDirectoryPath(gitopsDir)
	}

	if cliConfigManager, err := NewConfigManager(""); err == nil && cliConfigManager != nil {
		gitopsDir := cliConfigManager.GetConfig().Paths.GitOpsDir
		if gitopsDir != "" {
			return normalizeDirectoryPath(gitopsDir)
		}
	}

	return normalizeDirectoryPath(filepath.Join(GetClustersDir(), "gitops"))
}

// GetBlueprintsDir returns the cluster blueprints root using the precedence:
// OPENCENTER_BLUEPRINTS_DIR, CLI settings paths.blueprintsDir, then clustersDir/blueprints.
func GetBlueprintsDir() string {
	if blueprintsDir := os.Getenv("OPENCENTER_BLUEPRINTS_DIR"); blueprintsDir != "" {
		return normalizeDirectoryPath(blueprintsDir)
	}

	if cliConfigManager, err := NewConfigManager(""); err == nil && cliConfigManager != nil {
		blueprintsDir := cliConfigManager.GetConfig().Paths.BlueprintsDir
		if blueprintsDir != "" {
			return normalizeDirectoryPath(blueprintsDir)
		}
	}

	return normalizeDirectoryPath(filepath.Join(GetClustersDir(), "blueprints"))
}

// GetClusterStateDir returns the per-cluster state root using the precedence:
// OPENCENTER_CLUSTER_STATE_DIR, CLI settings paths.clusterStateDir, then clustersDir/state.
func GetClusterStateDir() string {
	if stateDir := os.Getenv("OPENCENTER_CLUSTER_STATE_DIR"); stateDir != "" {
		return normalizeDirectoryPath(stateDir)
	}

	if cliConfigManager, err := NewConfigManager(""); err == nil && cliConfigManager != nil {
		stateDir := cliConfigManager.GetConfig().Paths.ClusterStateDir
		if stateDir != "" {
			return normalizeDirectoryPath(stateDir)
		}
	}

	return normalizeDirectoryPath(filepath.Join(GetClustersDir(), "state"))
}

// GetSecretsDir returns the per-cluster secrets root using the precedence:
// OPENCENTER_SECRETS_DIR, CLI config paths.secretsDir, then clustersDir/secrets.
func GetSecretsDir() string {
	if secretsDir := os.Getenv("OPENCENTER_SECRETS_DIR"); secretsDir != "" {
		return normalizeDirectoryPath(secretsDir)
	}

	if cliConfigManager, err := NewConfigManager(""); err == nil && cliConfigManager != nil {
		secretsDir := cliConfigManager.GetConfig().Paths.SecretsDir
		if secretsDir != "" {
			return normalizeDirectoryPath(secretsDir)
		}
	}

	return normalizeDirectoryPath(filepath.Join(GetClustersDir(), "secrets"))
}

// NewPathResolverFromConfig returns a secure zone-aware path resolver using the
// current CLI settings and environment variable precedence.
func NewPathResolverFromConfig() *corePaths.PathResolver {
	return corePaths.NewPathResolverWithRoots(
		GetClustersDir(),
		GetBlueprintsDir(),
		GetGitOpsDir(),
		GetClusterStateDir(),
		GetSecretsDir(),
		corePaths.DefaultResolutionOptions(),
	)
}

// GetConfigDir returns the settings directory from the CLI config.
// If the CLI config cannot be loaded or settingsDir is not set, it returns the default.
func GetConfigDir() string {
	if configDir := os.Getenv("OPENCENTER_CONFIG_DIR"); configDir != "" {
		return normalizeDirectoryPath(configDir)
	}

	// Try to load CLI config (but don't fail if it doesn't exist)
	cliConfigManager, err := NewConfigManager("")
	if err == nil && cliConfigManager != nil {
		settingsDir := cliConfigManager.GetConfig().Paths.SettingsDir
		if settingsDir != "" {
			return normalizeDirectoryPath(settingsDir)
		}
	}

	// Fallback to default
	return DefaultConfigDir()
}

// GetPluginsDir returns the plugins directory from the CLI config.
// If the CLI config cannot be loaded or pluginsDir is not set, it returns the default.
func GetPluginsDir() string {
	if pluginsDir := os.Getenv("OPENCENTER_PLUGINS_DIR"); pluginsDir != "" {
		return normalizeDirectoryPath(pluginsDir)
	}

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

func clustersDirFromEnv() string {
	return os.Getenv("OPENCENTER_CLUSTERS_DIR")
}

func normalizeDirectoryPath(path string) string {
	expanded := corePaths.ExpandPath(path)
	if normalized, err := configpersistence.NormalizeDir(expanded); err == nil {
		return normalized
	}
	return expanded
}
