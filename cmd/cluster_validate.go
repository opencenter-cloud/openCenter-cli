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
	"path/filepath"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// newClusterValidateCmd creates the command for validating a cluster's configuration.
//
// This command loads a cluster's configuration and runs a series of validation
// checks defined in the `config.Validate` function. If any validation rules
// are violated, it prints the errors to standard error and exits with a non-zero
// status code. If the configuration is valid, it prints a success message to
// standard output.
//
// Returns:
//   - *cobra.Command: A pointer to the configured `validate` command.
func newClusterValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [name]",
		Short: "Validate cluster configuration invariants and optionally generate complete config",
		Long: `Validate cluster configuration against schema and business rules.

This command performs comprehensive validation of cluster configuration including:
  • Schema validation against JSON schema
  • Required field validation
  • Cross-field dependency validation
  • Cloud provider credential validation
  • Network configuration validation
  • SOPS key validation

If no cluster name is provided, validates the currently active cluster.

Validation Checks:
  • Cluster name format and uniqueness
  • Kubernetes version compatibility
  • Network CIDR conflicts
  • SSH key format
  • Cloud provider credentials
  • SOPS encryption key availability
  • GitOps repository configuration

Troubleshooting:
  • Check error messages for specific validation failures
  • Use --generate-debug-config to save complete configuration
  • Verify cloud provider credentials are set correctly
  • Ensure SOPS key file exists and is readable`,
		Example: `  # Validate active cluster
  openCenter cluster validate

  # Validate specific cluster
  openCenter cluster validate my-cluster

  # Validate and generate debug config
  openCenter cluster validate my-cluster --generate-debug-config

  # Validate and save debug config to specific directory
  openCenter cluster validate my-cluster --generate-debug-config --output-dir=/tmp`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				var err error
				name, err = config.GetActive()
				if err != nil {
					return err
				}
				if name == "" {
					return fmt.Errorf("no active cluster; specify name")
				}
			}
			cfg, err := config.Load(name)
			if err != nil {
				return err
			}
			errs := config.Validate(cfg)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(cmd.ErrOrStderr(), e)
				}
				return fmt.Errorf("validation failed")
			}

			// Generate debug config if requested or if OPENCENTER_DEBUG environment variable exists
			generateDebug, _ := cmd.Flags().GetBool("generate-debug-config")
			if generateDebug || os.Getenv("OPENCENTER_DEBUG") != "" {
				// Determine output directory
				outputDir, _ := cmd.Flags().GetString("output-dir")
				if outputDir == "" {
					// Use GitOps directory if available, otherwise current directory
					if cfg.GitOps().GitDir != "" {
						outputDir = cfg.GitOps().GitDir
					} else {
						outputDir = "."
					}
				}

				if err := config.SaveDebugConfig(cfg.ClusterName(), outputDir); err != nil {
					return fmt.Errorf("failed to save debug config: %w", err)
				}
				debugPath := filepath.Join(outputDir, ".openCenter.yaml")
				fmt.Fprintf(cmd.OutOrStdout(), "Debug config saved to %s\n", debugPath)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Validation successful.")
			return nil
		},
	}

	cmd.Flags().Bool("generate-debug-config", false, "generate complete openCenter.yaml config for debugging")
	cmd.Flags().String("output-dir", "", "directory to save debug config (defaults to GitOps directory or current directory)")

	return cmd
}
