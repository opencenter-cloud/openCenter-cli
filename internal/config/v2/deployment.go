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

// DeploymentConfig represents deployment method configuration.
// Requirements: 5.1, 5.2, 5.3, 5.4, 5.5
type DeploymentConfig struct {
	AutoDeploy bool              `yaml:"auto_deploy" json:"auto_deploy"`
	Method     string            `yaml:"method" json:"method" validate:"required,oneof=kubespray talos kamaji eks gke aks cluster-api"`
	Kubespray  *KubesprayConfig  `yaml:"kubespray,omitempty" json:"kubespray,omitempty"`
	Talos      *TalosConfig      `yaml:"talos,omitempty" json:"talos,omitempty"`
	Kamaji     *KamajiConfig     `yaml:"kamaji,omitempty" json:"kamaji,omitempty"`
	ClusterAPI *ClusterAPIConfig `yaml:"cluster_api,omitempty" json:"cluster_api,omitempty"`
}

// KubesprayConfig represents Kubespray deployment configuration.
// Requirements: 5.2
type KubesprayConfig struct {
	Version string                  `yaml:"version" json:"version" validate:"required,semver"`
	Modules map[string]ModuleConfig `yaml:"modules,omitempty" json:"modules,omitempty"`
}

// ModuleConfig represents a deployment module configuration.
type ModuleConfig struct {
	Source  string         `yaml:"source,omitempty" json:"source,omitempty"`
	Enabled bool           `yaml:"enabled" json:"enabled"`
	Version string         `yaml:"version,omitempty" json:"version,omitempty"`
	Config  map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

// TalosConfig represents Talos Linux deployment configuration.
// Requirements: 5.3
type TalosConfig struct {
	Version           string             `yaml:"version" json:"version" validate:"required,semver"`
	KubernetesVersion string             `yaml:"kubernetes_version" json:"kubernetes_version" validate:"required,semver"`
	Endpoint          string             `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Install           TalosInstallConfig `yaml:"install" json:"install" validate:"required"`
	Network           TalosNetworkConfig `yaml:"network" json:"network" validate:"required"`
	Patches           TalosPatchesConfig `yaml:"patches,omitempty" json:"patches,omitempty"`
}

// TalosInstallConfig represents machine installation settings for Talos.
type TalosInstallConfig struct {
	Disk  string `yaml:"disk" json:"disk" validate:"required"`
	Image string `yaml:"image" json:"image" validate:"required"`
}

// TalosNetworkConfig represents Talos cluster network settings.
type TalosNetworkConfig struct {
	PodSubnet       string   `yaml:"pod_subnet" json:"pod_subnet" validate:"required,cidrv4"`
	ServiceSubnet   string   `yaml:"service_subnet" json:"service_subnet" validate:"required,cidrv4"`
	TalosAPIPort    int      `yaml:"talos_api_port" json:"talos_api_port" validate:"required,min=1,max=65535"`
	ManagementCIDRs []string `yaml:"management_cidrs" json:"management_cidrs,omitempty" validate:"omitempty,dive,cidrv4"`
}

// TalosPatchesConfig lists generated/static Talos machine config patches.
type TalosPatchesConfig struct {
	Static []string `yaml:"static,omitempty" json:"static,omitempty"`
}

// KamajiConfig represents Kamaji hosted control plane configuration.
// Requirements: 5.4, 5.5, 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8
type KamajiConfig struct {
	Enabled      bool                    `yaml:"enabled" json:"enabled"`
	Version      string                  `yaml:"version" json:"version" validate:"required_if=Enabled true,omitempty,semver"`
	ControlPlane KamajiControlPlane      `yaml:"control_plane" json:"control_plane" validate:"required_if=Enabled true"`
	ClusterAPI   ClusterAPIConfig        `yaml:"cluster_api" json:"cluster_api" validate:"required_if=Enabled true"`
	WorkerPools  []KamajiWorkerPool      `yaml:"worker_pools" json:"worker_pools" validate:"required_if=Enabled true,min=1,dive"`
	Modules      map[string]ModuleConfig `yaml:"modules,omitempty" json:"modules,omitempty"`
}

// KamajiControlPlane represents Kamaji control plane configuration.
// Requirements: 10.2, 10.3, 10.4
type KamajiControlPlane struct {
	Replicas      int                     `yaml:"replicas" json:"replicas" validate:"required,oneof=1 3 5 7"`
	Datastore     string                  `yaml:"datastore" json:"datastore" validate:"required,oneof=etcd postgresql mysql"`
	Etcd          *KamajiEtcdConfig       `yaml:"etcd,omitempty" json:"etcd,omitempty"`
	PostgreSQL    *KamajiPostgreSQLConfig `yaml:"postgresql,omitempty" json:"postgresql,omitempty"`
	MySQL         *KamajiMySQLConfig      `yaml:"mysql,omitempty" json:"mysql,omitempty"`
	ServiceType   string                  `yaml:"service_type" json:"service_type" validate:"required,oneof=LoadBalancer NodePort"`
	APIServerPort int                     `yaml:"api_server_port" json:"api_server_port" validate:"required,min=1,max=65535"`
	Resources     KamajiResourcesConfig   `yaml:"resources,omitempty" json:"resources,omitempty"`
}

// KamajiEtcdConfig represents Kamaji etcd datastore configuration.
// Requirements: 10.4
type KamajiEtcdConfig struct {
	StorageClass string `yaml:"storage_class" json:"storage_class" validate:"required"`
	StorageSize  string `yaml:"storage_size" json:"storage_size" validate:"required"`
	Replicas     int    `yaml:"replicas,omitempty" json:"replicas,omitempty" validate:"omitempty,oneof=1 3 5 7"`
}

// KamajiPostgreSQLConfig represents Kamaji PostgreSQL datastore configuration.
// Requirements: 10.5
type KamajiPostgreSQLConfig struct {
	Host     string `yaml:"host" json:"host" validate:"required,hostname|ip"`
	Port     int    `yaml:"port,omitempty" json:"port,omitempty" validate:"omitempty,min=1,max=65535"`
	Database string `yaml:"database" json:"database" validate:"required"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	SSLMode  string `yaml:"ssl_mode,omitempty" json:"ssl_mode,omitempty" validate:"omitempty,oneof=disable require verify-ca verify-full"`
}

// KamajiMySQLConfig represents Kamaji MySQL datastore configuration.
type KamajiMySQLConfig struct {
	Host     string `yaml:"host" json:"host" validate:"required,hostname|ip"`
	Port     int    `yaml:"port,omitempty" json:"port,omitempty" validate:"omitempty,min=1,max=65535"`
	Database string `yaml:"database" json:"database" validate:"required"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
}

// KamajiResourcesConfig represents Kamaji control plane resource configuration.
type KamajiResourcesConfig struct {
	CPURequest    string `yaml:"cpu_request,omitempty" json:"cpu_request,omitempty"`
	CPULimit      string `yaml:"cpu_limit,omitempty" json:"cpu_limit,omitempty"`
	MemoryRequest string `yaml:"memory_request,omitempty" json:"memory_request,omitempty"`
	MemoryLimit   string `yaml:"memory_limit,omitempty" json:"memory_limit,omitempty"`
}

// ClusterAPIConfig represents Cluster API configuration.
// Requirements: 10.6
type ClusterAPIConfig struct {
	Version   string               `yaml:"version" json:"version" validate:"required,semver"`
	Providers ClusterAPIProviders  `yaml:"providers" json:"providers" validate:"required"`
	OpenStack *CAPIOpenStackConfig `yaml:"openstack,omitempty" json:"openstack,omitempty"`
	AWS       *CAPIAWSConfig       `yaml:"aws,omitempty" json:"aws,omitempty"`
	Azure     *CAPIAzureConfig     `yaml:"azure,omitempty" json:"azure,omitempty"`
	VMware    *CAPIVMwareConfig    `yaml:"vmware,omitempty" json:"vmware,omitempty"`
}

// ClusterAPIProviders represents Cluster API provider configuration.
// Requirements: 10.6
type ClusterAPIProviders struct {
	Infrastructure string `yaml:"infrastructure" json:"infrastructure" validate:"required,oneof=openstack aws azure vsphere vmware metal3"`
	Bootstrap      string `yaml:"bootstrap" json:"bootstrap" validate:"required,oneof=kubeadm talos"`
	ControlPlane   string `yaml:"control_plane" json:"control_plane" validate:"required,oneof=kubeadm talos"`
}

// CAPIOpenStackConfig represents CAPI OpenStack provider configuration.
type CAPIOpenStackConfig struct {
	CloudsYAML string `yaml:"clouds_yaml,omitempty" json:"clouds_yaml,omitempty"`
	CloudName  string `yaml:"cloud_name,omitempty" json:"cloud_name,omitempty"`
}

// CAPIAWSConfig represents CAPI AWS provider configuration.
type CAPIAWSConfig struct {
	Region string `yaml:"region,omitempty" json:"region,omitempty"`
}

// CAPIAzureConfig represents CAPI Azure provider configuration.
type CAPIAzureConfig struct {
	Location string `yaml:"location,omitempty" json:"location,omitempty"`
}

// CAPIVMwareConfig represents CAPI VMware provider configuration.
type CAPIVMwareConfig struct {
	Server string `yaml:"server,omitempty" json:"server,omitempty"`
}

// KamajiWorkerPool represents a Kamaji worker pool configuration.
// Requirements: 10.7, 10.8
type KamajiWorkerPool struct {
	Name              string             `yaml:"name" json:"name" validate:"required,dns1123"`
	OS                string             `yaml:"os" json:"os" validate:"required,oneof=ubuntu windows talos"`
	Count             int                `yaml:"count" json:"count" validate:"required,min=1"`
	Flavor            string             `yaml:"flavor" json:"flavor" validate:"required"`
	Image             string             `yaml:"image" json:"image" validate:"required"`
	BootstrapProvider string             `yaml:"bootstrap_provider" json:"bootstrap_provider" validate:"required,oneof=kubeadm talos"`
	TalosVersion      string             `yaml:"talos_version,omitempty" json:"talos_version,omitempty" validate:"required_if=OS talos,omitempty,semver"`
	BootVolume        VolumeConfig       `yaml:"boot_volume" json:"boot_volume" validate:"required"`
	AdditionalVolumes []VolumeConfig     `yaml:"additional_volumes,omitempty" json:"additional_volumes,omitempty"`
	Labels            map[string]string  `yaml:"labels,omitempty" json:"labels,omitempty"`
	Taints            []TaintConfig      `yaml:"taints,omitempty" json:"taints,omitempty"`
	Autoscaling       AutoscalingConfig  `yaml:"autoscaling,omitempty" json:"autoscaling,omitempty"`
	TalosConfig       *TalosWorkerConfig `yaml:"talos_config,omitempty" json:"talos_config,omitempty"`
}

// AutoscalingConfig represents autoscaling configuration for worker pools.
type AutoscalingConfig struct {
	Enabled     bool `yaml:"enabled" json:"enabled"`
	MinReplicas int  `yaml:"min_replicas,omitempty" json:"min_replicas,omitempty" validate:"omitempty,min=1"`
	MaxReplicas int  `yaml:"max_replicas,omitempty" json:"max_replicas,omitempty" validate:"omitempty,gtefield=MinReplicas"`
}

// TalosWorkerConfig represents Talos-specific worker configuration.
type TalosWorkerConfig struct {
	InstallDisk string            `yaml:"install_disk,omitempty" json:"install_disk,omitempty"`
	Extensions  []string          `yaml:"extensions,omitempty" json:"extensions,omitempty"`
	KernelArgs  map[string]string `yaml:"kernel_args,omitempty" json:"kernel_args,omitempty"`
}
