locals {
  # this will be the user's name and the DNS zone prefix
  cluster_name = "dev-cluster"
  # Prefix to add to Openstack resource names
  naming_prefix                           = local.cluster_name
  openstack_auth_url                      = "https://keystone.api.sjc3.rackspacecloud.com/v3/"
  openstack_insecure                      = false
  openstack_region                        = "SJC3"
  availability_zone                       = "az1"
  openstack_user_name                     = var.openstack_user_name == "" ? local.cluster_name : var.openstack_user_name
  openstack_user_password                 = var.openstack_user_password
  openstack_admin_password                = var.openstack_admin_password
  openstack_project_domain_name           = "rackspace_cloud_domain"
  openstack_user_domain_name              = "rackspace_cloud_domain"
  openstack_tenant_name                   = "981977_Flex"
  floatingip_pool                         = "PUBLICNET"
  router_external_network_id              = "723f8fa2-dbf7-4cec-8d5f-017e62c12f79"
  # VLAN settings
  vlan_id                                 = ""
  mtu                                     = ""
  network_provider                        = "physnet1"
  #CIDR that the openstack VMs will use for K8s nodes
  subnet_nodes                            = "10.2.188.0/22"
  subnet_nodes_oct                       = join(".", slice(split(".", split("/", local.subnet_nodes)[0]), 0, 3))
  #Leave some IPs free for the VRRP IP and the MetalLB Range
  allocation_pool_start                   = "${local.subnet_nodes_oct}.50"
  allocation_pool_end                     = "${local.subnet_nodes_oct}.254"
  # vrrp_ip Must be an IP from subnet_nodes and will be used as the internal Kubernetes API VIP.
  vrrp_ip                                 = "${local.subnet_nodes_oct}.10"
  #CIDR that will be used by kubernetes pods. Not an openstack network.
  subnet_pods                             = "10.42.0.0/16"
  #CIDR that will be used for kubernetes services. Not an openstack network.
  subnet_services                         = "10.43.0.0/16"
  # use_octavia set to false to create a floating IP associated with the vrrp_ip port. true will create an octavia LB with a floating IP
  use_octavia                             = false
  loadbalancer_provider                   = "amphora"
  # vrrp_enabled cannot be set to true if use_octavia is true
  vrrp_enabled                            = true
  # Creates a DNS record using the LB floating IP and dns_zone_name
  use_designate                           = false
  # dns_zone_name is the dns zone to create if use_designate is true
  dns_zone_name                           = "${local.cluster_name}.demo.mk8s.net"
  # DNS servers to configure on the nodes
  dns_nameservers                         = ["8.8.8.8", "8.8.4.4"]
  image_id                                = "56277265-8f0c-40dc-87e2-944b7d320dae"
  image_id_windows                        = "899af84f-d98f-4255-bf98-ceba5e3a8257"
  k8s_api_port                            = 443
  worker_count                            = 2
  worker_count_windows                    = 0
  # Enter 1 or 3 masters.
  master_count                            = 3
  ssh_user                                = "ubuntu"
  # these are the ssh public keys that will be able to connect to the cluster's bastion node
  ssh_authorized_keys                     = ["ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDogzEullM89m//Vd8IGPERto2DotXnUCKGH6II1Vk/klEuDVqXx9kCb981XJKh8mU15bfJVdE4h078q/shK9EIcPMRKSQSMs2LkgF/1yUeVYPNYiIBph6CaqjIxKHy1kYxw3KUTIh8IIl1M4t5fc5c49Gr3QuDpeMN4Z/wrbR1DceIbFDiVxYNeyJWfOdowKgTn4AKh0n1xtg6/XLin3cCstpvfUJUKm0WOcmn3+DHK6cBNqNAMKdtxgnGwlY4MfizJOZE30Y7hwPqXUjOgLgB2vybcdcMpUvw9e8HopogOFQnVwwmlc9/7ZKPCaCKRBEC38IV82CJ6+/eePIMriPF migu4903@MNF0TUDV30"]
  node_worker                             = "-wn"
  node_master                             = "-cp"
  node_worker_windows                     = "-win"
  ub_version                              = "24"
  #FLEX Flavor Settings ==========================
  flavor_bastion                          = "gp.0.2.2"
  flavor_master                           = "gp.0.4.4"
  flavor_worker                           = "gp.0.4.8"
  # flavor_worker_windows                   = "gp.0.8.16"

  worker_node_bfv_volume_size             = 100  
  worker_node_bfv_destination_type        = "volume"
  worker_node_bfv_source_type             = "image"
  worker_node_bfv_volume_type             = "HA-Standard"

  # ====================================
  #ca_certificates add CA certificates to server's trusts. Good for trusting internal private Certificate Authorities.
  ca_certificates                         = ""
  openstack_ca                            = ""

  # ==================================== 
  #Kubespray Settings
  kubespray_version                       = "v2.28.1"
  kubernetes_version                      = "1.30.4"
  network_plugin                          = "calico"
  deploy_cluster                          = true
  #kub-vip settings
  kube_vip_enabled                        = true
  #Hardening
  k8s_hardening_enabled                   = true
  os_hardening_enabled                    = true
  kube_pod_security_exemptions_namespaces = [
    "kube-*",
    "calico-system",
    "calico-apiserver",
    "tigera-operator",
    "falco",
    "cilium",
    "metallb-system",
  ]
  #Certificate Handling
  kubelet_rotate_server_certificates      = true
  #OpenID Connect - OIDC Settings
  kube_oidc_auth_enabled                  = false
  kube_oidc_url                           = ""
  kube_oidc_client_id                     = ""
  kube_oidc_ca_file                       = "/etc/ssl/certs/ca-certificates.crt"
  kube_oidc_username_claim                = "email"
  kube_oidc_username_prefix               = "oidc:"
  kube_oidc_groups_claim                  = "groups"
  kube_oidc_groups_prefix                 = "oidc:"

  # cni_iface is used by calico/kubespray
  cni_iface   = "ens3"
  # calico settings defines if we are autodetecting the interface or providing it
  calico_interface_autodetect      = false
  calico_encapsulation_type        = "VXLANCrossSubnet"
  calico_nat_outgoing              = true
  calico_interface_autodetect_cidr = ""
}

