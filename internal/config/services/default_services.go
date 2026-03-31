package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
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
		"kafka-cluster",
		"kyverno",
		"mimir",
		"olm",
		"openstack-ccm",
		"openstack-csi",
		"postgres-operator",
		"rbac-manager",
		"sealed-secrets",
		"sources",
	}

	for _, name := range defaults {
		registry.RegisterServiceConfig(name, DefaultServiceConfig{})
	}
}
