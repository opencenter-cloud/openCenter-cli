// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the simplified root configuration for a cluster based on the new schema.
// The structure matches the testdata/schema.yaml format with opencenter, opentofu, cloud, and secrets sections.
// IAC field is included for backward compatibility with existing templates.
type Config struct {
	OpenCenter SimplifiedOpenCenter `yaml:"opencenter" json:"opencenter"`
	OpenTofu   SimplifiedOpenTofu   `yaml:"opentofu" json:"opentofu"`
	Secrets    Secrets              `yaml:"secrets" json:"secrets"`
	Networking Networking           `yaml:"networking,omitempty" json:"networking,omitempty"`
	Overrides  map[string]any       `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	IAC        IAC                  `yaml:"-" json:"-"` // Hidden from YAML output, for template compatibility
}

// ClusterMeta holds high-level metadata about the cluster.
type ClusterMeta struct {
	Name         string `yaml:"name" json:"name"`
	Env          string `yaml:"env" json:"env"`
	Region       string `yaml:"region" json:"region"`
	Status       string `yaml:"status" json:"status"`
	Stage        string `yaml:"stage" json:"stage"`
	Organization string `yaml:"organization" json:"organization"`
}

// Cluster Stages
const (
	StageInit      = "init"
	StagePreflight = "preflight"
	StageSetup     = "setup"
	StageBootstrap = "bootstrap"
	StageValidate  = "validate"
	StageDestroy   = "destroy"
	StageRender    = "render"
	StagePlan      = "plan"
	StageApply     = "apply"
)

// Cluster Statuses
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// OpenTofu holds OpenTofu-specific settings.
type OpenTofu struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Path    string      `yaml:"path" json:"path"`
	Backend TofuBackend `yaml:"backend" json:"backend"`
}

// TofuBackend describes the state backend configuration for OpenTofu.
// Type can be "local" or "s3". When "local", Backend.Local.Path is used.
// When "s3", Backend.S3 fields are used.
type TofuBackend struct {
	Type  string    `yaml:"type" json:"type"`
	Local TofuLocal `yaml:"local" json:"local"`
	S3    TofuS3    `yaml:"s3" json:"s3"`
}

type TofuLocal struct {
	Path string `yaml:"path" json:"path"`
}

type TofuS3 struct {
	Bucket   string `yaml:"bucket" json:"bucket"`
	Key      string `yaml:"key" json:"key"`
	Region   string `yaml:"region" json:"region"`
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Profile  string `yaml:"profile,omitempty" json:"profile,omitempty"`
	Encrypt  bool   `yaml:"encrypt,omitempty" json:"encrypt,omitempty"`
}

// OpenCenter holds global openCenter-level settings and secrets.
// The AWS credentials here are used by the OpenTofu S3 backend when provided.
type OpenCenter struct {
	AWSAccessKey       string `yaml:"aws_access_key" json:"aws_access_key"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key" json:"aws_secret_access_key"`
}

// Ansible holds Ansible-specific settings.
type Ansible struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	Path      string   `yaml:"path" json:"path"`
	Inventory string   `yaml:"inventory,omitempty" json:"inventory,omitempty"`
	Playbooks []string `yaml:"playbooks,omitempty" json:"playbooks,omitempty"`
}

// GitOpsConfig holds configuration related to GitOps scaffolding and repositories.
type GitOpsConfig struct {
	GitDir    string     `yaml:"git_dir" json:"git_dir"`
	GitURL    string     `yaml:"git_url" json:"git_url"`
	GitSSHKey string     `yaml:"git_ssh_key,omitempty" json:"git_ssh_key,omitempty"`
	GitSSHPub string     `yaml:"git_ssh_pub,omitempty" json:"git_ssh_pub,omitempty"`
	GitBranch string     `yaml:"git_branch,omitempty" json:"git_branch,omitempty"`
	Release   string     `yaml:"release,omitempty" json:"release,omitempty"`
	Branch    string     `yaml:"branch,omitempty" json:"branch,omitempty"`
	Uri       string     `yaml:"uri,omitempty" json:"uri,omitempty"`
	Flux      GitOpsFlux `yaml:"flux,omitempty" json:"flux,omitempty"`

	// New fields for GitOps base repository configuration
	GitOpsBaseRepo    string `yaml:"gitops_base_repo,omitempty" json:"gitops_base_repo,omitempty" jsonschema:"description=URL of the GitOps base repository"`
	GitOpsBaseRelease string `yaml:"gitops_base_release,omitempty" json:"gitops_base_release,omitempty" jsonschema:"description=Release tag of the GitOps base repository"`
	GitOpsBranch      string `yaml:"gitops_branch,omitempty" json:"gitops_branch,omitempty" jsonschema:"description=Branch of the GitOps base repository,default=main"`
}

// GitOpsFlux holds optional FluxCD settings for reconciliation behavior.
type GitOpsFlux struct {
	Interval string `yaml:"interval" json:"interval"`
	Prune    bool   `yaml:"prune" json:"prune"`
}

// Kubernetes groups settings for the Kubernetes cluster.
// It nests further objects for counts, images, flavors, and networking.
// Default values are applied at load time.
// IAC groups settings for infrastructure-as-code driven cluster provisioning.
// It retains the detailed node/layout fields and adds engine/stack selectors.
type IAC struct {
	// Main contains the values for the Terraform locals (rendered into main.tf)
	Main map[string]any `yaml:"main,omitempty" json:"main,omitempty"`
	// Modules contains per-module attribute maps (rendered into main.tf)
	Modules map[string]any `yaml:"modules,omitempty" json:"modules,omitempty"`
}

// Storage describes block volume storage sizes and types for various node roles.
type Storage struct {
	MasterNodeBFV        BFV `yaml:"master_node_bfv" json:"master_node_bfv"`
	WorkerNodeBFV        BFV `yaml:"worker_node_bfv" json:"worker_node_bfv"`
	WorkerNodeBFVWindows BFV `yaml:"worker_node_bfv_windows" json:"worker_node_bfv_windows"`
}

// BFV describes the block volume size and type.
type BFV struct {
	Size int    `yaml:"size" json:"size"`
	Type string `yaml:"type" json:"type"`
}

// Windows holds Windows-specific settings.
type Windows struct {
	User          string `yaml:"user" json:"user"`
	AdminPassword string `yaml:"admin_password" json:"admin_password"`
}

// Networking groups network settings and options around VRRP and service networks.
type Networking struct {
	SubnetNodes          string   `yaml:"subnet_nodes" json:"subnet_nodes"`
	AllocationPoolStart  string   `yaml:"allocation_pool_start" json:"allocation_pool_start"`
	AllocationPoolEnd    string   `yaml:"allocation_pool_end" json:"allocation_pool_end"`
	VRRPEnabled          bool     `yaml:"vrrp_enabled" json:"vrrp_enabled"`
	VRRPIP               string   `yaml:"vrrp_ip" json:"vrrp_ip"`
	SubnetServices       string   `yaml:"subnet_services" json:"subnet_services"`
	SubnetPods           string   `yaml:"subnet_pods" json:"subnet_pods"`
	UseOctavia           bool     `yaml:"use_octavia" json:"use_octavia"`
	LoadbalancerProvider string   `yaml:"loadbalancer_provider" json:"loadbalancer_provider"`
	UseDesignate         bool     `yaml:"use_designate" json:"use_designate"`
	DNSZoneName          string   `yaml:"dns_zone_name" json:"dns_zone_name"`
	DNSNameservers       []string `yaml:"dns_nameservers" json:"dns_nameservers"`
	VLAN                 VLAN     `yaml:"vlan" json:"vlan"`
}

// VLAN describes VLAN settings for the cluster.
type VLAN struct {
	ID       string `yaml:"id" json:"id"`
	MTU      int    `yaml:"mtu" json:"mtu"`
	Provider string `yaml:"provider" json:"provider"`
}

// Cloud holds provider-specific configuration. Currently, only OpenStack is supported.
type Cloud struct {
	Provider  string         `yaml:"provider" json:"provider"`
	OpenStack OpenStackCloud `yaml:"openstack" json:"openstack"`
	AWS       AWSCloud       `yaml:"aws" json:"aws"`
}

// OpenStackCloud contains options for connecting to an OpenStack deployment.
type OpenStackCloud struct {
	AuthURL                 string `yaml:"auth_url" json:"auth_url"`
	Insecure                bool   `yaml:"insecure" json:"insecure"`
	Region                  string `yaml:"region" json:"region"`
	UserName                string `yaml:"user_name" json:"user_name"`
	UserPassword            string `yaml:"user_password" json:"user_password"`
	AdminPassword           string `yaml:"admin_password" json:"admin_password"`
	ProjectDomainName       string `yaml:"project_domain_name" json:"project_domain_name"`
	UserDomainName          string `yaml:"user_domain_name" json:"user_domain_name"`
	TenantName              string `yaml:"tenant_name" json:"tenant_name"`
	AvailabilityZone        string `yaml:"availability_zone" json:"availability_zone"`
	FloatingIPPool          string `yaml:"floatingip_pool" json:"floatingip_pool"`
	RouterExternalNetworkID string `yaml:"router_external_network_id" json:"router_external_network_id"`
	DisableBastion          bool   `yaml:"disable_bastion" json:"disable_bastion"`
	CA                      string `yaml:"ca" json:"ca"`
	ExternalNetwork         string `yaml:"external_network" json:"external_network"`
	UseOctavia              bool   `yaml:"use_octavia" json:"use_octavia"`
	VRRPIP                  string `yaml:"vrrp_ip" json:"vrrp_ip"`
}

