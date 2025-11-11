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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PathResolver manages dynamic path resolution based on organization structure.
// It provides organization-aware directory structure creation and path management.
type PathResolver struct {
	configManager *ConfigManager
	pathCache     map[string]string // Cache for resolved paths
}

// ClusterPaths contains all organization-aware paths for a cluster.
// This structure supports the new organization-based directory layout.
type ClusterPaths struct {
	OrganizationDir   string // ~/.config/openCenter/clusters/<organization>
	GitOpsDir         string // ~/.config/openCenter/clusters/<organization>
	ClusterDir        string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>
	ApplicationsDir   string // ~/.config/openCenter/clusters/<organization>/applications/overlays/<cluster>
	SecretsDir        string // ~/.config/openCenter/clusters/<organization>/secrets
	SOPSKeyPath       string // ~/.config/openCenter/clusters/<organization>/secrets/age/keys/<cluster>-key.txt
	SOPSConfigPath    string // ~/.config/openCenter/clusters/<organization>/secrets/.sops.yaml
	KubeconfigPath    string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>/kubeconfig.yaml
	InventoryPath     string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>/inventory/
	VenvPath          string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>/venv/
	BinPath           string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>/.bin/
}

// MigrationManager handles migration from legacy flat structure to organization-based structure.
type MigrationManager struct {
	pathResolver  *PathResolver
	configManager *ConfigManager
}

// NewPathResolver creates a new path resolver with the given configuration manager.
func NewPathResolver(configManager *ConfigManager) *PathResolver {
	return &PathResolver{
		configManager: configManager,
		pathCache:     make(map[string]string),
	}
}

// ResolveClusterPaths resolves all cluster paths for the given cluster name and organization.
// If organization is empty, it uses the cluster name as the organization name.
func (pr *PathResolver) ResolveClusterPaths(clusterName, organization string) ClusterPaths {
	if organization == "" {
		organization = clusterName
	}

	// Get base clusters directory from configuration
	clustersDir := pr.configManager.GetConfig().Paths.ClustersDir
	if clustersDir == "" {
		// Fallback to default if not configured
		configDir, _ := ResolveConfigDir()
		clustersDir = filepath.Join(configDir, "clusters")
	}

	// Expand environment variables and tilde
	clustersDir = pr.ExpandPath(clustersDir)

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
		SOPSConfigPath:  filepath.Join(secretsDir, ".sops.yaml"),
		KubeconfigPath:  filepath.Join(clusterDir, "kubeconfig.yaml"),
		InventoryPath:   filepath.Join(clusterDir, "inventory"),
		VenvPath:        filepath.Join(clusterDir, "venv"),
		BinPath:         filepath.Join(clusterDir, ".bin"),
	}
}

// ExpandPath expands environment variables and tilde in a path.
// This is a wrapper around the existing ExpandPath function for consistency.
func (pr *PathResolver) ExpandPath(path string) string {
	return ExpandPath(path)
}

// ValidatePath validates that a path is safe and accessible.
func (pr *PathResolver) ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Expand the path first
	expandedPath := pr.ExpandPath(path)

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

