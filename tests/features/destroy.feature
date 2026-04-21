Feature: Destroy clusters safely

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/opencenter-demo"

  @destroy @priority7
  Scenario: Destroy removes config and GitOps directory
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
