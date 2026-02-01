package config

// KubernetesConfig represents the kubernetes configuration
type KubernetesConfig struct {
	Version                  string                   `yaml:"version" json:"version" validate:"required,semver"`
	KubesprayVersion         string                   `yaml:"kubespray_version" json:"kubespray_version" validate:"required,semver"`
	APIPort                  int                      `yaml:"api_port" json:"api_port" validate:"required,min=1,max=65535"`
	KubeVIPEnabled           bool                     `yaml:"kube_vip_enabled" json:"kube_vip_enabled"`
	KubeletRotateServerCerts bool                     `yaml:"kubelet_rotate_server_certificates" json:"kubelet_rotate_server_certificates"`
	FlavorBastion            string                   `yaml:"flavor_bastion" json:"flavor_bastion" validate:"required"`
	FlavorMaster             string                   `yaml:"flavor_master" json:"flavor_master"`
	FlavorWorker             string                   `yaml:"flavor_worker" json:"flavor_worker"`
	FlavorWorkerWindows      string                   `yaml:"flavor_worker_windows" json:"flavor_worker_windows"`
	SubnetPods               string                   `yaml:"subnet_pods" json:"subnet_pods" validate:"required,cidrv4"`
	SubnetServices           string                   `yaml:"subnet_services" json:"subnet_services" validate:"required,cidrv4"`
	LoadbalancerProvider     string                   `yaml:"loadbalancer_provider" json:"loadbalancer_provider" validate:"required,oneof=ovn octavia metallb cloud-native"`
	DNSZoneName              string                   `yaml:"dns_zone_name" json:"dns_zone_name" validate:"omitempty,fqdn"`
	MasterCount              int                      `yaml:"master_count" json:"master_count" validate:"min=0,max=100"`
	WorkerCount              int                      `yaml:"worker_count" json:"worker_count" validate:"min=0,max=1000"`
	WorkerCountWindows       int                      `yaml:"worker_count_windows" json:"worker_count_windows" validate:"min=0,max=100"`
	MasterNodes              []NodeConfig             `yaml:"master_nodes,omitempty" json:"master_nodes,omitempty" validate:"dive"`
	WorkerNodes              []NodeConfig             `yaml:"worker_nodes,omitempty" json:"worker_nodes,omitempty" validate:"dive"`
	WindowsNodes             []NodeConfig             `yaml:"windows_nodes,omitempty" json:"windows_nodes,omitempty" validate:"dive"`
	NetworkPlugin            NetworkPlugin            `yaml:"network_plugin" json:"network_plugin" validate:"required"`
	StoragePlugin            StoragePlugin            `yaml:"storage_plugin" json:"storage_plugin"`
	OIDC                     OIDCConfig               `yaml:"oidc" json:"oidc"`
	WindowsWorkers           WindowsWorkers           `yaml:"windows_workers" json:"windows_workers"`
	Security                 KubernetesSecurityConfig `yaml:"security" json:"security"`
	Networking               Networking               `yaml:"networking" json:"networking"`
	Modules                  KubernetesModulesConfig  `yaml:"modules" json:"modules"`
	// AdditionalServerPoolsWorker defines additional worker node pools with custom configurations
	AdditionalServerPoolsWorker []AdditionalServerPool `yaml:"additional_server_pools_worker" json:"additional_server_pools_worker,omitempty" validate:"dive"`
	// AdditionalServerPoolsWorkerWindows defines additional Windows worker node pools
	AdditionalServerPoolsWorkerWindows []AdditionalServerPoolWindows `yaml:"additional_server_pools_worker_windows" json:"additional_server_pools_worker_windows,omitempty" validate:"dive"`
}

// NodeConfig represents a baremetal node configuration with id, name, and IP
type NodeConfig struct {
	ID         string `yaml:"id" json:"id" validate:"required"`
	Name       string `yaml:"name" json:"name" validate:"required,dns1123"`
	AccessIPv4 string `yaml:"access_ip_v4" json:"access_ip_v4" validate:"required,ipv4"`
}

// NetworkPlugin represents the network plugin configuration
type StoragePlugin struct {
	vsphere  VsphereConfig  `yaml:"calico" json:"calico"`
	cinder   CinderConfig   `yaml:"cilium" json:"cilium"`
	longhorn LonghornConfig `yaml:"kube-ovn" json:"kube-ovn"`
}

type VsphereConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type CinderConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type LonghornConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// NetworkPlugin represents the network plugin configuration
type NetworkPlugin struct {
	Calico  CalicoConfig  `yaml:"calico" json:"calico"`
	Cilium  CiliumConfig  `yaml:"cilium" json:"cilium"`
	KubeOVN KubeOVNConfig `yaml:"kube-ovn" json:"kube-ovn"`
}

