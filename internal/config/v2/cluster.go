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

	// HostnameSubstitution opts the generator into emitting Flux
	// postBuild.substitute placeholders (e.g. ${CLUSTER_FQDN}) instead of
	// literal hostnames. Off by default; when disabled the output is unchanged.
	HostnameSubstitution HostnameSubstitutionConfig `yaml:"hostname_substitution,omitempty" json:"hostname_substitution,omitempty"`
}

// HostnameSubstitutionConfig controls Flux postBuild.substituteFrom rendering.
// See OCTR-759. When Enabled is false the generator emits literal hostnames and
// no cluster-vars ConfigMap, so output is byte-identical to prior releases.
type HostnameSubstitutionConfig struct {
	Enabled       bool              `yaml:"enabled,omitempty" json:"enabled,omitempty" jsonschema:"description=Emit Flux postBuild placeholders instead of literal hostnames; off by default"`
	ConfigMapName string            `yaml:"configmap_name,omitempty" json:"configmap_name,omitempty" jsonschema:"description=Name of the cluster-vars ConfigMap rendered under clusters/<cluster>/,default=cluster-vars"`
	Variables     map[string]string `yaml:"variables,omitempty" json:"variables,omitempty" jsonschema:"description=Placeholder name to cluster-config field mapping (e.g. CLUSTER_FQDN: cluster_fqdn)"`
}

// GetConfigMapName returns the configured ConfigMap name or the default.
func (h HostnameSubstitutionConfig) GetConfigMapName() string {
	if h.ConfigMapName == "" {
		return "cluster-vars"
	}
	return h.ConfigMapName
}

// GetVariables returns the placeholder->field map, defaulting to the standard
// CLUSTER_FQDN/BASE_DOMAIN/CLUSTER_NAME set when none is configured.
func (h HostnameSubstitutionConfig) GetVariables() map[string]string {
	if len(h.Variables) > 0 {
		return h.Variables
	}
	return map[string]string{
		"CLUSTER_FQDN": "cluster_fqdn",
		"BASE_DOMAIN":  "base_domain",
		"CLUSTER_NAME": "cluster_name",
	}
}

// FQDNPlaceholder returns the ${VAR} token whose mapped field is cluster_fqdn,
// or "" when substitution is disabled or no variable maps to cluster_fqdn.
func (h HostnameSubstitutionConfig) FQDNPlaceholder() string {
	if !h.Enabled {
		return ""
	}
	for name, field := range h.GetVariables() {
		if field == "cluster_fqdn" {
			return "${" + name + "}"
		}
	}
	return ""
}

// KubernetesConfig represents Kubernetes cluster configuration.
// Requirements: 3.2, 9.3, 9.4
type KubernetesConfig struct {
	Version                  string                   `yaml:"version" json:"version" validate:"required,semver"`
	APIPort                  int                      `yaml:"api_port" json:"api_port" validate:"required,min=1,max=65535"`
	KubeVIPEnabled           bool                     `yaml:"kube_vip_enabled" json:"kube_vip_enabled"`
	KubeletRotateServerCerts bool                     `yaml:"kubelet_rotate_server_certs" json:"kubelet_rotate_server_certs"`
	SubnetPods               string                   `yaml:"subnet_pods" json:"subnet_pods" validate:"required,cidrv4"`
	SubnetServices           string                   `yaml:"subnet_services" json:"subnet_services" validate:"required,cidrv4"`
	NetworkPlugin            NetworkPluginConfig      `yaml:"network_plugin" json:"network_plugin" validate:"required"`
	StoragePlugin            StoragePluginConfig      `yaml:"storage_plugin,omitempty" json:"storage_plugin,omitempty"`
	Security                 KubernetesSecurityConfig `yaml:"security,omitempty" json:"security,omitempty"`
	OIDC                     OIDCConfig               `yaml:"oidc,omitempty" json:"oidc,omitempty"`
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
	Enabled                   bool             `yaml:"enabled" json:"enabled"`
	Version                   string           `yaml:"version,omitempty" json:"version,omitempty"`
	IPIPMode                  string           `yaml:"ipip_mode,omitempty" json:"ipip_mode,omitempty" validate:"omitempty,oneof=Always CrossSubnet Never"`
	VXLANMode                 string           `yaml:"vxlan_mode,omitempty" json:"vxlan_mode,omitempty" validate:"omitempty,oneof=Always CrossSubnet Never"`
	NetworkPolicy             bool             `yaml:"network_policy" json:"network_policy"`
	CNIIface                  string           `yaml:"cni_iface,omitempty" json:"cni_iface,omitempty"`
	CalicoInterfaceAutodetect string           `yaml:"calico_interface_autodetect,omitempty" json:"calico_interface_autodetect,omitempty"`
	AutodetectCIDR            string           `yaml:"autodetect_cidr,omitempty" json:"autodetect_cidr,omitempty"`
	EncapsulationType         string           `yaml:"encapsulation_type,omitempty" json:"encapsulation_type,omitempty"`
	NATOutgoing               bool             `yaml:"nat_outgoing" json:"nat_outgoing"`
	Modules                   CNIModulesConfig `yaml:"modules,omitempty" json:"modules,omitempty"`
	// InstallMethod specifies how the CNI should be installed.
	// Valid values: "helm" (default), "kustomize-helm", or "kubespray" for non-OpenStack migration compatibility.
	InstallMethod string `yaml:"install_method,omitempty" json:"install_method,omitempty" validate:"omitempty,oneof=kubespray helm kustomize-helm"`
}

