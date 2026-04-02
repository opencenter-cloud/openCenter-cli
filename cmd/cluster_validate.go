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

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/spf13/cobra"
)

// newClusterValidateCmd creates the command for validating a cluster's configuration.
func newClusterValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [name]",
		Short: "Validate cluster configuration",
		Long: `Validate cluster configuration against schema and business rules.

This command performs comprehensive validation including:
  • Schema validation against JSON schema
  • Required field validation
  • Cross-field dependency validation
  • Cloud provider credential validation (optional)
  • Network configuration validation
  • SOPS key validation

Only v2 configurations (schema_version: "2.0") are supported.
Configurations with any other schema version are invalid.

If no cluster name is provided, validates the currently active cluster.`,
		Example: `  # Validate active cluster
  opencenter cluster validate

  # Validate specific cluster
  opencenter cluster validate my-cluster

  # Validate with organization/cluster-name format
  opencenter cluster validate my-org/my-cluster

  # Validate with connectivity checks
  opencenter cluster validate my-cluster --check-connectivity

  # Output as JSON (for CI/CD pipelines)
  opencenter cluster validate my-cluster --json

  # Validate and generate debug config
  opencenter cluster validate my-cluster --generate-debug-config`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := GetApp(cmd.Context())
			if err != nil {
				return err
			}
			validateService := app.ValidateService

			// Check if a configuration file was provided via --config flag
			configFile, _ := cmd.Flags().GetString("config")

			// Resolve cluster name, organization, and provider from args or active cluster
			var clusterName string
			var organization string
			var provider string
			if configFile == "" {
				// Determine identifier from args or active cluster
				var identifier string
				if len(args) > 0 {
					identifier = args[0]
				} else {
					// No args provided, use active cluster
					identifier, err = getActiveCluster()
					if err != nil || identifier == "" {
						return fmt.Errorf("no cluster name provided and no active cluster set")
					}
				}

				// Use loadConfigWithIdentifier to support organization/cluster-name format
				var cfg config.Config
				cfg, clusterName, organization, err = loadConfigWithIdentifier(cmd.Context(), identifier)
				if err != nil {
					return err
				}
				provider = cfg.OpenCenter.Infrastructure.Provider
			} else {
				// Get organization from global flag if using --config
				organization, _ = cmd.Flags().GetString("organization")
			}

			// Get validation options from flags
			checkConnectivity, _ := cmd.Flags().GetBool("check-connectivity")
			checkProvider, _ := cmd.Flags().GetBool("check-provider")
			generateDebug, _ := cmd.Flags().GetBool("generate-debug-config")
			outputDir, _ := cmd.Flags().GetString("output-dir")
			verbose, _ := cmd.Flags().GetBool("verbose")
			jsonOutput, _ := cmd.Flags().GetBool("json")

			outputFormat := "text"
			if jsonOutput {
				outputFormat = "json"
			}

			// Build validation options
			opts := cluster.ValidateOptions{
				ClusterName:         clusterName,
				Organization:        organization,
				ConfigPath:          configFile,
				CheckConnectivity:   checkConnectivity,
				CheckProvider:       checkProvider,
				GenerateDebugConfig: generateDebug,
				OutputDir:           outputDir,
				Verbose:             verbose,
				OutputFormat:        outputFormat,
				Provider:            provider,
			}

			// Perform validation
			result, err := validateService.Validate(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("validation error: %w", err)
			}

			// Format and display result
			if outputFormat == "json" {
				jsonStr, err := validateService.FormatResultJSON(result, provider)
				if err != nil {
					return fmt.Errorf("formatting JSON output: %w", err)
				}
				fmt.Fprint(cmd.OutOrStdout(), jsonStr)
			} else {
				output := validateService.FormatResultGrouped(result, provider)
				fmt.Fprint(cmd.OutOrStdout(), output)
			}

			// Show debug config path if generated (text mode only)
			if outputFormat != "json" && result.DebugConfigPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDebug config saved to: %s\n", result.DebugConfigPath)
			}

			// Return error if validation failed, but silence usage
			if !result.Valid {
				cmd.SilenceUsage = true
				return fmt.Errorf("validation failed")
			}

			return nil
		},
	}

	cmd.Flags().Bool("check-connectivity", false, "check connectivity to cloud provider")
	cmd.Flags().Bool("check-provider", false, "perform provider-specific validation")
	cmd.Flags().Bool("generate-debug-config", false, "generate complete config for debugging")
	cmd.Flags().String("config", "", "path to configuration file to validate")
	cmd.Flags().String("output-dir", "", "directory to save debug config (defaults to current directory)")
	cmd.Flags().BoolP("verbose", "v", false, "verbose output")
	cmd.Flags().Bool("json", false, "output validation results as JSON (for CI/CD pipelines)")

	return cmd
}
