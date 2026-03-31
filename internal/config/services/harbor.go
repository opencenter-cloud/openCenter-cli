package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// HarborConfig extends BaseConfig with Harbor-specific configuration
type HarborConfig struct {
	BaseConfig `yaml:",inline"`

	// Storage configuration
	StorageType        string `yaml:"storage_type" json:"storage_type,omitempty" jsonschema:"description=Storage backend type,enum=filesystem,enum=s3,enum=swift,default=filesystem"`
	RegistryVolumeSize int    `yaml:"registry_volume_size" json:"registry_volume_size,omitempty" jsonschema:"description=Registry persistent volume size in GB,default=100"`

	// Database configuration
	DatabaseType string `yaml:"database_type" json:"database_type,omitempty" jsonschema:"description=Database type,enum=internal,enum=external,default=internal"`
	DatabaseHost string `yaml:"database_host" json:"database_host,omitempty" jsonschema:"description=External database host"`
	DatabasePort int    `yaml:"database_port" json:"database_port,omitempty" jsonschema:"description=External database port"`
	DatabaseName string `yaml:"database_name" json:"database_name,omitempty" jsonschema:"description=External database name"`
	DatabaseUser string `yaml:"database_user" json:"database_user,omitempty" jsonschema:"description=External database user"`

	// S3 storage configuration
	S3Bucket string `yaml:"s3_bucket" json:"s3_bucket,omitempty" jsonschema:"description=S3 bucket name for image storage"`
	S3Region string `yaml:"s3_region" json:"s3_region,omitempty" jsonschema:"description=S3 region"`

	// Admin configuration
	AdminPassword   string `yaml:"admin_password" json:"admin_password,omitempty" jsonschema:"description=Harbor admin password,secret=true"`
	ExternalURL     string `yaml:"external_url" json:"external_url,omitempty" jsonschema:"description=External URL for Harbor"`
	EmitCertificate bool   `yaml:"emit_certificate,omitempty" json:"emit_certificate,omitempty" jsonschema:"description=Render the Harbor TLS certificate manifest"`
}

func init() {
	registry.RegisterServiceConfig("harbor", HarborConfig{})
}
