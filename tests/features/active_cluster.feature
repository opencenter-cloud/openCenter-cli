# tests/features/active_cluster.feature
# Verifies active cluster behavior:
# 1) `select` writes the selected name to the active pointer file.
# 2) Commands that rely on the active cluster read the pointer; error if unset.
# 3) When CWD == selected cluster's gitops.git_dir, the CLI prefixes output with "Active cluster: <name>".

Feature: Active cluster rules

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/repo-demo"

  @active_pointer @select
  Scenario: Selecting a cluster writes its name to the active pointer
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      cluster_name: demo
      gitops:
        git_dir: <<tmp>>/repo-demo
      """
    When I run "openCenter cluster select demo"
    Then the exit code should be 0
    And the file "<<tmp>>/conf/.active" should match regex "^demo$"

  @active_pointer @unset @error
  Scenario: Commands that need the active cluster fail when none is set
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      cluster_name: demo
      gitops:
        git_dir: <<tmp>>/repo-demo
      """
    And the file "<<tmp>>/conf/active" does not exist
    When I run "openCenter cluster info"
    Then the exit code should not be 0
    And stderr should contain "no active cluster"

  @active_pointer @context_header
  Scenario: When in the cluster's git directory, output starts with an active-cluster header
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      cluster_name: demo
      gitops:
        git_dir: <<tmp>>/repo-demo
      """
    And the directory "<<tmp>>/repo-demo" exists
    And I run "openCenter cluster select demo"
    And the exit code should be 0
    And I cd to "<<tmp>>/repo-demo"
    When I run "openCenter cluster info"
    Then the exit code should be 0
    And the first line of stdout should start with "Active cluster: demo"

  @active_pointer @read
  Scenario: Commands read the active pointer when no cluster name is provided
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      cluster_name: demo
      gitops:
        git_dir: <<tmp>>/repo-demo
      """
    And I run "openCenter cluster select demo"
    And the exit code should be 0
    When I run "openCenter cluster info"
    Then the exit code should be 0
    And stdout should contain "demo"