// CreateOrganizationStructure creates the complete organization-based directory structure.
func (pr *PathResolver) CreateOrganizationStructure(organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	// Validate organization name before creating directories
	if err := ValidateClusterName(organization); err != nil {
		return fmt.Errorf("invalid organization name '%s': %w", organization, err)
	}

	paths := pr.ResolveClusterPaths("", organization)

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

// CreateClusterDirectories creates all necessary directories for a cluster within an organization.
func (pr *PathResolver) CreateClusterDirectories(clusterName, organization string) error {
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

	paths := pr.ResolveClusterPaths(clusterName, organization)

	// Create all cluster-specific directories with proper error handling
	dirs := []string{
		paths.ClusterDir,
		paths.ApplicationsDir,
		paths.InventoryPath,
		paths.VenvPath,
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

// GetLegacyClusterPath returns the legacy cluster path for backward compatibility.
// This is used during migration to detect legacy clusters.
func (pr *PathResolver) GetLegacyClusterPath(clusterName string) (string, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	clustersDir := pr.configManager.GetConfig().Paths.ClustersDir
	if clustersDir == "" {
		configDir, err := ResolveConfigDir()
		if err != nil {
			return "", err
		}
		clustersDir = filepath.Join(configDir, "clusters")
	}

	clustersDir = pr.ExpandPath(clustersDir)
	return filepath.Join(clustersDir, clusterName), nil
}

// IsLegacyCluster checks if a cluster uses the legacy flat directory structure.
func (pr *PathResolver) IsLegacyCluster(clusterName string) (bool, error) {
	legacyPath, err := pr.GetLegacyClusterPath(clusterName)
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

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(pathResolver *PathResolver, configManager *ConfigManager) *MigrationManager {
	return &MigrationManager{
		pathResolver:  pathResolver,
		configManager: configManager,
	}
}

// DetectLegacyStructure detects all clusters using the legacy flat structure.
func (mm *MigrationManager) DetectLegacyStructure() ([]string, error) {
	clustersDir := mm.configManager.GetConfig().Paths.ClustersDir
	if clustersDir == "" {
		configDir, err := ResolveConfigDir()
		if err != nil {
			return nil, err
		}
		clustersDir = filepath.Join(configDir, "clusters")
	}

	clustersDir = mm.pathResolver.ExpandPath(clustersDir)

	// Check if clusters directory exists
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read clusters directory: %w", err)
	}

	var legacyClusters []string
	for _, entry := range entries {
		if entry.IsDir() {
			clusterName := entry.Name()
			isLegacy, err := mm.pathResolver.IsLegacyCluster(clusterName)
			if err != nil {
				continue // Skip clusters we can't validate
			}
			if isLegacy {
				legacyClusters = append(legacyClusters, clusterName)
			}
		}
	}

	return legacyClusters, nil
}

// MigrateClusterToOrganization migrates a cluster from legacy structure to organization-based structure.
func (mm *MigrationManager) MigrateClusterToOrganization(clusterName, organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	// Validate cluster name
	if err := ValidateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	// Check if cluster is actually legacy
	isLegacy, err := mm.pathResolver.IsLegacyCluster(clusterName)
	if err != nil {
		return fmt.Errorf("failed to check if cluster is legacy: %w", err)
	}
	if !isLegacy {
		return fmt.Errorf("cluster %s is not a legacy cluster", clusterName)
	}

	// Get legacy and new paths
	legacyPath, err := mm.pathResolver.GetLegacyClusterPath(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get legacy cluster path: %w", err)
	}

	newPaths := mm.pathResolver.ResolveClusterPaths(clusterName, organization)

	// Create organization structure
	if err := mm.pathResolver.CreateOrganizationStructure(organization); err != nil {
		return fmt.Errorf("failed to create organization structure: %w", err)
	}

	// Create cluster directories
	if err := mm.pathResolver.CreateClusterDirectories(clusterName, organization); err != nil {
		return fmt.Errorf("failed to create cluster directories: %w", err)
	}

	// Migrate files and directories
	if err := mm.migrateClusterFiles(legacyPath, newPaths); err != nil {
		return fmt.Errorf("failed to migrate cluster files: %w", err)
	}

	// Update cluster configuration with organization metadata
	if err := mm.updateClusterConfigWithOrganization(clusterName, organization, newPaths); err != nil {
		return fmt.Errorf("failed to update cluster configuration: %w", err)
	}

	// Remove the legacy directory if it's empty
	if err := mm.removeLegacyDirectoryIfEmpty(legacyPath); err != nil {
		// Log warning but don't fail migration
		fmt.Printf("Warning: failed to remove legacy directory %s: %v\n", legacyPath, err)
	}

	return nil
}

// migrateClusterFiles migrates files from legacy structure to organization structure.
func (mm *MigrationManager) migrateClusterFiles(legacyPath string, newPaths ClusterPaths) error {
	// Define migration mappings
	migrations := map[string]string{
		// Configuration file
		filepath.Join(legacyPath, "."+filepath.Base(legacyPath)+"-config.yaml"): filepath.Join(newPaths.ClusterDir, "."+filepath.Base(newPaths.ClusterDir)+"-config.yaml"),
		// Kubeconfig
		filepath.Join(legacyPath, "kubeconfig.yaml"): newPaths.KubeconfigPath,
		// Inventory directory
		filepath.Join(legacyPath, "inventory"): newPaths.InventoryPath,
		// Virtual environment
		filepath.Join(legacyPath, "venv"): newPaths.VenvPath,
		// Binary directory
		filepath.Join(legacyPath, ".bin"): newPaths.BinPath,
		// Terraform files
		filepath.Join(legacyPath, "main.tf"):      filepath.Join(newPaths.ClusterDir, "main.tf"),
		filepath.Join(legacyPath, "provider.tf"):  filepath.Join(newPaths.ClusterDir, "provider.tf"),
		filepath.Join(legacyPath, "variables.tf"): filepath.Join(newPaths.ClusterDir, "variables.tf"),
		filepath.Join(legacyPath, "Makefile"):     filepath.Join(newPaths.ClusterDir, "Makefile"),
		// SOPS secrets
		filepath.Join(legacyPath, "secrets"): newPaths.SecretsDir,
	}

	for src, dst := range migrations {
		if err := mm.migrateFileOrDir(src, dst); err != nil {
			return fmt.Errorf("failed to migrate %s to %s: %w", src, dst, err)
		}
	}

	return nil
}

// migrateFileOrDir migrates a file or directory from source to destination.
func (mm *MigrationManager) migrateFileOrDir(src, dst string) error {
	// Check if source exists
	srcInfo, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil // Source doesn't exist, nothing to migrate
	}
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", src, err)
	}

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// Check if destination already exists
	if _, err := os.Stat(dst); err == nil {
		// If destination exists and source is a directory, merge contents
		if srcInfo.IsDir() {
			// For directories, merge contents instead of failing
			if err := mm.copyDir(src, dst); err != nil {
				return fmt.Errorf("failed to merge directory %s to %s: %w", src, dst, err)
			}
			if err := os.RemoveAll(src); err != nil {
				return fmt.Errorf("failed to remove source directory %s: %w", src, err)
			}
			return nil
		}
		return fmt.Errorf("destination file %s already exists", dst)
	}

	// Move the file or directory
	if srcInfo.IsDir() {
		// For directories, we need to copy recursively then remove source
		if err := mm.copyDir(src, dst); err != nil {
			return fmt.Errorf("failed to copy directory %s to %s: %w", src, dst, err)
		}
		if err := os.RemoveAll(src); err != nil {
			return fmt.Errorf("failed to remove source directory %s: %w", src, err)
		}
	} else {
		// For files, we can use rename
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("failed to move file %s to %s: %w", src, dst, err)
		}
	}

	return nil
}

