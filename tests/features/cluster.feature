Feature: opencenter cluster basics
  Background:
    Given an empty directory "<<tmp>>/conf"

  Scenario: Initialize a cluster with defaults
    When I run "opencenter cluster init demo --config-dir <<tmp>>/conf"
    Then a file "<<tmp>>/conf/demo.yaml" should exist
    And the file "<<tmp>>/conf/demo.yaml" should contain "cluster_name: demo"

  Scenario: Select the cluster
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
      """
    When I run "opencenter cluster use demo --config-dir <<tmp>>/conf"
    Then the file "<<tmp>>/conf/.active" should match regex "^demo$"

  Scenario: Show current cluster
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
      """
    And I run "opencenter cluster use demo --config-dir <<tmp>>/conf"
    When I run "opencenter cluster active --config-dir <<tmp>>/conf"
    Then stdout should contain "demo"

  Scenario: List clusters
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
      """
    And a file "<<tmp>>/conf/blue.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: blue
      """
    And a file "<<tmp>>/conf/green.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: green
      """
    When I run "opencenter cluster list --config-dir <<tmp>>/conf"
    Then stdout should contain:
      """
      blue
      demo
      green
      """

  Scenario: List clusters as JSON
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
      """
    And a file "<<tmp>>/conf/blue.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: blue
      """
    And a file "<<tmp>>/conf/green.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: green
      """
    When I run "opencenter cluster list --output json --config-dir <<tmp>>/conf"
    Then stdout should contain '["blue","demo","green"]'

  Scenario: Info for a cluster
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
      """
    And I run "opencenter cluster use demo --config-dir <<tmp>>/conf"
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then stdout should contain "cluster_name: demo"

  Scenario: Validate constraints
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
          domain: example.com
        gitops:
          git_dir: "<<tmp>>/repo"
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
    When I run "opencenter cluster validate demo --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Validation successful"

  Scenario: Validate constraints failure
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
        gitops:
          git_dir: ""
      """
    When I run "opencenter cluster generate --render-only demo --config-dir <<tmp>>/conf"
    Then exit code should be 1
    And stderr should contain "opencenter.gitops.repository.local_dir must be set"

  Scenario: Preflight
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
      """
    And I run "opencenter cluster use demo --config-dir <<tmp>>/conf"
    When I run "opencenter cluster doctor --config-dir <<tmp>>/conf"
    Then stdout should contain "Preflight complete."

  @hangs @wip
  @wip
  Scenario: Bootstrap pushes a new commit to a remote repository
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
        gitops:
          git_dir: "<<tmp>>/opencenter-demo"
      """

    And a bare git repository exists at "<<tmp>>/remote.git"
    And I update the YAML "<<tmp>>/conf/demo.yaml" to set:
      """
      opencenter:
        gitops:
          git_url: "git@localhost:newuser/gitops-repo.git"
      """
    And I run "opencenter cluster generate --render-only demo --config-dir <<tmp>>/conf"
    When I run "opencenter cluster deploy demo --restart --config-dir <<tmp>>/conf"
    Then the command should succeed
    And the remote git repository should contain a "Bootstrap commit"

  @hangs
  Scenario: Setup with provisioning
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
        gitops:
          git_dir: "<<tmp>>/opencenter-demo"
      opentofu:
        enabled: true
      """
    When I run "opencenter cluster generate --render-only demo --config-dir <<tmp>>/conf"
    Then a file "<<tmp>>/opencenter-demo/infrastructure/clusters/demo/main.tf" should exist
    And a file "<<tmp>>/opencenter-demo/infrastructure/clusters/demo/provider.tf" should exist

  @skip @priority7
  Scenario: Destroy a cluster
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
        gitops:
          git_dir: "<<tmp>>/opencenter-demo"
      """
    When I run "opencenter cluster destroy demo --force --skip-infrastructure --remove-files --config-dir <<tmp>>/conf"
    Then the command should succeed
    And a file "<<tmp>>/conf/demo.yaml" should not exist
    And a directory "<<tmp>>/opencenter-demo" should not exist