// AWSCloud contains options for connecting to AWS environments.
type AWSCloud struct {
	Profile        string   `yaml:"profile" json:"profile"`
	Region         string   `yaml:"region" json:"region"`
	VPCID          string   `yaml:"vpc_id" json:"vpc_id"`
	PrivateSubnets []string `yaml:"private_subnets" json:"private_subnets"`
	PublicSubnets  []string `yaml:"public_subnets" json:"public_subnets"`
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

// Secrets holds paths or settings for secret management tools.
type Secrets struct {
	SopsAgeKeyFile string `yaml:"sops_age_key_file" json:"sops_age_key_file"`
	SSHKey         SSHKey `yaml:"ssh_key" json:"ssh_key"`

	// Service-specific secrets
	CertManager CertManagerSecrets `yaml:"cert_manager" json:"cert_manager"`
	Loki        LokiSecrets        `yaml:"loki" json:"loki"`
	Keycloak    KeycloakSecrets    `yaml:"keycloak" json:"keycloak"`
	Headlamp    HeadlampSecrets    `yaml:"headlamp" json:"headlamp"`
	WeaveGitOps WeaveGitOpsSecrets `yaml:"weave_gitops" json:"weave_gitops"`
	Grafana     GrafanaSecrets     `yaml:"grafana" json:"grafana"`
	AlertProxy  AlertProxySecrets  `yaml:"alert_proxy" json:"alert_proxy"`
	VSphereCsi  VSphereCsiSecrets  `yaml:"vsphere_csi" json:"vsphere_csi"`
}

// SSHKey holds SSH key configuration for cluster access
type SSHKey struct {
	Private string `yaml:"private" json:"private"`
	Public  string `yaml:"public" json:"public"`
	Cypher  string `yaml:"cypher" json:"cypher"`
}

// Simplified structures based on testdata/schema.yaml

// StorageConfig represents the storage configuration for the cluster
type StorageConfig struct {
	DefaultStorageClass string `yaml:"default_storage_class,omitempty" json:"default_storage_class,omitempty" jsonschema:"description=Default storage class for persistent volumes,default=csi-cinder-sc-delete"`
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

// SimplifiedOpenCenter represents the opencenter section of the new simplified schema
type SimplifiedOpenCenter struct {
	Meta           ClusterMeta           `yaml:"meta" json:"meta"`
	Secrets        OpenCenterSecrets     `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Infrastructure Infrastructure        `yaml:"infrastructure" json:"infrastructure"`
	Cluster        ClusterConfig         `yaml:"cluster" json:"cluster"`
	GitOps         GitOpsConfig          `yaml:"gitops" json:"gitops"`
	Storage        StorageConfig         `yaml:"storage,omitempty" json:"storage,omitempty"`
	Talos          *TalosConfig          `yaml:"talos,omitempty" json:"talos,omitempty"`
	ManagedService map[string]ServiceCfg `yaml:"managed-service" json:"managed-service"`
	Services       map[string]ServiceCfg `yaml:"services" json:"services"`
}

// TalosConfig represents Talos-specific configuration
type TalosConfig struct {
	Enabled        bool                `yaml:"enabled" json:"enabled" jsonschema:"description=Enable Talos Linux provider"`
	Version        string              `yaml:"version" json:"version" jsonschema:"description=Talos Linux version"`
	ImageURL       string              `yaml:"image_url" json:"image_url" jsonschema:"description=URL to Talos Linux image"`
	ImageSignature string              `yaml:"image_signature" json:"image_signature" jsonschema:"description=Cryptographic signature of Talos image"`
	MachineConfig  TalosMachineConfig  `yaml:"machine_config" json:"machine_config"`
	NetworkConfig  TalosNetworkConfig  `yaml:"network_config" json:"network_config"`
	SecurityConfig TalosSecurityConfig `yaml:"security_config" json:"security_config"`
	PulumiConfig   TalosPulumiConfig   `yaml:"pulumi_config" json:"pulumi_config"`
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
	ManagementSubnet string   `yaml:"management_subnet" json:"management_subnet" jsonschema:"description=CIDR for management network,default=10.0.1.0/24"`
	ControlSubnet    string   `yaml:"control_subnet" json:"control_subnet" jsonschema:"description=CIDR for control plane network,default=10.0.2.0/24"`
	DataSubnet       string   `yaml:"data_subnet" json:"data_subnet" jsonschema:"description=CIDR for data plane network,default=10.0.3.0/24"`
	WireGuardPort    int      `yaml:"wireguard_port" json:"wireguard_port" jsonschema:"description=UDP port for WireGuard VPN,default=51820"`
	TalosAPIPort     int      `yaml:"talos_api_port" json:"talos_api_port" jsonschema:"description=TCP port for Talos API,default=50000"`
	AllowedCIDRs     []string `yaml:"allowed_cidrs,omitempty" json:"allowed_cidrs,omitempty" jsonschema:"description=List of CIDRs allowed to access cluster"`
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
	StackName         string `yaml:"stack_name" json:"stack_name" jsonschema:"description=Pulumi stack name"`
	SwiftContainer    string `yaml:"swift_container" json:"swift_container" jsonschema:"description=Swift container for Pulumi state"`
	SwiftPrefix       string `yaml:"swift_prefix,omitempty" json:"swift_prefix,omitempty" jsonschema:"description=Swift prefix for state isolation"`
	SecretsPassphrase string `yaml:"secrets_passphrase,omitempty" json:"secrets_passphrase,omitempty" jsonschema:"secret=true,description=Passphrase for Pulumi secrets provider"`
}

// Infrastructure represents the infrastructure configuration block.
type Infrastructure struct {
	Provider string      `yaml:"provider" json:"provider"`
	Cloud    CloudConfig `yaml:"cloud" json:"cloud"`
}

// ServiceCfg captures the on/off toggle plus optional metadata for a service.
type ServiceCfg struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Status   string `yaml:"status,omitempty" json:"status,omitempty" jsonschema:"description=Service deployment status (pending/running/success/failed)"`
	Email    string `yaml:"email" json:"email"`
	Region   string `yaml:"region" json:"region"`
	S3Host   string `yaml:"s3_host" json:"s3_host"`
	S3Region string `yaml:"s3_region" json:"s3_region"`

	// Alert-proxy specific fields (non-secret)
	AlertManagerBaseUrl string `yaml:"alert_manager_base_url" json:"alert_manager_base_url"`
	HTTPRouteFQDN       string `yaml:"http_route_fqdn" json:"http_route_fqdn"`

	// Version control fields
	Release string `yaml:"release" json:"release"`
	Branch  string `yaml:"branch" json:"branch"`
	Uri     string `yaml:"uri" json:"uri"`

	// GitOps source fields (for managed services)
	GitOpsSourceRepo    string `yaml:"gitops_source_repo" json:"gitops_source_repo" jsonschema:"description=GitOps source repository URL"`
	GitOpsSourceRelease string `yaml:"gitops_source_release" json:"gitops_source_release" jsonschema:"description=GitOps source release tag"`
	GitOpsSourceBranch  string `yaml:"gitops_source_branch" json:"gitops_source_branch" jsonschema:"description=GitOps source branch"`

	// Common service fields
	Namespace       string `yaml:"namespace" json:"namespace" jsonschema:"description=Kubernetes namespace for the service"`
	Hostname        string `yaml:"hostname" json:"hostname" jsonschema:"description=Hostname for HTTPRoute configuration"`
	ImageRepository string `yaml:"image_repository" json:"image_repository" jsonschema:"description=Container image repository"`
	ImageTag        string `yaml:"image_tag" json:"image_tag" jsonschema:"description=Container image tag"`

	// Cert-manager fields
	LetsEncryptServer string `yaml:"letsencrypt_server" json:"letsencrypt_server" jsonschema:"description=LetsEncrypt ACME server URL"`

	// Loki fields
	LokiStorageType  string `yaml:"loki_storage_type" json:"loki_storage_type" jsonschema:"description=Loki storage backend type (s3 or swift),enum=s3,enum=swift,default=swift"`
	LokiBucketName   string `yaml:"loki_bucket_name" json:"loki_bucket_name" jsonschema:"description=Loki storage bucket/container name"`
	LokiVolumeSize   int    `yaml:"loki_volume_size" json:"loki_volume_size" jsonschema:"description=Loki persistent volume size in GB"`
	LokiStorageClass string `yaml:"loki_storage_class" json:"loki_storage_class" jsonschema:"description=Loki storage class"`

	// Swift storage fields
	SwiftAuthURL                 string `yaml:"swift_auth_url" json:"swift_auth_url" jsonschema:"description=Swift Keystone V3 authentication URL (must end in /v3)"`
	SwiftRegion                  string `yaml:"swift_region" json:"swift_region" jsonschema:"description=Swift region name"`
	SwiftAuthVersion             int    `yaml:"swift_auth_version" json:"swift_auth_version" jsonschema:"description=Swift authentication version,default=3"`
	SwiftApplicationCredentialID string `yaml:"swift_application_credential_id" json:"swift_application_credential_id" jsonschema:"description=Swift application credential ID (UUID)"`
	SwiftContainerName           string `yaml:"swift_container_name" json:"swift_container_name" jsonschema:"description=Swift container name for Loki logs"`
	SwiftUserDomainName          string `yaml:"swift_user_domain_name" json:"swift_user_domain_name" jsonschema:"description=Swift user domain name (optional, often implied by app credentials)"`

	// Legacy Swift fields (deprecated, use application credentials instead)
	SwiftUsername    string `yaml:"swift_username" json:"swift_username" jsonschema:"description=Swift username (deprecated: use application credentials)"`
	SwiftProjectName string `yaml:"swift_project_name" json:"swift_project_name" jsonschema:"description=Swift project name (deprecated: use application credentials)"`
	SwiftDomainName  string `yaml:"swift_domain_name" json:"swift_domain_name" jsonschema:"description=Swift domain name (deprecated: use application credentials)"`

	// S3 storage fields
	LokiS3Endpoint       string `yaml:"loki_s3_endpoint" json:"loki_s3_endpoint" jsonschema:"description=S3 endpoint URL (for MinIO, Ceph, DigitalOcean Spaces, etc.)"`
	LokiS3Region         string `yaml:"loki_s3_region" json:"loki_s3_region" jsonschema:"description=S3 region (e.g., us-east-1)"`
	LokiS3ForcePathStyle bool   `yaml:"loki_s3_force_path_style" json:"loki_s3_force_path_style" jsonschema:"description=Force S3 path style (required for MinIO and some S3-compatible services)"`
	LokiS3Insecure       bool   `yaml:"loki_s3_insecure" json:"loki_s3_insecure" jsonschema:"description=Allow insecure S3 connections (not using HTTPS)"`

	// Velero fields
	VeleroBackupBucket string `yaml:"velero_backup_bucket" json:"velero_backup_bucket" jsonschema:"description=Velero backup bucket name"`
	VeleroRegion       string `yaml:"velero_region" json:"velero_region" jsonschema:"description=Velero backup region"`

	// Keycloak fields
	KeycloakRealm       string `yaml:"keycloak_realm" json:"keycloak_realm" jsonschema:"description=Keycloak realm name"`
	KeycloakFrontendURL string `yaml:"keycloak_frontend_url" json:"keycloak_frontend_url" jsonschema:"description=Keycloak frontend URL"`
	KeycloakClientID    string `yaml:"keycloak_client_id" json:"keycloak_client_id" jsonschema:"description=Keycloak client ID"`

	// Grafana/Prometheus fields
	GrafanaVolumeSize        int    `yaml:"grafana_volume_size" json:"grafana_volume_size" jsonschema:"description=Grafana persistent volume size in GB"`
	GrafanaStorageClass      string `yaml:"grafana_storage_class" json:"grafana_storage_class" jsonschema:"description=Grafana storage class"`
	PrometheusVolumeSize     int    `yaml:"prometheus_volume_size" json:"prometheus_volume_size" jsonschema:"description=Prometheus persistent volume size in GB"`
	PrometheusStorageClass   string `yaml:"prometheus_storage_class" json:"prometheus_storage_class" jsonschema:"description=Prometheus storage class"`
	AlertmanagerVolumeSize   int    `yaml:"alertmanager_volume_size" json:"alertmanager_volume_size" jsonschema:"description=Alertmanager persistent volume size in GB"`
	AlertmanagerStorageClass string `yaml:"alertmanager_storage_class" json:"alertmanager_storage_class" jsonschema:"description=Alertmanager storage class"`

	// Headlamp fields
	HeadlampOIDCIssuerURL string `yaml:"headlamp_oidc_issuer_url" json:"headlamp_oidc_issuer_url" jsonschema:"description=Headlamp OIDC issuer URL"`
	HeadlampOIDCClientID  string `yaml:"headlamp_oidc_client_id" json:"headlamp_oidc_client_id" jsonschema:"description=Headlamp OIDC client ID"`

	// Calico fields
	CalicoKubeAPIServer string `yaml:"calico_kube_api_server" json:"calico_kube_api_server" jsonschema:"description=Calico Kubernetes API server address"`
}

// CloudConfig represents the cloud configuration within opencenter
type CloudConfig struct {
	AWS       SimplifiedAWSCloud       `yaml:"aws" json:"aws"`
	OpenStack SimplifiedOpenStackCloud `yaml:"openstack" json:"openstack"`
}

// ClusterConfig represents the cluster configuration section
type ClusterConfig struct {
	ClusterName        string           `yaml:"cluster_name" json:"cluster_name" jsonschema:"description=Name of the cluster"`
	AWSAccessKey       string           `yaml:"aws_access_key" json:"aws_access_key"`
	AWSSecretAccessKey string           `yaml:"aws_secret_access_key" json:"aws_secret_access_key"`
	K8sAPIPortACL      []string         `yaml:"k8s_api_port_acl" json:"k8s_api_port_acl"`
	SSHAuthorizedKeys  []string         `yaml:"ssh_authorized_keys" json:"ssh_authorized_keys"`
	Kubernetes         KubernetesConfig `yaml:"kubernetes" json:"kubernetes"`

	// New fields for configuration-driven templates
	BaseDomain  string `yaml:"base_domain,omitempty" json:"base_domain,omitempty" jsonschema:"description=Base domain for the cluster (e.g. k8s.opencenter.cloud)"`
	ClusterFQDN string `yaml:"cluster_fqdn,omitempty" json:"cluster_fqdn,omitempty" jsonschema:"description=Fully qualified domain name for the cluster"`
	AdminEmail  string `yaml:"admin_email,omitempty" json:"admin_email,omitempty" jsonschema:"description=Administrator email address for certificates and notifications"`
}

// KubernetesConfig represents the kubernetes configuration
type KubernetesConfig struct {
	Version              string         `yaml:"version" json:"version"`
	FlavorBastion        string         `yaml:"flavor_bastion" json:"flavor_bastion"`
	FlavorMaster         string         `yaml:"flavor_master" json:"flavor_master"`
	FlavorWorker         string         `yaml:"flavor_worker" json:"flavor_worker"`
	SubnetPods           string         `yaml:"subnet_pods" json:"subnet_pods"`
	SubnetServices       string         `yaml:"subnet_services" json:"subnet_services"`
	LoadbalancerProvider string         `yaml:"loadbalancer_provider" json:"loadbalancer_provider"`
	DNSZoneName          string         `yaml:"dns_zone_name" json:"dns_zone_name"`
	MasterCount          int            `yaml:"master_count" json:"master_count"`
	WorkerCount          int            `yaml:"worker_count" json:"worker_count"`
	WorkerCountWindows   int            `yaml:"worker_count_windows" json:"worker_count_windows"`
	MasterNodes          []NodeConfig   `yaml:"master_nodes,omitempty" json:"master_nodes,omitempty"`
	WorkerNodes          []NodeConfig   `yaml:"worker_nodes,omitempty" json:"worker_nodes,omitempty"`
	WindowsNodes         []NodeConfig   `yaml:"windows_nodes,omitempty" json:"windows_nodes,omitempty"`
	NetworkPlugin        NetworkPlugin  `yaml:"network_plugin" json:"network_plugin"`
	OIDC                 OIDCConfig     `yaml:"oidc" json:"oidc"`
	WindowsWorkers       WindowsWorkers `yaml:"windows_workers" json:"windows_workers"`
}

// NodeConfig represents a baremetal node configuration with id, name, and IP
type NodeConfig struct {
	ID         string `yaml:"id" json:"id"`
	Name       string `yaml:"name" json:"name"`
	AccessIPv4 string `yaml:"access_ip_v4" json:"access_ip_v4"`
}

// NetworkPlugin represents the network plugin configuration
type NetworkPlugin struct {
	Calico  CalicoConfig  `yaml:"calico" json:"calico"`
	Cilium  CiliumConfig  `yaml:"cilium" json:"cilium"`
	KubeOVN KubeOVNConfig `yaml:"kube-ovn" json:"kube-ovn"`
}

// CalicoConfig represents the Calico configuration
type CalicoConfig struct {
	Enabled                   bool   `yaml:"enabled" json:"enabled"`
	CNIIface                  string `yaml:"cni_iface" json:"cni_iface"`
	CalicoInterfaceAutodetect string `yaml:"calico_interface_autodetect" json:"calico_interface_autodetect"`
}

// CiliumConfig represents the Cilium configuration
type CiliumConfig struct {
	Enabled              bool `yaml:"enabled" json:"enabled"`
	OperatorEnabled      bool `yaml:"operator_enabled" json:"operator_enabled"`
	KubeProxyReplacement bool `yaml:"kubeProxyReplacement" json:"kubeProxyReplacement"`
}

// KubeOVNConfig represents the Kube-OVN configuration
type KubeOVNConfig struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`
	CiliumIntegration bool `yaml:"cilium_integration" json:"cilium_integration"`
}

// OIDCConfig represents the OIDC configuration
type OIDCConfig struct {
	Enabled                bool   `yaml:"enabled" json:"enabled"`
	KubeOIDCURL            string `yaml:"kube_oidc_url" json:"kube_oidc_url"`
	KubeOIDCClientID       string `yaml:"kube_oidc_client_id" json:"kube_oidc_client_id"`
	KubeOIDCCAFile         string `yaml:"kube_oidc_ca_file" json:"kube_oidc_ca_file"`
	KubeOIDCUsernameClaim  string `yaml:"kube_oidc_username_claim" json:"kube_oidc_username_claim"`
	KubeOIDCUsernamePrefix string `yaml:"kube_oidc_username_prefix" json:"kube_oidc_username_prefix"`
	KubeOIDCGroupsClaim    string `yaml:"kube_oidc_groups_claim" json:"kube_oidc_groups_claim"`
	KubeOIDCGroupsPrefix   string `yaml:"kube_oidc_groups_prefix" json:"kube_oidc_groups_prefix"`
}

// WindowsWorkers represents the Windows workers configuration
type WindowsWorkers struct {
	Enabled                  bool   `yaml:"enabled" json:"enabled"`
	WindowsUser              string `yaml:"windows_user" json:"windows_user"`
	WindowsAdminPassword     string `yaml:"windows_admin_password" json:"windows_admin_password"`
	WorkerNodeBFVSizeWindows int    `yaml:"worker_node_bfv_size_windows" json:"worker_node_bfv_size_windows"`
	WorkerNodeBFVTypeWindows string `yaml:"worker_node_bfv_type_windows" json:"worker_node_bfv_type_windows"`
}

// SimplifiedOpenTofu represents the opentofu section
type SimplifiedOpenTofu struct {
	Enabled bool                  `yaml:"enabled" json:"enabled"`
	Path    string                `yaml:"path" json:"path"`
	Backend SimplifiedTofuBackend `yaml:"backend" json:"backend"`
}

// SimplifiedTofuBackend represents the backend configuration
type SimplifiedTofuBackend struct {
	Type  string              `yaml:"type" json:"type"`
	Local SimplifiedTofuLocal `yaml:"local,omitempty" json:"local,omitempty"`
	S3    SimplifiedTofuS3    `yaml:"s3,omitempty" json:"s3,omitempty"`
}

// SimplifiedTofuLocal represents the local backend
type SimplifiedTofuLocal struct {
	Path string `yaml:"path" json:"path"`
}

// SimplifiedTofuS3 represents the S3 backend
type SimplifiedTofuS3 struct {
	Bucket   string `yaml:"bucket" json:"bucket"`
	Key      string `yaml:"key" json:"key"`
	Region   string `yaml:"region" json:"region"`
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Profile  string `yaml:"profile,omitempty" json:"profile,omitempty"`
	Encrypt  bool   `yaml:"encrypt,omitempty" json:"encrypt,omitempty"`
}

// SimplifiedCloud represents the cloud section
type SimplifiedCloud struct {
	Provider  string                   `yaml:"provider" json:"provider"`
	OpenStack SimplifiedOpenStackCloud `yaml:"openstack" json:"openstack"`
	AWS       SimplifiedAWSCloud       `yaml:"aws" json:"aws"`
}

// SimplifiedOpenStackCloud represents the OpenStack configuration
type SimplifiedOpenStackCloud struct {
	AuthURL                     string `yaml:"auth_url" json:"auth_url"`
	Insecure                    bool   `yaml:"insecure" json:"insecure"`
	Region                      string `yaml:"region" json:"region"`
	ApplicationCredentialID     string `yaml:"application_credential_id" json:"application_credential_id"`
	ApplicationCredentialSecret string `yaml:"application_credential_secret" json:"application_credential_secret"`
	Domain                      string `yaml:"domain" json:"domain"`
	TenantName                  string `yaml:"tenant_name" json:"tenant_name"`
	FloatingNetworkId           string `yaml:"floating_network_id" json:"floating_network_id"`
	SubnetId                    string `yaml:"subnet_id" json:"subnet_id"`
}

// SimplifiedAWSCloud represents the AWS configuration
type SimplifiedAWSCloud struct {
	Profile        string   `yaml:"profile" json:"profile"`
	Region         string   `yaml:"region" json:"region"`
	VPCID          string   `yaml:"vpc_id" json:"vpc_id"`
	PrivateSubnets []string `yaml:"private_subnets" json:"private_subnets"`
	PublicSubnets  []string `yaml:"public_subnets" json:"public_subnets"`
}

// defaultConfig returns a Config pre-populated with the default
// values based on the simplified schema. This function can be used to
// initialise new cluster configurations.
func defaultConfig(name string) Config {
	cfg := Config{
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         name,
				Env:          "",
				Region:       "",
				Status:       "",
				Organization: "opencenter",
			},
			Secrets: OpenCenterSecrets{
				Backend: "barbican",
				Barbican: BarbicanConfig{
					AuthURL:           "https://identity.example.com/v3",
					ProjectID:         "",
					Region:            "regionOne",
					UserDomainName:    "Default",
					ProjectDomainName: "Default",
					CACert:            "",
				},
			},
			Infrastructure: Infrastructure{
				Provider: "openstack",
				Cloud: CloudConfig{
					AWS: SimplifiedAWSCloud{
						Profile:        "",
						Region:         "",
						VPCID:          "",
						PrivateSubnets: []string{},
						PublicSubnets:  []string{},
					},
					OpenStack: SimplifiedOpenStackCloud{
						AuthURL:                     "https://keystone.api.sjc3.rackspacecloud.com/v3/",
						Insecure:                    false,
						Region:                      "SJC3",
						ApplicationCredentialID:     "",
						ApplicationCredentialSecret: "",
						Domain:                      "Default",
						TenantName:                  "default-tenant",
						FloatingNetworkId:           "",
						SubnetId:                    "",
					},
				},
			},
			Cluster: ClusterConfig{
				ClusterName:        name,
				AWSAccessKey:       "",
				AWSSecretAccessKey: "",
				K8sAPIPortACL:      []string{"0.0.0.0/0"},
				SSHAuthorizedKeys:  []string{"ssh-rsa ..."},
				BaseDomain:         "k8s.opencenter.cloud",
				ClusterFQDN:        fmt.Sprintf("%s.sjc3.k8s.opencenter.cloud", name),
				AdminEmail:         "admin@example.com",
				Kubernetes: KubernetesConfig{
					Version:              "1.31.4",
					FlavorBastion:        "gp.0.2.2",
					FlavorMaster:         "gp.0.4.4",
					FlavorWorker:         "gp.0.4.8",
					SubnetPods:           "10.42.0.0/16",
					SubnetServices:       "10.43.0.0/16",
					LoadbalancerProvider: "ovn",
					DNSZoneName:          fmt.Sprintf("%s.sjc3.k8s.opencenter.cloud", name),
					MasterCount:          3,
					WorkerCount:          2,
					WorkerCountWindows:   0,
					NetworkPlugin: NetworkPlugin{
						Calico: CalicoConfig{
							Enabled:                   true,
							CNIIface:                  "enp3s0",
							CalicoInterfaceAutodetect: "interface",
						},
						Cilium: CiliumConfig{
							Enabled:              false,
							OperatorEnabled:      true,
							KubeProxyReplacement: true,
						},
						KubeOVN: KubeOVNConfig{
							Enabled:           false,
							CiliumIntegration: true,
						},
					},
					OIDC: OIDCConfig{
						Enabled:                false,
						KubeOIDCURL:            "",
						KubeOIDCClientID:       "kubernetes",
						KubeOIDCCAFile:         "",
						KubeOIDCUsernameClaim:  "sub",
						KubeOIDCUsernamePrefix: "oidc:",
						KubeOIDCGroupsClaim:    "groups",
						KubeOIDCGroupsPrefix:   "oidc:",
					},
					WindowsWorkers: WindowsWorkers{
						Enabled:                  false,
						WindowsUser:              "Administrator",
						WindowsAdminPassword:     "",
						WorkerNodeBFVSizeWindows: 0,
						WorkerNodeBFVTypeWindows: "",
					},
				},
			},
			GitOps: GitOpsConfig{
				GitDir:            fmt.Sprintf("./testdata/test-git-repo-%s", name),
				GitURL:            "",
				GitSSHKey:         "",
				GitSSHPub:         "",
				GitBranch:         "main",
				GitOpsBaseRepo:    "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
				GitOpsBaseRelease: "v0.1.0",
				GitOpsBranch:      "main",
				Flux: GitOpsFlux{
					Interval: "15m",
					Prune:    true,
				},
			},
			Storage: StorageConfig{
				DefaultStorageClass: "csi-cinder-sc-delete",
			},
			Talos: nil, // Talos is disabled by default, can be enabled by user
			ManagedService: map[string]ServiceCfg{
				"alert-proxy": {
					Enabled:             true,
					ImageRepository:     "ghcr.io/rackerlabs/alert-proxy",
					ImageTag:            "latest",
					AlertManagerBaseUrl: fmt.Sprintf("http://alertmanager.example.com/api/v2/alerts"),
					HTTPRouteFQDN:       fmt.Sprintf("https://alerts.%s.sjc3.k8s.opencenter.cloud", name),
					GitOpsSourceRepo:    "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
					GitOpsSourceRelease: "v0.1.0",
					GitOpsSourceBranch:  "main",
				},
			},
			Services: map[string]ServiceCfg{
				"calico": {
					Enabled:             true,
					CalicoKubeAPIServer: fmt.Sprintf("https://api.%s.sjc3.k8s.opencenter.cloud:6443", name),
				},
				"cert-manager": {
					Enabled:           false,
					Email:             "mpk-support@rackspace.com",
					Region:            "us-east-1",
					LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
				},
				"etcd-backup": {
					Enabled:  true,
					S3Host:   "https://swift.api.dfw3.rackspacecloud.com",
					S3Region: "DFW3",
				},
				"external-snapshotter": {Enabled: true},
				"fluxcd":               {Enabled: true},
				"gateway":              {Enabled: true},
				"gateway-api":          {Enabled: true},
				"headlamp": {
					Enabled:               true,
					Hostname:              fmt.Sprintf("dashboard.%s.sjc3.k8s.opencenter.cloud", name),
					HeadlampOIDCIssuerURL: fmt.Sprintf("https://auth.%s.sjc3.k8s.opencenter.cloud/realms/opencenter", name),
					HeadlampOIDCClientID:  "kubernetes",
				},
				"keycloak": {
					Enabled:             false,
					Hostname:            fmt.Sprintf("auth.%s.sjc3.k8s.opencenter.cloud", name),
					KeycloakRealm:       "opencenter",
					KeycloakClientID:    "kubernetes",
					KeycloakFrontendURL: fmt.Sprintf("https://auth.%s.sjc3.k8s.opencenter.cloud", name),
				},
				"kube-prometheus-stack": {
					Enabled:                  true,
					PrometheusVolumeSize:     50,
					PrometheusStorageClass:   "csi-cinder-sc-delete",
					GrafanaVolumeSize:        10,
					GrafanaStorageClass:      "csi-cinder-sc-delete",
					AlertmanagerVolumeSize:   10,
					AlertmanagerStorageClass: "csi-cinder-sc-delete",
				},
				"kyverno": {Enabled: true},
				"loki": {
					Enabled:          false,
					LokiVolumeSize:   20,
					LokiStorageClass: "csi-cinder-sc-delete",
					LokiBucketName:   fmt.Sprintf("%s-loki", name),
					SwiftAuthURL:     "https://keystone.api.sjc3.rackspacecloud.com/v3/",
					SwiftUsername:    "",
					SwiftProjectName: "",
					SwiftRegion:      "SJC3",
					SwiftDomainName:  "Default",
				},
				"olm":               {Enabled: true},
				"openstack-ccm":     {Enabled: true},
				"openstack-csi":     {Enabled: true},
				"postgres-operator": {Enabled: true},
				"rbac-manager":      {Enabled: true},
				"sources":           {Enabled: true},
				"velero": {
					Enabled:            true,
					VeleroBackupBucket: fmt.Sprintf("%s-backups", name),
					VeleroRegion:       "us-east-1",
				},
				"vsphere-csi": {
					Enabled:         false, // Disabled by default, only for VMware environments
					Namespace:       "vmware-system-csi",
					ImageRepository: "registry.k8s.io/csi-vsphere",
					ImageTag:        "v3.3.0",
				},
				"weave-gitops": {
					Enabled:  true,
					Hostname: fmt.Sprintf("gitops.%s.sjc3.k8s.opencenter.cloud", name),
				},
			},
		},
		OpenTofu: SimplifiedOpenTofu{
			Enabled: true,
			Path:    "opentofu",
			Backend: SimplifiedTofuBackend{
				Type: "local",
				Local: SimplifiedTofuLocal{
					Path: fmt.Sprintf("./testdata/test-git-repo-%s/terraform.tfstate", name),
				},
				S3: SimplifiedTofuS3{
					Bucket: "",
					Key:    "",
					Region: "",
				},
			},
		},
		Secrets: Secrets{
			SopsAgeKeyFile: "",
			SSHKey: SSHKey{
				Private: fmt.Sprintf("./testdata/test-git-repo-%s/%s/secrets/ssh/%s", name, name, name),
				Public:  fmt.Sprintf("./testdata/test-git-repo-%s/%s/secrets/ssh/%s.pub", name, name, name),
				Cypher:  "ed25519",
			},
			// Service-specific secrets - must be provided by user
			CertManager: CertManagerSecrets{
				AWSAccessKey:       "",
				AWSSecretAccessKey: "",
			},
			Loki: LokiSecrets{
				SwiftPassword: "",
			},
			Keycloak: KeycloakSecrets{
				ClientSecret:  "",
				AdminPassword: "",
			},
			Headlamp: HeadlampSecrets{
				OIDCClientSecret: "",
			},
			WeaveGitOps: WeaveGitOpsSecrets{
				Password:     "",
				PasswordHash: "",
			},
			Grafana: GrafanaSecrets{
				AdminPassword: "",
			},
			AlertProxy: AlertProxySecrets{
				CoreDeviceId:        "",
				AccountServiceToken: "",
				CoreAccountNumber:   "",
			},
			VSphereCsi: VSphereCsiSecrets{
				VCenterHost:  "",
				Username:     "",
				Password:     "",
				Datacenters:  "",
				InsecureFlag: "false",
				Port:         "443",
			},
		},
	}

	// Populate IAC field from defaults
	if err := populateIAC(&cfg); err != nil {
		// If IAC population fails, return config with minimal IAC structure
		cfg.IAC = IAC{
			Main: map[string]any{
				"cluster_name":    name,
				"master_count":    3,
				"worker_count":    2,
				"subnet_nodes":    "10.2.184.0/22",
				"subnet_pods":     "10.42.0.0/16",
				"subnet_services": "10.43.0.0/16",
			},
			Modules: map[string]any{
				"calico": map[string]any{
					"source": "",
				},
				"kubespray-cluster": map[string]any{
					"source": "",
				},
				"openstack-nova": map[string]any{
					"source": "",
				},
			},
		}
	}

	return cfg
}

