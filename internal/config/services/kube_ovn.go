package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// KubeOVNConfig extends BaseConfig with Kube-OVN-specific configuration.
type KubeOVNConfig struct {
	BaseConfig `yaml:",inline"`

	CiliumIntegration bool   `yaml:"cilium_integration,omitempty" json:"cilium_integration,omitempty" jsonschema:"description=Enable Cilium integration for load balancing"`
	DefaultSubnet     string `yaml:"default_subnet,omitempty" json:"default_subnet,omitempty" jsonschema:"description=Default subnet for pod networking"`
	Version           string `yaml:"version,omitempty" json:"version,omitempty" jsonschema:"description=Kube-OVN version to install"`
	EnableLB          bool   `yaml:"enable_lb,omitempty" json:"enable_lb,omitempty" jsonschema:"description=Enable load balancing features"`
}

func init() {
	registry.RegisterServiceConfig("kube-ovn", KubeOVNConfig{})
}
