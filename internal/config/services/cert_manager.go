package services

import (
	"github.com/rackerlabs/opencenter-cli/internal/config/registry"
)

// CertManagerConfig extends BaseConfig with cert-manager configuration
type CertManagerConfig struct {
	BaseConfig `yaml:",inline"`

	LetsEncryptServer string `yaml:"letsencrypt_server" json:"letsencrypt_server,omitempty" jsonschema:"description=LetsEncrypt ACME server URL"`
	Email             string `yaml:"email" json:"email,omitempty" jsonschema:"description=Email for LetsEncrypt registration"`
	Region            string `yaml:"region" json:"region,omitempty" jsonschema:"description=Cloud region (deprecated: use service-specific config)"`
	
	// DNS provider configuration for ACME DNS-01 challenge
	DNSProvider string `yaml:"dns_provider" json:"dns_provider,omitempty" jsonschema:"description=DNS provider for ACME DNS-01 challenge (route53, designate, cloudflare, clouddns, azuredns),enum=route53,enum=designate,enum=cloudflare,enum=clouddns,enum=azuredns"`
}

func init() {
	registry.RegisterServiceConfig("cert-manager", CertManagerConfig{})
}
