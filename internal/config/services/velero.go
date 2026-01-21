package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// VeleroConfig extends BaseConfig with Velero-specific configuration
type VeleroConfig struct {
	BaseConfig `yaml:",inline"`

	BackupBucket string `yaml:"velero_backup_bucket" json:"velero_backup_bucket,omitempty" jsonschema:"description=Velero backup bucket name"`
	Region       string `yaml:"velero_region" json:"velero_region,omitempty" jsonschema:"description=Velero backup region"`
}

func init() {
	registry.RegisterServiceConfig("velero", VeleroConfig{})
}
