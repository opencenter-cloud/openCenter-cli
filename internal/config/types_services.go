package config

// BaseServiceCfg contains common fields for all services
type BaseServiceCfg struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Status    string `yaml:"status,omitempty" json:"status,omitempty" jsonschema:"description=Service deployment status (pending/running/success/failed)"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" jsonschema:"description=Kubernetes namespace for the service"`
	Hostname  string `yaml:"hostname,omitempty" json:"hostname,omitempty" jsonschema:"description=Hostname for HTTPRoute configuration"`
}

// ServiceCfg captures the on/off toggle plus optional metadata for a service.
// For backward compatibility, this still contains all fields but they should be
// migrated to specific service types over time.
type ServiceCfg struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Status  string `yaml:"status,omitempty" json:"status,omitempty" jsonschema:"description=Service deployment status (pending/running/success/failed)"`

	// Common service fields
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" jsonschema:"description=Kubernetes namespace for the service"`
	Hostname  string `yaml:"hostname,omitempty" json:"hostname,omitempty" jsonschema:"description=Hostname for HTTPRoute configuration"`

	// Image configuration
	ImageRepository string `yaml:"image_repository,omitempty" json:"image_repository,omitempty" jsonschema:"description=Container image repository"`
	ImageTag        string `yaml:"image_tag,omitempty" json:"image_tag,omitempty" jsonschema:"description=Container image tag"`

	// Version control fields (for GitOps managed services)
	Release string `yaml:"release,omitempty" json:"release,omitempty" jsonschema:"description=Release version"`
	Branch  string `yaml:"branch,omitempty" json:"branch,omitempty" jsonschema:"description=Git branch"`
	Uri     string `yaml:"uri,omitempty" json:"uri,omitempty" jsonschema:"description=Git repository URI"`

	// GitOps source fields (for managed services)
	GitOpsSourceRepo    string `yaml:"gitops_source_repo,omitempty" json:"gitops_source_repo,omitempty" jsonschema:"description=GitOps source repository URL"`
	GitOpsSourceRelease string `yaml:"gitops_source_release,omitempty" json:"gitops_source_release,omitempty" jsonschema:"description=GitOps source release tag"`
	GitOpsSourceBranch  string `yaml:"gitops_source_branch,omitempty" json:"gitops_source_branch,omitempty" jsonschema:"description=GitOps source branch"`

	// Cert-manager fields (deprecated: should be in cert-manager specific config)
	LetsEncryptServer string `yaml:"letsencrypt_server,omitempty" json:"letsencrypt_server,omitempty" jsonschema:"description=LetsEncrypt ACME server URL (deprecated)"`

	// Loki fields (deprecated: should be in loki specific config)
	LokiStorageType  string `yaml:"loki_storage_type,omitempty" json:"loki_storage_type,omitempty" jsonschema:"description=Loki storage backend type (deprecated)"`
	LokiBucketName   string `yaml:"loki_bucket_name,omitempty" json:"loki_bucket_name,omitempty" jsonschema:"description=Loki storage bucket/container name (deprecated)"`
	LokiVolumeSize   int    `yaml:"loki_volume_size,omitempty" json:"loki_volume_size,omitempty" jsonschema:"description=Loki persistent volume size in GB (deprecated)"`
	LokiStorageClass string `yaml:"loki_storage_class,omitempty" json:"loki_storage_class,omitempty" jsonschema:"description=Loki storage class (deprecated)"`

	// Swift storage fields (deprecated: should be in loki specific config)
	SwiftAuthURL                 string `yaml:"swift_auth_url,omitempty" json:"swift_auth_url,omitempty" jsonschema:"description=Swift Keystone V3 authentication URL (deprecated)"`
	SwiftRegion                  string `yaml:"swift_region,omitempty" json:"swift_region,omitempty" jsonschema:"description=Swift region name (deprecated)"`
	SwiftAuthVersion             int    `yaml:"swift_auth_version,omitempty" json:"swift_auth_version,omitempty" jsonschema:"description=Swift authentication version (deprecated)"`
	SwiftApplicationCredentialID string `yaml:"swift_application_credential_id,omitempty" json:"swift_application_credential_id,omitempty" jsonschema:"description=Swift application credential ID (deprecated)"`
	SwiftContainerName           string `yaml:"swift_container_name,omitempty" json:"swift_container_name,omitempty" jsonschema:"description=Swift container name for Loki logs (deprecated)"`
	SwiftUserDomainName          string `yaml:"swift_user_domain_name,omitempty" json:"swift_user_domain_name,omitempty" jsonschema:"description=Swift user domain name (deprecated)"`

	// S3 storage fields (deprecated: should be in loki specific config)
	LokiS3Endpoint       string `yaml:"loki_s3_endpoint,omitempty" json:"loki_s3_endpoint,omitempty" jsonschema:"description=S3 endpoint URL (deprecated)"`
	LokiS3Region         string `yaml:"loki_s3_region,omitempty" json:"loki_s3_region,omitempty" jsonschema:"description=S3 region (deprecated)"`
	LokiS3ForcePathStyle bool   `yaml:"loki_s3_force_path_style,omitempty" json:"loki_s3_force_path_style,omitempty" jsonschema:"description=Force S3 path style (deprecated)"`
	LokiS3Insecure       bool   `yaml:"loki_s3_insecure,omitempty" json:"loki_s3_insecure,omitempty" jsonschema:"description=Allow insecure S3 connections (deprecated)"`

	// Velero fields (deprecated: should be in velero specific config)
	VeleroBackupBucket string `yaml:"velero_backup_bucket,omitempty" json:"velero_backup_bucket,omitempty" jsonschema:"description=Velero backup bucket name (deprecated)"`
	VeleroRegion       string `yaml:"velero_region,omitempty" json:"velero_region,omitempty" jsonschema:"description=Velero backup region (deprecated)"`

	// Keycloak fields (deprecated: should be in keycloak specific config)
	KeycloakRealm       string `yaml:"keycloak_realm,omitempty" json:"keycloak_realm,omitempty" jsonschema:"description=Keycloak realm name (deprecated)"`
	KeycloakFrontendURL string `yaml:"keycloak_frontend_url,omitempty" json:"keycloak_frontend_url,omitempty" jsonschema:"description=Keycloak frontend URL (deprecated)"`
	KeycloakClientID    string `yaml:"keycloak_client_id,omitempty" json:"keycloak_client_id,omitempty" jsonschema:"description=Keycloak client ID (deprecated)"`

	// Grafana/Prometheus fields (deprecated: should be in prometheus-stack specific config)
	GrafanaVolumeSize        int    `yaml:"grafana_volume_size,omitempty" json:"grafana_volume_size,omitempty" jsonschema:"description=Grafana persistent volume size in GB (deprecated)"`
	GrafanaStorageClass      string `yaml:"grafana_storage_class,omitempty" json:"grafana_storage_class,omitempty" jsonschema:"description=Grafana storage class (deprecated)"`
	PrometheusVolumeSize     int    `yaml:"prometheus_volume_size,omitempty" json:"prometheus_volume_size,omitempty" jsonschema:"description=Prometheus persistent volume size in GB (deprecated)"`
	PrometheusStorageClass   string `yaml:"prometheus_storage_class,omitempty" json:"prometheus_storage_class,omitempty" jsonschema:"description=Prometheus storage class (deprecated)"`
	AlertmanagerVolumeSize   int    `yaml:"alertmanager_volume_size,omitempty" json:"alertmanager_volume_size,omitempty" jsonschema:"description=Alertmanager persistent volume size in GB (deprecated)"`
	AlertmanagerStorageClass string `yaml:"alertmanager_storage_class,omitempty" json:"alertmanager_storage_class,omitempty" jsonschema:"description=Alertmanager storage class (deprecated)"`
	WebhookURL               string `yaml:"webhook_url,omitempty" json:"webhook_url,omitempty" jsonschema:"description=Webhook URL for alerting integrations (deprecated)"`

	// Headlamp fields (deprecated: should be in headlamp specific config)
	HeadlampOIDCIssuerURL string `yaml:"headlamp_oidc_issuer_url,omitempty" json:"headlamp_oidc_issuer_url,omitempty" jsonschema:"description=Headlamp OIDC issuer URL (deprecated)"`
	HeadlampOIDCClientID  string `yaml:"headlamp_oidc_client_id,omitempty" json:"headlamp_oidc_client_id,omitempty" jsonschema:"description=Headlamp OIDC client ID (deprecated)"`

	// Calico fields (deprecated: should be in calico specific config)
	CalicoKubeAPIServer string `yaml:"calico_kube_api_server,omitempty" json:"calico_kube_api_server,omitempty" jsonschema:"description=Calico Kubernetes API server address (deprecated)"`
}

