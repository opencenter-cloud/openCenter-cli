# The CLI lifecycle contract is opt-in. Legacy mode keeps these outputs
# compatible with Kubespray modules that predate the contract.
output "opencenter_kubespray_inventory_path" {
  value = try(module.kubespray-cluster.inventory_path, null)
}

output "opencenter_kubespray_lifecycle_contract_version" {
  value = try(module.kubespray-cluster.lifecycle_contract_version, null)

  # Accept the contract range the CLI supports. v2 added inventory_output_dir;
  # v3 adds opencenter_network_id/opencenter_subnet_id. Rejecting v2 here would
  # break `tofu plan` for every cluster still pinned at a v2 module ref, which
  # deploy handles by leaving the network-binding path inert.
  precondition {
    condition = var.opencenter_lifecycle_mode == "legacy" || contains(
      ["2", "3"],
      try(tostring(module.kubespray-cluster.lifecycle_contract_version), ""),
    )
    error_message = "CLI lifecycle mode requires Kubespray lifecycle contract version 2 or 3 (inventory_output_dir support). Regenerate the infrastructure tree with 'opencenter cluster generate' and update the kubespray module source ref."
  }
}

output "opencenter_kubespray_api_address" {
  value = try(module.kubespray-cluster.k8s_api_address, null)
}

output "opencenter_kubespray_api_port" {
  value = try(module.kubespray-cluster.k8s_api_port, null)
}

# Contract v3: the cluster network/subnet identity as it exists after apply.
# openstack-nova creates the network only when vlan_id is empty and the subnet
# only when subnet_id was not supplied, so the module echoes back the effective
# value in both the created and the operator-selected case. `cluster observe`
# promotes these into opencenter.infrastructure.cloud.openstack.networking so
# the CCM [LoadBalancer] subnet-id stops being blank on a tofu-managed network.
# Only main-default.tf.tpl declares module "openstack-nova", so the guard is an
# exact provider match: baremetal and vmware would fail on an unknown module.
{{- if eq (.OpenCenter.Infrastructure.Provider | default "openstack") "openstack" }}
output "opencenter_network_id" {
  value = try(module.openstack-nova.network_id, null)
}

output "opencenter_subnet_id" {
  value = try(module.openstack-nova.subnet_id, null)
}
{{- end }}
