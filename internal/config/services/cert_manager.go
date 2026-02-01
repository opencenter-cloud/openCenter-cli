package services

import (
	"github.com/rackerlabs/opencenter-cli/internal/config/registry"
)

// CertManagerConfig extends BaseConfig with cert-manager configuration
type CertManagerConfig struct {
	BaseConfig `yaml:",inline"`

	LetsEncryptServer   string       `yaml:"letsencrypt_server" json:"letsencrypt_server,omitempty" jsonschema:"description=LetsEncrypt ACME server URL,default=https://acme-v02.api.letsencrypt.org/directory"`
	Email               string       `yaml:"email" json:"email,omitempty" jsonschema:"description=Email for LetsEncrypt registration"`
	Region              string       `yaml:"region" json:"region,omitempty" jsonschema:"description=AWS region for Route53 DNS validation"`
	DNSZones            []string     `yaml:"dns_zones" json:"dns_zones,omitempty" jsonschema:"description=DNS zones for certificate validation"`
	CreateClusterIssuer bool         `yaml:"create_cluster_issuer" json:"create_cluster_issuer,omitempty" jsonschema:"description=Create external ClusterIssuer resource,default=true"`
	Issuers             []CertIssuer `yaml:"issuers" json:"issuers,omitempty" jsonschema:"description=List of certificate issuers"`

	// DNS provider configuration for ACME DNS-01 challenge
	DNSProvider string `yaml:"dns_provider" json:"dns_provider,omitempty" jsonschema:"description=DNS provider for ACME DNS-01 challenge,enum=route53,enum=designate,enum=cloudflare,enum=clouddns,enum=azuredns"`
}

// CertIssuer represents a certificate issuer configuration
type CertIssuer struct {
	Name   string `yaml:"name" json:"name" jsonschema:"description=Issuer name,required"`
	Type   string `yaml:"type" json:"type" jsonschema:"description=Issuer type,enum=letsencrypt,enum=selfsigned,enum=ca,required"`
	Server string `yaml:"server" json:"server,omitempty" jsonschema:"description=ACME server URL for LetsEncrypt issuers"`
}

func init() {
	registry.RegisterServiceConfig("cert-manager", CertManagerConfig{})
}
