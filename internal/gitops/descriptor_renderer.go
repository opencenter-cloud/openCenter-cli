package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	descriptorcfg "github.com/opencenter-cloud/opencenter-cli/internal/services/descriptors"
	"gopkg.in/yaml.v3"
)

type clusterAppAction struct {
	Owner    string
	Template string
	Output   string
	Render   bool
	Content  string // Pre-rendered content (used by auto-descriptors). When set, Template is ignored.
}

// lastRenderDiagnostics stores the diagnostics from the most recent
// planClusterAppActions call. It is intended for test and debugging use only.
var lastRenderDiagnostics *RenderDiagnostics

var (
	clusterDescriptorOnce     sync.Once
	clusterDescriptorRegistry *descriptorcfg.Registry
	clusterDescriptorErr      error
)

func loadClusterDescriptorRegistry() (*descriptorcfg.Registry, error) {
	clusterDescriptorOnce.Do(func() {
		clusterDescriptorRegistry, clusterDescriptorErr = descriptorcfg.LoadEmbedded()
	})
	return clusterDescriptorRegistry, clusterDescriptorErr
}

func resolveClusterAppsTarget(workspace *GitOpsWorkspace, cfg v2.Config) (string, error) {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return "", fmt.Errorf("cluster name is empty")
	}

	resolver := paths.NewPathResolver(workspace.RootDir)
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), clusterName)
	if err == nil {
		return clusterPaths.ApplicationsDir, nil
	}

	return filepath.Join(workspace.RootDir, "applications", "overlays", clusterName), nil
}

func renderOutputPath(path string, cfg v2.Config) (string, error) {
	if path == "" {
		return "", fmt.Errorf("output path is empty")
	}

	rendered := strings.ReplaceAll(path, "cluster-name", cfg.ClusterName())
	rendered = strings.ReplaceAll(rendered, "cluster_name", cfg.ClusterName())
	if !strings.Contains(rendered, "{{") {
		return rendered, nil
	}

	tmpl, err := template.New("output-path").Funcs(sprig.TxtFuncMap()).Parse(rendered)
	if err != nil {
		return "", fmt.Errorf("parse output path template %q: %w", path, err)
	}

	var builder strings.Builder
	if err := tmpl.Execute(&builder, cfg); err != nil {
		return "", fmt.Errorf("render output path template %q: %w", path, err)
	}

	return builder.String(), nil
}

func normalizeRenderedOutput(path string) string {
	switch {
	case strings.HasSuffix(path, ".yaml.jtpl"):
		return strings.TrimSuffix(path, ".jtpl")
	case strings.HasSuffix(path, ".jtpl"):
		return strings.TrimSuffix(path, ".jtpl")
	case strings.HasSuffix(path, ".tmpl"):
		return strings.TrimSuffix(path, ".tmpl")
	case strings.HasSuffix(path, ".tpl"):
		return strings.TrimSuffix(path, ".tpl")
	default:
		return path
	}
}

func inferDescriptorRender(path string, override *bool) bool {
	if override != nil {
		return *override
	}

	return strings.HasSuffix(path, ".tpl") || strings.HasSuffix(path, ".tmpl") || strings.HasSuffix(path, ".jtpl")
}

// validateClusterAppActions validates every output path before any action can
// write. Outputs must already use the normalized ownership-path form so the
// planner and writer cannot disagree about the destination.
func validateClusterAppActions(actions []clusterAppAction, renderRoot string) error {
	for index, action := range actions {
		output := action.Output
		if strings.TrimSpace(output) == "" {
			return fmt.Errorf("action %d owned by %q has an empty output path", index, action.Owner)
		}
		if strings.ContainsRune(output, '\x00') || strings.Contains(output, "\\") {
			return fmt.Errorf("action %d owned by %q has invalid output path %q: path separators must be forward slashes", index, action.Owner, output)
		}
		if filepath.IsAbs(output) {
			return fmt.Errorf("action %d owned by %q has invalid output path %q: absolute paths are not allowed", index, action.Owner, output)
		}

		normalized, err := normalizeOwnershipPath(output)
		if err != nil {
			return fmt.Errorf("action %d owned by %q has invalid output path %q: %w", index, action.Owner, output, err)
		}
		if normalized == "." || normalized != output {
			return fmt.Errorf("action %d owned by %q has invalid output path %q: it must be a normalized relative ownership path", index, action.Owner, output)
		}

		if renderRoot != "" {
			full := filepath.Join(renderRoot, filepath.FromSlash(output))
			contained, err := safeRelativePath(renderRoot, full)
			if err != nil {
				return fmt.Errorf("action %d owned by %q has output path %q outside render root %q: %w", index, action.Owner, output, renderRoot, err)
			}
			if contained != output {
				return fmt.Errorf("action %d owned by %q has output path %q outside render root %q", index, action.Owner, output, renderRoot)
			}
		}
	}
	return nil
}

