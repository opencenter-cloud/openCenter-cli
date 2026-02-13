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
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// newClusterKeysCmd creates the parent command for key management operations.
func newClusterKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage encryption keys",
		Long: `Manage encryption keys for clusters.

This command provides subcommands for viewing and managing encryption keys
across clusters. Keys are tracked in the key registry with metadata including:

  • Cluster association
  • Key type (Age or SSH)
  • Fingerprint (unique identifier)
  • Creation and expiration dates
  • Status (active, archived, revoked)
  • Usage information

Use the subcommands to list keys, check expiration status, and manage
key lifecycle.`,
		Example: `  # List all keys
  opencenter cluster keys list

  # List keys for specific cluster
  opencenter cluster keys list --cluster my-cluster

  # List only active keys
  opencenter cluster keys list --status active

  # Output in JSON format
  opencenter cluster keys list --output json`,
	}

	// Add subcommands
	cmd.AddCommand(newClusterKeysListCmd())

	return cmd
}

// newClusterKeysListCmd creates the command for listing keys.
func newClusterKeysListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List encryption keys",
		Long: `List encryption keys from the key registry.

This command displays key metadata for all clusters or a specific cluster.
The key registry tracks:

  • Cluster: Which cluster the key belongs to
  • Type: Age (encryption) or SSH (access)
  • Fingerprint: Unique identifier for the key
  • Created: When the key was generated
  • Expires: When the key should be rotated
  • Status: active, archived, or revoked

Keys can be filtered by cluster and status. The output can be formatted
as human-readable text or JSON for automation.

Status meanings:
  • active: Currently in use for encryption/decryption
  • archived: Rotated out but kept for historical decryption
  • revoked: Explicitly revoked, no longer trusted`,
		Example: `  # List all keys
  opencenter cluster keys list

  # List keys for specific cluster
  opencenter cluster keys list --cluster my-cluster

  # List only active keys
  opencenter cluster keys list --status active

  # List only revoked keys
  opencenter cluster keys list --status revoked

  # Output in JSON format
  opencenter cluster keys list --output json

  # List archived keys for a cluster
  opencenter cluster keys list --cluster my-cluster --status archived`,
		RunE: runClusterKeysList,
	}

	cmd.Flags().String("cluster", "", "Filter to specific cluster")
	cmd.Flags().String("status", "", "Filter by status: active, archived, or revoked")
	cmd.Flags().String("output", "text", "Output format: text or json")

	return cmd
}

func runClusterKeysList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get flags
	clusterName, _ := cmd.Flags().GetString("cluster")
	statusFilter, _ := cmd.Flags().GetString("status")
	outputFormat, _ := cmd.Flags().GetString("output")

	// Validate output format
	if outputFormat != "text" && outputFormat != "json" {
		return fmt.Errorf("invalid output format: %s (must be 'text' or 'json')", outputFormat)
	}

	// Validate status filter
	if statusFilter != "" {
		validStatuses := map[string]bool{
			"active":   true,
			"archived": true,
			"revoked":  true,
		}
		if !validStatuses[statusFilter] {
			return fmt.Errorf("invalid status: %s (must be 'active', 'archived', or 'revoked')", statusFilter)
		}
	}

	// Initialize key registry
	registry, err := initializeKeyRegistry()
	if err != nil {
		return fmt.Errorf("failed to initialize key registry: %w", err)
	}

	// List keys
	keys, err := registry.ListKeys(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	// Filter by status if specified
	if statusFilter != "" {
		keys = filterKeysByStatus(keys, secrets.KeyStatus(statusFilter))
	}

	// Display results
	if outputFormat == "json" {
		displayKeysJSON(cmd, keys)
	} else {
		displayKeysText(cmd, keys, clusterName, statusFilter)
	}

	return nil
}

// filterKeysByStatus filters keys by status
func filterKeysByStatus(keys []secrets.KeyEntry, status secrets.KeyStatus) []secrets.KeyEntry {
	var filtered []secrets.KeyEntry
	for _, key := range keys {
		if key.Status == status {
			filtered = append(filtered, key)
		}
	}
	return filtered
}

