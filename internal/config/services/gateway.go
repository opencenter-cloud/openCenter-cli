package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// GatewayConfig extends BaseConfig with Gateway-specific configuration.
type GatewayConfig struct {
	BaseConfig `yaml:",inline"`

	GatewayName      string            `yaml:"gateway_name,omitempty" json:"gateway_name,omitempty" jsonschema:"description=Name of the Gateway resource,default=rmpk-gateway"`
	GatewayNamespace string            `yaml:"gateway_namespace,omitempty" json:"gateway_namespace,omitempty" jsonschema:"description=Namespace for the Gateway resource,default=rackspace-system"`
	GatewayClass     string            `yaml:"gateway_class,omitempty" json:"gateway_class,omitempty" jsonschema:"description=Gateway class name,default=eg"`
	DefaultIssuer    string            `yaml:"default_issuer,omitempty" json:"default_issuer,omitempty" jsonschema:"description=Default certificate issuer for Gateway listeners"`
	Listeners        []GatewayListener `yaml:"listeners,omitempty" json:"listeners,omitempty" jsonschema:"description=List of Gateway listeners"`
}

// GatewayListener represents a Gateway listener configuration.
type GatewayListener struct {
	Name          string `yaml:"name" json:"name" jsonschema:"description=Name of the listener,required"`
	Port          int    `yaml:"port" json:"port" jsonschema:"description=Port number for the listener,required"`
	Protocol      string `yaml:"protocol" json:"protocol" jsonschema:"description=Protocol (HTTP or HTTPS),enum=HTTP,enum=HTTPS,required"`
	Hostname      string `yaml:"hostname,omitempty" json:"hostname,omitempty" jsonschema:"description=Hostname for the listener"`
	TLSSecretName string `yaml:"tls_secret_name,omitempty" json:"tls_secret_name,omitempty" jsonschema:"description=Name of the TLS secret for HTTPS listeners"`
}

func init() {
	registry.RegisterServiceConfig("gateway", GatewayConfig{})
}
