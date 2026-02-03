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

package gitops

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
)

// IsGitOpsInitialized checks if a GitOps directory has already been initialized
// by looking for marker files that indicate a previous setup.
//
// It checks for the presence of:
//   - README.md: Base GitOps structure file
//   - .git directory: Git repository initialization
//
// Returns true if the directory appears to be initialized, false otherwise.
func IsGitOpsInitialized(gitDir string) (bool, error) {
	if gitDir == "" {
		return false, fmt.Errorf("git_dir is empty")
	}

	// Check if directory exists
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return false, nil
	}

	// Check for marker files that indicate initialization
	markerFiles := []string{
		"README.md",
	}

	for _, marker := range markerFiles {
		markerPath := filepath.Join(gitDir, marker)
		if _, err := os.Stat(markerPath); err == nil {
			// At least one marker file exists, consider it initialized
			return true, nil
		}
	}

	// Also check for .git directory as a strong indicator
	gitPath := filepath.Join(gitDir, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		return true, nil
	}

	return false, nil
}

// copyWorkspaceToTarget copies all files from workspace to target directory.
// This is used to finalize atomic operations by moving files from the workspace
// to the final destination.
func copyWorkspaceToTarget(workspaceDir, targetDir string) error {
	return filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Get relative path
		relPath, err := filepath.Rel(workspaceDir, path)
		if err != nil {
			return err
		}
		
		// Skip temp directory
		if strings.HasPrefix(relPath, ".tmp") {
			return nil
		}
		
		// Create destination path
		dstPath := filepath.Join(targetDir, relPath)
		
		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
		
		// Read source file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		
		// Write to destination (overwrites existing files)
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// CopyBase copies or renders embedded files from gitops-base-dir into the target directory
// specified by cfg.GitOps().GitDir.
//
// Files ending with .tpl are always rendered with the cluster configuration bound
// under the dot context and the .tpl suffix stripped from the destination path.
// When render is true, .tmpl files are rendered using the same rules. When render
// is false, .tmpl files are copied verbatim (extension preserved) to allow manual
// customization workflows.
//
// Non-template files are copied as-is. The directory structure under gitops-base-dir/
// is preserved. The target directory is created if it does not exist.
//
// Inputs:
//   - cfg: The cluster configuration.
//   - render: If true, both .tpl and .tmpl files render; if false, only .tpl
//     files render while .tmpl files are copied as-is for manual editing.
//
// Outputs:
//   - error: An error if one occurred during the copy or render operation.
func CopyBase(cfg config.Config, render bool) error {
	target := cfg.GitOps().GitDir
	if target == "" {
		return fmt.Errorf("opencenter.gitops.git_dir must be set")
	}
	
	// Use atomic version with temporary workspace
	tempDir := os.TempDir()
	manager := NewWorkspaceManager(tempDir)
	workspace, err := manager.CreateWorkspace(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}
	defer manager.CleanupWorkspace(context.Background(), workspace)
	
	// Copy to workspace atomically
	if err := CopyBaseAtomic(cfg, render, workspace); err != nil {
		return err
	}
	
	// Copy from workspace to target
	return copyWorkspaceToTarget(workspace.RootDir, target)
}

// renderTemplateAtomic reads the embedded template file at path, executes
// it using the provided configuration, and writes the result atomically to dst.
// It handles special cases where template files contain non-Go template syntax.
func renderTemplateAtomic(path, dst string, cfg config.Config, workspace *GitOpsWorkspace) error {
	data, err := Files.ReadFile(path)
	if err != nil {
		return err
	}

	// Handle special cases for files that contain conflicting template syntax
	content := string(data)
	filename := filepath.Base(path)

	// For Makefile.tpl, escape Helm template syntax to prevent Go template parsing conflicts
	if filename == "Makefile.tpl" {
		// Replace Helm template syntax with escaped version for Go template processing
		content = strings.ReplaceAll(content, `--template="{{.Version}}"`, `--template="{{"{{"}}.Version{{"}}"}}"`)
	}

	t, err := template.New(filename).Funcs(sprig.TxtFuncMap()).Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", path, err)
	}

	// Execute template to a buffer first
	var buf strings.Builder
	if err := t.Execute(&buf, cfg); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", path, err)
	}

	// Get relative path from workspace root
	relPath, err := filepath.Rel(workspace.RootDir, dst)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Write atomically using workspace writer
	writer := NewAtomicWriter(workspace)
	return writer.WriteFileString(relPath, buf.String(), 0o644)
}

