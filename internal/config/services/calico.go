package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// CalicoConfig extends BaseConfig with Calico-specific configuration.
type CalicoConfig struct {
	BaseConfig `yaml:",inline"`

	KubeAPIServer string `yaml:"kube_api_server,omitempty" json:"kube_api_server,omitempty" jsonschema:"description=Calico Kubernetes API server address"`
}

func init() {
	registry.RegisterServiceConfig("calico", CalicoConfig{})
}