// CalicoConfig represents the Calico configuration
type CalicoConfig struct {
	Enabled                   bool                `yaml:"enabled" json:"enabled"`
	CNIIface                  string              `yaml:"cni_iface" json:"cni_iface"`
	CalicoInterfaceAutodetect string              `yaml:"calico_interface_autodetect" json:"calico_interface_autodetect"`
	AutodetectCIDR            string              `yaml:"autodetect_cidr" json:"autodetect_cidr"`
	EncapsulationType         string              `yaml:"encapsulation_type" json:"encapsulation_type"`
	NATOutgoing               bool                `yaml:"nat_outgoing" json:"nat_outgoing"`
	Modules                   CalicoModulesConfig `yaml:"modules" json:"modules"`
}

// CiliumConfig represents the Cilium configuration
type CiliumConfig struct {
	Enabled              bool                `yaml:"enabled" json:"enabled"`
	OperatorEnabled      bool                `yaml:"operator_enabled" json:"operator_enabled"`
	KubeProxyReplacement bool                `yaml:"kubeProxyReplacement" json:"kubeProxyReplacement"`
	Modules              CiliumModulesConfig `yaml:"modules" json:"modules"`
}

// KubeOVNConfig represents the Kube-OVN configuration
type KubeOVNConfig struct {
	Enabled           bool                 `yaml:"enabled" json:"enabled"`
	CiliumIntegration bool                 `yaml:"cilium_integration" json:"cilium_integration"`
	Modules           KubeOVNModulesConfig `yaml:"modules" json:"modules"`
}

// OIDCConfig represents the OIDC configuration
type OIDCConfig struct {
	Enabled                bool   `yaml:"enabled" json:"enabled"`
	KubeOIDCURL            string `yaml:"kube_oidc_url" json:"kube_oidc_url" validate:"required_if=Enabled true,omitempty,url"`
	KubeOIDCClientID       string `yaml:"kube_oidc_client_id" json:"kube_oidc_client_id" validate:"required_if=Enabled true"`
	KubeOIDCCAFile         string `yaml:"kube_oidc_ca_file" json:"kube_oidc_ca_file"`
	KubeOIDCUsernameClaim  string `yaml:"kube_oidc_username_claim" json:"kube_oidc_username_claim"`
	KubeOIDCUsernamePrefix string `yaml:"kube_oidc_username_prefix" json:"kube_oidc_username_prefix"`
	KubeOIDCGroupsClaim    string `yaml:"kube_oidc_groups_claim" json:"kube_oidc_groups_claim"`
	KubeOIDCGroupsPrefix   string `yaml:"kube_oidc_groups_prefix" json:"kube_oidc_groups_prefix"`
}

// WindowsWorkers represents the Windows workers configuration
type WindowsWorkers struct {
	Enabled                  bool   `yaml:"enabled" json:"enabled"`
	WindowsUser              string `yaml:"windows_user" json:"windows_user"`
	WindowsAdminPassword     string `yaml:"windows_admin_password" json:"windows_admin_password"`
	WorkerNodeBFVSizeWindows int    `yaml:"worker_node_bfv_size_windows" json:"worker_node_bfv_size_windows"`
	WorkerNodeBFVTypeWindows string `yaml:"worker_node_bfv_type_windows" json:"worker_node_bfv_type_windows"`
}

// AdditionalServerPool defines configuration for an additional worker node pool
type AdditionalServerPool struct {
	// Name is the unique identifier for this worker pool
	Name string `yaml:"name" json:"name" jsonschema:"required,minLength=1,pattern=^[a-z0-9][a-z0-9-]*[a-z0-9]$,description=Unique name for this worker pool"`
	// WorkerCount is the number of worker nodes in this pool
	WorkerCount int `yaml:"worker_count" json:"worker_count" jsonschema:"required,minimum=0,maximum=100,description=Number of worker nodes in this pool"`
	// FlavorWorker is the instance flavor/size for this worker pool
	FlavorWorker string `yaml:"flavor_worker" json:"flavor_worker" jsonschema:"required,minLength=1,description=Instance flavor/size for this worker pool"`
	// NodeWorker is the node suffix identifier for this worker pool
	NodeWorker string `yaml:"node_worker" json:"node_worker" jsonschema:"required,minLength=1,description=Node suffix identifier for this worker pool"`
	// ServerGroupAffinity defines the server group affinity policy
	ServerGroupAffinity string `yaml:"server_group_affinity" json:"server_group_affinity" jsonschema:"enum=affinity;anti-affinity;soft-affinity;soft-anti-affinity,description=Server group affinity policy for this worker pool"`
	// ImageID is the OpenStack image ID for this worker pool
	ImageID string `yaml:"image_id" json:"image_id" jsonschema:"pattern=^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$,description=OpenStack image ID for this worker pool"`
	// ImageName is the OpenStack image name (alternative to ImageID)
	ImageName string `yaml:"image_name" json:"image_name" jsonschema:"description=OpenStack image name (alternative to image_id)"`
	// WorkerNodeBFVVolumeSize is the boot volume size in GB
	WorkerNodeBFVVolumeSize int `yaml:"worker_node_bfv_volume_size" json:"worker_node_bfv_volume_size" jsonschema:"default=40,minimum=10,maximum=1000,description=Boot volume size in GB"`
	// WorkerNodeBFVDestinationType is the boot volume destination type
	WorkerNodeBFVDestinationType string `yaml:"worker_node_bfv_destination_type" json:"worker_node_bfv_destination_type" jsonschema:"default=volume,enum=volume;local,description=Boot volume destination type"`
	// WorkerNodeBFVSourceType is the boot volume source type
	WorkerNodeBFVSourceType string `yaml:"worker_node_bfv_source_type" json:"worker_node_bfv_source_type" jsonschema:"default=image,enum=image;volume;snapshot,description=Boot volume source type"`
	// WorkerNodeBFVVolumeType is the boot volume type
	WorkerNodeBFVVolumeType string `yaml:"worker_node_bfv_volume_type" json:"worker_node_bfv_volume_type" jsonschema:"description=Boot volume type (e.g. HA-Standard HA-Performance)"`
	// WorkerNodeBFVDeleteOnTermination controls whether to delete boot volume when instance is terminated
	WorkerNodeBFVDeleteOnTermination bool `yaml:"worker_node_bfv_delete_on_termination" json:"worker_node_bfv_delete_on_termination" jsonschema:"default=true,description=Delete boot volume when instance is terminated"`
	// AdditionalBlockDevicesWorker defines additional block devices for this worker pool
	AdditionalBlockDevicesWorker []map[string]any `yaml:"additional_block_devices_worker" json:"additional_block_devices_worker" jsonschema:"description=Additional block devices for this worker pool"`
	// PF9Onboard enables Platform9 onboarding for this pool
	PF9Onboard bool `yaml:"pf9_onboard" json:"pf9_onboard" jsonschema:"default=false,description=Enable Platform9 onboarding for this pool"`
	// SubnetID is the specific subnet ID for this worker pool (optional)
	SubnetID string `yaml:"subnet_id" json:"subnet_id" jsonschema:"pattern=^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$,description=Specific subnet ID for this worker pool (optional)"`
}

