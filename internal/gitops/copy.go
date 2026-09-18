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
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
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
		catalog := newBuiltInRenderCatalog()
		if err != nil {
			return nil
		}
		var names []string
		for name, svc := range cfg.OpenCenter.Services {
			if IsServiceDisabled(svc) || IsServiceExternal(svc) {
				continue
			}
			if hasExplicitDescriptor(registry, name, serviceKindStandard) {
				continue
			}
			if _, owned := catalog.Lookup(name); !owned {
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	// autoNamespaceStages returns namespace-stage names for auto-descriptor services
	// that need their namespace created before their override reconciles.
	funcMap["autoNamespaceStages"] = func() []string {
		registry, err := loadClusterDescriptorRegistry()
		catalog := newBuiltInRenderCatalog()
		if err != nil {
			return nil
		}
		var names []string
		for name, svc := range cfg.OpenCenter.Services {
			if IsServiceDisabled(svc) || IsServiceExternal(svc) || hasExplicitDescriptor(registry, name, serviceKindStandard) {
				continue
			}
			spec, owned := catalog.Lookup(name)
			if owned && spec.NamespaceStage {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		return names
	}

	// autoServiceSourceName returns the source name for an auto-descriptor service.
	funcMap["autoServiceSourceName"] = func(serviceName string) string {
		if spec, ok := newBuiltInRenderCatalog().Lookup(serviceName); ok {
			return spec.SourceName
		}
		return "opencenter-" + serviceName
	}

	// sourceAuthBlock renders the shared active-and-commented GitRepository
	// auth block for a repository/ref pair. Auth selection is carried on the
	// run-only config field and never inferred from the URL scheme.
	sourceAuthBlock := func(repositoryURL, refType, refValue, secretName string) (string, error) {
		params, err := BuildSourceAuthParams(cfg.OpenCenter.GitOps.ResolvedAuthMethod, repositoryURL, refType, refValue, secretName)
		if err != nil {
			return "", err
		}
		return RenderSourceAuthBlock(params), nil
	}

	// sourceAuthBlockAnonymous renders a source with no secretRef. The shared
	// openCenter-gitops-base repository is public, so its GitRepository source
	// must access it anonymously rather than reuse the token-derived
	// opencenter-base Secret (which in local mode holds the Gitea token and
	// makes public-GitHub access fail with HTTP 401).
	sourceAuthBlockAnonymous := func(repositoryURL, refType, refValue string) (string, error) {
		params, err := BuildSourceAuthParams(cfg.OpenCenter.GitOps.ResolvedAuthMethod, repositoryURL, refType, refValue, "")
		if err != nil {
			return "", err
		}
		params.Anonymous = true
		return RenderSourceAuthBlock(params), nil
	}

	// sourceAuthBlockForService preserves service Source.Repo, Source.Branch,
	// and Source.Release overrides while providing both auth variants.
	funcMap["sourceAuthBlockForService"] = func(serviceName string) (string, error) {
		repositoryURL := cfg.OpenCenter.GitOps.BaseRepo.URL
		branch := cfg.OpenCenter.GitOps.BaseRepo.Branch
		release := cfg.OpenCenter.GitOps.BaseRepo.Release

		if svc, exists := cfg.OpenCenter.Services[serviceName]; exists {
			if base := extractBaseConfig(svc); base != nil {
				if value := strings.TrimSpace(base.Source.Repo); value != "" {
					repositoryURL = value
				}
				if value := strings.TrimSpace(base.Source.Release); value != "" {
					release = value
					branch = ""
				} else if value := strings.TrimSpace(base.Source.Branch); value != "" {
					branch = value
					release = ""
				}
			}
		}

		refType := "branch"
		refValue := strings.TrimSpace(branch)
		if strings.TrimSpace(release) != "" {
			refType = "tag"
			refValue = strings.TrimSpace(release)
		} else if refValue == "" {
			refValue = strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.Branch)
			if refValue == "" {
				refValue = "main"
			}
		}
		return sourceAuthBlockAnonymous(repositoryURL, refType, refValue)
	}

	// sourceAuthBlockDefault renders the base repository source using its
	// shared branch/release settings.
	funcMap["sourceAuthBlockDefault"] = func() (string, error) {
		branch := strings.TrimSpace(cfg.OpenCenter.GitOps.BaseRepo.Branch)
		release := strings.TrimSpace(cfg.OpenCenter.GitOps.BaseRepo.Release)
		refType := "branch"
		refValue := branch
		if release != "" {
			refType = "tag"
			refValue = release
		} else if refValue == "" {
			refValue = strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.Branch)
			if refValue == "" {
				refValue = "main"
			}
		}
		return sourceAuthBlockAnonymous(cfg.OpenCenter.GitOps.BaseRepo.URL, refType, refValue)
	}

	// sourceAuthBlockCustomerRepository retains customer-repository semantics
	// for sources such as keycloak-config while using the same renderer.
	funcMap["sourceAuthBlockCustomerRepository"] = func() (string, error) {
		branch := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.Branch)
		if branch == "" {
			branch = "main"
		}
		return sourceAuthBlock(cfg.OpenCenter.GitOps.Repository.URL, "branch", branch, "flux-system")
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

// OverlayEncryptor encrypts the generated cluster-apps overlay before it is promoted.
// The callback operates on the temporary workspace path, so failures leave the
// final target unchanged.
type OverlayEncryptor func(ctx context.Context, overlayPath string, cfg *v2.Config) error

// PlanClusterAppsPromotionWithOptions renders cluster applications into an
// isolated workspace and classifies promotion with mutation disabled.
func PlanClusterAppsPromotionWithOptions(cfg v2.Config, opts PromoteOptions) (*PromoteResult, error) {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is empty")
	}
	if cfg.GitDir() == "" {
		return nil, fmt.Errorf("opencenter.gitops.repository.local_dir must be set")
	}
	manager := NewWorkspaceManager(os.TempDir())
	workspace, err := manager.CreateWorkspace(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dry-run workspace: %w", err)
	}
	defer manager.CleanupWorkspace(context.Background(), workspace)
	if err := RenderClusterAppsAtomic(cfg, workspace); err != nil {
		return nil, fmt.Errorf("rendering cluster apps: %w", err)
	}
	workspaceOverlayDir := filepath.Join(workspace.RootDir, "applications", "overlays", clusterName)
	targetOverlayDir := filepath.Join(cfg.GitDir(), "applications", "overlays", clusterName)
	opts.DryRun = true
	return promoteOverlay(workspaceOverlayDir, targetOverlayDir, clusterName, opts)
}

// RenderClusterApps renders cluster-apps-base and promotes it to the final target.
// It is the raw rendering primitive; production callers that can materialize
// credentials must use RenderClusterAppsWithEncryption.
func RenderClusterApps(cfg v2.Config) error {
	_, err := renderClusterAppsWithOptions(context.Background(), cfg, nil, PromoteOptions{})
	return err
}

// RenderClusterAppsWithEncryption renders and returns the complete promotion
// classification. The legacy error-only wrapper remains available above.
func RenderClusterAppsWithEncryptionResult(ctx context.Context, cfg v2.Config, encrypt OverlayEncryptor, opts PromoteOptions) (*PromoteResult, error) {
	if encrypt == nil {
		return nil, fmt.Errorf("overlay encryptor is required")
	}
	return renderClusterAppsWithOptions(ctx, cfg, encrypt, opts)
}

// RenderClusterAppsWithEncryption renders cluster-apps-base into a temporary
// workspace, encrypts credential-bearing files, and only then promotes it to
// the final target.
func RenderClusterAppsWithEncryption(ctx context.Context, cfg v2.Config, encrypt OverlayEncryptor) error {
	_, err := RenderClusterAppsWithEncryptionResult(ctx, cfg, encrypt, PromoteOptions{})
	return err
}

func renderClusterApps(ctx context.Context, cfg v2.Config, encrypt OverlayEncryptor) error {
	_, err := renderClusterAppsWithOptions(ctx, cfg, encrypt, PromoteOptions{})
	return err
}

func renderClusterAppsWithOptions(ctx context.Context, cfg v2.Config, encrypt OverlayEncryptor, opts PromoteOptions) (*PromoteResult, error) {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is empty")
	}
	target := filepath.Join(cfg.GitDir(), "applications", "overlays", clusterName)

	// The promotion layer creates the target only after ownership preflight.

	// Create a temporary workspace for atomic operations
	tempDir := os.TempDir()
	manager := NewWorkspaceManager(tempDir)
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating workspace: %w", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	if err := RenderClusterAppsAtomic(cfg, workspace); err != nil {
		return nil, err
	}

	workspaceAppsDir := filepath.Join(workspace.RootDir, "applications", "overlays", clusterName)
	if encrypt != nil {
		if err := encrypt(ctx, workspaceAppsDir, &cfg); err != nil {
			return nil, fmt.Errorf("encrypting cluster apps overlay: %w", err)
		}
	}

	result, err := promoteOverlay(workspaceAppsDir, target, clusterName, opts)
	if err != nil {
		return nil, err
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	return result, nil
}

