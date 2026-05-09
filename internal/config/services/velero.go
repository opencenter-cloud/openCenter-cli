package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// VeleroConfig extends BaseConfig with Velero-specific configuration.
type VeleroConfig struct {
	BaseConfig `yaml:",inline"`

	BackupBucket string `yaml:"backup_bucket,omitempty" json:"backup_bucket,omitempty" jsonschema:"description=Velero backup bucket name"`
	Region       string `yaml:"region,omitempty" json:"region,omitempty" jsonschema:"description=Velero backup region"`
	StorageType  string `yaml:"storage_type,omitempty" json:"storage_type,omitempty" jsonschema:"description=Velero storage backend type,enum=s3,enum=swift,enum=gcs,enum=azure,default=s3"`
}

func init() {
	registry.RegisterServiceConfig("velero", VeleroConfig{})
}
