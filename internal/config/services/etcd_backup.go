package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// EtcdBackupConfig extends BaseConfig with Etcd Backup configuration
type EtcdBackupConfig struct {
	BaseConfig `yaml:",inline"`

	S3Host   string `yaml:"s3_host" json:"s3_host,omitempty" jsonschema:"description=S3 host"`
	S3Region string `yaml:"s3_region" json:"s3_region,omitempty" jsonschema:"description=S3 region"`
}

func init() {
	registry.RegisterServiceConfig("etcd-backup", EtcdBackupConfig{})
}