module "openstack-nova" {
  source                                  = "github.com/rackerlabs/openCenter-gitops-base.git//install/iac/terraform-openstack?ref=main"
  # source                                  = "../../../install/iac/terraform-openstack"
  # output
  naming_prefix                = local.naming_prefix
  openstack_auth_url           = local.openstack_auth_url
  openstack_insecure           = local.openstack_insecure
  openstack_region             = local.openstack_region
  user_name                    = local.openstack_user_name
  user_password                = local.openstack_user_password
  admin_password               = local.openstack_admin_password
  project_domain_name          = local.openstack_project_domain_name
  user_domain_name             = local.openstack_user_domain_name
  tenant_name                  = local.openstack_tenant_name
  availability_zone            = local.availability_zone
  floatingip_pool              = local.floatingip_pool
  router_external_network_id   = local.router_external_network_id
  #OpenStack Networking Settings
  network_provider             = local.network_provider
  vlan_id                      = local.vlan_id
  mtu                          = local.mtu
  #Openstack Network Subnets and dhcp config
  dns_nameservers              = local.dns_nameservers
  #Let OpenStack handle router and dhcp
  dhcp_enabled                 = true
  router_enable_nat            = true
  #K8s cluster settings
  cluster_name                 = local.cluster_name
  deploy_cluster               = local.deploy_cluster
  ssh_user                     = local.ssh_user
  ssh_au
  thorized_keys           = local.ssh_authorized_keys
  subnet_nodes                  = local.subnet_nodes
  subnet_services               = local.subnet_services
  subnet_pods                   = local.subnet_pods
  allocation_pool_start         = local.allocation_pool_start
  allocation_pool_end           = local.allocation_pool_end
  k8s_api_port                  = local.k8s_api_port
  size_master = {
    count  = local.master_count
    flavor = local.flavor_master
  }
  size_worker = {
    count  = local.worker_count
    flavor = local.flavor_worker
  }
  # size_worker_windows = {
  #   count  = local.worker_count_windows
  #   flavor = local.flavor_worker_windows
  # }
  node_master                  = local.node_master
  node_worker                  = local.node_worker
  node_worker_windows          = local.node_worker_windows
  ub_version                   = local.ub_version
  # windows_admin_password       = local.windows_admin_password
  # worker_node_bfv_size_windows = local.worker_node_bfv_size_windows
  # worker_node_bfv_type_windows = local.worker_node_bfv_type_windows
  # master_node_bfv_volume_size = 39
  # master_node_bfv_destination_type = "local"

  worker_node_bfv_volume_size = local.worker_node_bfv_volume_size  
  worker_node_bfv_destination_type = local.worker_node_bfv_destination_type
  worker_node_bfv_source_type = local.worker_node_bfv_source_type
  worker_node_bfv_volume_type = local.worker_node_bfv_volume_type
}

module "kubespray-cluster" {
  # source = "git@github.com:rackerlabs/terraform-mpk-kubespray.git/calico?ref=main"
  source = "github.com/rackerlabs/openCenter-gitops-base.git//install/iac/kubespray?ref=main"
  # source                                  = "../../../install/iac/kubespray"
  address_bastion                         = module.openstack-nova.bastion_floating_ip
  cluster_name                            = local.cluster_name
  cni_iface                               = local.cni_iface
  deploy_cluster                          = local.deploy_cluster
  dns_zone_name                           = local.dns_zone_name
  master_nodes                            = module.openstack-nova.master_nodes
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
  worker_nodes                            = module.openstack-nova.worker_nodes
  k8s_api_ip                              = module.openstack-nova.k8s_api_ip
  k8s_api_port                            = local.k8s_api_port
  vrrp_ip                                 = local.vrrp_ip
  vrrp_enabled                            = local.vrrp_enabled
  windows_nodes                           = module.openstack-nova.windows_nodes
  use_octavia                             = local.use_octavia
  # kube_oidc_auth_enabled                  = local.kube_oidc_auth_enabled
  # kube_oidc_url                           = local.kube_oidc_url
  # kube_oidc_client_id                     = local.kube_oidc_client_id
  # kube_oidc_ca_file                       = local.kube_oidc_ca_file
  # kube_oidc_username_claim                = local.kube_oidc_username_claim
  # kube_oidc_username_prefix               = local.kube_oidc_username_prefix
  # kube_oidc_groups_claim                  = local.kube_oidc_groups_claim
  # kube_oidc_groups_prefix                 = local.kube_oidc_groups_prefix
}


module "calico" {
  source = "github.com/rackerlabs/openCenter-gitops-base.git//install/iac/calico?ref=main"

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
  windows_dataplane                = length(module.openstack-nova.windows_nodes) > 0 ? "HSN" : "Disabled"
}

