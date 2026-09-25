package gitops

import (
	"fmt"
	"path/filepath"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	descriptorcfg "github.com/opencenter-cloud/opencenter-cli/internal/services/descriptors"
)

// RenderSpec describes all GitOps-owned rendering behavior for one auto-rendered
// service. It is deliberately independent from services.BaseConfig so internal
// topology and renderer choices cannot leak into user configuration.
type RenderSpec struct {
	ServiceName string

	DefaultNamespace string
	SourceName       string
	SourceGroup      string
	EmitSource       bool
	BasePath         string
	PostBaseStages   []postBaseStageSpec

	SingleStage             bool
	BaseOnly                bool
	OmitTargetNamespace     bool
	PrivilegedNamespace     bool
	HasOverrideValues       bool
	NamespaceStage          bool
	KustomizationName       string
	EnterpriseRegistry      bool
	GeneratedResourceFiles  []string
	ExtraDependencies       []string
	ConditionalDependencies []catalogConditionalDependency
	OverrideDependsOn       []string
	OverrideValues          string
	KustomizationContent    string

	OverrideValuesRenderer           OverrideValuesRenderer
	OverlayFilesRenderer             OverlayFilesRenderer
	KustomizationRenderer            KustomizationRenderer
	BaseKustomizationPatchesRenderer FluxKustomizationPatchesRenderer
}

type postBaseStageSpec struct {
	Name      string
	Path      string
	DependsOn []string
}

type catalogConditionalDependency struct {
	Name        string
	WhenEnabled string
}

type dynamicActionPlanner func(v2.Config, []secretartifacts.Artifact) ([]clusterAppAction, error)

type catalogDynamicPlanner struct {
	descriptorName string
	planner        dynamicActionPlanner
}

// RenderCatalog is an immutable lookup of built-in GitOps render specifications.
// The constructor is the only place where built-in entries are defined; callers
// receive values and cannot register or mutate catalog ownership globally.
type RenderCatalog struct {
	specs           []RenderSpec
	dynamicPlanners []catalogDynamicPlanner
}

