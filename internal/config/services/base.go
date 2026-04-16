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

// BaseConfig contains common fields for all services
type BaseConfig struct {
	Enabled      bool         `yaml:"enabled" json:"enabled"`
	AdoptionMode AdoptionMode `yaml:"adoption_mode,omitempty" json:"adoption_mode,omitempty" jsonschema:"description=How Flux interacts with this service,enum=managed,enum=external,enum=sync,enum=deferred,enum=takeover,default=managed"`
	Status       string       `yaml:"status,omitempty" json:"status,omitempty" jsonschema:"description=Service deployment status (pending/running/success/failed)"`
	Namespace    string       `yaml:"namespace" json:"namespace,omitempty" jsonschema:"description=Kubernetes namespace for the service"`
	Hostname     string       `yaml:"hostname" json:"hostname,omitempty" jsonschema:"description=Hostname for HTTPRoute configuration"`

	// Image configuration
	ImageRepository string `yaml:"image_repository" json:"image_repository,omitempty" jsonschema:"description=Container image repository"`
	ImageTag        string `yaml:"image_tag" json:"image_tag,omitempty" jsonschema:"description=Container image tag"`

	// Version control fields (for GitOps managed services)
	Release string `yaml:"release" json:"release,omitempty" jsonschema:"description=Release version"`
	Branch  string `yaml:"branch" json:"branch,omitempty" jsonschema:"description=Git branch"`
	Uri     string `yaml:"uri" json:"uri,omitempty" jsonschema:"description=Git repository URI"`

	// GitOps source fields (for managed services)
	GitOpsSourceRepo    string `yaml:"gitops_source_repo" json:"gitops_source_repo,omitempty" jsonschema:"description=GitOps source repository URL"`
	GitOpsSourceRelease string `yaml:"gitops_source_release" json:"gitops_source_release,omitempty" jsonschema:"description=GitOps source release tag"`
	GitOpsSourceBranch  string `yaml:"gitops_source_branch" json:"gitops_source_branch,omitempty" jsonschema:"description=GitOps source branch"`
}

// IsEnabled returns true if the service is enabled.
func (b BaseConfig) IsEnabled() bool {
	return b.Enabled
}

// GetStatus returns the status of the service.
func (b BaseConfig) GetStatus() string {
	return b.Status
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
