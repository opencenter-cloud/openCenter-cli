package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	descriptorcfg "github.com/opencenter-cloud/opencenter-cli/internal/services/descriptors"
)

type clusterAppAction struct {
	Owner    string
	Template string
	Output   string
	Render   bool
}

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

func resolveClusterAppsTarget(workspace *GitOpsWorkspace, cfg config.Config) (string, error) {
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

func renderOutputPath(path string, cfg config.Config) (string, error) {
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

func buildConfigView(cfg config.Config) (map[string]any, error) {
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
	default:
		return false, fmt.Errorf("unsupported descriptor operator %q", condition.Operator)
	}
}

func isDescriptorEnabled(cfg config.Config, view map[string]any, descriptor descriptorcfg.Descriptor) (bool, error) {
	if descriptor.Service != "" {
		service, exists := cfg.OpenCenter.Services[descriptor.Service]
		return exists && !IsServiceDisabled(service), nil
	}
	if descriptor.ManagedService != "" {
		service, exists := cfg.OpenCenter.ManagedService[descriptor.ManagedService]
		return exists && !IsServiceDisabled(service), nil
	}
	return evaluateDescriptorCondition(view, descriptor.EnabledWhen)
}

func expandDescriptorActions(descriptor descriptorcfg.Descriptor, cfg config.Config, view map[string]any) ([]clusterAppAction, error) {
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

func planClusterAppActions(cfg config.Config) ([]clusterAppAction, error) {
	if err := validateOverlayUnitConfig(cfg); err != nil {
		return nil, err
	}

	registry, err := loadClusterDescriptorRegistry()
	if err != nil {
		return nil, err
	}
	if err := validateDescriptorCoverage(registry); err != nil {
		return nil, err
	}

	view, err := buildConfigView(cfg)
	if err != nil {
		return nil, err
	}

	var actions []clusterAppAction
	for _, descriptor := range registry.Descriptors() {
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
	}

	return actions, nil
}

func planSingleServiceActions(cfg config.Config, serviceName string, isManaged bool) ([]clusterAppAction, error) {
	if err := validateOverlayUnitConfig(cfg); err != nil {
		return nil, err
	}

	registry, err := loadClusterDescriptorRegistry()
	if err != nil {
		return nil, err
	}
	if err := validateDescriptorCoverage(registry); err != nil {
		return nil, err
	}

	view, err := buildConfigView(cfg)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("descriptor not found for service %q", serviceName)
	}

	descriptorsToRender := []descriptorcfg.Descriptor{target}
	for _, aggregateName := range target.AggregateTargets {
		descriptor, ok := registry.Get(aggregateName)
		if !ok {
			return nil, fmt.Errorf("aggregate descriptor %q not found", aggregateName)
		}
		descriptorsToRender = append(descriptorsToRender, descriptor)
	}

	var actions []clusterAppAction
	for _, descriptor := range descriptorsToRender {
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
	}

	return actions, nil
}

func writeClusterAppActions(actions []clusterAppAction, target string, cfg config.Config, workspace *GitOpsWorkspace) error {
	for _, action := range actions {
		dst := filepath.Join(target, action.Output)
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

func cleanupRendererOwnedOverlay(target string) error {
	ownedPaths := []string{
		filepath.Join(target, "services"),
		filepath.Join(target, "managed-services"),
		filepath.Join(target, "customer-managed"),
		filepath.Join(target, "kustomization.yaml"),
		filepath.Join(target, ".sops.yaml"),
	}

	for _, path := range ownedPaths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("cleanup renderer-owned path %s: %w", path, err)
		}
	}

	return nil
}

func cleanupSingleServiceOutputs(target string, serviceName string, isManaged bool, actions []clusterAppAction) error {
	dirPrefix := "services"
	if isManaged {
		dirPrefix = "managed-services"
	}

	targetDir := filepath.Join(target, dirPrefix, serviceName)
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("cleanup service directory %s: %w", targetDir, err)
	}

	removedFiles := make(map[string]struct{})
	for _, action := range actions {
		outputPath := filepath.Clean(filepath.Join(target, action.Output))
		rel := filepath.Clean(action.Output)
		if strings.HasPrefix(rel, dirPrefix+string(filepath.Separator)+serviceName+string(filepath.Separator)) {
			continue
		}
		if _, seen := removedFiles[outputPath]; seen {
			continue
		}
		if err := os.RemoveAll(outputPath); err != nil {
			return fmt.Errorf("cleanup aggregate output %s: %w", outputPath, err)
		}
		removedFiles[outputPath] = struct{}{}
	}

	return nil
}

func isBoolValue(value any) bool {
	rv := reflect.ValueOf(value)
	return rv.IsValid() && rv.Kind() == reflect.Bool
}
