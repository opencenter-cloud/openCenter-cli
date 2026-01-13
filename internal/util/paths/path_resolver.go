/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPathResolver implements PathResolver interface
type DefaultPathResolver struct {
	baseDir   string
	pathCache map[string]string
	expander  PathExpander
	validator PathValidator
}

// NewDefaultPathResolver creates a new default path resolver
func NewDefaultPathResolver(baseDir string) *DefaultPathResolver {
	return &DefaultPathResolver{
		baseDir:   baseDir,
		pathCache: make(map[string]string),
		expander:  NewDefaultPathExpander(),
		validator: NewDefaultPathValidator(),
	}
}

// ResolveClusterPaths resolves all cluster paths for the given cluster name and organization
func (r *DefaultPathResolver) ResolveClusterPaths(clusterName, organization string) ClusterPaths {
	if organization == "" {
		organization = "opencenter"
	}

	// Get base clusters directory
	clustersDir := r.baseDir
	if clustersDir == "" {
		// Fallback to default
		homeDir, _ := os.UserHomeDir()
		clustersDir = filepath.Join(homeDir, ".config", "openCenter", "clusters")
	}

	// Expand environment variables and tilde
	clustersDir = r.expander.ExpandPath(clustersDir)

	// Build organization-based paths
	organizationDir := filepath.Join(clustersDir, organization)
	gitOpsDir := organizationDir
	clusterDir := filepath.Join(organizationDir, "infrastructure", "clusters", clusterName)
	applicationsDir := filepath.Join(organizationDir, "applications", "overlays", clusterName)
	secretsDir := filepath.Join(organizationDir, "secrets")

	return ClusterPaths{
		OrganizationDir: organizationDir,
		GitOpsDir:       gitOpsDir,
		ClusterDir:      clusterDir,
		ApplicationsDir: applicationsDir,
		SecretsDir:      secretsDir,
		SOPSKeyPath:     filepath.Join(secretsDir, "age", "keys", clusterName+"-key.txt"),
		SOPSConfigPath:  filepath.Join(organizationDir, ".sops.yaml"),
		KubeconfigPath:  filepath.Join(clusterDir, "kubeconfig.yaml"),
		InventoryPath:   filepath.Join(clusterDir, "inventory"),
		VenvPath:        filepath.Join(clusterDir, "venv"),
		BinPath:         filepath.Join(clusterDir, ".bin"),
	}
}

// ExpandPath expands environment variables and tilde in a path
func (r *DefaultPathResolver) ExpandPath(path string) string {
	return r.expander.ExpandPath(path)
}

// ValidatePath validates that a path is safe and accessible
func (r *DefaultPathResolver) ValidatePath(path string) error {
	return r.validator.ValidatePath(path)
}

// CreateOrganizationStructure creates the complete organization-based directory structure
func (r *DefaultPathResolver) CreateOrganizationStructure(organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	// Validate organization name
	if err := r.validator.ValidateOrganizationName(organization); err != nil {
		return fmt.Errorf("invalid organization name '%s': %w", organization, err)
	}

	paths := r.ResolveClusterPaths("", organization)

	// Create organization GitOps structure
	dirs := []string{
		paths.OrganizationDir,
		filepath.Join(paths.GitOpsDir, "applications", "overlays"),
		filepath.Join(paths.GitOpsDir, "infrastructure", "clusters"),
		filepath.Join(paths.SecretsDir, "age", "keys"),
	}

	dirManager := NewDefaultDirectoryManager()
	return dirManager.CreateDirectoryStructure(dirs, 0755)
}

// CreateClusterDirectories creates all necessary directories for a cluster within an organization
func (r *DefaultPathResolver) CreateClusterDirectories(clusterName, organization string) error {
	if err := r.validator.ValidateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	if organization == "" {
		organization = "opencenter"
	}

	if err := r.validator.ValidateOrganizationName(organization); err != nil {
		return fmt.Errorf("invalid organization name '%s': %w", organization, err)
	}

	paths := r.ResolveClusterPaths(clusterName, organization)

	// Create all cluster-specific directories
	dirs := []string{
		paths.ClusterDir,
		paths.ApplicationsDir,
		paths.InventoryPath,
		paths.BinPath,
		filepath.Dir(paths.SOPSKeyPath), // age/keys directory
	}

	dirManager := NewDefaultDirectoryManager()
	if err := dirManager.CreateDirectoryStructure(dirs, 0755); err != nil {
		return err
	}

	// Validate directory permissions
	for _, dir := range dirs {
		if err := r.validator.ValidateDirectoryPermissions(dir); err != nil {
			return fmt.Errorf("directory %s has insufficient permissions: %w", dir, err)
		}
	}

	return nil
}

