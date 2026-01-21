package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// AlertProxyConfig extends BaseConfig with AlertProxy-specific configuration
type AlertProxyConfig struct {
	BaseConfig `yaml:",inline"`

	AlertManagerBaseUrl string `yaml:"alert_manager_base_url" json:"alert_manager_base_url,omitempty" jsonschema:"description=Alert manager base URL"`
	HTTPRouteFQDN       string `yaml:"http_route_fqdn" json:"http_route_fqdn,omitempty" jsonschema:"description=HTTPRoute FQDN"`
}

func init() {
	registry.RegisterServiceConfig("alert-proxy", AlertProxyConfig{})
}
