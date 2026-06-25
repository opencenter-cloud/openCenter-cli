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
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	utilfs "github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
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
	// Create FileSystem instance for file operations
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)

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

		// Read source file using FileSystem wrapper
		data, err := fileSystem.ReadFile(path)
		if err != nil {
			return err
		}

		// Write to destination using FileSystem wrapper (overwrites existing files)
		return fileSystem.WriteFile(dstPath, data, info.Mode())
	})
}

// CopyBase copies or renders embedded files from gitops-base-dir into the target directory
// specified by cfg.GitDir().
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
func CopyBase(cfg v2.Config, render bool) error {
	target := cfg.GitDir()
	if target == "" {
		return fmt.Errorf("opencenter.gitops.repository.local_dir must be set")
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
func renderTemplateAtomic(path, dst string, cfg v2.Config, workspace *GitOpsWorkspace) error {
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

	// Build function map with sprig functions and adoption helpers
	funcMap := sprig.TxtFuncMap()

	// Add adoption mode helper functions
	funcMap["adoptionMode"] = func(serviceName string) string {
		if service, exists := cfg.OpenCenter.Services[serviceName]; exists {
			return string(GetAdoptionMode(service))
		}
		return string(AdoptionModeManaged)
	}

	funcMap["adoptionForce"] = func(serviceName string) bool {
		if service, exists := cfg.OpenCenter.Services[serviceName]; exists {
			return GetServiceAdoptionSettings(service).Force
		}
		return true // Default to force=true (managed behavior)
	}

	funcMap["adoptionSuspend"] = func(serviceName string) bool {
		if service, exists := cfg.OpenCenter.Services[serviceName]; exists {
			return GetServiceAdoptionSettings(service).Suspend
		}
		return false // Default to suspend=false (managed behavior)
	}

	funcMap["managedAdoptionMode"] = func(serviceName string) string {
		if service, exists := managedServices(cfg)[serviceName]; exists {
			return string(GetAdoptionMode(service))
		}
		return string(AdoptionModeManaged)
	}

	funcMap["managedAdoptionForce"] = func(serviceName string) bool {
		if service, exists := managedServices(cfg)[serviceName]; exists {
			return GetServiceAdoptionSettings(service).Force
		}
		return true
	}

	funcMap["managedAdoptionSuspend"] = func(serviceName string) bool {
		if service, exists := managedServices(cfg)[serviceName]; exists {
			return GetServiceAdoptionSettings(service).Suspend
		}
		return false
	}

	// autoServices returns service names that use auto-descriptors (no explicit
	// descriptor) and own their own source (not shared). Used by aggregate templates
	// to include dynamically-added services.
	funcMap["autoServices"] = func() []string {
		registry, err := loadClusterDescriptorRegistry()
		if err != nil {
			return nil
		}
		var names []string
		for name, svc := range cfg.OpenCenter.Services {
			if IsServiceDisabled(svc) || IsServiceExternal(svc) {
				continue
			}
			if hasExplicitDescriptor(registry, name) {
				continue
			}
			base := extractBaseConfig(svc)
			if base == nil {
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	// autoServiceSourceName returns the source name for an auto-descriptor service.
	funcMap["autoServiceSourceName"] = func(serviceName string) string {
		svc, exists := cfg.OpenCenter.Services[serviceName]
		if !exists {
			return "opencenter-" + serviceName
		}
		base := extractBaseConfig(svc)
		if base == nil {
			return "opencenter-" + serviceName
		}
		return base.GetSourceName(serviceName)
	}

	t, err := template.New(filename).Funcs(funcMap).Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", path, err)
	}

	// Execute template to a buffer first
	var buf strings.Builder
	if err := t.Execute(&buf, cfg); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", path, err)
	}

	// Skip writing empty output (templates with conditional rendering may produce empty content)
	output := strings.TrimSpace(buf.String())
	if output == "" {
		return nil
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
//
// Deprecated: This function was part of the convention-based (negative-list) renderer.
// The active renderer uses descriptor-driven planning via planClusterAppActions instead.
// This function is retained for reference and rollback purposes until formal cutover
// approval removes it. See docs/dev/rendering-contract.md for details.
func shouldSkipFile(relPath string, cfg v2.Config) bool {
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

				// Only skip if the service is explicitly present and disabled;
				// services not in config are included by default (template may
				// reference shared or infrastructure sources).
				if service, exists := cfg.OpenCenter.Services[extractedServiceName]; exists {
					if IsServiceDisabled(service) {
						return true
					}
				}
			}
		} else if serviceName != "fluxcd" {
			// Regular service directory check (fluxcd is structural, not a service)
			if service, exists := cfg.OpenCenter.Services[serviceName]; exists {
				if IsServiceDisabled(service) {
					return true
				}
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

				// Only skip if the managed service is explicitly present and disabled
				if service, exists := managedServices(cfg)[extractedServiceName]; exists {
					if IsServiceDisabled(service) {
						return true
					}
				}
			}
		} else if serviceName != "fluxcd" {
			// Regular managed service directory check (fluxcd is structural, not a service)
			if service, exists := managedServices(cfg)[serviceName]; exists {
				if IsServiceDisabled(service) {
					return true
				}
			}
		}
	}

	return false
}

// RenderClusterApps renders cluster-apps-base template to applications/overlays/<cluster-name>/
// This function processes all files in the cluster-apps-base template directory,
// renders .tmpl files with the cluster configuration, and copies others as-is.
// It skips directories for disabled services and managed services.
func RenderClusterApps(cfg v2.Config) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}
	target := filepath.Join(cfg.GitDir(), "applications", "overlays", clusterName)

	// Create target directory
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	// Remove all renderer-owned paths before copying the freshly rendered overlay.
	if err := cleanupRendererOwnedOverlay(target); err != nil {
		return fmt.Errorf("failed to cleanup renderer-owned paths: %w", err)
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

	// Copy from the workspace's applications overlay directory (where RenderClusterAppsAtomic
	// actually writes files) to the final target, avoiding double-nesting of the overlay path.
	workspaceAppsDir := filepath.Join(workspace.RootDir, "applications", "overlays", clusterName)
	return copyWorkspaceToTarget(workspaceAppsDir, target)
}

// cleanupDisabledServices removes service directories that are not enabled in the configuration.
// This ensures that when services are disabled or removed from config, their directories are cleaned up.

// RenderInfrastructureCluster renders infrastructure-cluster-template to infrastructure/clusters/<cluster-name>/
// This function processes all files in the infrastructure-cluster-template directory,
// renders .tmpl and .tpl files with the cluster configuration, and copies others as-is.
// It selects the appropriate main.tf template based on the infrastructure provider type.
func RenderInfrastructureCluster(cfg v2.Config) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	target := cfg.GitDir()
	if target == "" {
		return fmt.Errorf("opencenter.gitops.repository.local_dir must be set")
	}
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
func RenderSingleService(cfg v2.Config, serviceName string, isManaged bool) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}
	target := filepath.Join(cfg.GitDir(), "applications", "overlays", clusterName)

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

	actions, err := planSingleServiceActions(cfg, serviceName, isManaged)
	if err != nil {
		return err
	}

	targetRoot, err := resolveClusterAppsTarget(workspace, cfg)
	if err != nil {
		return err
	}
	if err := writeClusterAppActions(actions, targetRoot, cfg, workspace); err != nil {
		return err
	}

	// For cert-manager, also render dynamic credential files
	if serviceName == "cert-manager" && !isManaged {
		certManagerDir := filepath.Join(targetRoot, "services", "cert-manager")
		if err := renderCertManagerDynamicFiles(cfg, certManagerDir, workspace); err != nil {
			return fmt.Errorf("rendering cert-manager dynamic files: %w", err)
		}
	}

	if err := cleanupSingleServiceOutputs(target, serviceName, isManaged, actions); err != nil {
		return err
	}
	return copyWorkspaceToTarget(targetRoot, target)
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
func CopyBaseAtomic(cfg v2.Config, render bool, workspace *GitOpsWorkspace) error {
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
func RenderClusterAppsAtomic(cfg v2.Config, workspace *GitOpsWorkspace) error {
	target, err := resolveClusterAppsTarget(workspace, cfg)
	if err != nil {
		return err
	}

	actions, err := planClusterAppActions(cfg)
	if err != nil {
		return err
	}

	if err := writeClusterAppActions(actions, target, cfg, workspace); err != nil {
		return err
	}

	// Render dynamic cert-manager credential files (secrets + issuers + kustomization).
	// These are rendered outside the descriptor pipeline because they produce N files
	// from a map of enabled credentials rather than a fixed set of templates.
	certManagerDir := filepath.Join(target, "services", "cert-manager")
	if err := renderCertManagerDynamicFiles(cfg, certManagerDir, workspace); err != nil {
		return fmt.Errorf("rendering cert-manager dynamic files: %w", err)
	}

	return nil
}

// RenderInfrastructureClusterAtomic renders infrastructure-cluster-template to infrastructure/clusters/<cluster-name>/
// using atomic file operations to prevent partial writes.
//
// This is the workspace-aware version of RenderInfrastructureCluster that ensures all file operations
// are atomic and can be rolled back if needed.
func RenderInfrastructureClusterAtomic(cfg v2.Config, workspace *GitOpsWorkspace) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	target := filepath.Join(workspace.RootDir, "infrastructure", "clusters", clusterName)

	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	if provider == "kind" {
		dst := filepath.Join(target, "kind-config.yaml")
		return renderTemplateAtomic("templates/kind-config.yaml.tpl", dst, cfg, workspace)
	}

	// Determine which main.tf template to use based on provider and deployment method.
	provider = strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	if provider == "" {
		provider = "openstack" // default
	}
	// Map provider to template file
	var mainTfTemplate string
	switch provider {
	case "baremetal":
		mainTfTemplate = "main-baremetal.tf.tpl"
	case "vmware":
		mainTfTemplate = "main-vmware.tf.tpl"
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
		if filename == "main-baremetal.tf.tpl" || filename == "main-vmware.tf.tpl" || filename == "main-default.tf.tpl" {
			if filename != mainTfTemplate {
				// Skip this template, it's not for the current provider
				return nil
			}
			// This is the correct template for the provider, render it as main.tf
			dst := filepath.Join(target, "main.tf")
			return renderTemplateAtomic(path, dst, cfg, workspace)
		}

		// Skip talos/ directory since Talos is no longer supported
		if strings.HasPrefix(filepath.ToSlash(rel), "talos/") {
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
