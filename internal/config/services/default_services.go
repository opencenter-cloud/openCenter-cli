package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// DefaultServiceConfig is used for services that don't have specific configuration
type DefaultServiceConfig struct {
	BaseConfig `yaml:",inline"`
}

func init() {
	// Register default services
	defaults := []string{
		"external-snapshotter",
		"fluxcd",
		"gateway",
		"gateway-api",
		"kyverno",
		"olm",
		"openstack-ccm",
		"openstack-csi",
		"postgres-operator",
		"rbac-manager",
		"sources",
	}
	
	for _, name := range defaults {
		registry.RegisterServiceConfig(name, DefaultServiceConfig{})
	}
}
