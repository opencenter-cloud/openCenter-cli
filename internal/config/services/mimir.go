package services

import "github.com/opencenter-cloud/opencenter-cli/internal/config/registry"

// MimirConfig extends BaseConfig with Mimir blocks-storage configuration.
//
// Swift fields are retained because existing OpenStack deployments may still
// use Mimir's legacy Swift blocks backend. New deployments can select the
// portable S3-compatible backend instead.
type MimirConfig struct {
	BaseConfig `yaml:",inline"`

	// Storage
	StorageType            string `yaml:"storage_type,omitempty" json:"storage_type,omitempty" jsonschema:"description=Mimir storage backend type,enum=s3,enum=swift,default=swift"`
	BucketName             string `yaml:"bucket_name,omitempty" json:"bucket_name,omitempty" jsonschema:"description=Mimir blocks storage bucket/container name"`
	RulerBucketName        string `yaml:"ruler_bucket_name,omitempty" json:"ruler_bucket_name,omitempty" jsonschema:"description=Optional Mimir ruler storage bucket/container name"`
	AlertmanagerBucketName string `yaml:"alertmanager_bucket_name,omitempty" json:"alertmanager_bucket_name,omitempty" jsonschema:"description=Optional Mimir alertmanager storage bucket/container name"`

	// S3 backend
	S3Endpoint       string `yaml:"s3_endpoint,omitempty" json:"s3_endpoint,omitempty" jsonschema:"description=S3 endpoint URL"`
	S3Region         string `yaml:"s3_region,omitempty" json:"s3_region,omitempty" jsonschema:"description=S3 region"`
	S3CredentialID   string `yaml:"s3_credential_id,omitempty" json:"s3_credential_id,omitempty" jsonschema:"description=OpenStack EC2 credential ID"`
	S3ForcePathStyle bool   `yaml:"s3_force_path_style,omitempty" json:"s3_force_path_style,omitempty" jsonschema:"description=Force S3 path style"`
	S3Insecure       bool   `yaml:"s3_insecure,omitempty" json:"s3_insecure,omitempty" jsonschema:"description=Allow insecure S3 connections"`

	// Swift backend
	SwiftAuthURL                 string `yaml:"swift_auth_url,omitempty" json:"swift_auth_url,omitempty" jsonschema:"description=Swift Keystone V3 authentication URL (must end in /v3)"`
	SwiftRegion                  string `yaml:"swift_region,omitempty" json:"swift_region,omitempty" jsonschema:"description=Swift region name"`
	SwiftAuthVersion             int    `yaml:"swift_auth_version,omitempty" json:"swift_auth_version,omitempty" jsonschema:"description=Swift authentication version,default=3"`
	SwiftUsername                string `yaml:"swift_username,omitempty" json:"swift_username,omitempty" jsonschema:"description=Swift Keystone username (service user) for username/password auth"`
	SwiftProjectName             string `yaml:"swift_project_name,omitempty" json:"swift_project_name,omitempty" jsonschema:"description=Swift Keystone project name"`
	SwiftProjectDomainName       string `yaml:"swift_project_domain_name,omitempty" json:"swift_project_domain_name,omitempty" jsonschema:"description=Swift Keystone project domain name (defaults to swift_domain_name)"`
	SwiftContainerName           string `yaml:"swift_container_name,omitempty" json:"swift_container_name,omitempty" jsonschema:"description=Swift container name for Mimir blocks"`
	SwiftUserDomainName          string `yaml:"swift_user_domain_name,omitempty" json:"swift_user_domain_name,omitempty" jsonschema:"description=Swift user domain name"`
	SwiftDomainName              string `yaml:"swift_domain_name,omitempty" json:"swift_domain_name,omitempty" jsonschema:"description=Swift domain name"`
	SwiftApplicationCredentialID string `yaml:"swift_application_credential_id,omitempty" json:"swift_application_credential_id,omitempty" jsonschema:"description=Swift application credential ID"`
}

func init() {
	registry.RegisterServiceConfig("mimir", MimirConfig{})
}
