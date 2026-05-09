package services

// AdoptionMode defines how Flux interacts with a service that may already exist in the cluster.
type AdoptionMode string

const (
	// AdoptionModeManaged means Flux fully manages the service (default behavior).
	AdoptionModeManaged AdoptionMode = "managed"

	// AdoptionModeExternal means the service exists outside of Flux management.
	AdoptionModeExternal AdoptionMode = "external"

	// AdoptionModeSync means Flux renders manifests but won't force changes.
	AdoptionModeSync AdoptionMode = "sync"

	// AdoptionModeDeferred means Flux renders manifests but suspends the Kustomization.
	AdoptionModeDeferred AdoptionMode = "deferred"

	// AdoptionModeTakeover means Flux will take over management of an existing service.
	AdoptionModeTakeover AdoptionMode = "takeover"
)

// ServiceSource describes where the GitOps manifests for a service come from.
type ServiceSource struct {
	Repo    string `yaml:"repo,omitempty" json:"repo,omitempty" jsonschema:"description=GitOps source repository URL"`
	Branch  string `yaml:"branch,omitempty" json:"branch,omitempty" jsonschema:"description=Git branch to track"`
	Release string `yaml:"release,omitempty" json:"release,omitempty" jsonschema:"description=Pinned release tag (mutually exclusive with branch)"`
}

// ServiceImage describes the container image for a service.
type ServiceImage struct {
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty" jsonschema:"description=Container image repository"`
	Tag        string `yaml:"tag,omitempty" json:"tag,omitempty" jsonschema:"description=Container image tag"`
}

// BaseConfig contains common fields for all services.
// Service-specific configs embed this via yaml:",inline".
//
// Design decisions (clean break from v1 flat layout):
//   - `status` removed: runtime state does not belong in declarative config.
//   - `hostname` / `uri` removed from base: derivable from cluster FQDN + service name;
//     services that genuinely need a custom hostname declare it in their own config section.
//   - `source` groups all GitOps source fields into a nested object.
//   - `image` groups container image fields into a nested object.
//   - `adoption_mode` constrained to a known enum.
type BaseConfig struct {
	Enabled      bool         `yaml:"enabled" json:"enabled" jsonschema:"description=Whether this service is deployed"`
	AdoptionMode AdoptionMode `yaml:"adoption_mode,omitempty" json:"adoption_mode,omitempty" jsonschema:"description=How Flux interacts with this service,enum=managed,enum=external,enum=sync,enum=deferred,enum=takeover,default=managed"`
	Namespace    string       `yaml:"namespace,omitempty" json:"namespace,omitempty" jsonschema:"description=Kubernetes namespace for the service"`
	Source       ServiceSource `yaml:"source,omitempty" json:"source,omitempty" jsonschema:"description=GitOps source configuration"`
	Image        ServiceImage  `yaml:"image,omitempty" json:"image,omitempty" jsonschema:"description=Container image configuration"`
}

// IsEnabled returns true if the service is enabled.
func (b BaseConfig) IsEnabled() bool {
	return b.Enabled
}

// GetStatus is retained for interface compatibility but always returns empty.
// Status is no longer stored in declarative config.
func (b BaseConfig) GetStatus() string {
	return ""
}

// GetAdoptionMode returns the adoption mode of the service.
// Returns AdoptionModeManaged if not set (default behavior).
func (b BaseConfig) GetAdoptionMode() AdoptionMode {
	if b.AdoptionMode == "" {
		return AdoptionModeManaged
	}
	return b.AdoptionMode
}

// IsExternal returns true if the service is externally managed (not rendered by Flux).
func (b BaseConfig) IsExternal() bool {
	return b.GetAdoptionMode() == AdoptionModeExternal
}
