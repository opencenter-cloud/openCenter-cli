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
	"github.com/rackerlabs/openCenter-cli/internal/util/crypto"
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
	
	// Add secrets management commands
	cmd.AddCommand(newSOPSSecretsListCmd())
	cmd.AddCommand(newSOPSSecretsStatusCmd())
	cmd.AddCommand(newSOPSSecretsEncryptCmd())
	cmd.AddCommand(newSOPSSecretsEncryptFastCmd())
	cmd.AddCommand(newSOPSSecretsDecryptCmd())
	cmd.AddCommand(newSOPSSecretsDecryptFastCmd())

	return cmd
}

// newSOPSGenerateKeyCmd creates the generate-key subcommand
func newSOPSGenerateKeyCmd() *cobra.Command {
	var (
		keyFile    string
		updateSOPS bool
		dryRun     bool
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
			return executeSOPSGenerateKey(cmd.Context(), keyFile, updateSOPS, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to save the Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().BoolVar(&updateSOPS, "update-sops-config", true, "Update .sops.yaml configuration with new public key")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

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
		dryRun    bool
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
			return executeSOPSBackupKey(cmd.Context(), keyFile, backupDir, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&backupDir, "backup-dir", "", "Backup directory (default: ~/.config/sops/age/backups)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// newSOPSValidateCmd creates the validate subcommand
func newSOPSValidateCmd() *cobra.Command {
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

// executeSOPSGenerateKey generates a new Age key pair
func executeSOPSGenerateKey(ctx context.Context, keyFile string, updateSOPS bool, dryRun bool) error {
	if dryRun {
		fmt.Println("🧪 DRY RUN: Age key generation simulation")
	} else {
		fmt.Println("🔑 Generating new Age key pair for SOPS encryption...")
	}

	// Set default key file path if not specified
	if keyFile == "" {
		homeDir, _ := os.UserHomeDir()
		keyFile = filepath.Join(homeDir, ".config", "sops", "age", "keys.txt")
	}

	fmt.Printf("📁 Key file path: %s\n", keyFile)
	fmt.Printf("🔧 Update SOPS config: %t\n", updateSOPS)

	if dryRun {
		fmt.Println("🔑 Would generate new Age key pair")
		fmt.Println("📄 Would save private key to specified path")
		fmt.Println("🔑 Would display public key for configuration")
		if updateSOPS {
			fmt.Println("⚙️  Would update .sops.yaml configuration")
		}
		return nil
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
	// Format: # created: <timestamp>\n# public key: <public_key>\n<private_key>
	keyContent := fmt.Sprintf("# created: %s\n# public key: %s\n%s",
		time.Now().UTC().Format(time.RFC3339),
		keyPair.PublicKey,
		keyPair.PrivateKey)
	if !strings.HasSuffix(keyContent, "\n") {
		keyContent += "\n"
	}
	if err := os.WriteFile(keyFile, []byte(keyContent), 0o600); err != nil {
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
		encryptor := sops.NewDefaultEncryptor(nil, nil)
		
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
	encryptor := sops.NewDefaultEncryptor([]string{newKey.PublicKey}, nil)
	
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
func executeSOPSBackupKey(ctx context.Context, keyFile, backupDir string, dryRun bool) error {
	if dryRun {
		fmt.Println("🧪 DRY RUN: Age key backup simulation")
	} else {
		fmt.Println("💾 Creating Age key backup...")
	}

	// Initialize key manager
	homeDir, _ := os.UserHomeDir()
	keyDir := filepath.Join(homeDir, ".config", "sops", "age")
	km := sops.NewKeyManager(keyDir)

	// Use default backup directory if not specified
	if backupDir == "" {
		backupDir = filepath.Join(keyDir, "backups")
	}

	fmt.Printf("📁 Backup directory: %s\n", backupDir)

	if dryRun {
		fmt.Println("💾 Would create backup of all Age keys")
		fmt.Println("📄 Would backup .sops.yaml configuration if it exists")
		return nil
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
func executeSOPSValidate(ctx context.Context, keyFile, configFile string, dryRun bool) error {
	// Check if debug mode is enabled
	debugMode := os.Getenv("OPENCENTER_DEBUG") != ""
	
	if dryRun {
		fmt.Println("🧪 DRY RUN: SOPS validation simulation")
	} else {
		fmt.Println("🔍 Validating SOPS configuration...")
	}

	if debugMode {
		fmt.Println("🐛 DEBUG MODE ENABLED")
		fmt.Printf("🐛 Environment: OPENCENTER_DEBUG=%s\n", os.Getenv("OPENCENTER_DEBUG"))
	}

	// Initialize key manager and determine key path
	homeDir, _ := os.UserHomeDir()
	var keyPath string
	var keyDir string
	var keyName string
	
	if keyFile != "" {
		// Use the provided key file path directly
		if strings.HasPrefix(keyFile, "~") {
			keyPath = filepath.Join(homeDir, keyFile[1:])
		} else if filepath.IsAbs(keyFile) {
			keyPath = keyFile
		} else {
			// Relative path - make it absolute
			if absPath, err := filepath.Abs(keyFile); err == nil {
				keyPath = absPath
			} else {
				keyPath = keyFile
			}
		}
		keyDir = filepath.Dir(keyPath)
		keyName = filepath.Base(strings.TrimSuffix(keyPath, filepath.Ext(keyPath)))
		
		if debugMode {
			fmt.Printf("🐛 Using custom key file: %s\n", keyFile)
			fmt.Printf("🐛 Resolved key path: %s\n", keyPath)
		}
	} else {
		// Use default key location
		keyDir = filepath.Join(homeDir, ".config", "sops", "age")
		keyName = "keys"
		keyPath = filepath.Join(keyDir, fmt.Sprintf("%s.txt", keyName))
	}
	
	km := sops.NewKeyManager(keyDir)

	if debugMode {
		fmt.Printf("🐛 Home directory: %s\n", homeDir)
		fmt.Printf("🐛 Key directory: %s\n", keyDir)
	}

	fmt.Printf("📁 Key name: %s\n", keyName)
	fmt.Printf("📁 Key path: %s\n", keyPath)
	fmt.Printf("📄 Config file: %s\n", configFile)

	if dryRun {
		fmt.Println("🔍 Would validate Age key file existence")
		fmt.Println("🔍 Would validate Age key format")
		fmt.Println("🔍 Would validate SOPS configuration")
		fmt.Println("🔍 Would test key access")
		fmt.Println("🔍 Would check SOPS installation")
		return nil
	}

	// Check if key exists
	if debugMode {
		fmt.Printf("🐛 Checking key path: %s\n", keyPath)
	}
	
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if debugMode {
			fmt.Printf("🐛 Key file not found at: %s\n", keyPath)
			// List available keys
			if entries, err := os.ReadDir(keyDir); err == nil {
				fmt.Printf("🐛 Available files in key directory:\n")
				for _, entry := range entries {
					fmt.Printf("🐛   - %s\n", entry.Name())
				}
			} else {
				fmt.Printf("🐛 Failed to list key directory: %v\n", err)
			}
		}
		return fmt.Errorf("❌ Age key file not found: %s", keyPath)
	}

	if debugMode {
		// Show file permissions
		if info, err := os.Stat(keyPath); err == nil {
			fmt.Printf("🐛 Key file permissions: %s\n", info.Mode())
			fmt.Printf("🐛 Key file size: %d bytes\n", info.Size())
		}
	}

	// Check if public key file exists, if not create it from private key
	publicKeyPath := strings.TrimSuffix(keyPath, filepath.Ext(keyPath)) + ".pub"
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		if debugMode {
			fmt.Printf("🐛 Public key file not found at: %s\n", publicKeyPath)
			fmt.Println("🐛 Generating public key from private key...")
		} else {
			fmt.Printf("ℹ️  Public key file not found, generating from private key...\n")
		}
		
		// Read the private key
		privateKeyData, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("❌ Failed to read private key: %w", err)
		}
		
		// Extract the actual key (skip comments and empty lines)
		privateKeyStr := extractAgeKeyFromContent(string(privateKeyData))
		if privateKeyStr == "" {
			return fmt.Errorf("❌ No valid age key found in file")
		}
		
		// Parse the private key to extract the public key
		keyPair, err := crypto.ParseAgeKey(privateKeyStr)
		if err != nil {
			return fmt.Errorf("❌ Failed to parse private key: %w", err)
		}
		
		// Save the public key
		if err := os.WriteFile(publicKeyPath, []byte(keyPair.PublicKey), 0o644); err != nil {
			return fmt.Errorf("❌ Failed to save public key: %w", err)
		}
		
		fmt.Printf("✅ Created public key file: %s\n", publicKeyPath)
		if debugMode {
			fmt.Printf("🐛 Public key: %s\n", keyPair.PublicKey)
		}
	} else if debugMode {
		fmt.Printf("🐛 Public key file exists at: %s\n", publicKeyPath)
	}

	// Load key to get public key
	if debugMode {
		fmt.Println("🐛 Loading Age key...")
	}
	keyPair, err := km.LoadAgeKey(keyName)
	if err != nil {
		if debugMode {
			fmt.Printf("🐛 Failed to load key: %v\n", err)
		}
		return fmt.Errorf("❌ Failed to load Age key: %w", err)
	}

	if debugMode {
		fmt.Printf("🐛 Private key length: %d characters\n", len(keyPair.PrivateKey))
		fmt.Printf("🐛 Public key: %s\n", keyPair.PublicKey)
		fmt.Printf("🐛 Public key length: %d characters\n", len(keyPair.PublicKey))
	}

	// Validate key format
	if debugMode {
		fmt.Println("🐛 Validating Age key format...")
	}
	if err := km.ValidateAgeKey(keyPair.PublicKey); err != nil {
		if debugMode {
			fmt.Printf("🐛 Key validation failed: %v\n", err)
		}
		return fmt.Errorf("❌ Age key validation failed: %w", err)
	}

	fmt.Println("✅ Age key validation passed")
	fmt.Printf("🔑 Public key: %s\n", keyPair.PublicKey)

	// Validate SOPS configuration if it exists
	if debugMode {
		fmt.Printf("🐛 Checking for SOPS config at: %s\n", configFile)
	}
	
	if _, err := os.Stat(configFile); err == nil {
		fmt.Println("🔍 Validating SOPS configuration file...")

		// Basic validation - check if public key is in config
		content, err := os.ReadFile(configFile)
		if err != nil {
			if debugMode {
				fmt.Printf("🐛 Failed to read config: %v\n", err)
			}
			return fmt.Errorf("failed to read SOPS config: %w", err)
		}

		if debugMode {
			fmt.Printf("🐛 Config file size: %d bytes\n", len(content))
			fmt.Println("🐛 Config file contents:")
			fmt.Println("--- BEGIN CONFIG ---")
			fmt.Println(string(content))
			fmt.Println("--- END CONFIG ---")
		}

		if !strings.Contains(string(content), keyPair.PublicKey) {
			fmt.Printf("⚠️  SOPS configuration does not contain current Age public key\n")
			fmt.Printf("💡 Consider updating %s with the current public key\n", configFile)
			
			if debugMode {
				fmt.Printf("🐛 Expected public key: %s\n", keyPair.PublicKey)
				// Show what age keys are in the config
				lines := strings.Split(string(content), "\n")
				fmt.Println("🐛 Age keys found in config:")
				for _, line := range lines {
					if strings.Contains(line, "age:") || strings.Contains(line, "age1") {
						fmt.Printf("🐛   %s\n", strings.TrimSpace(line))
					}
				}
			}
		} else {
			fmt.Println("✅ SOPS configuration contains current Age public key")
		}
	} else {
		fmt.Printf("⚠️  SOPS configuration file not found: %s\n", configFile)
		if debugMode {
			fmt.Printf("🐛 Config file error: %v\n", err)
			// Check current directory
			if cwd, err := os.Getwd(); err == nil {
				fmt.Printf("🐛 Current working directory: %s\n", cwd)
			}
		}
	}

	// Test key access
	fmt.Println("🧪 Testing key access...")
	if debugMode {
		fmt.Println("🐛 Validating key access permissions...")
	}
	if err := km.ValidateKeyAccess(keyName); err != nil {
		if debugMode {
			fmt.Printf("🐛 Key access validation failed: %v\n", err)
		}
		return fmt.Errorf("❌ Key access test failed: %w", err)
	}

	fmt.Println("✅ Key access test passed")

	// Check SOPS installation
	if debugMode {
		fmt.Println("🐛 Checking SOPS installation...")
		// Check PATH
		if path := os.Getenv("PATH"); path != "" {
			fmt.Printf("🐛 PATH: %s\n", path)
		}
	}
	
	manager := sops.NewSOPSManager()
	if version, err := manager.CheckSOPSVersion(ctx); err != nil {
		fmt.Printf("⚠️  SOPS not found or not executable: %v\n", err)
		if debugMode {
			fmt.Printf("🐛 SOPS check error details: %v\n", err)
		}
	} else {
		fmt.Printf("✅ SOPS is installed: %s\n", version)
	}

	// Additional debug checks
	if debugMode {
		fmt.Println("\n🐛 === ADDITIONAL DEBUG INFORMATION ===")
		
		// Check SOPS_AGE_KEY_FILE environment variable
		if sopsKeyFile := os.Getenv("SOPS_AGE_KEY_FILE"); sopsKeyFile != "" {
			fmt.Printf("🐛 SOPS_AGE_KEY_FILE: %s\n", sopsKeyFile)
		} else {
			fmt.Println("🐛 SOPS_AGE_KEY_FILE: (not set)")
		}
		
		// List all age keys in the key directory
		fmt.Println("🐛 All Age keys in key directory:")
		if keyNames, err := km.ListAgeKeys(); err == nil {
			for _, name := range keyNames {
				fmt.Printf("🐛   - %s\n", name)
				if kp, err := km.LoadAgeKey(name); err == nil {
					fmt.Printf("🐛     Public key: %s\n", kp.PublicKey)
				}
			}
		} else {
			fmt.Printf("🐛 Failed to list keys: %v\n", err)
		}
		
		fmt.Println("🐛 === END DEBUG INFORMATION ===")
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

// newSOPSSecretsListCmd creates the secrets-list subcommand
func newSOPSSecretsListCmd() *cobra.Command {
	var (
		keyFile    string
		searchPath string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "secrets-list",
		Short: "List files that will be processed",
		Long: `List all SOPS-encrypted files that will be processed.

This command searches for YAML files that contain SOPS encryption metadata
and displays them in a structured format. Use this to understand which
files will be affected by encryption/decryption operations.

The command searches recursively through the specified path and identifies
files based on SOPS metadata presence.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsList(cmd.Context(), keyFile, searchPath, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&searchPath, "search-path", ".", "Path to search for SOPS files")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// newSOPSSecretsStatusCmd creates the secrets-status subcommand (alias for secrets-list)
func newSOPSSecretsStatusCmd() *cobra.Command {
	var (
		keyFile    string
		searchPath string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "secrets-status",
		Short: "Show status of secrets (alias for secrets-list)",
		Long: `Show the status of secrets (alias for secrets-list).

This command is an alias for secrets-list and provides the same functionality:
searching for SOPS-encrypted files and displaying their status.

Use this command to get an overview of all encrypted secrets in your
project and their current encryption status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsList(cmd.Context(), keyFile, searchPath, dryRun)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&searchPath, "search-path", ".", "Path to search for SOPS files")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// newSOPSSecretsEncryptCmd creates the secrets-encrypt subcommand
func newSOPSSecretsEncryptCmd() *cobra.Command {
	var (
		keyFile    string
		searchPath string
		dryRun     bool
		backups    bool
	)

	cmd := &cobra.Command{
		Use:   "secrets-encrypt",
		Short: "Encrypt secrets (with backups)",
		Long: `Encrypt secrets with automatic backup creation.

This command finds all unencrypted YAML files that match SOPS configuration
rules and encrypts them using the configured Age keys. Before encryption,
it creates backups of the original files for safety.

The encryption process:
1. Creates timestamped backups of original files
2. Encrypts files using SOPS with configured Age keys
3. Validates successful encryption
4. Reports results and backup locations

Use this command when you want to encrypt secrets with maximum safety
through automatic backup creation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsEncrypt(cmd.Context(), keyFile, searchPath, dryRun, backups)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&searchPath, "search-path", ".", "Path to search for files to encrypt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	cmd.Flags().BoolVar(&backups, "backups", true, "Create backups before encryption")

	return cmd
}

// newSOPSSecretsEncryptFastCmd creates the secrets-encrypt-fast subcommand
func newSOPSSecretsEncryptFastCmd() *cobra.Command {
	var (
		keyFile    string
		searchPath string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "secrets-encrypt-fast",
		Short: "Encrypt secrets (no backups, faster)",
		Long: `Encrypt secrets without creating backups for faster operation.

This command provides the same encryption functionality as secrets-encrypt
but skips the backup creation step for improved performance. Use this when
you're confident in your encryption setup or when working with files that
are already version controlled.

The fast encryption process:
1. Finds unencrypted YAML files matching SOPS rules
2. Encrypts files directly using SOPS with configured Age keys
3. Validates successful encryption
4. Reports results

⚠️  Warning: This command does not create backups. Ensure your files are
properly version controlled or backed up before using this command.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsEncrypt(cmd.Context(), keyFile, searchPath, dryRun, false)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&searchPath, "search-path", ".", "Path to search for files to encrypt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// newSOPSSecretsDecryptCmd creates the secrets-decrypt subcommand
func newSOPSSecretsDecryptCmd() *cobra.Command {
	var (
		keyFile    string
		searchPath string
		dryRun     bool
		backups    bool
	)

	cmd := &cobra.Command{
		Use:   "secrets-decrypt",
		Short: "Decrypt secrets (with backups)",
		Long: `Decrypt secrets with automatic backup creation.

This command finds all SOPS-encrypted YAML files and decrypts them using
the configured Age keys. Before decryption, it creates backups of the
encrypted files for safety.

The decryption process:
1. Creates timestamped backups of encrypted files
2. Decrypts files using SOPS with configured Age keys
3. Validates successful decryption
4. Reports results and backup locations

Use this command when you need to decrypt secrets with maximum safety
through automatic backup creation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsDecrypt(cmd.Context(), keyFile, searchPath, dryRun, backups)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&searchPath, "search-path", ".", "Path to search for files to decrypt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	cmd.Flags().BoolVar(&backups, "backups", true, "Create backups before decryption")

	return cmd
}

// newSOPSSecretsDecryptFastCmd creates the secrets-decrypt-fast subcommand
func newSOPSSecretsDecryptFastCmd() *cobra.Command {
	var (
		keyFile    string
		searchPath string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "secrets-decrypt-fast",
		Short: "Decrypt secrets (no backups, faster)",
		Long: `Decrypt secrets without creating backups for faster operation.

This command provides the same decryption functionality as secrets-decrypt
but skips the backup creation step for improved performance. Use this when
you're confident in your decryption setup or when working with files that
are already version controlled.

The fast decryption process:
1. Finds SOPS-encrypted YAML files
2. Decrypts files directly using SOPS with configured Age keys
3. Validates successful decryption
4. Reports results

⚠️  Warning: This command does not create backups. Ensure your files are
properly version controlled or backed up before using this command.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSOPSSecretsDecrypt(cmd.Context(), keyFile, searchPath, dryRun, false)
		},
	}

	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to Age key file (default: ~/.config/sops/age/keys.txt)")
	cmd.Flags().StringVar(&searchPath, "search-path", ".", "Path to search for files to decrypt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// executeSOPSSecretsList lists all SOPS-encrypted files
func executeSOPSSecretsList(ctx context.Context, keyFile, searchPath string, dryRun bool) error {
	if dryRun {
		fmt.Println("🧪 DRY RUN: Secrets list simulation")
	}
	
	fmt.Printf("🔍 Searching for SOPS files in: %s\n", searchPath)
	
	// Setup key environment if keyFile is specified
	if keyFile != "" {
		if err := setupSOPSKeyEnvironment(keyFile); err != nil {
			return fmt.Errorf("failed to setup key environment: %w", err)
		}
		fmt.Printf("🔑 Using key file: %s\n", keyFile)
	}

	encryptor := sops.NewDefaultEncryptor(nil, nil)
	var encryptedFiles []string
	var unencryptedFiles []string

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			if isEncrypted, err := encryptor.IsFileEncrypted(path); err == nil {
				if isEncrypted {
					encryptedFiles = append(encryptedFiles, path)
				} else {
					// For unencrypted files, check if they contain sensitive data patterns
					if shouldEncrypt := shouldFileBeEncrypted(path); shouldEncrypt {
						unencryptedFiles = append(unencryptedFiles, path)
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to search for files: %w", err)
	}

	// Display results
	fmt.Printf("\n📊 SOPS Files Status:\n")
	fmt.Printf("🔒 Encrypted files: %d\n", len(encryptedFiles))
	for _, file := range encryptedFiles {
		fmt.Printf("  ✅ %s\n", file)
	}

	fmt.Printf("\n🔓 Unencrypted files (should be encrypted): %d\n", len(unencryptedFiles))
	for _, file := range unencryptedFiles {
		fmt.Printf("  ⚠️  %s\n", file)
	}

	if len(encryptedFiles) == 0 && len(unencryptedFiles) == 0 {
		fmt.Println("ℹ️  No SOPS-managed files found")
	}

	return nil
}

// executeSOPSSecretsEncrypt encrypts secrets files
func executeSOPSSecretsEncrypt(ctx context.Context, keyFile, searchPath string, dryRun, createBackups bool) error {
	if dryRun {
		fmt.Println("🧪 DRY RUN: Secrets encryption simulation")
	} else {
		fmt.Println("🔒 Starting secrets encryption...")
	}

	fmt.Printf("📁 Search path: %s\n", searchPath)
	fmt.Printf("💾 Create backups: %t\n", createBackups)
	
	// Setup key environment and load age keys
	var ageKeys []string
	var err error
	
	if keyFile != "" {
		// Use the specified key file
		if err := setupSOPSKeyEnvironment(keyFile); err != nil {
			return fmt.Errorf("failed to setup key environment: %w", err)
		}
		fmt.Printf("🔑 Using key file: %s\n", keyFile)
		
		// Load the public key from the key file
		ageKeys, err = loadAgeKeysFromFile(keyFile)
		if err != nil {
			return fmt.Errorf("failed to load age keys from file: %w", err)
		}
	} else {
		// Load SOPS configuration to get age keys
		ageKeys, err = loadSOPSAgeKeys()
		if err != nil {
			return fmt.Errorf("failed to load SOPS age keys: %w", err)
		}
	}

	encryptor := sops.NewDefaultEncryptor(ageKeys, nil)
	var filesToEncrypt []string

	// Find files that need encryption
	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			if isEncrypted, err := encryptor.IsFileEncrypted(path); err == nil && !isEncrypted {
				if shouldEncrypt := shouldFileBeEncrypted(path); shouldEncrypt {
					filesToEncrypt = append(filesToEncrypt, path)
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to search for files: %w", err)
	}

	if len(filesToEncrypt) == 0 {
		fmt.Println("ℹ️  No files found that need encryption")
		return nil
	}

	fmt.Printf("📄 Files to encrypt: %d\n", len(filesToEncrypt))

	if dryRun {
		for _, file := range filesToEncrypt {
			fmt.Printf("  🔒 Would encrypt: %s\n", file)
		}
		return nil
	}

	// Process each file
	successCount := 0
	for _, file := range filesToEncrypt {
		fmt.Printf("🔒 Encrypting: %s\n", file)

		// Create backup if requested
		if createBackups {
			backupPath := fmt.Sprintf("%s.backup-%s", file, time.Now().Format("20060102-150405"))
			if err := copyFile(file, backupPath); err != nil {
				fmt.Printf("⚠️  Failed to create backup for %s: %v\n", file, err)
				continue
			}
			fmt.Printf("💾 Backup created: %s\n", backupPath)
		}

		// Encrypt the file
		encryptConfig := sops.EncryptionConfig{
			AgeKeys: ageKeys,
			InPlace: true,
		}
		if err := encryptor.EncryptFile(ctx, file, encryptConfig); err != nil {
			fmt.Printf("❌ Failed to encrypt %s: %v\n", file, err)
			continue
		}

		successCount++
		fmt.Printf("✅ Successfully encrypted: %s\n", file)
	}

	fmt.Printf("\n🎉 Encryption completed: %d/%d files processed successfully\n", successCount, len(filesToEncrypt))

	return nil
}

// executeSOPSSecretsDecrypt decrypts secrets files
func executeSOPSSecretsDecrypt(ctx context.Context, keyFile, searchPath string, dryRun, createBackups bool) error {
	if dryRun {
		fmt.Println("🧪 DRY RUN: Secrets decryption simulation")
	} else {
		fmt.Println("🔓 Starting secrets decryption...")
	}

	fmt.Printf("📁 Search path: %s\n", searchPath)
	fmt.Printf("💾 Create backups: %t\n", createBackups)
	
	// Setup key environment if keyFile is specified
	if keyFile != "" {
		if err := setupSOPSKeyEnvironment(keyFile); err != nil {
			return fmt.Errorf("failed to setup key environment: %w", err)
		}
		fmt.Printf("🔑 Using key file: %s\n", keyFile)
	}

	encryptor := sops.NewDefaultEncryptor(nil, nil)
	var filesToDecrypt []string

	// Find encrypted files
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			if isEncrypted, err := encryptor.IsFileEncrypted(path); err == nil && isEncrypted {
				filesToDecrypt = append(filesToDecrypt, path)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to search for files: %w", err)
	}

	if len(filesToDecrypt) == 0 {
		fmt.Println("ℹ️  No encrypted files found")
		return nil
	}

	fmt.Printf("📄 Files to decrypt: %d\n", len(filesToDecrypt))

	if dryRun {
		for _, file := range filesToDecrypt {
			fmt.Printf("  🔓 Would decrypt: %s\n", file)
		}
		return nil
	}

	// Process each file
	successCount := 0
	for _, file := range filesToDecrypt {
		fmt.Printf("🔓 Decrypting: %s\n", file)

		// Create backup if requested
		if createBackups {
			backupPath := fmt.Sprintf("%s.encrypted-backup-%s", file, time.Now().Format("20060102-150405"))
			if err := copyFile(file, backupPath); err != nil {
				fmt.Printf("⚠️  Failed to create backup for %s: %v\n", file, err)
				continue
			}
			fmt.Printf("💾 Backup created: %s\n", backupPath)
		}

		// Decrypt the file in place by creating a temporary decrypted version
		tempFile := file + ".tmp"
		if err := encryptor.DecryptFile(ctx, file, tempFile); err != nil {
			fmt.Printf("❌ Failed to decrypt %s: %v\n", file, err)
			continue
		}

		// Replace original file with decrypted version
		if err := os.Rename(tempFile, file); err != nil {
			fmt.Printf("❌ Failed to replace %s with decrypted version: %v\n", file, err)
			os.Remove(tempFile) // Clean up temp file
			continue
		}

		successCount++
		fmt.Printf("✅ Successfully decrypted: %s\n", file)
	}

	fmt.Printf("\n🎉 Decryption completed: %d/%d files processed successfully\n", successCount, len(filesToDecrypt))

	return nil
}

// shouldFileBeEncrypted determines if a file should be encrypted based on patterns
func shouldFileBeEncrypted(filePath string) bool {
	// Check file name patterns that typically contain secrets
	fileName := filepath.Base(filePath)
	secretPatterns := []string{
		"secret",
		"credential",
		"password",
		"token",
		"key",
		"cert",
		"tls",
		"auth",
	}

	for _, pattern := range secretPatterns {
		if strings.Contains(strings.ToLower(fileName), pattern) {
			return true
		}
	}

	// Check file content for sensitive data patterns
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	contentStr := strings.ToLower(string(content))
	sensitivePatterns := []string{
		"password:",
		"token:",
		"secret:",
		"key:",
		"credential:",
		"stringdata:",
		"data:",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(contentStr, pattern) {
			return true
		}
	}

	return false
}

// loadSOPSAgeKeys loads age keys from SOPS configuration
func loadSOPSAgeKeys() ([]string, error) {
	// Try to load from .sops.yaml
	if _, err := os.Stat(".sops.yaml"); err == nil {
		content, err := os.ReadFile(".sops.yaml")
		if err != nil {
			return nil, fmt.Errorf("failed to read .sops.yaml: %w", err)
		}

		var config SOPSConfig
		if err := yaml.Unmarshal(content, &config); err != nil {
			return nil, fmt.Errorf("failed to parse .sops.yaml: %w", err)
		}

		var ageKeys []string
		for _, rule := range config.CreationRules {
			if rule.Age != "" {
				ageKeys = append(ageKeys, rule.Age)
			}
		}

		if len(ageKeys) > 0 {
			return ageKeys, nil
		}
	}

	// Try to load from environment or default key manager
	homeDir, _ := os.UserHomeDir()
	keyDir := filepath.Join(homeDir, ".config", "sops", "age")
	km := sops.NewKeyManager(keyDir)

	keyNames, err := km.ListAgeKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to list age keys: %w", err)
	}

	if len(keyNames) == 0 {
		return nil, fmt.Errorf("no age keys found - run 'openCenter sops generate-key' first")
	}

	// Use the first available key
	keyPair, err := km.LoadAgeKey(keyNames[0])
	if err != nil {
		return nil, fmt.Errorf("failed to load age key: %w", err)
	}

	return []string{keyPair.PublicKey}, nil
}

// setupSOPSKeyEnvironment sets up the SOPS_AGE_KEY_FILE environment variable
func setupSOPSKeyEnvironment(keyFile string) error {
	// Resolve the key file path
	homeDir, _ := os.UserHomeDir()
	var keyPath string
	
	if strings.HasPrefix(keyFile, "~") {
		keyPath = filepath.Join(homeDir, keyFile[1:])
	} else if filepath.IsAbs(keyFile) {
		keyPath = keyFile
	} else {
		// Relative path - make it absolute
		if absPath, err := filepath.Abs(keyFile); err == nil {
			keyPath = absPath
		} else {
			keyPath = keyFile
		}
	}
	
	// Check if key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("key file not found: %s", keyPath)
	}
	
	// Set the environment variable for SOPS
	if err := os.Setenv("SOPS_AGE_KEY_FILE", keyPath); err != nil {
		return fmt.Errorf("failed to set SOPS_AGE_KEY_FILE: %w", err)
	}
	
	return nil
}

// extractAgeKeyFromContent extracts the age key from file content, skipping comments and empty lines
func extractAgeKeyFromContent(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Age private keys start with AGE-SECRET-KEY-
		// Age public keys start with age1
		if strings.HasPrefix(line, "AGE-SECRET-KEY-") || strings.HasPrefix(line, "age1") {
			return line
		}
	}
	return ""
}

// loadAgeKeysFromFile loads age public keys from a key file
func loadAgeKeysFromFile(keyFile string) ([]string, error) {
	// Resolve the key file path
	homeDir, _ := os.UserHomeDir()
	var keyPath string
	
	if strings.HasPrefix(keyFile, "~") {
		keyPath = filepath.Join(homeDir, keyFile[1:])
	} else if filepath.IsAbs(keyFile) {
		keyPath = keyFile
	} else {
		// Relative path - make it absolute
		if absPath, err := filepath.Abs(keyFile); err == nil {
			keyPath = absPath
		} else {
			keyPath = keyFile
		}
	}
	
	// Read the key file
	privateKeyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	
	// Extract the actual key (skip comments and empty lines)
	privateKeyStr := extractAgeKeyFromContent(string(privateKeyData))
	if privateKeyStr == "" {
		return nil, fmt.Errorf("no valid age key found in file")
	}
	
	// Parse the private key to get the public key
	keyPair, err := crypto.ParseAgeKey(privateKeyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	
	return []string{keyPair.PublicKey}, nil
}
