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

	"github.com/rackerlabs/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// newClusterRevokeKeyCmd creates the command for revoking encryption keys.
func newClusterRevokeKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke-key [cluster]",
		Short: "Revoke encryption keys for departed users or compromised keys",
		Long: `Revoke encryption keys to remove access for departed team members or compromised keys.

This command provides three revocation modes:

1. Revoke by user (--user):
   • Removes all keys associated with a user email
   • Re-encrypts all secrets without the user's keys
   • Logs revocation event to audit log

2. Revoke by fingerprint (--key):
   • Removes a specific key by fingerprint
   • Re-encrypts all secrets without the revoked key
   • Useful for revoking individual compromised keys

3. Emergency revocation (--emergency):
   • Immediately revokes the specified key
   • Generates a new primary key
   • Re-encrypts all secrets with the new key
   • Use when a key is compromised and immediate action is needed

The revocation process:
  • Validates that at least one key will remain active
  • Removes the revoked key from .sops.yaml
  • Re-encrypts all manifests without the revoked key
  • Updates key registry with revocation details
  • Logs the revocation event for audit trail

If no cluster name is provided, uses the currently active cluster.`,
		Example: `  # Revoke all keys for a user
  opencenter cluster revoke-key my-cluster --user user@example.com

  # Revoke a specific key by fingerprint
  opencenter cluster revoke-key my-cluster --key age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn

  # Emergency revocation (generates new key immediately)
  opencenter cluster revoke-key my-cluster --key <fingerprint> --emergency

  # Preview revocation without making changes
  opencenter cluster revoke-key my-cluster --user user@example.com --dry-run`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterRevokeKey,
	}

	cmd.Flags().String("user", "", "Revoke all keys for user email")
	cmd.Flags().String("key", "", "Revoke specific key by fingerprint")
	cmd.Flags().Bool("emergency", false, "Perform emergency revocation with new key generation")
	cmd.Flags().Bool("dry-run", false, "Preview revocation without making changes")

	return cmd
}

func runClusterRevokeKey(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get flags
	user, _ := cmd.Flags().GetString("user")
	keyFingerprint, _ := cmd.Flags().GetString("key")
	emergency, _ := cmd.Flags().GetBool("emergency")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Validate flags
	if user == "" && keyFingerprint == "" {
		return fmt.Errorf("must specify either --user or --key")
	}

	if user != "" && keyFingerprint != "" {
		return fmt.Errorf("cannot specify both --user and --key")
	}

	if emergency && user != "" {
		return fmt.Errorf("--emergency can only be used with --key")
	}

	if emergency && dryRun {
		return fmt.Errorf("cannot use --emergency with --dry-run")
	}

	// Resolve cluster name
	clusterName, err := resolveClusterName(args, true)
	if err != nil {
		return err
	}

	// Initialize key revoker
	revoker, err := initializeKeyRevoker()
	if err != nil {
		return fmt.Errorf("failed to initialize key revoker: %w", err)
	}

	// Execute revocation based on mode
	var result *secrets.RevocationResult

	if emergency {
		// Emergency revocation
		if dryRun {
			return fmt.Errorf("emergency revocation cannot be performed in dry-run mode")
		}

		result, err = revoker.EmergencyRevoke(ctx, clusterName, keyFingerprint)
		if err != nil {
			return fmt.Errorf("failed to perform emergency revocation: %w", err)
		}
	} else if user != "" {
		// Revoke by user
		opts := secrets.RevokeOptions{
			Cluster: clusterName,
			User:    user,
			DryRun:  dryRun,
			Reason:  "User access revoked",
		}

		result, err = revoker.RevokeByUser(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to revoke keys for user: %w", err)
		}
	} else {
		// Revoke by fingerprint
		opts := secrets.RevokeOptions{
			Cluster:     clusterName,
			Fingerprint: keyFingerprint,
			DryRun:      dryRun,
			Reason:      "Key revoked by administrator",
		}

		result, err = revoker.RevokeByFingerprint(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to revoke key: %w", err)
		}
	}

	// Display results
	displayRevocationResult(cmd, clusterName, result, dryRun, emergency, user, keyFingerprint)

	return nil
}

