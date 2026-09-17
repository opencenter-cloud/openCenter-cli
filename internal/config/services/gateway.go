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
	TLS              *GatewayTLSConfig `yaml:"tls,omitempty" json:"tls,omitempty" jsonschema:"description=Bring-your-own TLS configuration for Gateway listeners"`
	Listeners        []GatewayListener `yaml:"listeners,omitempty" json:"listeners,omitempty" jsonschema:"description=List of Gateway listeners"`
}

// GatewayTLSConfig lets operators supply their own TLS material instead of
// having cert-manager mint every leaf certificate. When any bring-your-own
// option is set, the generator stops emitting the
// cert-manager.io/cluster-issuer annotation on the Gateway.
type GatewayTLSConfig struct {
	// WildcardSecretName, when set, makes every HTTPS listener terminate with
	// this single pre-existing TLS Secret (e.g. a wildcard certificate) and
	// drops the cert-manager annotation.
	WildcardSecretName string `yaml:"wildcard_secret_name,omitempty" json:"wildcard_secret_name,omitempty" jsonschema:"description=A single pre-existing TLS Secret (e.g. a wildcard cert) used by every HTTPS listener; drops the cert-manager annotation"`
	// SecretNamespace optionally namespaces the referenced Secret(s). Empty
	// leaves the certificateRef without a namespace (same-namespace lookup).
	SecretNamespace string `yaml:"secret_namespace,omitempty" json:"secret_namespace,omitempty" jsonschema:"description=Namespace of the pre-existing TLS Secret(s); empty means the Gateway namespace"`
	// PerListenerSecrets overrides the TLS Secret for specific listeners by
	// listener logical name (e.g. keycloak, harbor, longhorn). Takes precedence
	// over the hardcoded default and over WildcardSecretName for that listener.
	PerListenerSecrets map[string]string `yaml:"per_listener_secrets,omitempty" json:"per_listener_secrets,omitempty" jsonschema:"description=Override the TLS Secret name for specific listeners by logical name (keycloak, harbor, longhorn, ...)"`
}

// IsBYO reports whether any bring-your-own TLS option is configured. When true,
// the generator omits the cert-manager cluster-issuer annotation.
func (g *GatewayConfig) IsBYO() bool {
	if g == nil || g.TLS == nil {
		return false
	}
	return g.TLS.WildcardSecretName != "" || len(g.TLS.PerListenerSecrets) > 0
}

// TLSSecretFor returns the TLS Secret name for a listener identified by its
// logical name (e.g. "keycloak") given the built-in default. Precedence:
// per-listener override, then wildcard, then the built-in default.
func (g *GatewayConfig) TLSSecretFor(listener, defaultSecret string) string {
	if g == nil || g.TLS == nil {
		return defaultSecret
	}
	if name, ok := g.TLS.PerListenerSecrets[listener]; ok && name != "" {
		return name
	}
	if g.TLS.WildcardSecretName != "" {
		return g.TLS.WildcardSecretName
	}
	return defaultSecret
}

// TLSSecretNamespace returns the configured Secret namespace, or "".
func (g *GatewayConfig) TLSSecretNamespace() string {
	if g == nil || g.TLS == nil {
		return ""
	}
	return g.TLS.SecretNamespace
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