func newBuiltInRenderCatalog() RenderCatalog {
	return RenderCatalog{specs: []RenderSpec{
		{
			ServiceName: "external-snapshotter", DefaultNamespace: "external-snapshotter",
			SourceName: "opencenter-external-snapshotter", SourceGroup: "external-snapshotter", EmitSource: true,
			BasePath: "applications/base/services/external-snapshotter", BaseOnly: true,
		},
		{
			ServiceName: "fluxcd", DefaultNamespace: "flux-system", HasOverrideValues: true,
			SourceName: "opencenter-fluxcd", SourceGroup: "fluxcd", EmitSource: true,
			BasePath: "applications/base/services/fluxcd",
		},
		{
			ServiceName: "gateway", DefaultNamespace: "gateway",
			SourceName: "opencenter-gateway", SourceGroup: "gateway", EmitSource: true,
			BasePath: "applications/base/services/gateway", SingleStage: true, HasOverrideValues: false,
			ExtraDependencies:      []string{"envoy-gateway-api-base"},
			GeneratedResourceFiles: []string{"namespace.yaml", "gateway-class.yaml", "gateway.yaml", "envoy-proxy-config.yaml"},
			// KustomizationRenderer lists the base four files plus one
			// envoy-proxy-config-<pool>.yaml per non-default MetalLB pool
			// (OCTR-762). For the single-pool case it emits exactly the four
			// files the previous static KustomizationContent listed, keeping
			// output byte-identical.
			KustomizationRenderer: gatewayKustomizationRenderer,
			OverlayFilesRenderer:  gatewayOverlayFilesRenderer,
		},
		{
			ServiceName: "gateway-api", DefaultNamespace: "envoy-gateway-system", HasOverrideValues: true,
			SourceName: "opencenter-gateway-api", SourceGroup: "gateway-api", EmitSource: true,
			BasePath: "applications/base/services/gateway-api", KustomizationName: "envoy-gateway-api",
			OverrideValues: "---\nenvoyGateway:\n  config:\n    envoyGateway:\n      logging:\n        level:\n          default: info\n",
		},
		{
			ServiceName: "headlamp", DefaultNamespace: "headlamp", HasOverrideValues: true,
			SourceName: "opencenter-headlamp", SourceGroup: "headlamp", EmitSource: true,
			BasePath: "applications/base/services/headlamp", OverrideValuesRenderer: templateRenderer(headlampTemplate),
		},
		{
			ServiceName: "postgres-operator", DefaultNamespace: "postgres-operator", HasOverrideValues: true,
			SourceName: "opencenter-postgres-operator", SourceGroup: "postgres-operator", EmitSource: true,
			BasePath: "applications/base/services/postgres-operator", OverrideValues: "configGeneral:\n  workers: 2\n",
		},
		{
			ServiceName: "rbac-manager", DefaultNamespace: "rbac-system",
			SourceName: "opencenter-rbac-manager", SourceGroup: "rbac-manager", EmitSource: true,
			BasePath: "applications/base/services/rbac-manager", BaseOnly: true,
			ConditionalDependencies: []catalogConditionalDependency{{Name: "kube-prometheus-stack-base", WhenEnabled: "kube-prometheus-stack"}},
		},
		{
			ServiceName: "sources", DefaultNamespace: "flux-system", HasOverrideValues: true,
			SourceName: "opencenter-sources", SourceGroup: "sources", EmitSource: true,
			BasePath: "applications/base/services/sources",
		},
		{
			ServiceName: "kube-prometheus-stack", DefaultNamespace: "observability", HasOverrideValues: true,
			SourceName: "opencenter-observability", SourceGroup: "observability", BasePath: "applications/base/services/observability/kube-prometheus-stack",
			ExtraDependencies: []string{"observability-namespace", "kube-prometheus-stack-override"}, OverrideDependsOn: []string{"sources", "envoy-gateway-api-base"},
			GeneratedResourceFiles:           []string{"prometheus-http-route.yaml", "alertmanager-http-route.yaml", "grafana-http-route.yaml"},
			OverrideValuesRenderer:           templateRenderer(kubePrometheusStackTemplate),
			OverlayFilesRenderer:             kubePrometheusStackOverlayFilesRenderer,
			BaseKustomizationPatchesRenderer: kubePrometheusStackBaseKustomizationPatchesRenderer,
		},
		{
			ServiceName: "kyverno", DefaultNamespace: "kyverno",
			SourceName: "opencenter-kyverno", SourceGroup: "kyverno", EmitSource: true,
			BasePath: "applications/base/services/kyverno/policy-engine", BaseOnly: true,
			PostBaseStages: []postBaseStageSpec{{
				Name:      "kyverno-default-ruleset",
				Path:      "applications/base/services/kyverno/default-ruleset",
				DependsOn: []string{"sources", "kyverno-base"},
			}},
		},
		{
			ServiceName: "loki", DefaultNamespace: "observability", HasOverrideValues: true,
			SourceName: "opencenter-observability", SourceGroup: "observability", BasePath: "applications/base/services/observability/loki",
			ExtraDependencies: []string{"observability-namespace", "observability-sources", "loki-override"}, OverrideDependsOn: []string{"sources"},
			OverrideValuesRenderer: templateRenderer(lokiTemplate),
		},
		{
			ServiceName: "openstack-ccm", DefaultNamespace: "openstack-ccm", HasOverrideValues: true, NamespaceStage: true,
			SourceName: "opencenter-openstack-ccm", SourceGroup: "openstack-ccm", EmitSource: true,
			BasePath: "applications/base/services/openstack-ccm", ExtraDependencies: []string{"openstack-ccm-override"}, OverrideDependsOn: []string{"sources", "openstack-ccm-namespace"},
			OverrideValuesRenderer: templateRenderer(openstackCCMTemplate),
		},
		{
			ServiceName: "openstack-csi", DefaultNamespace: "openstack-csi", HasOverrideValues: true, NamespaceStage: true,
			PrivilegedNamespace: true,
			SourceName:          "opencenter-openstack-csi", SourceGroup: "openstack-csi", EmitSource: true,
			BasePath: "applications/base/services/openstack-csi", ExtraDependencies: []string{"openstack-csi-override"}, OverrideDependsOn: []string{"sources", "openstack-csi-namespace"},
			OverrideValuesRenderer: templateRenderer(openstackCSITemplate),
		},
		{
			ServiceName: "tempo", DefaultNamespace: "observability", HasOverrideValues: true,
			SourceName: "opencenter-observability", SourceGroup: "observability", BasePath: "applications/base/services/observability/tempo",
			ExtraDependencies: []string{"observability-namespace", "observability-sources", "tempo-override"}, OverrideDependsOn: []string{"sources"},
			OverrideValuesRenderer: templateRenderer(tempoTemplate),
		},
		{
			ServiceName: "velero", DefaultNamespace: "velero", HasOverrideValues: true, NamespaceStage: true,
			SourceName: "opencenter-velero", SourceGroup: "velero", EmitSource: true,
			BasePath: "applications/base/services/velero", ExtraDependencies: []string{"velero-override"}, OverrideDependsOn: []string{"sources", "velero-namespace"},
			OverrideValuesRenderer: veleroRenderer,
		},
		{
			ServiceName: "vsphere-csi", DefaultNamespace: "vmware-system-csi", HasOverrideValues: true,
			SourceName: "opencenter-vsphere-csi", SourceGroup: "vsphere-csi", EmitSource: true,
			BasePath: "applications/base/services/vsphere-csi", OverrideValuesRenderer: templateRenderer(vsphereCsiTemplate),
		},
		{
			ServiceName: "weave-gitops", DefaultNamespace: "flux-system", HasOverrideValues: true,
			SourceName: "opencenter-weave-gitops", SourceGroup: "weave-gitops", EmitSource: true,
			BasePath: "applications/base/services/weave-gitops", OverrideDependsOn: []string{"sources", "envoy-gateway-api-base"},
		},
		{
			ServiceName: "longhorn", DefaultNamespace: "longhorn-system", HasOverrideValues: true,
			SourceName: "opencenter-longhorn", SourceGroup: "longhorn", EmitSource: true,
			BasePath: "applications/base/services/longhorn", OverlayFilesRenderer: longhornOverlayFilesRenderer,
			OverrideValues:    "persistence:\n  defaultClass: false\n",
			OverrideDependsOn: []string{"sources", "longhorn-base", "envoy-gateway-api-base"},
		},
		{
			ServiceName: "metallb", DefaultNamespace: "metallb-system", HasOverrideValues: true,
			SourceName: "opencenter-metallb", SourceGroup: "metallb", EmitSource: true,
			BasePath: "applications/base/services/metallb", OverlayFilesRenderer: metallbOverlayFilesRenderer,
		},
		{
			ServiceName: "mimir", DefaultNamespace: "observability", HasOverrideValues: true,
			SourceName: "opencenter-observability", SourceGroup: "observability", BasePath: "applications/base/services/observability/mimir",
			ExtraDependencies: []string{"observability-namespace", "observability-sources", "mimir-override"}, OverrideDependsOn: []string{"sources"},
			OverrideValuesRenderer: templateRenderer(mimirTemplate),
		},
		{
			ServiceName: "opentelemetry-kube-stack", DefaultNamespace: "observability", HasOverrideValues: true,
			SourceName: "opencenter-observability", SourceGroup: "observability", BasePath: "applications/base/services/observability/opentelemetry-kube-stack",
			OverrideValuesRenderer: staticRenderer(otelTemplate),
		},
		{
			ServiceName: "sealed-secrets", DefaultNamespace: "sealed-secrets", HasOverrideValues: true, NamespaceStage: true,
			SourceName: "opencenter-sealed-secrets", SourceGroup: "sealed-secrets", EmitSource: true,
			BasePath: "applications/base/services/sealed-secrets", OverrideValues: "keyrenewperiod: \"0\"\n",
			ExtraDependencies: []string{"sealed-secrets-override"}, OverrideDependsOn: []string{"sources", "sealed-secrets-namespace"},
		},
	}, dynamicPlanners: []catalogDynamicPlanner{
		{descriptorName: "service-cert-manager", planner: planCertManagerDynamicActions},
		{descriptorName: "service-etcd-backup", planner: planEtcdBackupDynamicActions},
	}}
}

