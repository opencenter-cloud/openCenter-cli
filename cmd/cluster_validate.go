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
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
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

  # Validate generated GitOps manifests
  opencenter cluster validate my-cluster --manifests

  # Output as JSON (for CI/CD pipelines)
  opencenter cluster validate my-cluster --output json

  # Validate and generate debug config
  opencenter cluster validate my-cluster --generate-debug-config`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			validateManifests, _ := cmd.Flags().GetBool("manifests")
			if validateManifests {
				return runClusterValidateManifests(cmd, args)
			}

			app, err := GetApp(cmd.Context())
			if err != nil {
				return err
			}
			validateService := app.ValidateService

			// Check if a configuration file was provided via --config-file flag.
			configFile := getValidateConfigFileFlag(cmd)

			// Resolve cluster identifier from args or active cluster. The validation
			// service owns storage resolution and config loading.
			var clusterName string
			var organization string
			if configFile == "" {
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

				clusterName = identifier
				if strings.Contains(identifier, "/") {
					parts := strings.SplitN(identifier, "/", 2)
					organization = parts[0]
					clusterName = parts[1]
				}
			}

			// Get validation options from flags
			checkConnectivity, _ := cmd.Flags().GetBool("check-connectivity")
			checkProvider, _ := cmd.Flags().GetBool("check-provider")
			generateDebug, _ := cmd.Flags().GetBool("generate-debug-config")
			outputDir, _ := cmd.Flags().GetString("output-dir")
			verbose, _ := cmd.Flags().GetBool("verbose")
			outputFormat := string(getGlobalOptions(cmd).Output)

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
			}

			// Perform validation
			result, err := validateService.Validate(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("validation error: %w", err)
			}

			// Format and display result
			if outputFormat == "json" {
				jsonStr, err := validateService.FormatResultJSON(result, result.Provider)
				if err != nil {
					return fmt.Errorf("formatting JSON output: %w", err)
				}
				fmt.Fprint(cmd.OutOrStdout(), jsonStr)
			} else {
				output := validateService.FormatResultGrouped(result, result.Provider)
				fmt.Fprint(cmd.OutOrStdout(), output)
			}

			// Show debug config path if generated (text mode only)
			if outputFormat != "json" && result.DebugConfigPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDebug config saved to: %s\n", result.DebugConfigPath)
			}

			// Return error if validation failed
			if !result.Valid {
				return fmt.Errorf("validation failed")
			}

			return nil
		},
	}

	cmd.Flags().Bool("check-connectivity", false, "check connectivity to cloud provider")
	cmd.Flags().Bool("check-provider", false, "perform provider-specific validation")
	cmd.Flags().Bool("generate-debug-config", false, "generate complete config for debugging")
	cmd.Flags().Bool("manifests", false, "validate generated GitOps manifests")
	cmd.Flags().String("config-file", "", "path to configuration file to validate")
	cmd.Flags().String("config", "", "path to configuration file to validate")
	_ = cmd.Flags().MarkHidden("config")
	cmd.Flags().String("output-dir", "", "directory to save debug config (defaults to current directory)")
	cmd.Flags().BoolP("verbose", "v", false, "verbose output")

	return cmd
}

func getValidateConfigFileFlag(cmd *cobra.Command) string {
	if configFile, _ := cmd.Flags().GetString("config-file"); configFile != "" {
		return configFile
	}
	configFile, _ := cmd.Flags().GetString("config")
	return configFile
}