// copyDir recursively copies a directory from src to dst.
func (mm *MigrationManager) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		return mm.copyFile(path, dstPath, info.Mode())
	})
}

// copyFile copies a single file from src to dst with the given mode.
func (mm *MigrationManager) copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy file contents
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := srcFile.Read(buf)
		if n > 0 {
			if _, writeErr := dstFile.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}

	return nil
}

// updateClusterConfigWithOrganization updates the cluster configuration to include organization metadata.
func (mm *MigrationManager) updateClusterConfigWithOrganization(clusterName, organization string, paths ClusterPaths) error {
	// Load the cluster configuration from the new location
	configPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
	
	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read cluster configuration: %w", err)
	}

	// Parse as generic map to preserve structure
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse cluster configuration: %w", err)
	}

	// Add organization metadata
	if opencenter, ok := config["opencenter"].(map[string]interface{}); ok {
		if meta, ok := opencenter["meta"].(map[string]interface{}); ok {
			meta["organization"] = organization
		} else {
			opencenter["meta"] = map[string]interface{}{
				"organization": organization,
			}
		}
	} else {
		config["opencenter"] = map[string]interface{}{
			"meta": map[string]interface{}{
				"organization": organization,
			},
		}
	}

	// Update GitOps directory to point to organization root
	if opencenter, ok := config["opencenter"].(map[string]interface{}); ok {
		if gitops, ok := opencenter["gitops"].(map[string]interface{}); ok {
			gitops["git_dir"] = paths.GitOpsDir
		}
	}

	// Marshal back to YAML
	updatedData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal updated configuration: %w", err)
	}

	// Write back to file
	if err := os.WriteFile(configPath, updatedData, 0600); err != nil {
		return fmt.Errorf("failed to write updated configuration: %w", err)
	}

	return nil
}

