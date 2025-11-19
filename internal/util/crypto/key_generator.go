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
	"crypto/rand"
	"fmt"
	"time"

	"filippo.io/age"
)

// AgeKeyGenerator implements KeyGenerator for Age keys
type AgeKeyGenerator struct{}

// NewAgeKeyGenerator creates a new Age key generator
func NewAgeKeyGenerator() *AgeKeyGenerator {
	return &AgeKeyGenerator{}
}

// GenerateAgeKey generates a new age key pair with validation
func (g *AgeKeyGenerator) GenerateAgeKey() (*AgeKeyPair, error) {
	// Generate age identity
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("failed to generate age identity: %w", err)
	}

	keyPair := &AgeKeyPair{
		PrivateKey: identity.String(),
		PublicKey:  identity.Recipient().String(),
		Recipient:  identity.Recipient().String(),
	}

	// Validate generated key pair
	validator := NewAgeKeyValidator()
	if err := validator.ValidateAgeKey(keyPair.PrivateKey); err != nil {
		return nil, fmt.Errorf("generated private key validation failed: %w", err)
	}
	if err := validator.ValidateAgeKey(keyPair.PublicKey); err != nil {
		return nil, fmt.Errorf("generated public key validation failed: %w", err)
	}

	return keyPair, nil
}

// GenerateFallbackKey generates a fallback age key when no key is available
func (g *AgeKeyGenerator) GenerateFallbackKey() (*AgeKeyPair, error) {
	// Generate a new key pair
	keyPair, err := g.GenerateAgeKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate fallback key: %w", err)
	}

	return keyPair, nil
}

// GenerateRandomPassword generates a random password for key encryption
func (g *AgeKeyGenerator) GenerateRandomPassword(length int) (string, error) {
	if length <= 0 {
		length = 32
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	password := make([]byte, length)

	for i := range password {
		randomIndex := make([]byte, 1)
		if _, err := rand.Read(randomIndex); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}
		password[i] = charset[randomIndex[0]%byte(len(charset))]
	}

	return string(password), nil
}

// ParseAgeKey parses an age private key and returns the key pair
func ParseAgeKey(privateKey string) (*AgeKeyPair, error) {
	// Parse private key to get public key
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

// GenerateKeyWithTimestamp generates a key with a timestamp-based name
func GenerateKeyWithTimestamp(prefix string) (string, *AgeKeyPair, error) {
	generator := NewAgeKeyGenerator()
	keyPair, err := generator.GenerateAgeKey()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate key: %w", err)
	}

	keyName := fmt.Sprintf("%s-%d", prefix, time.Now().Unix())
	return keyName, keyPair, nil
}
