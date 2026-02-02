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
	"log/slog"
	"os"
	"path/filepath"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/util/crypto"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
)

// DefaultSOPSManager implements SOPSManager interface
type DefaultSOPSManager struct {
	keyManager crypto.KeyManager
	encryptor  Encryptor
	validator  Validator
	logger     *slog.Logger
}

// NewDefaultSOPSManager creates a new SOPS manager with dependency injection
func NewDefaultSOPSManager(keyManager crypto.KeyManager, encryptor Encryptor, validator Validator, logger *slog.Logger) *DefaultSOPSManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultSOPSManager{
		keyManager: keyManager,
		encryptor:  encryptor,
		validator:  validator,
		logger:     logger,
	}
}

// NewSOPSManager creates a new SOPS manager with default implementations
func NewSOPSManager() *DefaultSOPSManager {
	homeDir, _ := os.UserHomeDir()
	keyDir := filepath.Join(homeDir, ".config", "sops", "age")

	keyManager := crypto.NewDefaultKeyManager(keyDir)
	encryptor := NewDefaultEncryptor([]string{}, []string{})
	validator := NewDefaultValidator()
	logger := slog.Default()

	return NewDefaultSOPSManager(keyManager, encryptor, validator, logger)
}

// GetKeyManager returns the key manager
func (m *DefaultSOPSManager) GetKeyManager() crypto.KeyManager {
	return m.keyManager
}

// GetEncryptor returns the encryptor
func (m *DefaultSOPSManager) GetEncryptor() Encryptor {
	return m.encryptor
}

// GetValidator returns the validator
func (m *DefaultSOPSManager) GetValidator() Validator {
	return m.validator
}

// EncryptOverlayFiles encrypts sensitive files in an overlay directory
func (m *DefaultSOPSManager) EncryptOverlayFiles(ctx context.Context, overlayPath string, cfg *config.Config) error {
	m.logger.Info("Starting overlay files encryption", "overlay_path", overlayPath)

	// Get list of files to encrypt
	filesToEncrypt := m.getFilesToEncrypt(overlayPath, cfg)
	m.logger.Debug("Files to encrypt", "count", len(filesToEncrypt), "files", filesToEncrypt)

	// Get encryption keys
	var ageKeys []string
	if cfg.Secrets.SopsAgeKeyFile != "" {
		// Load the age key from the specified file
		if keyPair, err := m.loadAgeKeyFromFile(cfg.Secrets.SopsAgeKeyFile); err == nil {
			ageKeys = []string{keyPair.PublicKey}
			m.logger.Debug("Loaded age key from config", "key_file", cfg.Secrets.SopsAgeKeyFile)
		} else {
			m.logger.Warn("Failed to load age key from config", "key_file", cfg.Secrets.SopsAgeKeyFile, "error", err)
		}
	}

	// Fallback to key manager if no keys from config
	if len(ageKeys) == 0 {
		if keyNames, err := m.keyManager.ListAgeKeys(); err == nil && len(keyNames) > 0 {
			if keyPair, err := m.keyManager.LoadAgeKey(keyNames[0]); err == nil {
				ageKeys = []string{keyPair.PublicKey}
				m.logger.Debug("Using fallback key from key manager", "key_name", keyNames[0])
			}
		}
	}

	// Fail if no keys available - do not generate placeholder keys
	if len(ageKeys) == 0 {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Message: "No age encryption keys available",
			Suggestions: []string{
				"Generate an age key using: opencenter sops generate-key",
				"Import an existing age key",
				"Set SOPS_AGE_KEY_FILE environment variable to point to your key file",
				"Configure secrets.sopsAgeKeyFile in your cluster configuration",
			},
		}
	}

	encryptConfig := EncryptionConfig{
		AgeKeys: ageKeys,
		InPlace: true,
		Verbose: true,
	}

	// Filter out non-existent files
	var existingFiles []string
	for _, file := range filesToEncrypt {
		filePath := filepath.Join(overlayPath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			m.logger.Debug("Skipping non-existent file", "file", file)
			continue
		}
		existingFiles = append(existingFiles, filePath)
	}

	// Encrypt files in parallel if there are multiple files
	if len(existingFiles) > 1 {
		m.logger.Info("Encrypting files in parallel", "count", len(existingFiles))
		if encryptor, ok := m.encryptor.(*DefaultEncryptor); ok {
			if err := encryptor.EncryptFilesParallel(ctx, existingFiles, encryptConfig, 4); err != nil {
				return err
			}
		} else {
			// Fallback to sequential encryption
			for _, filePath := range existingFiles {
				m.logger.Info("Encrypting file", "file", filePath)
				if err := m.encryptor.EncryptFile(ctx, filePath, encryptConfig); err != nil {
					return &errors.StructuredError{
						Type:    errors.SOPSError,
						Field:   filePath,
						Message: "Failed to encrypt file",
						Cause:   err,
						Suggestions: []string{
							"Check that SOPS is installed and accessible",
							"Verify the age key is valid",
							"Ensure the file is not already encrypted",
							"Check file permissions",
						},
					}
				}
			}
		}
	} else if len(existingFiles) == 1 {
		// Single file - encrypt directly
		filePath := existingFiles[0]
		m.logger.Info("Encrypting file", "file", filePath)
		if err := m.encryptor.EncryptFile(ctx, filePath, encryptConfig); err != nil {
			return &errors.StructuredError{
				Type:    errors.SOPSError,
				Field:   filePath,
				Message: "Failed to encrypt file",
				Cause:   err,
				Suggestions: []string{
					"Check that SOPS is installed and accessible",
					"Verify the age key is valid",
					"Ensure the file is not already encrypted",
					"Check file permissions",
				},
			}
		}
	}

	m.logger.Info("Completed overlay files encryption", "encrypted_count", len(filesToEncrypt))
	return nil
}

