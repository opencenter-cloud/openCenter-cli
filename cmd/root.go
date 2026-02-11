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
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/di"
	"github.com/rackerlabs/opencenter-cli/internal/plugins"
)

// ContainerKey is the context key for the DI container
type contextKey string

const ContainerKey contextKey = "container"

var (
	globalContainer di.Container
	containerOnce   sync.Once
)

// getContainer returns the global DI container, initializing it if necessary
func getContainer() di.Container {
	containerOnce.Do(func() {
		globalContainer = initializeContainer()
	})
	return globalContainer
}

// initializeContainer creates and initializes the DI container with all services
func initializeContainer() di.Container {
	// Get base directory for path resolver
	// Use default config directory: ~/.config/opencenter on Linux/macOS
	baseDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if baseDir == "" {
		if runtime.GOOS == "windows" {
			base := os.Getenv("APPDATA")
			if base == "" {
				base = os.Getenv("LOCALAPPDATA")
			}
			if base == "" {
				base = os.Getenv("USERPROFILE")
			}
			baseDir = filepath.Join(base, "opencenter", "clusters")
		} else {
			home, _ := os.UserHomeDir()
			baseDir = filepath.Join(home, ".config", "opencenter", "clusters")
		}
	} else {
		baseDir = filepath.Join(baseDir, "clusters")
	}

	// Use the unified SetupContainer function
	// Requirements: 5.6
	container, err := di.SetupContainer(baseDir)
	if err != nil {
		// Log error and return a basic container
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize DI container: %v\n", err)
		container = di.NewContainer()
	}

	// Register domain services (these are not yet in SetupContainer)
	_ = container.Singleton("PathResolver", func() (*paths.PathResolver, error) {
		return di.ProvidePathResolver(baseDir)
	})

	_ = container.Singleton("ConfigManager", func() (*config.ConfigManager, error) {
		return di.ProvideConfigManager()
	})

	_ = container.Singleton("ValidationEngine", func() (*validation.ValidationEngine, error) {
		return di.ProvideValidationEngine()
	})

	_ = container.Singleton("InitService", di.ProvideInitService)
	_ = container.Singleton("ValidateService", di.ProvideValidateService)
	_ = container.Singleton("SetupService", di.ProvideSetupService)
	_ = container.Singleton("BootstrapService", di.ProvideBootstrapService)

	// Initialize all singletons
	_ = container.Initialize()

	return container
}

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
	ShowActive bool     // --show-active: display the current active cluster
}

var rootCmd = &cobra.Command{
	Use:   "opencenter",
	Short: "opencenter CLI manages cluster configurations and GitOps scaffolding",
	Long: `opencenter is a command-line tool for managing Kubernetes cluster configurations
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
Support: https://github.com/rackerlabs/opencenter-cli/issues`,
	Example: `  # Initialize a new cluster configuration
  opencenter cluster init my-cluster

  # Validate cluster configuration
  opencenter cluster validate my-cluster

  # Generate and view JSON schema
  opencenter cluster schema --pretty

  # List all clusters
  opencenter cluster list

  # Bootstrap a cluster with GitOps
  opencenter cluster bootstrap my-cluster`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Parse and apply global flags for all commands
		globalFlags, err := parseGlobalFlags(cmd)
		if err != nil {
			return fmt.Errorf("failed to parse global flags: %w", err)
		}

		// Apply log level if specified
		if globalFlags.LogLevel != "" {
			if err := config.SetLogLevel(globalFlags.LogLevel); err != nil {
				return fmt.Errorf("failed to set log level: %w", err)
			}
		}

		// Log environment and configuration paths for debugging
		config.Debug("=== OpenCenter CLI Debug Information ===")
		config.Debugf("Command: %s", cmd.CommandPath())
		config.Debugf("Arguments: %v", args)
		
		// Log environment variables
		config.Debug("Environment Variables:")
		if configDir := os.Getenv("OPENCENTER_CONFIG_DIR"); configDir != "" {
			config.Debugf("  OPENCENTER_CONFIG_DIR: %s", configDir)
		} else {
			config.Debug("  OPENCENTER_CONFIG_DIR: (not set)")
		}
		if home, err := os.UserHomeDir(); err == nil {
			config.Debugf("  HOME: %s", home)
		}
		
		// Log configuration paths
		config.Debug("Configuration Paths:")
		if runtime.GOOS == "windows" {
			if appData := os.Getenv("APPDATA"); appData != "" {
				config.Debugf("  APPDATA: %s", appData)
			}
			if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
				config.Debugf("  LOCALAPPDATA: %s", localAppData)
			}
		}
		
		// Log computed base directory
		baseDir := os.Getenv("OPENCENTER_CONFIG_DIR")
		if baseDir == "" {
			if runtime.GOOS == "windows" {
				base := os.Getenv("APPDATA")
				if base == "" {
					base = os.Getenv("LOCALAPPDATA")
				}
				if base == "" {
					base = os.Getenv("USERPROFILE")
				}
				baseDir = filepath.Join(base, "opencenter", "clusters")
			} else {
				home, _ := os.UserHomeDir()
				baseDir = filepath.Join(home, ".config", "opencenter", "clusters")
			}
		} else {
			baseDir = filepath.Join(baseDir, "clusters")
		}
		config.Debugf("  Clusters Directory: %s", baseDir)
		
		// Log global flags
		config.Debug("Global Flags:")
		config.Debugf("  --log-level: %s", globalFlags.LogLevel)
		config.Debugf("  --dry-run: %v", globalFlags.DryRun)
		config.Debugf("  --config: %s", globalFlags.Config)
		if len(globalFlags.Set) > 0 {
			config.Debugf("  --set: %v", globalFlags.Set)
		}
		config.Debug("========================================")

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
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

	// Initialize DI container and add to context
	container := getContainer()
	ctx = context.WithValue(ctx, ContainerKey, container)

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
	rootCmd.AddCommand(newShellInitCmd())
	// Discover and attach external plugins as subcommands
	plugins.LoadExternalPlugins(rootCmd)

	// Execute with context
	return rootCmd.ExecuteContext(ctx)
}

// addGlobalFlags adds global persistent flags to the root command.
func addGlobalFlags(cmd *cobra.Command) {
	// Legacy config-dir flag (kept for backward compatibility)
	cmd.PersistentFlags().String("config-dir", "", "configuration directory (defaults to ~/.config/opencenter on Linux/macOS)")

	// New global flags
	cmd.PersistentFlags().String("config", "", "alternative cluster configuration file path")
	cmd.PersistentFlags().Bool("dry-run", false, "enable dry-run mode to print planned actions without executing them")
	cmd.PersistentFlags().String("log-level", "warn", "set log level explicitly (debug, info, warn, error)")
	cmd.PersistentFlags().StringArray("set", []string{}, "override configuration values using dot notation (e.g., --set spec.provider=openstack)")
	cmd.PersistentFlags().Bool("show-active", false, "display the current active cluster")
}

// parseGlobalFlags extracts global flags from the command.
func parseGlobalFlags(cmd *cobra.Command) (*GlobalFlags, error) {
	config, _ := cmd.Flags().GetString("config")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	logLevel, _ := cmd.Flags().GetString("log-level")
	set, _ := cmd.Flags().GetStringArray("set")
	showActive, _ := cmd.Flags().GetBool("show-active")

	return &GlobalFlags{
		Config:     config,
		DryRun:     dryRun,
		LogLevel:   logLevel,
		Set:        set,
		ShowActive: showActive,
	}, nil
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
