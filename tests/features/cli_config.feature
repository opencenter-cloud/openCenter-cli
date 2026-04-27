Feature: CLI configuration system
  Tests for the CLI configuration management system including:
  config view/set/get/reset/path commands, global flags, precedence,
  cross-platform path handling, and error recovery.

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/custom-config"

  # ---------------------------------------------------------------------------
  # Config commands: view, path, get, set, reset
  # ---------------------------------------------------------------------------

  @config @commands
  Scenario: Config path shows the configuration file location
    When I run "opencenter config path --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/conf/config.yaml"

  @config @commands
  Scenario: Config view shows default configuration
    When I run "opencenter config view --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "logging:"
    And stdout should contain "level: warn"
    And stdout should contain "format: text"
    And stdout should contain "paths:"
    And stdout should contain "behavior:"
    And stdout should contain "defaults:"

  @config @commands
  Scenario: Config get retrieves a specific value
    When I run "opencenter config get logging.level --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^warn$"

  @config @commands @set_get
  Scenario: Config set and get round-trip correctly
    When I run "opencenter config set logging.level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated: logging.level = debug"
    When I run "opencenter config get logging.level --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^debug$"

  @config @commands @set_get
  Scenario: Config set works for boolean and path values
    When I run "opencenter config set behavior.autoConfirm true --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated: behavior.autoConfirm = true"
    When I run "opencenter config get behavior.autoConfirm --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^true$"
    When I run "opencenter config set paths.clustersDir <<tmp>>/custom-clusters --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter config get paths.clustersDir --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/custom-clusters"

  @config @commands @reset
  Scenario: Config reset restores defaults
    Given I run "opencenter config set logging.level debug --config-dir <<tmp>>/conf"
    And the exit code should be 0
    And I run "opencenter config set behavior.dryRun true --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter config reset --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration reset to default values"
    When I run "opencenter config get logging.level --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^warn$"
    When I run "opencenter config get behavior.dryRun --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^false$"

  # ---------------------------------------------------------------------------
  # Input validation
  # ---------------------------------------------------------------------------

  @config @validation
  Scenario: Config set validates input values
    When I run "opencenter config set logging.level invalid --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "invalid value"

  @config @validation
  Scenario: Config set rejects non-boolean for boolean fields
    When I run "opencenter config set behavior.autoConfirm maybe --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "expected boolean value"

  @config @validation
  Scenario: Config set rejects non-integer for integer fields
    When I run "opencenter config set logging.file.maxSize abc --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "expected integer value"

  @config @validation
  Scenario: Config get for non-existent key fails
    When I run "opencenter config get nonexistent.key --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "failed to get configuration value"

  # ---------------------------------------------------------------------------
  # Global flags and precedence
  # ---------------------------------------------------------------------------

  @config @global_flags
  Scenario: Global --log-level flag overrides configuration
    Given I run "opencenter config set logging.level info --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster list --log-level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0

  @config @global_flags
  Scenario: Global --dry-run flag errors on read-only commands
    When I run "opencenter cluster list --dry-run --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "--dry-run has no effect for read-only command"

  @config @precedence
  Scenario: Environment variables override configuration file values
    Given I run "opencenter config set paths.clustersDir <<tmp>>/config-clusters --config-dir <<tmp>>/conf"
    And the exit code should be 0
    And I set environment variable "OPENCENTER_CONFIG_DIR" to "<<tmp>>/env-config"
    When I run "opencenter config path"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/env-config/config.yaml"

  # ---------------------------------------------------------------------------
  # Filesystem operations
  # ---------------------------------------------------------------------------

  @config @filesystem
  Scenario: Config system automatically creates required directories
    Given the directory "<<tmp>>/fresh-config" does not exist
    When I run "opencenter config view --config-dir <<tmp>>/fresh-config"
    Then the exit code should be 0
    And the directory "<<tmp>>/fresh-config" should exist
    And a file "<<tmp>>/fresh-config/config.yaml" should exist

  @config @filesystem
  Scenario: Custom configuration paths work correctly
    Given I run "opencenter config set paths.clustersDir <<tmp>>/custom-clusters --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster init custom-path-test --org custom-org"
    Then the exit code should be 0
    And a directory "<<tmp>>/custom-clusters/custom-org/infrastructure/clusters/custom-path-test" should exist
    And a file "<<tmp>>/custom-clusters/custom-org/.custom-path-test-config.yaml" should exist

  # ---------------------------------------------------------------------------
  # Cross-platform path handling
  # ---------------------------------------------------------------------------

  @config @cross_platform
  Scenario: Tilde expansion works in configuration paths
    When I run "opencenter config set paths.clustersDir ~/test-clusters --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated"
    When I run "opencenter config get paths.clustersDir --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should not contain "~"

  @config @cross_platform
  Scenario: Environment variable expansion works in configuration
    Given I set environment variable "TEST_CLUSTER_DIR" to "<<tmp>>/env-clusters"
    When I run "opencenter config set paths.clustersDir ${TEST_CLUSTER_DIR} --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter config get paths.clustersDir --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/env-clusters"

  # ---------------------------------------------------------------------------
  # Error handling and recovery
  # ---------------------------------------------------------------------------

  @config @error_handling
  Scenario: System handles invalid configuration file gracefully
    Given a file "<<tmp>>/conf/config.yaml" with content:
      """
      invalid: yaml: content:
      """
    When I run "opencenter config view --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "failed to load configuration"

  @config @error_handling
  Scenario: System handles permission errors gracefully
    When I run "opencenter config view --config-dir /root/no-permission"
    Then the exit code should not be 0
    And stderr should contain "failed"

  @config @error_handling
  Scenario: System can recover from configuration errors via reset
    Given a file "<<tmp>>/conf/config.yaml" with content:
      """
      logging:
        level: invalid-level
      """
    When I run "opencenter config reset --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration reset to default values"
    When I run "opencenter config get logging.level --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^warn$"

  # ---------------------------------------------------------------------------
  # Integration with cluster lifecycle
  # ---------------------------------------------------------------------------

  @config @integration
  Scenario: Configuration system integrates with complete cluster lifecycle
    Given I run "opencenter config set defaults.provider openstack --config-dir <<tmp>>/conf"
    And I run "opencenter config set defaults.region {{ .OpenCenter.Cluster.ClusterRegion }} --config-dir <<tmp>>/conf"
    And I run "opencenter config set behavior.autoConfirm true --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster init lifecycle-test --org lifecycle-org --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/lifecycle-org/infrastructure/clusters/lifecycle-test" should exist
    And the cluster configuration "lifecycle-test" should have "opencenter.infrastructure.provider" set to "openstack"
    When I run "opencenter cluster use lifecycle-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: lifecycle-test"
    And stdout should contain "organization: lifecycle-org"
    And stdout should contain "provider: openstack"
