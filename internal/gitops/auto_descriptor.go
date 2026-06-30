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
	"reflect"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	descriptorcfg "github.com/opencenter-cloud/opencenter-cli/internal/services/descriptors"
)

// autoServiceContext holds the data passed to generic service templates.
type autoServiceContext struct {
	ServiceName             string
	Namespace               string
	SourceName              string
	BasePath                string
	Edition                 string
	SingleStage             bool
	BaseOnly                bool
	KustomizationName       string
	HasOverrideValues       bool
	EnterpriseRegistry      bool
	CustomResources         []string
	ExtraDependencies       []string
	OverrideDependsOn       []string
	OverrideValues          string
	OverrideValuesRendererKey string
	KustomizationContent      string
	OverlayFilesRendererKey   string
	ClusterName             string
	BaseRepoURL             string
	RepoBranch              string
	IsSSH                   bool
	FluxInterval            string
	Force                   bool
	Suspend                 bool
}

// planAutoServiceActions generates render actions for enabled services that lack
// an explicit descriptor in the registry. Uses BaseConfig rendering fields.
func planAutoServiceActions(cfg v2.Config, registry *descriptorcfg.Registry) ([]clusterAppAction, error) {
	var actions []clusterAppAction

	for serviceName, serviceCfg := range cfg.OpenCenter.Services {
		if IsServiceDisabled(serviceCfg) || IsServiceExternal(serviceCfg) {
			continue
		}
		if hasExplicitDescriptor(registry, serviceName) {
			continue
		}

		base := extractBaseConfig(serviceCfg)
		if base == nil {
			continue
		}

		ctx := buildAutoServiceContext(serviceName, base, cfg)
		svcActions, err := renderAutoServiceActions(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("auto-render service %s: %w", serviceName, err)
		}
		actions = append(actions, svcActions...)
	}

	return actions, nil
}

// hasExplicitDescriptor checks if the registry already has a descriptor for this service.
func hasExplicitDescriptor(registry *descriptorcfg.Registry, serviceName string) bool {
	// Structural services that are handled by aggregate descriptors, not auto-descriptors.
	switch serviceName {
	case "fluxcd", "sources":
		return true
	}
	for _, d := range registry.Descriptors() {
		if d.Service == serviceName {
			return true
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
	baseRepoURL := cfg.OpenCenter.GitOps.BaseRepo.URL
	branch := cfg.OpenCenter.GitOps.BaseRepo.Branch
	if branch == "" {
		branch = cfg.OpenCenter.GitOps.Repository.Branch
	}
	if branch == "" {
		branch = "main"
	}

	interval := cfg.OpenCenter.GitOps.Flux.Interval
	if interval == "" {
		interval = "15m"
	}

	adoption := GetAdoptionSettings(AdoptionMode(base.GetAdoptionMode()))

	// Resolve conditional dependencies: include only when the gate service is enabled.
	extraDeps := append([]string{}, base.ExtraDependencies...)
	for _, cd := range base.ConditionalDependencies {
		if svc, exists := cfg.OpenCenter.Services[cd.WhenEnabled]; exists && !IsServiceDisabled(svc) {
			extraDeps = append(extraDeps, cd.Name)
		}
	}

	return autoServiceContext{
		ServiceName:               serviceName,
		Namespace:                 base.Namespace,
		SourceName:                base.GetSourceName(serviceName),
		BasePath:                  base.GetBasePath(serviceName),
		Edition:                   base.Edition,
		SingleStage:               base.SingleStage,
		BaseOnly:                  base.BaseOnly,
		KustomizationName:         kustomizationName(serviceName, base.KustomizationName),
		HasOverrideValues:         base.GetHasOverrideValues(),
		EnterpriseRegistry:        base.EnterpriseRegistry,
		CustomResources:           base.CustomResources,
		ExtraDependencies:         extraDeps,
		OverrideDependsOn:         base.OverrideDependsOn,
		OverrideValues:            base.OverrideValues,
		OverrideValuesRendererKey: base.OverrideValuesRendererKey,
		KustomizationContent:      base.KustomizationContent,
		OverlayFilesRendererKey:   base.OverlayFilesRendererKey,
		ClusterName:               cfg.ClusterName(),
		BaseRepoURL:               baseRepoURL,
		RepoBranch:                branch,
		IsSSH:                     !strings.HasPrefix(baseRepoURL, "https://"),
		FluxInterval:              interval,
		Force:                     adoption.Force,
		Suspend:                   adoption.Suspend,
	}
}

// renderAutoServiceActions renders all files for an auto-generated service.
func renderAutoServiceActions(ctx autoServiceContext, cfg v2.Config) ([]clusterAppAction, error) {
	var actions []clusterAppAction

	// 1. Source file (skip if shared source owned by another service)
	if ctx.SourceName == "opencenter-"+ctx.ServiceName {
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
	actions = append(actions, clusterAppAction{
		Owner:   "auto-service-" + ctx.ServiceName,
		Output:  fmt.Sprintf("services/fluxcd/%s.yaml", ctx.ServiceName),
		Content: content,
	})

	// BaseOnly services have no overlay directory — skip kustomization and override-values.
	if ctx.BaseOnly {
		return actions, nil
	}

	// 3. Service overlay kustomization.yaml
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

	// 4. Override values
	if ctx.HasOverrideValues {
		overrideContent := "---\n...\n"
		if ctx.OverrideValuesRendererKey != "" {
			renderer, err := getOverrideValuesRenderer(ctx.OverrideValuesRendererKey)
			if err != nil {
				return nil, fmt.Errorf("override-values: %w", err)
			}
			rendered, err := renderer(cfg)
			if err != nil {
				return nil, fmt.Errorf("override-values renderer %q: %w", ctx.OverrideValuesRendererKey, err)
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

	// 5. Dynamic overlay files (templated content like gateway resources, HTTPRoutes)
	if ctx.OverlayFilesRendererKey != "" {
		renderer, err := getOverlayFilesRenderer(ctx.OverlayFilesRendererKey)
		if err != nil {
			return nil, fmt.Errorf("overlay-files: %w", err)
		}
		files, err := renderer(cfg)
		if err != nil {
			return nil, fmt.Errorf("overlay-files renderer %q: %w", ctx.OverlayFilesRendererKey, err)
		}
		for filename, fileContent := range files {
			actions = append(actions, clusterAppAction{
				Owner:   "auto-service-" + ctx.ServiceName,
				Output:  fmt.Sprintf("services/%s/%s", ctx.ServiceName, filename),
				Content: fileContent,
			})
		}
	}

	return actions, nil
}

func kustomizationName(serviceName, override string) string {
	if override != "" {
		return override
	}
	return serviceName
}

func renderInlineAutoTemplate(tmplStr string, ctx autoServiceContext) (string, error) {
	funcMap := sprig.TxtFuncMap()
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
  url: {{ .BaseRepoURL }}
  ref:
    branch: {{ .RepoBranch }}
{{- if .IsSSH }}
  secretRef:
    name: opencenter-base
{{- end }}
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
  cluster:
    name: {{ .ClusterName }}
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
  targetNamespace: {{ .Namespace }}
  prune: true
  wait: true
  force: {{ .Force }}
  suspend: {{ .Suspend }}
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
{{- if or .CustomResources .EnterpriseRegistry }}
resources:
{{- range .CustomResources }}
  - {{ . }}
{{- end }}
{{- if .EnterpriseRegistry }}
  - "../global/rackspace-registry/"
{{- end }}
{{- end }}
`
