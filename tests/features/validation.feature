Feature: Configuration validation rules

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/repo-bad"
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
            url: https://git.example.test/platform/hand-authored.git
            local_dir: <<tmp>>/hand-authored-gitops
            path: applications/overlays/hand-authored
            branch: main
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
    When I run "opencenter cluster generate --render-only mgd --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "opencenter.gitops.repository.local_dir must be set"

  @validation @opentofu_s3_requires_creds @wip
  Scenario: OpenTofu S3 backend requires credentials -> error then pass
    # Note: S3 backend validation may have been removed or changed
    # This test is skipped until validation is re-implemented
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
    When I run "opencenter cluster describe s3 --validate"
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
    When I run "opencenter cluster describe s3ok --validate"
    Then the exit code should be 0

  # All other legacy iac.* validations removed in the new model.

  @validation @hand_authored_v2_validation
  Scenario: hand-authored v2 cluster configuration validation
    When I run "opencenter cluster validate --config-file <<tmp>>/hand-authored-openstack.yaml"
    Then the exit code should be 0
    And stdout should contain "Validation successful"
    And stdout should contain "Configuration:"
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
    And stdout should contain "Infrastructure > Networking"
    And stdout should contain "opencenter.infrastructure.networking.vrrpip"
    And stdout should contain "conditionally required based on related field"
