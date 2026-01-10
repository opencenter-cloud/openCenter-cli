locals {
  # this will be the user's name and the DNS zone prefix
  cluster_name                            = "{{ .OpenCenter.Cluster.ClusterName }}"
  naming_prefix                           = "${local.cluster_name}-"

  #CIDR that the openstack VMs will use for K8s nodes
  subnet_nodes                            = "{{ .IAC.Main.subnet_nodes | default "172.23.0.0/24" }}"
  subnet_nodes_oct                       = join(".", slice(split(".", split("/", local.subnet_nodes)[0]), 0, 3))
  #Leave some IPs free for the VRRP IP and the MetalLB Range
  allocation_pool_start                   = "{{ .IAC.Main.allocation_pool_start | default "" }}" != "" ? "{{ .IAC.Main.allocation_pool_start }}" : "${local.subnet_nodes_oct}.50"
  allocation_pool_end                     = "{{ .IAC.Main.allocation_pool_end | default "" }}" != "" ? "{{ .IAC.Main.allocation_pool_end }}" : "${local.subnet_nodes_oct}.254"
  # vrrp_ip Must be an IP from subnet_nodes and will be used as  the internal Kubernetes API VIP.
  vrrp_ip                                 = "{{ .IAC.Main.vrrp_ip | default "" }}" != "" ? "{{ .IAC.Main.vrrp_ip }}" : "${local.subnet_nodes_oct}.10"
  #CIDR that will be used by kubernetes pods. Not an openstack network.
  subnet_pods                             = "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods | default "10.42.0.0/16" }}"
  #CIDR that will be used for kubernetes services. Not an openstack network.
  subnet_services                         = "{{ .OpenCenter.Cluster.Kubernetes.SubnetServices | default "10.43.0.0/16" }}"
  # use_octavia set to false to create a floating IP associated with the vrrp_ip port. true will create an octavia LB with a floating IP
  use_octavia                             = {{ .IAC.Main.use_octavia | default false }}
  #loadbalancer_provider                   = "{{ .OpenCenter.Cluster.Kubernetes.LoadbalancerProvider | default "amphora" }}"
  # vrrp_enabled cannot be set to true if use_octavia is true
  vrrp_enabled                            = {{ .IAC.Main.vrrp_enabled | default true }}
  # Creates a DNS record using the LB floating IP and dns_zone_name
  use_designate                           = {{ .IAC.Main.use_designate | default false }}
  # dns_zone_name is the dns zone to create if use_designate is true
  dns_zone_name                           = "{{ .IAC.Main.dns_zone_name | default "k8s-dev.farmcreditfunding.com" }}"
  # DNS servers to configure on the nodes
  dns_nameservers                         = {{ if .IAC.Main.dns_nameservers }}[{{ range $i, $dns := .IAC.Main.dns_nameservers }}{{if $i}}, {{end}}"{{ $dns }}"{{ end }}]{{ else }}["1.1.1.1","8.8.8.8"]{{ end }}
  ntp_servers                             = {{ if .IAC.Main.ntp_servers }}[{{ range $i, $ntp := .IAC.Main.ntp_servers }}{{if $i}}, {{end}}"{{ $ntp }}"{{ end }}]{{ else }}["time.dfw3.rackspace.com","time2.dfw3.rackspace.com"]{{ end }}

  k8s_api_port                            = {{ .IAC.Main.k8s_api_port | default 443 }}
  k8s_api_port_acl                        = {{ if .OpenCenter.Cluster.K8sAPIPortACL }}[{{ range $i, $acl := .OpenCenter.Cluster.K8sAPIPortACL }}{{if $i}}, {{end}}"{{ $acl }}"{{ end }}]{{ else }}["0.0.0.0/0"]{{ end }}
  worker_count                            = {{ .OpenCenter.Cluster.Kubernetes.WorkerCount | default 3 }}
  worker_count_windows                    = {{ .OpenCenter.Cluster.Kubernetes.WorkerCountWindows | default 0 }}
  # Enter 1 or 3 masters.
  master_count                            = {{ .OpenCenter.Cluster.Kubernetes.MasterCount | default 3 }}
  ssh_user                                = "{{ .IAC.Main.ssh_user | default "ubuntu" }}"
  # these are the ssh public keys that will be able to connect to the cluster's bastion node
  ssh_authorized_keys                     = {{ if .OpenCenter.Cluster.SSHAuthorizedKeys }}[{{ range $i, $key := .OpenCenter.Cluster.SSHAuthorizedKeys }}{{if $i}}, {{end}}"{{ $key }}"{{ end }}]{{ else }}["ssh-rsa ..."]{{ end }}
  node_worker                             = "{{ .OpenCenter.Infrastructure.NodeNaming.Worker | default "wn" }}"
  node_master                             = "{{ .OpenCenter.Infrastructure.NodeNaming.Master | default "cp" }}"
  node_worker_windows                     = "{{ .OpenCenter.Infrastructure.NodeNaming.WorkerWindows | default "win" }}"
  ub_version                              = "{{ .IAC.Main.ub_version | default "24" }}"

  worker_node_bfv_volume_size             = {{ .IAC.Main.worker_node_bfv_volume_size | default 20 }}
  worker_node_bfv_destination_type        = "{{ .IAC.Main.worker_node_bfv_destination_type | default "volume" }}"
  worker_node_bfv_source_type             = "{{ .IAC.Main.worker_node_bfv_source_type | default "image" }}"
  worker_node_bfv_volume_type             = "{{ .IAC.Main.worker_node_bfv_volume_type | default "Standard" }}"

  additional_block_devices_worker = {{ if .IAC.Main.additional_block_devices_worker }}[{{ range $i, $device := .IAC.Main.additional_block_devices_worker }}{{if $i}}, {{end}}{{ $device }}{{ end }}]{{ else }}[]{{ end }}

  # ====================================
  #ca_certificates add CA certificates to server's trusts. Good for trusting internal private Certificate Authorities.
  ca_certificates                         = "{{ .IAC.Main.ca_certificates | default "" }}"
  openstack_ca                            = "{{ .IAC.Main.openstack_ca | default "" }}"

  # ====================================
  #Kubespray Settings
  kubespray_version                       = "{{ .IAC.Main.kubespray_version | default "v2.28.1" }}"
  kubernetes_version                      = "{{ .OpenCenter.Cluster.Kubernetes.Version | default "1.32.8" }}"
  network_plugin                          = "{{ if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled }}calico{{ else if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled }}cilium{{ else if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled }}kube-ovn{{ else }}calico{{ end }}"
  deploy_cluster                          = {{ .IAC.Main.deploy_cluster | default true }}
  #kub-vip settings
  kube_vip_enabled                        = {{ .IAC.Main.kube_vip_enabled | default true }}
  #Hardening
  k8s_hardening_enabled                   = {{ .IAC.Main.k8s_hardening_enabled | default true }}
  kube_pod_security_exemptions_namespaces = {{ if .IAC.Main.kube_pod_security_exemptions_namespaces }}[{{ range $i, $ns := .IAC.Main.kube_pod_security_exemptions_namespaces }}{{if $i}}, {{end}}"{{ $ns }}"{{ end }}]{{ else }}["trivy-temp"]{{ end }}
  kubelet_rotate_server_certificates      = {{ .IAC.Main.kubelet_rotate_server_certificates | default false }}
  os_hardening_enabled                    = {{ .IAC.Main.os_hardening_enabled | default true }}

  {{- if .OpenCenter.Cluster.Kubernetes.OIDC.Enabled }}
  #OIDC Settings
  kube_oidc_auth_enabled                 = {{ .OpenCenter.Cluster.Kubernetes.OIDC.Enabled | default false }}
  kube_oidc_url                          = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCURL | default "" }}"
  kube_oidc_client_id                    = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCClientID | default "kubernetes" }}"
  # # Optional settings fo OIDC
  kube_oidc_ca_file                      = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCCAFile | default "" }}"
  kube_oidc_username_claim               = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernameClaim | default "sub" }}"
  kube_oidc_username_prefix              = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernamePrefix | default "oidc:" }}"
  kube_oidc_groups_claim                 = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCGroupsClaim | default "groups" }}"
  kube_oidc_groups_prefix                = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCGroupsPrefix | default "oidc:" }}"
  {{- end }}

  {{- if gt (.OpenCenter.Cluster.Kubernetes.WorkerCountWindows | default 0) 0 }}
  # ## Windows settings
  windows_user                            = "{{ .OpenCenter.Cluster.Kubernetes.WindowsWorkers.WindowsUser | default "Administrator" }}"
  windows_admin_password                  = "{{ .OpenCenter.Cluster.Kubernetes.WindowsWorkers.WindowsAdminPassword | default "" }}"
  {{- end }}


  #Calico Settings
  cni_iface                               = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CNIIface | default "enp3s0" }}"
  #Interface detection method for Calico nodeAddressAutodetectionV4. Can be "first-found", "interface", "cidr"
  #https://docs.tigera.io/calico/latest/reference/installation/api#operator.tigera.io%2fv1.NodeAddressAutodetection
  calico_interface_autodetect             = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CalicoInterfaceAutodetect | default "interface" }}"
  calico_interface_autodetect_cidr        = "{{ .IAC.Main.calico_interface_autodetect_cidr | default "" }}"
  calico_encapsulation_type               = "{{ .IAC.Main.calico_encapsulation_type | default "VXLAN" }}"
  calico_nat_outgoing                     = {{ .IAC.Main.calico_nat_outgoing | default true }}

