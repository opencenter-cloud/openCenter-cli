Feature: Cluster commands integration with new directory structure

  Background:
    Given an empty directory "<<tmp>>/conf"

  Scenario: Cluster select, info, and validate work with new directory structure
    When I run "openCenter cluster init integration-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/integration-test.yaml" should exist

    # Add required fields for validation
    Given I update the YAML "<<tmp>>/conf/integration-test.yaml" to set:
      """
      opencenter:
        cluster:
          domain: example.com
        infrastructure:
          provider: openstack
          cloud:
            openstack:
              domain: "Default"
              networking:
                floating_network_id: "12345678-1234-1234-1234-123456789012"
              application_credential_id: "12345678-1234-1234-1234-123456789012"
              application_credential_secret: "test-secret"
      secrets:
        global:
          openstack:
            application_credential_id: "12345678-1234-1234-1234-123456789012"
            application_credential_secret: "test-secret"
      """

    When I run "openCenter cluster select integration-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster set to integration-test"
    And the file "<<tmp>>/conf/.active" should match regex "^integration-test$"

    When I run "openCenter cluster info --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: integration-test"
    And stdout should contain "cluster_name: integration-test"

    When I run "openCenter cluster info integration-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "cluster_name: integration-test"

    When I run "openCenter cluster validate integration-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @priority3
  Scenario: Cluster commands handle non-existent clusters correctly
    When I run "openCenter cluster select missing-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "cluster configuration directory 'missing-cluster' not found in clusters subdirectory"

    When I run "openCenter cluster info missing-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "failed to resolve configuration path for cluster"

    When I run "openCenter cluster validate missing-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "failed to resolve configuration path for cluster"

  @priority3
  Scenario: Multiple clusters work correctly with new directory structure
    When I run "openCenter cluster init cluster-a --config-dir <<tmp>>/conf"
    And I run "openCenter cluster init cluster-b --config-dir <<tmp>>/conf"
    Then the exit code should be 0

    When I run "openCenter cluster list --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "cluster-a"
    And stdout should contain "cluster-b"

    When I run "openCenter cluster select cluster-a --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster set to cluster-a"

    When I run "openCenter cluster info --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: cluster-a"

    When I run "openCenter cluster select cluster-b --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster set to cluster-b"

    When I run "openCenter cluster info --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: cluster-b"