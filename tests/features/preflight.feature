Feature: Provider preflight checks

  Background:
    Given an empty directory "<<tmp>>/conf"

  @preflight
  Scenario: Preflight runs for the selected cluster
    Given a file "<<tmp>>/conf/demo.yaml" with content:
      """
      cluster_name: demo
      """
    And I run "openCenter cluster select demo --config-dir <<tmp>>/conf"
    When I run "openCenter cluster preflight --config-dir <<tmp>>/conf"
    Then stdout should contain "Preflight complete."