// ValidatePostMigration validates that the migration was successful.
func (mm *MigrationManager) ValidatePostMigration(clusterName, organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	paths := mm.pathResolver.ResolveClusterPaths(clusterName, organization)

	// Check that essential directories exist
	requiredDirs := []string{
		paths.OrganizationDir,
		paths.ClusterDir,
		paths.SecretsDir,
	}

	for _, dir := range requiredDirs {
		if stat, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("required directory %s does not exist after migration", dir)
		} else if err != nil {
			return fmt.Errorf("failed to access directory %s after migration: %w", dir, err)
		} else if !stat.IsDir() {
			return fmt.Errorf("path %s exists but is not a directory after migration", dir)
		}
	}

	// Check that configuration file exists and contains organization metadata
	configPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("cluster configuration file %s does not exist after migration", configPath)
	}

	// Verify organization metadata in configuration
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read migrated configuration: %w", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse migrated configuration: %w", err)
	}

	// Check organization metadata
	if opencenter, ok := config["opencenter"].(map[string]interface{}); ok {
		if meta, ok := opencenter["meta"].(map[string]interface{}); ok {
			if org, ok := meta["organization"].(string); ok && org == organization {
				return nil // Validation successful
			}
		}
	}

	return fmt.Errorf("organization metadata not found or incorrect in migrated configuration")
}

// BackupCluster creates a backup of a cluster before migration.
func (mm *MigrationManager) BackupCluster(clusterName string) (string, error) {
	legacyPath, err := mm.pathResolver.GetLegacyClusterPath(clusterName)
	if err != nil {
		return "", fmt.Errorf("failed to get legacy cluster path: %w", err)
	}

	// Create backup directory with timestamp
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	backupDir := legacyPath + ".backup." + timestamp

	// Copy the entire cluster directory to backup location
	if err := mm.copyDir(legacyPath, backupDir); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupDir, nil
}

// RestoreCluster restores a cluster from backup (rollback functionality).
func (mm *MigrationManager) RestoreCluster(clusterName, backupPath string) error {
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup directory %s does not exist", backupPath)
	}

	legacyPath, err := mm.pathResolver.GetLegacyClusterPath(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get legacy cluster path: %w", err)
	}

	// Remove current cluster directory if it exists
	if _, err := os.Stat(legacyPath); err == nil {
		if err := os.RemoveAll(legacyPath); err != nil {
			return fmt.Errorf("failed to remove current cluster directory: %w", err)
		}
	}

	// Restore from backup
	if err := mm.copyDir(backupPath, legacyPath); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

// MigrateAllLegacyClusters migrates all detected legacy clusters to organization structure.
func (mm *MigrationManager) MigrateAllLegacyClusters(organization string) ([]string, []error) {
	if organization == "" {
		organization = "opencenter"
	}

	legacyClusters, err := mm.DetectLegacyStructure()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to detect legacy clusters: %w", err)}
	}

	var migrated []string
	var errors []error

	for _, clusterName := range legacyClusters {
		if err := mm.MigrateClusterToOrganization(clusterName, organization); err != nil {
			errors = append(errors, fmt.Errorf("failed to migrate cluster %s: %w", clusterName, err))
		} else {
			migrated = append(migrated, clusterName)
		}
	}

	return migrated, errors
}

// removeLegacyDirectoryIfEmpty removes the legacy directory if it's empty or only contains empty directories.
func (mm *MigrationManager) removeLegacyDirectoryIfEmpty(legacyPath string) error {
	// Check if directory exists
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil // Already removed
	}

	// Check if directory is empty or only contains empty directories
	entries, err := os.ReadDir(legacyPath)
	if err != nil {
		return err
	}

	// Remove any empty subdirectories first
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(legacyPath, entry.Name())
			if err := mm.removeLegacyDirectoryIfEmpty(subPath); err != nil {
				return err
			}
		}
	}

	// Check again if directory is now empty
	entries, err = os.ReadDir(legacyPath)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return os.Remove(legacyPath)
	}

	return nil // Directory is not empty, leave it
}

