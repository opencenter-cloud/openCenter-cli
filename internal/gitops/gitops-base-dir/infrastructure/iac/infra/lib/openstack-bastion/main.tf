resource "openstack_networking_secgroup_v2" "bastion" {
  name = "${var.naming_prefix}bastion"
}

resource "openstack_networking_secgroup_rule_v2" "bastion_ipv4" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.bastion.id
}

resource "openstack_networking_secgroup_rule_v2" "bastion_ipv6" {
  direction         = "ingress"
  ethertype         = "IPv6"
  remote_ip_prefix  = "::/0"
  port_range_min    = 22
  port_range_max    = 22
  protocol          = "tcp"
  security_group_id = openstack_networking_secgroup_v2.bastion.id
}

resource "openstack_compute_instance_v2" "bastion" {
  name              = "${var.naming_prefix}bastion"
  config_drive      = true
  flavor_name       = var.flavor_bastion
  image_id          = var.image_id
  image_name        = var.image_name
  user_data         = var.user_data
  availability_zone = var.availability_zone
  key_pair          = var.key_pair.name

  security_groups = tolist([openstack_networking_secgroup_v2.bastion.name, var.security_group_name])
  depends_on      = [var.module_depends_on]

  network {
    uuid = var.network_id
  }

  lifecycle {
    ignore_changes = [
      user_data
    ]
  }
}

resource "openstack_compute_floatingip_v2" "bastion" {
  count = var.floatingip_pool == "" ? 0 : 1
  pool  = var.floatingip_pool
}

resource "openstack_compute_floatingip_associate_v2" "bastion" {
  count       = var.floatingip_pool == "" ? 0 : 1
  floating_ip = openstack_compute_floatingip_v2.bastion[0].address
  instance_id = openstack_compute_instance_v2.bastion.id
}
