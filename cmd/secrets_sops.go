package cmd

import (
	"github.com/spf13/cobra"
)

// newSecretsEncryptCmd creates the encrypt subcommand
func newSecretsEncryptCmd() *cobra.Command {
	var (
		path     string
		noBackup bool
	)

	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt secrets in YAML files",
		Long: `Encrypt secrets in YAML files using SOPS.

This command finds all unencrypted YAML files that match SOPS configuration
rules and encrypts them using the configured Age keys. By default, it creates
backups of the original files for safety.

The encryption process:
1. Creates timestamped backups of original files (unless --no-backup is set)
2. Encrypts files using SOPS with configured Age keys
3. Validates successful encryption
4. Reports results and backup locations

Use this command when you want to encrypt secrets in your project.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsEncrypt(cmd.Context(), "", path, false, !noBackup)
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Path to search for files to encrypt")
	cmd.Flags().BoolVar(&noBackup, "no-backup", false, "Skip backup creation before encryption")

	return cmd
}

// newSecretsDecryptCmd creates the decrypt subcommand
func newSecretsDecryptCmd() *cobra.Command {
	var (
		path     string
		noBackup bool
	)

	cmd := &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt secrets in YAML files",
		Long: `Decrypt secrets in YAML files using SOPS.

This command finds all SOPS-encrypted YAML files and decrypts them using
the configured Age keys. By default, it creates backups of the encrypted
files for safety.

The decryption process:
1. Creates timestamped backups of encrypted files (unless --no-backup is set)
2. Decrypts files using SOPS with configured Age keys
3. Validates successful decryption
4. Reports results and backup locations

Use this command when you need to decrypt secrets in your project.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsDecrypt(cmd.Context(), "", path, false, !noBackup)
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Path to search for files to decrypt")
	cmd.Flags().BoolVar(&noBackup, "no-backup", false, "Skip backup creation before decryption")

	return cmd
}

// newSecretsStatusCmd creates the status subcommand
func newSecretsStatusCmd() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show encryption status of YAML files",
		Long: `Show the encryption status of YAML files in your project.

This command searches for YAML files in the specified path and displays
their SOPS encryption status. It identifies:
• Encrypted files (already protected with SOPS)
• Unencrypted files that should be encrypted (based on SOPS rules)

Use this command to get an overview of all secrets in your project and
verify which files are encrypted and which need encryption.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsList(cmd.Context(), "", path, false)
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Path to search for YAML files")

	return cmd
}
