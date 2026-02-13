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

	"github.com/rackerlabs/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// newClusterCheckKeysCmd creates the command for checking key expiration status.
func newClusterCheckKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-keys",
		Short: "Check encryption key expiration status",
		Long: `Check the expiration status of encryption keys across clusters.

This command monitors Age and SSH key expiration to help maintain security
through proactive key rotation. It displays:

  • Days until expiration for each key
  • Warning indicators for keys expiring within threshold (default: 14 days)
  • Error indicators for expired keys
  • Key metadata (cluster, type, fingerprint, creation date)

Keys are tracked in the key registry with default expiration policies:
  • Age keys: 90 days
  • SSH keys: 180 days

Use --all to check all clusters, or --cluster to check a specific cluster.
The --warn-days flag controls the warning threshold.`,
		Example: `  # Check keys for all clusters
  opencenter cluster check-keys --all

  # Check keys for specific cluster
  opencenter cluster check-keys --cluster my-cluster

  # Check with custom warning threshold (30 days)
  opencenter cluster check-keys --all --warn-days 30

  # Output in JSON format for automation
  opencenter cluster check-keys --all --output json`,
		RunE: runClusterCheckKeys,
	}

	cmd.Flags().Bool("all", false, "Check keys for all clusters")
	cmd.Flags().String("cluster", "", "Check keys for specific cluster")
	cmd.Flags().String("output", "text", "Output format: text or json")
	cmd.Flags().Int("warn-days", 14, "Warning threshold in days")

	return cmd
}

func runClusterCheckKeys(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get flags
	all, _ := cmd.Flags().GetBool("all")
	clusterName, _ := cmd.Flags().GetString("cluster")
	outputFormat, _ := cmd.Flags().GetString("output")
	warnDays, _ := cmd.Flags().GetInt("warn-days")

	// Validate output format
	if outputFormat != "text" && outputFormat != "json" {
		return fmt.Errorf("invalid output format: %s (must be 'text' or 'json')", outputFormat)
	}

	// Validate flags
	if !all && clusterName == "" {
		return fmt.Errorf("must specify either --all or --cluster")
	}

	if all && clusterName != "" {
		return fmt.Errorf("cannot specify both --all and --cluster")
	}

	// Initialize key registry
	registry, err := initializeKeyRegistry()
	if err != nil {
		return fmt.Errorf("failed to initialize key registry: %w", err)
	}

	// Check expiration
	report, err := registry.CheckExpiration(ctx, warnDays)
	if err != nil {
		return fmt.Errorf("failed to check key expiration: %w", err)
	}

	// Filter by cluster if specified
	if clusterName != "" {
		report = filterReportByCluster(report, clusterName)
	}

	// Display results
	if outputFormat == "json" {
		displayExpirationReportJSON(cmd, report)
	} else {
		displayExpirationReportText(cmd, report, warnDays)
	}

	return nil
}

// filterReportByCluster filters an expiration report to a specific cluster
func filterReportByCluster(report *secrets.ExpirationReport, cluster string) *secrets.ExpirationReport {
	filtered := &secrets.ExpirationReport{
		Expired: []secrets.KeyExpirationInfo{},
		Warning: []secrets.KeyExpirationInfo{},
		Valid:   []secrets.KeyExpirationInfo{},
	}

	for _, info := range report.Expired {
		if info.Cluster == cluster {
			filtered.Expired = append(filtered.Expired, info)
		}
	}

	for _, info := range report.Warning {
		if info.Cluster == cluster {
			filtered.Warning = append(filtered.Warning, info)
		}
	}

	for _, info := range report.Valid {
		if info.Cluster == cluster {
			filtered.Valid = append(filtered.Valid, info)
		}
	}

	return filtered
}

