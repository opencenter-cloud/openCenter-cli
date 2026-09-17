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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	descriptorcfg "github.com/opencenter-cloud/opencenter-cli/internal/services/descriptors"
)

// autoServiceContext holds the data passed to generic service templates.
type autoServiceContext struct {
	ServiceName            string
	Namespace              string
	SourceName             string
	EmitSource             bool
	BasePath               string
	SingleStage            bool
	BaseOnly               bool
	PostBaseStages         []postBaseStageSpec
	OmitTargetNamespace    bool
	PrivilegedNamespace    bool
	KustomizationName      string
	HasOverrideValues      bool
	NamespaceStage         bool
	EnterpriseRegistry     bool
	GeneratedResourceFiles []string
	ExtraDependencies      []string
	OverrideDependsOn      []string
	OverrideValues         string
	OverrideValuesRenderer OverrideValuesRenderer
	KustomizationContent   string
	OverlayFilesRenderer   OverlayFilesRenderer
	ClusterName            string
	BaseRepoURL            string
	RepoBranch             string
	RepoTag                string
	GitopsAuthMethod       string
	FluxInterval           string
	Force                  bool
	Suspend                bool
	// PostBuildSubstituteFrom is a pre-rendered YAML block appended under a Flux
	// Kustomization's spec when hostname substitution is enabled (OCTR-759).
	// Empty when disabled, so output is unchanged.
	PostBuildSubstituteFrom string
}

// planAutoServiceActions generates render actions for enabled services that lack
// an explicit descriptor in the registry. Uses BaseConfig rendering fields.
func planAutoServiceActions(cfg v2.Config, registry *descriptorcfg.Registry) ([]clusterAppAction, error) {
	artifacts, err := secretartifacts.Plan(&cfg)
	if err != nil {
		return nil, err
	}
	return planAutoServiceActionsWithArtifacts(cfg, registry, artifacts)
}

func planAutoServiceActionsWithArtifacts(cfg v2.Config, registry *descriptorcfg.Registry, artifacts []secretartifacts.Artifact) ([]clusterAppAction, error) {
	catalog := newBuiltInRenderCatalog()
	if err := catalog.ValidateConfigOwnership(cfg, registry); err != nil {
		return nil, err
	}

	var actions []clusterAppAction
	for serviceName, serviceCfg := range cfg.OpenCenter.Services {
		if IsServiceDisabled(serviceCfg) || IsServiceExternal(serviceCfg) || hasExplicitDescriptor(registry, serviceName, serviceKindStandard) {
			continue
		}
		if _, owned := catalog.Lookup(serviceName); !owned {
			return nil, fmt.Errorf("enabled service %q has neither an explicit descriptor nor a built-in render catalog entry", serviceName)
		}

		base := extractBaseConfig(serviceCfg)
		if base == nil {
			return nil, fmt.Errorf("catalog-owned service %q does not expose declarative service configuration", serviceName)
		}

		ctx := buildAutoServiceContextWithArtifacts(serviceName, base, cfg, artifacts)
		svcActions, err := renderAutoServiceActions(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("auto-render service %s: %w", serviceName, err)
		}
		actions = append(actions, appendCustomSeedAction(svcActions, ctx)...)
	}

	return actions, nil
}

type serviceKind string

const (
	serviceKindStandard serviceKind = "service"
	serviceKindManaged  serviceKind = "managed_service"
)

// hasExplicitDescriptor reports whether a descriptor explicitly owns the named
// service in the requested config map. Keeping the kind explicit prevents a
// managed service from being mistaken for a regular service (or vice versa).
func hasExplicitDescriptor(registry *descriptorcfg.Registry, serviceName string, kind serviceKind) bool {
	// Structural services that are handled by aggregate descriptors, not auto-descriptors.
	if kind == serviceKindStandard {
		switch serviceName {
		case "fluxcd", "sources":
			return true
		}
	}
	for _, d := range registry.Descriptors() {
		switch kind {
		case serviceKindStandard:
			if d.Service == serviceName {
				return true
			}
		case serviceKindManaged:
			if d.ManagedService == serviceName {
				return true
			}
		}
	}
	return false
}