// Specific service configuration types for services that need additional fields

// LokiServiceCfg extends BaseServiceCfg with Loki-specific configuration
type LokiServiceCfg struct {
	BaseServiceCfg `yaml:",inline"`

	// Storage configuration
	StorageType  string `yaml:"loki_storage_type,omitempty" json:"loki_storage_type,omitempty" jsonschema:"description=Loki storage backend type (s3 or swift),enum=s3,enum=swift,default=swift"`
	BucketName   string `yaml:"loki_bucket_name,omitempty" json:"loki_bucket_name,omitempty" jsonschema:"description=Loki storage bucket/container name"`
	VolumeSize   int    `yaml:"loki_volume_size,omitempty" json:"loki_volume_size,omitempty" jsonschema:"description=Loki persistent volume size in GB"`
	StorageClass string `yaml:"loki_storage_class,omitempty" json:"loki_storage_class,omitempty" jsonschema:"description=Loki storage class"`

	// Swift storage fields
	SwiftAuthURL                 string `yaml:"swift_auth_url,omitempty" json:"swift_auth_url,omitempty" jsonschema:"description=Swift Keystone V3 authentication URL (must end in /v3)"`
	SwiftRegion                  string `yaml:"swift_region,omitempty" json:"swift_region,omitempty" jsonschema:"description=Swift region name"`
	SwiftAuthVersion             int    `yaml:"swift_auth_version,omitempty" json:"swift_auth_version,omitempty" jsonschema:"description=Swift authentication version,default=3"`
	SwiftApplicationCredentialID string `yaml:"swift_application_credential_id,omitempty" json:"swift_application_credential_id,omitempty" jsonschema:"description=Swift application credential ID (UUID)"`
	SwiftContainerName           string `yaml:"swift_container_name,omitempty" json:"swift_container_name,omitempty" jsonschema:"description=Swift container name for Loki logs"`
	SwiftUserDomainName          string `yaml:"swift_user_domain_name,omitempty" json:"swift_user_domain_name,omitempty" jsonschema:"description=Swift user domain name"`
	SwiftDomainName              string `yaml:"swift_domain_name,omitempty" json:"swift_domain_name,omitempty" jsonschema:"description=Swift domain name"`

	// S3 storage fields
	S3Endpoint       string `yaml:"loki_s3_endpoint,omitempty" json:"loki_s3_endpoint,omitempty" jsonschema:"description=S3 endpoint URL"`
	S3Region         string `yaml:"loki_s3_region,omitempty" json:"loki_s3_region,omitempty" jsonschema:"description=S3 region"`
	S3ForcePathStyle bool   `yaml:"loki_s3_force_path_style,omitempty" json:"loki_s3_force_path_style,omitempty" jsonschema:"description=Force S3 path style"`
	S3Insecure       bool   `yaml:"loki_s3_insecure,omitempty" json:"loki_s3_insecure,omitempty" jsonschema:"description=Allow insecure S3 connections"`
}

