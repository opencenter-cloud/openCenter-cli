package importer

import (
	"context"
	"reflect"
	"sort"

	configregistry "github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

type ServiceDetector interface {
	ServiceName() string
	Detect(context.Context, *detectContext) ServiceInferenceResult
}

type detectContext struct {
	clusterName string
	sources     ClusterSources
	legacy      legacyConfigData
	namespaces  *NamespaceRegistry
	config      *v2.Config
}

type DetectorRegistry struct {
	detectors map[string]ServiceDetector
	names     []string
}

func NewDetectorRegistry() *DetectorRegistry {
	names := configregistry.GetRegisteredServices()
	sort.Strings(names)

	detectors := make(map[string]ServiceDetector, len(names))
	for _, name := range names {
		detectors[name] = genericServiceDetector{name: name}
	}

	return &DetectorRegistry{
		detectors: detectors,
		names:     names,
	}
}

func (r *DetectorRegistry) Names() []string {
	return append([]string(nil), r.names...)
}

func (r *DetectorRegistry) Detector(name string) ServiceDetector {
	return r.detectors[name]
}

type genericServiceDetector struct {
	name string
}

func (d genericServiceDetector) ServiceName() string {
	return d.name
}

func (d genericServiceDetector) Detect(_ context.Context, ctx *detectContext) ServiceInferenceResult {
	result := ServiceInferenceResult{
		ServiceName: d.name,
		Namespaces:  ctx.namespaces.NamespacesFor(d.name),
	}

	target, _, ok := serviceConfigTarget(ctx.config, d.name)
	if !ok {
		result.Skipped = append(result.Skipped, SkippedField{
			Path:   "opencenter.services." + d.name,
			Reason: "service is not present in the target configuration",
		})
		return result
	}

	overlayEnabled, overlayEvidence := overlayServiceEnabled(ctx.sources, d.name)
	legacySection, legacyFound := ctx.legacy.serviceConfigSection(d.name)

	if legacyFound {
		if err := decodeLegacyServiceConfig(legacySection, target); err == nil {
			setServiceNamespace(target, result.Namespaces)
			enabled := serviceEnabledValue(target)
			result.Enabled = &enabled
			result.Fields = append(result.Fields,
				FieldInferenceResult{
					Path:       serviceConfigPath(ctx.config, d.name) + ".enabled",
					Value:      enabled,
					Confidence: ConfidenceHigh,
					Origin:     FieldOriginGitOps,
					Evidence:   []EvidenceRef{{Source: "legacy-config", Path: ctx.sources.LegacyConfigPath}},
				},
				FieldInferenceResult{
					Path:       serviceConfigPath(ctx.config, d.name) + ".namespace",
					Value:      result.Namespaces[0],
					Confidence: ConfidenceHigh,
					Origin:     FieldOriginDefault,
					Evidence:   []EvidenceRef{{Source: "namespace-registry", Detail: d.name}},
				},
			)
			return result
		}

		result.Skipped = append(result.Skipped, SkippedField{
			Path:   serviceConfigPath(ctx.config, d.name),
			Reason: "legacy service configuration could not be decoded",
		})
	}

	if overlayEnabled {
		enableService(target)
		setServiceNamespace(target, result.Namespaces)
		enabled := true
		result.Enabled = &enabled
		result.Fields = append(result.Fields,
			FieldInferenceResult{
				Path:       serviceConfigPath(ctx.config, d.name) + ".enabled",
				Value:      true,
				Confidence: ConfidenceHigh,
				Origin:     FieldOriginGitOps,
				Evidence:   []EvidenceRef{overlayEvidence},
			},
			FieldInferenceResult{
				Path:       serviceConfigPath(ctx.config, d.name) + ".namespace",
				Value:      result.Namespaces[0],
				Confidence: ConfidenceHigh,
				Origin:     FieldOriginDefault,
				Evidence:   []EvidenceRef{{Source: "namespace-registry", Detail: d.name}},
			},
		)
	}

	return result
}

func serviceConfigTarget(cfg *v2.Config, serviceName string) (any, string, bool) {
	if cfg == nil {
		return nil, "", false
	}
	if cfg.OpenCenter.ManagedServices != nil {
		if service, ok := cfg.OpenCenter.ManagedServices[serviceName]; ok {
			return service, "opencenter.managed_services." + serviceName, true
		}
	}
	if cfg.OpenCenter.Services != nil {
		if service, ok := cfg.OpenCenter.Services[serviceName]; ok {
			return service, "opencenter.services." + serviceName, true
		}
	}
	return nil, "", false
}

func serviceConfigPath(cfg *v2.Config, serviceName string) string {
	_, path, ok := serviceConfigTarget(cfg, serviceName)
	if !ok {
		return "opencenter.services." + serviceName
	}
	return path
}

func enableService(service any) {
	if base := baseConfigPointer(service); base != nil {
		base.Enabled = true
	}
}

func serviceEnabledValue(service any) bool {
	if enabler, ok := service.(interface{ IsEnabled() bool }); ok {
		return enabler.IsEnabled()
	}
	if base := baseConfigPointer(service); base != nil {
		return base.Enabled
	}
	return false
}

func setServiceNamespace(service any, namespaces []string) {
	if len(namespaces) == 0 {
		return
	}
	if base := baseConfigPointer(service); base != nil {
		base.Namespace = namespaces[0]
	}
}

func baseConfigPointer(service any) *configservices.BaseConfig {
	if service == nil {
		return nil
	}

	value := reflect.ValueOf(service)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return nil
	}

	field := value.FieldByName("BaseConfig")
	if !field.IsValid() || !field.CanAddr() {
		return nil
	}

	base, ok := field.Addr().Interface().(*configservices.BaseConfig)
	if !ok {
		return nil
	}
	return base
}
