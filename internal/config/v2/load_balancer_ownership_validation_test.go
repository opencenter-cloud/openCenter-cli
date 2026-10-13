package v2

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
)

func TestValidateServicesRejectsOpenStackCCMAndMetalLB(t *testing.T) {
	cfg := &Config{OpenCenter: OpenCenterConfig{
		Infrastructure: InfrastructureConfig{Provider: "openstack"},
		Services: ServiceMap{
			"openstack-ccm": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
			"metallb":       &services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		},
	}}

	err := NewValidator().ValidateServices(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "both controllers manage classless LoadBalancer Services")
}

func TestValidateServicesAllowsOpenStackCCMWithoutMetalLB(t *testing.T) {
	cfg := &Config{OpenCenter: OpenCenterConfig{
		Infrastructure: InfrastructureConfig{Provider: "openstack"},
		Services: ServiceMap{
			"openstack-ccm": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
			"metallb":       &services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		},
	}}

	require.NoError(t, NewValidator().ValidateServices(cfg))
}