// NewDefault returns a Config initialized with the default values for the given cluster name.
//
// Inputs:
//   - name: The name of the cluster.
//
// Outputs:
//   - Config: A new Config object with default values.
func NewDefault(name string) Config {
	return defaultConfig(name)
}

// DefaultTalosConfig returns a TalosConfig initialized with secure default values.
// This function should be called when enabling Talos for a cluster.
//
// Inputs:
//   - clusterName: The name of the cluster.
//
// Outputs:
//   - *TalosConfig: A new TalosConfig object with default values.
func DefaultTalosConfig(clusterName string) *TalosConfig {
	return &TalosConfig{
		Enabled:        true,
		Version:        "v1.8.0",
		ImageURL:       "https://github.com/siderolabs/talos/releases/download/v1.8.0/openstack-amd64.raw.xz",
		ImageSignature: "",
		MachineConfig: TalosMachineConfig{
			AppArmorEnabled:  true,
			SeccompEnabled:   true,
			DiskEncryption:   true,
			KubePrismEnabled: true,
			SystemExtensions: []string{},
			LogDestination:   "",
		},
		NetworkConfig: TalosNetworkConfig{
			ManagementSubnet: "10.0.1.0/24",
			ControlSubnet:    "10.0.2.0/24",
			DataSubnet:       "10.0.3.0/24",
			WireGuardPort:    51820,
			TalosAPIPort:     50000,
			AllowedCIDRs:     []string{},
		},
		SecurityConfig: TalosSecurityConfig{
			VTPMEnabled:       true,
			BarbicanKeyID:     "",
			ImageVerification: true,
			MFARequired:       true,
			AuditLogEnabled:   true,
		},
		PulumiConfig: TalosPulumiConfig{
			StackName:         fmt.Sprintf("%s-talos", clusterName),
			SwiftContainer:    fmt.Sprintf("%s-pulumi-state", clusterName),
			SwiftPrefix:       clusterName,
			SecretsPassphrase: "",
		},
	}
}