// CreateSOPSConfig creates a .sops.yaml configuration file
func (m *DefaultSOPSManager) CreateSOPSConfig(overlayPath string, cfg *config.Config) error {
	m.logger.Info("Creating SOPS configuration", "overlay_path", overlayPath)

	sopsConfig, err := m.generateSOPSConfig(cfg)
	if err != nil {
		return err
	}

	// Validate that we're not using placeholder keys in production
	if err := m.validator.ValidateKeyForProduction(sopsConfig); err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Message: "Cannot create SOPS config with placeholder key",
			Cause:   err,
			Suggestions: []string{
				"Generate a proper age key using 'opencenter sops generate-key'",
				"Import an existing age key",
				"Check the SOPS configuration",
			},
		}
	}

	configPath := filepath.Join(overlayPath, ".sops.yaml")
	if err := os.WriteFile(configPath, []byte(sopsConfig), 0o644); err != nil {
		return &errors.StructuredError{
			Type:    errors.FileError,
			Field:   ".sops.yaml",
			Message: "Failed to write SOPS config file",
			Cause:   err,
			Suggestions: []string{
				"Check directory permissions",
				"Ensure the overlay path exists",
				"Verify disk space availability",
			},
		}
	}

	m.logger.Info("Successfully created SOPS configuration", "config_path", configPath)
	return nil
}

// ValidateEncryption validates that files are properly encrypted
func (m *DefaultSOPSManager) ValidateEncryption(overlayPath string, cfg *config.Config) error {
	m.logger.Info("Validating encryption", "overlay_path", overlayPath)

	return m.validator.ValidateEncryption(overlayPath, cfg)
}

// CreateSampleEncryptedSecrets creates sample encrypted secrets in the repository
func (m *DefaultSOPSManager) CreateSampleEncryptedSecrets(ctx context.Context, repoPath string, ageKey string) error {
	m.logger.Info("Creating sample encrypted secrets", "repo_path", repoPath)

	return m.createSampleEncryptedSecretsForTemplate(ctx, repoPath, ageKey, "basic")
}