// copyFileAtomic copies an embedded file from src to dst atomically within a workspace.
// The file is written atomically to prevent partial writes.
func copyFileAtomic(src, dst string, workspace *GitOpsWorkspace) error {
	data, err := Files.ReadFile(src)
	if err != nil {
		return err
	}

	// Get relative path from workspace root
	relPath, err := filepath.Rel(workspace.RootDir, dst)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Use atomic writer
	writer := NewAtomicWriter(workspace)
	return writer.WriteFile(relPath, data, 0o644)
}

// shouldSkipFile determines if a file should be skipped based on service configuration.
// It checks if the file belongs to a disabled service or managed service.
func shouldSkipFile(relPath string, cfg config.Config) bool {
	pathParts := strings.Split(relPath, string(filepath.Separator))

	// Skip files in disabled services directories
	if len(pathParts) >= 2 && pathParts[0] == "services" {
		serviceName := pathParts[1]

		// Special handling for sources directory
		if serviceName == "sources" && len(pathParts) >= 3 {
			// Extract service name from source filename (e.g., opencenter-cert-manager.yaml -> cert-manager)
			filename := pathParts[len(pathParts)-1]
			if strings.HasPrefix(filename, "opencenter-") {
				extractedServiceName := strings.TrimPrefix(filename, "opencenter-")
				extractedServiceName = strings.TrimSuffix(extractedServiceName, ".yaml")
				extractedServiceName = strings.TrimSuffix(extractedServiceName, ".yaml.tpl")

				// Check if this service exists and is enabled
				if service, exists := cfg.OpenCenter.Services[extractedServiceName]; exists {
					if IsServiceDisabled(service) {
						return true
					}
				} else {
					// Service not in config - skip it
					return true
				}
			}
		} else {
			// Regular service directory check
			if service, exists := cfg.OpenCenter.Services[serviceName]; exists {
				if IsServiceDisabled(service) {
					return true
				}
			} else {
				// Service not in config - skip it
				return true
			}
		}
	}

	// Skip files in disabled managed services directories
	if len(pathParts) >= 2 && pathParts[0] == "managed-services" {
		serviceName := pathParts[1]

		// Special handling for sources directory
		if serviceName == "sources" && len(pathParts) >= 3 {
			// Extract service name from source filename (e.g., opencenter-alert-proxy.yaml -> alert-proxy)
			filename := pathParts[len(pathParts)-1]
			if strings.HasPrefix(filename, "opencenter-") {
				extractedServiceName := strings.TrimPrefix(filename, "opencenter-")
				extractedServiceName = strings.TrimSuffix(extractedServiceName, ".yaml")
				extractedServiceName = strings.TrimSuffix(extractedServiceName, ".yaml.tpl")

				// Check if this managed service exists and is enabled
				if service, exists := cfg.OpenCenter.ManagedService[extractedServiceName]; exists {
					if IsServiceDisabled(service) {
						return true
					}
				} else {
					// Managed service not in config - skip it
					return true
				}
			}
		} else {
			// Regular managed service directory check
			if service, exists := cfg.OpenCenter.ManagedService[serviceName]; exists {
				if IsServiceDisabled(service) {
					return true
				}
			} else {
				// Managed service not in config - skip it
				return true
			}
		}
	}

	return false
}

