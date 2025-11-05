Feature: SOPS age key generation

  Scenario: Generate an age key to a specific path
    When I run "openCenter sops generate-key --key-file <<tmp>>/age.keys"
    Then a file "<<tmp>>/age.keys" should exist
    And the file "<<tmp>>/age.keys" should contain "AGE-SECRET-KEY-1"

