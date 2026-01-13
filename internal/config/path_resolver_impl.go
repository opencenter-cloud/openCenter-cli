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
)

// PathResolverImpl implements the PathResolverInterface with organization and legacy support.
type PathResolverImpl struct {
	configManager *ConfigManager
	pathCache     map[string]string
	mu            sync.RWMutex
}

// NewPathResolverImpl creates a new path resolver implementation.
func NewPathResolverImpl(configManager *ConfigManager) *PathResolverImpl {
	return &PathResolverImpl{
		configManager: configManager,
		pathCache:     make(map[string]string),
	}
}

// ResolveClusterPaths resolves all paths for a cluster with organization support.
func (pr *PathResolverImpl) ResolveClusterPaths(ctx context.Context, clusterName, organization string) (*OrganizationClusterPaths, error) {
	// Allow empty cluster name for organization-only path resolution
	if clusterName != "" {
		if err := ValidateClusterName(clusterName); err != nil {
			return nil, fmt.Errorf("invalid cluster name: %w", err)
		}
	}

	if organization == "" {
		// Try to determine organization from existing cluster
		if detectedOrg, err := pr.GetClusterOrganization(ctx, clusterName); err == nil && detectedOrg != "" {
			organization = detectedOrg
		} else {
			organization = "opencenter" // Default organization
		}
	}

	// Validate organization name
	if err := ValidateClusterName(organization); err != nil {
		return nil, fmt.Errorf("invalid organization name '%s': %w", organization, err)
	}

	// Get base clusters directory from configuration
	clustersDir := ""
	if pr.configManager != nil {
		clustersDir = pr.configManager.GetConfig().Paths.ClustersDir
	}
	if clustersDir == "" {
		// Fallback to default if not configured
		configDir, err := ResolveConfigDir()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve config directory: %w", err)
		}
		clustersDir = filepath.Join(configDir, "clusters")
	}

	// Expand environment variables and tilde
	clustersDir = ExpandPath(clustersDir)

	// Build organization-based paths
	organizationDir := filepath.Join(clustersDir, organization)
	gitOpsDir := organizationDir

	var clusterDir, applicationsDir string
	if clusterName != "" {
		clusterDir = filepath.Join(organizationDir, "infrastructure", "clusters", clusterName)
		applicationsDir = filepath.Join(organizationDir, "applications", "overlays", clusterName)
	} else {
		clusterDir = filepath.Join(organizationDir, "infrastructure", "clusters")
		applicationsDir = filepath.Join(organizationDir, "applications", "overlays")
	}

	secretsDir := filepath.Join(organizationDir, "secrets")

	var sopsKeyPath, kubeconfigPath, inventoryPath, venvPath, binPath string
	if clusterName != "" {
		sopsKeyPath = filepath.Join(secretsDir, "age", "keys", clusterName+"-key.txt")
		kubeconfigPath = filepath.Join(clusterDir, "kubeconfig.yaml")
		inventoryPath = filepath.Join(clusterDir, "inventory")
		venvPath = filepath.Join(clusterDir, "venv")
		binPath = filepath.Join(clusterDir, ".bin")
	}

	return &OrganizationClusterPaths{
		OrganizationDir: organizationDir,
		GitOpsDir:       gitOpsDir,
		ClusterDir:      clusterDir,
		ApplicationsDir: applicationsDir,
		SecretsDir:      secretsDir,
		SOPSKeyPath:     sopsKeyPath,
		SOPSConfigPath:  filepath.Join(organizationDir, ".sops.yaml"),
		KubeconfigPath:  kubeconfigPath,
		InventoryPath:   inventoryPath,
		VenvPath:        venvPath,
		BinPath:         binPath,
	}, nil
}

