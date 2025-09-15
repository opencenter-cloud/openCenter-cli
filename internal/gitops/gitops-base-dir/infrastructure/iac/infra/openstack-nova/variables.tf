variable "additional_ports_master" {
  description = "List of additional ports to create security group rules for custom applications"
  type        = list(string)
  default     = []  # No rules created by default
}

variable "additional_ports_worker" {
  description = "List of additional ports to create security group rules for custom applications"
  type        = list(string)
  default     = []  # No rules created by default
}

variable "ansible_inventory_enabled"{
  type    = bool
  default = false
}

variable "application_credential_id" {
  type    = string
  default = ""
}

variable "application_credential_secret" {
  type    = string
  default = ""
}

variable "audit_max_age" {
  type    = number
  default = 10
}

variable "audit_max_backup" {
  type    = number
  default = 10
}

variable "audit_max_size" {
  type    = number
  default = 100
}

variable "availability_zone" {
  type    = string
  default = "nova"
}

variable "access_key" {
  type    = string
  default = ""
}

variable "ca_certificates" {
  type    = string
  default = ""
}

variable "calico_mtu" {
  type    = number
  default = 0
}

variable "calico_version" {
  type    = string
  default = ""
}

variable "calico_timeout_seconds" {
  type    = number
  default = 1
}

variable "container_name" {
  type    = string
  default = ""
}

variable "create_container" {
  type    = bool
  default = false
}

variable "csi_enabled" {
  type    = bool
  default = false
}

variable "disable_bastion" {
  type = bool
  default = false
}

variable "ntp_servers" {
  type = list(string)
  default = [
    "time.dfw1.rackspace.com",
    "time2.dfw1.rackspace.com",
  ]
}

variable "dns_nameservers" {
  type = list(string)
  default = [
    "8.8.8.8",
    "8.8.4.4",
  ]
}

variable "dns_zone_name" {
  type = string
}

variable "enable_dex" {
  type    = bool
  default = true
}

variable "flavor_bastion" {
  type = string
}

variable "floatingip_pool" {
  type = string
}

variable "kured_helm_repo_url" {
  type    = string
  default = "https://weaveworks.github.io/kured"
}

variable "image_id" {
  type = string
}

variable "image_id_windows" {
  type = string
  default = "899af84f-d98f-4255-bf98-ceba5e3a8257"
}

variable "bastion_image_id" {
  type    = string
  default = ""
}

variable "image_name" {
  type    = string
  default = ""
}

variable "kubernetes_resources" {
  type    = bool
  default = false
}

variable "system_images" {
  type    = string
  default = ""
}

variable "naming_prefix" {
  type = string
}

variable "network_id" {
  type    = string
  default = ""
}

variable "node_cidr_mask_size" {
  type    = string
  default = ""
}

variable "openstack_auth_url" {
  type = string
}

variable "openstack_ca" {
  type = string
}

variable "openstack_insecure" {
  type    = bool
  default = false
}

variable "openstack_password" {
  type = string
}

variable "openstack_region" {
  type    = string
  default = "RegionOne"
}

variable "openstack_tenant_name" {
  type = string
}

variable "openstack_user_name" {
  type = string
}

variable "openstack_project_domain_name" {
  type = string
  default = null
}

variable "openstack_user_domain_name" {
  type = string
  default = null
}

variable "rke_version" {
  default = ""
  type    = string
}

variable "router_external_network_id" {
  type = string
}

variable "router_id" {
  type    = string
  default = ""
}

variable "secret_key" {
  type    = string
  default = ""
}

variable "services_ca_enabled" {
  type    = bool
  default = false
}

variable "services_ca_crt" {
  type    = string
  default = ""
}

variable "services_ca_key" {
  type    = string
  default = ""
}

variable "size_master" {
  type = object({
    count  = number
    flavor = string
  })
}

variable "size_worker" {
  type = object({
    count  = number
    flavor = string
  })
}

variable "size_worker_windows" {
  type = object({
    count  = number
    flavor = string
  })
  default = {
    count = 0
    flavor = "gp.0.8.16"
  }
}

variable "ssh_key_path" {
  type    = string
  default = "~/.ssh/id_rsa"
}

variable "ssh_authorized_keys" {
  type = list(string)
}

variable "swift_endpoint" {
  type    = string
  default = ""
}

variable "subnet_id" {
  type    = string
  default = ""
}

variable "subnet_nodes" {
  type    = string
  default = "10.0.0.0/22"
}

variable "subnet_pods" {
  type    = string
  default = "10.42.0.0/16"
}

variable "subnet_services" {
  type    = string
  default = "10.43.0.0/16"
}

variable "subnet_servers" {
  type    = string
  default = ""
}

variable "ssh_user" {
  type    = string
  default = "ubuntu"
}

variable "pkg_manager_proxy" {
  type    = string
  default = ""
}

variable "docker_registry" {
  type    = string
  default = ""
}

variable "yum_repo" {
  type    = bool
  default = false
}

variable "apt_repo" {
  type    = bool
  default = false
}
variable "docker_ce_gpg_key_url" {
  type    = string
  default = ""
}

variable "docker_ce_repo_base_url" {
  type    = string
  default = ""
}

