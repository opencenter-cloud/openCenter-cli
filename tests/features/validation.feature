Feature: Configuration validation rules

  Background:
    Given an empty directory "<<tmp>>/conf"
    And a file "<<tmp>>/hand-authored-openstack.yaml" with content:
      """
      schema_version: "2.0"
      opencenter:
        meta:
          name: hand-authored
          organization: example-platform
          env: dev
          region: dfw3
        cluster:
          cluster_name: hand-authored
          base_domain: k8s.example.test
          cluster_fqdn: hand-authored.dfw3.k8s.example.test
          admin_email: admin@example.test
          kubernetes:
            version: 1.33.5
            api_port: 443
            kube_vip_enabled: true
            subnet_pods: 10.42.0.0/16
            subnet_services: 10.43.0.0/16
            network_plugin:
              calico:
                enabled: true
                version: 3.29.2
                vxlan_mode: Always
                network_policy: true
                install_method: helm
        infrastructure:
          provider: openstack
          ssh:
            authorized_keys:
              - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEzYf7ZEXAMPLE0000000000000000000000000000 test@example
            username: ubuntu
            user: ubuntu
            key_path: <<tmp>>/keys/hand-authored
          os_version: "24"
          networking:
            subnet_nodes: 10.2.128.0/22
            allocation_pool_start: 10.2.128.10
            allocation_pool_end: 10.2.131.250
            gateway: 10.2.128.1
            vrrp_ip: 10.2.128.5
            vrrp_enabled: true
            loadbalancer_provider: ovn
            use_designate: false
            dns_zone_name: hand-authored.dfw3.k8s.example.test
            dns_nameservers:
              - 8.8.8.8
            ntp_servers:
              - time.example.test
          compute:
            master_count: 3
            worker_count: 2
            worker_count_windows: 0
          storage:
            default_storage_class: csi-cinder-sc-delete
            worker_volume_size: 40
            worker_volume_destination_type: volume
            worker_volume_source_type: image
            worker_volume_type: local
            worker_volume_delete_on_termination: true
            master_volume_size: 40
            master_volume_destination_type: volume
            master_volume_source_type: image
            master_volume_type: local
            master_volume_delete_on_termination: true
          cloud:
            openstack:
              auth_url: https://identity.example.test/v3/
              region: DFW3
              project_id: 00000000-0000-0000-0000-000000000000
              project_name: example-project
              application_credential_id: 11111111-1111-1111-1111-111111111111
              application_credential_secret: example-secret
              domain: Default
              image_id: 22222222-2222-2222-2222-222222222222
        gitops:
          repository:
            url: ssh://git@git.example.test/platform/hand-authored.git
            local_dir: <<tmp>>/hand-authored-gitops
            path: applications/overlays/hand-authored
            branch: main
          auth:
            ssh:
              private_key: <<tmp>>/keys/hand-authored
              public_key: <<tmp>>/keys/hand-authored.pub
          flux:
            interval: 15m
            prune: true
      deployment:
        method: kubespray
        kubespray:
          version: 2.27.0
      opentofu:
        enabled: true
        backend:
          type: local
          local:
            path: terraform.tfstate
      secrets:
        sops_age_key_file: <<tmp>>/keys/age.txt
        global:
          openstack_auth_url: https://identity.example.test/v3/
      """

  @validation @hand_authored_v2_validation
  Scenario: hand-authored v2 cluster configuration validation
    When I run "opencenter cluster validate --config-file <<tmp>>/hand-authored-openstack.yaml"
    Then the exit code should be 0
    And stdout should contain "Validation successful"
    And stdout should contain "Cluster:"
    And stdout should contain "Provider:"

  @validation @hand_authored_v2_debug_config
  Scenario: hand-authored v2 debug config generation
    When I run "opencenter cluster validate --config-file <<tmp>>/hand-authored-openstack.yaml --generate-debug-config --output-dir <<tmp>>"
    Then the exit code should be 0
    And stdout should contain "Debug config saved to"
    And stdout should contain "Validation successful"
    And a file "<<tmp>>/.opencenter-v2.yaml" should exist

  @validation @hand_authored_v2_vrrp_missing_ip
  Scenario: hand-authored v2 networking validation reports missing VRRP IP
    Given I update the YAML "<<tmp>>/hand-authored-openstack.yaml" to set:
      """
      opencenter:
        infrastructure:
          networking:
            vrrp_ip: ""
      """
    When I run "opencenter cluster validate --config-file <<tmp>>/hand-authored-openstack.yaml"
    Then the exit code should not be 0
    And stdout should contain "VRRPIP"
    And stdout should contain "required_if"

  @validation @missing-secrets @priority4
  Scenario: Cert-manager without explicit secrets still passes validation
    Given a file "<<tmp>>/cert-manager-test.yaml" with content:
      """
      schema_version: "2.0"
      opencenter:
        meta:
          name: cert-test
          organization: test-org
          env: dev
          region: dfw3
        cluster:
          cluster_name: cert-test
          base_domain: k8s.example.test
          cluster_fqdn: cert-test.dfw3.k8s.example.test
          admin_email: admin@example.test
          kubernetes:
            version: 1.33.5
            api_port: 443
            kube_vip_enabled: false
            subnet_pods: 10.42.0.0/16
            subnet_services: 10.43.0.0/16
            network_plugin:
              calico:
                enabled: true
                version: 3.29.2
                vxlan_mode: Always
                network_policy: true
                install_method: helm
        infrastructure:
          provider: openstack
          ssh:
            authorized_keys:
              - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEzYf7ZEXAMPLE0000000000000000000000000000 test@example
            username: ubuntu
            user: ubuntu
            key_path: <<tmp>>/keys/cert-test
          os_version: "24"
          networking:
            subnet_nodes: 10.2.128.0/22
            allocation_pool_start: 10.2.128.10
            allocation_pool_end: 10.2.131.250
            gateway: 10.2.128.1
            loadbalancer_provider: ovn
            use_designate: false
            dns_zone_name: cert-test.dfw3.k8s.example.test
            dns_nameservers:
              - 8.8.8.8
            ntp_servers:
              - time.example.test
          compute:
            master_count: 3
            worker_count: 2
          storage:
            default_storage_class: csi-cinder-sc-delete
            worker_volume_size: 40
            worker_volume_destination_type: volume
            worker_volume_source_type: image
            worker_volume_type: local
            master_volume_size: 40
            master_volume_destination_type: volume
            master_volume_source_type: image
            master_volume_type: local
          cloud:
            openstack:
              auth_url: https://identity.example.test/v3/
              region: DFW3
              project_id: 00000000-0000-0000-0000-000000000000
              project_name: example-project
              application_credential_id: 11111111-1111-1111-1111-111111111111
              application_credential_secret: example-secret
              domain: Default
              image_id: 22222222-2222-2222-2222-222222222222
        gitops:
          repository:
            url: ssh://git@git.example.test/platform/cert-test.git
            local_dir: <<tmp>>/cert-test-gitops
            path: applications/overlays/cert-test
            branch: main
          auth:
            ssh:
              private_key: <<tmp>>/keys/cert-test
              public_key: <<tmp>>/keys/cert-test.pub
          flux:
            interval: 15m
            prune: true
        services:
          cert-manager:
            enabled: true
      deployment:
        method: kubespray
        kubespray:
          version: 2.27.0
      opentofu:
        enabled: true
        backend:
          type: local
          local:
            path: terraform.tfstate
      secrets:
        sops_age_key_file: <<tmp>>/keys/age.txt
        global:
          openstack_auth_url: https://identity.example.test/v3/
      """
    When I run "opencenter cluster validate --config-file <<tmp>>/cert-manager-test.yaml"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @validation @service-secrets @priority4
  Scenario: Loki with required Swift credentials passes validation
    Given a file "<<tmp>>/loki-test.yaml" with content:
      """
      schema_version: "2.0"
      opencenter:
        meta:
          name: loki-test
          organization: test-org
          env: dev
          region: dfw3
        cluster:
          cluster_name: loki-test
          base_domain: k8s.example.test
          cluster_fqdn: loki-test.dfw3.k8s.example.test
          admin_email: admin@example.test
          kubernetes:
            version: 1.33.5
            api_port: 443
            kube_vip_enabled: false
            subnet_pods: 10.42.0.0/16
            subnet_services: 10.43.0.0/16
            network_plugin:
              calico:
                enabled: true
                version: 3.29.2
                vxlan_mode: Always
                network_policy: true
                install_method: helm
        infrastructure:
          provider: openstack
          ssh:
            authorized_keys:
              - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEzYf7ZEXAMPLE0000000000000000000000000000 test@example
            username: ubuntu
            user: ubuntu
            key_path: <<tmp>>/keys/loki-test
          os_version: "24"
          networking:
            subnet_nodes: 10.2.128.0/22
            allocation_pool_start: 10.2.128.10
            allocation_pool_end: 10.2.131.250
            gateway: 10.2.128.1
            loadbalancer_provider: ovn
            use_designate: false
            dns_zone_name: loki-test.dfw3.k8s.example.test
            dns_nameservers:
              - 8.8.8.8
            ntp_servers:
              - time.example.test
          compute:
            master_count: 3
            worker_count: 2
          storage:
            default_storage_class: csi-cinder-sc-delete
            worker_volume_size: 40
            worker_volume_destination_type: volume
            worker_volume_source_type: image
            worker_volume_type: local
            master_volume_size: 40
            master_volume_destination_type: volume
            master_volume_source_type: image
            master_volume_type: local
          cloud:
            openstack:
              auth_url: https://identity.example.test/v3/
              region: DFW3
              project_id: 00000000-0000-0000-0000-000000000000
              project_name: example-project
              application_credential_id: 11111111-1111-1111-1111-111111111111
              application_credential_secret: example-secret
              domain: Default
              image_id: 22222222-2222-2222-2222-222222222222
        gitops:
          repository:
            url: ssh://git@git.example.test/platform/loki-test.git
            local_dir: <<tmp>>/loki-test-gitops
            path: applications/overlays/loki-test
            branch: main
          auth:
            ssh:
              private_key: <<tmp>>/keys/loki-test
              public_key: <<tmp>>/keys/loki-test.pub
          flux:
            interval: 15m
            prune: true
        services:
          loki:
            enabled: true
            swift_auth_url: https://keystone.example.com/v3/
            swift_username: loki
            swift_project_name: project
            swift_region: REGION
            swift_domain_name: default
      deployment:
        method: kubespray
        kubespray:
          version: 2.27.0
      opentofu:
        enabled: true
        backend:
          type: local
          local:
            path: terraform.tfstate
      secrets:
        sops_age_key_file: <<tmp>>/keys/age.txt
        global:
          openstack_auth_url: https://identity.example.test/v3/
        loki:
          swift_application_credential_secret: test-swift-secret
      """
    When I run "opencenter cluster validate --config-file <<tmp>>/loki-test.yaml"
    Then the exit code should be 0
    And stdout should contain "Validation successful"