// CreateClusterDirectories creates all necessary directories for a cluster.
func (pr *PathResolverImpl) CreateClusterDirectories(ctx context.Context, clusterName, organization string) error {
	if err := ValidateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	if organization == "" {
		organization = "opencenter"
	}

	// Validate organization name
	if err := ValidateClusterName(organization); err != nil {
		return fmt.Errorf("invalid organization name '%s': %w", organization, err)
	}

	paths, err := pr.ResolveClusterPaths(ctx, clusterName, organization)
	if err != nil {
		return fmt.Errorf("failed to resolve cluster paths: %w", err)
	}

	// Create all cluster-specific directories with proper error handling
	dirs := []string{
		paths.ClusterDir,
		paths.ApplicationsDir,
		paths.InventoryPath,
		paths.BinPath,
		filepath.Dir(paths.SOPSKeyPath), // age/keys directory
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create cluster directory %s: %w", dir, err)
		}

		// Verify the directory was created and is accessible
		if stat, err := os.Stat(dir); err != nil {
			return fmt.Errorf("failed to verify cluster directory %s: %w", dir, err)
		} else if !stat.IsDir() {
			return fmt.Errorf("path %s exists but is not a directory", dir)
		}

		// Check directory permissions
		if err := pr.validateDirectoryPermissions(dir); err != nil {
			return fmt.Errorf("directory %s has insufficient permissions: %w", dir, err)
		}
	}

	return nil
}

// CreateOrganizationStructure creates the organization directory structure.
func (pr *PathResolverImpl) CreateOrganizationStructure(ctx context.Context, organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	// Validate organization name before creating directories
	if err := ValidateClusterName(organization); err != nil {
		return fmt.Errorf("invalid organization name '%s': %w", organization, err)
	}

	paths, err := pr.ResolveClusterPaths(ctx, "", organization)
	if err != nil {
		return fmt.Errorf("failed to resolve organization paths: %w", err)
	}

	// Create organization GitOps structure with proper error handling
	dirs := []string{
		paths.OrganizationDir,
		filepath.Join(paths.GitOpsDir, "applications", "overlays"),
		filepath.Join(paths.GitOpsDir, "infrastructure", "clusters"),
		filepath.Join(paths.SecretsDir, "age", "keys"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create organization directory %s: %w", dir, err)
		}

		// Verify the directory was created and is accessible
		if stat, err := os.Stat(dir); err != nil {
			return fmt.Errorf("failed to verify organization directory %s: %w", dir, err)
		} else if !stat.IsDir() {
			return fmt.Errorf("path %s exists but is not a directory", dir)
		}
	}

	return nil
}

// ValidatePath validates that a path is safe and accessible.
func (pr *PathResolverImpl) ValidatePath(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Expand the path first
	expandedPath := ExpandPath(path)

	// Check for path traversal attempts
	if strings.Contains(expandedPath, "..") {
		return fmt.Errorf("path contains directory traversal elements: %s", path)
	}

	// Check if the path is absolute after expansion
	if !filepath.IsAbs(expandedPath) {
		return fmt.Errorf("path must be absolute after expansion: %s", expandedPath)
	}

	return nil
}

// IsLegacyCluster checks if a cluster uses legacy structure.
func (pr *PathResolverImpl) IsLegacyCluster(ctx context.Context, clusterName string) (bool, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return false, fmt.Errorf("invalid cluster name: %w", err)
	}

	legacyPath, err := pr.getLegacyClusterPath(clusterName)
	if err != nil {
		return false, err
	}

	// Check if legacy config file exists
	legacyConfigPath := filepath.Join(legacyPath, "."+clusterName+"-config.yaml")
	if _, err := os.Stat(legacyConfigPath); err == nil {
		// Also check that it's not in an organization structure
		// (i.e., the parent directory is "clusters", not an organization)
		parentDir := filepath.Base(filepath.Dir(legacyPath))
		return parentDir == "clusters", nil
	}

	return false, nil
}

