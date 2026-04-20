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

// ClusterConfig represents Kubernetes-specific configuration independent of infrastructure.
// Requirements: 1.4, 3.2, 9.3, 9.4, 9.5
type ClusterConfig struct {
	ClusterName string           `yaml:"cluster_name" json:"cluster_name" validate:"required,dns1123"`
	BaseDomain  string           `yaml:"base_domain" json:"base_domain" validate:"required,fqdn"`
	ClusterFQDN string           `yaml:"cluster_fqdn" json:"cluster_fqdn" validate:"required,fqdn"`
	AdminEmail  string           `yaml:"admin_email" json:"admin_email" validate:"required,email"`
	Kubernetes  KubernetesConfig `yaml:"kubernetes" json:"kubernetes" validate:"required"`
}

// KubernetesConfig represents Kubernetes cluster configuration.
// Requirements: 3.2, 9.3, 9.4
type KubernetesConfig struct {
	Version        string                   `yaml:"version" json:"version" validate:"required,semver"`
	APIPort        int                      `yaml:"api_port" json:"api_port" validate:"required,min=1,max=65535"`
	KubeVIPEnabled bool                     `yaml:"kube_vip_enabled" json:"kube_vip_enabled"`
	SubnetPods     string                   `yaml:"subnet_pods" json:"subnet_pods" validate:"required,cidrv4"`
	SubnetServices string                   `yaml:"subnet_services" json:"subnet_services" validate:"required,cidrv4"`
	NetworkPlugin  NetworkPluginConfig      `yaml:"network_plugin" json:"network_plugin" validate:"required"`
	StoragePlugin  StoragePluginConfig      `yaml:"storage_plugin,omitempty" json:"storage_plugin,omitempty"`
	Security       KubernetesSecurityConfig `yaml:"security,omitempty" json:"security,omitempty"`
	OIDC           OIDCConfig               `yaml:"oidc,omitempty" json:"oidc,omitempty"`
}

// NetworkPluginConfig represents CNI plugin configuration.
// Requirements: 3.2
type NetworkPluginConfig struct {
	Calico  *CalicoConfig  `yaml:"calico,omitempty" json:"calico,omitempty"`
	Cilium  *CiliumConfig  `yaml:"cilium,omitempty" json:"cilium,omitempty"`
	KubeOVN *KubeOVNConfig `yaml:"kube-ovn,omitempty" json:"kube-ovn,omitempty"`
}

// CalicoConfig represents Calico CNI configuration.
type CalicoConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Version       string `yaml:"version,omitempty" json:"version,omitempty"`
	IPIPMode      string `yaml:"ipip_mode,omitempty" json:"ipip_mode,omitempty" validate:"omitempty,oneof=Always CrossSubnet Never"`
	VXLANMode     string `yaml:"vxlan_mode,omitempty" json:"vxlan_mode,omitempty" validate:"omitempty,oneof=Always CrossSubnet Never"`
	NetworkPolicy bool   `yaml:"network_policy" json:"network_policy"`
	// InstallMethod specifies how the CNI should be installed.
	// Valid values: "kubespray" (default, installed during cluster bootstrap) or "helm" (installed via GitOps/Helm after cluster is up)
	InstallMethod string `yaml:"install_method,omitempty" json:"install_method,omitempty" validate:"omitempty,oneof=kubespray helm"`
}

// CiliumConfig represents Cilium CNI configuration.
type CiliumConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Version       string `yaml:"version,omitempty" json:"version,omitempty"`
	TunnelMode    string `yaml:"tunnel_mode,omitempty" json:"tunnel_mode,omitempty" validate:"omitempty,oneof=vxlan geneve disabled"`
	Hubble        bool   `yaml:"hubble" json:"hubble"`
	NetworkPolicy bool   `yaml:"network_policy" json:"network_policy"`
	// InstallMethod specifies how the CNI should be installed.
	// Valid values: "kubespray" (default, installed during cluster bootstrap) or "helm" (installed via GitOps/Helm after cluster is up)
	InstallMethod string `yaml:"install_method,omitempty" json:"install_method,omitempty" validate:"omitempty,oneof=kubespray helm"`
}

