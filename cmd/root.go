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

    "github.com/spf13/cobra"

    "github.com/rackerlabs/openCenter/internal/plugins"
)

var rootCmd = &cobra.Command{
    Use:   "openCenter",
    Short: "openCenter CLI manages cluster configurations and GitOps scaffolding",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Handle config-dir flag
        if cfgDir, _ := cmd.Flags().GetString("config-dir"); cfgDir != "" {
            // Set environment variable so that config.ResolveConfigDir picks it up
            if err := os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir); err != nil {
                return err
            }
        }
        return nil
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
    // Define global persistent flag for config-dir early so we can pre-parse it
    rootCmd.PersistentFlags().String("config-dir", "", "configuration directory (defaults to ~/.config/openCenter on Linux/macOS)")

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
    rootCmd.AddCommand(newSecretsCmd())
    rootCmd.AddCommand(newPluginsCmd())
    // Discover and attach external plugins as subcommands
    plugins.LoadExternalPlugins(rootCmd)
    return rootCmd.Execute()
}

// helpers for printing errors. In Cobra commands, returning an error
// will cause it to be printed and the process to exit with a non-zero
// code. Use fmt.Errorf to wrap underlying errors.
func failf(format string, a ...interface{}) error {
    return fmt.Errorf(format, a...)
}