func buildConfigView(cfg v2.Config) (map[string]any, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config view: %w", err)
	}

	view := make(map[string]any)
	if err := json.Unmarshal(data, &view); err != nil {
		return nil, fmt.Errorf("unmarshal config view: %w", err)
	}

	return view, nil
}

func lookupViewField(view map[string]any, field string) (any, bool) {
	current := any(view)
	for _, part := range strings.Split(field, ".") {
		next, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, exists := next[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func evaluateDescriptorCondition(view map[string]any, condition *descriptorcfg.Condition) (bool, error) {
	if condition == nil {
		return true, nil
	}

	value, exists := lookupViewField(view, condition.Field)
	switch condition.Operator {
	case descriptorcfg.ConditionOperatorExists:
		return exists, nil
	case descriptorcfg.ConditionOperatorTrue:
		if !exists {
			return false, nil
		}
		boolean, ok := value.(bool)
		return ok && boolean, nil
	case descriptorcfg.ConditionOperatorFalse:
		if !exists {
			return false, nil
		}
		boolean, ok := value.(bool)
		return ok && !boolean, nil
	case descriptorcfg.ConditionOperatorEquals:
		if !exists {
			return false, nil
		}
		return fmt.Sprint(value) == condition.Value, nil
	case descriptorcfg.ConditionOperatorNotEquals:
		if !exists {
			return true, nil
		}
		return fmt.Sprint(value) != condition.Value, nil
	default:
		return false, fmt.Errorf("unsupported descriptor operator %q", condition.Operator)
	}
}

func isDescriptorEnabled(cfg v2.Config, view map[string]any, descriptor descriptorcfg.Descriptor) (bool, error) {
	if descriptor.Service != "" {
		service, exists := cfg.OpenCenter.Services[descriptor.Service]
		if !exists || IsServiceDisabled(service) {
			return false, nil
		}
		// Check if service is externally managed (skip rendering)
		if IsServiceExternal(service) {
			return false, nil
		}
		return true, nil
	}
	if descriptor.ManagedService != "" {
		service, exists := managedServices(cfg)[descriptor.ManagedService]
		if !exists || IsServiceDisabled(service) {
			return false, nil
		}
		// Check if managed service is externally managed (skip rendering)
		if IsServiceExternal(service) {
			return false, nil
		}
		return true, nil
	}
	return evaluateDescriptorCondition(view, descriptor.EnabledWhen)
}

func expandDescriptorActions(descriptor descriptorcfg.Descriptor, cfg v2.Config, view map[string]any) ([]clusterAppAction, error) {
	var actions []clusterAppAction

	for _, root := range descriptor.Roots {
		ok, err := evaluateDescriptorCondition(view, root.When)
		if err != nil {
			return nil, fmt.Errorf("descriptor %s root %s: %w", descriptor.Name, root.Path, err)
		}
		if !ok {
			continue
		}

		rootPath := filepath.Join("templates/cluster-apps-base", filepath.Clean(root.Path))
		outputRoot := root.Path
		if strings.TrimSpace(root.Output) != "" {
			outputRoot = root.Output
		}

		excluded := make(map[string]struct{}, len(root.Excludes))
		for _, item := range root.Excludes {
			excluded[filepath.Clean(item)] = struct{}{}
		}

		err = fs.WalkDir(Files, rootPath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(rootPath, path)
			if err != nil {
				return err
			}
			if _, skip := excluded[filepath.Clean(rel)]; skip {
				return nil
			}

			outputPath := filepath.Join(outputRoot, rel)
			outputPath, err = renderOutputPath(outputPath, cfg)
			if err != nil {
				return err
			}

			actions = append(actions, clusterAppAction{
				Owner:    descriptor.Name,
				Template: path,
				Output:   normalizeRenderedOutput(outputPath),
				Render:   inferDescriptorRender(path, nil),
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("expand descriptor root %s: %w", root.Path, err)
		}
	}

	for _, file := range descriptor.Files {
		ok, err := evaluateDescriptorCondition(view, file.When)
		if err != nil {
			return nil, fmt.Errorf("descriptor %s file %s: %w", descriptor.Name, file.Template, err)
		}
		if !ok {
			continue
		}

		templatePath := filepath.Join("templates/cluster-apps-base", filepath.Clean(file.Template))
		outputPath := file.Template
		if strings.TrimSpace(file.Output) != "" {
			outputPath = file.Output
		}
		outputPath, err = renderOutputPath(outputPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("descriptor %s output %s: %w", descriptor.Name, outputPath, err)
		}

		actions = append(actions, clusterAppAction{
			Owner:    descriptor.Name,
			Template: templatePath,
			Output:   normalizeRenderedOutput(outputPath),
			Render:   inferDescriptorRender(file.Template, file.Render),
		})
	}

	return actions, nil
}

func validateDescriptorCoverage(registry *descriptorcfg.Registry) error {
	if registry == nil {
		return fmt.Errorf("descriptor registry is nil")
	}

	owners := make(map[string][]string)
	for _, descriptor := range registry.Descriptors() {
		for _, root := range descriptor.Roots {
			rootPath := filepath.Join("templates/cluster-apps-base", filepath.Clean(root.Path))
			excluded := make(map[string]struct{}, len(root.Excludes))
			for _, item := range root.Excludes {
				excluded[filepath.Clean(item)] = struct{}{}
			}
			err := fs.WalkDir(Files, rootPath, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(rootPath, path)
				if err != nil {
					return err
				}
				if _, skip := excluded[filepath.Clean(rel)]; skip {
					return nil
				}
				owners[path] = append(owners[path], descriptor.Name)
				return nil
			})
			if err != nil {
				return fmt.Errorf("expand coverage root %s: %w", root.Path, err)
			}
		}
		for _, file := range descriptor.Files {
			templatePath := filepath.Join("templates/cluster-apps-base", filepath.Clean(file.Template))
			owners[templatePath] = append(owners[templatePath], descriptor.Name)
		}
	}

	var missing []string
	var duplicated []string
	err := fs.WalkDir(Files, "templates/cluster-apps-base", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		descriptorOwners := owners[path]
		switch len(descriptorOwners) {
		case 0:
			missing = append(missing, path)
		case 1:
			return nil
		default:
			duplicated = append(duplicated, fmt.Sprintf("%s => %s", path, strings.Join(descriptorOwners, ",")))
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(missing) > 0 || len(duplicated) > 0 {
		slices.Sort(missing)
		slices.Sort(duplicated)
		return fmt.Errorf("descriptor coverage mismatch: missing=%v duplicate=%v", missing, duplicated)
	}

	return nil
}

func planClusterAppActions(cfg v2.Config) ([]clusterAppAction, error) {
	actions, _, err := planClusterAppActionsWithArtifacts(cfg)
	return actions, err
}

func planClusterAppActionsWithArtifacts(cfg v2.Config) ([]clusterAppAction, []secretartifacts.Artifact, error) {
	if err := validateOverlayUnitConfig(cfg); err != nil {
		return nil, nil, err
	}

	registry, err := loadClusterDescriptorRegistry()
	if err != nil {
		return nil, nil, err
	}
	if err := validateDescriptorCoverage(registry); err != nil {
		return nil, nil, err
	}
	catalog := newBuiltInRenderCatalog()
	if err := catalog.ValidateConfigOwnership(cfg, registry); err != nil {
		return nil, nil, err
	}

	view, err := buildConfigView(cfg)
	if err != nil {
		return nil, nil, err
	}
	artifacts, err := secretartifacts.Plan(&cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("plan secret artifacts: %w", err)
	}
	if err := secretartifacts.ValidateTargets(&cfg, artifacts); err != nil {
		return nil, nil, err
	}

	diag := &RenderDiagnostics{
		Cluster: cfg.ClusterName(),
	}

	var actions []clusterAppAction
	for _, descriptor := range registry.Descriptors() {
		enabled, err := isDescriptorEnabled(cfg, view, descriptor)
		if err != nil {
			return nil, nil, fmt.Errorf("descriptor %s: %w", descriptor.Name, err)
		}

		diag.Descriptors = append(diag.Descriptors, DescriptorDecision{
			Name:    descriptor.Name,
			Enabled: enabled,
			Reason:  descriptorEnableReason(descriptor, enabled),
		})

		if !enabled {
			continue
		}
		expanded, err := expandDescriptorActions(descriptor, cfg, view)
		if err != nil {
			return nil, nil, err
		}
		actions = append(actions, expanded...)
		dynamicActions, err := catalog.planDynamicActionsForDescriptor(cfg, descriptor, artifacts)
		if err != nil {
			return nil, nil, err
		}
		actions = append(actions, dynamicActions...)
	}

	for _, action := range actions {
		diag.Actions = append(diag.Actions, ActionDiagnostic{
			Owner:    action.Owner,
			Output:   action.Output,
			Rendered: action.Render,
		})
	}

	// Auto-generate actions for services without explicit descriptors.
	autoActions, err := planAutoServiceActionsWithArtifacts(cfg, registry, artifacts)
	if err != nil {
		return nil, nil, err
	}
	actions = append(actions, autoActions...)
	if err := validateClusterAppActions(actions, ""); err != nil {
		return nil, nil, fmt.Errorf("planned GitOps action output validation failed: %w", err)
	}

	for _, action := range autoActions {
		diag.Actions = append(diag.Actions, ActionDiagnostic{
			Owner:    action.Owner,
			Output:   action.Output,
			Rendered: action.Content != "",
		})
	}

	lastRenderDiagnostics = diag
	return actions, artifacts, nil
}

// descriptorEnableReason returns a human-readable reason for a descriptor's
// enabled/disabled state.
func descriptorEnableReason(d descriptorcfg.Descriptor, enabled bool) string {
	if d.Service != "" {
		if enabled {
			return fmt.Sprintf("service %q is enabled in config", d.Service)
		}
		return fmt.Sprintf("service %q is disabled, absent, or externally managed in config", d.Service)
	}
	if d.ManagedService != "" {
		if enabled {
			return fmt.Sprintf("managed service %q is enabled in config", d.ManagedService)
		}
		return fmt.Sprintf("managed service %q is disabled, absent, or externally managed in config", d.ManagedService)
	}
	if d.EnabledWhen != nil {
		if enabled {
			return fmt.Sprintf("condition %s %s %s evaluated to true", d.EnabledWhen.Field, d.EnabledWhen.Operator, d.EnabledWhen.Value)
		}
		return fmt.Sprintf("condition %s %s %s evaluated to false", d.EnabledWhen.Field, d.EnabledWhen.Operator, d.EnabledWhen.Value)
	}
	if enabled {
		return "unconditionally enabled (no condition)"
	}
	return "disabled (unknown reason)"
}

func planSingleServiceActionsWithArtifacts(cfg v2.Config, serviceName string, isManaged bool) ([]clusterAppAction, []secretartifacts.Artifact, error) {
	if err := validateOverlayUnitConfig(cfg); err != nil {
		return nil, nil, err
	}
	artifacts, err := secretartifacts.Plan(&cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("plan secret artifacts: %w", err)
	}

	registry, err := loadClusterDescriptorRegistry()
	if err != nil {
		return nil, nil, err
	}
	if err := validateDescriptorCoverage(registry); err != nil {
		return nil, nil, err
	}
	catalog := newBuiltInRenderCatalog()
	if err := catalog.ValidateAgainstDescriptors(registry); err != nil {
		return nil, nil, err
	}

	view, err := buildConfigView(cfg)
	if err != nil {
		return nil, nil, err
	}

	var target descriptorcfg.Descriptor
	found := false
	for _, descriptor := range registry.Descriptors() {
		if isManaged {
			if descriptor.ManagedService == serviceName {
				target = descriptor
				found = true
				break
			}
			continue
		}
		if descriptor.Service == serviceName {
			target = descriptor
			found = true
			break
		}
	}
	if !found {
		if isManaged {
			return nil, nil, fmt.Errorf("descriptor not found for managed service %q", serviceName)
		}
		if _, owned := catalog.Lookup(serviceName); !owned {
			return nil, nil, fmt.Errorf("service %q has neither an explicit descriptor nor a built-in render catalog entry", serviceName)
		}
		serviceCfg, exists := cfg.OpenCenter.Services[serviceName]
		if !exists || IsServiceDisabled(serviceCfg) || IsServiceExternal(serviceCfg) {
			return nil, nil, fmt.Errorf("catalog-owned service %q is disabled, absent, or externally managed", serviceName)
		}
		base := extractBaseConfig(serviceCfg)
		if base == nil {
			return nil, nil, fmt.Errorf("catalog-owned service %q does not expose declarative service configuration", serviceName)
		}
		ctx := buildAutoServiceContextWithArtifacts(serviceName, base, cfg, artifacts)
		actions, renderErr := renderAutoServiceActions(ctx, cfg)
		if renderErr != nil {
			return nil, nil, renderErr
		}
		actions = appendCustomSeedAction(actions, ctx)
		// Re-render the always-on aggregate descriptors so the top-level
		// aggregator kustomizations (services/fluxcd/kustomization.yaml and
		// services/sources/kustomization.yaml) are rebuilt from the full catalog +
		// enabled services. Explicit-descriptor services get this via their
		// AggregateTargets; catalog-owned auto services (including NamespaceStage
		// services such as sealed-secrets/velero/openstack-ccm/openstack-csi) have
		// no descriptor, so without this a scoped enable --render would add a new
		// top-level Kustomization (e.g. <svc>-namespace.yaml) without listing it in
		// the aggregator, leaving it stale.
		aggregateActions, aggErr := planAutoServiceAggregateActions(cfg, view, registry, artifacts, catalog)
		if aggErr != nil {
			return nil, nil, aggErr
		}
		actions = append(actions, aggregateActions...)
		if err := validateClusterAppActions(actions, ""); err != nil {
			return nil, nil, fmt.Errorf("planned GitOps action output validation failed: %w", err)
		}
		return actions, artifacts, nil
	}

	descriptorsToRender := []descriptorcfg.Descriptor{target}
	for _, aggregateName := range target.AggregateTargets {
		descriptor, ok := registry.Get(aggregateName)
		if !ok {
			return nil, nil, fmt.Errorf("aggregate descriptor %q not found", aggregateName)
		}
		descriptorsToRender = append(descriptorsToRender, descriptor)
	}

	var actions []clusterAppAction
	for _, descriptor := range descriptorsToRender {
		enabled, err := isDescriptorEnabled(cfg, view, descriptor)
		if err != nil {
			return nil, nil, fmt.Errorf("descriptor %s: %w", descriptor.Name, err)
		}
		if !enabled {
			continue
		}
		expanded, err := expandDescriptorActions(descriptor, cfg, view)
		if err != nil {
			return nil, nil, err
		}
		actions = append(actions, expanded...)
		dynamicActions, err := catalog.planDynamicActionsForDescriptor(cfg, descriptor, artifacts)
		if err != nil {
			return nil, nil, err
		}
		actions = append(actions, dynamicActions...)
	}
	if err := validateClusterAppActions(actions, ""); err != nil {
		return nil, nil, fmt.Errorf("planned GitOps action output validation failed: %w", err)
	}

	return actions, artifacts, nil
}

// autoServiceAggregateDescriptors are the always-on aggregate descriptors that
// own the top-level aggregator kustomizations. Catalog-owned (auto) services have
// no explicit descriptor and therefore no AggregateTargets, so a scoped
// enable --render must expand these explicitly to keep the aggregator current.
var autoServiceAggregateDescriptors = []string{"services-fluxcd-aggregate", "services-sources-aggregate"}

// planAutoServiceAggregateActions expands the always-on aggregate descriptors so
// the aggregator kustomizations are re-derived from the full catalog + enabled
// services, matching the behavior of a full cluster generate.
func planAutoServiceAggregateActions(cfg v2.Config, view map[string]any, registry *descriptorcfg.Registry, artifacts []secretartifacts.Artifact, catalog RenderCatalog) ([]clusterAppAction, error) {
	var actions []clusterAppAction
	for _, name := range autoServiceAggregateDescriptors {
		descriptor, ok := registry.Get(name)
		if !ok {
			// Aggregate descriptor is optional; skip if the registry doesn't define it.
			continue
		}
		enabled, err := isDescriptorEnabled(cfg, view, descriptor)
		if err != nil {
			return nil, fmt.Errorf("descriptor %s: %w", descriptor.Name, err)
		}
		if !enabled {
			continue
		}
		expanded, err := expandDescriptorActions(descriptor, cfg, view)
		if err != nil {
			return nil, err
		}
		actions = append(actions, expanded...)
		dynamicActions, err := catalog.planDynamicActionsForDescriptor(cfg, descriptor, artifacts)
		if err != nil {
			return nil, err
		}
		actions = append(actions, dynamicActions...)
	}
	return actions, nil
}

func writeClusterAppActions(actions []clusterAppAction, target string, cfg v2.Config, workspace *GitOpsWorkspace) error {
	if err := validateClusterAppActions(actions, target); err != nil {
		return err
	}
	for _, action := range actions {
		dst := filepath.Join(target, action.Output)

		// Auto-descriptor actions provide pre-rendered content directly.
		if action.Content != "" {
			relPath, err := filepath.Rel(workspace.RootDir, dst)
			if err != nil {
				return fmt.Errorf("relative path for %s: %w", action.Output, err)
			}
			writer := NewAtomicWriter(workspace)
			if err := writer.WriteFileString(relPath, action.Content, 0o644); err != nil {
				return err
			}
			continue
		}

		if action.Render {
			if err := renderTemplateAtomic(action.Template, dst, cfg, workspace); err != nil {
				return err
			}
			continue
		}
		if err := copyFileAtomic(action.Template, dst, workspace); err != nil {
			return err
		}
	}
	return nil
}

func validateMaterializedSecretMembership(cfg v2.Config, actions []clusterAppAction, artifacts []secretartifacts.Artifact, renderedOverlay string) error {
	ownershipOverlay := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName())
	state, _, err := secretartifacts.LoadOwnershipState(ownershipOverlay)
	if err != nil {
		return err
	}
	records := state.ByPath()
	for _, artifact := range artifacts {
		record, ok := records[artifact.Path]
		if !ok {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(ownershipOverlay, filepath.FromSlash(artifact.Path)))
		if readErr != nil || secretartifacts.HashBytes(data) != record.Hash {
			continue
		}

		output := filepath.ToSlash(filepath.Join(filepath.Dir(artifact.Path), "kustomization.yaml"))
		found := false
		for _, action := range actions {
			if filepath.ToSlash(action.Output) == output {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("secret artifact %q is materialized but service %q has no renderable overlay kustomization", artifact.Path, artifact.TargetService)
		}

		kustomizationPath := filepath.Join(renderedOverlay, filepath.FromSlash(output))
		content, readErr := os.ReadFile(kustomizationPath)
		if readErr != nil {
			return fmt.Errorf("secret artifact %q is materialized but cannot read final kustomization for service %q: %w", artifact.Path, artifact.TargetService, readErr)
		}
		var kustomization struct {
			Resources []string `yaml:"resources"`
		}
		if err := yaml.Unmarshal(content, &kustomization); err != nil {
			return fmt.Errorf("secret artifact %q is materialized but final kustomization for service %q is invalid: %w", artifact.Path, artifact.TargetService, err)
		}
		included := false
		for _, resource := range kustomization.Resources {
			resource = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(resource)), "./")
			if resource == "secret.yaml" {
				included = true
				break
			}
		}
		if !included {
			return fmt.Errorf("secret artifact %q is materialized but final kustomization for service %q does not include secret.yaml in resources", artifact.Path, artifact.TargetService)
		}
	}
	return nil
}
