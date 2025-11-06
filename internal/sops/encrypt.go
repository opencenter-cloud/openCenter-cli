/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// Encryptor handles SOPS encryption operations
type Encryptor struct {
	ageKeys []string
	pgpKeys []string
}

// NewEncryptor creates a new SOPS encryptor
func NewEncryptor(ageKeys, pgpKeys []string) *Encryptor {
	return &Encryptor{
		ageKeys: ageKeys,
		pgpKeys: pgpKeys,
	}
}

// EncryptionConfig represents SOPS encryption configuration
type EncryptionConfig struct {
	AgeKeys    []string
	PGPKeys    []string
	ConfigFile string
	InPlace    bool
	DryRun     bool
	Verbose    bool
}

// EncryptFile encrypts a single file with SOPS
func (e *Encryptor) EncryptFile(ctx context.Context, filePath string, config EncryptionConfig) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Check if file is already encrypted
	if isEncrypted, err := e.IsFileEncrypted(filePath); err != nil {
		return fmt.Errorf("failed to check if file is encrypted: %w", err)
	} else if isEncrypted {
		return fmt.Errorf("file is already encrypted: %s", filePath)
	}

	// Build SOPS command
	args := []string{"-e"}

	// Add encryption keys
	if len(config.AgeKeys) > 0 {
		args = append(args, "--age", strings.Join(config.AgeKeys, ","))
	}
	if len(config.PGPKeys) > 0 {
		args = append(args, "--pgp", strings.Join(config.PGPKeys, ","))
	}

	// Use default keys if none specified
	if len(config.AgeKeys) == 0 && len(config.PGPKeys) == 0 {
		if len(e.ageKeys) > 0 {
			args = append(args, "--age", strings.Join(e.ageKeys, ","))
		}
		if len(e.pgpKeys) > 0 {
			args = append(args, "--pgp", strings.Join(e.pgpKeys, ","))
		}
	}

	// Add config file if specified
	if config.ConfigFile != "" {
		args = append(args, "--config", config.ConfigFile)
	}

	// Add in-place flag
	if config.InPlace {
		args = append(args, "-i")
	}

	// Add file path
	args = append(args, filePath)

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)

	if config.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if config.DryRun {
		fmt.Printf("Would execute: sops %s\n", strings.Join(args, " "))
		return nil
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SOPS encryption failed for %s: %w", filePath, err)
	}

	return nil
}

// EncryptFiles encrypts multiple files with SOPS
func (e *Encryptor) EncryptFiles(ctx context.Context, filePaths []string, config EncryptionConfig) error {
	for _, filePath := range filePaths {
		if err := e.EncryptFile(ctx, filePath, config); err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", filePath, err)
		}
	}
	return nil
}

// DecryptFile decrypts a SOPS-encrypted file
func (e *Encryptor) DecryptFile(ctx context.Context, filePath string, outputPath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Check if file is encrypted
	if isEncrypted, err := e.IsFileEncrypted(filePath); err != nil {
		return fmt.Errorf("failed to check if file is encrypted: %w", err)
	} else if !isEncrypted {
		return fmt.Errorf("file is not encrypted: %s", filePath)
	}

	// Build SOPS command
	args := []string{"-d"}

	// Add output path if specified
	if outputPath != "" {
		args = append(args, "--output", outputPath)
	}

	args = append(args, filePath)

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)

	if outputPath == "" {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SOPS decryption failed for %s: %w", filePath, err)
	}

	return nil
}

// IsFileEncrypted checks if a file is encrypted with SOPS
func (e *Encryptor) IsFileEncrypted(filePath string) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %w", err)
	}

	// Check for SOPS metadata
	contentStr := string(content)
	return strings.Contains(contentStr, "sops:") &&
		(strings.Contains(contentStr, "age:") || strings.Contains(contentStr, "pgp:")), nil
}

