package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// CertManagerConfig extends BaseConfig with cert-manager configuration
type CertManagerConfig struct {
	BaseConfig `yaml:",inline"`

	LetsEncryptServer string `yaml:"letsencrypt_server" json:"letsencrypt_server,omitempty" jsonschema:"description=LetsEncrypt ACME server URL"`
	Email             string `yaml:"email" json:"email,omitempty" jsonschema:"description=Email for LetsEncrypt registration"`
	Region            string `yaml:"region" json:"region,omitempty" jsonschema:"description=Cloud region (deprecated: use service-specific config)"`
}

func init() {
	registry.RegisterServiceConfig("cert-manager", CertManagerConfig{})
}
