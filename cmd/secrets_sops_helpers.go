package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
)

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

// SOPSPathMatcher handles path matching based on SOPS configuration
type SOPSPathMatcher struct {
	rules []*SOPSPathRule
}

// SOPSPathRule represents a compiled SOPS creation rule
type SOPSPathRule struct {
	pathRegex      *regexp2.Regexp
	encryptedRegex *regexp2.Regexp
	original       CreationRule
}

// NewSOPSPathMatcher creates a new path matcher from SOPS configuration
func NewSOPSPathMatcher(configPath string) (*SOPSPathMatcher, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return empty matcher if no config exists
		return &SOPSPathMatcher{rules: []*SOPSPathRule{}}, nil
	}

	// Load SOPS configuration
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SOPS config: %w", err)
	}

	var config SOPSConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("failed to parse SOPS config: %w", err)
	}

	// Compile regex patterns
	matcher := &SOPSPathMatcher{
		rules: make([]*SOPSPathRule, 0, len(config.CreationRules)),
	}

	for _, rule := range config.CreationRules {
		pathRule := &SOPSPathRule{
			original: rule,
		}

		// Compile path regex if present
		if rule.PathRegex != "" {
			pathRegex, err := regexp2.Compile(rule.PathRegex, regexp2.None)
			if err != nil {
				return nil, fmt.Errorf("failed to compile path_regex '%s': %w", rule.PathRegex, err)
			}
			pathRule.pathRegex = pathRegex
		}

		// Compile encrypted_regex if present
		if rule.EncryptedRegex != "" {
			encryptedRegex, err := regexp2.Compile(rule.EncryptedRegex, regexp2.None)
			if err != nil {
				return nil, fmt.Errorf("failed to compile encrypted_regex '%s': %w", rule.EncryptedRegex, err)
			}
			pathRule.encryptedRegex = encryptedRegex
		}

		matcher.rules = append(matcher.rules, pathRule)
	}

	return matcher, nil
}

// ShouldEncryptPath checks if a path should be encrypted based on SOPS rules
func (m *SOPSPathMatcher) ShouldEncryptPath(path string) bool {
	// If no rules, fall back to basic pattern matching
	if len(m.rules) == 0 {
		return shouldFileBeEncrypted(path)
	}

	// Normalize path to use forward slashes
	normalizedPath := filepath.ToSlash(path)

	// Check each rule
	for _, rule := range m.rules {
		if rule.pathRegex != nil {
			match, err := rule.pathRegex.MatchString(normalizedPath)
			if err != nil {
				// If regex fails, skip this rule
				continue
			}
			if match {
				return true
			}
		}
	}

	return false
}

// ShouldSkipPath checks if a path should be skipped (excluded from encryption)
func (m *SOPSPathMatcher) ShouldSkipPath(path string) bool {
	// If no rules, fall back to directory-based exclusion
	if len(m.rules) == 0 {
		return false
	}

	// Normalize path to use forward slashes
	normalizedPath := filepath.ToSlash(path)

	// Check each rule - if path matches a rule with negative lookahead,
	// it means the path is explicitly excluded
	for _, rule := range m.rules {
		if rule.pathRegex != nil {
			match, err := rule.pathRegex.MatchString(normalizedPath)
			if err != nil {
				continue
			}

			// If the pattern contains negative lookahead
			if strings.Contains(rule.original.PathRegex, "(?!") {
				// Extract the base pattern before the negative lookahead
				basePattern := extractBasePattern(rule.original.PathRegex)
				if basePattern != "" && strings.HasPrefix(normalizedPath, basePattern) {
					// If the path starts with the base pattern but doesn't match the full regex,
					// it means it's in an excluded directory
					if !match {
						return true
					}
				}
			}
		}
	}

	return false
}

// shouldSkipDirectory determines if a directory should be skipped during file walking
func shouldSkipDirectory(dirPath string) bool {
	// List of directory patterns to exclude from encryption
	excludedDirs := []string{
		"venv",
		".venv",
		"kubespray",
		".terraform",
		".bin",
		"node_modules",
		".git",
		"__pycache__",
		".pytest_cache",
		".mypy_cache",
		".tox",
		"vendor",
		"target",
		"build",
		"dist",
	}

	// Get the directory name
	dirName := filepath.Base(dirPath)

	// Check if directory name matches any excluded pattern
	for _, excluded := range excludedDirs {
		if dirName == excluded {
			return true
		}
	}

	return false
}

// isSOPSYAMLFile returns true if the file extension indicates a YAML file that
// SOPS should process. This includes standard .yaml/.yml as well as .yaml.enc
// and .yml.enc variants used for encrypted kubeconfigs and similar artifacts.
func isSOPSYAMLFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".yaml.enc") ||
		strings.HasSuffix(lower, ".yml.enc")
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
		return nil, fmt.Errorf("no age keys found - run 'opencenter sops generate-key' first")
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

