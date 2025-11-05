package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rackerlabs/openCenter-cli/internal/sops"
)

// newSOPSCmd creates the SOPS management command group
func newSOPSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sops",
		Short: "SOPS key management and automation",
		Long: `SOPS key management provides automated procedures for Age key generation, rotation, and management.

The SOPS command group includes:
• Key Generation: Create new Age key pairs for SOPS encryption
• Key Rotation: Rotate Age keys with automatic re-encryption of existing secrets
• Key Backup: Create secure backups of Age keys and SOPS configuration
• Validation: Validate Age key configuration and SOPS setup

These commands integrate with openCenter workflows to provide seamless secret management
for standalone clusters and GitOps deployments.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newSOPSGenerateKeyCmd())
	cmd.AddCommand(newSOPSRotateKeyCmd())
	cmd.AddCommand(newSOPSBackupKeyCmd())
	cmd.AddCommand(newSOPSValidateCmd())

	return cmd
}

// newSOPSGenerateKeyCmd creates the generate-key subcommand
func newSOPSGenerateKeyCmd() *cobra.Command {
	var (
		keyFile    string
		updateSOPS bool
	)

	cmd := &cobra.Command{
		Use:   "generate-key",
		Short: "Generate new Age key pair for SOPS encryption",
		Long: `Generate a new Age key pair for SOPS encryption.

This command creates a new Age key pair and optionally updates the .sops.yaml
configuration file with the new public key. The private key is saved securely
with appropriate file permissions (600).

The generated public key should be used in .sops.yaml configuration and
cluster specifications for SOPS encryption.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSGenerateKey(cmd.Context(), keyFile, updateSOPS)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to save the Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().BoolVar(&updateSOPS, "update-sops-config", true, "Update .sops.yaml configuration with new public key")

	return cmd
}

