package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// CalicoConfig extends BaseConfig with Calico-specific configuration
type CalicoConfig struct {
	BaseConfig `yaml:",inline"`

	KubeAPIServer string `yaml:"calico_kube_api_server" json:"calico_kube_api_server,omitempty" jsonschema:"description=Calico Kubernetes API server address"`
}

func init() {
	registry.RegisterServiceConfig("calico", CalicoConfig{})
}
