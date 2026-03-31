package overlay

// UnitsConfig defines cluster-scoped overlay units that are not tied directly
// to the standard services or managed-service maps.
type UnitsConfig struct {
	CustomerManaged CustomerManagedConfig `yaml:"customer_managed,omitempty" json:"customer_managed,omitempty"`
	SOPS            SOPSGenerationConfig  `yaml:"sops,omitempty" json:"sops,omitempty"`
}

// CustomerManagedConfig controls generation of the customer-managed overlay
// layer and the Flux resources that point to a customer repository.
type CustomerManagedConfig struct {
	Enabled        bool                           `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	RepositoryName string                         `yaml:"repository_name,omitempty" json:"repository_name,omitempty"`
	RepositoryURL  string                         `yaml:"repository_url,omitempty" json:"repository_url,omitempty"`
	Branch         string                         `yaml:"branch,omitempty" json:"branch,omitempty"`
	Interval       string                         `yaml:"interval,omitempty" json:"interval,omitempty"`
	FluxNamePrefix string                         `yaml:"flux_name_prefix,omitempty" json:"flux_name_prefix,omitempty"`
	SecretName     string                         `yaml:"secret_name,omitempty" json:"secret_name,omitempty"`
	EmitSecret     bool                           `yaml:"emit_secret,omitempty" json:"emit_secret,omitempty"`
	Kustomizations []CustomerManagedKustomization `yaml:"kustomizations,omitempty" json:"kustomizations,omitempty"`
}

// CustomerManagedKustomization defines one Flux Kustomization emitted for a
// customer-managed repository.
type CustomerManagedKustomization struct {
	Name      string   `yaml:"name,omitempty" json:"name,omitempty"`
	Path      string   `yaml:"path,omitempty" json:"path,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
}

// SOPSGenerationConfig controls generation of overlay-local .sops.yaml files.
type SOPSGenerationConfig struct {
	Enabled bool                 `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Rules   []SOPSGenerationRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// SOPSGenerationRule defines one creation rule entry in .sops.yaml.
type SOPSGenerationRule struct {
	PathRegex      string   `yaml:"path_regex,omitempty" json:"path_regex,omitempty"`
	AgeRecipients  []string `yaml:"age_recipients,omitempty" json:"age_recipients,omitempty"`
	EncryptedRegex string   `yaml:"encrypted_regex,omitempty" json:"encrypted_regex,omitempty"`
}

// Secrets defines cluster-scoped secret inputs used by overlay units.
type Secrets struct {
	CustomerManaged CustomerManagedSecrets `yaml:"customer_managed,omitempty" json:"customer_managed,omitempty"`
}

// CustomerManagedSecrets provides the secret material for customer-managed
// GitRepository authentication when the overlay is configured to emit it.
type CustomerManagedSecrets struct {
	Identity    string `yaml:"identity,omitempty" json:"identity,omitempty"`
	IdentityPub string `yaml:"identity_pub,omitempty" json:"identity_pub,omitempty"`
	KnownHosts  string `yaml:"known_hosts,omitempty" json:"known_hosts,omitempty"`
}
