Feature: Cluster use, listing, and inspection
  Verifies cluster list, use, describe, and active cluster behaviour.

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/repo-dev"
    And an empty directory "<<tmp>>/repo-prod"
    And a file "<<tmp>>/conf/dev.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: dev
          kubernetes:
            master_count: 1
            worker_count: 1
            subnet_pods: "10.42.0.0/16"
            subnet_services: "10.43.0.0/16"
            loadbalancer_provider: octavia
        gitops:
          git_dir: "<<tmp>>/repo-dev"
          git_url: ""
        infrastructure:
          provider: openstack
          cloud:
            openstack:
              region: "regionOne"
      """
    And a file "<<tmp>>/conf/prod.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: prod
          kubernetes:
            master_count: 3
            worker_count: 6
            subnet_pods: "10.42.0.0/16"
            subnet_services: "10.43.0.0/16"
            loadbalancer_provider: octavia
        gitops:
          git_dir: "<<tmp>>/repo-prod"
          git_url: ""
        infrastructure:
          provider: openstack
          cloud:
            openstack:
              region: "regionOne"
      """

  # ---------------------------------------------------------------------------
  # Help
  # ---------------------------------------------------------------------------

  @help @priority6
  Scenario: "opencenter cluster" prints help with all subcommands
    When I run "opencenter cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "list"
    And stdout should contain "use"
    And stdout should contain "describe"
    And stdout should contain "init"
    And stdout should contain "generate"
    And stdout should contain "deploy"

  # ---------------------------------------------------------------------------
  # List
  # ---------------------------------------------------------------------------

  @list
  Scenario: Listing clusters shows names without .yaml
    When I run "opencenter cluster list --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "dev"
    And stdout should contain "prod"
    And stdout should not contain ".yaml"

  @list @json
  Scenario: Listing clusters as JSON
    When I run "opencenter cluster ls --output json --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain '["dev","prod"]'

  @list
  Scenario: If config_dir does not exist, create it and print no entries
    Given the directory "<<tmp>>/fresh-conf" does not exist
    When I run "opencenter cluster list --config-dir <<tmp>>/fresh-conf"
    Then the exit code should be 0
    And the directory "<<tmp>>/fresh-conf" should exist
    And stdout should be empty

  # ---------------------------------------------------------------------------
  # Use (select)
  # ---------------------------------------------------------------------------

  @select @by_name
  Scenario: Selecting a cluster by name writes active pointer
    When I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And the file "<<tmp>>/conf/.active" should match regex "^dev$"
    And stdout should contain "Active cluster set to dev"

  @select @interactive
  Scenario: Selecting a cluster interactively
    When I run interactively "opencenter cluster use --config-dir <<tmp>>/conf"
    And I choose "prod" from the prompt
    Then the exit code should be 0
    And the file "<<tmp>>/conf/.active" should match regex "^prod$"
    And stdout should contain "Active cluster set to prod"

  @select @missing @priority3
  Scenario: Selecting a non-existent cluster yields a helpful error
    When I run "opencenter cluster use missing --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "cluster not found"
    And stderr should contain "opencenter cluster list"

  # ---------------------------------------------------------------------------
  # Active cluster behaviour
  # ---------------------------------------------------------------------------

  @active @unset
  Scenario: Commands that need the active cluster fail when none is set
    Given the file "<<tmp>>/conf/active" does not exist
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "no active cluster"

  @active @read
  Scenario: Commands read the active pointer when no cluster name is provided
    Given I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "dev"

  @active @context_header
  Scenario: When in the cluster's git directory, output starts with an active-cluster header
    Given the directory "<<tmp>>/repo-dev" exists
    And I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    And the exit code should be 0
    And I cd to "<<tmp>>/repo-dev"
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And the first line of stdout should start with "Active cluster: dev"

  @active
  Scenario: Show current active cluster
    Given I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster active --config-dir <<tmp>>/conf"
    Then stdout should contain "dev"

  # ---------------------------------------------------------------------------
  # Describe (info)
  # ---------------------------------------------------------------------------

  @describe @active
  Scenario: Describe for the active cluster shows summary
    Given I run "opencenter cluster use prod --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "cluster_name: prod"
    And stdout should contain "git_dir:"

  @describe @named @json
  Scenario: Describe for a named cluster with JSON output
    When I run "opencenter cluster describe dev --output json --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain '"cluster_name": "dev"'
    And stdout should contain '"git_dir":'

  @describe @validate
  Scenario: Validating configuration with --validate
    When I run "opencenter cluster describe dev --validate --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "Validation failed"

  @describe @invalid_yaml
  Scenario: Invalid YAML is surfaced as a parse error
    Given a file "<<tmp>>/conf/bad.yaml" with content:
      """
      : not: yaml:
      """
    When I run "opencenter cluster describe bad --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "parse"
    And stderr should contain "yaml"

  @describe @missing @priority3
  Scenario: Describe for a non-existent cluster yields a helpful error
    When I run "opencenter cluster describe missing-cluster --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "opencenter cluster list"

  # ---------------------------------------------------------------------------
  # Organization-based list and describe
  # ---------------------------------------------------------------------------

  @org @list @priority3
  Scenario: Cluster list works with organization-based structure
    Given I run "opencenter cluster init list-test-a --org list-org --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init list-test-b --org list-org --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init list-test-c --org other-org --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster list --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "list-org/list-test-a"
    And stdout should contain "list-org/list-test-b"
    And stdout should contain "other-org/list-test-c"

  @org @select
  Scenario: Enhanced cluster use command shows organization metadata
    Given I run "opencenter cluster init enhanced-test --org enhanced-org --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster use enhanced-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster set to enhanced-test"
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: enhanced-test"
    And stdout should contain "organization: enhanced-org"

  @org @describe @priority6
  Scenario: Cluster describe shows organization-based paths
    Given I run "opencenter cluster init info-test --org info-org --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster describe info-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "cluster_name: info-test"
    And stdout should contain "organization: info-org"
    And stdout should contain "git_dir:"
    And stdout should contain "clusters/gitops/info-org"

  # ---------------------------------------------------------------------------
  # Multiple clusters lifecycle
  # ---------------------------------------------------------------------------

  @lifecycle @priority3
  Scenario: Multiple clusters work correctly with switching
    Given I run "opencenter cluster init cluster-a --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init cluster-b --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster list --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "cluster-a"
    And stdout should contain "cluster-b"
    When I run "opencenter cluster use cluster-a --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: cluster-a"
    When I run "opencenter cluster use cluster-b --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: cluster-b"

  # ---------------------------------------------------------------------------
  # Preflight / Doctor
  # ---------------------------------------------------------------------------

  @preflight
  Scenario: Preflight runs for the selected cluster
    Given I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster doctor --config-dir <<tmp>>/conf"
    Then stdout should contain "Doctor checks complete."