// PrometheusStackServiceCfg extends BaseServiceCfg with Prometheus stack configuration
type PrometheusStackServiceCfg struct {
	BaseServiceCfg `yaml:",inline"`

	// Storage configuration for each component
	GrafanaVolumeSize        int    `yaml:"grafana_volume_size,omitempty" json:"grafana_volume_size,omitempty" jsonschema:"description=Grafana persistent volume size in GB"`
	GrafanaStorageClass      string `yaml:"grafana_storage_class,omitempty" json:"grafana_storage_class,omitempty" jsonschema:"description=Grafana storage class"`
	PrometheusVolumeSize     int    `yaml:"prometheus_volume_size,omitempty" json:"prometheus_volume_size,omitempty" jsonschema:"description=Prometheus persistent volume size in GB"`
	PrometheusStorageClass   string `yaml:"prometheus_storage_class,omitempty" json:"prometheus_storage_class,omitempty" jsonschema:"description=Prometheus storage class"`
	AlertmanagerVolumeSize   int    `yaml:"alertmanager_volume_size,omitempty" json:"alertmanager_volume_size,omitempty" jsonschema:"description=Alertmanager persistent volume size in GB"`
	AlertmanagerStorageClass string `yaml:"alertmanager_storage_class,omitempty" json:"alertmanager_storage_class,omitempty" jsonschema:"description=Alertmanager storage class"`
	WebhookURL               string `yaml:"webhook_url,omitempty" json:"webhook_url,omitempty" jsonschema:"description=Webhook URL for alerting integrations"`
}