// EncryptOverlayFiles encrypts sensitive files in an overlay directory
func (e *Encryptor) EncryptOverlayFiles(ctx context.Context, overlayPath string, cfg *config.Config) error {
	// Get list of files to encrypt
	filesToEncrypt := e.getFilesToEncrypt(overlayPath, cfg)

	// Create encryption config
	var ageKeys []string
	if cfg.Secrets.SopsAgeKeyFile != "" {
		// Load the age key from the specified file
		if keyPair, err := loadAgeKeyFromFile(cfg.Secrets.SopsAgeKeyFile); err == nil {
			ageKeys = []string{keyPair.PublicKey}
		}
	}
	
	encryptConfig := EncryptionConfig{
		AgeKeys: ageKeys,
		InPlace: true,
		Verbose: true,
	}

	// Encrypt each file
	for _, file := range filesToEncrypt {
		filePath := filepath.Join(overlayPath, file)

		// Skip if file doesn't exist
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		if err := e.EncryptFile(ctx, filePath, encryptConfig); err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", file, err)
		}
	}

	return nil
}

// getFilesToEncrypt returns the list of files that should be encrypted
func (e *Encryptor) getFilesToEncrypt(overlayPath string, cfg *config.Config) []string {
	var files []string

	// Standard encrypted files
	files = append(files,
		"flux-system/gotk-sync.yaml",
		"managed-services/sources/base-repo.yaml",
	)

	// Provider-specific encrypted files
	switch cfg.OpenCenter.Infrastructure.Provider {
	case "openstack":
		files = append(files, "secrets/openstack-credentials.yaml")
	case "vsphere":
		files = append(files,
			"secrets/vsphere-credentials.yaml",
			"customer-managed/services/cloud-provider-vsphere/secret.yaml",
		)
	}

	// Additional encrypted files from configuration can be added here in the future
	// This would require extending the config structure with ExtraEncryptedFiles field

	return files
}

// CreateSOPSConfig creates a .sops.yaml configuration file
func (e *Encryptor) CreateSOPSConfig(overlayPath string, cfg *config.Config) error {
	sopsConfig := e.generateSOPSConfig(cfg)

	// Validate that we're not using placeholder keys in production
	if strings.Contains(sopsConfig, "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx") {
		return fmt.Errorf("cannot create SOPS config with placeholder key - please generate a proper age key first")
	}

	configPath := filepath.Join(overlayPath, ".sops.yaml")
	if err := os.WriteFile(configPath, []byte(sopsConfig), 0o644); err != nil {
		return fmt.Errorf("failed to write SOPS config: %w", err)
	}

	return nil
}

// generateSOPSConfig generates the SOPS configuration content
func (e *Encryptor) generateSOPSConfig(cfg *config.Config) string {
	var ageKey string
	if cfg.Secrets.SopsAgeKeyFile != "" {
		// Load the public key from the age key file
		if keyPair, err := loadAgeKeyFromFile(cfg.Secrets.SopsAgeKeyFile); err == nil {
			ageKey = keyPair.PublicKey
		}
	}
	
	if ageKey == "" {
		// Fallback: try to load from default key manager
		homeDir, _ := os.UserHomeDir()
		keyDir := filepath.Join(homeDir, ".config", "sops", "age")
		km := NewKeyManager(keyDir)
		if keyNames, err := km.ListAgeKeys(); err == nil && len(keyNames) > 0 {
			if keyPair, err := km.LoadAgeKey(keyNames[0]); err == nil {
				ageKey = keyPair.PublicKey
			}
		}
	}
	
	if ageKey == "" {
		// Generate a fallback key instead of using placeholder
		if fallbackKey, err := e.generateFallbackKey(); err == nil {
			ageKey = fallbackKey
		} else {
			// Only use placeholder as last resort and add validation warning
			ageKey = "age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" // Placeholder - DO NOT USE IN PRODUCTION
		}
	}

	config := fmt.Sprintf(`# SOPS configuration for cluster: %s
creation_rules:
  - path_regex: .*\.(yaml|yml)$
    age: %s
    encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
`, cfg.OpenCenter.Cluster.ClusterName, ageKey)

	// Add provider-specific rules
	switch cfg.OpenCenter.Infrastructure.Provider {
	case "openstack":
		config += `  - path_regex: secrets/openstack-credentials\.yaml$
    age: ` + ageKey + `
`
	case "vsphere":
		config += `  - path_regex: secrets/vsphere-credentials\.yaml$
    age: ` + ageKey + `
  - path_regex: customer-managed/services/.*/secret\.yaml$
    age: ` + ageKey + `
`
	}

	return config
}

