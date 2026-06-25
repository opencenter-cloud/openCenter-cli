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
	"sync"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
	"github.com/opencenter-cloud/opencenter-cli/internal/plugins"
)

// ContainerKey is the context key for the DI container
type contextKey string

const (
	AppKey       contextKey = "app"
	ContainerKey contextKey = "container"
)

var (
	globalContainer di.Container
	globalApp       *di.App
	containerOnce   sync.Once
)

// resetContainerForTests resets the lazy global DI container.
// Command tests call this after changing OPENCENTER_CONFIG_DIR so the container
// picks up the current path resolver configuration.
func resetContainerForTests() {
	globalContainer = nil
	globalApp = nil
	containerOnce = sync.Once{}
}

// getContainer returns the global application container, initializing it if necessary.
func getContainer() di.Container {
	if globalContainer != nil {
		return globalContainer
	}
	containerOnce.Do(func() {
		globalContainer = initializeContainer()
	})
	return globalContainer
}

func initializeApp() (*di.App, error) {
	return di.NewApp(config.ResolveClustersDir())
}

// initializeContainer creates the legacy service locator backed by the typed app graph.
func initializeContainer() di.Container {
	baseDir := config.ResolveClustersDir()

	app, err := di.NewApp(baseDir)
	if err == nil {
		globalApp = app
		return di.NewAppContainer(app)
	}

	// Typed wiring is the default path. Fall back only if startup wiring fails.
	fmt.Fprintf(os.Stderr, "Warning: Failed to initialize typed app graph: %v\n", err)
	container, legacyErr := di.SetupContainer(baseDir)
	if legacyErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize legacy DI container: %v\n", legacyErr)
		return di.NewContainer()
	}
	return container
}

// GetApp retrieves the typed application graph from the context.
func GetApp(ctx context.Context) (*di.App, error) {
	app, ok := ctx.Value(AppKey).(*di.App)
	if ok && app != nil {
		return app, nil
	}

	if globalApp != nil {
		return globalApp, nil
	}

	container := getContainer()
	if container != nil && globalApp != nil {
		return globalApp, nil
	}

	return nil, fmt.Errorf("application graph not found in context")
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
	Config     string   // legacy alternative cluster configuration file path
	DryRun     bool     // --dry-run: enable dry-run mode
	LogLevel   string   // --log-level: set log level explicitly
	Set        []string // legacy configuration overrides using dot notation
	ShowActive bool     // legacy active-cluster display toggle
	BreakLock  bool     // --break-lock: force removal of existing lock before operation
}

var rootCmd = &cobra.Command{
	Use:           "opencenter",
	Short:         "opencenter CLI manages cluster configurations and GitOps scaffolding",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `opencenter is a command-line tool for managing Kubernetes cluster configurations
and GitOps repositories. It provides a declarative approach to cluster lifecycle
management with built-in validation, secrets management, and multi-provider support.

Key Features:
  • Declarative YAML-based cluster configuration
  • Automatic GitOps repository scaffolding
  • SOPS integration for secrets management
  • Multi-cloud provider support (OpenStack, VMware, Kind, Baremetal)
  • Comprehensive validation and doctor checks
  • Organization-based multi-tenancy support

Documentation: https://docs.opencenter.cloud
Support: https://github.com/opencenter-cloud/opencenter-cli/issues`,
	Example: `  # Initialize a new cluster configuration
  opencenter cluster init my-cluster

  # Validate cluster configuration
  opencenter cluster validate my-cluster

  # List all clusters
  opencenter cluster list

  # Generate GitOps assets
  opencenter cluster generate my-cluster

  # Deploy a cluster
  opencenter cluster deploy my-cluster

  # Show the active cluster
  opencenter cluster active`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return applyGlobalOptions(cmd, args)
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

	// Pre-parse --config-dir from os.Args so plugin discovery can use it
	// before Cobra runs PersistentPreRunE or the DI container is initialized.
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

	app, err := initializeApp()
	if err != nil {
		return fmt.Errorf("initialize application graph: %w", err)
	}
	globalApp = app
	container := di.NewAppContainer(app)
	globalContainer = container
	containerOnce = sync.Once{}

	// Initialize app graph and add to context
	ctx = context.WithValue(ctx, AppKey, app)
	ctx = context.WithValue(ctx, ContainerKey, container)

	// Add global persistent flags
	addGlobalFlags(rootCmd)

	// Register subcommands
	rootCmd.AddCommand(NewClusterCmd())
	rootCmd.AddCommand(NewSettingsCmd())
	rootCmd.AddCommand(NewSecretsCmd())
	rootCmd.AddCommand(NewPluginsCmd())
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewShellInitCmd())
	// Discover and attach external plugins as subcommands
	plugins.LoadExternalPlugins(rootCmd)

	// Execute with context
	return rootCmd.ExecuteContext(ctx)
}

// addGlobalFlags adds global persistent flags to the root command.
func addGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("config-dir", "", "configuration directory (defaults to ~/.config/opencenter on Linux/macOS)")
	cmd.PersistentFlags().String("log-level", "warn", "set log level explicitly (debug, info, warn, error)")
	cmd.PersistentFlags().String("output", string(OutputText), "output format for supported commands: text, json, yaml")
	cmd.PersistentFlags().Bool("quiet", false, "suppress nonessential human output")
	cmd.PersistentFlags().Bool("yes", false, "answer yes to confirmation prompts")
	cmd.PersistentFlags().Bool("dry-run", false, "preview mutating operations without writing or acting")
}

// parseGlobalFlags extracts global flags from the command.

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