func (c RenderCatalog) Lookup(serviceName string) (RenderSpec, bool) {
	for _, spec := range c.specs {
		if spec.ServiceName == serviceName {
			return spec, true
		}
	}
	return RenderSpec{}, false
}

func (c RenderCatalog) dynamicPlannerForDescriptor(descriptorName string) (dynamicActionPlanner, bool) {
	for _, dynamicPlanner := range c.dynamicPlanners {
		if dynamicPlanner.descriptorName == descriptorName {
			return dynamicPlanner.planner, true
		}
	}
	return nil, false
}

func (c RenderCatalog) planDynamicActionsForDescriptor(cfg v2.Config, descriptor descriptorcfg.Descriptor, artifacts []secretartifacts.Artifact) ([]clusterAppAction, error) {
	planner, ok := c.dynamicPlannerForDescriptor(descriptor.Name)
	if !ok {
		return nil, nil
	}
	if planner == nil {
		return nil, fmt.Errorf("render catalog dynamic planner for descriptor %q is nil", descriptor.Name)
	}
	actions, err := planner(cfg, artifacts)
	if err != nil {
		return nil, fmt.Errorf("dynamic planner for descriptor %q: %w", descriptor.Name, err)
	}
	if err := validateClusterAppActions(actions, ""); err != nil {
		return nil, fmt.Errorf("dynamic planner for descriptor %q produced invalid action output: %w", descriptor.Name, err)
	}
	return actions, nil
}

