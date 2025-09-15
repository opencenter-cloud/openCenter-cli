variable "address_bastion" {
  type    = string
  default = "0.0.0.0"
}

variable "baremetal_deployment" {
  type    = bool
  default = false
}

variable "cert_manager_enabled" {
  type        = bool
  default     = false
  description = "Enable cert-manager for the cluster. This will deploy cert-manager on to the cluster."
}

variable "cluster_name" {
  type    = string
  default = ""
}

variable "cni_iface" {
  type    = string
  default = "enp3s0"
}

variable "deploy_cluster" {
  type    = bool
  default = false
}

variable "dns_zone_name" {
  type        = string
  default     = ""
  description = "The DNS zone name to use for the cluster."

}

variable "enable_nodelocaldns" {
  type        = bool
  default     = false
  description = "Enable nodelocaldns for the cluster. This is useful for clusters with many nodes to reduce DNS query latency."
}

variable "k8s_hardening_enabled" {
  type        = bool
  default     = false
  description = "Enable hardening for the cluster. This will apply settings from https://kubespray.io/#/docs/operations/hardening"
}

variable "host_baseline_repo" {
  type        = string
  default     = ""
  description = "Repository that contains the kubespray base files."
}

variable "host_baseline_path" {
  type        = string
  default     = "sjc-lab"
  description = "Path under the repository that contains the kubespray base files. Typically this is the short name of the lab or environment."
}

variable "k8s_internal_ip" {
  type        = string
  default     = ""
  description = "The internal IP address used as a VIP for the kube-apiserver."
}

variable "k8s_api_ip" {
  type    = string
  default = ""
}

variable "k8s_api_port" {
  type    = number
  default = 6443
}

variable "kubelet_rotate_server_certificates" {
  type    = bool
  default = false
}

variable "kubespray_version" {
  type    = string
  default = "v2.28.0"
}
variable "kubernetes_version" {
  type    = string
  default = "1.30.4"
}

variable "kubeconfig_path" {
  type    = string
  default = "./kubeconfig"
}

variable "kube_oidc_auth_enabled" {
  type    = bool
  default = false
}

variable "kube_oidc_url" {
  type    = string
  default = "https://"
}

variable "kube_oidc_client_id" {
  type    = string
  default = "kubernetes"
}
variable "kube_oidc_ca_file" {
  type    = string
  default = "/etc/kubernetes/ssl/ca.pem"
}
variable "kube_oidc_username_claim" {
  type    = string
  default = "sub"
}
variable "kube_oidc_username_prefix" {
  type    = string
  default = "oidc:"
}
variable "kube_oidc_groups_claim" {
  type    = string
  default = "groups"
}
variable "kube_oidc_groups_prefix" {
  type    = string
  default = "oidc:"
}

variable "kube_vip_enabled" {
  type        = bool
  default     = false
  description = "Enable kube-vip for the cluster. This will deploy kube-vip on to the cluster."
}

variable "kube_pod_security_exemptions_namespaces" {
  type    = list(string)
  default = []
}

variable "master_nodes" {
  type = list(object({
    id           = string
    name         = string
    access_ip_v4 = string
  }))
}

variable "metrics_server_enabled" {
  type        = bool
  default     = true
  description = "Enable metrics server for the cluster. This is useful for monitoring and scaling."

}

variable "network_plugin" {
  type    = string
  default = "none"
}

variable "os_hardening_enabled" {
  type        = bool
  default     = false
  description = "Enable hardening for the operating system. This will apply settings to the OS from https://opendev.org/openstack/ansible-hardening"
}

variable "ansible_hardening_version"  {
  type    = string
  default = "stable/2025.1"
}

variable "ssh_key_path" {
  type    = string
  default = ""
}

variable "ssh_user" {
  type    = string
  default = "ubuntu"
}

variable "subnet_nodes" {
  type    = string
  default = ""
}

variable "subnet_pods" {
  type    = string
  default = "10.42.0.0/16"
}

variable "subnet_services" {
  type    = string
  default = "10.43.0.0/16"
}

variable "subnet_join" {
  type    = string
  default = "100.64.0.0/16"
}

variable "worker_nodes" {
  type = list(object({
    id           = string
    name         = string
    access_ip_v4 = string
  }))
}

variable "vrrp_enabled" {
  type        = bool
  default     = true
  description = "Will create a port to use as a VIP. If floating IP pool is defined it will get a floating IP assigned to it."
}

variable "vrrp_ip" {
  type        = string
  default     = ""
  description = "The IP address used as a VIP for the kube-apiserver."
}

variable "windows_nodes" {
  type = list(object({
    id           = string
    name         = string
    access_ip_v4 = string
  }))
  default = []
}

variable "use_octavia" {
  type    = bool
  default = true
}