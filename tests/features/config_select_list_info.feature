# Configuration: list, select, and info flows

Feature: Configuration selection and inspection

  Background:
    Given an empty directory "tmp/conf"
    And an empty directory "tmp/repo-dev"
    And an empty directory "tmp/repo-prod"
    And a file "tmp/conf/dev.yaml" with content:
      """
      cluster_name: dev
      git_dir: tmp/repo-dev
      git_url: ""
      """
    And a file "tmp/conf/prod.yaml" with content:
      """
      cluster_name: prod
      git_dir: tmp/repo-prod
      git_url: ""
      """

  # list / ls
  @config @list
  Scenario: Listing clusters shows file basenames without .yaml
    When I run "openCenter cluster list --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "dev"
    And stdout should contain "prod"
    And stdout should not contain ".yaml"

  @config @list @json
  Scenario: Listing clusters as JSON
    When I run "openCenter cluster ls --json --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "["
    And stdout should contain "\"dev\""
    And stdout should contain "\"prod\""

  @config @list @missing_dir
  Scenario: If config_dir does not exist, create it and print no entries
    Given the directory "<<tmp>>/fresh-conf" does not exist
    When I run "openCenter cluster list --config-dir <<tmp>>/fresh-conf"
    Then the exit code should be 0
    And the directory "<<tmp>>/fresh-conf" should exist
    And stdout should be empty

  # select
  @config @select @by_name
  Scenario: Selecting a cluster by name verifies file and writes active_pointer
    When I run "openCenter cluster select dev --config-dir tmp/conf"
    Then the exit code should be 0
    And the file "tmp/conf/.active" should match regex "^dev$"
    And stdout should contain "Selected cluster: dev"

  @config @select @interactive
  Scenario: Selecting a cluster interactively
    When I run interactively "openCenter cluster select --config-dir tmp/conf"
    And I choose "prod" from the prompt
    Then the exit code should be 0
    And the file "tmp/conf/.active" should match regex "^prod$"
    And stdout should contain "Selected cluster: prod"

  @config @select @missing
  Scenario: Selecting a non-existent cluster yields a helpful error
    When I run "openCenter cluster select missing --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "cluster 'missing' not found"
    And stderr should contain "openCenter cluster list"

  @config @select @header_in_git_dir
  Scenario: When CWD equals selected cluster's git_dir, subsequent commands show an active header
    Given I run "openCenter cluster select dev --config-dir tmp/conf"
    And the exit code should be 0
    And I cd to "tmp/repo-dev"
    When I run "openCenter cluster info --config-dir ../conf"
    Then the exit code should be 0
    And the first line of stdout should start with "Active cluster: dev"

  # info
  @config @info @active
  Scenario: Info without argument reads active_pointer
    Given I run "openCenter cluster select prod --config-dir tmp/conf"
    And the exit code should be 0
    When I run "openCenter cluster info --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "cluster_name: prod"
    And stdout should contain "git_dir: tmp/repo-prod"

  @config @info @named @json
  Scenario: Info for a named cluster with --json prints full parsed config
    When I run "openCenter cluster info dev --json --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "\"cluster_name\":\"dev\""
    And stdout should contain "\"git_dir\":\"tmp/repo-dev\""

  @config @info @unset_active
  Scenario: Info without active cluster set yields helpful message
    Given the file "tmp/conf/active" does not exist
    When I run "openCenter cluster info --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "no active cluster"

  @config @info @invalid_yaml
  Scenario: Invalid YAML is surfaced as a parse error
    Given a file "tmp/conf/bad.yaml" with content:
      """
      : not: yaml:
      """
    When I run "openCenter cluster info bad --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "parse"
    And stderr should contain "yaml"

