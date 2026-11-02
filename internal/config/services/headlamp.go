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
	// BrandingImage is the OpenCenter Headlamp branding plugin image, copied into
	// the plugins volume by the headlamp-plugins init container. Pin by digest:
	// a tag breaks rollback and Helm drift detection. Empty selects
	// DefaultBrandingImage. Override it to point at a mirrored copy, which
	// requires the mirror's own digest.
	BrandingImage string `yaml:"branding_image,omitempty" json:"branding_image,omitempty" jsonschema:"description=Digest-pinned OpenCenter branding plugin image copied into the Headlamp plugins directory"`
}

// DefaultBrandingImage is the digest-pinned OpenCenter branding plugin image
// deployed into Headlamp's plugins directory. Built from
// github.com/opencenter-cloud/opencenter-headlamp-plugins (plugins/branding) at
// commit e433cf1; the bundle is reproducible from that commit. Bump it together
// with the Headlamp chart version in openCenter-gitops-base: the plugin SDK must
// track the deployed Headlamp, because React, MUI and the Headlamp frontend
// libraries are externalized at build time and resolved from the host app at
// runtime.
//
// This digest ships plugins/branding/package.json alongside main.js, produced by
// the SDK's own `headlamp-plugin extract`. Earlier images copied only dist/, and
// Headlamp disables any plugin whose manifest is missing or lacks a compatible
// @kinvolk/headlamp-plugin devDependency, so the bundle was copied, served with
// HTTP 200, and never executed.
const DefaultBrandingImage = "ghcr.io/opencenter-cloud/opencenter-headlamp-plugin-branding@sha256:1afca1d1965c1698b0d8e2ce2878290cfc93d6437173c4e3111764745628c00c"

// ResolvedBrandingImage returns the branding plugin image for this config,
// falling back to DefaultBrandingImage when unset.
func (c HeadlampConfig) ResolvedBrandingImage() string {
	if c.BrandingImage == "" {
		return DefaultBrandingImage
	}
	return c.BrandingImage
}

func init() {
	registry.RegisterServiceConfig("headlamp", HeadlampConfig{})
}
