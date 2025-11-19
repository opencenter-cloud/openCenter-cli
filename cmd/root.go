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
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/plugins"
)

// GlobalFlags represents the global flags available across all commands.
type GlobalFlags struct {
	Config   string   // --config: alternative cluster configuration file path
	DryRun   bool     // --dry-run: enable dry-run mode
	LogLevel string   // --log-level: set log level explicitly
	Set      []string // --set: override configuration values using dot notation
	Verbose  bool     // --verbose: enable verbose logging
}

// configManager holds the global configuration manager instance
var configManager *config.ConfigManager

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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeGlobalConfig(cmd)
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
	rootCmd.AddCommand(newClusterCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newSOPSCmd())
	rootCmd.AddCommand(newSecretsCmd())
	rootCmd.AddCommand(newPluginsCmd())
	rootCmd.AddCommand(newVersionCmd())
	// Discover and attach external plugins as subcommands
	plugins.LoadExternalPlugins(rootCmd)
	return rootCmd.Execute()
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
}

// parseGlobalFlags extracts global flags from the command.
func parseGlobalFlags(cmd *cobra.Command) (*GlobalFlags, error) {
	config, _ := cmd.Flags().GetString("config")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	logLevel, _ := cmd.Flags().GetString("log-level")
	set, _ := cmd.Flags().GetStringArray("set")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// If verbose is set, override log level to debug
	if verbose {
		logLevel = "debug"
	}

	return &GlobalFlags{
		Config:   config,
		DryRun:   dryRun,
		LogLevel: logLevel,
		Set:      set,
		Verbose:  verbose,
	}, nil
}

// initializeGlobalConfig initializes the global configuration manager and applies overrides.
func initializeGlobalConfig(cmd *cobra.Command) error {
	// Handle legacy config-dir flag
	if cfgDir, _ := cmd.Flags().GetString("config-dir"); cfgDir != "" {
		// Set environment variable so that config.ResolveConfigDir picks it up
		if err := os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir); err != nil {
			return err
		}
	}

	// Parse global flags
	globalFlags, err := parseGlobalFlags(cmd)
	if err != nil {
		return fmt.Errorf("failed to parse global flags: %w", err)
	}

	// Initialize configuration manager
	var configPath string
	if globalFlags.Config != "" {
		configPath = globalFlags.Config
	}

	configManager, err = config.NewConfigManager(configPath)
	if err != nil {
		return fmt.Errorf("failed to initialize configuration: %w", err)
	}

	// Apply global flag overrides
	if err := applyGlobalFlagOverrides(globalFlags); err != nil {
		return fmt.Errorf("failed to apply global flag overrides: %w", err)
	}

	// Log that configuration has been initialized
	config.Debug("Configuration initialized successfully")
	config.WithFields(logrus.Fields{
		"config_path": configManager.GetConfigPath(),
		"log_level":   configManager.GetConfig().Logging.Level,
		"log_format":  configManager.GetConfig().Logging.Format,
		"log_output":  configManager.GetConfig().Logging.Output,
	}).Debug("Configuration details")

	return nil
}

// applyGlobalFlagOverrides applies global flag overrides to the configuration.
func applyGlobalFlagOverrides(globalFlags *GlobalFlags) error {
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

// GetConfigManager returns the global configuration manager instance.
func GetConfigManager() *config.ConfigManager {
	return configManager
}

// helpers for printing errors. In Cobra commands, returning an error
// will cause it to be printed and the process to exit with a non-zero
// code. Use fmt.Errorf to wrap underlying errors.
func failf(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}
