resource "openstack_networking_secgroup_v2" "controlplane" {
  name = "${var.naming_prefix}controlplane"
}

resource "openstack_networking_secgroup_rule_v2" "controlplane_ipv4" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_group_id   = openstack_networking_secgroup_v2.controlplane.id
  security_group_id = openstack_networking_secgroup_v2.controlplane.id
}

resource "openstack_networking_secgroup_rule_v2" "controlplane_ipv4_pods" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = var.subnet_pods
  security_group_id = openstack_networking_secgroup_v2.controlplane.id
}

resource "openstack_networking_secgroup_rule_v2" "controlplane_ipv4_services" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = var.subnet_services
  security_group_id = openstack_networking_secgroup_v2.controlplane.id
}

resource "openstack_networking_secgroup_rule_v2" "controlplane_ipv4_servers" {
  count             = var.subnet_servers != "" ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = var.subnet_servers
  security_group_id = openstack_networking_secgroup_v2.controlplane.id
}

resource "openstack_networking_secgroup_rule_v2" "controlplane_ipv4_workers" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_group_id   = openstack_networking_secgroup_v2.worker.id
  security_group_id = openstack_networking_secgroup_v2.controlplane.id
}

resource "openstack_networking_secgroup_rule_v2" "controlplane_ipv6" {
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_group_id   = openstack_networking_secgroup_v2.controlplane.id
  security_group_id = openstack_networking_secgroup_v2.controlplane.id
}

resource "openstack_networking_secgroup_v2" "master" {
  name = "${var.naming_prefix}master"
}

resource "openstack_networking_secgroup_rule_v2" "kube_api_ipv4" {
  for_each = toset(var.k8s_api_port_acl) 
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = each.value
  port_range_min    = var.k8s_api_port
  port_range_max    = var.k8s_api_port
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.master.id
}

# resource "openstack_networking_secgroup_rule_v2" "master_ipv4" {
#   direction         = "ingress"
#   ethertype         = "IPv4"
#   remote_ip_prefix  = "0.0.0.0/0"
#   port_range_min    = var.k8s_api_port
#   port_range_max    = var.k8s_api_port
#   protocol          = "tcp"
#   security_group_id = openstack_networking_secgroup_v2.master.id
# }

# resource "openstack_networking_secgroup_rule_v2" "master_ipv6" {
#   direction         = "ingress"
#   ethertype         = "IPv6"
#   remote_ip_prefix  = "::/0"
#   port_range_min    = var.k8s_api_port
#   port_range_max    = var.k8s_api_port
#   protocol          = "tcp"
#   security_group_id = openstack_networking_secgroup_v2.master.id
# }

resource "openstack_networking_secgroup_rule_v2" "master_ssh_ipv4" {
  count             = var.disable_bastion == true ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = "0.0.0.0/0"
  port_range_min    = 22
  port_range_max    = 22
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_rule_v2" "master_ssh_ipv6" {
  count             = var.disable_bastion == true ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_ip_prefix  = "::/0"
  port_range_min    = 22
  port_range_max    = 22
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.master.id
}


resource "openstack_networking_secgroup_rule_v2" "vrrp_ipv4" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = "0.0.0.0/0"
  protocol          = 112
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_rule_v2" "vrrp_ipv6" {
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_ip_prefix  = "::/0"
  protocol          = 112
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_rule_v2" "vrrp_ipv4_2" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = "0.0.0.0/0"
  protocol          = 51
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_rule_v2" "vrrp_ipv6_2" {
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_ip_prefix  = "::/0"
  protocol          = 51
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_v2" "worker" {
  name = "${var.naming_prefix}worker"
}

resource "openstack_networking_secgroup_rule_v2" "worker_ipv4" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.worker.id
}

resource "openstack_networking_secgroup_rule_v2" "worker_ipv6" {
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_ip_prefix  = "::/0"
  security_group_id = openstack_networking_secgroup_v2.worker.id
}

# resource "openstack_networking_secgroup_rule_v2" "worker_int_ipv4" {
#   direction         = "ingress"
#   ethertype         = "IPv4"
#   remote_group_id   = openstack_networking_secgroup_v2.controlplane.id
#   security_group_id = openstack_networking_secgroup_v2.worker.id
# }


resource "openstack_networking_secgroup_v2" "worker_windows" {
  count = var.worker_count_windows > 0 ? 1 : 0
  name = "${var.naming_prefix}worker-windows"
}

resource "openstack_networking_secgroup_rule_v2" "worker_windows_nodeport_ipv4" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  port_range_min    = 30000
  port_range_max    = 32767
  remote_ip_prefix  = "0.0.0.0/0"
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.worker_windows[0].id
}

resource "openstack_networking_secgroup_rule_v2" "controlplane_ipv4_windows" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_group_id   = openstack_networking_secgroup_v2.worker_windows[0].id
  security_group_id = openstack_networking_secgroup_v2.controlplane.id
}

resource "openstack_networking_secgroup_rule_v2" "rdp_ipv4" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = "0.0.0.0/0"
  port_range_min    = 3389
  port_range_max    = 3389
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.worker_windows[0].id
}

resource "openstack_networking_secgroup_rule_v2" "rdp_ipv6" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_ip_prefix  = "::/0"
  port_range_min    = 3389
  port_range_max    = 3389
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.worker_windows[0].id
}

resource "openstack_networking_secgroup_rule_v2" "worker_windows_int_ipv4" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_group_id   = openstack_networking_secgroup_v2.worker_windows[0].id
  security_group_id = openstack_networking_secgroup_v2.worker.id
}

resource "openstack_networking_secgroup_rule_v2" "worker_windows_int_ipv6" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_group_id   = openstack_networking_secgroup_v2.worker_windows[0].id
  security_group_id = openstack_networking_secgroup_v2.worker.id
}

resource "openstack_networking_secgroup_rule_v2" "winrm_ipv4" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = var.subnet_servers
  port_range_min    = 5985
  port_range_max    = 5986
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.worker_windows[0].id
}

resource "openstack_networking_secgroup_rule_v2" "ssh_ipv4" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = "0.0.0.0/0"
  port_range_min    = 22
  port_range_max    = 22
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.worker_windows[0].id
}

resource "openstack_networking_secgroup_rule_v2" "ssh_ipv6" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_ip_prefix  = "::/0"
  port_range_min    = 22
  port_range_max    = 22
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.worker_windows[0].id
}

resource "openstack_networking_secgroup_rule_v2" "worker_windows_nodes_ipv4" {
  count             = var.worker_count_windows > 0 ? 1 : 0
  direction         = "ingress"
  ethertype         = "IPv4"
  port_range_min    = 0
  port_range_max    = 0
  remote_ip_prefix  = var.subnet_servers
  security_group_id = openstack_networking_secgroup_v2.worker_windows[0].id
}

resource "openstack_networking_secgroup_rule_v2" "master_additional_ports_ipv4" {
  for_each = toset(var.additional_ports_master) 
  
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = var.subnet_servers
  port_range_min    = each.value
  port_range_max    = each.value
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.master.id
}
resource "openstack_networking_secgroup_rule_v2" "worker_additional_ports_ipv4" {
  for_each = toset(var.additional_ports_worker) 
  
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = var.subnet_servers
  port_range_min    = each.value
  port_range_max    = each.value
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.worker.id
}

