// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/di"
	"github.com/rackerlabs/openCenter-cli/internal/plugins"
)

// ContainerKey is the context key for the DI container
type contextKey string

const ContainerKey contextKey = "container"

// GetContainer retrieves the DI container from the context
func GetContainer(ctx context.Context) (di.Container, error) {
	container, ok := ctx.Value(ContainerKey).(di.Container)
	if !ok || container == nil {
		return nil, fmt.Errorf("DI container not found in context")
	}
	return container, nil
}

// GlobalFlags represents the global flags available across all commands.
type GlobalFlags struct {
	Config     string   // --config: alternative cluster configuration file path
	DryRun     bool     // --dry-run: enable dry-run mode
	LogLevel   string   // --log-level: set log level explicitly
	Set        []string // --set: override configuration values using dot notation
	Verbose    bool     // --verbose: enable verbose logging
	ShowActive bool     // --show-active: display the current active cluster
}

var rootCmd = &cobra.Command{
	Use:   "openCenter",
	Short: "openCenter CLI manages cluster configurations and GitOps scaffolding",
	Long: `openCenter is a command-line tool for managing Kubernetes cluster configurations
and GitOps repositories. It provides a declarative approach to cluster lifecycle
management with built-in validation, secrets management, and multi-provider support.

Key Features:
  • Declarative YAML-based cluster configuration
  • Automatic GitOps repository scaffolding
  • SOPS integration for secrets management
  • Multi-cloud provider support (OpenStack, AWS, VMware, Kind)
  • Comprehensive validation and preflight checks
  • Organization-based multi-tenancy support

Documentation: https://docs.opencenter.cloud
Support: https://github.com/rackerlabs/openCenter-cli/issues`,
	Example: `  # Initialize a new cluster configuration
  openCenter cluster init my-cluster

  # Validate cluster configuration
  openCenter cluster validate my-cluster

  # Generate and view JSON schema
  openCenter cluster schema --pretty

  # List all clusters
  openCenter cluster list

  # Bootstrap a cluster with GitOps
  openCenter cluster bootstrap my-cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute runs the root command and returns any error. This is the main
// entrypoint for the openCenter CLI. It is called by main.main().
//
// Inputs:
//   - version: The version string for the application.
//
// Outputs:
//   - error: An error if one occurred during execution.
func Execute(version string) error {
	return ExecuteWithContext(context.Background(), version)
}

// ExecuteWithContext runs the root command with a context containing the DI container.
//
// Inputs:
//   - ctx: Context containing the DI container
//   - version: The version string for the application.
//
// Outputs:
//   - error: An error if one occurred during execution.
func ExecuteWithContext(ctx context.Context, version string) error {
	rootCmd.Version = version

	// Add global persistent flags
	addGlobalFlags(rootCmd)

	// Pre-parse --config-dir from os.Args so plugin discovery can use it
	// before Cobra runs PersistentPreRunE.
	for i := 0; i < len(os.Args); i++ {
		a := os.Args[i]
		if a == "--config-dir" && i+1 < len(os.Args) {
			_ = os.Setenv("OPENCENTER_CONFIG_DIR", os.Args[i+1])
			break
		}
		if strings.HasPrefix(a, "--config-dir=") {
			_ = os.Setenv("OPENCENTER_CONFIG_DIR", strings.TrimPrefix(a, "--config-dir="))
			break
		}
	}

	// Register subcommands
	rootCmd.AddCommand(NewClusterCmd())
	rootCmd.AddCommand(NewConfigCmd())
	rootCmd.AddCommand(NewSOPSCmd())
	rootCmd.AddCommand(NewSecretsCmd())
	rootCmd.AddCommand(NewPluginsCmd())
	rootCmd.AddCommand(NewVersionCmd())
	// Discover and attach external plugins as subcommands
	plugins.LoadExternalPlugins(rootCmd)
	
	// Execute with context
	return rootCmd.ExecuteContext(ctx)
}

// addGlobalFlags adds global persistent flags to the root command.
func addGlobalFlags(cmd *cobra.Command) {
	// Legacy config-dir flag (kept for backward compatibility)
	cmd.PersistentFlags().String("config-dir", "", "configuration directory (defaults to ~/.config/openCenter on Linux/macOS)")

	// New global flags
	cmd.PersistentFlags().String("config", "", "alternative cluster configuration file path")
	cmd.PersistentFlags().Bool("dry-run", false, "enable dry-run mode to print planned actions without executing them")
	cmd.PersistentFlags().String("log-level", "warn", "set log level explicitly (debug, info, warn, error)")
	cmd.PersistentFlags().StringArray("set", []string{}, "override configuration values using dot notation (e.g., --set spec.provider=openstack)")
	cmd.PersistentFlags().Bool("verbose", false, "enable verbose logging by setting log level to debug")
	cmd.PersistentFlags().Bool("show-active", false, "display the current active cluster")
}

// parseGlobalFlags extracts global flags from the command.
func parseGlobalFlags(cmd *cobra.Command) (*GlobalFlags, error) {
	config, _ := cmd.Flags().GetString("config")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	logLevel, _ := cmd.Flags().GetString("log-level")
	set, _ := cmd.Flags().GetStringArray("set")
	verbose, _ := cmd.Flags().GetBool("verbose")
	showActive, _ := cmd.Flags().GetBool("show-active")

	// If verbose is set, override log level to debug
	if verbose {
		logLevel = "debug"
	}

	return &GlobalFlags{
		Config:     config,
		DryRun:     dryRun,
		LogLevel:   logLevel,
		Set:        set,
		Verbose:    verbose,
		ShowActive: showActive,
	}, nil
}

// initializeGlobalConfig initializes the configuration manager and applies overrides.
// It returns the initialized config manager instead of storing it globally.
func initializeGlobalConfig(cmd *cobra.Command) (*config.ConfigManager, error) {
	// Handle legacy config-dir flag
	if cfgDir, _ := cmd.Flags().GetString("config-dir"); cfgDir != "" {
		// Set environment variable so that config.ResolveConfigDir picks it up
		if err := os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir); err != nil {
			return nil, err
		}
	}

	// Parse global flags
	globalFlags, err := parseGlobalFlags(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to parse global flags: %w", err)
	}

	// Handle --show-active flag early, before other initialization
	if globalFlags.ShowActive {
		if err := displayActiveCluster(cmd); err != nil {
			return nil, err
		}
		// Return nil to indicate early exit
		return nil, nil
	}

	// Initialize configuration manager
	var configPath string
	if globalFlags.Config != "" {
		configPath = globalFlags.Config
	}

	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize configuration: %w", err)
	}

	// Apply global flag overrides
	if err := applyGlobalFlagOverrides(configManager, globalFlags); err != nil {
		return nil, fmt.Errorf("failed to apply global flag overrides: %w", err)
	}

	// Log that configuration has been initialized
	config.Debug("Configuration initialized successfully")
	config.WithFields(logrus.Fields{
		"config_path": configManager.GetConfigPath(),
		"log_level":   configManager.GetConfig().Logging.Level,
		"log_format":  configManager.GetConfig().Logging.Format,
		"log_output":  configManager.GetConfig().Logging.Output,
	}).Debug("Configuration details")

	return configManager, nil
}

// applyGlobalFlagOverrides applies global flag overrides to the configuration.
func applyGlobalFlagOverrides(configManager *config.ConfigManager, globalFlags *GlobalFlags) error {
	cliConfig := configManager.GetConfig()

	// Create a copy to apply overrides
	overriddenConfig := *cliConfig

	// Apply log level override
	if globalFlags.LogLevel != "warn" || globalFlags.Verbose {
		config.Debugf("Overriding log level from '%s' to '%s'", overriddenConfig.Logging.Level, globalFlags.LogLevel)
		overriddenConfig.Logging.Level = globalFlags.LogLevel
	}

	// Apply dry-run override
	if globalFlags.DryRun {
		config.Debug("Enabling dry-run mode via global flag")
		overriddenConfig.Behavior.DryRun = true
	}

	// Apply verbose override
	if globalFlags.Verbose {
		config.Debug("Enabling verbose mode via global flag")
		overriddenConfig.Behavior.Verbose = true
	}

	// Apply --set flag overrides
	if err := applySetFlagOverrides(&overriddenConfig, globalFlags.Set); err != nil {
		return fmt.Errorf("failed to apply --set overrides: %w", err)
	}

	// Update the configuration manager with overridden config
	// Note: We don't save these overrides to file, they're runtime-only
	if err := configManager.LoadWithConfig(&overriddenConfig); err != nil {
		return fmt.Errorf("failed to load overridden configuration: %w", err)
	}

	return nil
}

// applySetFlagOverrides applies --set flag overrides to the configuration.
func applySetFlagOverrides(cliConfig *config.CLIConfig, setFlags []string) error {
	// Create a temporary config manager to use its SetValue method
	tempManager, err := config.NewConfigManagerWithConfig(cliConfig)
	if err != nil {
		return fmt.Errorf("failed to create temporary config manager: %w", err)
	}

	for _, setFlag := range setFlags {
		parts := strings.SplitN(setFlag, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --set format '%s', expected key=value", setFlag)
		}

		key := parts[0]
		value := parts[1]

		// Parse the value (try to detect type)
		var parsedValue interface{}
		if value == "true" {
			parsedValue = true
		} else if value == "false" {
			parsedValue = false
		} else {
			// Try to parse as number
			if intVal, err := fmt.Sscanf(value, "%d", new(int)); err == nil && intVal == 1 {
				var num int
				fmt.Sscanf(value, "%d", &num)
				parsedValue = num
			} else {
				// Treat as string
				parsedValue = value
			}
		}

		// Apply the override using the configuration manager's dot notation
		if err := tempManager.SetValue(key, parsedValue); err != nil {
			return fmt.Errorf("failed to set configuration value '%s=%s': %w", key, value, err)
		}

		config.WithFields(logrus.Fields{
			"key":   key,
			"value": parsedValue,
		}).Debug("Applied --set flag override")
	}

	// Get the updated configuration back
	*cliConfig = *tempManager.GetConfig()
	return nil
}

// displayActiveCluster displays the current active cluster and exits.
func displayActiveCluster(cmd *cobra.Command) error {
	activeCluster, err := config.GetActive()
	if err != nil {
		return fmt.Errorf("failed to get active cluster: %w", err)
	}

	if activeCluster == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "No active cluster set\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Use 'openCenter cluster select <name>' to set an active cluster\n")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Active cluster: %s\n", activeCluster)
	}

	return nil
}

// GetRootCmd returns the root cobra command.
func GetRootCmd() *cobra.Command {
	return rootCmd
}

// helpers for printing errors. In Cobra commands, returning an error
// will cause it to be printed and the process to exit with a non-zero
// code. Use fmt.Errorf to wrap underlying errors.
func failf(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}

// GetConfigManager is a temporary stub that returns nil.
// This will be replaced with DI container access in subtask 17.4.
// Deprecated: Use DI container instead.
func GetConfigManager() *config.ConfigManager {
	return nil
}

// formatError formats an error (temporary stub).
// This will be replaced with DI container access.
// Deprecated: Use DI container instead.
func formatError(err error) error {
	return err
}

// formatErrorWithCode formats an error with an error code (temporary stub).
// This will be replaced with DI container access.
// Deprecated: Use DI container instead.
func formatErrorWithCode(err error, code string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("[%s] %w", code, err)
}

// formatErrorWithFix formats an error with a fix suggestion (temporary stub).
// This will be replaced with DI container access.
// Deprecated: Use DI container instead.
func formatErrorWithFix(err error, fix string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w\nFix: %s", err, fix)
}

// formatMultipleErrors formats multiple errors (temporary stub).
// This will be replaced with DI container access.
// Deprecated: Use DI container instead.
func formatMultipleErrors(errs []error, verbose bool) error {
	if len(errs) == 0 {
		return nil
	}
	
	maxErrors := 5
	if verbose {
		maxErrors = len(errs)
	}
	
	var msg strings.Builder
	for i, err := range errs {
		if i >= maxErrors {
			msg.WriteString(fmt.Sprintf("\n... and %d more errors", len(errs)-maxErrors))
			break
		}
		if i > 0 {
			msg.WriteString("\n")
		}
		msg.WriteString(fmt.Sprintf("%d. %v", i+1, err))
	}
	
	return fmt.Errorf("%s", msg.String())
}

// formatErrorWithInfo formats an error with complete error information.
// This is a temporary stub that provides basic formatting.
// This will be replaced with DI container access.
// Deprecated: Use DI container instead.
// Requirements: 15.1, 15.2, 15.3, 15.4, 15.8
func formatErrorWithInfo(err error, code string) error {
	if err == nil {
		return nil
	}
	
	// Provide basic error info for known codes
	switch code {
	case "E1001":
		return fmt.Errorf(`Error: OpenStack region not configured (%s)

The OpenStack provider requires a region to be specified.

Fix: Add region to your configuration:
  openCenter cluster update my-cluster \
    --opencenter.infrastructure.cloud.openstack.region=RegionOne

Hint: List available regions:
  openstack region list

Learn more: https://docs.opencenter.cloud/errors/E1001

Original error: %w`, code, err)
	default:
		return formatErrorWithCode(err, code)
	}
}
