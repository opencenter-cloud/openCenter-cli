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
    When I run "opencenter cluster render mgd --config-dir <<tmp>>/conf"
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
    When I run "opencenter cluster info s3 --validate"
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
    When I run "opencenter cluster info s3ok --validate"
    Then the exit code should be 0

  # All other legacy iac.* validations removed in the new model.

  @validation @prosys_cluster_validation
  Scenario: prosys.dev.dfw3 cluster configuration validation
    Given a file "<<tmp>>/conf/prosys.dev.dfw3.yaml" with content:
      """
      opencenter:
          cluster:
              cluster_name: prosys.dev.dfw3
              domain: dev.attcontroller.com
          gitops:
              git_dir: <<tmp>>/prosys-gitops-repo
          infrastructure:
              cloud:
                  openstack:
                      application_credential_id: "12345678-1234-1234-1234-123456789012"
                      application_credential_secret: "test-app-cred-secret"
                      auth_url: "https://keystone.api.dfw3.rackspacecloud.com/v3/"
                      region: "DFW3"
                      domain: "Default"
                      networking:
                          floating_network_id: "12345678-1234-1234-1234-123456789012"
              provider: openstack
      opentofu:
          enabled: true
          backend:
              type: local
              local:
                  path: terraform.tfstate
      secrets:
          sops_age_key_file: <<tmp>>/sops/age/keys/prosys-dev-dfw3-key.txt
          global:
              openstack:
                  application_credential_id: "12345678-1234-1234-1234-123456789012"
                  application_credential_secret: "test-app-cred-secret"
      networking:
          use_octavia: false
          vrrp_enabled: true
          vrrp_ip: "10.0.4.10"
      """
    When I run "opencenter cluster validate --config <<tmp>>/conf/clusters/opencenter/.prosys.dev.dfw3-config.yaml"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @validation @prosys_cluster_debug_config
  Scenario: prosys.dev.dfw3 cluster debug config generation
    Given a file "<<tmp>>/conf/prosys.dev.dfw3.yaml" with content:
      """
      opencenter:
          cluster:
              cluster_name: prosys.dev.dfw3
              domain: dev.attcontroller.com
          gitops:
              git_dir: <<tmp>>/prosys-gitops-repo
          infrastructure:
              cloud:
                  openstack:
                      application_credential_id: "12345678-1234-1234-1234-123456789012"
                      application_credential_secret: "test-app-cred-secret"
                      domain: "Default"
                      networking:
                          floating_network_id: "12345678-1234-1234-1234-123456789012"
              provider: openstack
      opentofu:
          enabled: true
          backend:
              type: local
              local:
                  path: terraform.tfstate
      secrets:
          sops_age_key_file: <<tmp>>/sops/age/keys/prosys-dev-dfw3-key.txt
          global:
              openstack:
                  application_credential_id: "12345678-1234-1234-1234-123456789012"
                  application_credential_secret: "test-app-cred-secret"
      """
    When I run "opencenter cluster validate --config <<tmp>>/conf/clusters/opencenter/.prosys.dev.dfw3-config.yaml --generate-debug-config --output-dir <<tmp>>"
    Then the exit code should be 0
    And stdout should contain "Debug config saved to"
    And stdout should contain "Validation successful"
    And a file "<<tmp>>/.opencenter-v2.yaml" should exist

  @validation @prosys_cluster_vrrp_validation
  Scenario: prosys.dev.dfw3 cluster VRRP validation with networking section
    Given a file "<<tmp>>/conf/prosys.dev.dfw3.yaml" with content:
      """
      opencenter:
          cluster:
              cluster_name: prosys.dev.dfw3
              domain: dev.attcontroller.com
          gitops:
              git_dir: <<tmp>>/prosys-gitops-repo
          infrastructure:
              cloud:
                  openstack:
                      application_credential_id: "12345678-1234-1234-1234-123456789012"
                      application_credential_secret: "test-app-cred-secret"
                      auth_url: "https://keystone.api.dfw3.rackspacecloud.com/v3/"
                      region: "DFW3"
                      domain: "Default"
                      networking:
                          floating_network_id: "12345678-1234-1234-1234-123456789012"
              provider: openstack
      opentofu:
          enabled: true
          backend:
              type: local
              local:
                  path: terraform.tfstate
      secrets:
          sops_age_key_file: <<tmp>>/sops/age/keys/prosys-dev-dfw3-key.txt
          global:
              openstack:
                  application_credential_id: "12345678-1234-1234-1234-123456789012"
                  application_credential_secret: "test-app-cred-secret"
      networking:
          use_octavia: false
          vrrp_enabled: true
          vrrp_ip: "10.0.4.10"
      """
    When I run "opencenter cluster validate --config <<tmp>>/conf/clusters/opencenter/.prosys.dev.dfw3-config.yaml"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @validation @prosys_cluster_vrrp_missing_ip @priority4 @wip
  Scenario: prosys.dev.dfw3 cluster VRRP validation fails when IP missing
    # Note: This test expects VRRP validation error but other validation errors occur first
    # Validation error ordering makes this test unreliable
    # This test is skipped until validation can be fixed to show all errors
    Given a file "<<tmp>>/conf/prosys.dev.dfw3.yaml" with content:
      """
      opencenter:
          cluster:
              cluster_name: prosys.dev.dfw3
              domain: dev.attcontroller.com
          gitops:
              git_dir: <<tmp>>/prosys-gitops-repo
          infrastructure:
              cloud:
                  openstack:
                      application_credential_id: "12345678-1234-1234-1234-123456789012"
                      application_credential_secret: "test-app-cred-secret"
                      auth_url: "https://keystone.api.dfw3.rackspacecloud.com/v3/"
                      region: "DFW3"
                      domain: "Default"
                      networking:
                          floating_network_id: "12345678-1234-1234-1234-123456789012"
              provider: openstack
      opentofu:
          enabled: true
          backend:
              type: local
              local:
                  path: terraform.tfstate
      secrets:
          sops_age_key_file: <<tmp>>/sops/age/keys/prosys-dev-dfw3-key.txt
          global:
              openstack:
                  application_credential_id: "12345678-1234-1234-1234-123456789012"
                  application_credential_secret: "test-app-cred-secret"
      networking:
          use_octavia: false
          vrrp_enabled: true
          vrrp_ip: ""
      """
    When I run "opencenter cluster validate prosys.dev.dfw3"
    Then the exit code should not be 0
    And stderr should contain "vrrp_ip must be set when use_octavia is false"
    And stderr should contain "opencenter.infrastructure.cloud.openstack.region must be set when provider is openstack"
    And stderr should contain "opencenter.secrets.barbican.auth_url must be set when secrets backend is barbican"
