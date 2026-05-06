package config

// OpenCenter holds global opencenter-level settings and secrets.
// The AWS credentials here are used by the OpenTofu S3 backend when provided.
type OpenCenter struct {
	AWSAccessKey       string `yaml:"aws_access_key" json:"aws_access_key"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key" json:"aws_secret_access_key"`
}

// ClusterMeta holds high-level metadata about the cluster.
type ClusterMeta struct {
	Name         string `yaml:"name" json:"name" validate:"required,dns1123"`
	Env          string `yaml:"env" json:"env" validate:"omitempty,oneof=dev staging production"`
	Region       string `yaml:"region" json:"region" validate:"required"`
	Status       string `yaml:"status" json:"status"`
	Stage        string `yaml:"stage" json:"stage"`
	Organization string `yaml:"organization" json:"organization" validate:"required"`
	Locked       bool   `yaml:"locked,omitempty" json:"locked,omitempty"`
	LockReason   string `yaml:"lock_reason,omitempty" json:"lock_reason,omitempty"`
}

// SimplifiedOpenCenter represents the opencenter section of the new simplified schema
type SimplifiedOpenCenter struct {
	Meta           ClusterMeta         `yaml:"meta" json:"meta" validate:"required"`
	Secrets        OpenCenterSecrets   `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Infrastructure Infrastructure      `yaml:"infrastructure" json:"infrastructure" validate:"required"`
	Cluster        ClusterConfig       `yaml:"cluster" json:"cluster" validate:"required"`
	GitOps         GitOpsConfig        `yaml:"gitops" json:"gitops" validate:"required"`
	Identity       IdentityConfig      `yaml:"identity,omitempty" json:"identity,omitempty"`
	Gateway        GatewayGlobalConfig `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	OIDC           GlobalOIDCConfig    `yaml:"oidc,omitempty" json:"oidc,omitempty"`
	Storage        StorageConfig       `yaml:"storage,omitempty" json:"storage,omitempty" validate:"required"`
	ManagedService ServiceMap          `yaml:"managed-service" json:"managed-service"`
	Services       ServiceMap          `yaml:"services" json:"services"`
}

const (
	OIDCSourceInternal = "internal"
	OIDCSourceExternal = "external"

	OIDCProviderKeycloak = "keycloak"
	OIDCProviderEntra    = "entra"
	OIDCProviderGeneric  = "generic"
)

// IdentityConfig holds cluster identity provider settings.
type IdentityConfig struct {
	OIDC IdentityOIDCConfig `yaml:"oidc,omitempty" json:"oidc,omitempty" jsonschema:"description=OIDC identity provider settings"`
}

// IdentityOIDCConfig describes where OIDC authentication is provided from.
type IdentityOIDCConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled" jsonschema:"description=Enable OIDC authentication,default=true"`
	Source   string `yaml:"source,omitempty" json:"source,omitempty" jsonschema:"description=OIDC provider source,enum=internal,enum=external,default=internal"`
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty" jsonschema:"description=OIDC provider implementation,enum=keycloak,enum=entra,enum=generic,default=keycloak"`
}

// GatewayGlobalConfig holds global gateway configuration
type GatewayGlobalConfig struct {
	Name          string `yaml:"name" json:"name,omitempty" jsonschema:"description=Default gateway name,default=rmpk-gateway"`
	Namespace     string `yaml:"namespace" json:"namespace,omitempty" jsonschema:"description=Default gateway namespace,default=rackspace-system"`
	ClassName     string `yaml:"class_name" json:"class_name,omitempty" jsonschema:"description=Gateway class name,default=eg"`
	DefaultIssuer string `yaml:"default_issuer" json:"default_issuer,omitempty" jsonschema:"description=Default certificate issuer"`
}

// GlobalOIDCConfig holds global OIDC configuration for services
type GlobalOIDCConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled,omitempty" jsonschema:"description=Enable OIDC authentication,default=true"`
	ClientID   string   `yaml:"client_id" json:"client_id,omitempty" jsonschema:"description=OIDC client ID,default=opencenter"`
	SecretName string   `yaml:"secret_name" json:"secret_name,omitempty" jsonschema:"description=OIDC secret name,default=gateway-oidc-secret"`
	Scopes     []string `yaml:"scopes" json:"scopes,omitempty" jsonschema:"description=OIDC scopes"`
	LogoutPath string   `yaml:"logout_path" json:"logout_path,omitempty" jsonschema:"description=OIDC logout path,default=/logout"`
}


