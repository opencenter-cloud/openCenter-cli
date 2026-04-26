locals {
  # this will be the user's name and the DNS zone prefix
  cluster_name                            = "{{ .OpenCenter.Cluster.ClusterName }}"

  #CIDR that the nodes will use for K8s nodes
  subnet_nodes                            = "{{ .OpenCenter.Infrastructure.Networking.SubnetNodes | default "172.26.0.0/24" }}"
  #CIDR that will be used by kubernetes pods. Not an infrastructure network.
  subnet_pods                             = "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods | default "10.42.0.0/16" }}"
  #CIDR that will be used for kubernetes services. Not an infrastructure network.
  subnet_services                         = "{{ .OpenCenter.Cluster.Kubernetes.SubnetServices | default "10.43.0.0/16" }}"
  # vrrp_enabled cannot be set to true if use_octavia is true
  vrrp_enabled                            = {{ .OpenCenter.Infrastructure.Networking.VRRPEnabled | default true }}
  # Creates a DNS record using the LB floating IP and dns_zone_name
  dns_zone_name                           = "{{ .OpenCenter.Infrastructure.Networking.DNSZoneName | default "" }}"

  k8s_api_port                            = {{ .OpenCenter.Cluster.Kubernetes.APIPort | default 443 }}
  ssh_user                                = "{{ .OpenCenter.Infrastructure.SSHUser | default "ubuntu" }}"
  use_octavia                             = false

  # ====================================
  #Kubespray Settings
  kubespray_version                       = "{{ if hasPrefix "v" (.Deployment.Kubespray.Version | default "v2.31.0") }}{{ .Deployment.Kubespray.Version | default "v2.31.0" }}{{ else }}v{{ .Deployment.Kubespray.Version }}{{ end }}"
  kubernetes_version                      = "{{ .OpenCenter.Cluster.Kubernetes.Version | default "1.33.7" }}"
  network_plugin                          = "{{ if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled }}calico{{ else if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled }}cilium{{ else if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled }}kube-ovn{{ else }}calico{{ end }}"
  deploy_cluster                          = {{ .Deployment.AutoDeploy | default true }}
  #kube-vip settings
  kube_vip_enabled                        = {{ .OpenCenter.Cluster.Kubernetes.KubeVIPEnabled | default true }}
  #Hardening
  k8s_hardening_enabled                   = {{ .OpenCenter.Cluster.Kubernetes.Security.K8sHardening | default true }}
  kube_pod_security_exemptions_namespaces = {{ if .OpenCenter.Cluster.Kubernetes.Security.PodSecurityExemptions }}[{{ range $i, $ns := .OpenCenter.Cluster.Kubernetes.Security.PodSecurityExemptions }}{{if $i}}, {{end}}"{{ $ns }}"{{ end }}]{{ else }}["trivy-temp"]{{ end }}
  kubelet_rotate_server_certificates      = {{ .OpenCenter.Cluster.Kubernetes.KubeletRotateServerCerts | default true }}
  os_hardening_enabled                    = {{ .OpenCenter.Infrastructure.Networking.Security.OSHardening | default true }}

  {{- if .OpenCenter.Cluster.Kubernetes.OIDC.Enabled }}
  #OIDC Settings
  kube_oidc_auth_enabled                 = {{ .OpenCenter.Cluster.Kubernetes.OIDC.Enabled | default true }}
  kube_oidc_url                          = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCURL | default "" }}"
  kube_oidc_client_id                    = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCClientID | default "opencenter" }}"
  # Optional settings for OIDC
  kube_oidc_ca_file                      = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCCAFile | default "/etc/ssl/certs/ca-certificates.crt" }}"
  kube_oidc_username_claim               = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernameClaim | default "preferred_username" }}"
  kube_oidc_username_prefix              = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernamePrefix | default "oidc:" }}"
  kube_oidc_groups_claim                 = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCGroupsClaim | default "groups" }}"
  kube_oidc_groups_prefix                = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCGroupsPrefix | default "oidc:" }}"
  {{- else }}
  kube_oidc_auth_enabled                 = false
  {{- end }}

  #Calico Settings
  cni_iface                               = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CNIIface | default "ens192" }}"
  #Interface detection method for Calico nodeAddressAutodetectionV4. Can be "first-found", "interface", "cidr"
  calico_interface_autodetect             = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CalicoInterfaceAutodetect | default "interface" }}"
  calico_interface_autodetect_cidr        = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.AutodetectCIDR | default "" }}"
  calico_encapsulation_type               = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.EncapsulationType | default "VXLAN" }}"
  calico_nat_outgoing                     = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.NATOutgoing | default true }}

  ######################
  # Baremetal-specific settings
  ######################
  address_bastion                         = "{{ .OpenCenter.Infrastructure.Bastion.Address | default "localhost" }}"
  k8s_api_ip                              = "{{ .OpenCenter.Infrastructure.K8sAPIIP | default "" }}" != "" ? "{{ .OpenCenter.Infrastructure.K8sAPIIP }}" : local.vrrp_ip
  windows_dataplane                       = {{ if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}"HNS"{{ else }}"Disabled"{{ end }}
  vrrp_ip                                 = "{{ .OpenCenter.Infrastructure.Networking.VRRPIP | default "172.26.0.5" }}"
  ssh_key_path                            = "{{ .OpenCenter.Infrastructure.SSHKeyPath | default "" }}"
  
  {{- if .OpenCenter.Infrastructure.Compute.MasterNodes }}
  master_nodes = [
  {{- range .OpenCenter.Infrastructure.Compute.MasterNodes }}
    {
      id           = "{{ .ID }}"
      name         = "{{ .Name }}"
      access_ip_v4 = "{{ .AccessIPv4 }}"
    },
  {{- end }}
  ]
  {{- else }}
  master_nodes = []
  {{- end }}

  {{- if .OpenCenter.Infrastructure.Compute.WorkerNodes }}
  worker_nodes = [
  {{- range .OpenCenter.Infrastructure.Compute.WorkerNodes }}
    {
      id           = "{{ .ID }}"
      name         = "{{ .Name }}"
      access_ip_v4 = "{{ .AccessIPv4 }}"
    },
  {{- end }}
  ]
  {{- else }}
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

module "kubespray-cluster" {
  source = "{{ .Deployment.Kubespray.Modules.KubesprayCluster.Source | default "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/provider/kubespray?ref=main" }}"
  address_bastion                         = local.address_bastion
  cluster_name                            = local.cluster_name
  cni_iface                               = local.cni_iface
  deploy_cluster                          = local.deploy_cluster
  dns_zone_name                           = local.dns_zone_name
  master_nodes                            = local.master_nodes
  network_plugin                          = local.network_plugin
  k8s_hardening_enabled                   = local.k8s_hardening_enabled
  os_hardening_enabled                    = local.os_hardening_enabled
  ssh_user                                = local.ssh_user
  subnet_nodes                            = local.subnet_nodes
  subnet_pods                             = local.subnet_pods
  subnet_services                         = local.subnet_services
  kubernetes_version                      = local.kubernetes_version
  kubespray_version                       = local.kubespray_version
  kube_vip_enabled                        = local.kube_vip_enabled
  kube_pod_security_exemptions_namespaces = local.kube_pod_security_exemptions_namespaces
  kubelet_rotate_server_certificates      = local.kubelet_rotate_server_certificates
  worker_nodes                            = local.worker_nodes
  k8s_api_ip                              = local.k8s_api_ip
  k8s_api_port                            = local.k8s_api_port
  k8s_internal_ip                         = local.vrrp_ip
  vrrp_ip                                 = local.vrrp_ip
  vrrp_enabled                            = local.vrrp_enabled
  windows_nodes                           = local.windows_nodes
  ssh_key_path                            = local.ssh_key_path
  use_octavia                             = local.use_octavia
  {{- if .OpenCenter.Cluster.Kubernetes.OIDC.Enabled }}
  kube_oidc_auth_enabled                  = local.kube_oidc_auth_enabled
  kube_oidc_url                           = local.kube_oidc_url
  kube_oidc_client_id                     = local.kube_oidc_client_id
  kube_oidc_ca_file                       = local.kube_oidc_ca_file
  kube_oidc_username_claim                = local.kube_oidc_username_claim
  kube_oidc_username_prefix               = local.kube_oidc_username_prefix
  kube_oidc_groups_claim                  = local.kube_oidc_groups_claim
  kube_oidc_groups_prefix                 = local.kube_oidc_groups_prefix
  {{- else }}
  {{- end }}
}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled }}
module "calico" {
  source = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Modules.Calico.Source | default "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/calico?ref=main" }}"
  calico_interface_autodetect      = local.calico_interface_autodetect
  calico_encapsulation_type        = local.calico_encapsulation_type
  calico_nat_outgoing              = local.calico_nat_outgoing
  calico_interface_autodetect_cidr = local.calico_interface_autodetect_cidr == "" ? local.subnet_nodes : local.calico_interface_autodetect_cidr
  cni_iface                        = local.cni_iface
  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = local.vrrp_ip
  k8s_api_port                     = local.k8s_api_port
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  windows_dataplane                = local.windows_dataplane
}
{{- end }}

{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled }}
module "cilium" {
  source = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Modules.Cilium.Source | default "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/cilium?ref=main" }}"
  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = local.vrrp_ip
  k8s_api_port                     = local.k8s_api_port
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  cilium_operator_enabled          = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.OperatorEnabled | default true }}
  cilium_kube_proxy_replacement    = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.KubeProxyReplacement | default true }}
}
{{- end }}

{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled }}
module "kube-ovn" {
  source = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Modules.KubeOVN.Source | default "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/kube-ovn?ref=main" }}"
  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = local.vrrp_ip
  k8s_api_port                     = local.k8s_api_port
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  kube_ovn_cilium_integration      = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.CiliumIntegration | default true }}
}
{{- end }}