// AdditionalServerPoolWindows defines configuration for an additional Windows worker node pool
type AdditionalServerPoolWindows struct {
	// Name is the unique identifier for this Windows worker pool
	Name string `yaml:"name" json:"name" jsonschema:"required,minLength=1,pattern=^[a-z0-9][a-z0-9-]*[a-z0-9]$,description=Unique name for this Windows worker pool"`
	// WorkerCount is the number of Windows worker nodes in this pool
	WorkerCount int `yaml:"worker_count" json:"worker_count" jsonschema:"required,minimum=0,maximum=50,description=Number of Windows worker nodes in this pool"`
	// FlavorWorker is the instance flavor/size for this Windows worker pool
	FlavorWorker string `yaml:"flavor_worker" json:"flavor_worker" jsonschema:"required,minLength=1,description=Instance flavor/size for this Windows worker pool"`
	// NodeWorker is the node suffix identifier for this Windows worker pool
	NodeWorker string `yaml:"node_worker" json:"node_worker" jsonschema:"required,minLength=1,description=Node suffix identifier for this Windows worker pool"`
	// ServerGroupAffinity defines the server group affinity policy
	ServerGroupAffinity string `yaml:"server_group_affinity,omitempty" json:"server_group_affinity,omitempty" jsonschema:"enum=affinity;anti-affinity;soft-affinity;soft-anti-affinity,description=Server group affinity policy for this Windows worker pool"`
	// ImageID is the OpenStack Windows image ID for this worker pool
	ImageID string `yaml:"image_id,omitempty" json:"image_id,omitempty" jsonschema:"pattern=^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$,description=OpenStack Windows image ID for this worker pool"`
}

// KubernetesModulesConfig represents the Kubernetes module configurations
type KubernetesModulesConfig struct {
	KubesprayCluster KubesprayClusterModuleConfig `yaml:"kubespray_cluster" json:"kubespray_cluster"`
}

// KubesprayClusterModuleConfig represents the kubespray-cluster module configuration
type KubesprayClusterModuleConfig struct {
	Source string `yaml:"source" json:"source"`
}

// CalicoModulesConfig represents the Calico module configurations
type CalicoModulesConfig struct {
	Calico CalicoModuleConfig `yaml:"calico" json:"calico"`
}

// CalicoModuleConfig represents the calico module configuration
type CalicoModuleConfig struct {
	Source string `yaml:"source" json:"source"`
}

// CiliumModulesConfig represents the Cilium module configurations
type CiliumModulesConfig struct {
	Cilium CiliumModuleConfig `yaml:"cilium" json:"cilium"`
}

// CiliumModuleConfig represents the cilium module configuration
type CiliumModuleConfig struct {
	Source string `yaml:"source" json:"source"`
}

// KubeOVNModulesConfig represents the Kube-OVN module configurations
type KubeOVNModulesConfig struct {
	KubeOVN KubeOVNModuleConfig `yaml:"kube_ovn" json:"kube_ovn"`
}

// KubeOVNModuleConfig represents the kube-ovn module configuration
type KubeOVNModuleConfig struct {
	Source string `yaml:"source" json:"source"`
}
