package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// LokiConfig extends BaseConfig with Loki-specific configuration
type LokiConfig struct {
	BaseConfig `yaml:",inline"`

	// Storage configuration
	StorageType  string `yaml:"loki_storage_type" json:"loki_storage_type,omitempty" jsonschema:"description=Loki storage backend type (s3 or swift),enum=s3,enum=swift,default=swift"`
	BucketName   string `yaml:"loki_bucket_name" json:"loki_bucket_name,omitempty" jsonschema:"description=Loki storage bucket/container name"`
	VolumeSize   int    `yaml:"loki_volume_size" json:"loki_volume_size,omitempty" jsonschema:"description=Loki persistent volume size in GB"`
	StorageClass string `yaml:"loki_storage_class" json:"loki_storage_class,omitempty" jsonschema:"description=Loki storage class"`

	// Swift storage fields
	SwiftAuthURL                 string `yaml:"swift_auth_url" json:"swift_auth_url,omitempty" jsonschema:"description=Swift Keystone V3 authentication URL (must end in /v3)"`
	SwiftRegion                  string `yaml:"swift_region" json:"swift_region,omitempty" jsonschema:"description=Swift region name"`
	SwiftAuthVersion             int    `yaml:"swift_auth_version" json:"swift_auth_version,omitempty" jsonschema:"description=Swift authentication version,default=3"`
	SwiftApplicationCredentialID string `yaml:"swift_application_credential_id" json:"swift_application_credential_id,omitempty" jsonschema:"description=Swift application credential ID (UUID)"`
	SwiftContainerName           string `yaml:"swift_container_name" json:"swift_container_name,omitempty" jsonschema:"description=Swift container name for Loki logs"`
	SwiftUserDomainName          string `yaml:"swift_user_domain_name" json:"swift_user_domain_name,omitempty" jsonschema:"description=Swift user domain name"`
	SwiftDomainName              string `yaml:"swift_domain_name" json:"swift_domain_name,omitempty" jsonschema:"description=Swift domain name"`
	// Deprecated Swift fields (mapped to standard ones where possible or kept for compatibility)
	SwiftUsername    string `yaml:"swift_username" json:"swift_username,omitempty" jsonschema:"description=Swift username (deprecated)"`
	SwiftProjectName string `yaml:"swift_project_name" json:"swift_project_name,omitempty" jsonschema:"description=Swift project name (deprecated)"`

	// S3 storage fields
	S3Endpoint       string `yaml:"loki_s3_endpoint" json:"loki_s3_endpoint,omitempty" jsonschema:"description=S3 endpoint URL"`
	S3Region         string `yaml:"loki_s3_region" json:"loki_s3_region,omitempty" jsonschema:"description=S3 region"`
	S3ForcePathStyle bool   `yaml:"loki_s3_force_path_style" json:"loki_s3_force_path_style,omitempty" jsonschema:"description=Force S3 path style"`
	S3Insecure       bool   `yaml:"loki_s3_insecure" json:"loki_s3_insecure,omitempty" jsonschema:"description=Allow insecure S3 connections"`

	// Backward compatibility fields that were in the god object
	LokiStorageType string `yaml:"-" json:"-"` // Alias for StorageType
	LokiVolumeSize  int    `yaml:"-" json:"-"` // Alias for VolumeSize
}

func init() {
	registry.RegisterServiceConfig("loki", LokiConfig{})
}