// GetClusterOrganization determines the organization for a cluster.
func (pr *PathResolverImpl) GetClusterOrganization(ctx context.Context, clusterName string) (string, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	// Check cache first
	pr.mu.RLock()
	cacheKey := "org:" + clusterName
	if cachedOrg, exists := pr.pathCache[cacheKey]; exists {
		pr.mu.RUnlock()
		return cachedOrg, nil
	}
	pr.mu.RUnlock()

	clustersDir := ""
	if pr.configManager != nil {
		clustersDir = pr.configManager.GetConfig().Paths.ClustersDir
	}
	if clustersDir == "" {
		configDir, err := ResolveConfigDir()
		if err != nil {
			return "", err
		}
		clustersDir = filepath.Join(configDir, "clusters")
	}

	clustersDir = ExpandPath(clustersDir)

	// Check if clusters directory exists
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		return "", nil
	}

	// Look for the cluster in organization directories
	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			orgName := entry.Name()

			// Skip if this looks like a legacy cluster directory
			legacyConfigPath := filepath.Join(clustersDir, orgName, "."+orgName+"-config.yaml")
			if _, err := os.Stat(legacyConfigPath); err == nil {
				continue // This is a legacy cluster, not an organization
			}

			// Check if cluster exists in this organization
			clusterPath := filepath.Join(clustersDir, orgName, "infrastructure", "clusters", clusterName)
			if _, err := os.Stat(clusterPath); err == nil {
				// Cache the result
				pr.mu.Lock()
				pr.pathCache[cacheKey] = orgName
				pr.mu.Unlock()
				return orgName, nil
			}
		}
	}

	// Cache empty result to avoid repeated filesystem scans
	pr.mu.Lock()
	pr.pathCache[cacheKey] = ""
	pr.mu.Unlock()
	return "", nil
}

// GetLegacyClusterPath returns the legacy cluster path for backward compatibility.
func (pr *PathResolverImpl) GetLegacyClusterPath(ctx context.Context, clusterName string) (string, error) {
	return pr.getLegacyClusterPath(clusterName)
}

// OrganizationAwareConfigPath returns the configuration file path with organization support.
func (pr *PathResolverImpl) OrganizationAwareConfigPath(ctx context.Context, clusterName string) (string, error) {
	// Check cache first
	pr.mu.RLock()
	cacheKey := "config:" + clusterName
	if cachedPath, exists := pr.pathCache[cacheKey]; exists {
		pr.mu.RUnlock()
		// Verify cached path still exists
		if _, err := os.Stat(cachedPath); err == nil {
			return cachedPath, nil
		}
		// Remove invalid cache entry
		pr.mu.Lock()
		delete(pr.pathCache, cacheKey)
		pr.mu.Unlock()
	} else {
		pr.mu.RUnlock()
	}

	// Try organization-aware path first
	organization, err := pr.GetClusterOrganization(ctx, clusterName)
	if err == nil && organization != "" {
		paths, err := pr.ResolveClusterPaths(ctx, clusterName, organization)
		if err == nil {
			configPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
			if _, err := os.Stat(configPath); err == nil {
				// Cache the resolved path
				pr.mu.Lock()
				pr.pathCache[cacheKey] = configPath
				pr.mu.Unlock()
				return configPath, nil
			}
		}
	}

	// Fall back to legacy path
	legacyPath, err := pr.getLegacyClusterPath(clusterName)
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(legacyPath, "."+clusterName+"-config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		// Cache the resolved path
		pr.mu.Lock()
		pr.pathCache[cacheKey] = configPath
		pr.mu.Unlock()
		return configPath, nil
	}

	// Fall back to flat config file
	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	flatConfigPath := filepath.Join(configDir, clusterName+".yaml")

	// Cache the resolved path if it exists
	if _, err := os.Stat(flatConfigPath); err == nil {
		pr.mu.Lock()
		pr.pathCache[cacheKey] = flatConfigPath
		pr.mu.Unlock()
	}

	return flatConfigPath, nil
}

// OrganizationAwareSecretsPath returns the secrets path with organization support.
func (pr *PathResolverImpl) OrganizationAwareSecretsPath(ctx context.Context, clusterName string) (string, error) {
	organization, err := pr.GetClusterOrganization(ctx, clusterName)
	if err != nil || organization == "" {
		// Fall back to legacy path
		return ClusterSecretsPath(clusterName)
	}

	paths, err := pr.ResolveClusterPaths(ctx, clusterName, organization)
	if err != nil {
		return "", err
	}

	return filepath.Join(paths.SecretsDir, "age", "keys"), nil
}

