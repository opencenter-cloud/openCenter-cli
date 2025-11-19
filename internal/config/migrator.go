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
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigMigrator implements the ConfigMigratorInterface for migrating configurations.
type ConfigMigrator struct {
	pathResolver PathResolverInterface
	loader       ConfigLoaderInterface
	validator    ConfigValidatorInterface
}

// NewConfigMigrator creates a new configuration migrator.
func NewConfigMigrator(
	pathResolver PathResolverInterface,
	loader ConfigLoaderInterface,
	validator ConfigValidatorInterface,
) *ConfigMigrator {
	return &ConfigMigrator{
		pathResolver: pathResolver,
		loader:       loader,
		validator:    validator,
	}
}

// MigrateToOrganization migrates a cluster from flat to organization structure.
func (cm *ConfigMigrator) MigrateToOrganization(ctx context.Context, clusterName, organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	// Validate cluster name
	if err := ValidateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	// Validate organization name
	if err := ValidateClusterName(organization); err != nil {
		return fmt.Errorf("invalid organization name '%s': %w", organization, err)
	}

	// Check if cluster is actually legacy
	isLegacy, err := cm.pathResolver.IsLegacyCluster(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to check if cluster is legacy: %w", err)
	}
	if !isLegacy {
		return fmt.Errorf("cluster %s is not a legacy cluster", clusterName)
	}

	// Get legacy and new paths
	legacyPaths, err := cm.pathResolver.(*PathResolverImpl).ResolveLegacyPaths(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to resolve legacy cluster paths: %w", err)
	}

	newPaths, err := cm.pathResolver.ResolveClusterPaths(ctx, clusterName, organization)
	if err != nil {
		return fmt.Errorf("failed to resolve new cluster paths: %w", err)
	}

	// Create organization structure
	if err := cm.pathResolver.CreateOrganizationStructure(ctx, organization); err != nil {
		return fmt.Errorf("failed to create organization structure: %w", err)
	}

	// Create cluster directories
	if err := cm.pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		return fmt.Errorf("failed to create cluster directories: %w", err)
	}

	// Migrate files and directories
	if err := cm.migrateClusterFiles(legacyPaths, newPaths); err != nil {
		return fmt.Errorf("failed to migrate cluster files: %w", err)
	}

	// Update cluster configuration with organization metadata
	if err := cm.updateClusterConfigWithOrganization(clusterName, organization, newPaths); err != nil {
		return fmt.Errorf("failed to update cluster configuration: %w", err)
	}

	// Remove the legacy directory if it's empty
	if err := cm.removeLegacyDirectoryIfEmpty(legacyPaths.ClusterDir); err != nil {
		// Log warning but don't fail migration
		fmt.Printf("Warning: failed to remove legacy directory %s: %v\n", legacyPaths.ClusterDir, err)
	}

	return nil
}

// DetectLegacyStructure detects clusters using legacy flat structure.
func (cm *ConfigMigrator) DetectLegacyStructure(ctx context.Context) ([]string, error) {
	// Get clusters directory
	configDir, err := ResolveConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config directory: %w", err)
	}
	clustersDir := filepath.Join(configDir, "clusters")

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
			isLegacy, err := cm.pathResolver.IsLegacyCluster(ctx, clusterName)
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

// ValidatePostMigration validates that migration was successful.
func (cm *ConfigMigrator) ValidatePostMigration(ctx context.Context, clusterName, organization string) error {
	if organization == "" {
		organization = "opencenter"
	}

	paths, err := cm.pathResolver.ResolveClusterPaths(ctx, clusterName, organization)
	if err != nil {
		return fmt.Errorf("failed to resolve cluster paths: %w", err)
	}

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

// BackupCluster creates a backup before migration.
func (cm *ConfigMigrator) BackupCluster(ctx context.Context, clusterName string) (string, error) {
	if err := ValidateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	// Get legacy cluster path
	legacyPaths, err := cm.pathResolver.(*PathResolverImpl).ResolveLegacyPaths(ctx, clusterName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve legacy cluster paths: %w", err)
	}

	legacyPath := legacyPaths.ClusterDir

	// Check if the cluster directory exists
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("cluster directory does not exist: %s", legacyPath)
	}

	// Create backup directory with timestamp
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	backupDir := legacyPath + ".backup." + timestamp

	// Copy the entire cluster directory to backup location
	if err := cm.copyDir(legacyPath, backupDir); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupDir, nil
}

// RestoreCluster restores a cluster from backup.
func (cm *ConfigMigrator) RestoreCluster(ctx context.Context, clusterName, backupPath string) error {
	if err := ValidateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup directory %s does not exist", backupPath)
	}

	// Get legacy cluster path
	legacyPaths, err := cm.pathResolver.(*PathResolverImpl).ResolveLegacyPaths(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to resolve legacy cluster paths: %w", err)
	}

	legacyPath := legacyPaths.ClusterDir

	// Remove current cluster directory if it exists
	if _, err := os.Stat(legacyPath); err == nil {
		if err := os.RemoveAll(legacyPath); err != nil {
			return fmt.Errorf("failed to remove current cluster directory: %w", err)
		}
	}

	// Restore from backup
	if err := cm.copyDir(backupPath, legacyPath); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