// extractBaseConfig extracts the BaseConfig from a service config using reflection.
func extractBaseConfig(serviceCfg any) *services.BaseConfig {
	val := reflect.ValueOf(serviceCfg)
	for val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	baseField := val.FieldByName("BaseConfig")
	if !baseField.IsValid() {
		return nil
	}
	if base, ok := baseField.Addr().Interface().(*services.BaseConfig); ok {
		return base
	}
	return nil
}

// buildAutoServiceContext populates the template context from config.
func buildAutoServiceContext(serviceName string, base *services.BaseConfig, cfg v2.Config) autoServiceContext {
	return buildAutoServiceContextWithArtifacts(serviceName, base, cfg, nil)
}

func buildAutoServiceContextWithArtifacts(serviceName string, base *services.BaseConfig, cfg v2.Config, artifacts []secretartifacts.Artifact) autoServiceContext {
	catalog := newBuiltInRenderCatalog()
	spec, _ := catalog.Lookup(serviceName)

	baseRepoURL := cfg.OpenCenter.GitOps.BaseRepo.URL
	if strings.TrimSpace(base.Source.Repo) != "" {
		baseRepoURL = strings.TrimSpace(base.Source.Repo)
	}

	// A service-level ref takes precedence over the shared base-repository ref.
	// Otherwise use a pinned base release before branch tracking.
	branch := strings.TrimSpace(base.Source.Branch)
	release := strings.TrimSpace(base.Source.Release)
	if release == "" && branch == "" {
		release = cfg.OpenCenter.GitOps.BaseRepo.Release
		branch = cfg.OpenCenter.GitOps.BaseRepo.Branch
	}

	var repoTag string
	if release != "" {
		repoTag = release
		branch = ""
	} else {
		if branch == "" {
			branch = cfg.OpenCenter.GitOps.Repository.Branch
		}
		if branch == "" {
			branch = "main"
		}
	}

	interval := cfg.OpenCenter.GitOps.Flux.Interval
	if interval == "" {
		interval = "15m"
	}

	adoption := GetAdoptionSettings(AdoptionMode(base.GetAdoptionMode()))
	extraDeps := append([]string{}, spec.ExtraDependencies...)
	for _, cd := range spec.ConditionalDependencies {
		if svc, exists := cfg.OpenCenter.Services[cd.WhenEnabled]; exists && !IsServiceDisabled(svc) {
			extraDeps = append(extraDeps, cd.Name)
		}
	}

	generatedResourceFiles := append([]string{}, spec.GeneratedResourceFiles...)
	if secretArtifactTargetMaterialized(cfg, serviceName, artifacts) && !containsString(generatedResourceFiles, "secret.yaml") {
		generatedResourceFiles = append(generatedResourceFiles, "secret.yaml")
	}
	namespace := base.Namespace
	if namespace == "" {
		namespace = spec.DefaultNamespace
	}

	return autoServiceContext{
		ServiceName:            serviceName,
		Namespace:              namespace,
		SourceName:             spec.SourceName,
		BasePath:               spec.BasePath,
		EmitSource:             spec.EmitSource,
		SingleStage:            spec.SingleStage,
		BaseOnly:               spec.BaseOnly,
		PostBaseStages:         append([]postBaseStageSpec{}, spec.PostBaseStages...),
		OmitTargetNamespace:    spec.OmitTargetNamespace,
		PrivilegedNamespace:    spec.PrivilegedNamespace,
		KustomizationName:      kustomizationName(serviceName, spec.KustomizationName),
		HasOverrideValues:      spec.HasOverrideValues,
		NamespaceStage:         spec.NamespaceStage,
		EnterpriseRegistry:     spec.EnterpriseRegistry,
		GeneratedResourceFiles: generatedResourceFiles,
		ExtraDependencies:      extraDeps,
		OverrideDependsOn:      append([]string{}, spec.OverrideDependsOn...),
		OverrideValues:         spec.OverrideValues,
		OverrideValuesRenderer: spec.OverrideValuesRenderer,
		KustomizationContent:   spec.KustomizationContent,
		OverlayFilesRenderer:   spec.OverlayFilesRenderer,
		ClusterName:            cfg.ClusterName(),
		BaseRepoURL:            baseRepoURL,
		RepoBranch:             branch,
		RepoTag:                repoTag,
		GitopsAuthMethod:       cfg.OpenCenter.GitOps.ResolvedAuthMethod,
		FluxInterval:           interval,
		Force:                  adoption.Force,
		Suspend:                adoption.Suspend,
		PostBuildSubstituteFrom: postBuildSubstituteFromBlock(cfg),
	}
}