// displayExpirationReportText formats and displays the expiration report in text format
func displayExpirationReportText(cmd *cobra.Command, report *secrets.ExpirationReport, warnDays int) {
	totalKeys := len(report.Expired) + len(report.Warning) + len(report.Valid)

	fmt.Fprintf(cmd.OutOrStdout(), "Key Expiration Status (%d keys)\n\n", totalKeys)

	// Display expired keys
	if len(report.Expired) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "❌ EXPIRED KEYS (%d) - Immediate rotation required:\n", len(report.Expired))
		for _, info := range report.Expired {
			fmt.Fprintf(cmd.ErrOrStderr(), "  • %s (%s): expired %d days ago\n",
				info.Cluster,
				info.KeyType,
				-info.DaysRemaining)
			fmt.Fprintf(cmd.ErrOrStderr(), "    Fingerprint: %s\n", info.Fingerprint)
			fmt.Fprintf(cmd.ErrOrStderr(), "    Expired: %s\n", info.ExpiresAt.Format("2006-01-02"))
		}
		fmt.Fprintln(cmd.ErrOrStderr())
	}

	// Display warning keys
	if len(report.Warning) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠️  EXPIRING SOON (%d) - Rotation recommended within %d days:\n", len(report.Warning), warnDays)
		for _, info := range report.Warning {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s (%s): %d days remaining\n",
				info.Cluster,
				info.KeyType,
				info.DaysRemaining)
			fmt.Fprintf(cmd.OutOrStdout(), "    Fingerprint: %s\n", info.Fingerprint)
			fmt.Fprintf(cmd.OutOrStdout(), "    Expires: %s\n", info.ExpiresAt.Format("2006-01-02"))
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display valid keys
	if len(report.Valid) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ VALID KEYS (%d):\n", len(report.Valid))
		for _, info := range report.Valid {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s (%s): %d days remaining\n",
				info.Cluster,
				info.KeyType,
				info.DaysRemaining)
			fmt.Fprintf(cmd.OutOrStdout(), "    Fingerprint: %s\n", info.Fingerprint)
			fmt.Fprintf(cmd.OutOrStdout(), "    Expires: %s\n", info.ExpiresAt.Format("2006-01-02"))
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display summary
	fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Total keys: %d\n", totalKeys)
	fmt.Fprintf(cmd.OutOrStdout(), "  Expired: %d\n", len(report.Expired))
	fmt.Fprintf(cmd.OutOrStdout(), "  Expiring soon: %d\n", len(report.Warning))
	fmt.Fprintf(cmd.OutOrStdout(), "  Valid: %d\n", len(report.Valid))

	// Display recommendations
	if len(report.Expired) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nRecommendations:")
		fmt.Fprintln(cmd.OutOrStdout(), "  Rotate expired keys immediately:")
		for _, info := range report.Expired {
			fmt.Fprintf(cmd.OutOrStdout(), "    opencenter cluster rotate-keys %s --type %s\n", info.Cluster, info.KeyType)
		}
	} else if len(report.Warning) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nRecommendations:")
		fmt.Fprintln(cmd.OutOrStdout(), "  Schedule rotation for expiring keys:")
		for _, info := range report.Warning {
			fmt.Fprintf(cmd.OutOrStdout(), "    opencenter cluster rotate-keys %s --type %s\n", info.Cluster, info.KeyType)
		}
	}
}

// displayExpirationReportJSON formats and displays the expiration report in JSON format
func displayExpirationReportJSON(cmd *cobra.Command, report *secrets.ExpirationReport) {
	output, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(output))
}

// initializeKeyRegistry creates and configures a key registry instance
func initializeKeyRegistry() (secrets.KeyRegistry, error) {
	logger := createSecretsLogger()
	sopsManager := createSOPSManager(logger)

	// Get registry path
	registryPath, err := getSecretsRegistryPath()
	if err != nil {
		return nil, err
	}

	// Create SOPS encryptor adapter
	encryptor := &sopsEncryptorAdapter{manager: sopsManager}

	// Create key registry
	registry := secrets.NewDefaultKeyRegistry(registryPath, encryptor, logger)

	return registry, nil
}