/*
Setting this variable to false will cause the
`load_balancer` section of the rke config to be empty,
which will prevent Kubernetes from using Octavia
for `LoadBalancer` resources. Therefore, setting this
variable to true should correspond to the user adding an additional
module definition in their `.tf` file for deploying a loadbalancer.
*/
variable "use_octavia" {
  type    = bool
  default = true
}

variable "loadbalancer_provider" {
  type = string
  default = "amphora"
}

variable "pod_security_policy" {
  type    = bool
  default = false
}

/*
extra_env allows providing environment variables to kubelet and kube-controller.
*/
variable "extra_env" {
  type    = list(string)
  default = []
}

/*
kubelet_extra_binds allows providing additional bind mounts to kubelet.
*/
variable "kubelet_extra_binds" {
  type    = list(string)
  default = []
}

variable "k8s_api_port" {
  type = number
  default = 443
}

variable "pf9_onboard" {
  type    = bool
  default = false
}

variable "pf9ctl_setup_download_url" {
  type    = string
  default = "https://pmkft-assets.s3-us-west-1.amazonaws.com"
}

variable "pf9_account_url" {
  type    = string
  default = ""
}

variable "pf9_username" {
  type    = string
  default = ""
}

variable "pf9_password" {
  type      = string
  default   = ""
  sensitive = true
}

variable "pf9_tenant" {
  type    = string
  default = "service"
}

variable "pf9_region" {
  type    = string
  default = "RegionOne"
}

variable "pf9_use_hostname" {
  type    = bool
  default = false
}

variable "use_designate" {
  type = bool
}

variable "vrrp_ip" {
  type = string
}


variable "allocation_pool_start" {
  type    = string
  default = null
}

variable "allocation_pool_end" {
  type    = string
  default = null
}

variable "vlan_id" {
  type    = string
  default = ""
}

variable "node_master" {
  type    = string
  default = ""
  description = "define the role to be used in hostname"
}

variable "node_worker" {
  type    = string
  default = ""
  description = "define the role to be used in hostname"
}

variable "node_worker_windows" {
  type    = string
  default = ""
  description = "define the role to be used in hostname"
}

variable "master_node_bfv_source_type" {
  type    = string
  default = "image"
  description = "he source type of the device. Must be one of blank, image, volume, or snapshot. Changing this creates a new server."
}

variable "master_node_bfv_volume_size" {
  type    = number
  default = 0
  description = "boot from volume size for the master nodes"
}

variable "master_node_bfv_destination_type" {
  type   = string
  default = "local"
  description = "boot from volume type for the master nodes"
}

variable "master_node_bfv_delete_on_termination" {
  type    = bool
  default = true
  description = "If true, the volume will be deleted when the server is terminated."
}

variable "master_node_bfv_volume_type" {
  type    = string
  default = "Standard"
  description = "The volume type that will be used, for example SSD or HDD storage. The available types depend on the OpenStack deployment."
}

variable "worker_node_bfv_source_type" {
  type    = string
  default = "image"
  description = "he source type of the device. Must be one of blank, image, volume, or snapshot. Changing this creates a new server."
}

variable "worker_node_bfv_volume_size" {
  type    = number
  default = 0
  description = "boot from volume size for the master nodes"
}

variable "worker_node_bfv_destination_type" {
  type   = string
  default = "local"
  description = "boot from volume type for the master nodes"
}

variable "worker_node_bfv_delete_on_termination" {
  type    = bool
  default = true
  description = "If true, the volume will be deleted when the server is terminated."
}

variable "worker_node_bfv_volume_type" {
  type    = string
  default = "standard"
  description = "The volume type that will be used, for example SSD or HDD storage. The available types depend on the OpenStack deployment."
}


variable "worker_node_bfv_size_windows" {
  type    = number
  default = 100
  description = "boot from volume size for the worker nodes"
}

variable "worker_node_bfv_type_windows" {
  type   = string
  default = "local"
  description = "boot from volume type for the worker nodes"
}

variable "ub_version" {
  type = string
  default = ""
}

variable "rke2_enable" {
  type = bool
  default = false
  description = "value to create and configure the infrastructure for rke2"
}

variable "rke_server_port" {
  type    = number
  default = 9345
  description = "value to set the port for the rke2 join api"
}

variable "reboot" {
  type    = bool
  default = true
  description = "Reboot the node on cloud-init run?"
}

variable "vrrp_enabled" {
  type = bool
  default = true
  description = "Will create a port to use as a VIP. If floating IP pool is defined it will get a floating IP assigned to it."
}

variable "k8s_api_port_acl" {
  type = list(string)
  default = ["0.0.0.0/0"]
  description = "List of CIDR blocks to allow access to to the K8s API Port"
}

variable "windows_user" {
  type    = string
  default = "administrator"
  
}

variable "windows_admin_password" {
  type      = string
  default   = ""
  sensitive = true
  description = "The password for the Windows administrator user."
}


# variable "internal_network" {
#   type    = bool
#   default = false
# }

# variable "internal_network_cidr" {
#   type    = string
#   default = "172.30.0.0/22"
# }

# variable "internal_network_id" {
#   type    = string
#   default = ""
# }