func (c RenderCatalog) Validate() error {
	seen := make(map[string]struct{}, len(c.specs))
	sourceOwners := make(map[string]string)
	for _, spec := range c.specs {
		if strings.TrimSpace(spec.ServiceName) == "" {
			return fmt.Errorf("render catalog contains a spec with an empty service name")
		}
		if _, exists := seen[spec.ServiceName]; exists {
			return fmt.Errorf("render catalog contains duplicate service %q", spec.ServiceName)
		}
		seen[spec.ServiceName] = struct{}{}
		if strings.TrimSpace(spec.SourceName) == "" || strings.TrimSpace(spec.BasePath) == "" {
			return fmt.Errorf("render catalog service %q must define source name and base path", spec.ServiceName)
		}
		if strings.TrimSpace(spec.DefaultNamespace) == "" {
			return fmt.Errorf("render catalog service %q must define a default namespace", spec.ServiceName)
		}
		if spec.EmitSource {
			if owner, exists := sourceOwners[spec.SourceName]; exists {
				return fmt.Errorf("render catalog source %q is emitted by both %q and %q", spec.SourceName, owner, spec.ServiceName)
			}
			sourceOwners[spec.SourceName] = spec.ServiceName
		}
		if strings.TrimSpace(spec.SourceGroup) == "" {
			return fmt.Errorf("render catalog service %q must define source group", spec.ServiceName)
		}
		if spec.SingleStage && spec.BaseOnly {
			return fmt.Errorf("render catalog service %q cannot be both single-stage and base-only", spec.ServiceName)
		}
		if spec.BaseOnly && (spec.KustomizationContent != "" || spec.OverrideValuesRenderer != nil || spec.OverlayFilesRenderer != nil) {
			return fmt.Errorf("render catalog base-only service %q cannot define overlay rendering", spec.ServiceName)
		}
		for _, path := range spec.GeneratedResourceFiles {
			if err := validateCatalogRelativePath(path); err != nil {
				return fmt.Errorf("render catalog service %q resource %q: %w", spec.ServiceName, path, err)
			}
		}
		for _, dependency := range spec.ExtraDependencies {
			if strings.TrimSpace(dependency) == "" {
				return fmt.Errorf("render catalog service %q contains an empty dependency", spec.ServiceName)
			}
		}
		for _, stage := range spec.PostBaseStages {
			if strings.TrimSpace(stage.Name) == "" || strings.TrimSpace(stage.Path) == "" {
				return fmt.Errorf("render catalog service %q post-base stage must define name and path", spec.ServiceName)
			}
			if err := validateCatalogRelativePath(stage.Path); err != nil {
				return fmt.Errorf("render catalog service %q post-base stage %q: %w", spec.ServiceName, stage.Name, err)
			}
			for _, dependency := range stage.DependsOn {
				if strings.TrimSpace(dependency) == "" {
					return fmt.Errorf("render catalog service %q post-base stage %q contains an empty dependency", spec.ServiceName, stage.Name)
				}
			}
		}
	}
	seenDynamicDescriptors := make(map[string]struct{}, len(c.dynamicPlanners))
	for _, dynamicPlanner := range c.dynamicPlanners {
		if strings.TrimSpace(dynamicPlanner.descriptorName) == "" {
			return fmt.Errorf("render catalog dynamic planner must define a descriptor name")
		}
		if dynamicPlanner.planner == nil {
			return fmt.Errorf("render catalog dynamic planner for descriptor %q is nil", dynamicPlanner.descriptorName)
		}
		if _, exists := seenDynamicDescriptors[dynamicPlanner.descriptorName]; exists {
			return fmt.Errorf("render catalog contains duplicate dynamic planner for descriptor %q", dynamicPlanner.descriptorName)
		}
		seenDynamicDescriptors[dynamicPlanner.descriptorName] = struct{}{}
	}
	return nil
}