// KubeOVNConfig represents Kube-OVN CNI configuration.
type KubeOVNConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Version       string `yaml:"version,omitempty" json:"version,omitempty"`
	NetworkPolicy bool   `yaml:"network_policy" json:"network_policy"`
	// InstallMethod specifies how the CNI should be installed.
	// Valid values: "kubespray" (default, installed during cluster bootstrap) or "helm" (installed via GitOps/Helm after cluster is up)
	InstallMethod string `yaml:"install_method,omitempty" json:"install_method,omitempty" validate:"omitempty,oneof=kubespray helm"`
}

// StoragePluginConfig represents CSI plugin configuration.
// Requirements: 9.3, 9.4
type StoragePluginConfig struct {
	VSphereCsi    *VSphereCsiConfig    `yaml:"vsphere_csi,omitempty" json:"vsphere_csi,omitempty"`
	CinderCsi     *CinderCsiConfig     `yaml:"cinder_csi,omitempty" json:"cinder_csi,omitempty"`
	AwsEbsCsi     *AwsEbsCsiConfig     `yaml:"aws_ebs_csi,omitempty" json:"aws_ebs_csi,omitempty"`
	GcpComputeCsi *GcpComputeCsiConfig `yaml:"gcp_compute_csi,omitempty" json:"gcp_compute_csi,omitempty"`
	AzureDiskCsi  *AzureDiskCsiConfig  `yaml:"azure_disk_csi,omitempty" json:"azure_disk_csi,omitempty"`
	Ceph          *CephCsiConfig       `yaml:"ceph,omitempty" json:"ceph,omitempty"`
	Trident       *TridentCsiConfig    `yaml:"trident,omitempty" json:"trident,omitempty"`
}

// VSphereCsiConfig represents vSphere CSI driver configuration.
type VSphereCsiConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// CinderCsiConfig represents OpenStack Cinder CSI driver configuration.
type CinderCsiConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// AwsEbsCsiConfig represents AWS EBS CSI driver configuration.
type AwsEbsCsiConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// GcpComputeCsiConfig represents GCP Compute CSI driver configuration.
type GcpComputeCsiConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// AzureDiskCsiConfig represents Azure Disk CSI driver configuration.
type AzureDiskCsiConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// CephCsiConfig represents Ceph CSI driver configuration.
type CephCsiConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// TridentCsiConfig represents NetApp Trident CSI driver configuration.
type TridentCsiConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

// KubernetesSecurityConfig represents Kubernetes security configuration.
type KubernetesSecurityConfig struct {
	PodSecurityPolicy    bool     `yaml:"pod_security_policy" json:"pod_security_policy"`
	PodSecurityStandards string   `yaml:"pod_security_standards,omitempty" json:"pod_security_standards,omitempty" validate:"omitempty,oneof=privileged baseline restricted"`
	AuditLogging         bool     `yaml:"audit_logging" json:"audit_logging"`
	EncryptionAtRest     bool     `yaml:"encryption_at_rest" json:"encryption_at_rest"`
	AdmissionControllers []string `yaml:"admission_controllers,omitempty" json:"admission_controllers,omitempty"`
}

// OIDCConfig represents OIDC authentication configuration.
type OIDCConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	IssuerURL     string `yaml:"issuer_url,omitempty" json:"issuer_url,omitempty" validate:"required_if=Enabled true,omitempty,url"`
	ClientID      string `yaml:"client_id,omitempty" json:"client_id,omitempty" validate:"required_if=Enabled true"`
	ClientSecret  string `yaml:"client_secret,omitempty" json:"client_secret,omitempty" validate:"required_if=Enabled true"`
	UsernameClaim string `yaml:"username_claim,omitempty" json:"username_claim,omitempty"`
	GroupsClaim   string `yaml:"groups_claim,omitempty" json:"groups_claim,omitempty"`
}
