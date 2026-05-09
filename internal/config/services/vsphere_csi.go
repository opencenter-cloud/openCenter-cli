package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// VSphereCSIConfig extends BaseConfig with vSphere CSI configuration.
type VSphereCSIConfig struct {
	BaseConfig `yaml:",inline"`

	StorageClasses []VSphereStorageClass `yaml:"storage_classes,omitempty" json:"storage_classes,omitempty" jsonschema:"description=List of storage classes to create"`
}

// VSphereStorageClass represents a vSphere storage class configuration.
type VSphereStorageClass struct {
	Name              string `yaml:"name" json:"name" jsonschema:"description=Storage class name,required"`
	DatastoreURL      string `yaml:"datastore_url" json:"datastore_url" jsonschema:"description=Datastore URL (ds:///vmfs/volumes/...),required"`
	ReclaimPolicy     string `yaml:"reclaim_policy,omitempty" json:"reclaim_policy,omitempty" jsonschema:"description=Reclaim policy,enum=Retain,enum=Delete,default=Retain"`
	VolumeBindingMode string `yaml:"volume_binding_mode,omitempty" json:"volume_binding_mode,omitempty" jsonschema:"description=Volume binding mode,enum=Immediate,enum=WaitForFirstConsumer,default=Immediate"`
	AllowExpansion    bool   `yaml:"allow_expansion,omitempty" json:"allow_expansion,omitempty" jsonschema:"description=Allow volume expansion,default=true"`
}

func init() {
	registry.RegisterServiceConfig("vsphere-csi", VSphereCSIConfig{})
}
