Feature: Cluster commands integration with new directory structure

  Background:
    Given an empty directory "<<tmp>>/conf"

  Scenario: Cluster use, describe, and validate work with new directory structure
    When I run "opencenter cluster init integration-test --config-dir <<tmp>>/conf"
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

    When I run "opencenter cluster use integration-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster set to integration-test"
    And the file "<<tmp>>/conf/.active" should match regex "^integration-test$"

    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: integration-test"
    And stdout should contain "cluster_name: integration-test"

    When I run "opencenter cluster describe integration-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "cluster_name: integration-test"

    When I run "opencenter cluster validate integration-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  @priority3
  Scenario: Cluster commands handle non-existent clusters correctly
    When I run "opencenter cluster use missing-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "opencenter cluster list"

    When I run "opencenter cluster describe missing-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "opencenter cluster list"

    When I run "opencenter cluster validate missing-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "opencenter cluster list"

  @priority3
  Scenario: Multiple clusters work correctly with new directory structure
    When I run "opencenter cluster init cluster-a --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init cluster-b --config-dir <<tmp>>/conf"
    Then the exit code should be 0

    When I run "opencenter cluster list --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "cluster-a"
    And stdout should contain "cluster-b"

    When I run "opencenter cluster use cluster-a --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster set to cluster-a"

    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: cluster-a"

    When I run "opencenter cluster use cluster-b --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster set to cluster-b"

    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: cluster-b"
