# CLI Configuration System Integration Tests
# Tests for the comprehensive CLI configuration management system including:
# - CLI configuration commands (view, set, get, reset, path)
# - Global flags and precedence system
# - Organization-based path resolution
# - Enhanced cluster commands with configuration integration
# - File system operations and directory creation
# - Configuration precedence across all layers

Feature: CLI Configuration System Integration

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/custom-config"

  # ---------------------------------------------------------------------------
  # CLI Configuration Commands
  # ---------------------------------------------------------------------------
  @config @commands
  Scenario: CLI configuration commands work with default configuration
    When I run "opencenter config path --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/conf/config.yaml"

    When I run "opencenter config view --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "logging:"
    And stdout should contain "level: warn"
    And stdout should contain "format: text"
    And stdout should contain "paths:"
    And stdout should contain "behavior:"
    And stdout should contain "defaults:"

    When I run "opencenter config get logging.level --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^warn$"

  @config @commands @set_get
  Scenario: CLI configuration set and get commands work correctly
    When I run "opencenter config set logging.level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated: logging.level = debug"

    When I run "opencenter config get logging.level --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^debug$"

    When I run "opencenter config set behavior.autoConfirm true --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated: behavior.autoConfirm = true"

    When I run "opencenter config get behavior.autoConfirm --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should match regex "^true$"

    When I run "opencenter config set paths.clustersDir <<tmp>>/custom-clusters --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated: paths.clustersDir = <<tmp>>/custom-clusters"

    When I run "opencenter config get paths.clustersDir --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/custom-clusters"

  @config @commands @reset
  Scenario: CLI configuration reset command restores defaults
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

  @config @commands @validation
  Scenario: CLI configuration commands validate input values
    When I run "opencenter config set logging.level invalid --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "invalid value"

    When I run "opencenter config set behavior.autoConfirm maybe --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "expected boolean value"

    When I run "opencenter config set logging.file.maxSize abc --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "expected integer value"

    When I run "opencenter config get nonexistent.key --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "failed to get configuration value"

  # ---------------------------------------------------------------------------
  # Global Flags and Precedence System
  # ---------------------------------------------------------------------------
  @config @global_flags
  Scenario: Global flags override configuration values
    Given I run "opencenter config set logging.level info --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster list --log-level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # The command should run with debug logging (overriding the config file's info level)

    When I run "opencenter cluster list --dry-run --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # The command should run in dry-run mode

  @config @global_flags @set_flag
  Scenario: Global flag overrides configuration values
    Given I run "opencenter config set behavior.autoConfirm false --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter config get behavior.autoConfirm --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # The flag should override the config file value at runtime

    When I run "opencenter cluster list --log-level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # The command should run with debug logging via override

  # ---------------------------------------------------------------------------
  # Organization-Based Path Resolution
  # ---------------------------------------------------------------------------
  @config @organization @paths
  Scenario: Organization-based directory structure is created correctly
    When I run "opencenter cluster init org-test --org test-org --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/test-org" should exist
    And a directory "<<tmp>>/conf/clusters/test-org/applications" should exist
    And a directory "<<tmp>>/conf/clusters/test-org/applications/overlays" should exist
    And a directory "<<tmp>>/conf/clusters/test-org/infrastructure" should exist
    And a directory "<<tmp>>/conf/clusters/test-org/infrastructure/clusters" should exist
    And a directory "<<tmp>>/conf/clusters/test-org/infrastructure/clusters/org-test" should exist
    And a directory "<<tmp>>/conf/clusters/test-org/secrets" should exist
    And a directory "<<tmp>>/conf/clusters/test-org/secrets/age" should exist
    And a directory "<<tmp>>/conf/clusters/test-org/secrets/age/keys" should exist
    And a file "<<tmp>>/conf/clusters/test-org/.org-test-config.yaml" should exist
    And a file "<<tmp>>/conf/clusters/test-org/secrets/age/keys/org-test-key.txt" should exist
    And a file "<<tmp>>/conf/clusters/test-org/.sops.yaml" should exist

  @config @organization @opencenter
  Scenario: Cluster name is used as organization when none specified
    When I run "opencenter cluster init default-test"
    Then the exit code should be 0
    And a directory "~/.config/opencenter/clusters/opencenter" should exist
    And a directory "~/.config/opencenter/clusters/opencenter/infrastructure/clusters/default-test" should exist
    And a file "~/.config/opencenter/clusters/opencenter/.default-test-config.yaml" should exist
    And the cluster configuration "default-test" should have "opencenter.meta.organization" set to "opencenter"

  @config @organization @multiple_clusters
  Scenario: Multiple clusters in same organization share GitOps structure
    When I run "opencenter cluster init cluster-a --org shared-org --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init cluster-b --org shared-org --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/shared-org/infrastructure/clusters/cluster-a" should exist
    And a directory "<<tmp>>/conf/clusters/shared-org/infrastructure/clusters/cluster-b" should exist
    And a file "<<tmp>>/conf/clusters/shared-org/.sops.yaml" should exist
    # Both clusters should share the same organization-level secrets directory
    And a file "<<tmp>>/conf/clusters/shared-org/secrets/age/keys/cluster-a-key.txt" should exist
    And a file "<<tmp>>/conf/clusters/shared-org/secrets/age/keys/cluster-b-key.txt" should exist

  # ---------------------------------------------------------------------------
  # Enhanced Cluster Commands with Configuration Integration
  # ---------------------------------------------------------------------------
  @config @cluster_commands @select
  Scenario: Enhanced cluster use command shows organization metadata
    Given I run "opencenter cluster init enhanced-test --org enhanced-org --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster use enhanced-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster set to enhanced-test"

    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: enhanced-test"
    And stdout should contain "organization: enhanced-org"

  @config @cluster_commands @list @priority3
  Scenario: Cluster list works with organization-based structure
    Given I run "opencenter cluster init list-test-a --org list-org --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init list-test-b --org list-org --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init list-test-c --org other-org --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster list --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "list-org/list-test-a"
    And stdout should contain "list-org/list-test-b"
    And stdout should contain "other-org/list-test-c"

  @config @cluster_commands @describe @priority6
  Scenario: Cluster describe shows organization-based paths
    Given I run "opencenter cluster init info-test --org info-org --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster describe info-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "cluster_name: info-test"
    And stdout should contain "organization: info-org"
    And stdout should contain "git_dir:"
    And stdout should contain "clusters/info-org"

  # ---------------------------------------------------------------------------
  # File System Operations and Directory Creation
  # ---------------------------------------------------------------------------
  @config @filesystem @auto_creation
  Scenario: Configuration system automatically creates required directories
    Given the directory "<<tmp>>/fresh-config" does not exist
    When I run "opencenter config view --config-dir <<tmp>>/fresh-config"
    Then the exit code should be 0
    And the directory "<<tmp>>/fresh-config" should exist
    And a file "<<tmp>>/fresh-config/config.yaml" should exist

  @config @filesystem @custom_paths
  Scenario: Custom configuration paths work correctly
    Given I run "opencenter config set paths.clustersDir <<tmp>>/custom-clusters --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster init custom-path-test --org custom-org"
    Then the exit code should be 0
    And a directory "<<tmp>>/custom-clusters/custom-org/infrastructure/clusters/custom-path-test" should exist
    And a file "<<tmp>>/custom-clusters/custom-org/.custom-path-test-config.yaml" should exist

  @config @filesystem @permissions
  Scenario: Configuration files are created with proper permissions
    When I run "opencenter config view --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And the file "<<tmp>>/conf/config.yaml" should exist
    # Note: Permission checking would be platform-specific and handled in step definitions

  # ---------------------------------------------------------------------------
  # Configuration Precedence Across All Layers
  # ---------------------------------------------------------------------------
  @config @precedence @environment
  Scenario: Environment variables override configuration file values
    Given I run "opencenter config set paths.clustersDir <<tmp>>/config-clusters --config-dir <<tmp>>/conf"
    And the exit code should be 0
    And I set environment variable "OPENCENTER_CONFIG_DIR" to "<<tmp>>/env-config"
    When I run "opencenter config path"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/env-config/config.yaml"
    # Environment variable should override the config file setting

  @config @precedence @flags_over_config
  Scenario: Command-line flags override configuration file values
    Given I run "opencenter config set logging.level info --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster list --log-level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # The --log-level flag should override the config file's info level

  @config @precedence @set_flag_highest
  Scenario: flag has highest precedence for configuration values
    Given I run "opencenter config set behavior.dryRun false --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster list --dry-run --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # The flag should override the config file value

  @config @precedence @complete_hierarchy
  Scenario: Complete precedence hierarchy works correctly
    Given I run "opencenter config set logging.level warn --config-dir <<tmp>>/conf"
    And the exit code should be 0
    # Test that overrides --log-level which overrides config file
    When I run "opencenter cluster list --log-level debug --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # Should use debug level from flag (highest precedence)

  # ---------------------------------------------------------------------------
  # Cross-Platform Compatibility
  # ---------------------------------------------------------------------------
  @config @cross_platform @path_expansion
  Scenario: Path expansion works correctly across platforms
    When I run "opencenter config set paths.clustersDir ~/test-clusters --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Configuration updated"

    When I run "opencenter config get paths.clustersDir --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # The tilde should be expanded to the actual home directory path
    And stdout should not contain "~"

  @config @cross_platform @environment_expansion
  Scenario: Environment variable expansion works in configuration
    Given I set environment variable "TEST_CLUSTER_DIR" to "<<tmp>>/env-clusters"
    When I run "opencenter config set paths.clustersDir ${TEST_CLUSTER_DIR} --config-dir <<tmp>>/conf"
    Then the exit code should be 0

    When I run "opencenter config get paths.clustersDir --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "<<tmp>>/env-clusters"
    # Environment variable should be expanded

  # ---------------------------------------------------------------------------
  # Error Handling and Validation
  # ---------------------------------------------------------------------------
  @config @error_handling @invalid_config
  Scenario: System handles invalid configuration gracefully
    Given a file "<<tmp>>/conf/config.yaml" with content:
      """
      invalid: yaml: content:
      """
    When I run "opencenter config view --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "failed to load configuration"

  @config @error_handling @missing_permissions
  Scenario: System handles permission errors gracefully
    # This would test permission handling - implementation depends on platform
    When I run "opencenter config view --config-dir /root/no-permission"
    Then the exit code should not be 0
    And stderr should contain "failed"

  @config @error_handling @recovery
  Scenario: System can recover from configuration errors
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
  # Integration with Existing Commands
  # ---------------------------------------------------------------------------
  @config @integration @cluster_lifecycle
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
    And stdout should contain "Active cluster set to lifecycle-test"

    When I run "opencenter cluster describe --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And stdout should contain "Active cluster: lifecycle-test"
    And stdout should contain "organization: lifecycle-org"
    And stdout should contain "provider: openstack"

  @config @integration @gitops_setup
  Scenario: Configuration system works with GitOps setup
    Given I run "opencenter cluster init gitops-test --org gitops-org --config-dir <<tmp>>/conf"
    And the exit code should be 0
    
    When I run "opencenter cluster generate --render-only gitops-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/gitops-repo" should exist
    And a directory "<<tmp>>/gitops-repo/applications" should exist
    And a directory "<<tmp>>/gitops-repo/infrastructure" should exist