// UpdateExistingPathFunctions updates the existing path functions to support organization structure.
// This provides backward compatibility while enabling organization-aware paths.

// OrganizationAwareClusterDirectoryPath returns the cluster directory path with organization support.
// If the cluster has organization metadata, it uses the organization structure.
// Otherwise, it falls back to the legacy structure for backward compatibility.
func (pr *PathResolver) OrganizationAwareClusterDirectoryPath(clusterName string) (string, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	// Try to determine organization from existing cluster configuration
	organization, err := pr.getClusterOrganization(clusterName)
	if err != nil {
		// If we can't determine organization, fall back to legacy path
		return ClusterDirectoryPath(clusterName)
	}

	if organization != "" {
		// Use organization-based path
		paths := pr.ResolveClusterPaths(clusterName, organization)
		return paths.ClusterDir, nil
	}

	// Fall back to legacy path
	return ClusterDirectoryPath(clusterName)
}

// OrganizationAwareConfigPath returns the configuration file path with organization support.
// It uses caching to improve performance for repeated lookups.
func (pr *PathResolver) OrganizationAwareConfigPath(clusterName string) (string, error) {
	// Check cache first
	cacheKey := "config:" + clusterName
	if cachedPath, exists := pr.pathCache[cacheKey]; exists {
		// Verify cached path still exists
		if _, err := os.Stat(cachedPath); err == nil {
			return cachedPath, nil
		}
		// Remove invalid cache entry
		delete(pr.pathCache, cacheKey)
	}

	clusterDir, err := pr.OrganizationAwareClusterDirectoryPath(clusterName)
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")
	
	// Cache the resolved path if the file exists
	if _, err := os.Stat(configPath); err == nil {
		pr.pathCache[cacheKey] = configPath
	}

	return configPath, nil
}

// OrganizationAwareSecretsPath returns the secrets path with organization support.
func (pr *PathResolver) OrganizationAwareSecretsPath(clusterName string) (string, error) {
	organization, err := pr.getClusterOrganization(clusterName)
	if err != nil || organization == "" {
		// Fall back to legacy path
		return ClusterSecretsPath(clusterName)
	}

	paths := pr.ResolveClusterPaths(clusterName, organization)
	return filepath.Join(paths.SecretsDir, "age", "keys"), nil
}

// validateDirectoryPermissions validates that a directory has proper read/write permissions.
func (pr *PathResolver) validateDirectoryPermissions(dir string) error {
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

// ClearCache clears the path resolution cache.
// This should be called when the filesystem structure changes.
func (pr *PathResolver) ClearCache() {
	pr.pathCache = make(map[string]string)
}

// InvalidateCacheForCluster invalidates cache entries for a specific cluster.
func (pr *PathResolver) InvalidateCacheForCluster(clusterName string) {
	delete(pr.pathCache, "config:"+clusterName)
	delete(pr.pathCache, "org:"+clusterName)
}

// getClusterOrganization attempts to determine the organization for a cluster.
// It first checks if the cluster exists in organization structure, then checks configuration.
func (pr *PathResolver) getClusterOrganization(clusterName string) (string, error) {
	// Check cache first
	cacheKey := "org:" + clusterName
	if cachedOrg, exists := pr.pathCache[cacheKey]; exists {
		return cachedOrg, nil
	}

	clustersDir := pr.configManager.GetConfig().Paths.ClustersDir
	if clustersDir == "" {
		configDir, err := ResolveConfigDir()
		if err != nil {
			return "", err
		}
		clustersDir = filepath.Join(configDir, "clusters")
	}

	clustersDir = pr.ExpandPath(clustersDir)

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
				pr.pathCache[cacheKey] = orgName
				return orgName, nil
			}
		}
	}

	// Cache empty result to avoid repeated filesystem scans
	pr.pathCache[cacheKey] = ""
	return "", nil
}