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

package crypto

import (
	"fmt"
	"regexp"
	"strings"
)

// AgeKeyValidator implements KeyValidator for Age keys
type AgeKeyValidator struct{}

// NewAgeKeyValidator creates a new Age key validator
func NewAgeKeyValidator() *AgeKeyValidator {
	return &AgeKeyValidator{}
}

// ValidateAgeKey validates an age key format
func (v *AgeKeyValidator) ValidateAgeKey(key string) error {
	// Age public keys start with "age1" and are base64-encoded
	agePublicKeyRegex := regexp.MustCompile(`^age1[a-z0-9]{58}$`)
	if agePublicKeyRegex.MatchString(key) {
		return nil
	}

	// Age private keys start with "AGE-SECRET-KEY-1"
	agePrivateKeyRegex := regexp.MustCompile(`^AGE-SECRET-KEY-1[A-Z0-9]{58}$`)
	if agePrivateKeyRegex.MatchString(key) {
		return nil
	}

	return fmt.Errorf("invalid age key format: %s", key)
}

// ValidatePGPKey validates a PGP key format
func (v *AgeKeyValidator) ValidatePGPKey(key string) error {
	// PGP keys are typically 40-character hex strings (fingerprints)
	pgpKeyRegex := regexp.MustCompile(`^[A-F0-9]{40}$`)
	if pgpKeyRegex.MatchString(strings.ToUpper(key)) {
		return nil
	}

	// Also accept shorter key IDs
	shortPGPKeyRegex := regexp.MustCompile(`^[A-F0-9]{8,16}$`)
	if shortPGPKeyRegex.MatchString(strings.ToUpper(key)) {
		return nil
	}

	return fmt.Errorf("invalid PGP key format: %s", key)
}

// ValidateKeyForProduction validates that a key is not a placeholder
func (v *AgeKeyValidator) ValidateKeyForProduction(key string) error {
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

// ValidateKeyAccess validates that a key can be used for encryption/decryption
// This is a placeholder implementation that would need to be implemented by the concrete key manager
func (v *AgeKeyValidator) ValidateKeyAccess(keyName string) error {
	// This method should be implemented by the concrete key manager
	// as it requires access to the key storage and encryption/decryption capabilities
	return fmt.Errorf("ValidateKeyAccess must be implemented by the concrete key manager")
}

// ValidateKeyPair validates that a private and public key pair match
func ValidateKeyPair(privateKey, publicKey string) error {
	validator := NewAgeKeyValidator()

	// Validate individual keys
	if err := validator.ValidateAgeKey(privateKey); err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	if err := validator.ValidateAgeKey(publicKey); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Parse private key to derive public key
	keyPair, err := ParseAgeKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Check if derived public key matches provided public key
	if keyPair.PublicKey != publicKey {
		return fmt.Errorf("public key does not match private key")
	}

	return nil
}

// IsPlaceholderKey checks if a key is a placeholder
func IsPlaceholderKey(key string) bool {
	return strings.Contains(key, "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
}

// IsValidAgePublicKey checks if a string is a valid age public key
func IsValidAgePublicKey(key string) bool {
	validator := NewAgeKeyValidator()
	return validator.ValidateAgeKey(key) == nil && strings.HasPrefix(key, "age1")
}

// IsValidAgePrivateKey checks if a string is a valid age private key
func IsValidAgePrivateKey(key string) bool {
	validator := NewAgeKeyValidator()
	return validator.ValidateAgeKey(key) == nil && strings.HasPrefix(key, "AGE-SECRET-KEY-1")
}
