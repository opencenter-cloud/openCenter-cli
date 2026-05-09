package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// HarborConfig extends BaseConfig with Harbor-specific configuration.
type HarborConfig struct {
	BaseConfig `yaml:",inline"`

	// Access
	Hostname    string `yaml:"hostname,omitempty" json:"hostname,omitempty" jsonschema:"description=Harbor external hostname"`
	ExternalURL string `yaml:"external_url,omitempty" json:"external_url,omitempty" jsonschema:"description=External URL for Harbor"`

	// Storage
	StorageType        string `yaml:"storage_type,omitempty" json:"storage_type,omitempty" jsonschema:"description=Storage backend type,enum=filesystem,enum=s3,enum=swift,default=filesystem"`
	RegistryVolumeSize int    `yaml:"registry_volume_size,omitempty" json:"registry_volume_size,omitempty" jsonschema:"description=Registry persistent volume size in GB,default=100"`
	S3Bucket           string `yaml:"s3_bucket,omitempty" json:"s3_bucket,omitempty" jsonschema:"description=S3 bucket name for image storage"`
	S3Region           string `yaml:"s3_region,omitempty" json:"s3_region,omitempty" jsonschema:"description=S3 region"`

	// Database
	DatabaseType string `yaml:"database_type,omitempty" json:"database_type,omitempty" jsonschema:"description=Database type,enum=internal,enum=external,default=internal"`
	DatabaseHost string `yaml:"database_host,omitempty" json:"database_host,omitempty" jsonschema:"description=External database host"`
	DatabasePort int    `yaml:"database_port,omitempty" json:"database_port,omitempty" jsonschema:"description=External database port"`
	DatabaseName string `yaml:"database_name,omitempty" json:"database_name,omitempty" jsonschema:"description=External database name"`
	DatabaseUser string `yaml:"database_user,omitempty" json:"database_user,omitempty" jsonschema:"description=External database user"`

	// TLS
	EmitCertificate bool `yaml:"emit_certificate,omitempty" json:"emit_certificate,omitempty" jsonschema:"description=Render the Harbor TLS certificate manifest"`
}

func init() {
	registry.RegisterServiceConfig("harbor", HarborConfig{})
}
