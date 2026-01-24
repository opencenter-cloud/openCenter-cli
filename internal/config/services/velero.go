package services

import (
	"github.com/rackerlabs/opencenter-cli/internal/config/registry"
)

// VeleroConfig extends BaseConfig with Velero-specific configuration
type VeleroConfig struct {
	BaseConfig `yaml:",inline"`

	BackupBucket   string `yaml:"velero_backup_bucket" json:"velero_backup_bucket,omitempty" jsonschema:"description=Velero backup bucket name"`
	Region         string `yaml:"velero_region" json:"velero_region,omitempty" jsonschema:"description=Velero backup region"`
	StorageType    string `yaml:"storage_type" json:"storage_type,omitempty" jsonschema:"description=Velero storage backend type (s3, swift, gcs, azure),enum=s3,enum=swift,enum=gcs,enum=azure,default=s3"`
}

func init() {
	registry.RegisterServiceConfig("velero", VeleroConfig{})
}
