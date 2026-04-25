Feature: Provider preflight checks

  Background:
    Given an empty directory "<<tmp>>/conf"

  @preflight
  Scenario: Preflight runs for the selected cluster
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
      """
    And I run "opencenter cluster use demo --config-dir <<tmp>>/conf"
    When I run "opencenter cluster doctor --config-dir <<tmp>>/conf"
    Then stdout should contain "Preflight complete."