######################
  address_bastion                        = "{{ .IAC.Main.address_bastion | default "50.56.158.76" }}" ##Or Public IP NATed to Bastion
  windows_dataplane                       = {{ if gt (.OpenCenter.Cluster.Kubernetes.WorkerCountWindows | default 0) 0 }}"HSN"{{ else }}"Disabled"{{ end }}
  k8s_api_ip                              = "{{ .IAC.Main.k8s_api_ip | default "" }}" != "" ? "{{ .IAC.Main.k8s_api_ip }}" : local.vrrp_ip
  ssh_key_path                            = "{{ .IAC.Main.ssh_key_path | default "" }}" 

  {{- if .OpenCenter.Cluster.Kubernetes.MasterNodes }}
  master_nodes = [
  {{- range .OpenCenter.Cluster.Kubernetes.MasterNodes }}
  {
  id = "{{ .ID }}"
  name = "{{ .Name }}"
  access_ip_v4 = "{{ .AccessIPv4 }}"
  },
  {{- end }}
  ]
  {{- else }}
  master_nodes = []
  {{- end }}

  {{- if .OpenCenter.Cluster.Kubernetes.WorkerNodes }}
  worker_nodes = [
  {{- range .OpenCenter.Cluster.Kubernetes.WorkerNodes }}
  {
  id = "{{ .ID }}"
  name = "{{ .Name }}"
  access_ip_v4 = "{{ .AccessIPv4 }}"
  },
  {{- end }}
  ]
  {{- else }}
  worker_nodes = []
  {{- end }}

  {{- if .OpenCenter.Cluster.Kubernetes.WindowsNodes }}
  windows_nodes = [
  {{- range .OpenCenter.Cluster.Kubernetes.WindowsNodes }}
  {
  id = "{{ .ID }}"
  name = "{{ .Name }}"
  access_ip_v4 = "{{ .AccessIPv4 }}"
  },
  {{- end }}
  ]
  {{- else }}
  windows_nodes = []
  {{- end }}
}

module "kubespray-cluster" {
  source = "{{ (index .IAC.Modules "kubespray-cluster").source | default "github.com/rackerlabs/openCenter-gitops-base.git//iac/provider/kubespray?ref=main" }}"
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
  {{- if gt (.OpenCenter.Cluster.Kubernetes.WorkerCountWindows | default 0) 0 }}
  windows_nodes                           = local.windows_nodes
  {{- else }}
  windows_nodes                           = []
  {{- end }}
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
  {{- end }}
}


{{- if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled }}
module "calico" {
  source = "{{ (index .IAC.Modules "calico").source | default "github.com/rackerlabs/openCenter-gitops-base.git//iac/cni/calico?ref=main" }}"

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

{{- if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled }}
module "cilium" {
  source = "{{ (index .IAC.Modules "cilium").source | default "github.com/rackerlabs/openCenter-gitops-base.git//iac/cni/cilium?ref=main" }}"

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

{{- if .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled }}
module "kube-ovn" {
  source = "{{ (index .IAC.Modules "kube-ovn").source | default "github.com/rackerlabs/openCenter-gitops-base.git//iac/cni/kube-ovn?ref=main" }}"

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

