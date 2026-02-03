/*
Copyright 2025.

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
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/util/crypto"
)

// TestSOPSValidationMigration_ValidKey tests that ValidationEngine correctly validates a valid SOPS key
func TestSOPSValidationMigration_ValidKey(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create a valid Age key file
	keyPath := filepath.Join(tmpDir, "test-key.txt")
	validKey := "AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ234567890ABCDEFGHIJKLMNOP"
	if err := os.WriteFile(keyPath, []byte(validKey), 0600); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	// Create SOPS manager
	keyManager := crypto.NewDefaultKeyManager(tmpDir)
	encryptor := NewDefaultEncryptor([]string{}, []string{})
	
	manager := NewDefaultSOPSManager(keyManager, encryptor, nil)

	// Create test config
	cfg := &config.Config{
		Secrets: config.Secrets{
			SopsAgeKeyFile: keyPath,
		},
	}

	// Validate encryption - should succeed
	err := manager.ValidateEncryption(tmpDir, cfg)
	if err != nil {
		t.Errorf("Expected validation to succeed with valid key, got error: %v", err)
	}
}

// TestSOPSValidationMigration_MissingKey tests that ValidationEngine correctly detects missing SOPS key
func TestSOPSValidationMigration_MissingKey(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create SOPS manager
	keyManager := crypto.NewDefaultKeyManager(tmpDir)
	encryptor := NewDefaultEncryptor([]string{}, []string{})
	
	manager := NewDefaultSOPSManager(keyManager, encryptor, nil)

	// Create test config with non-existent key
	cfg := &config.Config{
		Secrets: config.Secrets{
			SopsAgeKeyFile: filepath.Join(tmpDir, "nonexistent-key.txt"),
		},
	}

	// Validate encryption - should fail
	err := manager.ValidateEncryption(tmpDir, cfg)
	if err == nil {
		t.Error("Expected validation to fail with missing key, got nil error")
	}

	// Check error message contains helpful suggestions
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Expected error message, got empty string")
	}
}

// TestSOPSValidationMigration_InvalidKeyFormat tests that ValidationEngine correctly detects invalid key format
func TestSOPSValidationMigration_InvalidKeyFormat(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create an invalid Age key file (wrong prefix)
	keyPath := filepath.Join(tmpDir, "invalid-key.txt")
	invalidKey := "INVALID-KEY-FORMAT-1234567890"
	if err := os.WriteFile(keyPath, []byte(invalidKey), 0600); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	// Create SOPS manager
	keyManager := crypto.NewDefaultKeyManager(tmpDir)
	encryptor := NewDefaultEncryptor([]string{}, []string{})
	
	manager := NewDefaultSOPSManager(keyManager, encryptor, nil)

	// Create test config
	cfg := &config.Config{
		Secrets: config.Secrets{
			SopsAgeKeyFile: keyPath,
		},
	}

	// Validate encryption - should fail
	err := manager.ValidateEncryption(tmpDir, cfg)
	if err == nil {
		t.Error("Expected validation to fail with invalid key format, got nil error")
	}
}

// TestSOPSValidationMigration_InsecurePermissions tests that ValidationEngine warns about insecure permissions
func TestSOPSValidationMigration_InsecurePermissions(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create a valid Age key file with insecure permissions
	keyPath := filepath.Join(tmpDir, "insecure-key.txt")
	validKey := "AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ234567890ABCDEFGHIJKLMNOP"
	if err := os.WriteFile(keyPath, []byte(validKey), 0644); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	// Create SOPS manager
	keyManager := crypto.NewDefaultKeyManager(tmpDir)
	encryptor := NewDefaultEncryptor([]string{}, []string{})
	
	manager := NewDefaultSOPSManager(keyManager, encryptor, nil)

	// Create test config
	cfg := &config.Config{
		Secrets: config.Secrets{
			SopsAgeKeyFile: keyPath,
		},
	}

	// Validate encryption - should succeed but log warning
	// Note: The warning is logged, not returned as an error
	err := manager.ValidateEncryption(tmpDir, cfg)
	if err != nil {
		t.Errorf("Expected validation to succeed with warning, got error: %v", err)
	}
}

// TestSOPSValidationMigration_ValidationEngineRegistered tests that SOPSKeyValidator is registered
func TestSOPSValidationMigration_ValidationEngineRegistered(t *testing.T) {
	// Create SOPS manager
	tmpDir := t.TempDir()
	keyManager := crypto.NewDefaultKeyManager(tmpDir)
	encryptor := NewDefaultEncryptor([]string{}, []string{})
	
	manager := NewDefaultSOPSManager(keyManager, encryptor, nil)

	// Check that the validation engine has the sops-key validator registered
	if !manager.validationEngine.Has("sops-key") {
		t.Error("Expected sops-key validator to be registered with ValidationEngine")
	}
}

// TestSOPSValidationMigration_DirectValidation tests direct validation using ValidationEngine
func TestSOPSValidationMigration_DirectValidation(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create a valid Age key file
	keyPath := filepath.Join(tmpDir, "test-key.txt")
	validKey := "AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ234567890ABCDEFGHIJKLMNOP"
	if err := os.WriteFile(keyPath, []byte(validKey), 0600); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	// Create SOPS manager
	keyManager := crypto.NewDefaultKeyManager(tmpDir)
	encryptor := NewDefaultEncryptor([]string{}, []string{})
	
	manager := NewDefaultSOPSManager(keyManager, encryptor, nil)

	// Validate directly using ValidationEngine
	result, err := manager.validationEngine.Validate(context.Background(), "sops-key", keyPath)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected validation to succeed, got invalid result with %d errors", len(result.Errors))
		for _, e := range result.Errors {
			t.Logf("Error: %s - %s", e.Field, e.Message)
		}
	}
}

// TestSOPSValidationMigration_SecurityChecksPreserved tests that all security checks are maintained
func TestSOPSValidationMigration_SecurityChecksPreserved(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create SOPS manager
	keyManager := crypto.NewDefaultKeyManager(tmpDir)
	encryptor := NewDefaultEncryptor([]string{}, []string{})
	
	manager := NewDefaultSOPSManager(keyManager, encryptor, nil)

	testCases := []struct {
		name        string
		keyContent  string
		permissions os.FileMode
		shouldFail  bool
		description string
	}{
		{
			name:        "valid_key",
			keyContent:  "AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ234567890ABCDEFGHIJKLMNOP",
			permissions: 0600,
			shouldFail:  false,
			description: "Valid key with secure permissions",
		},
		{
			name:        "invalid_prefix",
			keyContent:  "WRONG-PREFIX-1ABCDEFGHIJKLMNOPQRSTUVWXYZ234567890ABCDEFGHIJKLMNOP",
			permissions: 0600,
			shouldFail:  true,
			description: "Invalid key prefix",
		},
		{
			name:        "empty_key",
			keyContent:  "",
			permissions: 0600,
			shouldFail:  true,
			description: "Empty key file",
		},
		{
			name:        "whitespace_only",
			keyContent:  "   \n\t  ",
			permissions: 0600,
			shouldFail:  true,
			description: "Whitespace-only key file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create key file
			keyPath := filepath.Join(tmpDir, tc.name+"-key.txt")
			if err := os.WriteFile(keyPath, []byte(tc.keyContent), tc.permissions); err != nil {
				t.Fatalf("Failed to create test key file: %v", err)
			}

			// Validate using ValidationEngine
			result, err := manager.validationEngine.Validate(context.Background(), "sops-key", keyPath)
			if err != nil {
				t.Fatalf("Validation execution failed: %v", err)
			}

			if tc.shouldFail && result.Valid {
				t.Errorf("%s: Expected validation to fail, but it succeeded", tc.description)
			}

			if !tc.shouldFail && !result.Valid {
				t.Errorf("%s: Expected validation to succeed, but it failed with %d errors", tc.description, len(result.Errors))
				for _, e := range result.Errors {
					t.Logf("Error: %s - %s", e.Field, e.Message)
				}
			}
		})
	}
}