// ClearCache clears the path resolution cache.
func (pr *PathResolverImpl) ClearCache(ctx context.Context) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.pathCache = make(map[string]string)
}

// InvalidateCacheForCluster invalidates cache entries for a specific cluster.
func (pr *PathResolverImpl) InvalidateCacheForCluster(ctx context.Context, clusterName string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	delete(pr.pathCache, "config:"+clusterName)
	delete(pr.pathCache, "org:"+clusterName)
}

// GetCacheStats returns cache statistics for debugging.
func (pr *PathResolverImpl) GetCacheStats(ctx context.Context) map[string]interface{} {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	return map[string]interface{}{
		"total_entries": len(pr.pathCache),
		"cache_keys":    pr.getCacheKeys(),
	}
}

// getLegacyClusterPath returns the legacy cluster path for backward compatibility.
func (pr *PathResolverImpl) getLegacyClusterPath(clusterName string) (string, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	clustersDir := ""
	if pr.configManager != nil {
		clustersDir = pr.configManager.GetConfig().Paths.ClustersDir
	}
	if clustersDir == "" {
		configDir, err := ResolveConfigDir()
		if err != nil {
			return "", err
		}
		clustersDir = filepath.Join(configDir, "clusters")
	}

	clustersDir = ExpandPath(clustersDir)
	return filepath.Join(clustersDir, clusterName), nil
}

// validateDirectoryPermissions validates that a directory has proper read/write permissions.
func (pr *PathResolverImpl) validateDirectoryPermissions(dir string) error {
	// Test write permissions by creating a temporary file
	testFile := filepath.Join(dir, ".openCenter_permission_test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory: %w", err)
	}
	file.Close()

	// Clean up test file
	if err := os.Remove(testFile); err != nil {
		// Log warning but don't fail - the directory is writable
		fmt.Printf("Warning: failed to remove test file %s: %v\n", testFile, err)
	}

	return nil
}

// getCacheKeys returns all cache keys (for debugging).
func (pr *PathResolverImpl) getCacheKeys() []string {
	keys := make([]string, 0, len(pr.pathCache))
	for key := range pr.pathCache {
		keys = append(keys, key)
	}
	return keys
}

// ResolveLegacyPaths resolves paths using the legacy flat structure.
func (pr *PathResolverImpl) ResolveLegacyPaths(ctx context.Context, clusterName string) (*OrganizationClusterPaths, error) {
	legacyPath, err := pr.getLegacyClusterPath(clusterName)
	if err != nil {
		return nil, err
	}

	return &OrganizationClusterPaths{
		OrganizationDir: filepath.Dir(legacyPath), // clusters directory
		GitOpsDir:       legacyPath,
		ClusterDir:      legacyPath,
		ApplicationsDir: legacyPath,
		SecretsDir:      filepath.Join(legacyPath, "secrets"),
		SOPSKeyPath:     filepath.Join(legacyPath, "secrets", "age", "keys", clusterName+"-key.txt"),
		SOPSConfigPath:  filepath.Join(legacyPath, ".sops.yaml"),
		KubeconfigPath:  filepath.Join(legacyPath, "kubeconfig.yaml"),
		InventoryPath:   filepath.Join(legacyPath, "inventory"),
		VenvPath:        filepath.Join(legacyPath, "venv"),
		BinPath:         filepath.Join(legacyPath, ".bin"),
	}, nil
}

// DetectStructureType detects whether a cluster uses organization or legacy structure.
func (pr *PathResolverImpl) DetectStructureType(ctx context.Context, clusterName string) (string, error) {
	isLegacy, err := pr.IsLegacyCluster(ctx, clusterName)
	if err != nil {
		return "", err
	}

	if isLegacy {
		return "legacy", nil
	}

	organization, err := pr.GetClusterOrganization(ctx, clusterName)
	if err != nil {
		return "", err
	}

	if organization != "" {
		return "organization", nil
	}

	return "unknown", nil
}