// RenderClusterApps renders cluster-apps-base template to applications/overlays/<cluster-name>/
// This function processes all files in the cluster-apps-base template directory,
// renders .tmpl files with the cluster configuration, and copies others as-is.
// It skips directories for disabled services and managed services.
func RenderClusterApps(cfg config.Config) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	// Try to use PathResolver to get the applications directory
	var target string
	baseDir := filepath.Join(os.Getenv("HOME"), ".config", "opencenter", "clusters")
	resolver := paths.NewPathResolver(baseDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err == nil {
		// Successfully resolved paths
		target = clusterPaths.ApplicationsDir
	} else {
		// Fallback to GitOps.GitDir for test environments or when cluster doesn't exist yet
		target = filepath.Join(cfg.GitOps().GitDir, "applications", "overlays", clusterName)
	}

	// Create target directory
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	// Clean up disabled services before rendering
	if err := cleanupDisabledServices(target, cfg); err != nil {
		return fmt.Errorf("failed to cleanup disabled services: %w", err)
	}

	// Create a temporary workspace for atomic operations
	tempDir := os.TempDir()
	manager := NewWorkspaceManager(tempDir)
	workspace, err := manager.CreateWorkspace(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}
	defer manager.CleanupWorkspace(context.Background(), workspace)
	
	// Use atomic version
	if err := RenderClusterAppsAtomic(cfg, workspace); err != nil {
		return err
	}
	
	// Copy files from workspace to target
	return copyWorkspaceToTarget(workspace.RootDir, target)
}

