package config

import overlaycfg "github.com/opencenter-cloud/opencenter-cli/internal/config/overlay"

// GitOpsConfig represents GitOps repository and FluxCD configuration.
type GitOpsConfig struct {
	// Repository holds cluster-specific GitOps repository settings.
	Repository GitOpsRepository `yaml:"repository" json:"repository" validate:"required"`

	// BaseRepo holds upstream template repository settings.
	BaseRepo GitOpsBaseRepo `yaml:"base_repo,omitempty" json:"base_repo,omitempty"`

	// Auth holds authentication configuration (SSH or Token).
	Auth GitOpsAuth `yaml:"auth,omitempty" json:"auth,omitempty"`

	// Flux holds FluxCD reconciliation settings.
	Flux GitOpsFlux `yaml:"flux,omitempty" json:"flux,omitempty"`

	// OverlayUnits holds service overlay customization.
	OverlayUnits overlaycfg.UnitsConfig `yaml:"overlay_units,omitempty" json:"overlay_units,omitempty"`
}

// GitOpsRepository holds cluster-specific repository settings.
type GitOpsRepository struct {
	// URL is the remote repository URL (SSH or HTTPS).
	URL string `yaml:"url" json:"url" validate:"required,url"`

	// Branch is the target branch (default: main).
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`

	// Path is the directory within the repo for this cluster's manifests.
	Path string `yaml:"path,omitempty" json:"path,omitempty"`

	// LocalDir is the local checkout directory.
	LocalDir string `yaml:"local_dir,omitempty" json:"local_dir,omitempty"`

	// SecretName is the K8s secret name for repository access.
	SecretName string `yaml:"secret_name,omitempty" json:"secret_name,omitempty"`
}

// GitOpsBaseRepo holds upstream template repository settings.
type GitOpsBaseRepo struct {
	// URL is the base GitOps templates repository.
	URL string `yaml:"url,omitempty" json:"url,omitempty" validate:"omitempty,url"`

	// Release is the version tag to use (e.g., v0.1.0).
	Release string `yaml:"release,omitempty" json:"release,omitempty"`

	// Branch is the branch to track (alternative to Release).
	Branch string `yaml:"branch,omitempty" json:"branch,omitempty"`
}

// GitOpsAuth holds authentication configuration.
type GitOpsAuth struct {
	// SSH holds SSH key authentication settings.
	SSH *GitOpsSSHAuth `yaml:"ssh,omitempty" json:"ssh,omitempty"`

	// Token holds token-based authentication settings.
	Token *GitOpsTokenAuth `yaml:"token,omitempty" json:"token,omitempty"`
}

// GitOpsSSHAuth holds SSH key authentication settings.
type GitOpsSSHAuth struct {
	// PrivateKey is the path to the SSH private key file.
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty"`

	// PublicKey is the path to the SSH public key file.
	PublicKey string `yaml:"public_key,omitempty" json:"public_key,omitempty"`
}

// GitOpsTokenAuth holds token-based authentication settings.
type GitOpsTokenAuth struct {
	// Provider is the Git provider: github, gitlab, gitea.
	Provider string `yaml:"provider" json:"provider" validate:"required,oneof=github gitlab gitea"`

	// TokenFile is the path to the file containing the access token.
	// Required when using token authentication for bootstrap.
	TokenFile string `yaml:"token_file,omitempty" json:"token_file,omitempty"`

	// Owner is the repository owner (username or organization).
	// If empty, extracted from repository URL.
	Owner string `yaml:"owner,omitempty" json:"owner,omitempty"`
}

// GitOpsFlux holds FluxCD reconciliation settings.
type GitOpsFlux struct {
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Prune    bool   `yaml:"prune" json:"prune"`
}