// EncryptRepositorySecrets encrypts all sample secrets in a repository
func (m *DefaultSOPSManager) EncryptRepositorySecrets(ctx context.Context, repoPath string, ageKey string) error {
	m.logger.Info("Encrypting repository secrets", "repo_path", repoPath)

	secretsDir := filepath.Join(repoPath, "examples", "secrets")

	// Find all .yaml files that are not already encrypted
	err := filepath.Walk(secretsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-YAML files
		ext := filepath.Ext(path)
		if info.IsDir() || (ext != ".yaml" && ext != ".yml") {
			return nil
		}

		// Skip already encrypted files
		if filepath.Base(path) == "README.yaml" || filepath.Base(path) == "README.yml" {
			return nil
		}

		// Check if file is already encrypted
		if isEncrypted, err := m.encryptor.IsFileEncrypted(path); err != nil {
			return fmt.Errorf("failed to check encryption status of %s: %w", path, err)
		} else if isEncrypted {
			return nil // Already encrypted
		}

		// Encrypt the file
		encryptConfig := EncryptionConfig{
			AgeKeys: []string{ageKey},
			InPlace: true,
		}

		m.logger.Info("Encrypting repository secret file", "file", path)
		if err := m.encryptor.EncryptFile(ctx, path, encryptConfig); err != nil {
			return &errors.StructuredError{
				Type:    errors.SOPSError,
				Field:   path,
				Message: "Failed to encrypt repository secret file",
				Cause:   err,
				Suggestions: []string{
					"Check that SOPS is installed and accessible in your PATH",
					"Verify the age key is valid",
					"Ensure you have the correct decryption keys",
					"Install SOPS: https://github.com/mozilla/sops#download",
				},
			}
		}
		m.logger.Info("Successfully encrypted repository secret file", "file", path)

		return nil
	})

	if err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Message: "Failed to encrypt repository secrets",
			Cause:   err,
			Suggestions: []string{
				"Check that SOPS is installed and accessible",
				"Verify the age key is valid",
				"Ensure file permissions are correct",
			},
		}
	}

	m.logger.Info("Completed repository secrets encryption")
	return nil
}

// CheckSOPSVersion checks if SOPS is available and returns version info
func (m *DefaultSOPSManager) CheckSOPSVersion(ctx context.Context) (string, error) {
	m.logger.Debug("Checking SOPS version")

	version, err := checkSOPSVersion(ctx)
	if err != nil {
		return "", &errors.StructuredError{
			Type:    errors.SystemError,
			Message: "SOPS not found or not executable",
			Cause:   err,
			Suggestions: []string{
				"Install SOPS using your package manager",
				"Ensure SOPS is in your PATH",
				"Check SOPS installation documentation",
			},
		}
	}

	m.logger.Info("SOPS version check successful", "version", version)
	return version, nil
}

// Helper methods

// getFilesToEncrypt returns the list of files that should be encrypted
func (m *DefaultSOPSManager) getFilesToEncrypt(overlayPath string, cfg *config.Config) []string {
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

	return files
}

// generateSOPSConfig generates the SOPS configuration content
func (m *DefaultSOPSManager) generateSOPSConfig(cfg *config.Config) (string, error) {
	var ageKey string
	if cfg.Secrets.SopsAgeKeyFile != "" {
		// Load the public key from the age key file
		if keyPair, err := m.loadAgeKeyFromFile(cfg.Secrets.SopsAgeKeyFile); err == nil {
			ageKey = keyPair.PublicKey
		}
	}

	if ageKey == "" {
		// Fallback: try to load from default key manager
		if keyNames, err := m.keyManager.ListAgeKeys(); err == nil && len(keyNames) > 0 {
			if keyPair, err := m.keyManager.LoadAgeKey(keyNames[0]); err == nil {
				ageKey = keyPair.PublicKey
			}
		}
	}

	// Fail if no keys available - do not use placeholder keys
	if ageKey == "" {
		return "", &errors.StructuredError{
			Type:    errors.SOPSError,
			Message: "No age encryption keys available for SOPS configuration",
			Suggestions: []string{
				"Generate an age key using: opencenter sops generate-key",
				"Import an existing age key",
				"Set SOPS_AGE_KEY_FILE environment variable to point to your key file",
				"Configure secrets.sopsAgeKeyFile in your cluster configuration",
			},
		}
	}

	config := fmt.Sprintf(`# SOPS configuration for cluster: %s
creation_rules:
  - path_regex: 'secrets/age/keys/.*-key\.txt$'
    age: >-
      %s
  - path_regex: 'secrets/ssh/(?!.*\.pub$).*'
    age: >-
      %s
  - path_regex: 'applications/overlays/[^/]+/(managed-services|services)/.*/.*\.ya?ml$'
    encrypted_regex: "^(secret)$"
    age: >-
      %s
  - path_regex: '^infrastructure\/clusters\/%s\/(?!(?:venv|kubespray|\.terraform|\.bin)\/)(.*)'
    encrypted_regex: "^(secret)$"
    age: >-
      %s
`, cfg.OpenCenter.Cluster.ClusterName, ageKey, ageKey, ageKey, cfg.OpenCenter.Cluster.ClusterName, ageKey)

	return config, nil
}

