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
	"log/slog"
	"strings"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/util/crypto"
	testhelpers "github.com/rackerlabs/opencenter-cli/internal/testing"
)

// **Property 9: Multi-Key SOPS Configuration**
// **Validates: Requirements 5.5, 5.6**
func TestMultiKeySOPSConfiguration(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create key manager
	manager := NewEnhancedKeyManager(tempDir, slog.Default())
	manager.SetKeyringEnabled(false) // Use file storage for predictable testing

	clusterName := "test-cluster"

	// Generate primary key
	primaryKey, err := manager.GenerateKey(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to generate primary key")

	// Generate additional keys
	additionalKey1, err := manager.GenerateAdditionalKey(clusterName, 1)
	testhelpers.AssertNoError(t, err, "Failed to generate additional key 1")

	additionalKey2, err := manager.GenerateAdditionalKey(clusterName, 2)
	testhelpers.AssertNoError(t, err, "Failed to generate additional key 2")

	// List all cluster keys
	keys, err := manager.ListClusterKeys(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to list cluster keys")

	// Verify we have 3 keys
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Verify keys match
	if keys[0].PrivateKey != primaryKey.PrivateKey {
		t.Error("Primary key mismatch")
	}
	if keys[1].PrivateKey != additionalKey1.PrivateKey {
		t.Error("Additional key 1 mismatch")
	}
	if keys[2].PrivateKey != additionalKey2.PrivateKey {
		t.Error("Additional key 2 mismatch")
	}

	// Generate SOPS config
	sopsConfig, err := manager.GenerateSOPSConfig(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to generate SOPS config")

	// Verify SOPS config contains all public keys
	if !strings.Contains(sopsConfig, primaryKey.PublicKey) {
		t.Error("SOPS config missing primary key")
	}
	if !strings.Contains(sopsConfig, additionalKey1.PublicKey) {
		t.Error("SOPS config missing additional key 1")
	}
	if !strings.Contains(sopsConfig, additionalKey2.PublicKey) {
		t.Error("SOPS config missing additional key 2")
	}

	// Verify SOPS config has correct format
	if !strings.Contains(sopsConfig, "creation_rules:") {
		t.Error("SOPS config missing creation_rules")
	}
	if !strings.Contains(sopsConfig, "age: >-") {
		t.Error("SOPS config missing age key section")
	}
	if !strings.Contains(sopsConfig, "encrypted_regex:") {
		t.Error("SOPS config missing encrypted_regex")
	}

	// Clean up
	manager.DeleteKey(clusterName)
	manager.DeleteKey(clusterName + "-key-1")
	manager.DeleteKey(clusterName + "-key-2")
}

func TestMultiKeySOPSConfigurationWithSingleKey(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create key manager
	manager := NewEnhancedKeyManager(tempDir, slog.Default())
	manager.SetKeyringEnabled(false)

	clusterName := "single-key-cluster"

	// Generate only primary key
	primaryKey, err := manager.GenerateKey(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to generate primary key")

	// List cluster keys
	keys, err := manager.ListClusterKeys(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to list cluster keys")

	// Verify we have 1 key
	if len(keys) != 1 {
		t.Errorf("Expected 1 key, got %d", len(keys))
	}

	// Generate SOPS config
	sopsConfig, err := manager.GenerateSOPSConfig(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to generate SOPS config")

	// Verify SOPS config contains the public key
	if !strings.Contains(sopsConfig, primaryKey.PublicKey) {
		t.Error("SOPS config missing primary key")
	}

	// Clean up
	manager.DeleteKey(clusterName)
}

func TestKeyRotation(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create key manager
	manager := NewEnhancedKeyManager(tempDir, slog.Default())
	manager.SetKeyringEnabled(false)

	clusterName := "rotation-test-cluster"

	// Generate initial keys
	originalPrimaryKey, err := manager.GenerateKey(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to generate primary key")

	originalAdditionalKey, err := manager.GenerateAdditionalKey(clusterName, 1)
	testhelpers.AssertNoError(t, err, "Failed to generate additional key")

	// Rotate keys
	err = manager.RotateClusterKeys(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to rotate keys")

	// Retrieve new keys
	newKeys, err := manager.ListClusterKeys(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to list keys after rotation")

	// Verify we still have 2 keys
	if len(newKeys) != 2 {
		t.Errorf("Expected 2 keys after rotation, got %d", len(newKeys))
	}

	// Verify keys are different from original
	if newKeys[0].PrivateKey == originalPrimaryKey.PrivateKey {
		t.Error("Primary key was not rotated")
	}
	if newKeys[1].PrivateKey == originalAdditionalKey.PrivateKey {
		t.Error("Additional key was not rotated")
	}

	// Clean up
	manager.DeleteKey(clusterName)
	manager.DeleteKey(clusterName + "-key-1")
}

func TestListClusterKeysNoKeys(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create key manager
	manager := NewEnhancedKeyManager(tempDir, slog.Default())
	manager.SetKeyringEnabled(false)

	clusterName := "nonexistent-cluster"

	// Try to list keys for non-existent cluster
	_, err := manager.ListClusterKeys(clusterName)
	testhelpers.AssertError(t, err, "Expected error when listing keys for non-existent cluster")
}

func TestGenerateAdditionalKeyIndependence(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create key manager
	manager := NewEnhancedKeyManager(tempDir, slog.Default())
	manager.SetKeyringEnabled(false)

	clusterName := "independence-test"

	// Generate primary key
	primaryKey, err := manager.GenerateKey(clusterName)
	testhelpers.AssertNoError(t, err, "Failed to generate primary key")

	// Generate additional keys with different indices
	key1, err := manager.GenerateAdditionalKey(clusterName, 1)
	testhelpers.AssertNoError(t, err, "Failed to generate key 1")

	key2, err := manager.GenerateAdditionalKey(clusterName, 2)
	testhelpers.AssertNoError(t, err, "Failed to generate key 2")

	key5, err := manager.GenerateAdditionalKey(clusterName, 5)
	testhelpers.AssertNoError(t, err, "Failed to generate key 5")

	// Verify all keys are different
	keys := []*crypto.AgeKeyPair{primaryKey, key1, key2, key5}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i].PrivateKey == keys[j].PrivateKey {
				t.Errorf("Keys %d and %d are identical", i, j)
			}
			if keys[i].PublicKey == keys[j].PublicKey {
				t.Errorf("Public keys %d and %d are identical", i, j)
			}
		}
	}

	// Clean up
	manager.DeleteKey(clusterName)
	manager.DeleteKey(clusterName + "-key-1")
	manager.DeleteKey(clusterName + "-key-2")
	manager.DeleteKey(clusterName + "-key-5")
}
