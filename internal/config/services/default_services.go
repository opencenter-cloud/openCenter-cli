package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// DefaultServiceConfig is used for services that don't have specific configuration
// beyond the common base fields.
type DefaultServiceConfig struct {
	BaseConfig `yaml:",inline"`
}

func init() {
	// Register default services that have no service-specific config
	defaults := []string{
		"external-snapshotter",
		"fluxcd",
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
		"weave-gitops",
	}

	for _, name := range defaults {
		registry.RegisterServiceConfig(name, DefaultServiceConfig{})
	}
}
