locals {
  # this will be the user's name and the DNS zone prefix
  cluster_name                            = "{{ .OpenCenter.Cluster.ClusterName }}"
  # Prefix to add to Openstack resource names
  naming_prefix                           = "${local.cluster_name}-"
{{- if ne (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
  openstack_auth_url                      = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL | default "https://keystone.api.sjc3.rackspacecloud.com/v3/" }}"
  openstack_insecure                      = {{ .OpenCenter.Infrastructure.Cloud.OpenStack.Insecure | default false }}
  openstack_region                        = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Region | default "SJC3" }}"
  availability_zone                       = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.AvailabilityZone | default "az1" }}"
  openstack_user_name                     = ""
  openstack_user_password                 = ""
  application_credential_id               = var.os_application_credential_id
  application_credential_secret           = var.os_application_credential_secret
  openstack_project_domain_name           = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.ProjectDomainName | default "rackspace_cloud_domain" }}"
  openstack_user_domain_name              = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.UserDomainName | default "rackspace_cloud_domain" }}"
  openstack_tenant_name                   = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.TenantName | default "f2823901-4194-40c7-9dc4-d56d2105e81a" }}"
  floatingip_pool                         = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.FloatingIPPool | default "PUBLICNET" }}"
  router_external_network_id              = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.RouterExternalNetworkID | default "" }}"
  # VLAN settings
  vlan_id                                 = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.VLAN.ID | default "" }}"
  mtu                                     = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.VLAN.MTU | default "" }}"
  network_provider                        = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.VLAN.Provider | default "physnet1" }}"
{{- end }}
  #CIDR that the openstack VMs will use for K8s nodes
  subnet_nodes                            = "{{ .OpenCenter.Infrastructure.Networking.SubnetNodes | default "10.2.128.0/22" }}"
  subnet_nodes_oct                       = join(".", slice(split(".", split("/", local.subnet_nodes)[0]), 0, 3))
  #Leave some IPs free for the VRRP IP and the MetalLB Range
  allocation_pool_start                   = "{{ .OpenCenter.Infrastructure.Networking.AllocationPoolStart | default "${local.subnet_nodes_oct}.50" }}"
  allocation_pool_end                     = "{{ .OpenCenter.Infrastructure.Networking.AllocationPoolEnd | default "${local.subnet_nodes_oct}.254" }}"
  # vrrp_ip Must be an IP from subnet_nodes and will be used as the internal Kubernetes API VIP.
  vrrp_ip                                 = "{{ .OpenCenter.Infrastructure.Networking.VRRPIP | default "${local.subnet_nodes_oct}.10" }}"
  #CIDR that will be used by kubernetes pods. Not an openstack network.
  subnet_pods                             = "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods | default "10.42.0.0/16" }}"
  #CIDR that will be used for kubernetes services. Not an openstack network.
  subnet_services                         = "{{ .OpenCenter.Cluster.Kubernetes.SubnetServices | default "10.43.0.0/16" }}"
  # use_octavia set to false to create a floating IP associated with the vrrp_ip port. true will create an octavia LB with a floating IP
  use_octavia                             = {{ .OpenCenter.Infrastructure.Networking.UseOctavia | default false }}
  loadbalancer_provider                   = "{{ .OpenCenter.Infrastructure.Networking.LoadbalancerProvider | default "ovn" }}"
  # vrrp_enabled cannot be set to true if use_octavia is true
  vrrp_enabled                            = "{{ .OpenCenter.Infrastructure.Networking.VRRPEnabled | default true }}"
  # Creates a DNS record using the LB floating IP and dns_zone_name
  use_designate                           = {{ .OpenCenter.Infrastructure.Networking.UseDesignate | default false }}
  # dns_zone_name is the dns zone to create if use_designate is true
  dns_zone_name                           = "{{ .OpenCenter.Infrastructure.Networking.DNSZoneName | default "" }}"
  # DNS servers to configure on the nodes
  dns_nameservers                         = {{ if .OpenCenter.Infrastructure.Networking.DNSNameservers }}[{{ range $i, $dns := .OpenCenter.Infrastructure.Networking.DNSNameservers }}{{if $i}}, {{end}}"{{ $dns }}"{{ end }}]{{ else }}["1.1.1.1","8.8.8.8"]{{ end }}
  ntp_servers                             = {{ if .OpenCenter.Infrastructure.Networking.NTPServers }}[{{ range $i, $ntp := .OpenCenter.Infrastructure.Networking.NTPServers }}{{if $i}}, {{end}}"{{ $ntp }}"{{ end }}]{{ else }}["time.dfw3.rackspace.com","time2.dfw3.rackspace.com"]{{ end }}
{{- if ne (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
  image_id                                = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.ImageID | default "" }}"
  image_id_windows                        = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.ImageIDWindows | default "" }}"
{{- end }}
  k8s_api_port                            = {{ .OpenCenter.Cluster.Kubernetes.APIPort | default 443 }}
  k8s_api_port_acl                        = {{ if .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL }}[{{ range $i, $acl := .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL }}{{if $i}}, {{end}}"{{ $acl }}"{{ end }}]{{ else }}["0.0.0.0/0"]{{ end }}
  worker_count                            = {{ .OpenCenter.Infrastructure.Compute.WorkerCount | default 4 }}
  worker_count_windows                    = {{ .OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0 }}
  # Enter 1 or 3 masters.
  master_count                            = {{ .OpenCenter.Infrastructure.Compute.MasterCount | default 3 }}
  ssh_user                                = "{{ .OpenCenter.Infrastructure.SSH.Username | default .OpenCenter.Infrastructure.SSH.User | default "ubuntu" }}"
  # these are the ssh public keys that will be able to connect to the cluster's bastion node
  ssh_authorized_keys                     = {{ if .OpenCenter.Infrastructure.SSH.AuthorizedKeys }}[{{ range $i, $key := .OpenCenter.Infrastructure.SSH.AuthorizedKeys }}{{if $i}}, {{end}}"{{ $key }}"{{ end }}]{{ else }}["ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDogzEullM89m//Vd8IGPERto2DotXnUCKGH6II1Vk/klEuDVqXx9kCb981XJKh8mU15bfJVdE4h078q/shK9EIcPMRKSQSMs2LkgF/1yUeVYPNYiIBph6CaqjIxKHy1kYxw3KUTIh8IIl1M4t5fc5c49Gr3QuDpeMN4Z/wrbR1DceIbFDiVxYNeyJWfOdowKgTn4AKh0n1xtg6/XLin3cCstpvfUJUKm0WOcmn3+DHK6cBNqNAMKdtxgnGwlY4MfizJOZE30Y7hwPqXUjOgLgB2vybcdcMpUvw9e8HopogOFQnVwwmlc9/7ZKPCaCKRBEC38IV82CJ6+/eePIMriPF migu4903@MNF0TUDV30"]{{ end }}
  node_worker                             = "wn"
  node_master                             = "cp"
  node_worker_windows                     = "win"
  ub_version                              = "{{ .OpenCenter.Infrastructure.OSVersion | default "24" }}"
{{- if ne (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
  #FLEX Flavor Settings ==========================
  flavor_bastion                          = "{{ .OpenCenter.Infrastructure.Compute.FlavorBastion | default "" }}"
  flavor_master                           = "{{ .OpenCenter.Infrastructure.Compute.FlavorMaster | default "" }}"
  flavor_worker                           = "{{ .OpenCenter.Infrastructure.Compute.FlavorWorker | default "" }}"
  flavor_worker_windows                   = "{{ .OpenCenter.Infrastructure.Compute.FlavorWorkerWindows | default "" }}"

  worker_node_bfv_volume_size             = {{ .OpenCenter.Infrastructure.Storage.WorkerVolumeSize | default 100 }}
  worker_node_bfv_destination_type        = "{{ .OpenCenter.Infrastructure.Storage.WorkerVolumeDestinationType | default "volume" }}"
  worker_node_bfv_source_type             = "{{ .OpenCenter.Infrastructure.Storage.WorkerVolumeSourceType | default "image" }}"
  worker_node_bfv_volume_type             = "{{ .OpenCenter.Infrastructure.Storage.WorkerVolumeType | default "HA-Performance" }}"
  wn_server_group_affinity                = {{ if .OpenCenter.Infrastructure.ServerGroupAffinity }}[{{ range $i, $affinity := .OpenCenter.Infrastructure.ServerGroupAffinity }}{{if $i}}, {{end}}"{{ $affinity }}"{{ end }}]{{ else }}["anti-affinity"]{{ end }}
{{- else }}
  worker_node_bfv_volume_size             = {{ .OpenCenter.Infrastructure.Storage.WorkerVolumeSize | default 20 }}
  worker_node_bfv_destination_type        = "{{ .OpenCenter.Infrastructure.Storage.WorkerVolumeDestinationType | default "volume" }}"
  worker_node_bfv_source_type             = "{{ .OpenCenter.Infrastructure.Storage.WorkerVolumeSourceType | default "image" }}"
  worker_node_bfv_volume_type             = "{{ .OpenCenter.Infrastructure.Storage.WorkerVolumeType | default "Standard" }}"
{{- end }}
  additional_block_devices_worker = {{ if .OpenCenter.Infrastructure.Storage.AdditionalBlockDevices }}[{{ range $i, $device := .OpenCenter.Infrastructure.Storage.AdditionalBlockDevices }}{{if $i}}, {{end}}{{ $device }}{{ end }}]{{ else }}[]{{ end }}

  additional_server_pools_worker = {{ if .OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker }}[{{ range $i, $pool := .OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker }}{{if $i}}, {{end}}{
    name                                = "{{ $pool.Name }}"
    worker_count                        = {{ $pool.Count }}
    flavor_worker                       = "{{ $pool.Flavor }}"
    node_worker                         = "{{ $pool.Name }}"
    {{- if $pool.Image }}
    image_id                            = "{{ $pool.Image }}"
    {{- end }}
    {{- if $pool.BootVolume.Size }}
    worker_node_bfv_volume_size         = {{ $pool.BootVolume.Size }}
    {{- end }}
    {{- if $pool.BootVolume.DestinationType }}
    worker_node_bfv_destination_type    = "{{ $pool.BootVolume.DestinationType }}"
    {{- end }}
    {{- if $pool.BootVolume.SourceType }}
    worker_node_bfv_source_type         = "{{ $pool.BootVolume.SourceType }}"
    {{- end }}
    {{- if $pool.BootVolume.Type }}
    worker_node_bfv_volume_type         = "{{ $pool.BootVolume.Type }}"
    {{- end }}
    worker_node_bfv_delete_on_termination = {{ $pool.BootVolume.DeleteOnTermination }}
  }{{ end }}]{{ else }}[]{{ end }}

  # ===================================
  #ca_certificates add CA certificates to server's trusts. Good for trusting internal private Certificate Authorities.
  ca_certificates                         = ""
  openstack_ca                            = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.CA | default "" }}"

  # ====================================
  #Kubespray Settings
  kubespray_version                       = "{{ if hasPrefix "v" (.Deployment.Kubespray.Version | default "v2.31.0") }}{{ .Deployment.Kubespray.Version | default "v2.31.0" }}{{ else }}v{{ .Deployment.Kubespray.Version }}{{ end }}"
  kubernetes_version                      = "{{ .OpenCenter.Cluster.Kubernetes.Version | default "1.33.7" }}"
  # CNI install_method: "helm" (default) skips CNI in Kubespray, CNI will be installed via GitOps/Helm after cluster is up
  #                     "kubespray" installs CNI during cluster bootstrap
  # When install_method is "helm", network_plugin is set to "none" so Kubespray skips CNI installation
  network_plugin                          = "{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled }}{{- if eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod | default "helm") "helm" }}none{{- else }}calico{{- end }}{{- else if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled }}{{- if eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.InstallMethod | default "helm") "helm" }}none{{- else }}cilium{{- end }}{{- else if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled }}{{- if eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.InstallMethod | default "helm") "helm" }}none{{- else }}kube-ovn{{- end }}{{- else }}none{{- end }}"
  deploy_cluster                          = {{ .Deployment.AutoDeploy | default true }}
  #kub-vip settings
  kube_vip_enabled                        = {{ .OpenCenter.Cluster.Kubernetes.KubeVIPEnabled | default true }}
  #Hardening
  k8s_hardening_enabled                   = true
  kube_pod_security_exemptions_namespaces = ["trivy-temp"]
  kubelet_rotate_server_certificates      = true
  os_hardening_enabled                    = true

  {{- if .OpenCenter.Cluster.Kubernetes.OIDC.Enabled }}
  #OIDC Settings
  kube_oidc_auth_enabled                 = {{ .OpenCenter.Cluster.Kubernetes.OIDC.Enabled | default true }}
  kube_oidc_url                          = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.IssuerURL | default "https://auth.gdo.prod.sjc3.k8s.opencenter.cloud/realms/opencenter" }}"
  kube_oidc_client_id                    = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.ClientID | default "opencenter" }}"
  # Optional settings fo OIDC
  kube_oidc_ca_file                      = "/etc/ssl/certs/ca-certificates.crt"
  kube_oidc_username_claim               = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.UsernameClaim | default "preferred_username" }}"
  kube_oidc_username_prefix              = "oidc:"
  kube_oidc_groups_claim                 = "{{ .OpenCenter.Cluster.Kubernetes.OIDC.GroupsClaim | default "groups" }}"
  kube_oidc_groups_prefix                = "oidc:"
  {{- else }}
  {{- end }}

  #Calico Settings
  cni_iface                               = "enp3s0"
  #Interface detection method for Calico nodeAddressAutodetectionV4. Can be "first-found", "interface", "cidr"
  #https://docs.tigera.io/calico/latest/reference/installation/api#operator.tigera.io%2fv1.NodeAddressAutodetection
  calico_interface_autodetect             = "interface"
  calico_interface_autodetect_cidr        = ""
  calico_encapsulation_type               = "VXLAN"
  calico_nat_outgoing                     = true

  {{- if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}
  # ## Windows settings
  windows_user                            = "Administrator"
  windows_admin_password                  = ""
  worker_node_bfv_size_windows            = 100
  worker_node_bfv_type_windows            = "volume"
  additional_server_pools_worker_windows  = []
  {{- else }}
  additional_server_pools_worker_windows = []
  {{- end }}

{{- if eq (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
######################
# Baremetal-specific settings
######################
  address_bastion                        = "{{ .OpenCenter.Infrastructure.Bastion.Address | default "50.56.158.76" }}" ##Or Public IP NATed to Bastion
  windows_dataplane                       = {{ if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}"HSN"{{ else }}"Disabled"{{ end }}
  k8s_api_ip                              = "{{ .OpenCenter.Infrastructure.K8sAPIIP | default "" }}" != "" ? "{{ .OpenCenter.Infrastructure.K8sAPIIP }}" : local.vrrp_ip
  ssh_key_path                            = "{{ .OpenCenter.Infrastructure.SSH.KeyPath | default "" }}" 

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
{{- end }}
}

{{- if ne (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
module "openstack-nova" {
  source = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Modules.OpenstackNova.Source | default "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cloud/openstack/openstack-nova?ref=main" }}"
  # source = "../../../gitclones/openCenter-gitops-base/iac/cloud/openstack/openstack-nova"
  availability_zone             = local.availability_zone
  additional_block_devices_worker      = local.additional_block_devices_worker
  {{- if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}
  additional_server_pools_worker_windows = local.additional_server_pools_worker_windows
  {{- end }}
  {{- if .OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker }}
  additional_server_pools_worker = local.additional_server_pools_worker
  {{- end }}
  application_credential_id     = local.application_credential_id
  application_credential_secret = local.application_credential_secret
  ca_certificates               = local.ca_certificates
  use_octavia                   = local.use_octavia
  use_designate                 = local.use_designate
  dns_nameservers               = local.dns_nameservers
  dns_zone_name                 = local.dns_zone_name
  flavor_bastion                = local.flavor_bastion
  openstack_auth_url            = local.openstack_auth_url
  openstack_ca                  = local.openstack_ca
  openstack_insecure            = local.openstack_insecure
  openstack_region              = local.openstack_region
  openstack_tenant_name         = local.openstack_tenant_name
  openstack_user_name           = local.openstack_user_name
  openstack_password            = local.openstack_user_password
  openstack_project_domain_name = local.openstack_project_domain_name
  openstack_user_domain_name    = local.openstack_user_domain_name
  naming_prefix                 = local.naming_prefix
  ntp_servers                   = local.ntp_servers
  ssh_user                      = local.ssh_user
  floatingip_pool               = local.floatingip_pool
  image_id                      = local.image_id
  image_id_windows              = local.image_id_windows
  router_external_network_id    = local.router_external_network_id
  network_id                    = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.Networking.NetworkID | default "" }}"
  vlan_id                       = local.vlan_id
  vrrp_enabled                  = local.vrrp_enabled
  vrrp_ip                       = local.vrrp_ip
  ssh_authorized_keys           = local.ssh_authorized_keys
  subnet_nodes                  = local.subnet_nodes
  subnet_services               = local.subnet_services
  subnet_pods                   = local.subnet_pods
  allocation_pool_start         = local.allocation_pool_start
  allocation_pool_end           = local.allocation_pool_end
  k8s_api_port                  = local.k8s_api_port
  k8s_api_port_acl              = local.k8s_api_port_acl
  size_master = {
    count  = local.master_count
    flavor = local.flavor_master
  }
  size_worker = {
    count  = local.worker_count
    flavor = local.flavor_worker
  }
  {{- if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}
  size_worker_windows = {
      count  = local.worker_count_windows
      flavor = local.flavor_worker_windows
  }
  {{- end }}
  node_master                  = local.node_master
  node_worker                  = local.node_worker
  node_worker_windows          = local.node_worker_windows
  ub_version                   = local.ub_version

  worker_node_bfv_volume_size = local.worker_node_bfv_volume_size
  worker_node_bfv_destination_type = local.worker_node_bfv_destination_type
  worker_node_bfv_source_type = local.worker_node_bfv_source_type
  worker_node_bfv_volume_type = local.worker_node_bfv_volume_type
  wn_server_group_affinity = local.wn_server_group_affinity
}
{{- end }}

module "kubespray-cluster" {
  source = "{{ (index .Deployment.Kubespray.Modules "kubespray").Source | default "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/provider/kubespray?ref=main" }}"
{{- if eq (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
  address_bastion                         = local.address_bastion
{{- else }}
  address_bastion                         = module.openstack-nova.bastion_floating_ip
{{- end }}
  cluster_name                            = local.cluster_name
  cni_iface                               = local.cni_iface
  deploy_cluster                          = local.deploy_cluster
  dns_zone_name                           = local.dns_zone_name
{{- if eq (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
  master_nodes                            = local.master_nodes
{{- else }}
  master_nodes                            = module.openstack-nova.master_nodes
{{- end }}
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
{{- if eq (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
  worker_nodes                            = local.worker_nodes
  k8s_api_ip                              = local.k8s_api_ip
  k8s_internal_ip                         = local.vrrp_ip
{{- else }}
  worker_nodes                            = module.openstack-nova.worker_nodes
  k8s_api_ip                              = module.openstack-nova.k8s_api_ip
  k8s_internal_ip                         = module.openstack-nova.k8s_internal_ip
{{- end }}
  k8s_api_port                            = local.k8s_api_port
  vrrp_ip                                 = local.vrrp_ip
  vrrp_enabled                            = local.vrrp_enabled
{{- if eq (.OpenCenter.Infrastructure.Provider | default "openstack") "baremetal" }}
  {{- if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}
  windows_nodes                           = local.windows_nodes
  {{- else }}
  windows_nodes                           = []
  {{- end }}
  ssh_key_path                            = local.ssh_key_path
  use_octavia                             = local.use_octavia
{{- else }}
  {{- if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}
  windows_nodes = concat(
    module.openstack-nova.windows_nodes,
    flatten([
      for pool_name, pool_nodes in module.openstack-nova.additional_worker_pools_windows_nodes : pool_nodes
    ])
  )
  {{- else }}
  windows_nodes                           = []
  {{- end }}
  use_octavia                             = local.use_octavia
{{- end }}
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


{{- /* Only include Calico Terraform module when install_method is explicitly "kubespray" */}}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled (eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod | default "helm") "kubespray") }}
module "calico" {
  source = "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/calico?ref=main"

  calico_interface_autodetect      = local.calico_interface_autodetect
  calico_encapsulation_type        = local.calico_encapsulation_type
  calico_nat_outgoing              = local.calico_nat_outgoing
  calico_interface_autodetect_cidr = local.calico_interface_autodetect_cidr == "" ? local.subnet_nodes : local.calico_interface_autodetect_cidr
  cni_iface                        = local.cni_iface
  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = module.openstack-nova.k8s_internal_ip
  k8s_api_port                     = local.k8s_api_port
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  {{- if gt (.OpenCenter.Infrastructure.Compute.WorkerCountWindows | default 0) 0 }}
  windows_dataplane                = "HNS"
  {{- else }}
  windows_dataplane                = "Disabled"
  {{- end }}
}
{{- end }}

{{- /* Only include Cilium Terraform module when install_method is explicitly "kubespray" */}}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled (eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.InstallMethod | default "helm") "kubespray") }}
module "cilium" {
  source = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Modules.Cilium.Source | default "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/cilium?ref=main" }}"

  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = module.openstack-nova.k8s_internal_ip
  k8s_api_port                     = local.k8s_api_port
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  cilium_operator_enabled          = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.OperatorEnabled | default true }}
  cilium_kube_proxy_replacement    = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.KubeProxyReplacement | default true }}
}
{{- end }}

{{- /* Only include Kube-OVN Terraform module when install_method is explicitly "kubespray" */}}
{{- if and .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled (eq (.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.InstallMethod | default "helm") "kubespray") }}
module "kube-ovn" {
  source = "{{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Modules.KubeOVN.Source | default "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/kube-ovn?ref=main" }}"

  cluster_name                     = local.cluster_name
  deploy_cluster                   = local.deploy_cluster
  k8s_internal_ip                  = module.openstack-nova.k8s_internal_ip
  k8s_api_port                     = local.k8s_api_port
  subnet_nodes                     = local.subnet_nodes
  subnet_pods                      = local.subnet_pods
  subnet_services                  = local.subnet_services
  kube_ovn_cilium_integration      = {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.CiliumIntegration | default true }}
}
{{- end }}
