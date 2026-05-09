package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// CertManagerConfig extends BaseConfig with cert-manager configuration.
type CertManagerConfig struct {
	BaseConfig `yaml:",inline"`

	LetsEncryptServer   string       `yaml:"letsencrypt_server,omitempty" json:"letsencrypt_server,omitempty" jsonschema:"description=LetsEncrypt ACME server URL,default=https://acme-v02.api.letsencrypt.org/directory"`
	Email               string       `yaml:"email,omitempty" json:"email,omitempty" jsonschema:"description=Email for LetsEncrypt registration"`
	Region              string       `yaml:"region,omitempty" json:"region,omitempty" jsonschema:"description=Cloud region for DNS validation (e.g. AWS Route53 region)"`
	DNSZones            []string     `yaml:"dns_zones,omitempty" json:"dns_zones,omitempty" jsonschema:"description=DNS zones for certificate validation"`
	CreateClusterIssuer bool         `yaml:"create_cluster_issuer,omitempty" json:"create_cluster_issuer,omitempty" jsonschema:"description=Create external ClusterIssuer resource,default=true"`
	Issuers             []CertIssuer `yaml:"issuers,omitempty" json:"issuers,omitempty" jsonschema:"description=List of certificate issuers"`
	DNSProvider         string       `yaml:"dns_provider,omitempty" json:"dns_provider,omitempty" jsonschema:"description=DNS provider for ACME DNS-01 challenge,enum=route53,enum=designate,enum=cloudflare,enum=clouddns,enum=azuredns"`
}

// CertIssuer represents a certificate issuer configuration.
type CertIssuer struct {
	Name   string `yaml:"name" json:"name" jsonschema:"description=Issuer name,required"`
	Type   string `yaml:"type" json:"type" jsonschema:"description=Issuer type,enum=letsencrypt,enum=selfsigned,enum=ca,required"`
	Server string `yaml:"server,omitempty" json:"server,omitempty" jsonschema:"description=ACME server URL for LetsEncrypt issuers"`
}

func init() {
	registry.RegisterServiceConfig("cert-manager", CertManagerConfig{})
}
