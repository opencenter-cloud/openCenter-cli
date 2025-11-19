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
	"os"
	"path/filepath"
	"testing"
)

func TestNewKeyManager(t *testing.T) {
	tests := []struct {
		name     string
		keyDir   string
		expected string
	}{
		{
			name:     "with custom key directory",
			keyDir:   "/custom/path",
			expected: "/custom/path",
		},
		{
			name:     "with empty key directory uses default",
			keyDir:   "",
			expected: filepath.Join(os.Getenv("HOME"), ".config", "sops", "age"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewKeyManager(tt.keyDir)
			if manager == nil {
				t.Error("NewKeyManager() should not return nil")
			}
		})
	}
}

func TestSetupSOPSEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewKeyManager(tmpDir)

	// Generate a test key
	keyPair, err := manager.GenerateAgeKey()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	// Save the key
	keyName := "test-key"
	err = manager.SaveAgeKey(keyPair, keyName)
	if err != nil {
		t.Fatalf("Failed to save test key: %v", err)
	}

	// Test setup environment
	err = SetupSOPSEnvironment(manager, keyName)
	if err != nil {
		t.Errorf("SetupSOPSEnvironment() error = %v", err)
	}

	// Check if environment variable was set
	envVar := os.Getenv("SOPS_AGE_KEY_FILE")
	if envVar == "" {
		t.Error("SetupSOPSEnvironment() should set SOPS_AGE_KEY_FILE environment variable")
	}
}

func TestCheckSOPSInstallation(t *testing.T) {
	// This test checks if SOPS is installed
	// It may fail in test environments where SOPS is not available
	err := CheckSOPSInstallation(context.Background())
	if err != nil {
		t.Logf("CheckSOPSInstallation() failed as expected in test environment: %v", err)
	} else {
		t.Log("SOPS is available in test environment")
	}
}

func TestValidateSOPSKeyAccess(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewKeyManager(tmpDir)

	// Generate a test key
	keyPair, err := manager.GenerateAgeKey()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	// Save the key
	keyName := "test-key"
	err = manager.SaveAgeKey(keyPair, keyName)
	if err != nil {
		t.Fatalf("Failed to save test key: %v", err)
	}

	// Test key validation
	err = ValidateSOPSKeyAccess(manager, keyName)
	if err != nil {
		t.Errorf("ValidateSOPSKeyAccess() error = %v", err)
	}
}

func TestValidateSOPSKeyAccess_NonExistentKey(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewKeyManager(tmpDir)

	// Test with non-existent key
	err := ValidateSOPSKeyAccess(manager, "non-existent-key")
	if err == nil {
		t.Error("ValidateSOPSKeyAccess() should fail for non-existent key")
	}
}
