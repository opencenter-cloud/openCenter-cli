package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// HeadlampConfig extends BaseConfig with Headlamp-specific configuration
type HeadlampConfig struct {
	BaseConfig `yaml:",inline"`

	OIDCIssuerURL string `yaml:"headlamp_oidc_issuer_url" json:"headlamp_oidc_issuer_url,omitempty" jsonschema:"description=Headlamp OIDC issuer URL"`
	OIDCClientID  string `yaml:"headlamp_oidc_client_id" json:"headlamp_oidc_client_id,omitempty" jsonschema:"description=Headlamp OIDC client ID"`
}

func init() {
	registry.RegisterServiceConfig("headlamp", HeadlampConfig{})
}
