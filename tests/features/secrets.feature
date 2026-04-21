Feature: SOPS age key generation

  Scenario: Generate an age key to a specific path
    When I run "opencenter secrets keys generate --key-file <<tmp>>/age.keys --update-sops-config=false"
    Then the exit code should be 0
    And a file "<<tmp>>/age.keys" should exist
    And the file "<<tmp>>/age.keys" should contain "AGE-SECRET-KEY-1"
