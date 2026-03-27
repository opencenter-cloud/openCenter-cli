package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
)

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

			// Skip excluded directories
			if info.IsDir() && shouldSkipDirectory(path) {
				return filepath.SkipDir
			}

			if !info.IsDir() && isSOPSYAMLFile(path) {
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

		// Skip excluded directories
		if info.IsDir() && shouldSkipDirectory(path) {
			return filepath.SkipDir
		}

		if !info.IsDir() && isSOPSYAMLFile(path) {
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
