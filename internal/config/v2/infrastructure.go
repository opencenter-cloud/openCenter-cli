// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2

// InfrastructureConfig represents provider-agnostic infrastructure with provider-specific extensions.
// Requirements: 2.1, 3.1, 4.1, 9.1, 9.2
type InfrastructureConfig struct {
	Provider            string           `yaml:"provider" json:"provider" validate:"required,oneof=openstack aws gcp azure baremetal vsphere"`
	SSH                 SSHConfig        `yaml:"ssh" json:"ssh" validate:"required"`
	OSVersion           string           `yaml:"os_version" json:"os_version" validate:"required"`
	ServerGroupAffinity []string         `yaml:"server_group_affinity,omitempty" json:"server_group_affinity,omitempty"`
	K8sAPIIP            string           `yaml:"k8s_api_ip,omitempty" json:"k8s_api_ip,omitempty" validate:"omitempty,ipv4"`
	NodeNaming          NodeNamingConfig `yaml:"node_naming,omitempty" json:"node_naming,omitempty"`
	Bastion             BastionConfig    `yaml:"bastion,omitempty" json:"bastion,omitempty"`
	Networking          NetworkingConfig `yaml:"networking" json:"networking" validate:"required"`
	Compute             ComputeConfig    `yaml:"compute" json:"compute" validate:"required"`
	Storage             StorageConfig    `yaml:"storage" json:"storage" validate:"required"`
	Cloud               CloudConfig      `yaml:"cloud" json:"cloud" validate:"required"`
}

// SSHConfig represents SSH configuration for cluster nodes.
type SSHConfig struct {
	AuthorizedKeys []string `yaml:"authorized_keys" json:"authorized_keys" validate:"required,min=1"`
	Username       string   `yaml:"username,omitempty" json:"username,omitempty"`
}

// NodeNamingConfig represents node naming configuration.
type NodeNamingConfig struct {
	Prefix string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Suffix string `yaml:"suffix,omitempty" json:"suffix,omitempty"`
}

// BastionConfig represents bastion host configuration.
type BastionConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Flavor  string `yaml:"flavor,omitempty" json:"flavor,omitempty" validate:"required_if=Enabled true"`
	Image   string `yaml:"image,omitempty" json:"image,omitempty" validate:"required_if=Enabled true"`
}

// NetworkingConfig represents infrastructure networking configuration.
// Requirements: 2.1, 3.1, 3.2
type NetworkingConfig struct {
	// Network topology
	SubnetNodes         string `yaml:"subnet_nodes" json:"subnet_nodes" validate:"required,cidrv4"`
	AllocationPoolStart string `yaml:"allocation_pool_start" json:"allocation_pool_start" validate:"required,ipv4"`
	AllocationPoolEnd   string `yaml:"allocation_pool_end" json:"allocation_pool_end" validate:"required,ipv4"`
	Gateway             string `yaml:"gateway,omitempty" json:"gateway,omitempty" validate:"omitempty,ipv4"`

	// High availability
	VRRPIP      string `yaml:"vrrp_ip" json:"vrrp_ip" validate:"required_if=VRRPEnabled true,omitempty,ipv4"`
	VRRPEnabled bool   `yaml:"vrrp_enabled" json:"vrrp_enabled"`

	// Load balancing
	LoadbalancerProvider string `yaml:"loadbalancer_provider" json:"loadbalancer_provider" validate:"required,oneof=ovn octavia metallb cloud-native"`

	// DNS
	UseDesignate   bool     `yaml:"use_designate" json:"use_designate"`
	DNSZoneName    string   `yaml:"dns_zone_name" json:"dns_zone_name" validate:"required,fqdn"`
	DNSNameservers []string `yaml:"dns_nameservers" json:"dns_nameservers" validate:"required,min=1,dive,ipv4"`

	// Time synchronization
	NTPServers []string `yaml:"ntp_servers" json:"ntp_servers" validate:"required,min=1"`

	// Security
	Security NetworkSecurityConfig `yaml:"security,omitempty" json:"security,omitempty"`

	// VLAN
	VLAN VLANConfig `yaml:"vlan,omitempty" json:"vlan,omitempty"`
}

// NetworkSecurityConfig represents network security configuration.
type NetworkSecurityConfig struct {
	AllowedCIDRs []string `yaml:"allowed_cidrs,omitempty" json:"allowed_cidrs,omitempty" validate:"dive,cidrv4"`
	DenyAll      bool     `yaml:"deny_all" json:"deny_all"`
}

// VLANConfig represents VLAN configuration.
type VLANConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	ID      int  `yaml:"id,omitempty" json:"id,omitempty" validate:"omitempty,min=1,max=4094"`
}

