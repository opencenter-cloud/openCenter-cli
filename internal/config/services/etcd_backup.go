package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// EtcdBackupConfig extends BaseConfig with etcd backup configuration.
type EtcdBackupConfig struct {
	BaseConfig `yaml:",inline"`

	S3Host   string `yaml:"s3_host,omitempty" json:"s3_host,omitempty" jsonschema:"description=S3-compatible endpoint host"`
	S3Region string `yaml:"s3_region,omitempty" json:"s3_region,omitempty" jsonschema:"description=S3 region"`
}

func init() {
	registry.RegisterServiceConfig("etcd-backup", EtcdBackupConfig{})
}