// KeycloakServiceCfg extends BaseServiceCfg with Keycloak-specific configuration
type KeycloakServiceCfg struct {
	BaseServiceCfg `yaml:",inline"`

	Realm       string `yaml:"keycloak_realm,omitempty" json:"keycloak_realm,omitempty" jsonschema:"description=Keycloak realm name"`
	FrontendURL string `yaml:"keycloak_frontend_url,omitempty" json:"keycloak_frontend_url,omitempty" jsonschema:"description=Keycloak frontend URL"`
	ClientID    string `yaml:"keycloak_client_id,omitempty" json:"keycloak_client_id,omitempty" jsonschema:"description=Keycloak client ID"`
}

// HeadlampServiceCfg extends BaseServiceCfg with Headlamp-specific configuration
type HeadlampServiceCfg struct {
	BaseServiceCfg `yaml:",inline"`

	OIDCIssuerURL string `yaml:"headlamp_oidc_issuer_url,omitempty" json:"headlamp_oidc_issuer_url,omitempty" jsonschema:"description=Headlamp OIDC issuer URL"`
	OIDCClientID  string `yaml:"headlamp_oidc_client_id,omitempty" json:"headlamp_oidc_client_id,omitempty" jsonschema:"description=Headlamp OIDC client ID"`
}

// VeleroServiceCfg extends BaseServiceCfg with Velero-specific configuration
type VeleroServiceCfg struct {
	BaseServiceCfg `yaml:",inline"`

	BackupBucket string `yaml:"velero_backup_bucket,omitempty" json:"velero_backup_bucket,omitempty" jsonschema:"description=Velero backup bucket name"`
	Region       string `yaml:"velero_region,omitempty" json:"velero_region,omitempty" jsonschema:"description=Velero backup region"`
}

// CertManagerServiceCfg extends BaseServiceCfg with cert-manager configuration
type CertManagerServiceCfg struct {
	BaseServiceCfg `yaml:",inline"`

	LetsEncryptServer string `yaml:"letsencrypt_server,omitempty" json:"letsencrypt_server,omitempty" jsonschema:"description=LetsEncrypt ACME server URL"`
	Email             string `yaml:"email,omitempty" json:"email,omitempty" jsonschema:"description=Email for LetsEncrypt registration"`
}

// VSphereCSIServiceCfg extends BaseServiceCfg with vSphere CSI configuration
type VSphereCSIServiceCfg struct {
	BaseServiceCfg `yaml:",inline"`

	ImageRepository string `yaml:"image_repository,omitempty" json:"image_repository,omitempty" jsonschema:"description=vSphere CSI image repository"`
	ImageTag        string `yaml:"image_tag,omitempty" json:"image_tag,omitempty" jsonschema:"description=vSphere CSI image tag"`
}

// CalicoServiceCfg extends BaseServiceCfg with Calico-specific configuration
type CalicoServiceCfg struct {
	BaseServiceCfg `yaml:",inline"`

	KubeAPIServer string `yaml:"calico_kube_api_server,omitempty" json:"calico_kube_api_server,omitempty" jsonschema:"description=Calico Kubernetes API server address"`
}
