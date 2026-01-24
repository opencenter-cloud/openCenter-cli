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
	Meta           ClusterMeta       `yaml:"meta" json:"meta" validate:"required"`
	Secrets        OpenCenterSecrets `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Infrastructure Infrastructure    `yaml:"infrastructure" json:"infrastructure" validate:"required"`
	Cluster        ClusterConfig     `yaml:"cluster" json:"cluster" validate:"required"`
	GitOps         GitOpsConfig      `yaml:"gitops" json:"gitops" validate:"required"`
	Storage        StorageConfig     `yaml:"storage,omitempty" json:"storage,omitempty" validate:"required"`
	Talos          *TalosConfig      `yaml:"talos,omitempty" json:"talos,omitempty"`
	ManagedService ServiceMap        `yaml:"managed-service" json:"managed-service"`
	Services       ServiceMap        `yaml:"services" json:"services"`
}

// TalosConfig represents Talos-specific configuration
type TalosConfig struct {
	Enabled        bool                `yaml:"enabled" json:"enabled" jsonschema:"description=Enable Talos Linux provider"`
	Version        string              `yaml:"version" json:"version" jsonschema:"description=Talos Linux version" validate:"required_if=Enabled true,omitempty,semver"`
	ImageURL       string              `yaml:"image_url" json:"image_url" jsonschema:"description=URL to Talos Linux image" validate:"required_if=Enabled true,omitempty,url"`
	ImageSignature string              `yaml:"image_signature" json:"image_signature" jsonschema:"description=Cryptographic signature of Talos image"`
	MachineConfig  TalosMachineConfig  `yaml:"machine_config" json:"machine_config"`
	NetworkConfig  TalosNetworkConfig  `yaml:"network_config" json:"network_config" validate:"required_if=Enabled true"`
	SecurityConfig TalosSecurityConfig `yaml:"security_config" json:"security_config"`
	PulumiConfig   TalosPulumiConfig   `yaml:"pulumi_config" json:"pulumi_config" validate:"required_if=Enabled true"`
}

// TalosMachineConfig holds Talos machine configuration settings
type TalosMachineConfig struct {
	AppArmorEnabled  bool     `yaml:"apparmor_enabled" json:"apparmor_enabled" jsonschema:"description=Enable AppArmor security profiles,default=true"`
	SeccompEnabled   bool     `yaml:"seccomp_enabled" json:"seccomp_enabled" jsonschema:"description=Enable Seccomp security profiles,default=true"`
	DiskEncryption   bool     `yaml:"disk_encryption" json:"disk_encryption" jsonschema:"description=Enable disk encryption with LUKS,default=true"`
	KubePrismEnabled bool     `yaml:"kubeprism_enabled" json:"kubeprism_enabled" jsonschema:"description=Enable KubePrism for internal load balancing,default=true"`
	SystemExtensions []string `yaml:"system_extensions,omitempty" json:"system_extensions,omitempty" jsonschema:"description=List of Talos system extensions to install"`
	LogDestination   string   `yaml:"log_destination,omitempty" json:"log_destination,omitempty" jsonschema:"description=Destination for Talos system logs"`
}

// TalosNetworkConfig holds network topology settings
type TalosNetworkConfig struct {
	ManagementSubnet string   `yaml:"management_subnet" json:"management_subnet" jsonschema:"description=CIDR for management network,default=10.0.1.0/24" validate:"required,cidrv4"`
	ControlSubnet    string   `yaml:"control_subnet" json:"control_subnet" jsonschema:"description=CIDR for control plane network,default=10.0.2.0/24" validate:"required,cidrv4"`
	DataSubnet       string   `yaml:"data_subnet" json:"data_subnet" jsonschema:"description=CIDR for data plane network,default=10.0.3.0/24" validate:"required,cidrv4"`
	WireGuardPort    int      `yaml:"wireguard_port" json:"wireguard_port" jsonschema:"description=UDP port for WireGuard VPN,default=51820" validate:"required,min=1,max=65535"`
	TalosAPIPort     int      `yaml:"talos_api_port" json:"talos_api_port" jsonschema:"description=TCP port for Talos API,default=50000" validate:"required,min=1,max=65535"`
	AllowedCIDRs     []string `yaml:"allowed_cidrs,omitempty" json:"allowed_cidrs,omitempty" jsonschema:"description=List of CIDRs allowed to access cluster" validate:"dive,cidrv4"`
}

// TalosSecurityConfig holds security-related settings
type TalosSecurityConfig struct {
	VTPMEnabled       bool   `yaml:"vtpm_enabled" json:"vtpm_enabled" jsonschema:"description=Enable vTPM for hardware-backed encryption,default=true"`
	BarbicanKeyID     string `yaml:"barbican_key_id,omitempty" json:"barbican_key_id,omitempty" jsonschema:"description=Barbican key ID for encryption"`
	ImageVerification bool   `yaml:"image_verification" json:"image_verification" jsonschema:"description=Enable cryptographic image verification,default=true"`
	MFARequired       bool   `yaml:"mfa_required" json:"mfa_required" jsonschema:"description=Require MFA for administrative access,default=true"`
	AuditLogEnabled   bool   `yaml:"audit_log_enabled" json:"audit_log_enabled" jsonschema:"description=Enable audit logging,default=true"`
}

// TalosPulumiConfig holds Pulumi-specific settings
type TalosPulumiConfig struct {
	StackName         string `yaml:"stack_name" json:"stack_name" jsonschema:"description=Pulumi stack name" validate:"required"`
	SwiftContainer    string `yaml:"swift_container" json:"swift_container" jsonschema:"description=Swift container for Pulumi state" validate:"required"`
	SwiftPrefix       string `yaml:"swift_prefix,omitempty" json:"swift_prefix,omitempty" jsonschema:"description=Swift prefix for state isolation"`
	SecretsPassphrase string `yaml:"secrets_passphrase,omitempty" json:"secrets_passphrase,omitempty" jsonschema:"secret=true,description=Passphrase for Pulumi secrets provider"`
}
