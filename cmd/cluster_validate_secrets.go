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
	"encoding/json"
	"fmt"
	"os"

	"github.com/rackerlabs/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// newClusterValidateSecretsCmd creates the command for validating secrets drift.
func newClusterValidateSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-secrets [cluster]",
		Short: "Validate secrets for configuration drift",
		Long: `Validate secrets by comparing config file against encrypted manifests.

This command detects configuration drift between the cluster's config file
(.k8s-<cluster>-config.yaml) and the deployed encrypted manifests. It identifies:

  • Secrets that differ between config and manifests (drift)
  • Secrets in config but missing from manifests
  • Secrets in manifests but not in config (orphaned)
  • Unencrypted secrets in manifests (security violations)

The validation returns exit code 0 if no drift is detected, or exit code 1 if
drift exists. This makes it suitable for CI/CD pipelines.

If no cluster name is provided, uses the currently active cluster.`,
		Example: `  # Validate secrets for active cluster
  opencenter cluster validate-secrets

  # Validate secrets for specific cluster
  opencenter cluster validate-secrets my-cluster

  # Auto-fix detected drift
  opencenter cluster validate-secrets my-cluster --fix

  # Output in JSON format for CI/CD
  opencenter cluster validate-secrets my-cluster --output json`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterValidateSecrets,
	}

	cmd.Flags().Bool("fix", false, "Automatically fix drift by running sync-secrets")
	cmd.Flags().String("output", "text", "Output format: text or json")

	return cmd
}

func runClusterValidateSecrets(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get flags
	fix, _ := cmd.Flags().GetBool("fix")
	outputFormat, _ := cmd.Flags().GetString("output")

	// Validate output format
	if outputFormat != "text" && outputFormat != "json" {
		return fmt.Errorf("invalid output format: %s (must be 'text' or 'json')", outputFormat)
	}

	// Resolve cluster name
	clusterName, err := resolveClusterName(args, true)
	if err != nil {
		return err
	}

	// Initialize secrets manager
	secretsManager, err := initializeSecretsManager()
	if err != nil {
		return fmt.Errorf("failed to initialize secrets manager: %w", err)
	}

	// Build validation options
	opts := secrets.ValidateOptions{
		Cluster:    clusterName,
		Fix:        fix,
		OutputJSON: outputFormat == "json",
	}

	// Execute validation
	result, err := secretsManager.ValidateSecrets(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to validate secrets: %w", err)
	}

	// Display results
	if outputFormat == "json" {
		displayValidationResultJSON(cmd, result)
	} else {
		displayValidationResultText(cmd, clusterName, result)
	}

	// Return appropriate exit code
	if !result.Valid {
		os.Exit(result.ExitCode)
	}

	return nil
}

// displayValidationResultText formats and displays the validation result in text format
func displayValidationResultText(cmd *cobra.Command, clusterName string, result *secrets.ValidationResult) {
	if result.Valid {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Secrets validation passed for cluster %s\n", clusterName)
		fmt.Fprintln(cmd.OutOrStdout(), "\nNo drift detected. Config and manifests are in sync.")
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✗ Secrets validation failed for cluster %s\n\n", clusterName)

	// Display drift items
	if len(result.DriftItems) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Configuration Drift (%d):\n", len(result.DriftItems))
		for _, item := range result.DriftItems {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s: %s\n", item.Service, item.FieldPath)
			fmt.Fprintf(cmd.OutOrStdout(), "    Config hash:   %s\n", item.ConfigHash)
			fmt.Fprintf(cmd.OutOrStdout(), "    Manifest hash: %s\n", item.ManifestHash)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display missing manifests
	if len(result.MissingManifests) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Missing Manifests (%d):\n", len(result.MissingManifests))
		for _, path := range result.MissingManifests {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", path)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display orphaned secrets
	if len(result.OrphanedSecrets) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Orphaned Secrets (%d):\n", len(result.OrphanedSecrets))
		for _, path := range result.OrphanedSecrets {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", path)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display security issues
	if len(result.SecurityIssues) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Security Violations (%d):\n", len(result.SecurityIssues))
		for _, issue := range result.SecurityIssues {
			fmt.Fprintf(cmd.ErrOrStderr(), "  • [%s] %s: %s\n", issue.Severity, issue.FilePath, issue.FieldPath)
		}
		fmt.Fprintln(cmd.ErrOrStderr())
	}

	// Display summary
	fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Drift items: %d\n", len(result.DriftItems))
	fmt.Fprintf(cmd.OutOrStdout(), "  Missing manifests: %d\n", len(result.MissingManifests))
	fmt.Fprintf(cmd.OutOrStdout(), "  Orphaned secrets: %d\n", len(result.OrphanedSecrets))
	fmt.Fprintf(cmd.OutOrStdout(), "  Security issues: %d\n", len(result.SecurityIssues))
	fmt.Fprintln(cmd.OutOrStdout())

	if result.ExitCode == 1 {
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'opencenter cluster sync-secrets' to fix drift, or use --fix flag.")
	}
}

// displayValidationResultJSON formats and displays the validation result in JSON format
func displayValidationResultJSON(cmd *cobra.Command, result *secrets.ValidationResult) {
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(output))
}