// Helper methods for backward compatibility

// ClusterName returns the cluster name from the simplified structure
func (c Config) ClusterName() string {
	return c.OpenCenter.Cluster.ClusterName
}

// GitOps returns the GitOps configuration from the simplified structure
func (c Config) GitOps() GitOpsConfig {
	return c.OpenCenter.GitOps
}

// ResolveConfigDir resolves the configuration directory based on the OPENCENTER_CONFIG_DIR
// environment variable. If the variable is not set, it falls back to the user's
// standard config directory (e.g., ~/.config/openCenter on Linux).
// The directory is created if it does not exist.
//
// Outputs:
//   - string: The absolute path to the configuration directory.
//   - error: An error if one occurred.
func ResolveConfigDir() (string, error) {
	var err error
	dir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if dir == "" {
		// Determine OS-specific config directory
		switch runtime.GOOS {
		case "windows":
			base := os.Getenv("APPDATA")
			if base == "" {
				base = os.Getenv("LOCALAPPDATA")
			}
			if base == "" {
				base = os.Getenv("USERPROFILE")
			}
			dir = filepath.Join(base, "openCenter")
		default:
			home, herr := os.UserHomeDir()
			if herr != nil {
				err = herr
				return "", err
			}
			dir = filepath.Join(home, ".config", "openCenter")
		}
	}
	// Ensure absolute path
	if !filepath.IsAbs(dir) {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return "", err
		}
	}
	// Create directory if not exists
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		err = mkErr
		return "", err
	}
	return dir, err
}

