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
	"path/filepath"

	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
)

// KeyManager is an alias for crypto.KeyManager for backward compatibility
type KeyManager = crypto.KeyManager

// AgeKeyPair is an alias for crypto.AgeKeyPair for backward compatibility
type AgeKeyPair = crypto.AgeKeyPair

// KeyInfo is an alias for crypto.KeyInfo for backward compatibility
type KeyInfo = crypto.KeyInfo

// NewKeyManager creates a new key manager using the crypto utilities
func NewKeyManager(keyDir string) crypto.KeyManager {
	if keyDir == "" {
		keyDir = filepath.Join(os.Getenv("HOME"), ".config", "sops", "age")
	}
	return crypto.NewDefaultKeyManager(keyDir)
}

// SOPS-specific key management functions

// SetupSOPSEnvironment sets up the SOPS environment for a specific key
func SetupSOPSEnvironment(keyManager crypto.KeyManager, keyName string) error {
	// This is a SOPS-specific function that sets up environment variables
	// for SOPS to use the specified key

	// Get key info to find the file path
	keyInfo, err := keyManager.GetKeyInfo(keyName)
	if err != nil {
		return fmt.Errorf("failed to get key info: %w", err)
	}

	// Set SOPS_AGE_KEY_FILE environment variable
	if err := os.Setenv("SOPS_AGE_KEY_FILE", keyInfo.FilePath); err != nil {
		return fmt.Errorf("failed to set SOPS_AGE_KEY_FILE: %w", err)
	}

	return nil
}

// CheckSOPSInstallation checks if SOPS is properly installed
func CheckSOPSInstallation(ctx context.Context) error {
	cmd, err := security.GetDefaultCommandRunner().PrepareCommandContext(ctx, "sops", "--version")
	if err != nil {
		return fmt.Errorf("failed to prepare sops version command: %w", err)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SOPS is not installed or not in PATH: %w", err)
	}
	return nil
}

// ValidateSOPSKeyAccess validates that a key can be used for SOPS operations
func ValidateSOPSKeyAccess(keyManager crypto.KeyManager, keyName string) error {
	// Load the key
	keyPair, err := keyManager.LoadAgeKey(keyName)
	if err != nil {
		return fmt.Errorf("failed to load key: %w", err)
	}

	// Validate key format
	if err := keyManager.ValidateAgeKey(keyPair.PrivateKey); err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	if err := keyManager.ValidateAgeKey(keyPair.PublicKey); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Test basic key parsing
	if _, err := crypto.ParseAgeKey(keyPair.PrivateKey); err != nil {
		return fmt.Errorf("key validation failed - unable to parse private key: %w", err)
	}

	return nil
}