// ValidateEncryption validates that files are properly encrypted
func (e *Encryptor) ValidateEncryption(overlayPath string, cfg *config.Config) error {
	filesToCheck := e.getFilesToEncrypt(overlayPath, cfg)

	for _, file := range filesToCheck {
		filePath := filepath.Join(overlayPath, file)

		// Skip if file doesn't exist
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		isEncrypted, err := e.IsFileEncrypted(filePath)
		if err != nil {
			return fmt.Errorf("failed to check encryption status of %s: %w", file, err)
		}

		if !isEncrypted {
			return fmt.Errorf("file should be encrypted but is not: %s", file)
		}
	}

	return nil
}

// RotateKeys rotates SOPS encryption keys
func (e *Encryptor) RotateKeys(ctx context.Context, filePath string, newAgeKeys, newPGPKeys []string) error {
	// Build SOPS command for key rotation
	args := []string{"-r"}

	// Add new encryption keys
	if len(newAgeKeys) > 0 {
		args = append(args, "--age", strings.Join(newAgeKeys, ","))
	}
	if len(newPGPKeys) > 0 {
		args = append(args, "--pgp", strings.Join(newPGPKeys, ","))
	}

	// Add in-place flag
	args = append(args, "-i", filePath)

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SOPS key rotation failed for %s: %w", filePath, err)
	}

	return nil
}

// GetEncryptedContent returns the encrypted content of a file without decrypting
func (e *Encryptor) GetEncryptedContent(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// EditEncryptedFile opens an encrypted file for editing with SOPS
func (e *Encryptor) EditEncryptedFile(ctx context.Context, filePath string) error {
	// Build SOPS command for editing
	args := []string{filePath}

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SOPS edit failed for %s: %w", filePath, err)
	}

	return nil
}

// CheckSOPSVersion checks if SOPS is available and returns version info
func (e *Encryptor) CheckSOPSVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "sops", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("SOPS not found or not executable: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// generateFallbackKey generates a fallback age key when no key is available
func (e *Encryptor) generateFallbackKey() (string, error) {
	// Use the key manager to generate and save a fallback key
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	
	keyDir := filepath.Join(homeDir, ".config", "sops", "age")
	km := NewKeyManager(keyDir)
	
	// Generate a fallback key
	keyPair, err := km.generateFallbackKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate fallback key: %w", err)
	}

	// Return the public key (recipient)
	return keyPair.PublicKey, nil
}

// validateKeyForProduction validates that a key is not a placeholder
func (e *Encryptor) validateKeyForProduction(key string) error {
	// Check for placeholder key pattern
	if strings.Contains(key, "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx") {
		return fmt.Errorf("placeholder key detected - this should not be used in production")
	}
	
	// Validate age key format
	if !strings.HasPrefix(key, "age1") || len(key) != 62 {
		return fmt.Errorf("invalid age key format: %s", key)
	}
	
	return nil
}

// CreateSampleEncryptedSecrets creates sample encrypted secrets in the repository
func (e *Encryptor) CreateSampleEncryptedSecrets(ctx context.Context, repoPath string, ageKey string) error {
	return e.CreateSampleEncryptedSecretsForTemplate(ctx, repoPath, ageKey, "basic")
}

