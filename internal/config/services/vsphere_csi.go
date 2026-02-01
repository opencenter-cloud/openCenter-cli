package services

import (
	"github.com/rackerlabs/opencenter-cli/internal/config/registry"
)

// VSphereCSIConfig extends BaseConfig with vSphere CSI configuration
type VSphereCSIConfig struct {
	BaseConfig `yaml:",inline"`

	// Datastore configuration for storage classes (deprecated - use StorageClasses)
	DataStore           string `yaml:"datastore_name" json:"datastore_name,omitempty" jsonschema:"description=Default datastore name for delete storage class (deprecated)"`
	DataStoreURL        string `yaml:"datastoreurl" json:"datastoreurl,omitempty" jsonschema:"description=Datastore name (deprecated)"`
	DeleteDataStoreUUID string `yaml:"delete_datastore_uuid" json:"delete_datastore_uuid,omitempty" jsonschema:"description=Datastore UUID for delete storage class (deprecated)"`
	RetainDataStoreName string `yaml:"retain_datastore_name" json:"retain_datastore_name,omitempty" jsonschema:"description=Datastore name for retain storage class (deprecated)"`
	RetainDataStoreUUID string `yaml:"retain_datastore_uuid" json:"retain_datastore_uuid,omitempty" jsonschema:"description=Datastore UUID for retain storage class (deprecated)"`

	// Storage class configuration
	StorageClasses []VSphereStorageClass `yaml:"storage_classes" json:"storage_classes,omitempty" jsonschema:"description=List of storage classes to create"`
}

// VSphereStorageClass represents a vSphere storage class configuration
type VSphereStorageClass struct {
	Name              string `yaml:"name" json:"name" jsonschema:"description=Storage class name,required"`
	DatastoreURL      string `yaml:"datastore_url" json:"datastore_url" jsonschema:"description=Datastore URL (ds:///vmfs/volumes/...),required"`
	ReclaimPolicy     string `yaml:"reclaim_policy" json:"reclaim_policy,omitempty" jsonschema:"description=Reclaim policy,enum=Retain,enum=Delete,default=Retain"`
	VolumeBindingMode string `yaml:"volume_binding_mode" json:"volume_binding_mode,omitempty" jsonschema:"description=Volume binding mode,enum=Immediate,enum=WaitForFirstConsumer,default=Immediate"`
	AllowExpansion    bool   `yaml:"allow_expansion" json:"allow_expansion,omitempty" jsonschema:"description=Allow volume expansion,default=true"`
}

func init() {
	registry.RegisterServiceConfig("vsphere-csi", VSphereCSIConfig{})
}
