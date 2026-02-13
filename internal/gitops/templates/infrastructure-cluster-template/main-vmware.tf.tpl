locals {
  # Cluster identification
  cluster_name                            = "{{ .OpenCenter.Cluster.ClusterName }}"

  # Network configuration - VMware node network
  subnet_nodes                            = "{{ .OpenCenter.Infrastructure.Cloud.VMware.Network | default "172.26.0.0/24" }}"
  # Kubernetes pod network (overlay)
  subnet_pods                             = "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods | default "10.42.0.0/16" }}"
  # Kubernetes service network (overlay)
  subnet_services                         = "{{ .OpenCenter.Cluster.Kubernetes.SubnetServices | default "10.43.0.0/16" }}"
  
  # VRRP configuration for HA control plane
  vrrp_enabled                            = {{ .OpenCenter.Infrastructure.Networking.VRRPEnabled | default true }}
  vrrp_ip                                 = "{{ .OpenCenter.Infrastructure.Networking.VRRPIP | default "172.26.0.5" }}"
  
  # DNS zone for cluster
  dns_zone_name                           = "{{ .OpenCenter.Infrastructure.Networking.DNSZoneName | default "" }}"

  # Kubernetes API configuration
  k8s_api_port                            = {{ .OpenCenter.Cluster.Kubernetes.APIPort | default 443 }}
  k8s_api_ip                              = "{{ .OpenCenter.Infrastructure.K8sAPIIP | default "" }}" != "" ? "{{ .OpenCenter.Infrastructure.K8sAPIIP }}" : local.vrrp_ip
  
  # SSH configuration
  ssh_user                                = "{{ .OpenCenter.Infrastructure.SSHUser | default "ubuntu" }}"
  ssh_key_path                            = "{{ .OpenCenter.Infrastructure.SSHKeyPath | default "" }}"

  # VMware does not use Octavia load balancer
  use_octavia                             = false

  # Kubespray deployment settings
  kubespray_version                       = "{{ .Deployment.Kubespray.Version | default "v2.29.1" }}"
  kubernetes_version                      = "{{ .OpenCenter.Cluster.Kubernetes.Version | default "1.33.7" }}"
  network_plugin                          = "{{ if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled }}calico{{ else if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled }}cilium{{ else if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled }}kube-ovn{{ else }}calico{{ end }}"
  deploy_cluster                          = {{ .Deployment.AutoDeploy | default true }}
  
  # Kube-VIP for HA API endpoint
  kube_vip_enabled                        = {{ .OpenCenter.Cluster.Kubernetes.KubeVIPEnabled | default true }}
  
  # Security hardening
  k8s_hardening_enabled                   = {{ .OpenCenter.Cluster.Kubernetes.Security.K8sHardening | default true }}
  kube_pod_security_exemptions_namespaces = {{ if .OpenCenter.Cluster.Kubernetes.Security.PodSecurityExemptions }}[{{ range $i, $ns := .OpenCenter.Cluster.Kubernetes.Security.PodSecurityExemptions }}{{if $i}}, {{end}}"{{ $ns }}"{{ end }}]{{ else }}["trivy-temp"]{{ end }}
  kubelet_rotate_server_certificates      = {{ .OpenCenter.Cluster.Kubernetes.KubeletRotateServerCerts | default true }}
  os_hardening_enabled                    = {{ .OpenCenter.Infrastructure.Networking.Security.OSHardening | default true }}

  {{- if .OpenCenter.Cluster.Kubernetes.OIDC.Enabled }}
  # OIDC authentication settings
  kube_oidc_auth_enabled                 = {{ .OpenCenter.Cluster.Kubernetes.OIDC.Enabled | default true }}
  kube_oidc_url                          = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCURL | default "" }}"
  kube_oidc_client_id                    = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCClientID | default "opencenter" }}"
  kube_oidc_ca_file                      = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCCAFile | default "/etc/ssl/certs/ca-certificates.crt" }}"
  kube_oidc_username_claim               = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernameClaim | default "preferred_username" }}"
  kube_oidc_username_prefix              = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernamePrefix | default "oidc:" }}"
  kube_oidc_groups_claim                 = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCGroupsClaim | default "groups" }}"
  kube_oidc_groups_prefix                = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCGroupsPrefix | default "oidc:" }}"
  {{- else }}
  kube_oidc_auth_enabled                 = false
  {{- end }}

  # Calico CNI settings (default for VMware)
  cni_iface                               = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CNIIface | default "ens192" }}"
  calico_interface_autodetect             = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CalicoInterfaceAutodetect | default "interface" }}"
  calico_interface_autodetect_cidr        = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.AutodetectCIDR | default "" }}"
  calico_encapsulation_type               = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.EncapsulationType | default "VXLAN" }}"
  calico_nat_outgoing                     = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.NATOutgoing | default true }}

  # VMware-specific settings
  address_bastion                         = "{{ .OpenCenter.Infrastructure.Bastion.Address | default "localhost" }}"
  windows_dataplane                       = {{ if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}"HNS"{{ else }}"Disabled"{{ end }}
  
  # Pre-provisioned VMware VM nodes
  {{- if .OpenCenter.Infrastructure.Cloud.VMware.Nodes }}
  master_nodes = [
  {{- range .OpenCenter.Infrastructure.Cloud.VMware.Nodes }}
  {{- if eq .Role "master" }}
    {
      id           = "{{ .Name }}"
      name         = "{{ .Name }}"
      access_ip_v4 = "{{ .IP }}"
    },
  {{- end }}
  {{- end }}
  ]

  worker_nodes = [
  {{- range .OpenCenter.Infrastructure.Cloud.VMware.Nodes }}
  {{- if eq .Role "worker" }}
    {
      id           = "{{ .Name }}"
      name         = "{{ .Name }}"
      access_ip_v4 = "{{ .IP }}"
    },
  {{- end }}
  {{- end }}
  ]
  {{- else }}
  master_nodes = []
  worker_nodes = []
  {{- end }}

  {{- if .OpenCenter.Infrastructure.Compute.WindowsNodes }}
  windows_nodes = [
  {{- range .OpenCenter.Infrastructure.Compute.WindowsNodes }}
    {
      id           = "{{ .ID }}"
      name         = "{{ .Name }}"
      access_ip_v4 = "{{ .AccessIPv4 }}"
    },
  {{- end }}
  ]
  {{- else }}
  windows_nodes = []
  {{- end }}
}

