locals {
  ssh_key_path     = var.ssh_key_path == "" ? "${path.cwd}/id_rsa" : var.ssh_key_path
  os_hardening_resource = var.os_hardening_enabled == true ? null_resource.os_hardening : null
}


resource "local_file" "ansible_inventory" {

  content = templatefile("${path.module}/hosts.tpl",
    {
      address_bastion    = var.address_bastion
      cluster_name       = var.cluster_name
      dns_zone_name      = var.dns_zone_name
      k8s_api_ip         = var.k8s_api_ip
      k8s_internal_ip   = var.k8s_internal_ip
      kubernetes_version = var.kubernetes_version
      master_nodes       = var.master_nodes
      network_plugin     = var.network_plugin
      ssh_key_path       = local.ssh_key_path
      ssh_user           = var.ssh_user
      worker_nodes       = var.worker_nodes
      windows_nodes      = var.windows_nodes
  })

  filename   = "./inventory/inventory.yaml"
  depends_on = [var.master_nodes, var.worker_nodes]
  #   lifecycle {
  #     replace_triggered_by = [var.master_nodes, var.worker_nodes]
  #   }
}

resource "local_file" "k8s_cluster" {
  content = templatefile("${path.module}/templates/k8s_cluster.tpl",
    {
      k8s_api_ip                = var.k8s_api_ip
      k8s_api_port              = var.k8s_api_port
      kubernetes_version        = var.kubernetes_version
      network_plugin            = var.network_plugin
      subnet_pods               = var.subnet_pods
      subnet_services           = var.subnet_services
      enable_nodelocaldns       = var.enable_nodelocaldns
      vrrp_ip                   = var.vrrp_ip
      vrrp_enabled              = var.vrrp_enabled
      use_octavia               = var.use_octavia
      kube_oidc_auth_enabled    = var.kube_oidc_auth_enabled
      kube_oidc_url             = var.kube_oidc_url
      kube_oidc_client_id       = var.kube_oidc_client_id
      kube_oidc_ca_file         = var.kube_oidc_ca_file == "" ? "{{ kube_cert_dir }}/ca.pem" : ""
      kube_oidc_username_claim  = var.kube_oidc_username_claim
      kube_oidc_username_prefix = var.kube_oidc_username_prefix
      kube_oidc_groups_claim    = var.kube_oidc_groups_claim
      kube_oidc_groups_prefix   = var.kube_oidc_groups_prefix
  })

  filename   = "./inventory/group_vars/k8s_cluster/k8s-cluster.yml"
  depends_on = [local_file.ansible_inventory]
}

resource "local_file" "addons" {
  content = templatefile("${path.module}/templates/addons.tpl",
    {
      cert_manager_enabled   = var.cert_manager_enabled
      cni_iface              = var.cni_iface
      k8s_api_ip             = var.k8s_api_ip
      k8s_api_port           = var.k8s_api_port
      kube_vip_enabled       = var.kube_vip_enabled
      metrics_server_enabled = var.metrics_server_enabled
      vrrp_ip                = var.vrrp_ip
  })

  filename   = "./inventory/group_vars/k8s_cluster/addons.yml"
  depends_on = [local_file.ansible_inventory]
}

resource "local_file" "k8s_hardening" {
  count = var.k8s_hardening_enabled ? 1 : 0
  content = templatefile("${path.module}/templates/hardening.tpl",
    {
      cluster_name                            = var.cluster_name
      cni_iface                               = var.cni_iface
      k8s_api_ip                              = var.k8s_api_ip
      vrrp_ip                                 = var.vrrp_ip
      k8s_api_port                            = var.k8s_api_port
      kube_vip_enabled                        = var.kube_vip_enabled
      network_plugin                          = var.network_plugin
      subnet_join                             = var.subnet_join
      kube_pod_security_exemptions_namespaces = var.kube_pod_security_exemptions_namespaces
      kubelet_rotate_server_certificates      = var.kubelet_rotate_server_certificates
  })

  filename   = "./inventory/k8s_hardening.yml"
  depends_on = [local_file.ansible_inventory]
}