// ComputeConfig represents compute resource configuration.
// Requirements: 9.1
type ComputeConfig struct {
	// Instance flavors
	FlavorBastion       string `yaml:"flavor_bastion,omitempty" json:"flavor_bastion,omitempty"`
	FlavorMaster        string `yaml:"flavor_master,omitempty" json:"flavor_master,omitempty"`
	FlavorWorker        string `yaml:"flavor_worker,omitempty" json:"flavor_worker,omitempty"`
	FlavorWorkerWindows string `yaml:"flavor_worker_windows,omitempty" json:"flavor_worker_windows,omitempty"`

	// Node counts
	MasterCount        int `yaml:"master_count" json:"master_count" validate:"min=0"`
	WorkerCount        int `yaml:"worker_count" json:"worker_count" validate:"min=0"`
	WorkerCountWindows int `yaml:"worker_count_windows" json:"worker_count_windows" validate:"min=0"`

	// Additional worker pools
	AdditionalServerPoolsWorker []WorkerPoolConfig `yaml:"additional_server_pools_worker,omitempty" json:"additional_server_pools_worker,omitempty"`
}

// WorkerPoolConfig represents additional worker pool configuration.
// Requirements: 9.5
type WorkerPoolConfig struct {
	Name              string            `yaml:"name" json:"name" validate:"required,dns1123"`
	Count             int               `yaml:"count" json:"count" validate:"required,min=1"`
	Flavor            string            `yaml:"flavor" json:"flavor" validate:"required"`
	Image             string            `yaml:"image,omitempty" json:"image,omitempty"`
	BootVolume        VolumeConfig      `yaml:"boot_volume,omitempty" json:"boot_volume,omitempty"`
	AdditionalVolumes []VolumeConfig    `yaml:"additional_volumes,omitempty" json:"additional_volumes,omitempty"`
	Labels            map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Taints            []TaintConfig     `yaml:"taints,omitempty" json:"taints,omitempty"`
}

// VolumeConfig represents volume configuration.
type VolumeConfig struct {
	Size                int    `yaml:"size" json:"size" validate:"required,min=1"`
	Type                string `yaml:"type,omitempty" json:"type,omitempty"`
	DestinationType     string `yaml:"destination_type,omitempty" json:"destination_type,omitempty" validate:"omitempty,oneof=volume local"`
	SourceType          string `yaml:"source_type,omitempty" json:"source_type,omitempty" validate:"omitempty,oneof=image volume snapshot"`
	DeleteOnTermination bool   `yaml:"delete_on_termination" json:"delete_on_termination"`
}

// TaintConfig represents Kubernetes node taint configuration.
type TaintConfig struct {
	Key    string `yaml:"key" json:"key" validate:"required"`
	Value  string `yaml:"value,omitempty" json:"value,omitempty"`
	Effect string `yaml:"effect" json:"effect" validate:"required,oneof=NoSchedule PreferNoSchedule NoExecute"`
}

// StorageConfig represents storage configuration.
// Requirements: 9.2
type StorageConfig struct {
	DefaultStorageClass             string              `yaml:"default_storage_class" json:"default_storage_class" validate:"required"`
	WorkerVolumeSize                int                 `yaml:"worker_volume_size" json:"worker_volume_size" validate:"required,min=1"`
	WorkerVolumeDestinationType     string              `yaml:"worker_volume_destination_type" json:"worker_volume_destination_type" validate:"required,oneof=volume local"`
	WorkerVolumeSourceType          string              `yaml:"worker_volume_source_type" json:"worker_volume_source_type" validate:"required,oneof=image volume snapshot"`
	WorkerVolumeType                string              `yaml:"worker_volume_type" json:"worker_volume_type" validate:"required"`
	WorkerVolumeDeleteOnTermination bool                `yaml:"worker_volume_delete_on_termination" json:"worker_volume_delete_on_termination"`
	MasterVolumeSize                int                 `yaml:"master_volume_size" json:"master_volume_size" validate:"min=0"`
	MasterVolumeDestinationType     string              `yaml:"master_volume_destination_type,omitempty" json:"master_volume_destination_type,omitempty"`
	MasterVolumeSourceType          string              `yaml:"master_volume_source_type,omitempty" json:"master_volume_source_type,omitempty"`
	MasterVolumeType                string              `yaml:"master_volume_type,omitempty" json:"master_volume_type,omitempty"`
	MasterVolumeDeleteOnTermination bool                `yaml:"master_volume_delete_on_termination" json:"master_volume_delete_on_termination"`
	AdditionalBlockDevices          []BlockDeviceConfig `yaml:"additional_block_devices,omitempty" json:"additional_block_devices,omitempty"`
}

// BlockDeviceConfig represents additional block device configuration.
type BlockDeviceConfig struct {
	Name                string `yaml:"name" json:"name" validate:"required"`
	Size                int    `yaml:"size" json:"size" validate:"required,min=1"`
	Type                string `yaml:"type,omitempty" json:"type,omitempty"`
	MountPath           string `yaml:"mount_path,omitempty" json:"mount_path,omitempty"`
	DeleteOnTermination bool   `yaml:"delete_on_termination" json:"delete_on_termination"`
}