// ParseClusterIdentifier parses a cluster identifier which can be in one of two formats:
// 1. "cluster" - just the cluster name (uses default "opencenter" organization)
// 2. "organization/cluster" - organization and cluster name
//
// Inputs:
//   - identifier: The cluster identifier to parse.
//
// Outputs:
//   - organization: The organization name (or "opencenter" if not specified).
//   - clusterName: The cluster name.
//   - error: An error if the identifier is invalid.
func ParseClusterIdentifier(identifier string) (organization string, clusterName string, err error) {
	if identifier == "" {
		return "", "", errors.New("cluster identifier cannot be empty")
	}

	// Check for organization/cluster format
	if strings.Contains(identifier, "/") {
		parts := strings.SplitN(identifier, "/", 2)
		if len(parts) != 2 {
			return "", "", errors.New("invalid cluster identifier format: expected 'organization/cluster'")
		}
		organization = parts[0]
		clusterName = parts[1]

		// Validate both parts
		if err := ValidateClusterName(organization); err != nil {
			return "", "", fmt.Errorf("invalid organization name: %w", err)
		}
		if err := ValidateClusterName(clusterName); err != nil {
			return "", "", fmt.Errorf("invalid cluster name: %w", err)
		}

		return organization, clusterName, nil
	}

	// Just cluster name, use default organization
	if err := ValidateClusterName(identifier); err != nil {
		return "", "", err
	}

	return "opencenter", identifier, nil
}

// ValidateClusterName validates and sanitizes a cluster name to ensure it's safe for use as a directory name.
// It checks for valid characters and prevents directory traversal attacks.
// Note: This validates individual cluster or organization names, not the full "org/cluster" format.
// Use ParseClusterIdentifier for validating the full identifier format.
//
// Inputs:
//   - name: The cluster name to validate.
//
// Outputs:
//   - error: An error if the name is invalid.
func ValidateClusterName(name string) error {
	if name == "" {
		return errors.New("cluster name cannot be empty for directory creation")
	}

	// Check for path separators and special characters that could cause issues
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return errors.New("cluster name cannot contain path separators (/ or \\) for directory structure")
	}

	// Check for relative path components
	if name == "." || name == ".." || strings.HasPrefix(name, ".") && (strings.Contains(name, "/") || strings.Contains(name, "\\")) {
		return errors.New("cluster name cannot be a relative path component for security reasons")
	}

	// Allow alphanumeric characters, hyphens, underscores, and dots (but not starting with dot)
	validName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	if !validName.MatchString(name) {
		return errors.New("cluster name must start with alphanumeric character and contain only alphanumeric characters, dots, hyphens, and underscores for directory naming")
	}

	// Prevent excessively long names that could cause filesystem issues
	if len(name) > 255 {
		return errors.New("cluster name cannot exceed 255 characters for filesystem compatibility")
	}

	return nil
}

// ClusterDirectoryPath returns the absolute path to a cluster's directory within the clusters subdirectory.
//
// Inputs:
//   - name: The name of the cluster.
//
// Outputs:
//   - string: The absolute path to the cluster directory.
//   - error: An error if one occurred.
func ClusterDirectoryPath(name string) (string, error) {
	if err := ValidateClusterName(name); err != nil {
		return "", fmt.Errorf("invalid cluster name for directory creation: %w", err)
	}

	dir, err := ResolveConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory for cluster '%s': %w", name, err)
	}

	return filepath.Join(dir, "clusters", name), nil
}

// ClusterSecretsPath returns the absolute path to a cluster's secrets directory for SOPS key storage.
//
// Inputs:
//   - name: The name of the cluster.
//
// Outputs:
//   - string: The absolute path to the cluster's secrets directory.
//   - error: An error if one occurred.
func ClusterSecretsPath(name string) (string, error) {
	clusterDir, err := ClusterDirectoryPath(name)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster directory for secrets path: %w", err)
	}

	return filepath.Join(clusterDir, "secrets", "age", "keys"), nil
}

// ConfigPath returns the absolute path to a cluster's configuration file.
// It implements a fallback strategy to support both organization-based and legacy structures.
// The name parameter can be in "cluster" or "organization/cluster" format.
// If no organization is specified, it searches all organizations for the cluster.
//
// Inputs:
//   - name: The name of the cluster (can be "cluster" or "organization/cluster").
//
// Outputs:
//   - string: The absolute path to the configuration file.
//   - error: An error if one occurred.
func ConfigPath(name string) (string, error) {
	// Parse the cluster identifier to extract organization and cluster name
	organization, clusterName, err := ParseClusterIdentifier(name)
	if err != nil {
		return "", fmt.Errorf("invalid cluster identifier: %w", err)
	}

	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory: %w", err)
	}

	// Load CLI configuration to get the configured clustersDir
	cliConfigManager, err := NewConfigManager("")
	var clustersDir string
	if err == nil {
		clustersDir = cliConfigManager.GetConfig().Paths.ClustersDir
		if clustersDir == "" {
			clustersDir = filepath.Join(configDir, "clusters")
		}
		clustersDir = ExpandPath(clustersDir)
	} else {
		clustersDir = filepath.Join(configDir, "clusters")
	}

	// Priority 1: If organization was explicitly specified, check organization-based paths
	if strings.Contains(name, "/") {
		if cliConfigManager != nil {
			pathResolver := NewPathResolver(cliConfigManager)
			paths := pathResolver.ResolveClusterPaths(clusterName, organization)

			// Check for config file at organization level (primary location)
			orgConfigPath := filepath.Join(paths.OrganizationDir, "."+clusterName+"-config.yaml")
			if _, statErr := os.Stat(orgConfigPath); statErr == nil {
				return orgConfigPath, nil
			}

			// Check for config file at cluster directory level (alternative location)
			clusterConfigPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
			if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
				return clusterConfigPath, nil
			}
		}
		// If explicitly specified org/cluster not found, return error
		return "", fmt.Errorf("cluster configuration file not found for cluster %s", name)
	}

	// Priority 2: No organization specified - search organization-based paths first
	if entries, readErr := os.ReadDir(clustersDir); readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				orgName := entry.Name()

				// Check for config file at organization level (primary location)
				orgConfigPath := filepath.Join(clustersDir, orgName, "."+clusterName+"-config.yaml")
				if _, statErr := os.Stat(orgConfigPath); statErr == nil {
					return orgConfigPath, nil
				}

				// Check for config file at cluster directory level (alternative location)
				clusterConfigPath := filepath.Join(clustersDir, orgName, "infrastructure", "clusters", clusterName, "."+clusterName+"-config.yaml")
				if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
					return clusterConfigPath, nil
				}
			}
		}
	}

	// Priority 3: Check for flat config file (backward compatibility)
	flatConfigPath := filepath.Join(configDir, clusterName+".yaml")
	if _, statErr := os.Stat(flatConfigPath); statErr == nil {
		return flatConfigPath, nil
	}

	// Priority 4: Fall back to legacy directory structure (backward compatibility)
	clusterDir, err := ClusterDirectoryPath(clusterName)
	if err == nil {
		legacyConfigPath := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")
		if _, statErr := os.Stat(legacyConfigPath); statErr == nil {
			return legacyConfigPath, nil
		}
	}

	// Config file not found anywhere
	return "", fmt.Errorf("cluster configuration file not found for cluster %s", name)
}

