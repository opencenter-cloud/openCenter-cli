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
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/operations"
	"github.com/spf13/cobra"
)

// newClusterBackupCmd creates the "cluster backup" command with subcommands
func newClusterBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage cluster backups",
		Long: `Manage cluster configuration backups for disaster recovery.

Backups include:
  - Cluster configuration file
  - SOPS Age encryption keys
  - SSH keys
  - GitOps repository state
  - Terraform state files

Backups are compressed, encrypted with AES-256-GCM, and include SHA-256 checksums
for integrity verification.`,
		Example: `  # Create a backup
  openCenter cluster backup create my-cluster

  # Create an encrypted backup
  openCenter cluster backup create my-cluster --passphrase

  # List backups for a cluster
  openCenter cluster backup list my-cluster

  # Restore from backup
  openCenter cluster backup restore my-cluster-20260118-143000

  # Schedule periodic backups
  openCenter cluster backup schedule my-cluster --interval=24h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newClusterBackupCreateCmd())
	cmd.AddCommand(newClusterBackupRestoreCmd())
	cmd.AddCommand(newClusterBackupListCmd())
	cmd.AddCommand(newClusterBackupDeleteCmd())
	cmd.AddCommand(newClusterBackupScheduleCmd())

	return cmd
}

// newClusterBackupCreateCmd creates the "cluster backup create" command
func newClusterBackupCreateCmd() *cobra.Command {
	var passphrase string
	var encrypt bool

	cmd := &cobra.Command{
		Use:   "create <cluster>",
		Short: "Create a backup of cluster configuration",
		Long: `Create a backup of cluster configuration and related files.

The backup includes:
  - Cluster configuration YAML
  - SOPS Age encryption keys
  - SSH keys
  - GitOps repository state
  - Terraform state files

Backups are compressed with gzip and can be encrypted with a passphrase.`,
		Example: `  # Create a backup
  openCenter cluster backup create my-cluster

  # Create an encrypted backup (will prompt for passphrase)
  openCenter cluster backup create my-cluster --encrypt

  # Create an encrypted backup with passphrase
  openCenter cluster backup create my-cluster --passphrase="secret123"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName := args[0]

			// Get config directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			configDir := filepath.Join(homeDir, ".config", "openCenter")
			backupDir := filepath.Join(configDir, "backups")

			// Create backup manager
			bm, err := operations.NewBackupManager(configDir, backupDir)
			if err != nil {
				return fmt.Errorf("failed to create backup manager: %w", err)
			}

			// Create backup
			fmt.Printf("Creating backup for cluster %s...\n", clusterName)
			backup, err := bm.CreateBackup(context.Background(), clusterName)
			if err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}

			fmt.Printf("✓ Backup created: %s\n", backup.ID)
			fmt.Printf("  Size: %d bytes\n", backup.Size)
			fmt.Printf("  Checksum: %s\n", backup.Checksum)
			fmt.Printf("  Location: %s\n", backup.StorageLocation)

			// Encrypt if requested
			if encrypt || passphrase != "" {
				if passphrase == "" {
					// Prompt for passphrase
					fmt.Print("Enter passphrase: ")
					fmt.Scanln(&passphrase)
				}

				if passphrase == "" {
					return fmt.Errorf("passphrase cannot be empty")
				}

				fmt.Println("Encrypting backup...")
				if err := operations.EncryptBackup(backup.StorageLocation, passphrase); err != nil {
					return fmt.Errorf("failed to encrypt backup: %w", err)
				}

				fmt.Println("✓ Backup encrypted")
			}

			fmt.Println("\nBackup created successfully!")
			fmt.Printf("Retention until: %s\n", backup.RetentionUntil.Format(time.RFC3339))

			return nil
		},
	}

	cmd.Flags().StringVar(&passphrase, "passphrase", "", "Passphrase for backup encryption")
	cmd.Flags().BoolVar(&encrypt, "encrypt", false, "Encrypt backup (will prompt for passphrase)")

	return cmd
}