// postBuildSubstituteFromLines adapts the substituteFrom block for the
// fmt-based postBase renderer: it appends a trailing newline so the block sits
// on its own lines, or returns "" (no lines) when substitution is disabled.
func postBuildSubstituteFromLines(block string) string {
	if block == "" {
		return ""
	}
	return block + "\n"
}

// postBuildSubstituteFromBlock renders the Flux spec.postBuild.substituteFrom
// block referencing the cluster-vars ConfigMap, or "" when hostname
// substitution is disabled (OCTR-759). The trailing newline is omitted so the
// block slots cleanly under a Kustomization spec.
func postBuildSubstituteFromBlock(cfg v2.Config) string {
	hs := cfg.OpenCenter.Cluster.HostnameSubstitution
	if !hs.Enabled {
		return ""
	}
	return fmt.Sprintf(`  postBuild:
    substituteFrom:
      - kind: ConfigMap
        name: %s`, hs.GetConfigMapName())
}

func secretArtifactTargetMaterialized(cfg v2.Config, serviceName string, artifacts []secretartifacts.Artifact) bool {
	overlay := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName())
	state, _, err := secretartifacts.LoadOwnershipState(overlay)
	if err != nil {
		return false
	}
	records := state.ByPath()
	for _, artifact := range artifacts {
		if artifact.TargetService != serviceName {
			continue
		}
		record, ok := records[artifact.Path]
		if !ok {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(artifact.Path)))
		if readErr == nil && secretartifacts.HashBytes(data) == record.Hash {
			return true
		}
	}
	return false
}

