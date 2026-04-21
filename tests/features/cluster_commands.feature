# tests/features/cluster_commands.feature
# Expected behavior for the "opencenter cluster" command group:
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
      opencenter:
        cluster:
          cluster_name: dev
        gitops:
          git_dir: tmp/repo-dev
          git_url: ""
      """
    And a file "tmp/conf/prod.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: prod
        gitops:
          git_dir: tmp/repo-prod
          git_url: ""
      """

  # ---------------------------------------------------------------------------
  # Parent: help shows subcommands
  # ---------------------------------------------------------------------------
  @help @priority6
  Scenario: "opencenter cluster" prints help with all subcommands
    When I run "opencenter cluster --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "list"
    And stdout should contain "select"
    And stdout should contain "info"
    And stdout should contain "init"
    And stdout should contain "render"
    And stdout should contain "bootstrap"

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
    When I run "opencenter cluster init newone --config-dir tmp/conf"
    Then the exit code should be 0
    And the file "tmp/conf/newone.yaml" should exist
    And stdout should contain "Created"
    When I run "opencenter cluster init newone --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "exists"
    When I run "opencenter cluster init newone --force --config-dir tmp/conf"
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
  
