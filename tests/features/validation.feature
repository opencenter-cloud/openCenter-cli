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

  # @wip triage (2026-04-26): OpenTofu S3 backend validation was removed or changed.
  # The @opentofu_s3_requires_creds scenario has been deleted. Re-add when S3 backend
  # validation is re-implemented.

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

  # ---------------------------------------------------------------------------
  # Structural validation (from config_template_rendering)
  # ---------------------------------------------------------------------------

  @validation @missing-secrets @priority4
  Scenario: Cert-manager without explicit secrets still passes structural validation
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
        gitops:
          git_dir: <<tmp>>/repo-bad
        services:
          cert-manager:
            enabled: true
      """
    When I run "opencenter cluster validate test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @validation @missing-secrets @priority4
  Scenario: Loki without explicit secrets still passes structural validation
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
        gitops:
          git_dir: <<tmp>>/repo-bad
        services:
          loki:
            enabled: true
            swift_auth_url: https://keystone.example.com/v3/
            swift_username: loki
            swift_project_name: project
            swift_region: REGION
            swift_domain_name: default
      secrets:
        cert_manager:
          aws_access_key: test
          aws_secret_access_key: test
        keycloak:
          admin_password: test
        grafana:
          admin_password: test
        weave_gitops:
          password_hash: test
      """
    When I run "opencenter cluster validate test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @validation @invalid-email @priority4
  Scenario: Invalid admin email should fail validation
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          admin_email: invalid-email-format
        gitops:
          git_dir: <<tmp>>/repo-bad
      secrets:
        cert_manager:
          aws_access_key: test
          aws_secret_access_key: test
        keycloak:
          admin_password: test
        grafana:
          admin_password: test
        weave_gitops:
          password_hash: test
      """
    When I run "opencenter cluster validate test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0

  @validation @invalid-domain @priority4
  Scenario: Invalid cluster FQDN should fail validation
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          cluster_fqdn: invalid domain with spaces
        gitops:
          git_dir: <<tmp>>/repo-bad
      secrets:
        cert_manager:
          aws_access_key: test
          aws_secret_access_key: test
        keycloak:
          admin_password: test
        grafana:
          admin_password: test
        weave_gitops:
          password_hash: test
      """
    When I run "opencenter cluster validate test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0

  @validation @valid-config
  Scenario: Valid configuration should pass validation
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          admin_email: admin@example.com
          cluster_fqdn: test.example.com
          base_domain: example.com
          domain: example.com
        gitops:
          git_dir: <<tmp>>/repo-bad
        infrastructure:
          cloud:
            openstack:
              application_credential_id: "12345678-1234-1234-1234-123456789012"
              application_credential_secret: "test-app-cred-secret"
              auth_url: "https://identity.example.com/v3"
              region: "RegionOne"
              domain: "Default"
              networking:
                floating_network_id: "12345678-1234-1234-1234-123456789012"
          provider: openstack
      secrets:
        cert_manager:
          aws_access_key: AKIATEST123
          aws_secret_access_key: secretkey123
        keycloak:
          admin_password: password123
        grafana:
          admin_password: password123
        weave_gitops:
          password_hash: $2a$10$hash
        global:
          openstack:
            application_credential_id: "12345678-1234-1234-1234-123456789012"
            application_credential_secret: "test-app-cred-secret"
      """
    When I run "opencenter cluster validate test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
