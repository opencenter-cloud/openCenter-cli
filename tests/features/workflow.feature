# tests/features/workflow_minimal_network.feature
# Maps to workflow:
#   ./openCenter cluster init demo
#   ./openCenter cluster select demo
#   # minimal network choice: use_octavia=false -> must set vrrp_ip
#   ./openCenter cluster validate
#   ./openCenter cluster setup --render
#   ./openCenter cluster bootstrap

Feature: Minimal network workflow (VRRP) from init to bootstrap

  Background:
    Given an empty directory "tmp/conf"
    And an empty directory "tmp/repo-demo"

  @workflow @init @select @validate @setup @bootstrap
  Scenario: Initialize, select, validate VRRP requirement, render setup, and bootstrap
    # ./openCenter cluster init demo
    When I run "openCenter cluster init demo --config-dir tmp/conf --force"
    Then the exit code should be 0
    And the file "tmp/conf/clusters/opencenter/infrastructure/clusters/demo/.demo-config.yaml" should exist

    # ./openCenter cluster select demo
    When I run "openCenter cluster select demo --config-dir tmp/conf"
    Then the exit code should be 0
    And the file "tmp/conf/active" should match regex "^demo\\s*$"

    # minimal network choice: use_octavia=false -> must set vrrp_ip (first show the failure)
    # Configure minimal fields with missing vrrp_ip to ensure validator catches it.
    Given I update the YAML "tmp/conf/demo.yaml" to set:
      """
      gitops:
        git_dir: tmp/repo-demo
        git_url: tmp/remote.git
      iac:
        counts: {}
        flavors: {}
        networking:
          use_octavia: false
          vrrp_enabled: true
          vrrp_ip: ""
      """
    # ./openCenter cluster validate  (expect failure due to missing vrrp_ip)
    When I run "openCenter cluster validate --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "vrrp_ip"
    And stderr should contain "must be set"

    # Fix the config: set a proper vrrp_ip and validate again
    Given I update the YAML "tmp/conf/demo.yaml" to set:
      """
      iac:
        networking:
          vrrp_ip: 10.0.0.10
      """
    When I run "openCenter cluster validate --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "validation"
    And stdout should not contain "ERROR"

    # ./openCenter cluster setup --render (renders and materializes the repo)
    When I run "openCenter cluster setup --render --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "Setup complete"
    And the directory "tmp/repo-demo" should exist
    And the directory "tmp/repo-demo" should contain a file matching "gitignore"
    And the directory "tmp/repo-demo" should contain a directory "applications"

    # Prepare the remote for bootstrap
    Given a bare git repository exists at "tmp/remote.git"

    # ./openCenter cluster bootstrap (pushes to remote)
    When I run "openCenter cluster bootstrap --config-dir tmp/conf"
    Then the exit code should be 0
    And the bare repo "tmp/remote.git" should have branch "main"