// cleanupDisabledServices removes service directories that are not enabled in the configuration.
// This ensures that when services are disabled or removed from config, their directories are cleaned up.

// RenderClusterFluxBridge renders the bridge Flux Kustomization into
// clusters/<cluster-name>/services.yaml. Flux's own `flux bootstrap` writes
// clusters/<cluster-name>/flux-system/, but nothing there references the
// per-service Flux Kustomizations under applications/overlays/<cluster-name>/
// services/fluxcd. Without this bridge, Flux reconciles only its own
// source-controller and stops.
//
// No top-level kustomization.yaml is emitted here - Flux's kustomize-controller
// auto-generates a kustomization.yaml at reconcile time when none is present,
// aggregating every YAML at the path (including files under flux-system/ and
// this services.yaml).
//
// This function never touches clusters/<cluster-name>/flux-system/ (owned by
// flux bootstrap).
func RenderClusterFluxBridge(cfg v2.Config) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}
	gitDir := cfg.GitDir()
	if gitDir == "" {
		return fmt.Errorf("opencenter.gitops.repository.local_dir must be set")
	}

	target := filepath.Join(gitDir, "clusters", clusterName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	tempDir := os.TempDir()
	manager := NewWorkspaceManager(tempDir)
	workspace, err := manager.CreateWorkspace(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}
	defer manager.CleanupWorkspace(context.Background(), workspace)

	if err := RenderClusterFluxBridgeAtomic(cfg, workspace); err != nil {
		return err
	}

	workspaceBridgeDir := filepath.Join(workspace.RootDir, "clusters", clusterName)
	return copyWorkspaceToTarget(workspaceBridgeDir, target)
}