// Load reads and unmarshals a YAML configuration file for the given cluster name.
// Default values are applied for any omitted fields.
// It supports both organization-based and legacy directory structures.
// The name parameter can be in "cluster" or "organization/cluster" format.
//
// Inputs:
//   - name: The name of the cluster (can be "cluster" or "organization/cluster").
//
// Outputs:
//   - Config: The loaded configuration.
//   - error: An error if the file does not exist or cannot be parsed.
func Load(name string) (Config, error) {
	// Parse the cluster identifier - ConfigPath will handle validation
	_, clusterName, err := ParseClusterIdentifier(name)
	if err != nil {
		return Config{}, fmt.Errorf("invalid cluster identifier: %w", err)
	}

	path, err := ConfigPath(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to resolve configuration path for cluster '%s': %w", name, err)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return Config{}, fmt.Errorf("failed to read cluster configuration file '%s': %w", path, readErr)
	}

	// Unmarshal YAML then overlay onto default config (use actual cluster name, not full identifier)
	cfg := defaultConfig(clusterName)
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		return Config{}, fmt.Errorf("failed to parse YAML configuration from '%s': %w", path, unmarshalErr)
	}

	// Apply organization-based defaults if not explicitly set
	applyOrganizationDefaults(&cfg)

	// Populate IAC field from defaults and user configuration
	if err := populateIAC(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to populate IAC configuration: %w", err)
	}

	return cfg, nil
}

// applyOrganizationDefaults applies organization-based defaults to the configuration.
// This ensures that S3 bucket names use the organization name (lowercase) by default.
func applyOrganizationDefaults(cfg *Config) {
	// Set S3 bucket to organization name (lowercase) if not explicitly set by user
	// Only apply if the bucket is still the default (cluster name)
	organization := cfg.OpenCenter.Meta.Organization
	if organization == "" {
		organization = cfg.ClusterName()
	}

	// If S3 bucket is set to the cluster name (default), update it to organization
	if cfg.OpenTofu.Backend.S3.Bucket == strings.ToLower(cfg.ClusterName()) {
		cfg.OpenTofu.Backend.S3.Bucket = strings.ToLower(organization)
	}

	// Ensure bucket name is always lowercase
	if cfg.OpenTofu.Backend.S3.Bucket != "" {
		cfg.OpenTofu.Backend.S3.Bucket = strings.ToLower(cfg.OpenTofu.Backend.S3.Bucket)
	}
}

// populateIAC populates the IAC field from default YAML data and user configuration.
// It merges the default IAC structure with values from the user's configuration.
func populateIAC(cfg *Config) error {
	// Parse the default IAC YAML structure
	var defaultIAC struct {
		Locals  map[string]any `yaml:"locals"`
		Modules map[string]any `yaml:"modules"`
	}

	if err := yaml.Unmarshal([]byte(defaultIACYAML), &defaultIAC); err != nil {
		return fmt.Errorf("failed to parse default IAC YAML: %w", err)
	}

	// Initialize IAC field
	cfg.IAC = IAC{
		Main:    make(map[string]any),
		Modules: make(map[string]any),
	}

	// Copy default locals to IAC.Main
	for k, v := range defaultIAC.Locals {
		cfg.IAC.Main[k] = v
	}

	// Copy default modules to IAC.Modules
	for k, v := range defaultIAC.Modules {
		cfg.IAC.Modules[k] = v
	}

	// Override with user configuration values
	if err := mergeUserConfigIntoIAC(cfg); err != nil {
		return fmt.Errorf("failed to merge user config into IAC: %w", err)
	}

	return nil
}

// mergeUserConfigIntoIAC merges user configuration values into the IAC structure.
func mergeUserConfigIntoIAC(cfg *Config) error {
	// Map user configuration to IAC locals
	if cfg.OpenCenter.Cluster.ClusterName != "" {
		cfg.IAC.Main["cluster_name"] = cfg.OpenCenter.Cluster.ClusterName
	}

	// Map OpenStack configuration
	if cfg.OpenCenter.Infrastructure.Provider == "openstack" {
		os := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
		if os.AuthURL != "" {
			cfg.IAC.Main["openstack_auth_url"] = os.AuthURL
		}
		if os.Region != "" {
			cfg.IAC.Main["openstack_region"] = os.Region
		}
		if os.TenantName != "" {
			cfg.IAC.Main["openstack_tenant_name"] = os.TenantName
		}
		if os.Domain != "" {
			cfg.IAC.Main["openstack_project_domain_name"] = os.Domain
			cfg.IAC.Main["openstack_user_domain_name"] = os.Domain
		}
		cfg.IAC.Main["openstack_insecure"] = os.Insecure
		if os.ApplicationCredentialID != "" {
			cfg.IAC.Main["openstack_user_name"] = os.ApplicationCredentialID
		}
		if os.ApplicationCredentialSecret != "" {
			cfg.IAC.Main["openstack_user_password"] = os.ApplicationCredentialSecret
		}
		if os.FloatingNetworkId != "" {
			cfg.IAC.Main["router_external_network_id"] = os.FloatingNetworkId
		}
	}

	// Map Kubernetes configuration
	k8s := cfg.OpenCenter.Cluster.Kubernetes
	if k8s.Version != "" {
		cfg.IAC.Main["kubernetes_version"] = k8s.Version
	}
	if k8s.MasterCount > 0 {
		cfg.IAC.Main["master_count"] = k8s.MasterCount
	}
	if k8s.WorkerCount > 0 {
		cfg.IAC.Main["worker_count"] = k8s.WorkerCount
	}
	if k8s.WorkerCountWindows > 0 {
		cfg.IAC.Main["worker_count_windows"] = k8s.WorkerCountWindows
	}
	if k8s.FlavorBastion != "" {
		cfg.IAC.Main["flavor_bastion"] = k8s.FlavorBastion
	}
	if k8s.FlavorMaster != "" {
		cfg.IAC.Main["flavor_master"] = k8s.FlavorMaster
	}
	if k8s.FlavorWorker != "" {
		cfg.IAC.Main["flavor_worker"] = k8s.FlavorWorker
	}
	if k8s.SubnetPods != "" {
		cfg.IAC.Main["subnet_pods"] = k8s.SubnetPods
	}
	if k8s.SubnetServices != "" {
		cfg.IAC.Main["subnet_services"] = k8s.SubnetServices
	}
	if k8s.LoadbalancerProvider != "" {
		cfg.IAC.Main["loadbalancer_provider"] = k8s.LoadbalancerProvider
	}
	if k8s.DNSZoneName != "" {
		cfg.IAC.Main["dns_zone_name"] = k8s.DNSZoneName
	}

	// Map network plugin configuration with proper conditional logic
	if k8s.NetworkPlugin.Calico.Enabled {
		cfg.IAC.Main["network_plugin"] = "calico"
		if k8s.NetworkPlugin.Calico.CNIIface != "" {
			cfg.IAC.Main["cni_iface"] = k8s.NetworkPlugin.Calico.CNIIface
		}
		if k8s.NetworkPlugin.Calico.CalicoInterfaceAutodetect != "" {
			cfg.IAC.Main["calico_interface_autodetect"] = k8s.NetworkPlugin.Calico.CalicoInterfaceAutodetect
		}
	} else if k8s.NetworkPlugin.Cilium.Enabled {
		cfg.IAC.Main["network_plugin"] = "cilium"
		cfg.IAC.Main["cilium_operator_enabled"] = k8s.NetworkPlugin.Cilium.OperatorEnabled
		cfg.IAC.Main["cilium_kube_proxy_replacement"] = k8s.NetworkPlugin.Cilium.KubeProxyReplacement
	} else if k8s.NetworkPlugin.KubeOVN.Enabled {
		cfg.IAC.Main["network_plugin"] = "kube-ovn"
		cfg.IAC.Main["kube_ovn_cilium_integration"] = k8s.NetworkPlugin.KubeOVN.CiliumIntegration
	}

	// Map SSH authorized keys
	if len(cfg.OpenCenter.Cluster.SSHAuthorizedKeys) > 0 {
		cfg.IAC.Main["ssh_authorized_keys"] = cfg.OpenCenter.Cluster.SSHAuthorizedKeys
	}

	// Map baremetal node configurations
	if len(k8s.MasterNodes) > 0 {
		cfg.IAC.Main["master_nodes"] = k8s.MasterNodes
	}
	if len(k8s.WorkerNodes) > 0 {
		cfg.IAC.Main["worker_nodes"] = k8s.WorkerNodes
	}

	return nil
}

// GenerateCompleteConfig generates a complete configuration by merging schema defaults
// with the actual cluster configuration. The opencenter values take precedence over
// schema defaults.
//
// Inputs:
//   - name: The cluster name to load configuration for.
//
// Outputs:
//   - Config: The complete merged configuration.
//   - error: An error if the configuration cannot be generated.
func GenerateCompleteConfig(name string) (Config, error) {
	// Generate schema defaults as YAML
	defaultYAML, err := GenerateDefaultFromSchema(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to generate schema defaults: %w", err)
	}

	// Read the actual cluster configuration file directly as YAML
	path, err := ConfigPath(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get config path: %w", err)
	}
	actualYAML, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read cluster config: %w", err)
	}

	// Parse both as generic maps to preserve all structure
	var schemaDefaults map[string]any
	if err := yaml.Unmarshal(defaultYAML, &schemaDefaults); err != nil {
		return Config{}, fmt.Errorf("failed to parse schema defaults: %w", err)
	}

	var actualConfig map[string]any
	if err := yaml.Unmarshal(actualYAML, &actualConfig); err != nil {
		return Config{}, fmt.Errorf("failed to parse actual config: %w", err)
	}

	// Merge the configurations with actual config taking precedence
	mergedConfig := mergeYAMLMaps(schemaDefaults, actualConfig)

	// Marshal back to YAML then unmarshal into Config struct
	mergedYAML, err := yaml.Marshal(mergedConfig)
	if err != nil {
		return Config{}, fmt.Errorf("failed to marshal merged config: %w", err)
	}

	var completeCfg Config
	if err := yaml.Unmarshal(mergedYAML, &completeCfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse merged config into struct: %w", err)
	}

	return completeCfg, nil
}

