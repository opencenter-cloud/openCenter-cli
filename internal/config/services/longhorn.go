package services

import (
	"github.com/rackerlabs/opencenter-cli/internal/config/registry"
)

// LonghornConfig extends BaseConfig with Longhorn-specific configuration
type LonghornConfig struct {
	BaseConfig `yaml:",inline"`

	// Storage configuration
	DefaultReplicaCount               int    `yaml:"default_replica_count" json:"default_replica_count,omitempty" jsonschema:"description=Default number of replicas for volumes,default=3"`
	DefaultDataPath                   string `yaml:"default_data_path" json:"default_data_path,omitempty" jsonschema:"description=Default path for storing volume data,default=/var/lib/longhorn"`
	StorageOverProvisioningPercentage int    `yaml:"storage_over_provisioning_percentage" json:"storage_over_provisioning_percentage,omitempty" jsonschema:"description=Storage over-provisioning percentage,default=200"`
	StorageMinimalAvailablePercentage int    `yaml:"storage_minimal_available_percentage" json:"storage_minimal_available_percentage,omitempty" jsonschema:"description=Minimal available storage percentage,default=25"`
	BackupTarget                      string `yaml:"backup_target" json:"backup_target,omitempty" jsonschema:"description=Backup target URL (s3:// or nfs://)"`
	BackupTargetCredentialSecret      string `yaml:"backup_target_credential_secret" json:"backup_target_credential_secret,omitempty" jsonschema:"description=Secret name for backup target credentials"`
}

func init() {
	registry.RegisterServiceConfig("longhorn", LonghornConfig{})
}