resource "null_resource" "clone_kubespray" {
  count = var.deploy_cluster ? 1 : 0
  provisioner "local-exec" {
    command = <<EOT
      if [ ! -d "./kubespray" ]; then
        git clone https://github.com/kubernetes-sigs/kubespray.git ./kubespray
        
      else
        echo "Directory ./kubespray already exists. Skipping clone."
      fi
      cd ./kubespray && git checkout ${var.kubespray_version}
    EOT
  }
}

resource "null_resource" "setup_kubespray_venv" {
  count      = var.deploy_cluster ? 1 : 0
  depends_on = [null_resource.clone_kubespray]

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    command = <<-EOT
      #!/bin/bash
      set -e

      # Find Python executable (try python3 first, then python)
      PYTHON_CMD=""
      if command -v python3 &> /dev/null; then
        PYTHON_CMD="python3"
      elif command -v python &> /dev/null; then
        # Verify it's Python 3
        if python --version 2>&1 | grep -q "Python 3"; then
          PYTHON_CMD="python"
        else
          echo "Error: Python 3 is required but not found"
          exit 1
        fi
      else
        echo "Error: Python 3 is required but not found"
        exit 1
      fi

      echo "Using Python command: $PYTHON_CMD"

      # Create virtual environment if it doesn't exist
      if [ ! -d "venv" ]; then
        echo "Creating Python virtual environment..."
        $PYTHON_CMD -m venv venv
      else
        echo "Virtual environment already exists. Skipping creation."
      fi

      # Activate virtual environment and install requirements
      echo "Activating virtual environment and installing requirements..."
      source venv/bin/activate
      
      # Upgrade pip first
      pip install --upgrade pip
      
      # Install kubespray requirements
      pip install -r kubespray/requirements.txt
      
      echo "Kubespray virtual environment setup complete."
    EOT
  }

}


resource "null_resource" "wait_cloudinit" {
  depends_on = [local_file.ansible_inventory, null_resource.setup_kubespray_venv]

  provisioner "local-exec" {
    environment = {
      ANSIBLE_INVENTORY         = "${path.cwd}/inventory/inventory.yaml"
      ANSIBLE_HOST_KEY_CHECKING = "False"

    }
    interpreter = ["bash", "-c"]
    command     = <<-EOT
      #!/bin/bash
      MAX_RETRIES=60
      SLEEP_INTERVAL=10

      source venv/bin/activate

      if [[ "${var.baremetal_deployment}" == "false" ]]; then
      for i in $(seq 1 $MAX_RETRIES); do
          echo "[$(date)] Checking cloud-init status on all nodes (attempt $i)..."
          
          ansible k8s_cluster -m shell -a 'cloud-init status --wait' -b

          if [ $? -eq 0 ]; then
              echo "All nodes have completed cloud-init."
              exit 0
          fi

          echo " Some nodes are still running cloud-init. Retrying in $SLEEP_INTERVALs..."
          sleep "$SLEEP_INTERVAL"
      done

      echo " Timed out waiting for cloud-init to complete on all nodes after $MAX_RETRIES attempts."
      exit 1
      fi

    EOT
  }

  triggers = {
    inventory = local_file.ansible_inventory.content
  }
}

resource "local_file" "os_hardening_playbook" {
  count = var.os_hardening_enabled == true ? 1 : 0
  content = templatefile("${path.module}/templates/os_hardening_playbook.tpl",
  {})

  filename = "./inventory/os_hardening_playbook.yml"
}

resource "null_resource" "clone_ansible_hardening" {
  count      = var.os_hardening_enabled == true ? 1 : 0

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]

    command = <<-EOT
      #!/bin/bash

      if [ ! -d "./inventory/roles/ansible-hardening" ]; then
      git clone https://opendev.org/openstack/ansible-hardening ./inventory/roles/ansible-hardening
      echo "Running OS hardening playbook"
      fi
      cd ./inventory/roles/ansible-hardening && git checkout ${var.ansible_hardening_version}
    EOT
  }
}