// displayKeysText formats and displays keys in text format
func displayKeysText(cmd *cobra.Command, keys []secrets.KeyEntry, clusterFilter, statusFilter string) {
	if len(keys) == 0 {
		if clusterFilter != "" && statusFilter != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "No %s keys found for cluster %s\n", statusFilter, clusterFilter)
		} else if clusterFilter != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "No keys found for cluster %s\n", clusterFilter)
		} else if statusFilter != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "No %s keys found\n", statusFilter)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "No keys found in registry")
		}
		return
	}

	// Build title
	title := "Encryption Keys"
	if clusterFilter != "" {
		title += fmt.Sprintf(" for cluster %s", clusterFilter)
	}
	if statusFilter != "" {
		title += fmt.Sprintf(" (%s)", statusFilter)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%d keys)\n\n", title, len(keys))

	// Group keys by cluster
	keysByCluster := make(map[string][]secrets.KeyEntry)
	for _, key := range keys {
		keysByCluster[key.Cluster] = append(keysByCluster[key.Cluster], key)
	}

	// Display keys grouped by cluster
	for cluster, clusterKeys := range keysByCluster {
		fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", cluster)
		
		// Group by type
		ageKeys := []secrets.KeyEntry{}
		sshKeys := []secrets.KeyEntry{}
		for _, key := range clusterKeys {
			if key.KeyType == secrets.KeyTypeAge {
				ageKeys = append(ageKeys, key)
			} else {
				sshKeys = append(sshKeys, key)
			}
		}

		// Display Age keys
		if len(ageKeys) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  Age Keys:")
			for _, key := range ageKeys {
				displayKeyEntry(cmd, key, "    ")
			}
		}

		// Display SSH keys
		if len(sshKeys) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  SSH Keys:")
			for _, key := range sshKeys {
				displayKeyEntry(cmd, key, "    ")
			}
		}

		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display summary
	activeCount := 0
	archivedCount := 0
	revokedCount := 0
	for _, key := range keys {
		switch key.Status {
		case secrets.KeyStatusActive:
			activeCount++
		case secrets.KeyStatusArchived:
			archivedCount++
		case secrets.KeyStatusRevoked:
			revokedCount++
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Total keys: %d\n", len(keys))
	fmt.Fprintf(cmd.OutOrStdout(), "  Active: %d\n", activeCount)
	fmt.Fprintf(cmd.OutOrStdout(), "  Archived: %d\n", archivedCount)
	fmt.Fprintf(cmd.OutOrStdout(), "  Revoked: %d\n", revokedCount)
}

// displayKeyEntry displays a single key entry with indentation
func displayKeyEntry(cmd *cobra.Command, key secrets.KeyEntry, indent string) {
	// Status indicator
	statusIcon := ""
	switch key.Status {
	case secrets.KeyStatusActive:
		statusIcon = "✓"
	case secrets.KeyStatusArchived:
		statusIcon = "📦"
	case secrets.KeyStatusRevoked:
		statusIcon = "❌"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s%s [%s]\n", indent, statusIcon, key.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "%s  Fingerprint: %s\n", indent, key.Fingerprint)
	fmt.Fprintf(cmd.OutOrStdout(), "%s  Created: %s\n", indent, key.CreatedAt.Format("2006-01-02 15:04:05"))
	
	// Show expiration for active keys
	if key.Status == secrets.KeyStatusActive {
		daysRemaining := int(time.Until(key.ExpiresAt).Hours() / 24)
		if daysRemaining < 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Expires: %s (EXPIRED %d days ago)\n", 
				indent, key.ExpiresAt.Format("2006-01-02"), -daysRemaining)
		} else if daysRemaining < 14 {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Expires: %s (⚠️  %d days remaining)\n", 
				indent, key.ExpiresAt.Format("2006-01-02"), daysRemaining)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Expires: %s (%d days remaining)\n", 
				indent, key.ExpiresAt.Format("2006-01-02"), daysRemaining)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  Expires: %s\n", indent, key.ExpiresAt.Format("2006-01-02"))
	}

	// Show revocation details
	if key.Status == secrets.KeyStatusRevoked {
		if !key.RevokedAt.IsZero() {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Revoked: %s\n", indent, key.RevokedAt.Format("2006-01-02 15:04:05"))
		}
		if key.RevokedBy != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Revoked by: %s\n", indent, key.RevokedBy)
		}
		if key.RevokedReason != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Reason: %s\n", indent, key.RevokedReason)
		}
	}

	// Show rotation details
	if key.RotatedFrom != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  Rotated from: %s\n", indent, key.RotatedFrom)
	}

	// Show user email if present
	if key.UserEmail != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  User: %s\n", indent, key.UserEmail)
	}

	fmt.Fprintln(cmd.OutOrStdout())
}

// displayKeysJSON formats and displays keys in JSON format
func displayKeysJSON(cmd *cobra.Command, keys []secrets.KeyEntry) {
	output, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(output))
}