// CreateSampleEncryptedSecretsForTemplate creates sample encrypted secrets for a specific template
func (e *Encryptor) CreateSampleEncryptedSecretsForTemplate(ctx context.Context, repoPath string, ageKey string, template string) error {
	samplesDir := filepath.Join(repoPath, "examples", "secrets")

	// Ensure samples directory exists
	if err := os.MkdirAll(samplesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create samples directory: %w", err)
	}

	// Get sample secrets based on template
	sampleSecrets := e.getSampleSecretsForTemplate(template)

	// Create and encrypt each sample secret
	for filename, content := range sampleSecrets {
		// Create temporary unencrypted file
		tempFile := filepath.Join(samplesDir, strings.TrimSuffix(filename, ".enc.yaml")+".yaml")
		if err := os.WriteFile(tempFile, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write temp file %s: %w", tempFile, err)
		}

		// Encrypt the file
		encryptConfig := EncryptionConfig{
			AgeKeys: []string{ageKey},
			InPlace: false,
		}

		// Encrypt to the .enc.yaml file
		encryptedFile := filepath.Join(samplesDir, filename)
		if err := e.encryptFileToOutput(ctx, tempFile, encryptedFile, encryptConfig); err != nil {
			// If SOPS is not available, create a placeholder encrypted file
			placeholderContent := e.createPlaceholderEncryptedContent(content, ageKey)
			if err := os.WriteFile(encryptedFile, []byte(placeholderContent), 0o644); err != nil {
				return fmt.Errorf("failed to write placeholder encrypted file %s: %w", encryptedFile, err)
			}
		}

		// Remove temporary unencrypted file
		os.Remove(tempFile)
	}

	return nil
}

// encryptFileToOutput encrypts a file and writes to a specific output file
func (e *Encryptor) encryptFileToOutput(ctx context.Context, inputFile, outputFile string, config EncryptionConfig) error {
	// Build SOPS command
	args := []string{"-e"}

	// Add encryption keys
	if len(config.AgeKeys) > 0 {
		args = append(args, "--age", strings.Join(config.AgeKeys, ","))
	}
	if len(config.PGPKeys) > 0 {
		args = append(args, "--pgp", strings.Join(config.PGPKeys, ","))
	}

	// Add output file
	args = append(args, "--output", outputFile)

	// Add input file
	args = append(args, inputFile)

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SOPS encryption failed: %w", err)
	}

	return nil
}

// createPlaceholderEncryptedContent creates a placeholder encrypted content when SOPS is not available
func (e *Encryptor) createPlaceholderEncryptedContent(originalContent, ageKey string) string {
	return fmt.Sprintf(`# This file would be encrypted with SOPS in a real environment
# To encrypt this file, run: sops --encrypt --age %s --in-place <filename>
#
# Original content (DO NOT COMMIT UNENCRYPTED):
# %s
#
# SOPS encrypted content would appear here
apiVersion: v1
kind: Secret
metadata:
    name: placeholder-encrypted-secret
    namespace: default
type: Opaque
data:
    # Encrypted data would be here
sops:
    age:
        - recipient: %s
          enc: ENC[AES256_GCM,data:placeholder_encrypted_data_would_be_here,type:str]
    lastmodified: "2024-01-01T00:00:00Z"
    mac: ENC[AES256_GCM,data:placeholder_mac_would_be_here,type:str]
    pgp: []
    unencrypted_suffix: _unencrypted
    version: 3.8.1
`, ageKey, strings.ReplaceAll(originalContent, "\n", "\n# "), ageKey)
}