resource "null_resource" "os_hardening" {
  count      = var.os_hardening_enabled == true ? 1 : 0
  depends_on = [null_resource.wait_cloudinit, local_file.os_hardening_playbook,null_resource.setup_kubespray_venv]

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    environment = {
      ANSIBLE_INVENTORY         = "${path.cwd}/inventory/inventory.yaml"
      ANSIBLE_HOST_KEY_CHECKING = "False"
      ANSIBLE_ROLES_PATH        = "${path.cwd}/inventory/roles"
    }

    command = <<-EOT
      #!/bin/bash

      source venv/bin/activate
      echo "Running OS hardening playbook with inventory: $ANSIBLE_INVENTORY"
      ansible-playbook ./inventory/os_hardening_playbook.yml -f 10 -b --become-user=root
    EOT
  }
}

resource "null_resource" "run_kubespray" {
  count      = var.deploy_cluster ? 1 : 0
  depends_on = [null_resource.wait_cloudinit, local.os_hardening_resource, null_resource.clone_kubespray,null_resource.setup_kubespray_venv]

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    environment = {
      ANSIBLE_INVENTORY         = "${path.cwd}/inventory/inventory.yaml"
      ANSIBLE_HOST_KEY_CHECKING = "False"
      ANSIBLE_ROLES_PATH        = "${path.cwd}/kubespray/roles"
    }

    command = <<-EOT
      #!/bin/bash

      source venv/bin/activate

      cd ./kubespray
      # Activate the virtual environment

      

      echo "Running kubespray with inventory: $ANSIBLE_INVENTORY"
      ansible -m shell -a 'hostnamectl set-hostname {{ inventory_hostname }}' --become all
      ansible -m shell -a "grep 127.0.0.1 /etc/hosts | grep -q {{ inventory_hostname }} || sed -i 's/^127.0.0.1.*/127.0.0.1 {{ inventory_hostname }} localhost.localdomain localhost/' /etc/hosts" --become all

      if [[ "${var.k8s_hardening_enabled}" == "true" ]]; then
        ansible-playbook -i $ANSIBLE_INVENTORY cluster.yml -f 10 -b --become-user=root -e "@../inventory/k8s_hardening.yml"
      else
        ansible-playbook -i $ANSIBLE_INVENTORY cluster.yml -f 10 -b --become-user=root
      fi
    EOT
  }

  # triggers = {
  #   inventory = local_file.ansible_inventory.content
  # }
}

resource "null_resource" "copy_and_update_kubeconfig" {
  depends_on = [null_resource.wait_cloudinit, null_resource.run_kubespray[0]]

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    environment = {
      ANSIBLE_INVENTORY         = "${path.cwd}/inventory/inventory.yaml"
      ANSIBLE_HOST_KEY_CHECKING = "False"
    }

    command = <<-EOT
      #!/bin/bash
      source venv/bin/activate
      echo "=== Step 1: Copy /etc/kubernetes/admin.conf from remote server ==="
      ansible kube_control_plane[0] \
        -b \
        -m fetch \
        -a "src=/etc/kubernetes/admin.conf dest=./kubeconfig.yaml flat=true"

      echo ""
      echo "=== Step 2: Update server endpoint in kubeconfig ==="
      ansible localhost \
        -c local \
        -m replace \
        -a "path=./kubeconfig.yaml regexp='server: https://.*:[0-9]*' replace='server: https://${var.k8s_api_ip}:${var.k8s_api_port}' backup=true"
    EOT
  }

  # triggers = {
  #   inventory = local_file.ansible_inventory.content
  # }
}


# provider "helm" {
#   kubernetes = {
#     config_path = "${path.cwd}/kubeconfig.yaml"
#   }
# }

# provider "kubernetes" {
#   config_path = "${path.cwd}/kubeconfig.yaml"
# }

# # This module sets up Kube-OVN as the network plugin for a Kubernetes cluster.
# module "kube_ovn" {
#   source = "./lib/kube-ovn"
#   count = var.network_plugin == "kube-ovn" ? 1 : 0
#   depends_on = [null_resource.copy_and_update_kubeconfig]
#   cni_iface = var.cni_iface
#   deploy_cluster = var.deploy_cluster
#   master_nodes = var.master_nodes
#   worker_nodes = var.worker_nodes
#   subnet_pods = var.subnet_pods
#   subnet_services = var.subnet_services
#   subnet_join = var.subnet_join
# }

