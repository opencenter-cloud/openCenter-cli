package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// AlertProxyConfig extends BaseConfig with AlertProxy-specific configuration.
type AlertProxyConfig struct {
	BaseConfig `yaml:",inline"`

	AlertManagerBaseURL string `yaml:"alert_manager_base_url,omitempty" json:"alert_manager_base_url,omitempty" jsonschema:"description=Alert manager base URL"`
	HTTPRouteFQDN       string `yaml:"http_route_fqdn,omitempty" json:"http_route_fqdn,omitempty" jsonschema:"description=HTTPRoute FQDN for the alert proxy"`
}

func init() {
	registry.RegisterServiceConfig("alert-proxy", AlertProxyConfig{})
}