// displayRevocationResult formats and displays the revocation result
func displayRevocationResult(cmd *cobra.Command, clusterName string, result *secrets.RevocationResult, dryRun, emergency bool, user, keyFingerprint string) {
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Key revocation plan for cluster %s (dry-run):\n\n", clusterName)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Key revocation completed for cluster %s:\n\n", clusterName)
	}

	// Display revocation mode
	if emergency {
		fmt.Fprintln(cmd.OutOrStdout(), "Revocation Mode: Emergency (immediate with new key generation)")
	} else if user != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Revocation Mode: By user (%s)\n", user)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Revocation Mode: By fingerprint (%s)\n", keyFingerprint)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Display revoked keys
	if len(result.RevokedKeys) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Revoked Keys (%d):\n", len(result.RevokedKeys))
		for _, fingerprint := range result.RevokedKeys {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", fingerprint)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display new primary key (emergency mode only)
	if result.NewPrimaryKey != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "New Primary Key: %s\n", result.NewPrimaryKey)
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display re-encrypted files
	if len(result.ReencryptedFiles) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Re-encrypted Files (%d):\n", len(result.ReencryptedFiles))
		// Show first 10 files, then summarize
		displayCount := len(result.ReencryptedFiles)
		if displayCount > 10 {
			displayCount = 10
		}
		for i := 0; i < displayCount; i++ {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", result.ReencryptedFiles[i])
		}
		if len(result.ReencryptedFiles) > 10 {
			fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more files\n", len(result.ReencryptedFiles)-10)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display summary
	fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Revoked keys: %d\n", len(result.RevokedKeys))
	fmt.Fprintf(cmd.OutOrStdout(), "  Re-encrypted files: %d\n", len(result.ReencryptedFiles))
	if result.NewPrimaryKey != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  New primary key generated: yes\n")
	}

	if !dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\nRevocation completed successfully.")
		fmt.Fprintln(cmd.OutOrStdout(), "The revoked key(s) can no longer decrypt cluster secrets.")
		
		if emergency {
			fmt.Fprintln(cmd.OutOrStdout(), "\n⚠️  Emergency revocation performed:")
			fmt.Fprintln(cmd.OutOrStdout(), "  • Distribute the new key to authorized users")
			fmt.Fprintln(cmd.OutOrStdout(), "  • Update any automation that uses the old key")
			fmt.Fprintln(cmd.OutOrStdout(), "  • Verify cluster operations with the new key")
		}
	}
}

// initializeKeyRevoker creates and configures a key revoker instance
func initializeKeyRevoker() (secrets.KeyRevoker, error) {
	logger := createSecretsLogger()
	configLoader := createConfigLoader()
	sopsManager := createSOPSManager(logger)
	auditLogger := &noOpAuditLogger{}

	// Get registry path
	registryPath, err := getSecretsRegistryPath()
	if err != nil {
		return nil, err
	}

	// Create SOPS encryptor adapter
	encryptor := &sopsEncryptorAdapter{manager: sopsManager}

	// Create key registry
	registry := secrets.NewDefaultKeyRegistry(registryPath, encryptor, logger)

	// Create secrets manager (needed by revoker)
	secretsManager := secrets.NewDefaultSecretsManager(configLoader, sopsManager, auditLogger, logger)

	// Create key rotator (needed for emergency revocation)
	rotator := secrets.NewDefaultKeyRotator(registry, secretsManager, auditLogger, logger)

	// Create key revoker
	revoker := secrets.NewDefaultKeyRevoker(registry, rotator, secretsManager, auditLogger, logger)

	return revoker, nil
}