// CiliumConfig represents Cilium CNI configuration.
type CiliumConfig struct {
	Enabled              bool             `yaml:"enabled" json:"enabled"`
	Version              string           `yaml:"version,omitempty" json:"version,omitempty"`
	TunnelMode           string           `yaml:"tunnel_mode,omitempty" json:"tunnel_mode,omitempty" validate:"omitempty,oneof=vxlan geneve disabled"`
	Hubble               bool             `yaml:"hubble" json:"hubble"`
	NetworkPolicy        bool             `yaml:"network_policy" json:"network_policy"`
	OperatorEnabled      bool             `yaml:"operator_enabled" json:"operator_enabled"`
	KubeProxyReplacement bool             `yaml:"kube_proxy_replacement" json:"kube_proxy_replacement"`
	Modules              CNIModulesConfig `yaml:"modules,omitempty" json:"modules,omitempty"`
	// InstallMethod specifies how the CNI should be installed.
	// Valid values: "helm" (default), "kustomize-helm", or "kubespray" for non-OpenStack migration compatibility.
	InstallMethod string `yaml:"install_method,omitempty" json:"install_method,omitempty" validate:"omitempty,oneof=kubespray helm kustomize-helm"`
}

// KubeOVNConfig represents Kube-OVN CNI configuration.
type KubeOVNConfig struct {
	Enabled           bool             `yaml:"enabled" json:"enabled"`
	Version           string           `yaml:"version,omitempty" json:"version,omitempty"`
	NetworkPolicy     bool             `yaml:"network_policy" json:"network_policy"`
	CiliumIntegration bool             `yaml:"cilium_integration" json:"cilium_integration"`
	Modules           CNIModulesConfig `yaml:"modules,omitempty" json:"modules,omitempty"`
	// InstallMethod specifies how the CNI should be installed.
	// Valid values: "helm" (default), "kustomize-helm", or "kubespray" for non-OpenStack migration compatibility.
	InstallMethod string `yaml:"install_method,omitempty" json:"install_method,omitempty" validate:"omitempty,oneof=kubespray helm kustomize-helm"`
}

// CNIModulesConfig holds module source references for CNI plugins.
type CNIModulesConfig struct {
	Calico  CNIModuleSource `yaml:"calico,omitempty" json:"calico,omitempty"`
	Cilium  CNIModuleSource `yaml:"cilium,omitempty" json:"cilium,omitempty"`
	KubeOVN CNIModuleSource `yaml:"kube_ovn,omitempty" json:"kube_ovn,omitempty"`
}

// CNIModuleSource holds the source URL for a CNI module.
type CNIModuleSource struct {
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
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
	PodSecurityPolicy     bool     `yaml:"pod_security_policy" json:"pod_security_policy"`
	PodSecurityStandards  string   `yaml:"pod_security_standards,omitempty" json:"pod_security_standards,omitempty" validate:"omitempty,oneof=privileged baseline restricted"`
	PodSecurityExemptions []string `yaml:"pod_security_exemptions,omitempty" json:"pod_security_exemptions,omitempty"`
	AuditLogging          bool     `yaml:"audit_logging" json:"audit_logging"`
	EncryptionAtRest      bool     `yaml:"encryption_at_rest" json:"encryption_at_rest"`
	AdmissionControllers  []string `yaml:"admission_controllers,omitempty" json:"admission_controllers,omitempty"`
	K8sHardening          bool     `yaml:"k8s_hardening" json:"k8s_hardening"`
}

// OIDCConfig represents OIDC authentication configuration.
type OIDCConfig struct {
	Enabled                bool   `yaml:"enabled" json:"enabled"`
	IssuerURL              string `yaml:"issuer_url,omitempty" json:"issuer_url,omitempty" validate:"required_if=Enabled true,omitempty,url"`
	ClientID               string `yaml:"client_id,omitempty" json:"client_id,omitempty" validate:"required_if=Enabled true"`
	ClientSecret           string `yaml:"client_secret,omitempty" json:"client_secret,omitempty" validate:"required_if=Enabled true"`
	UsernameClaim          string `yaml:"username_claim,omitempty" json:"username_claim,omitempty"`
	GroupsClaim            string `yaml:"groups_claim,omitempty" json:"groups_claim,omitempty"`
	KubeOIDCURL            string `yaml:"kube_oidc_url,omitempty" json:"kube_oidc_url,omitempty"`
	KubeOIDCClientID       string `yaml:"kube_oidc_client_id,omitempty" json:"kube_oidc_client_id,omitempty"`
	KubeOIDCCAFile         string `yaml:"kube_oidc_ca_file,omitempty" json:"kube_oidc_ca_file,omitempty"`
	KubeOIDCUsernameClaim  string `yaml:"kube_oidc_username_claim,omitempty" json:"kube_oidc_username_claim,omitempty"`
	KubeOIDCUsernamePrefix string `yaml:"kube_oidc_username_prefix,omitempty" json:"kube_oidc_username_prefix,omitempty"`
	KubeOIDCGroupsClaim    string `yaml:"kube_oidc_groups_claim,omitempty" json:"kube_oidc_groups_claim,omitempty"`
	KubeOIDCGroupsPrefix   string `yaml:"kube_oidc_groups_prefix,omitempty" json:"kube_oidc_groups_prefix,omitempty"`
}