// GetLegacyClusterPath returns the legacy cluster path for backward compatibility
func (r *DefaultPathResolver) GetLegacyClusterPath(clusterName string) (string, error) {
	if err := r.validator.ValidateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	clustersDir := r.baseDir
	if clustersDir == "" {
		homeDir, _ := os.UserHomeDir()
		clustersDir = filepath.Join(homeDir, ".config", "openCenter", "clusters")
	}

	clustersDir = r.expander.ExpandPath(clustersDir)
	return filepath.Join(clustersDir, clusterName), nil
}

// IsLegacyCluster checks if a cluster uses the legacy flat directory structure
func (r *DefaultPathResolver) IsLegacyCluster(clusterName string) (bool, error) {
	legacyPath, err := r.GetLegacyClusterPath(clusterName)
	if err != nil {
		return false, err
	}

	// Check if legacy config file exists
	legacyConfigPath := filepath.Join(legacyPath, "."+clusterName+"-config.yaml")
	if _, err := os.Stat(legacyConfigPath); err == nil {
		// Also check that it's not in an organization structure
		parentDir := filepath.Base(filepath.Dir(legacyPath))
		return parentDir == "clusters", nil
	}

	return false, nil
}

// OrganizationAwareClusterDirectoryPath returns the cluster directory path with organization support
func (r *DefaultPathResolver) OrganizationAwareClusterDirectoryPath(clusterName string) (string, error) {
	if err := r.validator.ValidateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	// Try to determine organization from existing cluster configuration
	organization, err := r.getClusterOrganization(clusterName)
	if err != nil {
		// If we can't determine organization, fall back to legacy path
		return r.GetLegacyClusterPath(clusterName)
	}

	if organization != "" {
		// Use organization-based path
		paths := r.ResolveClusterPaths(clusterName, organization)
		return paths.ClusterDir, nil
	}

	// Fall back to legacy path
	return r.GetLegacyClusterPath(clusterName)
}

// OrganizationAwareConfigPath returns the configuration file path with organization support
func (r *DefaultPathResolver) OrganizationAwareConfigPath(clusterName string) (string, error) {
	// Check cache first
	cacheKey := "config:" + clusterName
	if cachedPath, exists := r.pathCache[cacheKey]; exists {
		// Verify cached path still exists
		if _, err := os.Stat(cachedPath); err == nil {
			return cachedPath, nil
		}
		// Remove invalid cache entry
		delete(r.pathCache, cacheKey)
	}

	clusterDir, err := r.OrganizationAwareClusterDirectoryPath(clusterName)
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")

	// Cache the resolved path if the file exists
	if _, err := os.Stat(configPath); err == nil {
		r.pathCache[cacheKey] = configPath
	}

	return configPath, nil
}

// OrganizationAwareSecretsPath returns the secrets path with organization support
func (r *DefaultPathResolver) OrganizationAwareSecretsPath(clusterName string) (string, error) {
	organization, err := r.getClusterOrganization(clusterName)
	if err != nil || organization == "" {
		// Fall back to legacy path
		legacyPath, err := r.GetLegacyClusterPath(clusterName)
		if err != nil {
			return "", err
		}
		return filepath.Join(legacyPath, "secrets", "age", "keys"), nil
	}

	paths := r.ResolveClusterPaths(clusterName, organization)
	return filepath.Join(paths.SecretsDir, "age", "keys"), nil
}

// ClearCache clears the path resolution cache
func (r *DefaultPathResolver) ClearCache() {
	r.pathCache = make(map[string]string)
}

// InvalidateCacheForCluster invalidates cache entries for a specific cluster
func (r *DefaultPathResolver) InvalidateCacheForCluster(clusterName string) {
	delete(r.pathCache, "config:"+clusterName)
	delete(r.pathCache, "org:"+clusterName)
}

// getClusterOrganization attempts to determine the organization for a cluster
func (r *DefaultPathResolver) getClusterOrganization(clusterName string) (string, error) {
	// Check cache first
	cacheKey := "org:" + clusterName
	if cachedOrg, exists := r.pathCache[cacheKey]; exists {
		return cachedOrg, nil
	}

	clustersDir := r.baseDir
	if clustersDir == "" {
		homeDir, _ := os.UserHomeDir()
		clustersDir = filepath.Join(homeDir, ".config", "openCenter", "clusters")
	}

	clustersDir = r.expander.ExpandPath(clustersDir)

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
				r.pathCache[cacheKey] = orgName
				return orgName, nil
			}
		}
	}

	// Cache empty result to avoid repeated filesystem scans
	r.pathCache[cacheKey] = ""
	return "", nil
}