// cleanupDisabledServices removes service directories that are not enabled in the configuration.
// This ensures that when services are disabled or removed from config, their directories are cleaned up.
func cleanupDisabledServices(targetDir string, cfg config.Config) error {
	// Clean up regular services
	servicesDir := filepath.Join(targetDir, "services")
	if _, err := os.Stat(servicesDir); err == nil {
		entries, err := os.ReadDir(servicesDir)
		if err != nil {
			return fmt.Errorf("failed to read services directory: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			serviceName := entry.Name()
			// Skip special directories
			if serviceName == "sources" || serviceName == "." || serviceName == ".." {
				continue
			}

			// Check if service should be removed
			if service, exists := cfg.OpenCenter.Services[serviceName]; !exists || IsServiceDisabled(service) {
				serviceDir := filepath.Join(servicesDir, serviceName)
				if err := os.RemoveAll(serviceDir); err != nil {
					return fmt.Errorf("failed to remove disabled service directory %s: %w", serviceName, err)
				}
			}
		}
	}

	// Clean up managed services
	managedServicesDir := filepath.Join(targetDir, "managed-services")
	if _, err := os.Stat(managedServicesDir); err == nil {
		entries, err := os.ReadDir(managedServicesDir)
		if err != nil {
			return fmt.Errorf("failed to read managed-services directory: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			serviceName := entry.Name()
			// Skip special directories
			if serviceName == "sources" || serviceName == "." || serviceName == ".." {
				continue
			}

			// Check if managed service should be removed
			if service, exists := cfg.OpenCenter.ManagedService[serviceName]; !exists || IsServiceDisabled(service) {
				serviceDir := filepath.Join(managedServicesDir, serviceName)
				if err := os.RemoveAll(serviceDir); err != nil {
					return fmt.Errorf("failed to remove disabled managed service directory %s: %w", serviceName, err)
				}
			}
		}
	}

	return nil
}

// RenderInfrastructureCluster renders infrastructure-cluster-template to infrastructure/clusters/<cluster-name>/
// This function processes all files in the infrastructure-cluster-template directory,
// renders .tmpl and .tpl files with the cluster configuration, and copies others as-is.
// It selects the appropriate main.tf template based on the infrastructure provider type.
func RenderInfrastructureCluster(cfg config.Config) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	// Try to use PathResolver to get the cluster directory
	var target string
	baseDir := filepath.Join(os.Getenv("HOME"), ".config", "opencenter", "clusters")
	resolver := paths.NewPathResolver(baseDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err == nil {
		// Successfully resolved paths
		target = clusterPaths.ClusterDir
	} else {
		// Fallback to GitOps.GitDir for test environments or when cluster doesn't exist yet
		target = filepath.Join(cfg.GitOps().GitDir, "infrastructure", "clusters", clusterName)
	}

	// Create target directory
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	// Create a temporary workspace for atomic operations
	tempDir := os.TempDir()
	manager := NewWorkspaceManager(tempDir)
	workspace, err := manager.CreateWorkspace(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}
	defer manager.CleanupWorkspace(context.Background(), workspace)
	
	// Use atomic version
	if err := RenderInfrastructureClusterAtomic(cfg, workspace); err != nil {
		return err
	}
	
	// Copy files from workspace to target
	return copyWorkspaceToTarget(workspace.RootDir, target)
}

// RenderSingleService renders only the specified service to the cluster apps directory.
// This is useful for updating a single service without re-rendering the entire cluster.
func RenderSingleService(cfg config.Config, serviceName string, isManaged bool) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	// Try to use PathResolver to get the applications directory
	var target string
	baseDir := filepath.Join(os.Getenv("HOME"), ".config", "opencenter", "clusters")
	resolver := paths.NewPathResolver(baseDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err == nil {
		// Successfully resolved paths
		target = clusterPaths.ApplicationsDir
	} else {
		// Fallback to GitOps.GitDir for test environments or when cluster doesn't exist yet
		target = filepath.Join(cfg.GitOps().GitDir, "applications", "overlays", clusterName)
	}

	// Create target directory
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	// Determine the service directory prefix
	servicePrefix := "services"
	if isManaged {
		servicePrefix = "managed-services"
	}

	// Create a temporary workspace for atomic operations
	tempDir := os.TempDir()
	manager := NewWorkspaceManager(tempDir)
	workspace, err := manager.CreateWorkspace(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}
	defer manager.CleanupWorkspace(context.Background(), workspace)

	// Walk embedded cluster-apps-base files and only process the specified service
	err = fs.WalkDir(Files, "templates/cluster-apps-base", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel("templates/cluster-apps-base", path)
		if err != nil {
			return err
		}

		pathParts := strings.Split(rel, string(filepath.Separator))

		// Only process files for the specified service
		shouldProcess := false

		// Check if this file belongs to the service directory
		if len(pathParts) >= 2 && pathParts[0] == servicePrefix && pathParts[1] == serviceName {
			shouldProcess = true
		}

		// Check if this is a source file for the service
		if len(pathParts) >= 3 && pathParts[0] == servicePrefix && pathParts[1] == "sources" {
			filename := pathParts[len(pathParts)-1]
			expectedFilename := fmt.Sprintf("opencenter-%s.yaml", serviceName)
			expectedFilenameTPL := fmt.Sprintf("opencenter-%s.yaml.tpl", serviceName)
			if filename == expectedFilename || filename == expectedFilenameTPL {
				shouldProcess = true
			}
		}

		// Check if this is a fluxcd file for the service
		if len(pathParts) >= 3 && pathParts[0] == servicePrefix && pathParts[1] == "fluxcd" {
			filename := pathParts[len(pathParts)-1]
			expectedFilename := fmt.Sprintf("%s.yaml", serviceName)
			expectedFilenameTPL := fmt.Sprintf("%s.yaml.tpl", serviceName)
			if filename == expectedFilename || filename == expectedFilenameTPL {
				shouldProcess = true
			}
		}

		if !shouldProcess {
			return nil
		}

		// Replace cluster-name and cluster_name placeholders in filename
		relWithClusterName := strings.ReplaceAll(rel, "cluster-name", clusterName)
		relWithClusterName = strings.ReplaceAll(relWithClusterName, "cluster_name", clusterName)

		dst := filepath.Join(workspace.RootDir, relWithClusterName)

		// If template file, process and strip template extension
		if strings.HasSuffix(d.Name(), ".tmpl") || strings.HasSuffix(d.Name(), ".tpl") {
			if strings.HasSuffix(d.Name(), ".tmpl") {
				dst = strings.TrimSuffix(dst, ".tmpl")
			} else {
				dst = strings.TrimSuffix(dst, ".tpl")
			}
			return renderTemplateAtomic(path, dst, cfg, workspace)
		}

		// Copy file as-is
		return copyFileAtomic(path, dst, workspace)
	})
	
	if err != nil {
		return err
	}
	
	// Copy files from workspace to target
	return copyWorkspaceToTarget(workspace.RootDir, target)
}

// IsServiceDisabled checks if a service configuration has Enabled set to false.
// It uses reflection to access the Enabled field since the service config is an interface{}.
func IsServiceDisabled(serviceCfg any) bool {
	val := reflect.ValueOf(serviceCfg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		enabledField := val.FieldByName("Enabled")
		if enabledField.IsValid() && enabledField.Kind() == reflect.Bool {
			return !enabledField.Bool()
		}
	}
	return false
}

// CopyBaseAtomic copies or renders embedded files from gitops-base-dir into the workspace
// using atomic file operations to prevent partial writes.
//
// This is the workspace-aware version of CopyBase that ensures all file operations
// are atomic and can be rolled back if needed.
//
// Files ending with .tpl are always rendered with the cluster configuration bound
// under the dot context and the .tpl suffix stripped from the destination path.
// When render is true, .tmpl files are rendered using the same rules. When render
// is false, .tmpl files are copied verbatim (extension preserved) to allow manual
// customization workflows.
//
// Non-template files are copied as-is. The directory structure under gitops-base-dir/
// is preserved.
//
// Inputs:
//   - cfg: The cluster configuration.
//   - render: If true, both .tpl and .tmpl files render; if false, only .tpl
//     files render while .tmpl files are copied as-is for manual editing.
//   - workspace: The GitOps workspace for atomic operations.
//
// Outputs:
//   - error: An error if one occurred during the copy or render operation.
func CopyBaseAtomic(cfg config.Config, render bool, workspace *GitOpsWorkspace) error {
	target := workspace.RootDir

	// Walk embedded files
	err := fs.WalkDir(Files, "gitops-base-dir", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("gitops-base-dir", path)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		name := d.Name()
		isTpl := strings.HasSuffix(name, ".tpl")
		isTmpl := strings.HasSuffix(name, ".tmpl")
		if isTpl || isTmpl {
			shouldRender := render || isTpl
			if shouldRender {
				if isTpl {
					dst = strings.TrimSuffix(dst, ".tpl")
				} else {
					dst = strings.TrimSuffix(dst, ".tmpl")
				}
				return renderTemplateAtomic(path, dst, cfg, workspace)
			}
			return copyFileAtomic(path, dst, workspace)
		}
		// Copy file as-is
		return copyFileAtomic(path, dst, workspace)
	})
	return err
}