// renderAutoServiceActions renders all files for an auto-generated service.
func renderAutoServiceActions(ctx autoServiceContext, cfg v2.Config) ([]clusterAppAction, error) {
	var actions []clusterAppAction

	// 1. Source file (skip if shared source owned by another service)
	if ctx.EmitSource || ctx.SourceName == "opencenter-"+ctx.ServiceName {
		content, err := renderInlineAutoTemplate(autoSourceTemplate, ctx)
		if err != nil {
			return nil, fmt.Errorf("source: %w", err)
		}
		actions = append(actions, clusterAppAction{
			Owner:   "auto-service-" + ctx.ServiceName,
			Output:  fmt.Sprintf("services/sources/%s.yaml", ctx.SourceName),
			Content: content,
		})
	}

	// 2. FluxCD Kustomization
	var fluxTmpl string
	switch {
	case ctx.SingleStage:
		fluxTmpl = autoFluxSingleStageTemplate
	case ctx.BaseOnly:
		fluxTmpl = autoFluxBaseOnlyTemplate
	default:
		fluxTmpl = autoFluxTwoStageTemplate
	}
	content, err := renderInlineAutoTemplate(fluxTmpl, ctx)
	if err != nil {
		return nil, fmt.Errorf("fluxcd: %w", err)
	}
	for _, stage := range ctx.PostBaseStages {
		stageContent, err := renderPostBaseFluxKustomization(ctx, stage)
		if err != nil {
			return nil, fmt.Errorf("post-base stage %q: %w", stage.Name, err)
		}
		content += stageContent
	}
	actions = append(actions, clusterAppAction{
		Owner:   "auto-service-" + ctx.ServiceName,
		Output:  fmt.Sprintf("services/fluxcd/%s.yaml", ctx.ServiceName),
		Content: content,
	})

	if ctx.NamespaceStage {
		namespaceStage, err := renderInlineAutoTemplate(autoFluxNamespaceStageTemplate, ctx)
		if err != nil {
			return nil, fmt.Errorf("namespace stage: %w", err)
		}
		namespaceKustomization, err := renderInlineAutoTemplate(autoNamespaceKustomizationTemplate, ctx)
		if err != nil {
			return nil, fmt.Errorf("namespace kustomization: %w", err)
		}
		namespaceResource, err := renderInlineAutoTemplate(autoNamespaceResourceTemplate, ctx)
		if err != nil {
			return nil, fmt.Errorf("namespace resource: %w", err)
		}
		actions = append(actions,
			clusterAppAction{
				Owner:   "auto-service-" + ctx.ServiceName,
				Output:  fmt.Sprintf("services/fluxcd/%s-namespace.yaml", ctx.ServiceName),
				Content: namespaceStage,
			},
			clusterAppAction{
				Owner:   "auto-service-" + ctx.ServiceName,
				Output:  fmt.Sprintf("services/%s/namespace/kustomization.yaml", ctx.ServiceName),
				Content: namespaceKustomization,
			},
			clusterAppAction{
				Owner:   "auto-service-" + ctx.ServiceName,
				Output:  fmt.Sprintf("services/%s/namespace/namespace.yaml", ctx.ServiceName),
				Content: namespaceResource,
			},
		)
	}

	// BaseOnly services have no overlay directory — skip kustomization and override-values.
	if ctx.BaseOnly {
		return actions, nil
	}

	// Generated overlay kustomizations include the user-owned custom layer.
	// BaseOnly services have no overlay, and verbatim KustomizationContent services
	// must remain untouched.
	resources := append([]string{}, ctx.GeneratedResourceFiles...)
	if ctx.KustomizationContent == "" && !containsString(resources, CustomDirName+"/") {
		resources = append(resources, CustomDirName+"/")
	}
	var overlayFiles map[string]string
	if ctx.OverlayFilesRenderer != nil {
		overlayFiles, err = ctx.OverlayFilesRenderer(cfg)
		if err != nil {
			return nil, fmt.Errorf("overlay-files renderer for %q: %w", ctx.ServiceName, err)
		}
		for filename := range overlayFiles {
			if !containsString(resources, filename) {
				resources = append(resources, filename)
			}
		}
	}
	sort.Strings(resources)
	ctx.GeneratedResourceFiles = resources

	// 4. Service overlay kustomization.yaml
	if ctx.KustomizationContent != "" {
		actions = append(actions, clusterAppAction{
			Owner:   "auto-service-" + ctx.ServiceName,
			Output:  fmt.Sprintf("services/%s/kustomization.yaml", ctx.ServiceName),
			Content: ctx.KustomizationContent,
		})
	} else {
		content, err = renderInlineAutoTemplate(autoKustomizationTemplate, ctx)
		if err != nil {
			return nil, fmt.Errorf("kustomization: %w", err)
		}
		actions = append(actions, clusterAppAction{
			Owner:   "auto-service-" + ctx.ServiceName,
			Output:  fmt.Sprintf("services/%s/kustomization.yaml", ctx.ServiceName),
			Content: content,
		})
	}

	// 5. Override values
	if ctx.HasOverrideValues {
		overrideContent := "---\n...\n"
		if ctx.OverrideValuesRenderer != nil {
			rendered, err := ctx.OverrideValuesRenderer(cfg)
			if err != nil {
				return nil, fmt.Errorf("override-values renderer for %q: %w", ctx.ServiceName, err)
			}
			overrideContent = rendered
		} else if ctx.OverrideValues != "" {
			overrideContent = ctx.OverrideValues
		}
		actions = append(actions, clusterAppAction{
			Owner:   "auto-service-" + ctx.ServiceName,
			Output:  fmt.Sprintf("services/%s/helm-values/override-values.yaml", ctx.ServiceName),
			Content: overrideContent,
		})
	}

	for _, filename := range sortedOverlayFileKeys(overlayFiles) {
		actions = append(actions, clusterAppAction{
			Owner:   "auto-service-" + ctx.ServiceName,
			Output:  fmt.Sprintf("services/%s/%s", ctx.ServiceName, filename),
			Content: overlayFiles[filename],
		})
	}

	return actions, nil
}

