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

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/config/defaults"
	v2 "github.com/rackerlabs/opencenter-cli/internal/config/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
  opencenter cluster validate

  # Validate specific cluster
  opencenter cluster validate my-cluster

  # Validate and generate debug config
  opencenter cluster validate my-cluster --generate-debug-config

  # Validate and save debug config to specific directory
  opencenter cluster validate my-cluster --generate-debug-config --output-dir=/tmp`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if a configuration file was provided via --config flag
			configFile, _ := cmd.Flags().GetString("config")
			
			if configFile != "" {
				// Validate the provided config file directly
				return validateConfigFile(cmd, configFile)
			}

			// Resolve cluster name from args or active cluster
			name, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			// Get configuration file path
			configPath, err := config.ConfigPath(name)
			if err != nil {
				return fmt.Errorf("failed to resolve configuration path: %w", err)
			}

			// Detect schema version
			// Requirements: 13.2
			versionInfo, err := config.DetectSchemaVersionFromFile(configPath)
			if err != nil {
				return fmt.Errorf("failed to detect schema version: %w", err)
			}

			// Route to appropriate validator based on version
			// Requirements: 13.2
			if versionInfo.IsV2 {
				return validateV2Config(cmd, configPath, name)
			}

			// Default to v1 validation
			return validateV1Config(cmd, name)
		},
	}

	cmd.Flags().Bool("generate-debug-config", false, "generate complete opencenter.yaml config for debugging")
	cmd.Flags().String("output-dir", "", "directory to save debug config (defaults to GitOps directory or current directory)")

	return cmd
}

// validateConfigFile validates a configuration file directly (used with --config flag).
// Requirements: 13.2
func validateConfigFile(cmd *cobra.Command, configPath string) error {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("configuration file not found: %s", configPath)
	}

	// Detect schema version
	versionInfo, err := config.DetectSchemaVersionFromFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to detect schema version: %w", err)
	}

	// Extract cluster name from file path for context
	clusterName := filepath.Base(configPath)
	clusterName = clusterName[:len(clusterName)-len(filepath.Ext(clusterName))]

	// Route to appropriate validator based on version
	if versionInfo.IsV2 {
		return validateV2Config(cmd, configPath, clusterName)
	}

	// For v1, we need to load the config differently since it expects a cluster name
	// Load the config directly from the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse the config to extract cluster name
	var tempConfig struct {
		OpenCenter struct {
			Cluster struct {
				ClusterName string `yaml:"cluster_name"`
			} `yaml:"cluster"`
		} `yaml:"opencenter"`
	}
	
	if err := yaml.Unmarshal(data, &tempConfig); err != nil {
		return fmt.Errorf("failed to parse configuration: %w", err)
	}

	if tempConfig.OpenCenter.Cluster.ClusterName == "" {
		return fmt.Errorf("cluster name not found in configuration")
	}

	// Now validate using the extracted cluster name
	// For v1, we'll use a simplified validation that doesn't require the config to be in the standard location
	fmt.Fprintf(cmd.OutOrStdout(), "Validating v1 configuration from file: %s\n", configPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Note: Full validation requires the configuration to be in the standard location.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Use 'opencenter cluster init' to create a properly structured configuration.\n")
	
	return nil
}

// validateV1Config validates a v1 configuration.
// Requirements: 13.2
func validateV1Config(cmd *cobra.Command, name string) error {
	cfg, err := config.Load(name)
	if err != nil {
		return err
	}

	// Use comprehensive validator for thorough validation including service secrets
	configValidator := config.NewConfigValidator(false)
	result := configValidator.Validate(cmd.Context(), &cfg)

	if !result.Valid {
		// Display validation errors with field paths
		// Requirements: 11.7
		fmt.Fprintln(cmd.ErrOrStderr(), "Validation failed with the following errors:")
		for _, e := range result.Errors {
			// Format: [error_type] field_path: message
			if e.Field != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "[%s] %s: %s\n", e.Type, e.Field, e.Message)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "[%s] %s\n", e.Type, e.Message)
			}
			
			// Display suggestions if available
			if len(e.Suggestions) > 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "  Suggestions:")
				for _, suggestion := range e.Suggestions {
					fmt.Fprintf(cmd.ErrOrStderr(), "    - %s\n", suggestion)
				}
			}
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
		debugPath := filepath.Join(outputDir, ".opencenter.yaml")
		fmt.Fprintf(cmd.OutOrStdout(), "Debug config saved to %s\n", debugPath)
	}

	// Update stage and status
	if err := config.UpdateStatus(name, config.StageValidate, config.StatusSuccess); err != nil {
		// Don't fail the command if status update fails, just warn
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ Validation successful (v1 schema)")
	return nil
}

// validateV2Config validates a v2 configuration.
// Requirements: 13.2, 11.7
func validateV2Config(cmd *cobra.Command, configPath, name string) error {
	// Create v2 loader with default registry
	registry := defaults.NewRegistry()
	loader := v2.NewConfigLoader(registry)

	// Load and validate v2 configuration
	cfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		// Parse error to extract validation details
		// Requirements: 11.7
		fmt.Fprintln(cmd.ErrOrStderr(), "Validation failed with the following errors:")
		fmt.Fprintf(cmd.ErrOrStderr(), "[validation] %s\n", err.Error())
		return fmt.Errorf("validation failed")
	}

	// Generate debug config if requested
	generateDebug, _ := cmd.Flags().GetBool("generate-debug-config")
	if generateDebug || os.Getenv("OPENCENTER_DEBUG") != "" {
		// Determine output directory
		outputDir, _ := cmd.Flags().GetString("output-dir")
		if outputDir == "" {
			outputDir = "."
		}

		// Export effective configuration with applied defaults
		effectiveConfig, err := loader.ExportEffectiveConfig(cfg)
		if err != nil {
			return fmt.Errorf("failed to export effective config: %w", err)
		}

		debugPath := filepath.Join(outputDir, ".opencenter-v2.yaml")
		if err := os.WriteFile(debugPath, effectiveConfig, 0600); err != nil {
			return fmt.Errorf("failed to save debug config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Debug config saved to %s\n", debugPath)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ Validation successful (v2 schema)")
	return nil
}