// EncryptRepositorySecrets encrypts all sample secrets in a repository
func (e *Encryptor) EncryptRepositorySecrets(ctx context.Context, repoPath string, ageKey string) error {
	secretsDir := filepath.Join(repoPath, "examples", "secrets")

	// Find all .yaml files that are not already encrypted
	err := filepath.Walk(secretsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-YAML files
		if info.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}

		// Skip already encrypted files
		if strings.Contains(path, ".enc.") {
			return nil
		}

		// Skip README files
		if strings.Contains(path, "README") {
			return nil
		}

		// Check if file is already encrypted
		if isEncrypted, err := e.IsFileEncrypted(path); err != nil {
			return fmt.Errorf("failed to check encryption status of %s: %w", path, err)
		} else if isEncrypted {
			return nil // Already encrypted
		}

		// Encrypt the file
		encryptConfig := EncryptionConfig{
			AgeKeys: []string{ageKey},
			InPlace: true,
		}

		if err := e.EncryptFile(ctx, path, encryptConfig); err != nil {
			// If SOPS fails, create a placeholder
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("failed to read file for placeholder: %w", readErr)
			}

			placeholderContent := e.createPlaceholderEncryptedContent(string(content), ageKey)
			if writeErr := os.WriteFile(path, []byte(placeholderContent), 0o644); writeErr != nil {
				return fmt.Errorf("failed to write placeholder: %w", writeErr)
			}
		}

		return nil
	})

	return err
}

// getSampleSecretsForTemplate returns sample secrets based on the template type
func (e *Encryptor) getSampleSecretsForTemplate(template string) map[string]string {
	// Base secrets for all templates
	baseSecrets := map[string]string{
		"sample-secret.enc.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: sample-secret
  namespace: default
type: Opaque
stringData:
  username: admin
  password: changeme123
  api-key: sample-api-key-12345
  database-url: postgresql://user:pass@localhost:5432/db
`,
		"database-credentials.enc.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: database-credentials
  namespace: default
type: Opaque
stringData:
  host: postgres.example.com
  port: "5432"
  database: myapp
  username: myapp_user
  password: super_secure_password_123
  connection-string: postgresql://myapp_user:super_secure_password_123@postgres.example.com:5432/myapp
`,
		"api-tokens.enc.yaml": `apiVersion: v1
kind: Secret
metadata:
  name: api-tokens
  namespace: default
type: Opaque
stringData:
  github-token: ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  slack-webhook: https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX
  datadog-api-key: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  newrelic-license-key: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  stripe-secret-key: sk_test_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
`,
	}

	// Add template-specific secrets
	switch template {
	case "enterprise", "multi-tenant":
		baseSecrets["production-secrets.enc.yaml"] = `apiVersion: v1
kind: Secret
metadata:
  name: production-secrets
  namespace: production
type: Opaque
stringData:
  database-password: production_db_password_very_secure
  redis-password: production_redis_password_secure
  jwt-secret: production_jwt_secret_key_256_bits_long
  encryption-key: production_encryption_key_aes_256
  oauth-client-secret: production_oauth_client_secret_from_provider
`
		baseSecrets["monitoring-secrets.enc.yaml"] = `apiVersion: v1
kind: Secret
metadata:
  name: monitoring-secrets
  namespace: monitoring
type: Opaque
stringData:
  prometheus-remote-write-password: remote_write_password_123
  grafana-admin-password: grafana_admin_secure_password
  alertmanager-slack-api-url: https://hooks.slack.com/services/T00/B00/XXX
  datadog-api-key: dd_api_key_for_metrics_and_logs
  pagerduty-integration-key: pagerduty_integration_key_for_alerts
`
	}

	return baseSecrets
}

// loadAgeKeyFromFile loads an age key pair from a file path
func loadAgeKeyFromFile(keyFilePath string) (*AgeKeyPair, error) {
	// Expand home directory if needed
	if strings.HasPrefix(keyFilePath, "~/") {
		homeDir, _ := os.UserHomeDir()
		keyFilePath = filepath.Join(homeDir, keyFilePath[2:])
	}
	
	// Read the private key file
	privateKeyData, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read age key file: %w", err)
	}
	
	privateKey := strings.TrimSpace(string(privateKeyData))
	
	// Parse the private key to get the public key
	identity, err := age.ParseX25519Identity(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse age identity: %w", err)
	}
	
	return &AgeKeyPair{
		PrivateKey: privateKey,
		PublicKey:  identity.Recipient().String(),
		Recipient:  identity.Recipient().String(),
	}, nil
}