// CloudConfig represents polymorphic provider-specific configuration.
// Requirements: 4.1
type CloudConfig struct {
	OpenStack *OpenStackCloudConfig `yaml:"openstack,omitempty" json:"openstack,omitempty"`
	AWS       *AWSCloudConfig       `yaml:"aws,omitempty" json:"aws,omitempty"`
	GCP       *GCPCloudConfig       `yaml:"gcp,omitempty" json:"gcp,omitempty"`
	Azure     *AzureCloudConfig     `yaml:"azure,omitempty" json:"azure,omitempty"`
	VMware    *VMwareCloudConfig    `yaml:"vmware,omitempty" json:"vmware,omitempty"`
}

// OpenStackCloudConfig represents OpenStack-specific configuration.
// Requirements: 4.3
type OpenStackCloudConfig struct {
	AuthURL           string   `yaml:"auth_url" json:"auth_url" validate:"required,url"`
	Region            string   `yaml:"region" json:"region" validate:"required"`
	ProjectID         string   `yaml:"project_id" json:"project_id" validate:"required"`
	ProjectName       string   `yaml:"project_name,omitempty" json:"project_name,omitempty"`
	UserDomainName    string   `yaml:"user_domain_name,omitempty" json:"user_domain_name,omitempty"`
	ProjectDomainName string   `yaml:"project_domain_name,omitempty" json:"project_domain_name,omitempty"`
	ImageID           string   `yaml:"image_id" json:"image_id" validate:"required"`
	NetworkID         string   `yaml:"network_id" json:"network_id" validate:"required"`
	SubnetID          string   `yaml:"subnet_id,omitempty" json:"subnet_id,omitempty"`
	FloatingIPPool    string   `yaml:"floating_ip_pool,omitempty" json:"floating_ip_pool,omitempty"`
	AvailabilityZones []string `yaml:"availability_zones,omitempty" json:"availability_zones,omitempty"`
	UseOctavia        bool     `yaml:"use_octavia" json:"use_octavia"`
	UseDesignate      bool     `yaml:"use_designate" json:"use_designate"`
}

// AWSCloudConfig represents AWS-specific configuration.
// Requirements: 4.4
type AWSCloudConfig struct {
	Region            string   `yaml:"region" json:"region" validate:"required"`
	VPCID             string   `yaml:"vpc_id" json:"vpc_id" validate:"required"`
	SubnetIDs         []string `yaml:"subnet_ids" json:"subnet_ids" validate:"required,min=1"`
	AMIID             string   `yaml:"ami_id" json:"ami_id" validate:"required"`
	AvailabilityZones []string `yaml:"availability_zones,omitempty" json:"availability_zones,omitempty"`
	KeyPairName       string   `yaml:"key_pair_name,omitempty" json:"key_pair_name,omitempty"`
	SecurityGroupIDs  []string `yaml:"security_group_ids,omitempty" json:"security_group_ids,omitempty"`
}

// GCPCloudConfig represents GCP-specific configuration.
// Requirements: 4.5
type GCPCloudConfig struct {
	Project           string   `yaml:"project" json:"project" validate:"required"`
	Region            string   `yaml:"region" json:"region" validate:"required"`
	Zone              string   `yaml:"zone,omitempty" json:"zone,omitempty"`
	Network           string   `yaml:"network" json:"network" validate:"required"`
	Subnetwork        string   `yaml:"subnetwork" json:"subnetwork" validate:"required"`
	ImageFamily       string   `yaml:"image_family" json:"image_family" validate:"required"`
	AvailabilityZones []string `yaml:"availability_zones,omitempty" json:"availability_zones,omitempty"`
}

// AzureCloudConfig represents Azure-specific configuration.
// Requirements: 4.6
type AzureCloudConfig struct {
	SubscriptionID    string   `yaml:"subscription_id" json:"subscription_id" validate:"required"`
	ResourceGroup     string   `yaml:"resource_group" json:"resource_group" validate:"required"`
	Location          string   `yaml:"location" json:"location" validate:"required"`
	VNetName          string   `yaml:"vnet_name" json:"vnet_name" validate:"required"`
	SubnetName        string   `yaml:"subnet_name" json:"subnet_name" validate:"required"`
	ImageReference    string   `yaml:"image_reference" json:"image_reference" validate:"required"`
	AvailabilityZones []string `yaml:"availability_zones,omitempty" json:"availability_zones,omitempty"`
}

// VMwareCloudConfig represents VMware-specific configuration.
type VMwareCloudConfig struct {
	VCenterServer string `yaml:"vcenter_server" json:"vcenter_server" validate:"required,hostname|ip"`
	Datacenter    string `yaml:"datacenter" json:"datacenter" validate:"required"`
	Cluster       string `yaml:"cluster,omitempty" json:"cluster,omitempty"`
	Datastore     string `yaml:"datastore" json:"datastore" validate:"required"`
	Network       string `yaml:"network" json:"network" validate:"required"`
	Template      string `yaml:"template" json:"template" validate:"required"`
	Folder        string `yaml:"folder,omitempty" json:"folder,omitempty"`
}