// RenderClusterFluxBridgeAtomic is the workspace-aware version of
// RenderClusterFluxBridge. See RenderClusterFluxBridge for behaviour.
func RenderClusterFluxBridgeAtomic(cfg v2.Config, workspace *GitOpsWorkspace) error {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}
	target := filepath.Join(workspace.RootDir, "clusters", clusterName)

	src := filepath.ToSlash(filepath.Join("templates", "cluster-flux-bridge", "services.yaml.tpl"))
	dst := filepath.Join(target, "services.yaml")
	if err := renderTemplateAtomic(src, dst, cfg, workspace); err != nil {
		return fmt.Errorf("render services.yaml: %w", err)
	}

	// OCTR-759: when hostname substitution is enabled, emit the cluster-vars
	// ConfigMap here under clusters/<cluster>/ so it is reconciled by the
	// flux-system root Kustomization (same reconciliation path as services.yaml)
	// and lands in the flux-system namespace referenced by every
	// postBuild.substituteFrom. The ConfigStage fallback render lives on the
	// no-templates path and is pruned on the normal template-based generate, so
	// the placeholders would otherwise have no source ConfigMap.
	hs := cfg.OpenCenter.Cluster.HostnameSubstitution
	if hs.Enabled {
		cmDst := filepath.Join(target, "cluster-vars-configmap.yaml")
		writer := NewAtomicWriter(workspace)
		if err := writer.WriteFileString(
			mustRel(workspace.RootDir, cmDst),
			renderClusterVarsConfigMapYAML(cfg),
			0o644,
		); err != nil {
			return fmt.Errorf("render cluster-vars ConfigMap: %w", err)
		}
	}
	return nil
}

