package config

// Secrets holds paths or settings for secret management tools.
type Secrets struct {
	SopsAgeKeyFile string `yaml:"sops_age_key_file" json:"sops_age_key_file"`
	SSHKey         SSHKey `yaml:"ssh_key" json:"ssh_key"`

	// Global secrets organized by scope
	Global GlobalSecrets `yaml:"global" json:"global"`

	// Service-specific secrets
	CertManager CertManagerSecrets `yaml:"cert_manager" json:"cert_manager"`
	Loki        LokiSecrets        `yaml:"loki" json:"loki"`
	Keycloak    KeycloakSecrets    `yaml:"keycloak" json:"keycloak"`
	Headlamp    HeadlampSecrets    `yaml:"headlamp" json:"headlamp"`
	WeaveGitOps WeaveGitOpsSecrets `yaml:"weave_gitops" json:"weave_gitops"`
	Grafana     GrafanaSecrets     `yaml:"grafana" json:"grafana"`
	Tempo       TempoSecrets       `yaml:"tempo" json:"tempo"`
	AlertProxy  AlertProxySecrets  `yaml:"alert_proxy" json:"alert_proxy"`
	VSphereCsi  VSphereCsiSecrets  `yaml:"vsphere_csi" json:"vsphere_csi"`

	// Deprecated: Use Global.AWS.Infrastructure instead
	AWS AWSSecrets `yaml:"aws,omitempty" json:"aws,omitempty" jsonschema:"deprecated=true,description=Deprecated: Use global.aws.infrastructure instead"`
}

// SSHKey holds SSH key configuration for cluster access
type SSHKey struct {
	Private string `yaml:"private" json:"private"`
	Public  string `yaml:"public" json:"public"`
	Cypher  string `yaml:"cypher" json:"cypher"`
}

// OpenCenterSecrets holds the configuration for the secrets management backend.
type OpenCenterSecrets struct {
	Backend  string         `yaml:"backend,omitempty" json:"backend,omitempty"`
	Barbican BarbicanConfig `yaml:"barbican,omitempty" json:"barbican,omitempty"`
}

// BarbicanConfig holds the configuration for the Barbican secrets backend.
type BarbicanConfig struct {
	AuthURL           string `yaml:"auth_url,omitempty" json:"auth_url,omitempty"`
	ProjectID         string `yaml:"project_id,omitempty" json:"project_id,omitempty"`
	Region            string `yaml:"region,omitempty" json:"region,omitempty"`
	UserDomainName    string `yaml:"user_domain_name,omitempty" json:"user_domain_name,omitempty"`
	ProjectDomainName string `yaml:"project_domain_name,omitempty" json:"project_domain_name,omitempty"`
	CACert            string `yaml:"ca_cert,omitempty" json:"ca_cert,omitempty"`
}

// GlobalSecrets holds global secrets organized by scope and provider
type GlobalSecrets struct {
	AWS AWSGlobalSecrets `yaml:"aws" json:"aws"`
}

// AWSGlobalSecrets holds AWS credentials organized by purpose
type AWSGlobalSecrets struct {
	Infrastructure AWSSecrets `yaml:"infrastructure" json:"infrastructure"`
	Application    AWSSecrets `yaml:"application" json:"application"`
}

// AWSSecrets holds AWS credentials that can be used as fallback for services
type AWSSecrets struct {
	AccessKey       string `yaml:"access_key" json:"access_key" jsonschema:"secret=true,description=AWS access key"`
	SecretAccessKey string `yaml:"secret_access_key" json:"secret_access_key" jsonschema:"secret=true,description=AWS secret access key"`
	Region          string `yaml:"region,omitempty" json:"region,omitempty" jsonschema:"description=AWS region"`
}

// CertManagerSecrets holds cert-manager secret values
type CertManagerSecrets struct {
	AWSAccessKey       string `yaml:"aws_access_key" json:"aws_access_key" jsonschema:"secret=true,description=AWS access key for Route53 DNS validation"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key" json:"aws_secret_access_key" jsonschema:"secret=true,description=AWS secret access key for Route53 DNS validation"`
}

