Feature: Configuration validation rules

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/repo-bad"

  @validation @missing_git_dir
  Scenario: missing opencenter.gitops.git_dir -> error
    Given a file "<<tmp>>/conf/mgd.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: mgd
        gitops:
          git_dir: ""
      """
    When I run "openCenter cluster info mgd --validate"
    Then the exit code should not be 0
    And stderr should contain "opencenter.gitops.git_dir must be set"

  @validation @opentofu_s3_requires_creds
  Scenario: OpenTofu S3 backend requires credentials -> error then pass
    Given a file "<<tmp>>/conf/s3.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: s3
        gitops:
          git_dir: "<<tmp>>/repo-bad"
      opentofu:
        enabled: true
        backend:
          type: s3
          s3:
            bucket: b
            key: k
            region: us-east-1
      """
    When I run "openCenter cluster info s3 --validate"
    Then the exit code should not be 0
    And stderr should contain "opencenter.cluster.aws_access_key"
    And stderr should contain "opencenter.cluster.aws_secret_access_key"

  @validation @s3_with_creds_ok
  Scenario: OpenTofu S3 backend with credentials -> ok
    Given a file "<<tmp>>/conf/s3ok.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: s3ok
          aws_access_key: AKIA...
          aws_secret_access_key: secret
        gitops:
          git_dir: "<<tmp>>/repo-bad"
      opentofu:
        enabled: true
        backend:
          type: s3
          s3:
            bucket: b
            key: k
            region: us-east-1
      """
    When I run "openCenter cluster info s3ok --validate"
    Then the exit code should be 0

  # All other legacy iac.* validations removed in the new model.

  @validation @prosys_cluster_validation
  Scenario: prosys.dev.dfw3 cluster configuration validation
    Given a file "<<tmp>>/conf/prosys.dev.dfw3.yaml" with content:
      """
      opencenter:
          cluster:
              aws_access_key: ""
              aws_secret_access_key: ""
              cluster_name: prosys.dev.dfw3
              k8s_api_port_acl:
                  - "0.0.0.0/0"
              kubernetes:
                  dns_zone_name: "dev.attcontroller.com"
                  flavor_bastion: "gp.5.2.2"
                  flavor_master: "gp.5.4.8"
                  flavor_worker: "gp.5.4.8"
                  loadbalancer_provider: "amphora"
                  master_count: 3
                  network_plugin:
                      calico:
                          calico_interface_autodetect: "interface"
                          cni_iface: "enp3s0"
                          enabled: true
                      cilium:
                          enabled: false
                          kubeProxyReplacement: true
                          operator_enabled: true
                      kube-ovn:
                          cilium_integration: true
                          enabled: false
                  oidc:
                      enabled: false
                      kube_oidc_ca_file: ""
                      kube_oidc_client_id: "kubernetes"
                      kube_oidc_groups_claim: "groups"
                      kube_oidc_groups_prefix: 'oidc:'
                      kube_oidc_url: ""
                      kube_oidc_username_claim: "sub"
                      kube_oidc_username_prefix: 'oidc:'
                  subnet_pods: "10.42.0.0/16"
                  subnet_services: "10.43.0.0/16"
                  version: "1.32.8"
                  windows_workers:
                      enabled: false
                      windows_admin_password: ""
                      windows_user: "Administrator"
                      worker_node_bfv_size_windows: 0
                      worker_node_bfv_type_windows: "local"
                  worker_count: 4
                  worker_count_windows: 0
              ssh_authorized_keys:
                  - "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDogzEullM89m//Vd8IGPERto2DotXnUCKGH6II1Vk/klEuDVqXx9kCb981XJKh8mU15bfJVdE4h078q/shK9EIcPMRKSQSMs2LkgF/1yUeVYPNYiIBph6CaqjIxKHy1kYxw3KUTIh8IIl1M4t5fc5c49Gr3QuDpeMN4Z/wrbR1DceIbFDiVxYNeyJWfOdowKgTn4AKh0n1xtg6/XLin3cCstpvfUJUKm0WOcmn3+DHK6cBNqNAMKdtxgnGwlY4MfizJOZE30Y7hwPqXUjOgLgB2vybcdcMpUvw9e8HopogOFQnVwwmlc9/7ZKPCaCKRBEC38IV82CJ6+/eePIMriPF migu4903@MNF0TUDV30"
          gitops:
              flux:
                  interval: 15m
                  prune: true
              git_branch: main
              git_dir: <<tmp>>/prosys-gitops-repo
              git_ssh_key: ~/.ssh/id_ed25519-flux
              git_ssh_pub: ~/.ssh/id_ed25519-flux.pub
              git_url: ""
          infrastructure:
              cloud:
                  aws:
                      private_subnets: []
                      profile: ""
                      public_subnets: []
                      region: ""
                      vpc_id: ""
                  openstack:
                      application_credential_id: ""
                      application_credential_secret: ""
                      auth_url: "https://keystone.api.dfw3.rackspacecloud.com/v3/"
                      insecure: false
                      region: "DFW3"
              provider: openstack
          managed-service:
              alert-proxy:
                  enabled: true
          services:
              alert-manager:
                  enabled: true
              calico:
                  enabled: true
              cert-manager:
                  email: mpk-support@rackspace.com
                  enabled: true
              etcd-backup:
                  enabled: true
              gateway:
                  enabled: true
              gateway-api:
                  enabled: true
              headlamp:
                  enabled: true
              keycloak:
                  enabled: true
              kube-prometheus-stack:
                  enabled: true
              olm:
                  enabled: true
              openstack-ccm:
                  enabled: true
              openstack-csi:
                  enabled: true
              postgres-operator:
                  enabled: true
              sources:
                  enabled: true
              velero:
                  enabled: true
              weave-gitops:
                  enabled: false
      opentofu:
          backend:
              local:
                  path: terraform.tfstate
              s3:
                  bucket: ""
                  key: ""
                  region: ""
              type: local
          enabled: true
          path: opentofu
      secrets:
          sops_age_key_file: <<tmp>>/sops/age/keys/prosys-dev-dfw3-key.txt
      iac:
          main:
              cluster_name: "prosys.dev.dfw3"
              naming_prefix: "${local.cluster_name}-"
              openstack_auth_url: "https://keystone.api.dfw3.rackspacecloud.com/v3/"
              openstack_insecure: false
              openstack_region: "DFW3"
              availability_zone: "az1"
              openstack_user_name: ""
              openstack_user_password: ""
              application_credential_id: "var.os_application_credential_id"
              application_credential_secret: "var.os_application_credential_secret"
              openstack_project_domain_name: "rackspace_cloud_domain"
              openstack_user_domain_name: "rackspace_cloud_domain"
              openstack_tenant_name: "33d34083-ef71-464f-9d09-4b545f64baaf"
              floatingip_pool: "PUBLICNET"
              router_external_network_id: "82be3711-cd97-4f7c-8bbd-59f5524a949e"
              vlan_id: ""
              mtu: ""
              network_provider: "physnet1"
              subnet_nodes: "10.0.4.0/22"
              subnet_nodes_oct: 'join(".", slice(split(".", split("/", local.subnet_nodes)[0]), 0, 3))'
              allocation_pool_start: "${local.subnet_nodes_oct}.50"
              allocation_pool_end: "10.0.7.254"
              vrrp_ip: "${local.subnet_nodes_oct}.10"
              subnet_pods: "10.42.0.0/16"
              subnet_services: "10.43.0.0/16"
              use_octavia: false
              loadbalancer_provider: "amphora"
              vrrp_enabled: true
              use_designate: false
              dns_zone_name: "dev.attcontroller.com"
              dns_nameservers: ["1.1.1.1", "8.8.8.8"]
              ntp_servers: ["time.dfw3.rackspace.com", "time2.dfw3.rackspace.com"]
              image_id: "ec458631-309a-4b7d-846c-cd2ccc601137"
              image_id_windows: ""
              k8s_api_port: 443
              k8s_api_port_acl: ["0.0.0.0/0"]
              worker_count: 4
              worker_count_windows: 0
              master_count: 3
              ssh_user: "ubuntu"
              ssh_authorized_keys: ["ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDogzEullM89m//Vd8IGPERto2DotXnUCKGH6II1Vk/klEuDVqXx9kCb981XJKh8mU15bfJVdE4h078q/shK9EIcPMRKSQSMs2LkgF/1yUeVYPNYiIBph6CaqjIxKHy1kYxw3KUTIh8IIl1M4t5fc5c49Gr3QuDpeMN4Z/wrbR1DceIbFDiVxYNeyJWfOdowKgTn4AKh0n1xtg6/XLin3cCstpvfUJUKm0WOcmn3+DHK6cBNqNAMKdtxgnGwlY4MfizJOZE30Y7hwPqXUjOgLgB2vybcdcMpUvw9e8HopogOFQnVwwmlc9/7ZKPCaCKRBEC38IV82CJ6+/eePIMriPF migu4903@MNF0TUDV30"]
              node_worker: "wn"
              node_master: "cp"
              node_worker_windows: "win"
              ub_version: "24"
              flavor_bastion: "gp.5.2.2"
              flavor_master: "gp.5.4.8"
              flavor_worker: "gp.5.4.8"
              worker_node_bfv_volume_size: 100
              worker_node_bfv_destination_type: "volume"
              worker_node_bfv_source_type: "image"
              worker_node_bfv_volume_type: "Performance"
              ca_certificates: ""
              openstack_ca: ""
              kubespray_version: "v2.28.1"
              kubernetes_version: "1.32.8"
              network_plugin: "calico"
              deploy_cluster: true
              kube_vip_enabled: true
              k8s_hardening_enabled: true
              kube_pod_security_exemptions_namespaces: ["trivy-temp"]
              kubelet_rotate_server_certificates: true
              os_hardening_enabled: true
              cni_iface: "enp3s0"
              calico_interface_autodetect: "interface"
              calico_interface_autodetect_cidr: ""
              calico_encapsulation_type: "VXLAN"
              calico_nat_outgoing: true
          modules:
              openstack-nova:
                  source: "github.com/rackerlabs/openCenter-gitops-base.git//iac/cloud/openstack/openstack-nova?ref=main"
                  availability_zone: "local.availability_zone"
                  application_credential_id: "local.application_credential_id"
                  application_credential_secret: "local.application_credential_secret"
                  ca_certificates: "local.ca_certificates"
                  use_octavia: "local.use_octavia"
                  use_designate: "local.use_designate"
                  dns_nameservers: "local.dns_nameservers"
                  dns_zone_name: "local.dns_zone_name"
                  flavor_bastion: "local.flavor_bastion"
                  openstack_auth_url: "local.openstack_auth_url"
                  openstack_ca: "local.openstack_ca"
                  openstack_insecure: "local.openstack_insecure"
                  openstack_region: "local.openstack_region"
                  openstack_tenant_name: "local.openstack_tenant_name"
                  openstack_user_name: "local.openstack_user_name"
                  openstack_password: "local.openstack_user_password"
                  openstack_project_domain_name: "local.openstack_project_domain_name"
                  openstack_user_domain_name: "local.openstack_user_domain_name"
                  naming_prefix: "local.naming_prefix"
                  ntp_servers: "local.ntp_servers"
                  ssh_user: "local.ssh_user"
                  floatingip_pool: "local.floatingip_pool"
                  image_id: "local.image_id"
                  image_id_windows: "local.image_id_windows"
                  router_external_network_id: "local.router_external_network_id"
                  network_id: ""
                  vlan_id: "local.vlan_id"
                  vrrp_enabled: "local.vrrp_enabled"
                  vrrp_ip: "local.vrrp_ip"
                  ssh_authorized_keys: "local.ssh_authorized_keys"
                  subnet_nodes: "local.subnet_nodes"
                  subnet_services: "local.subnet_services"
                  subnet_pods: "local.subnet_pods"
                  allocation_pool_start: "local.allocation_pool_start"
                  allocation_pool_end: "local.allocation_pool_end"
                  k8s_api_port: "local.k8s_api_port"
                  k8s_api_port_acl: "local.k8s_api_port_acl"
                  size_master:
                      count: "local.master_count"
                      flavor: "local.flavor_master"
                  size_worker:
                      count: "local.worker_count"
                      flavor: "local.flavor_worker"
                  node_master: "local.node_master"
                  node_worker: "local.node_worker"
                  node_worker_windows: "local.node_worker_windows"
                  ub_version: "local.ub_version"
                  worker_node_bfv_volume_size: "local.worker_node_bfv_volume_size"
                  worker_node_bfv_destination_type: "local.worker_node_bfv_destination_type"
                  worker_node_bfv_source_type: "local.worker_node_bfv_source_type"
                  worker_node_bfv_volume_type: "local.worker_node_bfv_volume_type"
              kubespray-cluster:
                  source: "github.com/rackerlabs/openCenter-gitops-base.git//iac/provider/kubespray?ref=main"
                  address_bastion: "module.openstack-nova.bastion_floating_ip"
                  cluster_name: "local.cluster_name"
                  cni_iface: "local.cni_iface"
                  deploy_cluster: "local.deploy_cluster"
                  dns_zone_name: "local.dns_zone_name"
                  master_nodes: "module.openstack-nova.master_nodes"
                  network_plugin: "local.network_plugin"
                  k8s_hardening_enabled: "local.k8s_hardening_enabled"
                  os_hardening_enabled: "local.os_hardening_enabled"
                  ssh_user: "local.ssh_user"
                  subnet_nodes: "local.subnet_nodes"
                  subnet_pods: "local.subnet_pods"
                  subnet_services: "local.subnet_services"
                  kubernetes_version: "local.kubernetes_version"
                  kubespray_version: "local.kubespray_version"
                  kube_vip_enabled: "local.kube_vip_enabled"
                  kube_pod_security_exemptions_namespaces: "local.kube_pod_security_exemptions_namespaces"
                  kubelet_rotate_server_certificates: "local.kubelet_rotate_server_certificates"
                  worker_nodes: "module.openstack-nova.worker_nodes"
                  k8s_api_ip: "module.openstack-nova.k8s_api_ip"
                  k8s_api_port: "local.k8s_api_port"
                  k8s_internal_ip: "module.openstack-nova.k8s_internal_ip"
                  vrrp_ip: "local.vrrp_ip"
                  vrrp_enabled: "local.vrrp_enabled"
                  windows_nodes: "module.openstack-nova.windows_nodes"
                  use_octavia: "local.use_octavia"
              calico:
                  source: "github.com/rackerlabs/openCenter-gitops-base.git//iac/cni/calico?ref=main"
                  calico_interface_autodetect: "local.calico_interface_autodetect"
                  calico_encapsulation_type: "local.calico_encapsulation_type"
                  calico_nat_outgoing: "local.calico_nat_outgoing"
                  calico_interface_autodetect_cidr: 'local.calico_interface_autodetect_cidr == "" ? local.subnet_nodes : local.calico_interface_autodetect_cidr'
                  cni_iface: "local.cni_iface"
                  cluster_name: "local.cluster_name"
                  deploy_cluster: "local.deploy_cluster"
                  k8s_internal_ip: "module.openstack-nova.k8s_internal_ip"
                  k8s_api_port: "local.k8s_api_port"
                  subnet_nodes: "local.subnet_nodes"
                  subnet_pods: "local.subnet_pods"
                  subnet_services: "local.subnet_services"
                  windows_dataplane: 'length(module.openstack-nova.windows_nodes) > 0 ? "HSN" : "Disabled"'
      networking:
          use_octavia: false
          vrrp_enabled: true
          vrrp_ip: "10.0.4.10"
      """
    When I run "openCenter cluster validate prosys.dev.dfw3"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @validation @prosys_cluster_debug_config
  Scenario: prosys.dev.dfw3 cluster debug config generation
    Given a file "<<tmp>>/conf/prosys.dev.dfw3.yaml" with content:
      """
      opencenter:
          cluster:
              cluster_name: prosys.dev.dfw3
          gitops:
              git_dir: <<tmp>>/prosys-gitops-repo
      opentofu:
          enabled: true
          backend:
              type: local
              local:
                  path: terraform.tfstate
      secrets:
          sops_age_key_file: <<tmp>>/sops/age/keys/prosys-dev-dfw3-key.txt
      """
    When I run "openCenter cluster validate prosys.dev.dfw3 --generate-debug-config --output-dir <<tmp>>"
    Then the exit code should be 0
    And stdout should contain "Debug config saved to"
    And stdout should contain "Validation successful"
    And a file "<<tmp>>/.openCenter.yaml" should exist

  @validation @prosys_cluster_vrrp_validation
  Scenario: prosys.dev.dfw3 cluster VRRP validation with networking section
    Given a file "<<tmp>>/conf/prosys.dev.dfw3.yaml" with content:
      """
      opencenter:
          cluster:
              cluster_name: prosys.dev.dfw3
          gitops:
              git_dir: <<tmp>>/prosys-gitops-repo
      opentofu:
          enabled: true
          backend:
              type: local
              local:
                  path: terraform.tfstate
      secrets:
          sops_age_key_file: <<tmp>>/sops/age/keys/prosys-dev-dfw3-key.txt
      networking:
          use_octavia: false
          vrrp_enabled: true
          vrrp_ip: "10.0.4.10"
      """
    When I run "openCenter cluster validate prosys.dev.dfw3"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @validation @prosys_cluster_vrrp_missing_ip
  Scenario: prosys.dev.dfw3 cluster VRRP validation fails when IP missing
    Given a file "<<tmp>>/conf/prosys.dev.dfw3.yaml" with content:
      """
      opencenter:
          cluster:
              cluster_name: prosys.dev.dfw3
          gitops:
              git_dir: <<tmp>>/prosys-gitops-repo
      opentofu:
          enabled: true
          backend:
              type: local
              local:
                  path: terraform.tfstate
      secrets:
          sops_age_key_file: <<tmp>>/sops/age/keys/prosys-dev-dfw3-key.txt
      networking:
          use_octavia: false
          vrrp_enabled: true
          vrrp_ip: ""
      """
    When I run "openCenter cluster validate prosys.dev.dfw3"
    Then the exit code should not be 0
    And stderr should contain "vrrp_ip must be set when use_octavia is false"