// renderClusterVarsConfigMapYAML builds the cluster-vars ConfigMap consumed by
// Flux postBuild.substituteFrom (OCTR-759). Keys are the placeholder names and
// values are the resolved cluster-config fields, sorted for deterministic
// output. It is written into clusters/<cluster>/ so the flux-system root
// Kustomization applies it into the flux-system namespace.
func renderClusterVarsConfigMapYAML(cfg v2.Config) string {
	hs := cfg.OpenCenter.Cluster.HostnameSubstitution
	vars := hs.GetVariables()
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)

	var data strings.Builder
	for _, name := range names {
		field := vars[name]
		var value string
		switch field {
		case "cluster_fqdn":
			value = cfg.OpenCenter.Cluster.ClusterFQDN
		case "base_domain":
			value = cfg.OpenCenter.Cluster.BaseDomain
		case "cluster_name":
			value = cfg.OpenCenter.Cluster.ClusterName
		}
		data.WriteString(fmt.Sprintf("  %s: %q\n", name, value))
	}
	return fmt.Sprintf(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: flux-system
data:
%s`, hs.GetConfigMapName(), data.String())
}

// mustRel returns the path relative to root, or the original path if that fails.
func mustRel(root, p string) string {
	if rel, err := filepath.Rel(root, p); err == nil {
		return rel
	}
	return p
}

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

func RenderSingleService(cfg v2.Config, serviceName string, isManaged bool) error {
	_, err := renderSingleServiceWithOptions(context.Background(), cfg, serviceName, isManaged, nil, PromoteOptions{})
	return err
}

// RenderSingleServiceWithEncryptionResult returns the scoped promotion result.
func RenderSingleServiceWithEncryptionResult(ctx context.Context, cfg v2.Config, serviceName string, isManaged bool, encrypt OverlayEncryptor, opts PromoteOptions) (*PromoteResult, error) {
	if encrypt == nil {
		return nil, fmt.Errorf("overlay encryptor is required")
	}
	return renderSingleServiceWithOptions(ctx, cfg, serviceName, isManaged, encrypt, opts)
}

// RenderSingleServiceWithEncryption renders a service into a temporary
// workspace and promotes its scoped outputs.
func RenderSingleServiceWithEncryption(ctx context.Context, cfg v2.Config, serviceName string, isManaged bool, encrypt OverlayEncryptor) error {
	_, err := RenderSingleServiceWithEncryptionResult(ctx, cfg, serviceName, isManaged, encrypt, PromoteOptions{})
	return err
}

func renderSingleService(ctx context.Context, cfg v2.Config, serviceName string, isManaged bool, encrypt OverlayEncryptor) error {
	_, err := renderSingleServiceWithOptions(ctx, cfg, serviceName, isManaged, encrypt, PromoteOptions{})
	return err
}

func renderSingleServiceWithOptions(ctx context.Context, cfg v2.Config, serviceName string, isManaged bool, encrypt OverlayEncryptor, opts PromoteOptions) (*PromoteResult, error) {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is empty")
	}
	actions, artifacts, err := planSingleServiceActionsWithArtifacts(cfg, serviceName, isManaged)
	if err != nil {
		return nil, err
	}
	target := filepath.Join(cfg.GitDir(), "applications", "overlays", clusterName)

	// The promotion layer creates the target only after ownership preflight.

	// Create a temporary workspace for atomic operations
	tempDir := os.TempDir()
	manager := NewWorkspaceManager(tempDir)
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating workspace: %w", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	prefix := "services"
	if isManaged {
		prefix = "managed-services"
	}
	scopes := []string{filepath.ToSlash(filepath.Join(prefix, serviceName))}
	for _, action := range actions {
		rel, err := normalizeOwnershipPath(action.Output)
		if err != nil {
			return nil, fmt.Errorf("invalid descriptor output %q: %w", action.Output, err)
		}
		if !strings.HasPrefix(rel, scopes[0]+"/") && rel != scopes[0] {
			scopes = append(scopes, rel)
		}
	}

	targetRoot, err := resolveClusterAppsTarget(workspace, cfg)
	if err != nil {
		return nil, err
	}
	if err := writeClusterAppActions(actions, targetRoot, cfg, workspace); err != nil {
		return nil, err
	}

	validationArtifacts := filterSingleServiceArtifacts(serviceName, actions, artifacts)
	if err := validateMaterializedSecretMembership(cfg, actions, validationArtifacts, targetRoot); err != nil {
		return nil, err
	}

	if encrypt != nil {
		if err := encrypt(ctx, targetRoot, &cfg); err != nil {
			return nil, fmt.Errorf("encrypting cluster apps overlay: %w", err)
		}
	}

	opts.Scope = scopes
	result, err := promoteOverlay(targetRoot, target, clusterName, opts)
	if err != nil {
		return nil, err
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	return result, nil
}

func filterSingleServiceArtifacts(serviceName string, actions []clusterAppAction, artifacts []secretartifacts.Artifact) []secretartifacts.Artifact {
	actionOutputs := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		actionOutputs[filepath.ToSlash(action.Output)] = struct{}{}
	}

	filtered := make([]secretartifacts.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.TargetService == serviceName {
			filtered = append(filtered, artifact)
			continue
		}

		artifactKustomization := filepath.ToSlash(filepath.Join(
			filepath.Dir(filepath.FromSlash(artifact.Path)),
			"kustomization.yaml",
		))
		if _, ok := actionOutputs[artifactKustomization]; ok {
			filtered = append(filtered, artifact)
		}
	}
	return filtered
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

	actions, artifacts, err := planClusterAppActionsWithArtifacts(cfg)
	if err != nil {
		return err
	}
	if err := writeClusterAppActions(actions, target, cfg, workspace); err != nil {
		return err
	}
	if err := validateMaterializedSecretMembership(cfg, actions, artifacts, target); err != nil {
		return err
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

// StagedGenerationOptions controls complete-tree staging and promotion. The staged
// tree is the only render pass; validation, ownership planning, and apply all
// consume the same workspace contents.
type StagedGenerationOptions struct {
	Materialize           func(string) error
	Encrypt               OverlayEncryptor
	ValidateManifest      func(string) error
	IncludeInfrastructure bool
	IncludeFluxBridge     bool
	Promote               PromoteOptions
	BeforePromote         func() error
}

// GenerateClusterTree renders the requested GitOps tree once in a private
// workspace, validates it, runs ownership preflight without mutation, and
// promotes the already validated stage. It never writes the live target before
// the ownership preflight succeeds.
func GenerateClusterTree(ctx context.Context, cfg v2.Config, opts StagedGenerationOptions) (*PromoteResult, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.ClusterName() == "" {
		return nil, 0, fmt.Errorf("cluster name is empty")
	}
	if cfg.GitDir() == "" {
		return nil, 0, fmt.Errorf("opencenter.gitops.repository.local_dir must be set")
	}

	manager := NewWorkspaceManager(os.TempDir())
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		return nil, 0, fmt.Errorf("creating generation workspace: %w", err)
	}
	defer func() {
		_ = manager.CleanupWorkspace(ctx, workspace)
		if shutdown, ok := manager.(interface{ Shutdown(context.Context) error }); ok {
			_ = shutdown.Shutdown(ctx)
		}
	}()

	if err := CopyBaseAtomic(cfg, true, workspace); err != nil {
		return nil, 0, fmt.Errorf("rendering base GitOps structure: %w", err)
	}
	if err := RenderClusterAppsAtomic(cfg, workspace); err != nil {
		return nil, 0, fmt.Errorf("rendering cluster apps: %w", err)
	}

	clusterName := cfg.ClusterName()
	stagedOverlay := filepath.Join(workspace.RootDir, "applications", "overlays", clusterName)
	if opts.IncludeInfrastructure {
		if err := RenderInfrastructureClusterAtomic(cfg, workspace); err != nil {
			return nil, 0, fmt.Errorf("rendering infrastructure cluster: %w", err)
		}
	}
	if opts.IncludeFluxBridge {
		if err := RenderClusterFluxBridgeAtomic(cfg, workspace); err != nil {
			return nil, 0, fmt.Errorf("rendering cluster flux bridge: %w", err)
		}
	}
	if opts.Materialize != nil {
		if err := opts.Materialize(workspace.RootDir); err != nil {
			return nil, 0, fmt.Errorf("materializing staged generated files: %w", err)
		}
	}
	if opts.Encrypt != nil {
		if err := opts.Encrypt(ctx, stagedOverlay, &cfg); err != nil {
			return nil, 0, fmt.Errorf("encrypting staged cluster apps: %w", err)
		}
	}
	if opts.ValidateManifest != nil {
		if err := opts.ValidateManifest(workspace.RootDir); err != nil {
			return nil, 0, fmt.Errorf("manifest validation failed: %w", err)
		}
	}

	planOptions := opts.Promote
	planOptions.DryRun = true
	planned, err := promoteGeneratedTree(workspace.RootDir, cfg.GitDir(), clusterName, planOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("complete-tree ownership preflight refused promotion: %w", err)
	}
	manifestCount, err := countWorkspaceFiles(workspace.RootDir)
	if err != nil {
		return nil, 0, fmt.Errorf("counting staged files: %w", err)
	}
	if opts.Promote.DryRun {
		return planned, manifestCount, nil
	}
	if opts.BeforePromote != nil {
		if err := opts.BeforePromote(); err != nil {
			return nil, 0, fmt.Errorf("pre-promotion preparation failed: %w", err)
		}
	}

	promoted, err := promoteGeneratedTree(workspace.RootDir, cfg.GitDir(), clusterName, opts.Promote)
	if err != nil {
		return nil, 0, fmt.Errorf("promoting staged generated tree: %w", err)
	}
	return promoted, manifestCount, nil
}

func countWorkspaceFiles(root string) (int, error) {
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(filepath.Join(root, ".tmp"))+"/") {
			count++
		}
		return nil
	})
	return count, err
}
