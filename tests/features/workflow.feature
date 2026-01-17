# tests/features/organization_workflow.feature
# Maps to organization-based workflow:
#   ./openCenter cluster init demo --org my-org
#   ./openCenter cluster select my-org/demo
#   # minimal network choice: use_octavia=false -> must set vrrp_ip
#   ./openCenter cluster validate
#   ./openCenter cluster setup --render
#   ./openCenter cluster bootstrap

Feature: Organization-based minimal network workflow (VRRP) from init to bootstrap

  Background:
    Given an empty directory "tmp/conf"
    And an empty directory "tmp/repo-demo"

  @workflow @init @select @validate @setup @bootstrap
  Scenario: Initialize with org, select, validate VRRP requirement, render setup, and bootstrap
    # ./openCenter cluster init demo --org my-org
    When I run "openCenter cluster init demo --org my-org --config-dir tmp/conf --force"
    Then the exit code should be 0
    And the file "tmp/conf/clusters/my-org/.demo-config.yaml" should exist

    # ./openCenter cluster select my-org/demo
    When I run "openCenter cluster select my-org/demo --config-dir tmp/conf"
    Then the exit code should be 0
    And the file "tmp/conf/active" should match regex "^my-org/demo\\s*$"

    # minimal network choice: use_octavia=false -> must set vrrp_ip (first show the failure)
    # Configure minimal fields with missing vrrp_ip to ensure validator catches it.
    Given I update the YAML "tmp/conf/clusters/my-org/infrastructure/clusters/demo/.demo-config.yaml" to set:
      """
opencenter:
  gitops:
    git_dir: tmp/repo-demo
    git_url: tmp/remote.git
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
    Given I update the YAML "tmp/conf/clusters/my-org/infrastructure/clusters/demo/.demo-config.yaml" to set:
      """
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