// MigrateAllLegacyClusters migrates all detected legacy clusters to organization structure.
func (cm *ConfigMigrator) MigrateAllLegacyClusters(ctx context.Context, organization string) ([]string, []error) {
	if organization == "" {
		organization = "opencenter"
	}

	legacyClusters, err := cm.DetectLegacyStructure(ctx)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to detect legacy clusters: %w", err)}
	}

	var migrated []string
	var errors []error

	for _, clusterName := range legacyClusters {
		if err := cm.MigrateToOrganization(ctx, clusterName, organization); err != nil {
			errors = append(errors, fmt.Errorf("failed to migrate cluster %s: %w", clusterName, err))
		} else {
			migrated = append(migrated, clusterName)
		}
	}

	return migrated, errors
}

// migrateClusterFiles migrates files from legacy structure to organization structure.
func (cm *ConfigMigrator) migrateClusterFiles(legacyPaths, newPaths *OrganizationClusterPaths) error {
	clusterName := filepath.Base(legacyPaths.ClusterDir)

	// Define migration mappings
	migrations := map[string]string{
		// Configuration file
		filepath.Join(legacyPaths.ClusterDir, "."+clusterName+"-config.yaml"): filepath.Join(newPaths.ClusterDir, "."+clusterName+"-config.yaml"),
		// Kubeconfig
		legacyPaths.KubeconfigPath: newPaths.KubeconfigPath,
		// Inventory directory
		legacyPaths.InventoryPath: newPaths.InventoryPath,
		// Virtual environment
		legacyPaths.VenvPath: newPaths.VenvPath,
		// Binary directory
		legacyPaths.BinPath: newPaths.BinPath,
		// Terraform files
		filepath.Join(legacyPaths.ClusterDir, "main.tf"):      filepath.Join(newPaths.ClusterDir, "main.tf"),
		filepath.Join(legacyPaths.ClusterDir, "provider.tf"):  filepath.Join(newPaths.ClusterDir, "provider.tf"),
		filepath.Join(legacyPaths.ClusterDir, "variables.tf"): filepath.Join(newPaths.ClusterDir, "variables.tf"),
		filepath.Join(legacyPaths.ClusterDir, "Makefile"):     filepath.Join(newPaths.ClusterDir, "Makefile"),
		// SOPS secrets
		legacyPaths.SecretsDir: newPaths.SecretsDir,
	}

	for src, dst := range migrations {
		if err := cm.migrateFileOrDir(src, dst); err != nil {
			return fmt.Errorf("failed to migrate %s to %s: %w", src, dst, err)
		}
	}

	return nil
}

// migrateFileOrDir migrates a file or directory from source to destination.
func (cm *ConfigMigrator) migrateFileOrDir(src, dst string) error {
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
			if err := cm.copyDir(src, dst); err != nil {
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
		if err := cm.copyDir(src, dst); err != nil {
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
func (cm *ConfigMigrator) copyDir(src, dst string) error {
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
		return cm.copyFile(path, dstPath, info.Mode())
	})
}

// copyFile copies a single file from src to dst with the given mode.
func (cm *ConfigMigrator) copyFile(src, dst string, mode os.FileMode) error {
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
func (cm *ConfigMigrator) updateClusterConfigWithOrganization(clusterName, organization string, paths *OrganizationClusterPaths) error {
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

// removeLegacyDirectoryIfEmpty removes the legacy directory if it's empty or only contains empty directories.
func (cm *ConfigMigrator) removeLegacyDirectoryIfEmpty(legacyPath string) error {
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
			if err := cm.removeLegacyDirectoryIfEmpty(subPath); err != nil {
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

// GetMigrationStatus returns the migration status for a cluster.
func (cm *ConfigMigrator) GetMigrationStatus(ctx context.Context, clusterName string) (string, error) {
	isLegacy, err := cm.pathResolver.IsLegacyCluster(ctx, clusterName)
	if err != nil {
		return "", err
	}

	if isLegacy {
		return "legacy", nil
	}

	organization, err := cm.pathResolver.GetClusterOrganization(ctx, clusterName)
	if err != nil {
		return "", err
	}

	if organization != "" {
		return "migrated", nil
	}

	return "unknown", nil
}

// PreMigrationCheck performs checks before migration to ensure it's safe.
func (cm *ConfigMigrator) PreMigrationCheck(ctx context.Context, clusterName, organization string) error {
	// Check if cluster exists and is legacy
	isLegacy, err := cm.pathResolver.IsLegacyCluster(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to check cluster structure: %w", err)
	}
	if !isLegacy {
		return fmt.Errorf("cluster %s is not a legacy cluster", clusterName)
	}

	// Check if target organization structure would conflict
	newPaths, err := cm.pathResolver.ResolveClusterPaths(ctx, clusterName, organization)
	if err != nil {
		return fmt.Errorf("failed to resolve target paths: %w", err)
	}

	// Check if target cluster directory already exists
	if _, err := os.Stat(newPaths.ClusterDir); err == nil {
		return fmt.Errorf("target cluster directory already exists: %s", newPaths.ClusterDir)
	}

	// Validate cluster configuration before migration
	if cm.loader != nil && cm.validator != nil {
		config, err := cm.loader.LoadFromPath(ctx, clusterName)
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration: %w", err)
		}

		result := cm.validator.Validate(ctx, config)
		if !result.Valid {
			return fmt.Errorf("cluster configuration has validation errors: %v", result.Errors)
		}
	}

	return nil
}
