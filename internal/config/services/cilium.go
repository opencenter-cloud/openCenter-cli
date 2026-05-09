package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// CiliumConfig extends BaseConfig with Cilium-specific configuration.
type CiliumConfig struct {
	BaseConfig `yaml:",inline"`

	OperatorEnabled      bool   `yaml:"operator_enabled,omitempty" json:"operator_enabled,omitempty" jsonschema:"description=Enable Cilium operator for advanced features"`
	KubeProxyReplacement bool   `yaml:"kube_proxy_replacement,omitempty" json:"kube_proxy_replacement,omitempty" jsonschema:"description=Replace kube-proxy with Cilium eBPF implementation"`
	ModuleSource         string `yaml:"module_source,omitempty" json:"module_source,omitempty" jsonschema:"description=Cilium module source location"`
}

func init() {
	registry.RegisterServiceConfig("cilium", CiliumConfig{})
}