// RenderClusterAppsAtomic renders cluster-apps-base template to applications/overlays/<cluster-name>/
// using atomic file operations to prevent partial writes.
//
// This is the workspace-aware version of RenderClusterApps that ensures all file operations
// are atomic and can be rolled back if needed.
func RenderClusterAppsAtomic(cfg config.Config, workspace *GitOpsWorkspace) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	// Try to use PathResolver to get the applications directory relative to workspace
	var target string
	resolver := paths.NewPathResolver(workspace.RootDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err == nil {
		// Successfully resolved paths
		target = clusterPaths.ApplicationsDir
	} else {
		// Fallback to workspace root for test environments or when cluster doesn't exist yet
		target = filepath.Join(workspace.RootDir, "applications", "overlays", clusterName)
	}

	// Walk embedded cluster-apps-base files
	return fs.WalkDir(Files, "templates/cluster-apps-base", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel("templates/cluster-apps-base", path)
		if err != nil {
			return err
		}

		// Skip files for disabled services
		if shouldSkipFile(rel, cfg) {
			return nil
		}

		// Replace cluster-name and cluster_name placeholders in filename
		relWithClusterName := strings.ReplaceAll(rel, "cluster-name", clusterName)
		relWithClusterName = strings.ReplaceAll(relWithClusterName, "cluster_name", clusterName)

		dst := filepath.Join(target, relWithClusterName)

		// If template file, process and strip template extension
		if strings.HasSuffix(d.Name(), ".tmpl") || strings.HasSuffix(d.Name(), ".tpl") {
			if strings.HasSuffix(d.Name(), ".tmpl") {
				dst = strings.TrimSuffix(dst, ".tmpl")
			} else {
				dst = strings.TrimSuffix(dst, ".tpl")
			}
			return renderTemplateAtomic(path, dst, cfg, workspace)
		}

		// Copy file as-is
		return copyFileAtomic(path, dst, workspace)
	})
}