// mergeYAMLMaps recursively merges two YAML maps, with values from 'override' taking precedence
func mergeYAMLMaps(base, override map[string]any) map[string]any {
	result := make(map[string]any)

	// Start with all base values
	for k, v := range base {
		result[k] = v
	}

	// Override with values from override map
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			// If both values are maps, merge them recursively
			if baseMap, baseIsMap := baseVal.(map[string]any); baseIsMap {
				if overrideMap, overrideIsMap := v.(map[string]any); overrideIsMap {
					result[k] = mergeYAMLMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		// Otherwise, override value takes precedence
		result[k] = v
	}

	return result
}

// GenerateCompleteConfigYAML generates a complete configuration YAML by merging schema defaults
// with the actual cluster configuration, preserving all YAML structure.
//
// Inputs:
//   - name: The cluster name to load configuration for.
//
// Outputs:
//   - []byte: The complete merged configuration as YAML.
//   - error: An error if the configuration cannot be generated.
func GenerateCompleteConfigYAML(name string) ([]byte, error) {
	// Generate schema defaults as YAML
	defaultYAML, err := GenerateDefaultFromSchema(name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema defaults: %w", err)
	}

	// Read the actual cluster configuration file directly as YAML
	path, err := ConfigPath(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}
	actualYAML, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster config: %w", err)
	}

	// Parse both as generic maps to preserve all structure
	var schemaDefaults map[string]any
	if err := yaml.Unmarshal(defaultYAML, &schemaDefaults); err != nil {
		return nil, fmt.Errorf("failed to parse schema defaults: %w", err)
	}

	var actualConfig map[string]any
	if err := yaml.Unmarshal(actualYAML, &actualConfig); err != nil {
		return nil, fmt.Errorf("failed to parse actual config: %w", err)
	}

	// Merge the configurations with actual config taking precedence
	mergedConfig := mergeYAMLMaps(schemaDefaults, actualConfig)

	// Marshal back to YAML
	mergedYAML, err := yaml.Marshal(mergedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged config: %w", err)
	}

	return mergedYAML, nil
}

// SaveDebugConfig saves a complete configuration to the GitOps directory as .openCenter.yaml
// for debugging purposes. This is only called when OPENCENTER_DEBUG environment variable exists.
//
// Inputs:
//   - clusterName: The cluster name to generate complete config for.
//   - gitDir: The GitOps directory where to save the debug config.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func SaveDebugConfig(clusterName, gitDir string) error {
	if gitDir == "" {
		return fmt.Errorf("git directory is empty")
	}

	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return fmt.Errorf("failed to create git directory %s: %w", gitDir, err)
	}

	debugPath := filepath.Join(gitDir, ".openCenter.yaml")

	// Generate the complete config YAML
	data, err := GenerateCompleteConfigYAML(clusterName)
	if err != nil {
		return fmt.Errorf("failed to generate complete config: %w", err)
	}

	// Write the debug config file with 0600 permissions
	if err := os.WriteFile(debugPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write debug config to %s: %w", debugPath, err)
	}

	return nil
}

// Save writes the configuration to a YAML file. The file is saved with 0600
// permissions to protect sensitive data.
//
// Inputs:
//   - cfg: The configuration to save.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func Save(cfg Config) error {
	return saveConfig(cfg, false)
}

// SaveWithOmitEmpty writes the configuration to a YAML file, omitting empty fields.
// The file is saved with 0600 permissions to protect sensitive data.
// This is useful for cleaning up configurations by removing fields with zero values.
//
// Inputs:
//   - cfg: The configuration to save.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func SaveWithOmitEmpty(cfg Config) error {
	return saveConfig(cfg, true)
}

// saveConfig is the internal implementation for saving configurations.
func saveConfig(cfg Config, omitEmpty bool) error {
	if cfg.ClusterName() == "" {
		return errors.New("cluster_name must not be empty")
	}

	// Try to get existing config path first
	path, err := ConfigPath(cfg.ClusterName())
	if err != nil {
		// If config doesn't exist, determine where to create it based on organization
		path, err = getConfigPathForSave(cfg)
		if err != nil {
			return err
		}
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var data []byte
	var marshalErr error

	if omitEmpty {
		// Marshal to map first, then clean empty values
		var configMap map[string]any
		tempData, err := yaml.Marshal(&cfg)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(tempData, &configMap); err != nil {
			return err
		}

		// Remove empty values recursively
		cleanEmptyValues(configMap)

		data, marshalErr = yaml.Marshal(configMap)
	} else {
		// Standard marshal
		data, marshalErr = yaml.Marshal(&cfg)
	}

	if marshalErr != nil {
		return marshalErr
	}
	// Write with 0600 permissions
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
		return writeErr
	}
	return nil
}

// cleanEmptyValues recursively removes empty values from a map.
// Empty values include: nil, empty strings, empty slices, empty maps, and zero numbers.
func cleanEmptyValues(m map[string]any) {
	for key, value := range m {
		if isEmpty(value) {
			delete(m, key)
			continue
		}

		// Recursively clean nested maps
		if nestedMap, ok := value.(map[string]any); ok {
			cleanEmptyValues(nestedMap)
			// Remove the nested map if it became empty after cleaning
			if len(nestedMap) == 0 {
				delete(m, key)
			}
		}
	}
}

// isEmpty checks if a value is considered empty.
func isEmpty(v any) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case bool:
		return false // Keep boolean values even if false
	case int, int8, int16, int32, int64:
		return val == 0
	case uint, uint8, uint16, uint32, uint64:
		return val == 0
	case float32, float64:
		return val == 0
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}

// getConfigPathForSave determines where to save a new cluster configuration.
// It uses organization structure if organization is set, otherwise uses flat file structure.
func getConfigPathForSave(cfg Config) (string, error) {
	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory: %w", err)
	}

	organization := cfg.OpenCenter.Meta.Organization
	clusterName := cfg.ClusterName()

	if organization != "" && organization != "opencenter" {
		// Use organization structure: clusters/<org>/.<cluster>-config.yaml
		return filepath.Join(configDir, "clusters", organization, "."+clusterName+"-config.yaml"), nil
	}

	// Use flat file structure for backward compatibility and default organization
	return filepath.Join(configDir, clusterName+".yaml"), nil
}

// List returns a sorted list of cluster names from the configuration directory.
// It looks for cluster directories within the configured clustersDir.
// It supports both organization-based and legacy directory structures.
//
// Outputs:
//   - []string: A list of cluster names.
//   - error: An error if the directory cannot be read.
func List() ([]string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		Debugf("List: failed to resolve config directory: %v", err)
		return nil, fmt.Errorf("failed to resolve configuration directory: %w", err)
	}
	Debugf("List: resolved config directory: %s", dir)

	// Load CLI configuration to get the configured clustersDir
	configManager, err := NewConfigManager("")
	if err != nil {
		Debugf("List: failed to load CLI config manager: %v", err)
		// Fall back to default behavior if CLI config can't be loaded
	}

	var clustersDir string
	if configManager != nil {
		clustersDir = configManager.GetConfig().Paths.ClustersDir
		Debugf("List: using clustersDir from CLI config: %s", clustersDir)
	} else {
		// Fallback to default
		clustersDir = filepath.Join(dir, "clusters")
		Debugf("List: using default clustersDir: %s", clustersDir)
	}

	// Expand environment variables and tilde in clustersDir
	clustersDir = ExpandPath(clustersDir)
	Debugf("List: expanded clustersDir: %s", clustersDir)

	var names []string
	nameSet := make(map[string]bool) // Use set to avoid duplicates

	// First, check for flat YAML files in the config directory (for backward compatibility and tests)
	Debugf("List: checking for flat config files in: %s", dir)
	if flatEntries, flatErr := os.ReadDir(dir); flatErr == nil {
		for _, flatEntry := range flatEntries {
			if !flatEntry.IsDir() && strings.HasSuffix(flatEntry.Name(), ".yaml") {
				// Extract cluster name by removing .yaml extension
				clusterName := strings.TrimSuffix(flatEntry.Name(), ".yaml")
				// Skip the CLI config file itself
				if clusterName != "" && clusterName != "config" && !nameSet[clusterName] {
					Debugf("List: found flat config file: %s (cluster: %s)", flatEntry.Name(), clusterName)
					names = append(names, clusterName)
					nameSet[clusterName] = true
				}
			}
		}
	}
	Debugf("List: found %d flat config clusters", len(names))

	// Check clusters directory for legacy and organization-based structures
	Debugf("List: checking clusters directory: %s", clustersDir)
	entries, readErr := os.ReadDir(clustersDir)
	if readErr != nil {
		// If clusters directory doesn't exist, just return flat config files
		if os.IsNotExist(readErr) {
			Debugf("List: clusters directory does not exist, returning %d flat config clusters", len(names))
			// Sort lexically
			if len(names) > 1 {
				sortStrings(names)
			}
			return names, nil
		}
		Debugf("List: failed to read clusters directory: %v", readErr)
		return nil, fmt.Errorf("failed to read clusters directory: %w", readErr)
	}
	Debugf("List: found %d entries in clusters directory", len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			entryName := entry.Name()
			Debugf("List: processing directory entry: %s", entryName)

			// Check for legacy structure first: clustersDir/clusterName/.clusterName-config.yaml
			// This is for backward compatibility with old flat structure
			legacyConfigFile := filepath.Join(clustersDir, entryName, "."+entryName+"-config.yaml")
			Debugf("List: checking for legacy config file: %s", legacyConfigFile)
			if _, err := os.Stat(legacyConfigFile); err == nil {
				Debugf("List: found legacy config file for: %s", entryName)
				// Check if this is truly legacy (no infrastructure/clusters subdirs OR no applications subdirs)
				infraDir := filepath.Join(clustersDir, entryName, "infrastructure", "clusters")
				appsDir := filepath.Join(clustersDir, entryName, "applications", "overlays")
				hasInfra := false
				hasApps := false
				if _, err := os.Stat(infraDir); err == nil {
					hasInfra = true
					Debugf("List: %s has infrastructure directory", entryName)
				}
				if _, err := os.Stat(appsDir); err == nil {
					hasApps = true
					Debugf("List: %s has applications directory", entryName)
				}

				// If it has neither infrastructure nor applications subdirs, it's legacy flat structure
				if !hasInfra && !hasApps {
					Debugf("List: %s is legacy flat structure (no infra/apps dirs)", entryName)
					if !nameSet[entryName] {
						Debugf("List: adding legacy cluster: %s", entryName)
						names = append(names, entryName)
						nameSet[entryName] = true
					}
					continue // Skip organization check for this entry
				} else {
					Debugf("List: %s has subdirs (infra=%v, apps=%v), treating as organization", entryName, hasInfra, hasApps)
				}
			} else {
				Debugf("List: no legacy config file found for: %s", entryName)
			}

			// Check for organization-based structure
			// Look for clusters in: clustersDir/organization/infrastructure/clusters/<cluster>/.<cluster>-config.yaml
			orgDir := filepath.Join(clustersDir, entryName)
			infraClustersDir := filepath.Join(orgDir, "infrastructure", "clusters")
			Debugf("List: checking organization infrastructure/clusters directory: %s", infraClustersDir)

			if infraEntries, err := os.ReadDir(infraClustersDir); err == nil {
				Debugf("List: found %d entries in infrastructure/clusters directory for org: %s", len(infraEntries), entryName)
				for _, clusterEntry := range infraEntries {
					if clusterEntry.IsDir() {
						clusterName := clusterEntry.Name()
						// Check for config file at cluster directory level
						clusterConfigPath := filepath.Join(infraClustersDir, clusterName, "."+clusterName+"-config.yaml")
						Debugf("List: checking for config file: %s", clusterConfigPath)
						if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
							Debugf("List: found cluster config file for: %s", clusterName)
							// Format as organization/cluster
							fullName := entryName + "/" + clusterName
							if !nameSet[fullName] {
								Debugf("List: adding organization cluster: %s", fullName)
								names = append(names, fullName)
								nameSet[fullName] = true
							} else {
								Debugf("List: skipping duplicate cluster: %s", fullName)
							}
						}
					}
				}
			} else {
				Debugf("List: infrastructure/clusters directory does not exist for org %s: %v", entryName, err)
			}

			// Also check for config files at organization level (alternative location)
			if orgFiles, err := os.ReadDir(orgDir); err == nil {
				Debugf("List: found %d files in organization directory: %s", len(orgFiles), entryName)
				for _, orgFile := range orgFiles {
					if !orgFile.IsDir() && strings.HasPrefix(orgFile.Name(), ".") && strings.HasSuffix(orgFile.Name(), "-config.yaml") {
						Debugf("List: found organization-level config file: %s", orgFile.Name())
						// Extract cluster name from .<cluster>-config.yaml
						clusterName := strings.TrimPrefix(orgFile.Name(), ".")
						clusterName = strings.TrimSuffix(clusterName, "-config.yaml")
						Debugf("List: extracted cluster name: %s from file: %s", clusterName, orgFile.Name())
						if clusterName != "" {
							// Format as organization/cluster
							fullName := entryName + "/" + clusterName
							if !nameSet[fullName] {
								Debugf("List: adding organization cluster: %s", fullName)
								names = append(names, fullName)
								nameSet[fullName] = true
							} else {
								Debugf("List: skipping duplicate cluster: %s", fullName)
							}
						} else {
							Debugf("List: skipping cluster (name is empty)")
						}
					}
				}
			} else {
				Debugf("List: failed to read organization directory %s: %v", orgDir, err)
			}
		} else {
			Debugf("List: skipping non-directory entry: %s", entry.Name())
		}
	}

	// Sort lexically
	Debugf("List: sorting %d cluster names", len(names))
	if len(names) > 1 {
		sortStrings(names)
	}
	Debugf("List: returning %d total clusters", len(names))
	for i, name := range names {
		Debugf("List: final result[%d]: %s", i, name)
	}
	return names, nil
}

