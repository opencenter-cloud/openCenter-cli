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

	"github.com/opencenter-cloud/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// newClusterRotateKeysCmd creates the command for rotating encryption keys.
func newClusterRotateKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate-keys [cluster]",
		Short: "Rotate encryption keys for a cluster",
		Long: `Rotate encryption keys (Age or SSH) for a cluster.

This command implements a safe key rotation strategy using a dual-key approach:

1. Initial rotation (--type age or --type ssh):
   • Generates a new key pair
   • Adds new key alongside old key in configuration
   • Re-encrypts all secrets with both keys
   • Archives old key with timestamp

2. Complete rotation (--complete):
   • Removes old key from configuration
   • Re-encrypts secrets with new key only
   • Updates key registry

The dual-key approach ensures zero-downtime rotation. During the transition
period, secrets can be decrypted with either the old or new key.

Key rotation is recommended every 90 days for Age keys and 180 days for SSH keys.
Use 'opencenter secrets keys check' to monitor key expiration.

If no cluster name is provided, uses the currently active cluster.`,
		Example: `  # Rotate Age encryption key
  opencenter secrets keys rotate --cluster my-cluster --type age

  # Rotate SSH key
  opencenter secrets keys rotate --cluster my-cluster --type ssh

  # Complete rotation (remove old key)
  opencenter secrets keys rotate --cluster my-cluster --type age --complete

  # Preview rotation plan
  opencenter secrets keys rotate --cluster my-cluster --type age --dry-run`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterRotateKeys,
	}

	cmd.Flags().String("type", "", "Key type to rotate: age or ssh (required)")
	cmd.Flags().Bool("complete", false, "Complete dual-key rotation by removing old key")
	cmd.Flags().Bool("dry-run", false, "Preview rotation plan without making changes")

	cmd.MarkFlagRequired("type")

	return cmd
}

func runClusterRotateKeys(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Get flags
	keyType, _ := cmd.Flags().GetString("type")
	complete, _ := cmd.Flags().GetBool("complete")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Validate key type
	if keyType != "age" && keyType != "ssh" {
		return fmt.Errorf("invalid key type: %s (must be 'age' or 'ssh')", keyType)
	}

	// Resolve cluster name
	clusterName, err := resolveClusterNameForCommand(cmd, args, true)
	if err != nil {
		return err
	}

	// Initialize key rotator
	rotator, err := initializeKeyRotator()
	if err != nil {
		return fmt.Errorf("failed to initialize key rotator: %w", err)
	}

	// Build rotation options
	opts := secrets.RotateOptions{
		Cluster:  clusterName,
		KeyType:  secrets.KeyType(keyType),
		DryRun:   dryRun,
		Complete: complete,
	}

	// Execute rotation
	var result *secrets.RotationResult
	if keyType == "age" {
		result, err = rotator.RotateAgeKey(ctx, opts)
	} else {
		result, err = rotator.RotateSSHKey(ctx, opts)
	}

	if err != nil {
		return fmt.Errorf("failed to rotate %s key: %w", keyType, err)
	}

	// Display results
	displayRotationResult(cmd, clusterName, keyType, result, dryRun, complete)

	return nil
}

// displayRotationResult formats and displays the rotation result
func displayRotationResult(cmd *cobra.Command, clusterName, keyType string, result *secrets.RotationResult, dryRun, complete bool) {
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Key rotation plan for cluster %s (dry-run):\n\n", clusterName)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Key rotation completed for cluster %s:\n\n", clusterName)
	}

	// Display key information
	fmt.Fprintf(cmd.OutOrStdout(), "Key Type: %s\n", keyType)
	fmt.Fprintf(cmd.OutOrStdout(), "Old Key Fingerprint: %s\n", result.OldFingerprint)
	fmt.Fprintf(cmd.OutOrStdout(), "New Key Fingerprint: %s\n", result.NewFingerprint)
	fmt.Fprintln(cmd.OutOrStdout())

	// Display rotation mode
	if complete {
		fmt.Fprintln(cmd.OutOrStdout(), "Rotation Mode: Complete (old key removed)")
	} else if result.DualKeyActive {
		fmt.Fprintln(cmd.OutOrStdout(), "Rotation Mode: Dual-key (both keys active)")
		fmt.Fprintln(cmd.OutOrStdout(), "\nSecrets can be decrypted with either key during transition period.")
		fmt.Fprintf(cmd.OutOrStdout(), "Run 'opencenter secrets keys rotate --cluster %s --type %s --complete' to finalize rotation.\n", clusterName, keyType)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Rotation Mode: Single-key")
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Display re-encrypted files
	if len(result.ReencryptedFiles) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Re-encrypted Files (%d):\n", len(result.ReencryptedFiles))
		for _, path := range result.ReencryptedFiles {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", path)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display archived key path
	if result.ArchivedKeyPath != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Old key archived to: %s\n", result.ArchivedKeyPath)
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display summary
	fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Re-encrypted files: %d\n", len(result.ReencryptedFiles))
	fmt.Fprintf(cmd.OutOrStdout(), "  Dual-key active: %v\n", result.DualKeyActive)

	if !dryRun && !complete && result.DualKeyActive {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  1. Verify that all services can decrypt secrets with the new key")
		fmt.Fprintln(cmd.OutOrStdout(), "  2. Test cluster operations to ensure no issues")
		fmt.Fprintf(cmd.OutOrStdout(), "  3. Complete rotation: opencenter secrets keys rotate --cluster %s --type %s --complete\n", clusterName, keyType)
	}
}

// initializeKeyRotator creates and configures a key rotator instance
func initializeKeyRotator() (secrets.KeyRotator, error) {
	logger := createSecretsLogger()
	configLoader := createConfigLoader()
	sopsManager := createSOPSManager(logger)
	auditLogger, err := createAuditLogger()
	if err != nil {
		return nil, err
	}

	// Get registry path
	registryPath, err := getSecretsRegistryPath()
	if err != nil {
		return nil, err
	}

	// Create SOPS encryptor adapter
	encryptor := &sopsEncryptorAdapter{manager: sopsManager}

	// Create key registry
	registry := secrets.NewDefaultKeyRegistry(registryPath, encryptor, logger)

	// Create secrets manager (needed by rotator)
	secretsManager := secrets.NewDefaultSecretsManager(configLoader, sopsManager, auditLogger, logger)

	// Create key rotator
	rotator := secrets.NewDefaultKeyRotator(registry, secretsManager, auditLogger, logger)

	return rotator, nil
}
