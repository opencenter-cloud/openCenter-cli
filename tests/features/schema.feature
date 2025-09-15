Feature: JSON schema generation

  @schema
  Scenario: Generate the cluster configuration JSON schema
    When I run "openCenter cluster schema --pretty"
    Then the exit code should be 0
    And stdout should contain '"title":"openCenter Cluster Configuration"'
    And stdout should contain '"$schema":"https://json-schema.org/draft/2020-12/schema"'