// simple string sorter to avoid pulling in a larger dependency.
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// activeClusterPath returns the absolute path to the file tracking
// the active cluster. This file stores the cluster name as plain
// text.
func activeClusterPath() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".active"), nil
}

// SetActive writes the given cluster name into the active marker file.
// If the name is empty, the marker file is removed.
//
// Inputs:
//   - name: The name of the cluster to set as active.
//
// Outputs:
//   - error: An error if the file cannot be written.
func SetActive(name string) error {
	path, err := activeClusterPath()
	if err != nil {
		return err
	}
	if name == "" {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(name), 0o600)
}

// GetActive reads the active cluster name from the marker file.
// If the file does not exist or is empty, it returns an empty string.
//
// Outputs:
//   - string: The active cluster name.
//   - error: An error if the file cannot be read.
func GetActive() (string, error) {
	path, err := activeClusterPath()
	if err != nil {
		return "", err
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return "", nil
		}
		return "", readErr
	}
	return strings.TrimSpace(string(data)), nil
}

// Validate performs a set of invariant checks on the configuration.
//
// Inputs:
//   - cfg: The configuration to validate.
//
// Outputs:
//   - []string: A list of error messages describing any validation failures.
//     If the list is empty, the configuration is valid.
func Validate(cfg Config) []string {
	var errs []string
	// Required cluster name and opencenter.gitops.git_dir
	if cfg.ClusterName() == "" {
		errs = append(errs, "opencenter.cluster.cluster_name must be set")
	}
	if cfg.GitOps().GitDir == "" {
		errs = append(errs, "opencenter.gitops.git_dir must be set")
	}
	// OpenTofu validation
	if cfg.OpenTofu.Enabled {
		if cfg.OpenTofu.Path == "" {
			errs = append(errs, "opentofu.path must be set when opentofu.enabled=true")
		}
		bt := strings.ToLower(strings.TrimSpace(cfg.OpenTofu.Backend.Type))
		if bt == "" {
			bt = "local"
		}
		switch bt {
		case "local":
			if cfg.OpenTofu.Backend.Local.Path == "" {
				errs = append(errs, "opentofu.backend.local.path must be set for local backend")
			}
		case "s3":
			s3 := cfg.OpenTofu.Backend.S3
			if s3.Bucket == "" || s3.Key == "" || s3.Region == "" {
				errs = append(errs, "opentofu.backend.s3 requires bucket, key, and region")
			}
			// When using S3 backend, AWS credentials must be provided via opencenter
			if strings.TrimSpace(cfg.OpenCenter.Cluster.AWSAccessKey) == "" || strings.TrimSpace(cfg.OpenCenter.Cluster.AWSSecretAccessKey) == "" {
				errs = append(errs, "opencenter.cluster.aws_access_key and opencenter.cluster.aws_secret_access_key must be set when opentofu.backend.type=s3")
			}
		default:
			errs = append(errs, "opentofu.backend.type must be 'local' or 's3'")
		}
	}
	// iac validation is intentionally minimal for variables.tf-aligned shape

	// Network plugin validation - ensure only one is enabled
	networkPlugins := []struct {
		name    string
		enabled bool
	}{
		{"Calico", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled},
		{"Cilium", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled},
		{"Kube-OVN", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled},
	}

	enabledCount := 0
	var enabledPlugins []string
	for _, plugin := range networkPlugins {
		if plugin.enabled {
			enabledCount++
			enabledPlugins = append(enabledPlugins, plugin.name)
		}
	}

	if enabledCount == 0 {
		errs = append(errs, "at least one network plugin (Calico, Cilium, or Kube-OVN) must be enabled")
	} else if enabledCount > 1 {
		errs = append(errs, fmt.Sprintf("only one network plugin can be enabled at a time, but found: %s", strings.Join(enabledPlugins, ", ")))
	}

	// Windows node validation - exclude Windows blocks when worker_count_windows = 0
	if cfg.OpenCenter.Cluster.Kubernetes.WorkerCountWindows == 0 {
		// Windows workers should be disabled when count is 0
		if cfg.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled {
			errs = append(errs, "windows_workers.enabled must be false when worker_count_windows is 0")
		}
	}

	// Validate services: only one of release or branch can be set
	for serviceName, serviceCfg := range cfg.OpenCenter.Services {
		if serviceCfg.Release != "" && serviceCfg.Branch != "" {
			errs = append(errs, fmt.Sprintf("service '%s': only one of 'release' or 'branch' can be set, not both", serviceName))
		}
	}

	// Validate managed services: only one of release or branch can be set
	for serviceName, serviceCfg := range cfg.OpenCenter.ManagedService {
		if serviceCfg.Release != "" && serviceCfg.Branch != "" {
			errs = append(errs, fmt.Sprintf("managed-service '%s': only one of 'release' or 'branch' can be set, not both", serviceName))
		}
	}

	// Validate GitOps: only one of release or branch can be set
	if cfg.OpenCenter.GitOps.Release != "" && cfg.OpenCenter.GitOps.Branch != "" {
		errs = append(errs, "gitops: only one of 'release' or 'branch' can be set, not both")
	}

	// Validate service secrets
	errs = append(errs, validateServiceSecretsSimple(cfg)...)

	return errs
}

// validateServiceSecretsSimple validates service-specific secrets configuration.
// This function checks that required secrets are present when corresponding services are enabled.
func validateServiceSecretsSimple(cfg Config) []string {
	var errs []string

	// Validate cert-manager secrets
	if svc, exists := cfg.OpenCenter.Services["cert-manager"]; exists && svc.Enabled {
		if cfg.Secrets.CertManager.AWSAccessKey == "" {
			errs = append(errs, "secrets.cert_manager.aws_access_key is required when cert-manager is enabled")
		}
		if cfg.Secrets.CertManager.AWSSecretAccessKey == "" {
			errs = append(errs, "secrets.cert_manager.aws_secret_access_key is required when cert-manager is enabled")
		}
	}

	// Validate loki secrets
	if svc, exists := cfg.OpenCenter.Services["loki"]; exists && svc.Enabled {
		if cfg.Secrets.Loki.SwiftPassword == "" {
			errs = append(errs, "secrets.loki.swift_password is required when loki is enabled")
		}
	}

	// Validate keycloak secrets
	if svc, exists := cfg.OpenCenter.Services["keycloak"]; exists && svc.Enabled {
		if cfg.Secrets.Keycloak.AdminPassword == "" {
			errs = append(errs, "secrets.keycloak.admin_password is required when keycloak is enabled")
		}
	}

	return errs
}

// ToJSON marshals the configuration to JSON. This is used for generating
// the JSON schema and for other tools that consume JSON.
//
// Outputs:
//   - []byte: The JSON-encoded configuration.
//   - error: An error if the configuration cannot be marshaled.
func (c Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}