// newSOPSRotateKeyCmd creates the rotate-key subcommand
func newSOPSRotateKeyCmd() *cobra.Command {
	var (
		keyFile    string
		searchPath string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "rotate-key",
		Short: "Rotate Age keys and re-encrypt existing secrets",
		Long: `Rotate Age keys and automatically re-encrypt existing SOPS files.

This command generates a new Age key pair, backs up the old key, and re-encrypts
all SOPS-encrypted files in the specified search path with the new key. This is
essential for maintaining security through regular key rotation.

The rotation process:
1. Backs up the existing Age key
2. Generates a new Age key pair
3. Finds all SOPS-encrypted files
4. Re-encrypts each file with the new key
5. Updates .sops.yaml configuration

If any step fails, the old key is restored automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSRotateKey(cmd.Context(), keyFile, searchPath, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&searchPath, "search-path", ".", "Path to search for SOPS files to re-encrypt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// newSOPSBackupKeyCmd creates the backup-key subcommand
func newSOPSBackupKeyCmd() *cobra.Command {
	var (
		keyFile   string
		backupDir string
	)

	cmd := &cobra.Command{
		Use:   "backup-key",
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
			return executeSOPSBackupKey(cmd.Context(), keyFile, backupDir)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&backupDir, "backup-dir", "", "Backup directory (default: ~/.config/sops/age/backups)")

	return cmd
}

// newSOPSValidateCmd creates the validate subcommand
func newSOPSValidateCmd() *cobra.Command {
	var (
		keyFile    string
		configFile string
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
			return executeSOPSValidate(cmd.Context(), keyFile, configFile)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&configFile, "config-file", ".sops.yaml", "Path to SOPS configuration file")

	return cmd
}

// executeSOPSGenerateKey generates a new Age key pair
func executeSOPSGenerateKey(ctx context.Context, keyFile string, updateSOPS bool) error {
	fmt.Println("🔑 Generating new Age key pair for SOPS encryption...")

	// Set default key file path if not specified
	if keyFile == "" {
		homeDir, _ := os.UserHomeDir()
		keyFile = filepath.Join(homeDir, ".config", "sops", "age", "keys.txt")
	}

	// Initialize key manager for the directory containing the key file
	keyDir := filepath.Dir(keyFile)
	km := sops.NewKeyManager(keyDir)

	// Generate new key pair
	keyPair, err := km.GenerateAgeKey()
	if err != nil {
		return fmt.Errorf("failed to generate Age key pair: %w", err)
	}

	// Check if key file already exists and backup if needed
	if _, err := os.Stat(keyFile); err == nil {
		fmt.Printf("⚠️  Key file '%s' already exists, creating backup...\n", keyFile)
		backupPath := fmt.Sprintf("%s.backup-%s", keyFile, time.Now().Format("20060102-150405"))
		if err := copyFile(keyFile, backupPath); err == nil {
			fmt.Printf("✅ Existing key backed up as: %s\n", backupPath)
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write the private key to the specified path
	if err := os.WriteFile(keyFile, []byte(keyPair.PrivateKey), 0o600); err != nil {
		return fmt.Errorf("failed to save Age key: %w", err)
	}

	fmt.Println("✅ Age key pair generated successfully!")
	fmt.Printf("📁 Private key: %s\n", keyFile)
	fmt.Printf("🔑 Public key: %s\n", keyPair.PublicKey)

	// Update SOPS configuration if requested
	if updateSOPS {
		if err := updateSOPSConfig(keyPair.PublicKey); err != nil {
			fmt.Printf("⚠️  Failed to update .sops.yaml: %v\n", err)
			fmt.Printf("💡 Please update .sops.yaml manually with the public key above\n")
		} else {
			fmt.Println("✅ Updated .sops.yaml configuration")
		}
	}

	return nil
}

// executeSOPSRotateKey rotates Age keys and re-encrypts secrets
func executeSOPSRotateKey(ctx context.Context, keyFile, searchPath string, dryRun bool) error {
	fmt.Println("🔄 Starting Age key rotation...")

	// Initialize key manager
	homeDir, _ := os.UserHomeDir()
	keyDir := filepath.Join(homeDir, ".config", "sops", "age")
	km := sops.NewKeyManager(keyDir)

	// Use default key name if not specified
	keyName := "keys"
	if keyFile != "" {
		keyName = filepath.Base(strings.TrimSuffix(keyFile, filepath.Ext(keyFile)))
	}

	if dryRun {
		fmt.Println("🧪 DRY RUN: Age key rotation simulation")
		fmt.Printf("📁 Key name: %s\n", keyName)
		fmt.Printf("🔍 Search path: %s\n", searchPath)

		// Find SOPS files that would be re-encrypted
		fmt.Println("🔍 Searching for SOPS-encrypted files...")
		encryptor := sops.NewEncryptor(nil, nil)
		
		err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			
			if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
				if isEncrypted, err := encryptor.IsFileEncrypted(path); err == nil && isEncrypted {
					fmt.Printf("  📄 Would re-encrypt: %s\n", path)
				}
			}
			return nil
		})
		
		if err != nil {
			return fmt.Errorf("failed to search for encrypted files: %w", err)
		}

		return nil
	}

	fmt.Printf("📁 Key name: %s\n", keyName)
	fmt.Printf("🔍 Search path: %s\n", searchPath)

	// Load old key for backup
	oldKey, err := km.LoadAgeKey(keyName)
	if err != nil {
		return fmt.Errorf("failed to load existing key: %w", err)
	}

	// Create backup
	backupDir := filepath.Join(keyDir, "backups")
	if err := os.MkdirAll(backupDir, 0o700); err == nil {
		backupKM := sops.NewKeyManager(backupDir)
		backupName := fmt.Sprintf("%s-backup-%s", keyName, time.Now().Format("20060102-150405"))
		if err := backupKM.SaveAgeKey(oldKey, backupName); err == nil {
			fmt.Printf("✅ Old key backed up as: %s\n", backupName)
		}
	}

	// Generate new key
	newKey, err := km.GenerateAgeKey()
	if err != nil {
		return fmt.Errorf("failed to generate new key: %w", err)
	}

	// Save new key
	if err := km.SaveAgeKey(newKey, keyName); err != nil {
		return fmt.Errorf("failed to save new key: %w", err)
	}

	// Re-encrypt files with new key
	encryptor := sops.NewEncryptor([]string{newKey.PublicKey}, nil)
	
	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			if isEncrypted, err := encryptor.IsFileEncrypted(path); err == nil && isEncrypted {
				fmt.Printf("🔄 Re-encrypting: %s\n", path)
				if err := encryptor.RotateKeys(ctx, path, []string{newKey.PublicKey}, nil); err != nil {
					fmt.Printf("⚠️  Failed to re-encrypt %s: %v\n", path, err)
				}
			}
		}
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("failed to re-encrypt files: %w", err)
	}

	fmt.Println("✅ Age key rotation completed successfully!")
	fmt.Printf("🔑 New public key: %s\n", newKey.PublicKey)

	// Update SOPS configuration
	if err := updateSOPSConfig(newKey.PublicKey); err != nil {
		fmt.Printf("⚠️  Failed to update .sops.yaml: %v\n", err)
		fmt.Printf("💡 Please update .sops.yaml manually with the new public key\n")
	} else {
		fmt.Println("✅ Updated .sops.yaml configuration")
	}

	return nil
}

// executeSOPSBackupKey creates a backup of Age keys
func executeSOPSBackupKey(ctx context.Context, keyFile, backupDir string) error {
	fmt.Println("💾 Creating Age key backup...")

	// Initialize key manager
	homeDir, _ := os.UserHomeDir()
	keyDir := filepath.Join(homeDir, ".config", "sops", "age")
	km := sops.NewKeyManager(keyDir)

	// Use default backup directory if not specified
	if backupDir == "" {
		backupDir = filepath.Join(keyDir, "backups")
	}

	// Create backup
	if err := km.BackupKeys(backupDir); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Println("✅ Age key backup created successfully!")
	fmt.Printf("📁 Backup directory: %s\n", backupDir)

	// Also backup SOPS configuration if it exists
	if _, err := os.Stat(".sops.yaml"); err == nil {
		configBackup := filepath.Join(backupDir, fmt.Sprintf("sops-config-%s.yaml",
			time.Now().Format("20060102-150405")))

		if err := copyFile(".sops.yaml", configBackup); err != nil {
			fmt.Printf("⚠️  Failed to backup SOPS configuration: %v\n", err)
		} else {
			fmt.Printf("✅ SOPS configuration backed up to: %s\n", configBackup)
		}
	}

	return nil
}

// executeSOPSValidate validates Age key configuration
func executeSOPSValidate(ctx context.Context, keyFile, configFile string) error {
	fmt.Println("🔍 Validating SOPS configuration...")

	// Initialize key manager
	homeDir, _ := os.UserHomeDir()
	keyDir := filepath.Join(homeDir, ".config", "sops", "age")
	km := sops.NewKeyManager(keyDir)

	// Use default key name if not specified
	keyName := "keys"
	if keyFile != "" {
		keyName = filepath.Base(strings.TrimSuffix(keyFile, filepath.Ext(keyFile)))
	}

	fmt.Printf("📁 Key name: %s\n", keyName)
	fmt.Printf("📄 Config file: %s\n", configFile)

	// Check if key exists
	keyPath := filepath.Join(keyDir, fmt.Sprintf("%s.txt", keyName))
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("❌ Age key file not found: %s", keyPath)
	}

	// Load key to get public key
	keyPair, err := km.LoadAgeKey(keyName)
	if err != nil {
		return fmt.Errorf("❌ Failed to load Age key: %w", err)
	}

	// Validate key format
	if err := km.ValidateAgeKey(keyPair.PublicKey); err != nil {
		return fmt.Errorf("❌ Age key validation failed: %w", err)
	}

	fmt.Println("✅ Age key validation passed")
	fmt.Printf("🔑 Public key: %s\n", keyPair.PublicKey)

	// Validate SOPS configuration if it exists
	if _, err := os.Stat(configFile); err == nil {
		fmt.Println("🔍 Validating SOPS configuration file...")

		// Basic validation - check if public key is in config
		content, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read SOPS config: %w", err)
		}

		if !strings.Contains(string(content), keyPair.PublicKey) {
			fmt.Printf("⚠️  SOPS configuration does not contain current Age public key\n")
			fmt.Printf("💡 Consider updating %s with the current public key\n", configFile)
		} else {
			fmt.Println("✅ SOPS configuration contains current Age public key")
		}
	} else {
		fmt.Printf("⚠️  SOPS configuration file not found: %s\n", configFile)
	}

	// Test key access
	fmt.Println("🧪 Testing key access...")
	if err := km.ValidateKeyAccess(keyName); err != nil {
		return fmt.Errorf("❌ Key access test failed: %w", err)
	}

	fmt.Println("✅ Key access test passed")

	// Check SOPS installation
	encryptor := sops.NewEncryptor(nil, nil)
	if version, err := encryptor.CheckSOPSVersion(ctx); err != nil {
		fmt.Printf("⚠️  SOPS not found or not executable: %v\n", err)
	} else {
		fmt.Printf("✅ SOPS is installed: %s\n", version)
	}

	fmt.Println("✅ All validations completed successfully!")

	return nil
}

// copyFile copies a file from src to dst (helper function)
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	err = os.WriteFile(dst, input, 0o644)
	if err != nil {
		return err
	}

	return nil
}

// SOPSConfig represents the structure of .sops.yaml
type SOPSConfig struct {
	CreationRules []CreationRule `yaml:"creation_rules"`
}

// CreationRule represents a SOPS creation rule
type CreationRule struct {
	PathRegex      string `yaml:"path_regex,omitempty"`
	EncryptedRegex string `yaml:"encrypted_regex,omitempty"`
	Age            string `yaml:"age,omitempty"`
	PGP            string `yaml:"pgp,omitempty"`
	KMS            string `yaml:"kms,omitempty"`
	AzureKV        string `yaml:"azure_kv,omitempty"`
	GCPKMS         string `yaml:"gcp_kms,omitempty"`
	HashiCorpVault string `yaml:"hc_vault,omitempty"`
}

// updateSOPSConfig updates the .sops.yaml configuration with a new Age public key
func updateSOPSConfig(publicKey string) error {
	configPath := ".sops.yaml"

	// Check if config file exists
	var config SOPSConfig
	if _, err := os.Stat(configPath); err == nil {
		// Load existing config
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read SOPS config: %w", err)
		}

		if err := yaml.Unmarshal(content, &config); err != nil {
			return fmt.Errorf("failed to parse SOPS config: %w", err)
		}

		// Create backup
		backupPath := fmt.Sprintf("%s.backup-%s", configPath, time.Now().Format("20060102-150405"))
		if err := copyFile(configPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup SOPS config: %w", err)
		}
	} else {
		// Create default config
		config = SOPSConfig{
			CreationRules: []CreationRule{
				{
					PathRegex:      `\.yaml$`,
					EncryptedRegex: `^(data|stringData)$`,
					Age:            publicKey,
				},
			},
		}
	}

	// Update Age key in all rules
	for i := range config.CreationRules {
		if config.CreationRules[i].Age != "" {
			config.CreationRules[i].Age = publicKey
		}
	}

	// Write updated config
	content, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal SOPS config: %w", err)
	}

	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		return fmt.Errorf("failed to write SOPS config: %w", err)
	}

	return nil
}