func appendCustomSeedAction(actions []clusterAppAction, ctx autoServiceContext) []clusterAppAction {
	if ctx.BaseOnly || ctx.KustomizationContent != "" {
		return actions
	}
	return append(actions, clusterAppAction{
		Owner:   "auto-service-" + ctx.ServiceName,
		Output:  fmt.Sprintf("services/%s/%s/kustomization.yaml", ctx.ServiceName, CustomDirName),
		Content: customKustomizationSeed,
	})
}

const customKustomizationSeed = `# Files placed in this directory are owned by you, not by openCenter.
# "opencenter cluster generate" will never modify or delete them.
# Add files to this directory to the resources list below to have Kustomize apply them.
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: []
`

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortedOverlayFileKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func kustomizationName(serviceName, override string) string {
	if override != "" {
		return override
	}
	return serviceName
}

func renderPostBaseFluxKustomization(ctx autoServiceContext, stage postBaseStageSpec) (string, error) {
	var buf strings.Builder
	fmt.Fprintf(&buf, `---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %s
  namespace: flux-system
spec:
  dependsOn:
`, stage.Name)
	for _, dependency := range stage.DependsOn {
		fmt.Fprintf(&buf, `    - name: %s
      namespace: flux-system
`, dependency)
	}
	fmt.Fprintf(&buf, `  interval: %s
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: %s
    namespace: flux-system
  path: %s
  targetNamespace: %s
  prune: true
  wait: true
  force: %t
  suspend: %t
%s  commonMetadata:
    labels:
      app.kubernetes.io/part-of: %s
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
`, ctx.FluxInterval, ctx.SourceName, stage.Path, ctx.Namespace, ctx.Force, ctx.Suspend, postBuildSubstituteFromLines(ctx.PostBuildSubstituteFrom), ctx.ServiceName)
	return buf.String(), nil
}

