# tests/features/cluster_commands.feature
# Expected behavior for the "openCenter cluster" command group:
# - Parent "cluster" prints help & subcommands
# - list/ls scans config_dir for *.yaml and prints names (no .yaml); --json outputs JSON
# - select (by name & interactive), writes active_pointer; header when CWD == git_dir
# - info (active & named), human summary; --json prints full JSON; helpful errors
# - init (non-interactive), does not overwrite unless --force; prints next steps
# - setup (materialize embedded templates into git_dir), idempotent, --force overwrites
# - bootstrap (git init/commit/remote/push) with actionable errors on missing prereqs

Feature: Cluster command group

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

  # ---------------------------------------------------------------------------
  # Parent: help shows subcommands
  # ---------------------------------------------------------------------------
  @help
  Scenario: "openCenter cluster" prints help with all subcommands
    When I run "openCenter cluster --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "openCenter cluster list"
    And stdout should contain "openCenter cluster select"
    And stdout should contain "openCenter cluster info"
    And stdout should contain "openCenter cluster init"
    And stdout should contain "openCenter cluster setup"
    And stdout should contain "openCenter cluster bootstrap"

  # ---------------------------------------------------------------------------
  # list / ls (moved to config_select_list_info.feature)
  # ---------------------------------------------------------------------------
  

  # ---------------------------------------------------------------------------
  # select (moved)
  # ---------------------------------------------------------------------------
  

  # ---------------------------------------------------------------------------
  # info (moved)
  # ---------------------------------------------------------------------------
  

  # ---------------------------------------------------------------------------
  # init
  # ---------------------------------------------------------------------------
  @init @by_name
  Scenario: init <cluster-name> creates a YAML with defaults; does not overwrite unless --force
    Given the file "tmp/conf/newone.yaml" does not exist
    When I run "openCenter cluster init newone --config-dir tmp/conf"
    Then the exit code should be 0
    And the file "tmp/conf/newone.yaml" should exist
    And stdout should contain "Created"
    When I run "openCenter cluster init newone --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "exists"
    When I run "openCenter cluster init newone --force --config-dir tmp/conf"
    Then the exit code should be 0

  # interactive init wizard has been removed

  # ---------------------------------------------------------------------------
  # update (moved to config_update.feature)
  # ---------------------------------------------------------------------------
  

  # ---------------------------------------------------------------------------
  # setup (moved to gitops_setup.feature)
  # ---------------------------------------------------------------------------
  

  # ---------------------------------------------------------------------------
  # bootstrap (moved to gitops_bootstrap.feature)
  # ---------------------------------------------------------------------------
  
