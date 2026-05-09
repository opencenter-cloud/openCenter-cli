package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// HeadlampConfig extends BaseConfig with Headlamp-specific configuration.
type HeadlampConfig struct {
	BaseConfig `yaml:",inline"`

	Hostname      string `yaml:"hostname,omitempty" json:"hostname,omitempty" jsonschema:"description=Headlamp external hostname"`
	OIDCIssuerURL string `yaml:"oidc_issuer_url,omitempty" json:"oidc_issuer_url,omitempty" jsonschema:"description=OIDC issuer URL for Headlamp authentication"`
	OIDCClientID  string `yaml:"oidc_client_id,omitempty" json:"oidc_client_id,omitempty" jsonschema:"description=OIDC client ID for Headlamp authentication"`
}

func init() {
	registry.RegisterServiceConfig("headlamp", HeadlampConfig{})
}
