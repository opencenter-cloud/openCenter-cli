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

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
	"gopkg.in/yaml.v3"
)

var (
	// Global configuration manager instance (lazy-initialized)
	globalConfigManager *config.ConfigurationManager
	configManagerOnce   sync.Once
	configManagerErr    error
)

// resetConfigManagerForTests resets the lazy global configuration manager.
// This keeps command tests isolated when OPENCENTER_CONFIG_DIR changes between runs.
func resetConfigManagerForTests() {
	globalConfigManager = nil
	configManagerErr = nil
	configManagerOnce = sync.Once{}
}

// getConfigManager returns the global ConfigurationManager instance.
// It initializes the manager on first call and reuses it for subsequent calls.
func getConfigManager() (*config.ConfigurationManager, error) {
	configManagerOnce.Do(func() {
		globalConfigManager, configManagerErr = config.NewConfigurationManager()
	})
	return globalConfigManager, configManagerErr
}

// loadConfig loads a native v2 cluster configuration.
// The name parameter can be in format "cluster-name" or "organization/cluster-name".
// If organization is specified, it validates that the cluster belongs to that organization.
func loadConfig(ctx context.Context, name string) (v2.Config, error) {
	// Check if name contains organization prefix
	parts := strings.Split(name, "/")
	if len(parts) == 2 {
		// Use the helper that validates organization
		cfg, _, _, err := loadConfigWithIdentifier(ctx, name)
		return cfg, err
	}

	// Simple cluster name - load directly
	manager, err := getConfigManager()
	if err != nil {
		return v2.Config{}, err
	}

	cfg, err := manager.Load(ctx, name)
	if err != nil {
		return v2.Config{}, err
	}

	return *cfg, nil
}

// loadConfigWithIdentifier loads a cluster configuration using an identifier that may include organization.
// The identifier can be in format "cluster-name" or "organization/cluster-name".
// If organization is specified, it uses the ConfigurationManager directly with the full
// identifier so that path resolution targets the correct organization directory. This avoids
// ambiguity when multiple organizations contain a cluster with the same name.
func loadConfigWithIdentifier(ctx context.Context, identifier string) (v2.Config, string, string, error) {
	// Parse organization and cluster name from the identifier
	var clusterName, organization string
	parts := strings.Split(identifier, "/")
	if len(parts) == 2 {
		organization = parts[0]
		clusterName = parts[1]
	} else {
		clusterName = identifier
		// Organization will be determined from config
	}

	// When organization is known, load via manager.Load with the full "org/cluster"
	// identifier so it uses Resolve (org-scoped) instead of ResolveWithFallback.
	// This prevents incorrect matches when multiple orgs share a cluster name.
	var cfg v2.Config
	var err error
	if organization != "" {
		manager, mErr := getConfigManager()
		if mErr != nil {
			return v2.Config{}, "", "", mErr
		}
		loaded, lErr := manager.Load(ctx, identifier)
		if lErr != nil {
			return v2.Config{}, "", "", lErr
		}
		cfg = *loaded
	} else {
		cfg, err = loadConfig(ctx, clusterName)
		if err != nil {
			return v2.Config{}, "", "", err
		}
	}

	// If organization was specified in the identifier, verify it matches the config metadata
	if organization != "" && cfg.OpenCenter.Meta.Organization != organization {
		return v2.Config{}, "", "", fmt.Errorf("cluster %s not found in organization %s (found in %s)",
			clusterName, organization, cfg.OpenCenter.Meta.Organization)
	}

	return cfg, clusterName, cfg.OpenCenter.Meta.Organization, nil
}