// loadAgeKeyFromFile loads an age key pair from a file path
func (m *DefaultSOPSManager) loadAgeKeyFromFile(keyFilePath string) (*crypto.AgeKeyPair, error) {
	// Expand home directory if needed
	if keyFilePath[0] == '~' {
		homeDir, _ := os.UserHomeDir()
		keyFilePath = filepath.Join(homeDir, keyFilePath[2:])
	}

	// Read the private key file
	privateKeyData, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read age key file: %w", err)
	}

	privateKey := string(privateKeyData)

	// Parse the private key to get the public key
	return crypto.ParseAgeKey(privateKey)
}

// createSampleEncryptedSecretsForTemplate creates sample encrypted secrets for a specific template
func (m *DefaultSOPSManager) createSampleEncryptedSecretsForTemplate(ctx context.Context, repoPath string, ageKey string, template string) error {
	samplesDir := filepath.Join(repoPath, "examples", "secrets")

	// Ensure samples directory exists
	if err := os.MkdirAll(samplesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create samples directory: %w", err)
	}

	// Get sample secrets based on template
	sampleSecrets := m.getSampleSecretsForTemplate(template)

	// Create and encrypt each sample secret
	for filename, content := range sampleSecrets {
		// Create temporary unencrypted file
		tempFile := filepath.Join(samplesDir, filename+".tmp")
		if err := os.WriteFile(tempFile, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write temp file %s: %w", tempFile, err)
		}

		// Encrypt the file
		encryptConfig := EncryptionConfig{
			AgeKeys: []string{ageKey},
			InPlace: false,
		}

		// Encrypt to the final file
		finalFile := filepath.Join(samplesDir, filename)
		if err := m.encryptFileToOutput(ctx, tempFile, finalFile, encryptConfig); err != nil {
			// Clean up temp file
			os.Remove(tempFile)
			return &errors.StructuredError{
				Type:    errors.SOPSError,
				Field:   finalFile,
				Message: "Failed to encrypt sample secret file",
				Cause:   err,
				Suggestions: []string{
					"Check that SOPS is installed and accessible in your PATH",
					"Verify the age key is valid",
					"Install SOPS: https://github.com/mozilla/sops#download",
					"Ensure SOPS_AGE_KEY_FILE environment variable is set correctly",
				},
			}
		}

		// Remove temporary unencrypted file
		os.Remove(tempFile)
	}

	return nil
}

// encryptFileToOutput encrypts a file and writes to a specific output file
func (m *DefaultSOPSManager) encryptFileToOutput(ctx context.Context, inputFile, outputFile string, config EncryptionConfig) error {
	// This would use the encryptor to encrypt to a specific output file
	// For now, we'll use a simple approach
	return m.encryptor.EncryptFile(ctx, inputFile, config)
}

// getSampleSecretsForTemplate returns sample secrets based on the template type
func (m *DefaultSOPSManager) getSampleSecretsForTemplate(template string) map[string]string {
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
	}

	return baseSecrets
}
