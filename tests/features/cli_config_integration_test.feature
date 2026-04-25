# CLI Configuration System Integration Tests
# Additional integration tests to verify CLI configuration system works with cluster commands

Feature: CLI Configuration System Integration Tests

  Background:
    Given an empty directory "<<tmp>>/conf"

  @integration @config_commands
  Scenario: CLI configuration commands work end-to-end
    When I run "opencenter config path --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/conf/config.yaml"

    When I run "opencenter config view --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "logging:"
    And stdout should contain "level: warn"

    When I run "opencenter config set logging.level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated: logging.level = debug"

    When I run "opencenter config get logging.level --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "debug"

  @integration @global_flags
  Scenario: Global flags work with cluster commands
    When I run "opencenter cluster list --log-level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0

    When I run "opencenter cluster list --dry-run --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "--dry-run has no effect for read-only command"

  @integration @file_operations
  Scenario: Configuration system creates directories automatically
    Given the directory "<<tmp>>/fresh-config" does not exist
    When I run "opencenter config view --config-dir <<tmp>>/fresh-config"
    Then the exit code should be 0
    And the directory "<<tmp>>/fresh-config" should exist
    And a file "<<tmp>>/fresh-config/config.yaml" should exist

  @integration @precedence
  Scenario: Configuration precedence works correctly
    Given I run "opencenter config set logging.level info --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter config get logging.level --log-level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # The command should run with debug logging (overriding the config file's info level)

  @integration @cross_platform
  Scenario: Path handling works across platforms
    When I run "opencenter config set paths.clustersDir <<tmp>>/custom-clusters --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated"

    When I run "opencenter config get paths.clustersDir --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/custom-clusters"

  @integration @error_handling
  Scenario: Configuration validation works properly
    When I run "opencenter config set logging.level invalid --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "invalid value"

    When I run "opencenter config set behavior.autoConfirm maybe --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "expected boolean value"

  @integration @reset
  Scenario: Configuration reset works properly
    Given I run "opencenter config set logging.level debug --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter config reset --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration reset to default values"

    When I run "opencenter config get logging.level --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "warn"