# Kubespray module for Kubernetes deployment on pre-provisioned VMware VMs
module "kubespray-cluster" {
  source = "{{ .Deployment.Kubespray.Modules.KubesprayCluster.Source | default "github.com/rackerlabs/openCenter-gitops-base.git//iac/provider/kubespray?ref=main" }}"
  
  # Bastion and cluster identification
  address_bastion                         = local.address_bastion
  cluster_name                            = local.cluster_name
  
  # Network configuration
  cni_iface                               = local.cni_iface
  subnet_nodes                            = local.subnet_nodes
  subnet_pods                             = local.subnet_pods
  subnet_services                         = local.subnet_services
  
  # DNS configuration
  dns_zone_name                           = local.dns_zone_name
  
  # Node definitions
  master_nodes                            = local.master_nodes
  worker_nodes                            = local.worker_nodes
  windows_nodes                           = local.windows_nodes
  
  # Kubernetes configuration
  network_plugin                          = local.network_plugin
  kubernetes_version                      = local.kubernetes_version
  kubespray_version                       = local.kubespray_version
  deploy_cluster                          = local.deploy_cluster
  
  # API endpoint configuration
  k8s_api_ip                              = local.k8s_api_ip
  k8s_api_port                            = local.k8s_api_port
  k8s_internal_ip                         = local.vrrp_ip
  vrrp_ip                                 = local.vrrp_ip
  vrrp_enabled                            = local.vrrp_enabled
  kube_vip_enabled                        = local.kube_vip_enabled
  
  # Security hardening
  k8s_hardening_enabled                   = local.k8s_hardening_enabled
  os_hardening_enabled                    = local.os_hardening_enabled
  kube_pod_security_exemptions_namespaces = local.kube_pod_security_exemptions_namespaces
  kubelet_rotate_server_certificates      = local.kubelet_rotate_server_certificates
  
  # SSH configuration
  ssh_user                                = local.ssh_user
  ssh_key_path                            = local.ssh_key_path
  
  # Load balancer configuration
  use_octavia                             = local.use_octavia
  
  {{- if .OpenCenter.Cluster.Kubernetes.OIDC.Enabled }}
  # OIDC authentication
  kube_oidc_auth_enabled                  = local.kube_oidc_auth_enabled
  kube_oidc_url                           = local.kube_oidc_url
  kube_oidc_client_id                     = local.kube_oidc_client_id
  kube_oidc_ca_file                       = local.kube_oidc_ca_file
  kube_oidc_username_claim                = local.kube_oidc_username_claim
  kube_oidc_username_prefix               = local.kube_oidc_username_prefix
  kube_oidc_groups_claim                  = local.kube_oidc_groups_claim
  kube_oidc_groups_prefix                 = local.kube_oidc_groups_prefix
  {{- end }}
}

{{- if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled }}
# Calico CNI module for VMware networking
module "calico" {
  source = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Modules.Calico.Source | default "github.com/rackerlabs/openCenter-gitops-base.git//iac/cni/calico?ref=main" }}"
  
  # Calico configuration
  calico_interface_autodetect      = local.calico_interface_autodetect
  calico_encapsulation_type        = local.calico_encapsulation_type
  calico_nat_outgoing              = local.calico_nat_outgoing
  calico_interface_autodetect_cidr = local.calico_interface_autodetect_cidr == "" ? local.subnet_nodes : local.calico_interface_autodetect_cidr
  
  # Network interface
  cni_iface                        = local.cni_iface
  
  # Cluster configuration
  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = local.vrrp_ip
  k8s_api_port                     = local.k8s_api_port
  
  # Network CIDRs
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  
  # Windows support
  windows_dataplane                = local.windows_dataplane
}
{{- end }}

{{- if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled }}
# Cilium CNI module for VMware networking
module "cilium" {
  source = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Modules.Cilium.Source | default "github.com/rackerlabs/openCenter-gitops-base.git//iac/cni/cilium?ref=main" }}"
  
  # Cluster configuration
  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = local.vrrp_ip
  k8s_api_port                     = local.k8s_api_port
  
  # Network CIDRs
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  
  # Cilium features
  cilium_operator_enabled          = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.OperatorEnabled | default true }}
  cilium_kube_proxy_replacement    = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.KubeProxyReplacement | default true }}
}
{{- end }}

{{- if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled }}
# Kube-OVN CNI module for VMware networking
module "kube-ovn" {
  source = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Modules.KubeOVN.Source | default "github.com/rackerlabs/openCenter-gitops-base.git//iac/cni/kube-ovn?ref=main" }}"
  
  # Cluster configuration
  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = local.vrrp_ip
  k8s_api_port                     = local.k8s_api_port
  
  # Network CIDRs
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  
  # Kube-OVN features
  kube_ovn_cilium_integration      = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.CiliumIntegration | default true }}
}
{{- end }}