// newClusterBackupRestoreCmd creates the "cluster backup restore" command
func newClusterBackupRestoreCmd() *cobra.Command {
	var passphrase string

	cmd := &cobra.Command{
		Use:   "restore <backup-id>",
		Short: "Restore cluster configuration from backup",
		Long: `Restore cluster configuration and related files from a backup.

The backup ID is the filename without extension (e.g., my-cluster-20260118-143000).

Restored files are placed in a "restored" directory to avoid overwriting existing
configurations. You can then manually move them to the appropriate locations.`,
		Example: `  # Restore from backup
  openCenter cluster backup restore my-cluster-20260118-143000

  # Restore from encrypted backup
  openCenter cluster backup restore my-cluster-20260118-143000 --passphrase="secret123"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backupID := args[0]

			// Get config directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			configDir := filepath.Join(homeDir, ".config", "openCenter")
			backupDir := filepath.Join(configDir, "backups")

			// Create backup manager
			bm, err := operations.NewBackupManager(configDir, backupDir)
			if err != nil {
				return fmt.Errorf("failed to create backup manager: %w", err)
			}

			// Prompt for passphrase if not provided
			if passphrase == "" {
				fmt.Print("Enter passphrase (leave empty if backup is not encrypted): ")
				fmt.Scanln(&passphrase)
			}

			// Restore backup
			fmt.Printf("Restoring backup %s...\n", backupID)
			if err := bm.RestoreBackup(context.Background(), backupID, passphrase); err != nil {
				return fmt.Errorf("failed to restore backup: %w", err)
			}

			fmt.Println("✓ Backup restored successfully!")
			fmt.Println("\nRestored files are in the 'restored' directory:")
			fmt.Printf("  Config: %s/clusters/restored/.restored-config.yaml\n", configDir)
			fmt.Printf("  Age key: %s/secrets/age/restored-key.txt\n", configDir)
			fmt.Printf("  SSH keys: %s/secrets/ssh/restored-keys\n", configDir)
			fmt.Println("\nPlease review and move files to appropriate locations.")

			return nil
		},
	}

	cmd.Flags().StringVar(&passphrase, "passphrase", "", "Passphrase for backup decryption")

	return cmd
}

// newClusterBackupListCmd creates the "cluster backup list" command
func newClusterBackupListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [cluster]",
		Short: "List backups for a cluster",
		Long: `List all backups for a cluster or all backups if no cluster is specified.

Displays backup ID, creation time, size, and storage location.`,
		Example: `  # List all backups
  openCenter cluster backup list

  # List backups for a specific cluster
  openCenter cluster backup list my-cluster`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var clusterName string
			if len(args) > 0 {
				clusterName = args[0]
			}

			// Get config directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			configDir := filepath.Join(homeDir, ".config", "openCenter")
			backupDir := filepath.Join(configDir, "backups")

			// Create backup manager
			bm, err := operations.NewBackupManager(configDir, backupDir)
			if err != nil {
				return fmt.Errorf("failed to create backup manager: %w", err)
			}

			// List backups
			backups, err := bm.ListBackups(clusterName)
			if err != nil {
				return fmt.Errorf("failed to list backups: %w", err)
			}

			if len(backups) == 0 {
				if clusterName != "" {
					fmt.Printf("No backups found for cluster %s\n", clusterName)
				} else {
					fmt.Println("No backups found")
				}
				return nil
			}

			// Display backups in table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "BACKUP ID\tCLUSTER\tCREATED\tSIZE\tLOCATION")
			fmt.Fprintln(w, "---------\t-------\t-------\t----\t--------")

			for _, backup := range backups {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					backup.ID,
					backup.Cluster,
					backup.CreatedAt.Format("2006-01-02 15:04:05"),
					backup.Size,
					backup.StorageLocation,
				)
			}

			w.Flush()

			return nil
		},
	}

	return cmd
}

// newClusterBackupDeleteCmd creates the "cluster backup delete" command
func newClusterBackupDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <backup-id>",
		Short: "Delete a backup",
		Long: `Delete a backup by its ID.

This operation is irreversible. Use with caution.`,
		Example: `  # Delete a backup
  openCenter cluster backup delete my-cluster-20260118-143000

  # Delete without confirmation
  openCenter cluster backup delete my-cluster-20260118-143000 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backupID := args[0]

			// Confirm deletion unless --force is used
			if !force {
				fmt.Printf("Are you sure you want to delete backup %s? (yes/no): ", backupID)
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "yes" {
					fmt.Println("Deletion cancelled")
					return nil
				}
			}

			// Get config directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			configDir := filepath.Join(homeDir, ".config", "openCenter")
			backupDir := filepath.Join(configDir, "backups")

			// Create backup manager
			bm, err := operations.NewBackupManager(configDir, backupDir)
			if err != nil {
				return fmt.Errorf("failed to create backup manager: %w", err)
			}

			// Delete backup
			if err := bm.DeleteBackup(backupID); err != nil {
				return fmt.Errorf("failed to delete backup: %w", err)
			}

			fmt.Printf("✓ Backup %s deleted successfully\n", backupID)

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Delete without confirmation")

	return cmd
}

// newClusterBackupScheduleCmd creates the "cluster backup schedule" command
func newClusterBackupScheduleCmd() *cobra.Command {
	var interval string
	var retention string

	cmd := &cobra.Command{
		Use:   "schedule <cluster>",
		Short: "Schedule periodic backups for a cluster",
		Long: `Schedule periodic backups for a cluster.

This feature is not yet implemented. It will support cron-style scheduling
and automatic backup retention policies.`,
		Example: `  # Schedule daily backups
  openCenter cluster backup schedule my-cluster --interval=24h

  # Schedule with retention policy
  openCenter cluster backup schedule my-cluster --interval=24h --retention=30d`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName := args[0]

			fmt.Printf("Scheduling backups for cluster %s...\n", clusterName)
			fmt.Printf("  Interval: %s\n", interval)
			fmt.Printf("  Retention: %s\n", retention)
			fmt.Println("\n⚠ Backup scheduling is not yet implemented")
			fmt.Println("This feature will be available in a future release.")

			return nil
		},
	}

	cmd.Flags().StringVar(&interval, "interval", "24h", "Backup interval (e.g., 24h, 7d)")
	cmd.Flags().StringVar(&retention, "retention", "30d", "Backup retention period (e.g., 30d, 90d)")

	return cmd
}
