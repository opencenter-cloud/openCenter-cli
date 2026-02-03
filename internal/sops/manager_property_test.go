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
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/util/crypto"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
)

// Feature: stub-implementation-completion, Property 38: SOPS Unavailable Error
// **Validates: Requirements 9.1, 9.3**
func TestProperty_SOPSUnavailableError(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("encryption without keys returns error with instructions", prop.ForAll(
		func(clusterName string, provider string) bool {
			// Create temporary directory
			tempDir := t.TempDir()
			overlayPath := filepath.Join(tempDir, "overlay")
			if err := os.MkdirAll(overlayPath, 0o755); err != nil {
				t.Logf("Failed to create overlay directory: %v", err)
				return false
			}

			// Create a test file to encrypt
			testFile := filepath.Join(overlayPath, "flux-system", "gotk-sync.yaml")
			if err := os.MkdirAll(filepath.Dir(testFile), 0o755); err != nil {
				t.Logf("Failed to create test file directory: %v", err)
				return false
			}
			if err := os.WriteFile(testFile, []byte("test: data"), 0o644); err != nil {
				t.Logf("Failed to create test file: %v", err)
				return false
			}

			// Create config without age keys
			cfg := &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Name: clusterName,
					},
					Infrastructure: config.Infrastructure{
						Provider: provider,
					},
				},
				Secrets: config.Secrets{
					SopsAgeKeyFile: "", // No key file configured
				},
			}

			// Create key manager that returns no keys
			keyManager := crypto.NewDefaultKeyManager(filepath.Join(tempDir, "keys"))
			encryptor := NewDefaultEncryptor([]string{}, []string{})
			
			manager := NewDefaultSOPSManager(keyManager, encryptor, slog.Default())

			// Try to encrypt overlay files (should fail)
			err := manager.EncryptOverlayFiles(context.Background(), overlayPath, cfg)

			// Verify error is returned
			if err == nil {
				t.Logf("Expected error when no keys available, got nil")
				return false
			}

			// Verify error is a StructuredError
			structErr, ok := err.(*errors.StructuredError)
			if !ok {
				t.Logf("Expected StructuredError, got %T", err)
				return false
			}

			// Verify error type is SOPSError
			if structErr.Type != errors.SOPSError {
				t.Logf("Expected SOPSError, got %s", structErr.Type)
				return false
			}

			// Verify error message mentions missing keys
			if !strings.Contains(structErr.Message, "age") && !strings.Contains(structErr.Message, "key") {
				t.Logf("Error message should mention keys: %s", structErr.Message)
				return false
			}

			// Verify suggestions include key generation instructions
			hasKeyGenSuggestion := false
			for _, suggestion := range structErr.Suggestions {
				if strings.Contains(suggestion, "generate") || strings.Contains(suggestion, "Generate") {
					hasKeyGenSuggestion = true
					break
				}
			}
			if !hasKeyGenSuggestion {
				t.Logf("Suggestions should include key generation instructions: %v", structErr.Suggestions)
				return false
			}

			return true
		},
		genValidClusterName(),
		gen.OneConstOf("openstack", "aws", "vsphere", "kind"),
	))

	properties.Property("SOPS config generation without keys returns error", prop.ForAll(
		func(clusterName string, provider string) bool {
			// Create temporary directory
			tempDir := t.TempDir()
			overlayPath := filepath.Join(tempDir, "overlay")
			if err := os.MkdirAll(overlayPath, 0o755); err != nil {
				t.Logf("Failed to create overlay directory: %v", err)
				return false
			}

			// Create config without age keys
			cfg := &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Name: clusterName,
					},
					Infrastructure: config.Infrastructure{
						Provider: provider,
					},
				},
				Secrets: config.Secrets{
					SopsAgeKeyFile: "", // No key file configured
				},
			}

			// Create key manager that returns no keys
			keyManager := crypto.NewDefaultKeyManager(filepath.Join(tempDir, "keys"))
			encryptor := NewDefaultEncryptor([]string{}, []string{})
			
			manager := NewDefaultSOPSManager(keyManager, encryptor, slog.Default())

			// Try to create SOPS config (should fail)
			err := manager.CreateSOPSConfig(overlayPath, cfg)

			// Verify error is returned
			if err == nil {
				t.Logf("Expected error when no keys available, got nil")
				return false
			}

			// Verify error is a StructuredError
			structErr, ok := err.(*errors.StructuredError)
			if !ok {
				t.Logf("Expected StructuredError, got %T", err)
				return false
			}

			// Verify error type is SOPSError
			if structErr.Type != errors.SOPSError {
				t.Logf("Expected SOPSError, got %s", structErr.Type)
				return false
			}

			// Verify no .sops.yaml file was created
			sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
			if _, err := os.Stat(sopsConfigPath); !os.IsNotExist(err) {
				t.Logf("SOPS config file should not be created when keys are missing")
				return false
			}

			return true
		},
		genValidClusterName(),
		gen.OneConstOf("openstack", "aws", "vsphere", "kind"),
	))

	properties.Property("no placeholder content is created when SOPS unavailable", prop.ForAll(
		func(clusterName string) bool {
			// Create temporary directory
			tempDir := t.TempDir()
			overlayPath := filepath.Join(tempDir, "overlay")
			if err := os.MkdirAll(overlayPath, 0o755); err != nil {
				t.Logf("Failed to create overlay directory: %v", err)
				return false
			}

			// Create a test file to encrypt
			testFile := filepath.Join(overlayPath, "flux-system", "gotk-sync.yaml")
			if err := os.MkdirAll(filepath.Dir(testFile), 0o755); err != nil {
				t.Logf("Failed to create test file directory: %v", err)
				return false
			}
			originalContent := "test: data\npassword: secret"
			if err := os.WriteFile(testFile, []byte(originalContent), 0o644); err != nil {
				t.Logf("Failed to create test file: %v", err)
				return false
			}

			// Create config without age keys
			cfg := &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Name: clusterName,
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
				Secrets: config.Secrets{
					SopsAgeKeyFile: "", // No key file configured
				},
			}

			// Create key manager that returns no keys
			keyManager := crypto.NewDefaultKeyManager(filepath.Join(tempDir, "keys"))
			encryptor := NewDefaultEncryptor([]string{}, []string{})
			
			manager := NewDefaultSOPSManager(keyManager, encryptor, slog.Default())

			// Try to encrypt overlay files (should fail)
			err := manager.EncryptOverlayFiles(context.Background(), overlayPath, cfg)

			// Verify error is returned
			if err == nil {
				t.Logf("Expected error when no keys available, got nil")
				return false
			}

			// Read the file content
			content, readErr := os.ReadFile(testFile)
			if readErr != nil {
				t.Logf("Failed to read test file: %v", readErr)
				return false
			}

			// Verify file content is unchanged (no placeholder was created)
			if string(content) != originalContent {
				t.Logf("File content should be unchanged when encryption fails")
				return false
			}

			// Verify no placeholder markers in content
			contentStr := string(content)
			if strings.Contains(contentStr, "placeholder") || strings.Contains(contentStr, "PLACEHOLDER") {
				t.Logf("File should not contain placeholder content")
				return false
			}

			return true
		},
		genValidClusterName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: stub-implementation-completion, Property 39: Missing Key Error with Instructions
// **Validates: Requirements 9.2**
func TestProperty_MissingKeyErrorWithInstructions(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("missing age key file returns error with generation instructions", prop.ForAll(
		func(clusterName string, nonExistentPath string) bool {
			// Skip if path is empty
			if nonExistentPath == "" {
				return true
			}

			// Create temporary directory
			tempDir := t.TempDir()
			overlayPath := filepath.Join(tempDir, "overlay")
			if err := os.MkdirAll(overlayPath, 0o755); err != nil {
				t.Logf("Failed to create overlay directory: %v", err)
				return false
			}

			// Create config with non-existent key file
			cfg := &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Name: clusterName,
					},
					Cluster: config.ClusterConfig{
						ClusterName: clusterName,
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
				Secrets: config.Secrets{
					SopsAgeKeyFile: filepath.Join(tempDir, nonExistentPath, "nonexistent.txt"),
				},
			}

			// Create key manager
			keyManager := crypto.NewDefaultKeyManager(filepath.Join(tempDir, "keys"))
			encryptor := NewDefaultEncryptor([]string{}, []string{})
			
			manager := NewDefaultSOPSManager(keyManager, encryptor, slog.Default())

			// Try to create SOPS config (should fail)
			err := manager.CreateSOPSConfig(overlayPath, cfg)

			// Verify error is returned
			if err == nil {
				t.Logf("Expected error when key file doesn't exist, got nil")
				return false
			}

			// Verify error is a StructuredError
			structErr, ok := err.(*errors.StructuredError)
			if !ok {
				t.Logf("Expected StructuredError, got %T", err)
				return false
			}

			// Verify error type is SOPSError
			if structErr.Type != errors.SOPSError {
				t.Logf("Expected SOPSError, got %s", structErr.Type)
				return false
			}

			// Verify suggestions include key generation or import instructions
			hasKeyInstructions := false
			for _, suggestion := range structErr.Suggestions {
				if strings.Contains(suggestion, "generate") || strings.Contains(suggestion, "Generate") ||
					strings.Contains(suggestion, "import") || strings.Contains(suggestion, "Import") {
					hasKeyInstructions = true
					break
				}
			}
			if !hasKeyInstructions {
				t.Logf("Suggestions should include key generation or import instructions: %v", structErr.Suggestions)
				return false
			}

			return true
		},
		genValidClusterName(),
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: stub-implementation-completion, Property 40: Decryption Failure Classification
// **Validates: Requirements 9.4**
func TestProperty_DecryptionFailureClassification(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("decryption of non-encrypted file returns descriptive error", prop.ForAll(
		func(content string) bool {
			// Skip empty content
			if content == "" {
				return true
			}

			// Create temporary file
			tempDir := t.TempDir()
			testFile := filepath.Join(tempDir, "test.yaml")
			if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
				t.Logf("Failed to create test file: %v", err)
				return false
			}

			// Create encryptor
			encryptor := NewDefaultEncryptor([]string{}, []string{})

			// Try to decrypt non-encrypted file (should fail)
			err := encryptor.DecryptFile(context.Background(), testFile, "")

			// Verify error is returned
			if err == nil {
				t.Logf("Expected error when decrypting non-encrypted file, got nil")
				return false
			}

			// Verify error is a StructuredError
			structErr, ok := err.(*errors.StructuredError)
			if !ok {
				t.Logf("Expected StructuredError, got %T", err)
				return false
			}

			// Verify error type is SOPSError
			if structErr.Type != errors.SOPSError {
				t.Logf("Expected SOPSError, got %s", structErr.Type)
				return false
			}

			// Verify error message indicates file is not encrypted
			if !strings.Contains(structErr.Message, "not encrypted") {
				t.Logf("Error message should indicate file is not encrypted: %s", structErr.Message)
				return false
			}

			return true
		},
		gen.AlphaString(),
	))

	properties.Property("decryption of missing file returns file error", prop.ForAll(
		func(filename string) bool {
			// Skip empty filename
			if filename == "" {
				return true
			}

			// Create temporary directory
			tempDir := t.TempDir()
			nonExistentFile := filepath.Join(tempDir, filename)

			// Create encryptor
			encryptor := NewDefaultEncryptor([]string{}, []string{})

			// Try to decrypt non-existent file (should fail)
			err := encryptor.DecryptFile(context.Background(), nonExistentFile, "")

			// Verify error is returned
			if err == nil {
				t.Logf("Expected error when decrypting non-existent file, got nil")
				return false
			}

			// Verify error is a StructuredError
			structErr, ok := err.(*errors.StructuredError)
			if !ok {
				t.Logf("Expected StructuredError, got %T", err)
				return false
			}

			// Verify error type is FileError
			if structErr.Type != errors.FileError {
				t.Logf("Expected FileError, got %s", structErr.Type)
				return false
			}

			// Verify error message mentions file doesn't exist
			if !strings.Contains(structErr.Message, "not exist") && !strings.Contains(structErr.Message, "does not exist") {
				t.Logf("Error message should indicate file doesn't exist: %s", structErr.Message)
				return false
			}

			return true
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: stub-implementation-completion, Property 41: No Placeholder Key Fallback
// **Validates: Requirements 9.5**
func TestProperty_NoPlaceholderKeyFallback(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("SOPS config never contains placeholder age keys", prop.ForAll(
		func(clusterName string, provider string) bool {
			// Create temporary directory
			tempDir := t.TempDir()
			overlayPath := filepath.Join(tempDir, "overlay")
			if err := os.MkdirAll(overlayPath, 0o755); err != nil {
				t.Logf("Failed to create overlay directory: %v", err)
				return false
			}

			// Create config without age keys
			cfg := &config.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Name: clusterName,
					},
					Cluster: config.ClusterConfig{
						ClusterName: clusterName,
					},
					Infrastructure: config.Infrastructure{
						Provider: provider,
					},
				},
				Secrets: config.Secrets{
					SopsAgeKeyFile: "", // No key file configured
				},
			}

			// Create key manager that returns no keys
			keyManager := crypto.NewDefaultKeyManager(filepath.Join(tempDir, "keys"))
			encryptor := NewDefaultEncryptor([]string{}, []string{})
			
			manager := NewDefaultSOPSManager(keyManager, encryptor, slog.Default())

			// Try to create SOPS config (should fail)
			err := manager.CreateSOPSConfig(overlayPath, cfg)

			// Verify error is returned (no fallback to placeholder)
			if err == nil {
				t.Logf("Expected error when no keys available, got nil (no placeholder fallback should occur)")
				return false
			}

			// Verify no .sops.yaml file was created
			sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
			if _, statErr := os.Stat(sopsConfigPath); !os.IsNotExist(statErr) {
				// If file was created, check it doesn't contain placeholder keys
				content, readErr := os.ReadFile(sopsConfigPath)
				if readErr == nil {
					contentStr := string(content)
					// Check for placeholder patterns
					if strings.Contains(contentStr, "age1xxx") || strings.Contains(contentStr, "placeholder") {
						t.Logf("SOPS config should not contain placeholder keys")
						return false
					}
				}
			}

			return true
		},
		genValidClusterName(),
		gen.OneConstOf("openstack", "aws", "vsphere", "kind"),
	))

	properties.Property("encrypted files never contain placeholder SOPS metadata", prop.ForAll(
		func(clusterName string) bool {
			// Create temporary directory
			tempDir := t.TempDir()
			repoPath := filepath.Join(tempDir, "repo")
			secretsDir := filepath.Join(repoPath, "examples", "secrets")
			if err := os.MkdirAll(secretsDir, 0o755); err != nil {
				t.Logf("Failed to create secrets directory: %v", err)
				return false
			}

			// Create a test secret file
			testFile := filepath.Join(secretsDir, "test-secret.yaml")
			if err := os.WriteFile(testFile, []byte("password: secret"), 0o644); err != nil {
				t.Logf("Failed to create test file: %v", err)
				return false
			}

			// Create manager with no keys
			keyManager := crypto.NewDefaultKeyManager(filepath.Join(tempDir, "keys"))
			encryptor := NewDefaultEncryptor([]string{}, []string{})
			
			manager := NewDefaultSOPSManager(keyManager, encryptor, slog.Default())

			// Try to encrypt repository secrets (should fail)
			err := manager.EncryptRepositorySecrets(context.Background(), repoPath, "")

			// Verify error is returned
			if err == nil {
				t.Logf("Expected error when no age key provided, got nil")
				return false
			}

			// Read the file content
			content, readErr := os.ReadFile(testFile)
			if readErr != nil {
				t.Logf("Failed to read test file: %v", readErr)
				return false
			}

			contentStr := string(content)

			// Verify no placeholder SOPS metadata was added
			if strings.Contains(contentStr, "placeholder_encrypted_data") ||
				strings.Contains(contentStr, "placeholder_mac") ||
				strings.Contains(contentStr, "DO NOT COMMIT UNENCRYPTED") {
				t.Logf("File should not contain placeholder SOPS metadata")
				return false
			}

			// Verify original content is preserved
			if !strings.Contains(contentStr, "password: secret") {
				t.Logf("Original content should be preserved when encryption fails")
				return false
			}

			return true
		},
		genValidClusterName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
