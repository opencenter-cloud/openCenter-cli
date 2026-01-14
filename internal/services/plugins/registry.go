package plugins

import (
	"fmt"

	svc "github.com/rackerlabs/openCenter-cli/internal/services"
)

// RegisterBuiltInServices registers all built-in service plugins with the registry
func RegisterBuiltInServices(registry svc.ServiceRegistry) error {
	// Define all built-in services with their plugins
	services := []struct {
		plugin       svc.ServicePlugin
		dependencies []string
	}{
		// Core services
		{plugin: NewDefaultServicePlugin("fluxcd", svc.ServiceTypeGitOps), dependencies: []string{}},
		{plugin: NewDefaultServicePlugin("gateway-api", svc.ServiceTypeNetworking), dependencies: []string{}},
		{plugin: NewDefaultServicePlugin("gateway", svc.ServiceTypeNetworking), dependencies: []string{"gateway-api"}},
		
		// Networking services
		{plugin: NewCalicoPlugin(), dependencies: []string{}},
		
		// Security services
		{plugin: NewCertManagerPlugin(), dependencies: []string{}},
		{plugin: NewKeycloakPlugin(), dependencies: []string{"cert-manager"}},
		{plugin: NewDefaultServicePlugin("kyverno", svc.ServiceTypeSecurity), dependencies: []string{}},
		{plugin: NewDefaultServicePlugin("rbac-manager", svc.ServiceTypeSecurity), dependencies: []string{}},
		
		// Storage services
		{plugin: NewDefaultServicePlugin("external-snapshotter", svc.ServiceTypeStorage), dependencies: []string{}},
		{plugin: NewDefaultServicePlugin("openstack-csi", svc.ServiceTypeStorage), dependencies: []string{}},
		{plugin: NewVSphereCSIPlugin(), dependencies: []string{}},
		{plugin: NewVeleroPlugin(), dependencies: []string{}},
		{plugin: NewEtcdBackupPlugin(), dependencies: []string{}},
		
		// Monitoring services
		{plugin: NewPrometheusStackPlugin(), dependencies: []string{}},
		{plugin: NewAlertProxyPlugin(), dependencies: []string{"kube-prometheus-stack"}},
		
		// Logging services
		{plugin: NewLokiPlugin(), dependencies: []string{}},
		
		// Dashboard services
		{plugin: NewHeadlampPlugin(), dependencies: []string{}},
		{plugin: NewWeaveGitOpsPlugin(), dependencies: []string{"fluxcd"}},
		
		// Other services
		{plugin: NewDefaultServicePlugin("openstack-ccm", svc.ServiceTypeCore), dependencies: []string{}},
		{plugin: NewDefaultServicePlugin("olm", svc.ServiceTypeCore), dependencies: []string{}},
		{plugin: NewDefaultServicePlugin("postgres-operator", svc.ServiceTypeStorage), dependencies: []string{}},
		{plugin: NewDefaultServicePlugin("sources", svc.ServiceTypeGitOps), dependencies: []string{}},
	}

	// Register each service
	for _, s := range services {
		serviceDef := svc.ServiceDefinition{
			Name:         s.plugin.Name(),
			Type:         s.plugin.Type(),
			Version:      "1.0.0", // Default version for built-in services
			Description:  fmt.Sprintf("Built-in %s service", s.plugin.Name()),
			Dependencies: s.dependencies,
			Templates:    []svc.TemplateRef{},
			Plugin:       s.plugin,
			Lifecycle:    svc.ServiceLifecycle{},
			Metadata: svc.ServiceMetadata{
				Author: "openCenter",
				Tags:   []string{"built-in"},
			},
		}

		if err := registry.RegisterService(serviceDef); err != nil {
			return fmt.Errorf("failed to register service %s: %w", s.plugin.Name(), err)
		}
	}

	return nil
}

// GetBuiltInServiceNames returns a list of all built-in service names
func GetBuiltInServiceNames() []string {
	return []string{
		"alert-proxy",
		"calico",
		"cert-manager",
		"etcd-backup",
		"external-snapshotter",
		"fluxcd",
		"gateway",
		"gateway-api",
		"headlamp",
		"keycloak",
		"kube-prometheus-stack",
		"kyverno",
		"loki",
		"olm",
		"openstack-ccm",
		"openstack-csi",
		"postgres-operator",
		"rbac-manager",
		"sources",
		"velero",
		"vsphere-csi",
		"weave-gitops",
	}
}
