package config

// Storage describes block volume storage sizes and types for various node roles.
type Storage struct {
	MasterNodeBFV        BFV `yaml:"master_node_bfv" json:"master_node_bfv"`
	WorkerNodeBFV        BFV `yaml:"worker_node_bfv" json:"worker_node_bfv"`
	WorkerNodeBFVWindows BFV `yaml:"worker_node_bfv_windows" json:"worker_node_bfv_windows"`
}

// BFV describes the block volume size and type.
type BFV struct {
	Size int    `yaml:"size" json:"size"`
	Type string `yaml:"type" json:"type"`
	WorkerVolumeSize            int              `yaml:"worker_volume_size,omitempty" json:"worker_volume_size,omitempty" jsonschema:"description=Worker node boot volume size in GB,default=100"`
	WorkerVolumeDestinationType string           `yaml:"worker_volume_destination_type,omitempty" json:"worker_volume_destination_type,omitempty" jsonschema:"description=Worker node boot volume destination type,default=volume"`
	WorkerVolumeSourceType      string           `yaml:"worker_volume_source_type,omitempty" json:"worker_volume_source_type,omitempty" jsonschema:"description=Worker node boot volume source type,default=image"`
	WorkerVolumeType            string           `yaml:"worker_volume_type,omitempty" json:"worker_volume_type,omitempty" jsonschema:"description=Worker node boot volume type,default=HA-Performance"`
	AdditionalBlockDevices      []map[string]any `yaml:"additional_block_devices,omitempty" json:"additional_block_devices,omitempty" jsonschema:"description=Additional block devices for worker nodes"`
}

// Windows holds Windows-specific settings.
type Windows struct {
	User          string `yaml:"user" json:"user"`
	AdminPassword string `yaml:"admin_password" json:"admin_password"`
}

// StorageConfig represents the storage configuration for the cluster
type StorageConfig struct {
	DefaultStorageClass         string           `yaml:"default_storage_class,omitempty" json:"default_storage_class,omitempty" jsonschema:"description=Default storage class for persistent volumes,default=csi-cinder-sc-delete"`
	WorkerVolumeSize            int              `yaml:"worker_volume_size,omitempty" json:"worker_volume_size,omitempty" jsonschema:"description=Worker node boot volume size in GB,default=40"`
	WorkerVolumeDestinationType string           `yaml:"worker_volume_destination_type,omitempty" json:"worker_volume_destination_type,omitempty" jsonschema:"description=Worker node boot volume destination type,default=volume"`
	WorkerVolumeSourceType      string           `yaml:"worker_volume_source_type,omitempty" json:"worker_volume_source_type,omitempty" jsonschema:"description=Worker node boot volume source type,default=image"`
	WorkerVolumeType            string           `yaml:"worker_volume_type,omitempty" json:"worker_volume_type,omitempty" jsonschema:"description=Worker node boot volume type,default=HA-Standard"`
	AdditionalBlockDevices      []map[string]any `yaml:"additional_block_devices,omitempty" json:"additional_block_devices,omitempty" jsonschema:"description=Additional block devices for worker nodes"`
}
