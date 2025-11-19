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
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultMigrationManager implements MigrationManager interface
type DefaultMigrationManager struct {
	pathResolver     PathResolver
	directoryManager DirectoryManager
	validator        PathValidator
}

// NewDefaultMigrationManager creates a new default migration manager
func NewDefaultMigrationManager(pathResolver PathResolver) *DefaultMigrationManager {
	return &DefaultMigrationManager{
		pathResolver:     pathResolver,
		directoryManager: NewDefaultDirectoryManager(),
		validator:        NewDefaultPathValidator(),
	}
}

// DetectLegacyStructure detects all clusters using the legacy flat structure
func (m *DefaultMigrationManager) DetectLegacyStructure() ([]string, error) {
	// Get the base clusters directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	clustersDir := filepath.Join(homeDir, ".config", "openCenter", "clusters")

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
			isLegacy, err := m.pathResolver.IsLegacyCluster(clusterName)
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

// MigrateClusterToOrganization migrates a cluster from legacy structure to organization-based structure
func (m *DefaultMigrationManager) MigrateClusterToOrganization(clusterName, organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	// Validate cluster name
	if err := m.validator.ValidateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	// Check if cluster is actually legacy
	isLegacy, err := m.pathResolver.IsLegacyCluster(clusterName)
	if err != nil {
		return fmt.Errorf("failed to check if cluster is legacy: %w", err)
	}
	if !isLegacy {
		return fmt.Errorf("cluster %s is not a legacy cluster", clusterName)
	}

	// Get legacy and new paths
	legacyPath, err := m.pathResolver.GetLegacyClusterPath(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get legacy cluster path: %w", err)
	}

	newPaths := m.pathResolver.ResolveClusterPaths(clusterName, organization)

	// Create organization structure
	if err := m.pathResolver.CreateOrganizationStructure(organization); err != nil {
		return fmt.Errorf("failed to create organization structure: %w", err)
	}

	// Create cluster directories
	if err := m.pathResolver.CreateClusterDirectories(clusterName, organization); err != nil {
		return fmt.Errorf("failed to create cluster directories: %w", err)
	}

	// Migrate files and directories
	if err := m.migrateClusterFiles(legacyPath, newPaths); err != nil {
		return fmt.Errorf("failed to migrate cluster files: %w", err)
	}

	// Update cluster configuration with organization metadata
	if err := m.updateClusterConfigWithOrganization(clusterName, organization, newPaths); err != nil {
		return fmt.Errorf("failed to update cluster configuration: %w", err)
	}

	// Remove the legacy directory if it's empty
	if err := m.directoryManager.RemoveDirectoryIfEmpty(legacyPath); err != nil {
		// Log warning but don't fail migration
		fmt.Printf("Warning: failed to remove legacy directory %s: %v\n", legacyPath, err)
	}

	return nil
}

// ValidatePostMigration validates that the migration was successful
func (m *DefaultMigrationManager) ValidatePostMigration(clusterName, organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	paths := m.pathResolver.ResolveClusterPaths(clusterName, organization)

	// Check that essential directories exist
	requiredDirs := []string{
		paths.OrganizationDir,
		paths.ClusterDir,
		paths.SecretsDir,
	}

	for _, dir := range requiredDirs {
		if err := m.validator.ValidatePathIsDirectory(dir); err != nil {
			return fmt.Errorf("required directory validation failed for %s: %w", dir, err)
		}
	}

	// Check that configuration file exists and contains organization metadata
	configPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
	if err := m.validator.ValidatePathIsFile(configPath); err != nil {
		return fmt.Errorf("cluster configuration file validation failed: %w", err)
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

// BackupCluster creates a backup of a cluster before migration
func (m *DefaultMigrationManager) BackupCluster(clusterName string) (string, error) {
	legacyPath, err := m.pathResolver.GetLegacyClusterPath(clusterName)
	if err != nil {
		return "", fmt.Errorf("failed to get legacy cluster path: %w", err)
	}

	// Create backup directory with timestamp
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	backupDir := legacyPath + ".backup." + timestamp

	// Copy the entire cluster directory to backup location
	if err := m.directoryManager.CopyDirectory(legacyPath, backupDir); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupDir, nil
}

// RestoreCluster restores a cluster from backup (rollback functionality)
func (m *DefaultMigrationManager) RestoreCluster(clusterName, backupPath string) error {
	if err := m.validator.ValidatePathIsDirectory(backupPath); err != nil {
		return fmt.Errorf("backup directory validation failed: %w", err)
	}

	legacyPath, err := m.pathResolver.GetLegacyClusterPath(clusterName)
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
	if err := m.directoryManager.CopyDirectory(backupPath, legacyPath); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

// MigrateAllLegacyClusters migrates all detected legacy clusters to organization structure
func (m *DefaultMigrationManager) MigrateAllLegacyClusters(organization string) ([]string, []error) {
	if organization == "" {
		organization = "opencenter"
	}

	legacyClusters, err := m.DetectLegacyStructure()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to detect legacy clusters: %w", err)}
	}

	var migrated []string
	var errors []error

	for _, clusterName := range legacyClusters {
		if err := m.MigrateClusterToOrganization(clusterName, organization); err != nil {
			errors = append(errors, fmt.Errorf("failed to migrate cluster %s: %w", clusterName, err))
		} else {
			migrated = append(migrated, clusterName)
		}
	}

	return migrated, errors
}

// migrateClusterFiles migrates files from legacy structure to organization structure
func (m *DefaultMigrationManager) migrateClusterFiles(legacyPath string, newPaths ClusterPaths) error {
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
		if err := m.migrateFileOrDir(src, dst); err != nil {
			return fmt.Errorf("failed to migrate %s to %s: %w", src, dst, err)
		}
	}

	return nil
}

// migrateFileOrDir migrates a file or directory from source to destination
func (m *DefaultMigrationManager) migrateFileOrDir(src, dst string) error {
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
	if err := m.directoryManager.EnsureDirectoryExists(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// Check if destination already exists
	if _, err := os.Stat(dst); err == nil {
		// If destination exists and source is a directory, merge contents
		if srcInfo.IsDir() {
			// For directories, merge contents instead of failing
			if err := m.directoryManager.CopyDirectory(src, dst); err != nil {
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
		if err := m.directoryManager.CopyDirectory(src, dst); err != nil {
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

// updateClusterConfigWithOrganization updates the cluster configuration to include organization metadata
func (m *DefaultMigrationManager) updateClusterConfigWithOrganization(clusterName, organization string, paths ClusterPaths) error {
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