// extractBasePattern extracts the base path pattern before negative lookahead
func extractBasePattern(pattern string) string {
	// Extract pattern like "infrastructure/clusters/test-cluster/" from
	// "^infrastructure\/clusters\/test-cluster\/(?!(?:venv|kubespray|\.terraform|\.bin)\/)(.*)"

	// Remove anchors and escape sequences
	pattern = strings.TrimPrefix(pattern, "^")
	pattern = strings.ReplaceAll(pattern, "\\/", "/")

	// Find the negative lookahead position
	if idx := strings.Index(pattern, "(?!"); idx > 0 {
		return pattern[:idx]
	}

	return ""
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

// executeSOPSSecretsEncrypt encrypts secrets files
func executeSOPSSecretsEncrypt(ctx context.Context, keyFile, searchPath string, dryRun, createBackups bool) error {
	if dryRun {
		fmt.Println("🧪 DRY RUN: Secrets encryption simulation")
	} else {
		fmt.Println("🔒 Starting secrets encryption...")
	}

	fmt.Printf("📁 Search path: %s\n", searchPath)
	fmt.Printf("💾 Create backups: %t\n", createBackups)

	// Load SOPS configuration and create path matcher
	sopsConfigPath := ".sops.yaml"
	pathMatcher, err := NewSOPSPathMatcher(sopsConfigPath)
	if err != nil {
		fmt.Printf("⚠️  Failed to load SOPS configuration: %v\n", err)
		fmt.Println("ℹ️  Falling back to basic pattern matching")
		pathMatcher = &SOPSPathMatcher{rules: []*SOPSPathRule{}}
	} else if len(pathMatcher.rules) > 0 {
		fmt.Printf("✅ Loaded SOPS configuration with %d rules\n", len(pathMatcher.rules))
	}

	// Setup key environment and load age keys
	var ageKeys []string

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

		// Skip excluded directories (basic exclusion)
		if info.IsDir() && shouldSkipDirectory(path) {
			if dryRun {
				fmt.Printf("⏭️  Skipping directory: %s\n", path)
			}
			return filepath.SkipDir
		}

		// Check if path should be skipped based on SOPS rules
		if pathMatcher.ShouldSkipPath(path) {
			if dryRun && !info.IsDir() {
				fmt.Printf("⏭️  Skipping (SOPS rule): %s\n", path)
			}
			return nil
		}

		if !info.IsDir() && isSOPSYAMLFile(path) {
			if isEncrypted, err := encryptor.IsFileEncrypted(path); err == nil && !isEncrypted {
				// Use SOPS path matcher if available, otherwise fall back to basic matching
				shouldEncrypt := pathMatcher.ShouldEncryptPath(path)
				if !shouldEncrypt {
					shouldEncrypt = shouldFileBeEncrypted(path)
				}

				if shouldEncrypt {
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

	// Load SOPS configuration and create path matcher
	sopsConfigPath := ".sops.yaml"
	pathMatcher, err := NewSOPSPathMatcher(sopsConfigPath)
	if err != nil {
		fmt.Printf("⚠️  Failed to load SOPS configuration: %v\n", err)
		pathMatcher = &SOPSPathMatcher{rules: []*SOPSPathRule{}}
	} else if len(pathMatcher.rules) > 0 {
		fmt.Printf("✅ Loaded SOPS configuration with %d rules\n", len(pathMatcher.rules))
	}

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
	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories
		if info.IsDir() && shouldSkipDirectory(path) {
			if dryRun {
				fmt.Printf("⏭️  Skipping directory: %s\n", path)
			}
			return filepath.SkipDir
		}

		// Check if path should be skipped based on SOPS rules
		if pathMatcher.ShouldSkipPath(path) {
			if dryRun && !info.IsDir() {
				fmt.Printf("⏭️  Skipping (SOPS rule): %s\n", path)
			}
			return nil
		}

		if !info.IsDir() && isSOPSYAMLFile(path) {
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

// executeSOPSSecretsList lists all SOPS-encrypted files
func executeSOPSSecretsList(ctx context.Context, keyFile, searchPath string, dryRun bool) error {
	if dryRun {
		fmt.Println("🧪 DRY RUN: Secrets list simulation")
	}

	fmt.Printf("🔍 Searching for SOPS files in: %s\n", searchPath)

	// Load SOPS configuration and create path matcher
	sopsConfigPath := ".sops.yaml"
	pathMatcher, err := NewSOPSPathMatcher(sopsConfigPath)
	if err != nil {
		fmt.Printf("⚠️  Failed to load SOPS configuration: %v\n", err)
		fmt.Println("ℹ️  Falling back to basic pattern matching")
		pathMatcher = &SOPSPathMatcher{rules: []*SOPSPathRule{}}
	} else if len(pathMatcher.rules) > 0 {
		fmt.Printf("✅ Loaded SOPS configuration with %d rules\n", len(pathMatcher.rules))
	}

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
	var skippedDirs []string

	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories
		if info.IsDir() && shouldSkipDirectory(path) {
			skippedDirs = append(skippedDirs, path)
			return filepath.SkipDir
		}

		// Check if path should be skipped based on SOPS rules
		if pathMatcher.ShouldSkipPath(path) {
			return nil
		}

		if !info.IsDir() && isSOPSYAMLFile(path) {
			if isEncrypted, err := encryptor.IsFileEncrypted(path); err == nil {
				if isEncrypted {
					encryptedFiles = append(encryptedFiles, path)
				} else {
					// Use SOPS path matcher if available, otherwise fall back to basic matching
					shouldEncrypt := pathMatcher.ShouldEncryptPath(path)
					if !shouldEncrypt {
						shouldEncrypt = shouldFileBeEncrypted(path)
					}

					if shouldEncrypt {
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
	if len(skippedDirs) > 0 && dryRun {
		fmt.Printf("\n⏭️  Skipped directories: %d\n", len(skippedDirs))
		for _, dir := range skippedDirs {
			fmt.Printf("  • %s\n", dir)
		}
	}

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
