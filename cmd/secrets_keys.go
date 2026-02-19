package cmd

import (
	"github.com/spf13/cobra"
)

// NewSecretsKeysCmd creates the keys subcommand group for secrets management
func NewSecretsKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage SOPS encryption keys",
		Long: `Manage SOPS encryption keys for secrets management.

The keys subcommand group provides operations for Age key lifecycle management:
• Generate: Create new Age key pairs for SOPS encryption
• Rotate: Rotate Age keys with automatic re-encryption of existing secrets
• Backup: Create secure backups of Age keys and SOPS configuration
• Validate: Validate Age key configuration and SOPS setup

These commands integrate with opencenter workflows to provide seamless key management
for standalone clusters and GitOps deployments.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add key management subcommands
	cmd.AddCommand(newSecretsKeysGenerateCmd())
	cmd.AddCommand(newSecretsKeysRotateCmd())
	cmd.AddCommand(newSecretsKeysBackupCmd())
	cmd.AddCommand(newSecretsKeysValidateCmd())

	return cmd
}

// newSecretsKeysGenerateCmd creates the generate subcommand
func newSecretsKeysGenerateCmd() *cobra.Command {
	var (
		keyFile    string
		updateSOPS bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate new Age key pair for SOPS encryption",
		Long: `Generate a new Age key pair for SOPS encryption.

This command creates a new Age key pair and optionally updates the .sops.yaml
configuration file with the new public key. The private key is saved securely
with appropriate file permissions (600).

The generated public key should be used in .sops.yaml configuration and
cluster specifications for SOPS encryption.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSGenerateKey(cmd.Context(), keyFile, updateSOPS, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to save the Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().BoolVar(&updateSOPS, "update-sops-config", true, "Update .sops.yaml configuration with new public key")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// newSecretsKeysRotateCmd creates the rotate subcommand
func newSecretsKeysRotateCmd() *cobra.Command {
	var (
		keyFile string
		path    string
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate Age keys and re-encrypt existing secrets",
		Long: `Rotate Age keys and automatically re-encrypt existing SOPS files.

This command generates a new Age key pair, backs up the old key, and re-encrypts
all SOPS-encrypted files in the specified path with the new key. This is
essential for maintaining security through regular key rotation.

The rotation process:
1. Backs up the existing Age key
2. Generates a new Age key pair
3. Finds all SOPS-encrypted files
4. Re-encrypts each file with the new key
5. Updates .sops.yaml configuration

If any step fails, the old key is restored automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSRotateKey(cmd.Context(), keyFile, path, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&path, "path", ".", "Path to search for SOPS files to re-encrypt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// newSecretsKeysBackupCmd creates the backup subcommand
func newSecretsKeysBackupCmd() *cobra.Command {
	var (
		keyFile   string
		backupDir string
		dryRun    bool
	)

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create backup of Age keys and SOPS configuration",
		Long: `Create a secure backup of Age keys and SOPS configuration.

This command creates a timestamped backup of the Age key file and .sops.yaml
configuration. Backups are essential for disaster recovery and should be stored
securely in a separate location from the primary keys.

The backup includes:
• Age private key file
• SOPS configuration (.sops.yaml)
• Backup metadata with timestamp and creation details`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSBackupKey(cmd.Context(), keyFile, backupDir, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&backupDir, "backup-dir", "", "Backup directory (default: ~/.config/sops/age/backups)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// newSecretsKeysValidateCmd creates the validate subcommand
func newSecretsKeysValidateCmd() *cobra.Command {
	var (
		keyFile    string
		configFile string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Age key configuration and SOPS setup",
		Long: `Validate Age key configuration and SOPS setup.

This command performs comprehensive validation of the SOPS configuration:
• Checks Age key file existence and permissions
• Validates Age key format and functionality
• Tests SOPS encryption/decryption functionality
• Verifies .sops.yaml configuration
• Ensures all required tools are installed

Use this command to troubleshoot SOPS issues or verify configuration
after key rotation or setup changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSValidate(cmd.Context(), keyFile, configFile, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&configFile, "config-file", ".sops.yaml", "Path to SOPS configuration file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}