func renderInlineAutoTemplate(tmplStr string, ctx autoServiceContext) (string, error) {
	funcMap := sprig.TxtFuncMap()
	// Add the shared source auth block renderer. Returning an error from a
	// template function makes invalid repository URLs fail rendering clearly.
	funcMap["sourceAuthBlock"] = func() (string, error) {
		refType := "branch"
		refValue := ctx.RepoBranch
		if ctx.RepoTag != "" {
			refType = "tag"
			refValue = ctx.RepoTag
		}
		// The base repository (ctx.BaseRepoURL) is the public
		// openCenter-gitops-base repo, so its source is anonymous: no
		// secretRef, avoiding a wrong-credential HTTP 401 against GitHub.
		params, err := BuildSourceAuthParams(ctx.GitopsAuthMethod, ctx.BaseRepoURL, refType, refValue, "")
		if err != nil {
			return "", err
		}
		params.Anonymous = true
		return RenderSourceAuthBlock(params), nil
	}
	t, err := template.New("auto").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// --- Generic Templates ---

const autoSourceTemplate = `---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: {{ .SourceName }}
  namespace: flux-system
spec:
  interval: 15m
{{ sourceAuthBlock }}
`

const autoFluxTwoStageTemplate = `{{- $kn := .KustomizationName | default .ServiceName -}}
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: {{ $kn }}-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
{{- range .ExtraDependencies }}
    - name: {{ . }}
      namespace: flux-system
{{- end }}
  interval: {{ .FluxInterval }}
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: {{ .SourceName }}
    namespace: flux-system
  path: {{ .BasePath }}
  targetNamespace: {{ .Namespace }}
  prune: true
  wait: true
  force: {{ .Force }}
  suspend: {{ .Suspend }}
{{- if .PostBuildSubstituteFrom }}
{{ .PostBuildSubstituteFrom }}
{{- end }}
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: {{ .ServiceName }}
      namespace: {{ .Namespace }}
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: {{ .ServiceName }}
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: {{ $kn }}-override
  namespace: flux-system
spec:
  dependsOn:
{{- if .OverrideDependsOn }}
{{- range .OverrideDependsOn }}
    - name: {{ . }}
      namespace: flux-system
{{- end }}
{{- else }}
    - name: {{ $kn }}-base
      namespace: flux-system
{{- end }}
  interval: {{ .FluxInterval }}
  retryInterval: 1m
  timeout: 10m
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .ClusterName }}/services/{{ .ServiceName }}
  targetNamespace: {{ .Namespace }}
  prune: true
  wait: true
  force: {{ .Force }}
  suspend: {{ .Suspend }}
{{- if .PostBuildSubstituteFrom }}
{{ .PostBuildSubstituteFrom }}
{{- end }}
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: {{ .ServiceName }}
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
`

const autoFluxBaseOnlyTemplate = `{{- $kn := .KustomizationName | default .ServiceName -}}
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: {{ $kn }}-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
{{- range .ExtraDependencies }}
    - name: {{ . }}
      namespace: flux-system
{{- end }}
  interval: {{ .FluxInterval }}
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: {{ .SourceName }}
    namespace: flux-system
  path: {{ .BasePath }}
{{- if not .OmitTargetNamespace }}
  targetNamespace: {{ .Namespace }}
{{- end }}
  prune: true
  wait: true
  force: {{ .Force }}
  suspend: {{ .Suspend }}
{{- if .PostBuildSubstituteFrom }}
{{ .PostBuildSubstituteFrom }}
{{- end }}
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: {{ .ServiceName }}
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
`

const autoFluxSingleStageTemplate = `---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: {{ .ServiceName }}
  namespace: flux-system
spec:
{{- if .ExtraDependencies }}
  dependsOn:
{{- range .ExtraDependencies }}
    - name: {{ . }}
      namespace: flux-system
{{- end }}
{{- end }}
  interval: {{ .FluxInterval }}
  retryInterval: 1m
  timeout: 10m
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .ClusterName }}/services/{{ .ServiceName }}
  prune: true
  wait: true
{{- if .PostBuildSubstituteFrom }}
{{ .PostBuildSubstituteFrom }}
{{- end }}
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: {{ .ServiceName }}
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
`

const autoKustomizationTemplate = `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: {{ .Namespace }}
{{- if .HasOverrideValues }}
secretGenerator:
  - name: {{ .ServiceName }}-values-override
    type: Opaque
    files:
      - override.yaml=helm-values/override-values.yaml
    options:
      disableNameSuffixHash: true
{{- end }}
{{- if or .GeneratedResourceFiles .EnterpriseRegistry }}
resources:
{{- range .GeneratedResourceFiles }}
  - {{ . }}
{{- end }}
{{- if .EnterpriseRegistry }}
  - "../global/rackspace-registry/"
{{- end }}
{{- end }}
`

const autoFluxNamespaceStageTemplate = `---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: {{ .ServiceName }}-namespace
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: {{ .FluxInterval }}
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .ClusterName }}/services/{{ .ServiceName }}/namespace
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: {{ .ServiceName }}
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
`

const autoNamespaceKustomizationTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - namespace.yaml
`

const autoNamespaceResourceTemplate = `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Namespace }}
{{- if .PrivilegedNamespace }}
  labels:
    pod-security.kubernetes.io/enforce: privileged
{{- end }}
`
