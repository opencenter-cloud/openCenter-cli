package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// KeycloakConfig extends BaseConfig with Keycloak-specific configuration
type KeycloakConfig struct {
	BaseConfig `yaml:",inline"`

	Realm       string `yaml:"keycloak_realm" json:"keycloak_realm,omitempty" jsonschema:"description=Keycloak realm name"`
	FrontendURL string `yaml:"keycloak_frontend_url" json:"keycloak_frontend_url,omitempty" jsonschema:"description=Keycloak frontend URL"`
	ClientID    string `yaml:"keycloak_client_id" json:"keycloak_client_id,omitempty" jsonschema:"description=Keycloak client ID"`
}

func init() {
	registry.RegisterServiceConfig("keycloak", KeycloakConfig{})
}