func (c RenderCatalog) ValidateAgainstDescriptors(registry *descriptorcfg.Registry) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if registry == nil {
		return fmt.Errorf("cannot validate render catalog ownership against nil descriptor registry")
	}
	for _, dynamicPlanner := range c.dynamicPlanners {
		descriptor, exists := registry.Get(dynamicPlanner.descriptorName)
		if !exists {
			return fmt.Errorf("render catalog dynamic planner for descriptor %q has no explicit descriptor", dynamicPlanner.descriptorName)
		}
		serviceName := descriptor.Service
		if serviceName == "" {
			serviceName = descriptor.ManagedService
		}
		if serviceName == "" {
			return fmt.Errorf("render catalog dynamic planner for descriptor %q does not belong to a service descriptor", dynamicPlanner.descriptorName)
		}
		if _, autoOwned := c.Lookup(serviceName); autoOwned {
			return fmt.Errorf("render catalog dynamic planner for descriptor %q conflicts with auto-owned service %q", dynamicPlanner.descriptorName, serviceName)
		}
	}
	for _, descriptor := range registry.Descriptors() {
		serviceName := descriptor.Service
		if serviceName == "" {
			serviceName = descriptor.ManagedService
		}
		if serviceName == "" {
			continue
		}
		if _, owned := c.Lookup(serviceName); owned {
			return fmt.Errorf("service %q is owned by both render catalog and descriptor %q", serviceName, descriptor.Name)
		}
	}
	return nil
}

func (c RenderCatalog) ValidateConfigOwnership(cfg v2.Config, registry *descriptorcfg.Registry) error {
	if err := c.ValidateAgainstDescriptors(registry); err != nil {
		return err
	}
	for serviceName, serviceCfg := range cfg.OpenCenter.Services {
		if IsServiceDisabled(serviceCfg) || IsServiceExternal(serviceCfg) || hasExplicitDescriptor(registry, serviceName, serviceKindStandard) {
			continue
		}
		if _, owned := c.Lookup(serviceName); !owned {
			return fmt.Errorf("enabled service %q has neither an explicit descriptor nor a built-in render catalog entry", serviceName)
		}
	}
	for serviceName, serviceCfg := range cfg.OpenCenter.ManagedServices {
		if IsServiceDisabled(serviceCfg) || IsServiceExternal(serviceCfg) || hasExplicitDescriptor(registry, serviceName, serviceKindManaged) {
			continue
		}
		return fmt.Errorf("enabled service %q has neither an explicit descriptor nor a built-in render catalog entry", serviceName)
	}
	return nil
}

func validateCatalogRelativePath(path string) error {
	clean := filepath.Clean(path)
	if strings.TrimSpace(path) == "" || clean == "." || filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path must be a non-empty relative path")
	}
	return nil
}
