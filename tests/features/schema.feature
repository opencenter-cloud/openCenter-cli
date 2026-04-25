Feature: JSON schema generation

  @schema
  Scenario: Generate the cluster configuration JSON schema
    When I run "opencenter config ide --schema-only"
    Then the exit code should be 0
    And stdout should contain "Schema generated"
    And stdout should contain "schema/cluster.schema.json"