// extractClusterName extracts just the cluster name from an identifier that may include organization.
// For "organization/cluster-name" it returns "cluster-name".
// For "cluster-name" it returns "cluster-name".
func extractClusterName(identifier string) string {
	parts := strings.Split(identifier, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return identifier
}

// saveConfig saves a native v2 cluster configuration using the ConfigurationManager.
func saveConfig(ctx context.Context, cfg v2.Config) error {
	manager, err := getConfigManager()
	if err != nil {
		return err
	}

	return manager.Save(ctx, &cfg)
}

func loadNativeV2ConfigWithIdentifier(ctx context.Context, identifier string) (*v2.Config, string, string, *paths.ClusterPaths, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	clusterName := identifier
	organization := ""
	parts := strings.Split(identifier, "/")
	if len(parts) == 2 {
		organization = parts[0]
		clusterName = parts[1]
	}

	pathResolver, err := di.ProvidePathResolver(config.ResolveClustersDir())
	if err != nil {
		return nil, "", "", nil, err
	}

	var clusterPaths *paths.ClusterPaths
	if organization != "" {
		clusterPaths, err = pathResolver.Resolve(ctx, clusterName, organization)
	} else {
		clusterPaths, err = pathResolver.ResolveWithFallback(ctx, clusterName)
	}
	if err != nil {
		return nil, "", "", nil, err
	}

	loader := v2.NewConfigLoader(defaults.NewRegistry())
	cfg, err := loader.LoadFromFile(clusterPaths.ConfigPath)
	if err != nil {
		return nil, "", "", nil, err
	}

	resolvedOrganization := cfg.OpenCenter.Meta.Organization
	if resolvedOrganization == "" {
		resolvedOrganization = filepath.Base(clusterPaths.OrganizationDir)
	}

	if organization != "" && resolvedOrganization != "" && organization != resolvedOrganization {
		return nil, "", "", nil, fmt.Errorf("cluster %s not found in organization %s (found in %s)",
			clusterName, organization, resolvedOrganization)
	}

	return cfg, clusterName, resolvedOrganization, clusterPaths, nil
}

func saveNativeV2Config(ctx context.Context, cfg *v2.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	clusterName := strings.TrimSpace(cfg.OpenCenter.Cluster.ClusterName)
	if clusterName == "" {
		clusterName = strings.TrimSpace(cfg.OpenCenter.Meta.Name)
	}
	if clusterName == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	organization := strings.TrimSpace(cfg.OpenCenter.Meta.Organization)
	if organization == "" {
		return fmt.Errorf("organization cannot be empty")
	}

	pathResolver, err := di.ProvidePathResolver(config.ResolveClustersDir())
	if err != nil {
		return err
	}

	clusterPaths, err := pathResolver.Resolve(ctx, clusterName, organization)
	if err != nil {
		return err
	}

	configPath := clusterPaths.ConfigPath
	if data, err := os.ReadFile(configPath); err == nil {
		if err := os.WriteFile(configPath+".backup", data, 0o600); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	loader := v2.NewConfigLoader(defaults.NewRegistry())
	return loader.SaveToFile(cfg, configPath)
}

func validateNativeV2Config(cfg *v2.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal v2 config for validation: %w", err)
	}

	loader := v2.NewConfigLoader(defaults.NewRegistry())
	if _, err := loader.LoadFromBytes(data); err != nil {
		return err
	}

	return nil
}

// listClusters lists all cluster configurations using the ConfigurationManager.
func listClusters(ctx context.Context) ([]string, error) {
	manager, err := getConfigManager()
	if err != nil {
		return nil, err
	}

	names, err := manager.List(ctx)
	if err != nil {
		return nil, err
	}

	return normalizeClusterDisplayNames(names), nil
}

// getActiveCluster returns the active cluster name using ConfigurationManager.
func getActiveCluster() (string, error) {
	manager, err := getConfigManager()
	if err != nil {
		return "", err
	}

	return manager.GetActive()
}

// setActiveCluster sets the active cluster name using ConfigurationManager.
func setActiveCluster(name string) error {
	manager, err := getConfigManager()
	if err != nil {
		return err
	}

	return manager.SetActive(name)
}

// getConfigPath returns the configuration file path for a cluster.
func getConfigPath(ctx context.Context, name, organization string) (string, error) {
	manager, err := getConfigManager()
	if err != nil {
		return "", err
	}

	// Load the config to get the organization if not provided
	if organization == "" {
		cfg, err := manager.Load(ctx, name)
		if err != nil {
			return "", err
		}
		organization = cfg.OpenCenter.Meta.Organization
	}

	// Get the path resolver
	pathResolver, err := di.ProvidePathResolver(config.ResolveClustersDir())
	if err != nil {
		return "", err
	}

	clusterPaths, err := pathResolver.Resolve(ctx, name, organization)
	if err != nil {
		return "", err
	}

	return clusterPaths.ConfigPath, nil
}

func normalizeClusterDisplayNames(names []string) []string {
	if len(names) == 0 {
		return names
	}

	normalized := make([]string, 0, len(names))
	for _, name := range names {
		normalized = append(normalized, normalizeClusterDisplayName(name))
	}

	return normalized
}

func normalizeClusterDisplayName(name string) string {
	const defaultOrganizationPrefix = "opencenter/"

	if strings.HasPrefix(name, defaultOrganizationPrefix) {
		return strings.TrimPrefix(name, defaultOrganizationPrefix)
	}

	return name
}

// loadCanonicalConfig loads a cluster configuration and enforces the canonical schema.
//
// Parameters:
//   - clusterName: The cluster name to load
//
// Returns:
//   - v2.Config: The loaded configuration
//   - error: An error if the config cannot be loaded or does not use schema_version "2.0"
func loadCanonicalConfig(clusterName string) (v2.Config, error) {
	ctx := context.Background()
	cfg, err := loadConfig(ctx, clusterName)
	if err != nil {
		return cfg, err
	}

	// Check schema version - only v2 is supported
	if cfg.SchemaVersion != "2.0" {
		return cfg, fmt.Errorf("invalid schema version for cluster %s: expected 2.0, got %q", clusterName, cfg.SchemaVersion)
	}

	return cfg, nil
}