// RenderInfrastructureClusterAtomic renders infrastructure-cluster-template to infrastructure/clusters/<cluster-name>/
// using atomic file operations to prevent partial writes.
//
// This is the workspace-aware version of RenderInfrastructureCluster that ensures all file operations
// are atomic and can be rolled back if needed.
func RenderInfrastructureClusterAtomic(cfg config.Config, workspace *GitOpsWorkspace) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	// Try to use PathResolver to get the cluster directory relative to workspace
	var target string
	resolver := paths.NewPathResolver(workspace.RootDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err == nil {
		// Successfully resolved paths
		target = clusterPaths.ClusterDir
	} else {
		// Fallback to workspace root for test environments or when cluster doesn't exist yet
		target = filepath.Join(workspace.RootDir, "infrastructure", "clusters", clusterName)
	}

	// Determine which main.tf template to use based on provider
	provider := cfg.OpenCenter.Infrastructure.Provider
	if provider == "" {
		provider = "openstack" // default
	}

	// Map provider to template file
	var mainTfTemplate string
	switch provider {
	case "baremetal":
		mainTfTemplate = "main-baremetal.tf.tpl"
	default:
		// openstack and all other providers use main-default.tf.tpl
		mainTfTemplate = "main-default.tf.tpl"
	}

	// Walk embedded infrastructure-cluster-template files
	return fs.WalkDir(Files, "templates/infrastructure-cluster-template", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel("templates/infrastructure-cluster-template", path)
		if err != nil {
			return err
		}

		filename := d.Name()

		// Skip provider-specific main.tf templates that don't match current provider
		if filename == "main-baremetal.tf.tpl" || filename == "main-default.tf.tpl" {
			if filename != mainTfTemplate {
				// Skip this template, it's not for the current provider
				return nil
			}
			// This is the correct template for the provider, render it as main.tf
			dst := filepath.Join(target, "main.tf")
			return renderTemplateAtomic(path, dst, cfg, workspace)
		}

		// Replace cluster-name and cluster_name placeholders in filename
		relWithClusterName := strings.ReplaceAll(rel, "cluster-name", clusterName)
		relWithClusterName = strings.ReplaceAll(relWithClusterName, "cluster_name", clusterName)

		dst := filepath.Join(target, relWithClusterName)

		// If template file, process and strip template extension
		if strings.HasSuffix(d.Name(), ".tmpl") || strings.HasSuffix(d.Name(), ".tpl") {
			if strings.HasSuffix(d.Name(), ".tmpl") {
				dst = strings.TrimSuffix(dst, ".tmpl")
			} else {
				dst = strings.TrimSuffix(dst, ".tpl")
			}
			return renderTemplateAtomic(path, dst, cfg, workspace)
		}

		// Copy file as-is
		return copyFileAtomic(path, dst, workspace)
	})
}
