Feature: Destroy clusters safely

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/opencenter-demo"

  @destroy
  Scenario: Destroy removes config and GitOps directory
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      cluster_name: demo
      gitops:
        git_dir: "<<tmp>>/opencenter-demo"
      """
    When I run "openCenter cluster destroy demo --config-dir <<tmp>>/conf"
    Then the command should succeed
    And a file "<<tmp>>/conf/demo.yaml" should not exist
    And a directory "<<tmp>>/opencenter-demo" should not exist