// LokiSecrets holds Loki secret values
type LokiSecrets struct {
	// Swift secrets
	SwiftPassword                    string `yaml:"swift_password" json:"swift_password" jsonschema:"secret=true,description=Swift storage password (deprecated: use application credentials)"`
	SwiftApplicationCredentialSecret string `yaml:"swift_application_credential_secret" json:"swift_application_credential_secret" jsonschema:"secret=true,description=Swift application credential secret"`

	// S3 secrets
	S3AccessKeyID     string `yaml:"s3_access_key_id" json:"s3_access_key_id" jsonschema:"secret=true,description=S3 access key ID"`
	S3SecretAccessKey string `yaml:"s3_secret_access_key" json:"s3_secret_access_key" jsonschema:"secret=true,description=S3 secret access key"`
}

// KeycloakSecrets holds Keycloak secret values
type KeycloakSecrets struct {
	ClientSecret  string `yaml:"client_secret" json:"client_secret" jsonschema:"secret=true,description=Keycloak OIDC client secret"`
	AdminPassword string `yaml:"admin_password" json:"admin_password" jsonschema:"secret=true,description=Keycloak admin user password"`
}

// HeadlampSecrets holds Headlamp secret values
type HeadlampSecrets struct {
	OIDCClientSecret string `yaml:"oidc_client_secret" json:"oidc_client_secret" jsonschema:"secret=true,description=Headlamp OIDC client secret"`
}

// WeaveGitOpsSecrets holds Weave GitOps secret values
type WeaveGitOpsSecrets struct {
	Password     string `yaml:"password" json:"password" jsonschema:"secret=true,description=Weave GitOps admin password"`
	PasswordHash string `yaml:"password_hash" json:"password_hash" jsonschema:"secret=true,description=Weave GitOps admin password hash (bcrypt)"`
}

// GrafanaSecrets holds Grafana secret values
type GrafanaSecrets struct {
	AdminPassword string `yaml:"admin_password" json:"admin_password" jsonschema:"secret=true,description=Grafana admin password"`
}

// TempoSecrets holds Tempo secret values
type TempoSecrets struct {
	AccessKey string `yaml:"access_key" json:"access_key" jsonschema:"secret=true,description=Tempo S3 access key"`
	SecretKey string `yaml:"secret_key" json:"secret_key" jsonschema:"secret=true,description=Tempo S3 secret key"`
}

// AlertProxySecrets holds alert-proxy secret values
type AlertProxySecrets struct {
	CoreDeviceId        string `yaml:"core_device_id" json:"core_device_id" jsonschema:"secret=true,description=Alert proxy core device ID"`
	AccountServiceToken string `yaml:"account_service_token" json:"account_service_token" jsonschema:"secret=true,description=Alert proxy account service token"`
	CoreAccountNumber   string `yaml:"core_account_number" json:"core_account_number" jsonschema:"secret=true,description=Alert proxy core account number"`
}

// VSphereCsiSecrets holds vSphere CSI secret values
type VSphereCsiSecrets struct {
	VCenterHost  string `yaml:"vcenter_host" json:"vcenter_host" jsonschema:"secret=true,description=vCenter server hostname or IP address"`
	Username     string `yaml:"username" json:"username" jsonschema:"secret=true,description=vCenter username"`
	Password     string `yaml:"password" json:"password" jsonschema:"secret=true,description=vCenter password"`
	Datacenters  string `yaml:"datacenters" json:"datacenters" jsonschema:"secret=true,description=Comma-separated list of datacenters"`
	InsecureFlag string `yaml:"insecure_flag" json:"insecure_flag" jsonschema:"secret=true,description=Skip SSL certificate verification (true/false)"`
	Port         string `yaml:"port" json:"port" jsonschema:"secret=true,description=vCenter port (default: 443)"`
}
