// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crypto

import (
	"strings"
	"testing"
)

func TestNewAgeKeyGenerator(t *testing.T) {
	generator := NewAgeKeyGenerator()
	if generator == nil {
		t.Error("NewAgeKeyGenerator() returned nil")
	}
}

func TestAgeKeyGenerator_GenerateAgeKey(t *testing.T) {
	generator := NewAgeKeyGenerator()
	
	keyPair, err := generator.GenerateAgeKey()
	if err != nil {
		t.Fatalf("GenerateAgeKey() failed: %v", err)
	}
	
	if keyPair == nil {
		t.Fatal("GenerateAgeKey() returned nil key pair")
	}
	
	// Test private key format
	if !strings.HasPrefix(keyPair.PrivateKey, "AGE-SECRET-KEY-1") {
		t.Errorf("Private key should start with 'AGE-SECRET-KEY-1', got: %s", keyPair.PrivateKey[:20])
	}
	
	// Test private key length (should be 74 characters)
	if len(keyPair.PrivateKey) != 74 {
		t.Errorf("Private key should be 74 characters, got: %d", len(keyPair.PrivateKey))
	}
	
	// Test public key format
	if !strings.HasPrefix(keyPair.PublicKey, "age1") {
		t.Errorf("Public key should start with 'age1', got: %s", keyPair.PublicKey[:10])
	}
	
	// Test that recipient matches public key
	if keyPair.Recipient != keyPair.PublicKey {
		t.Error("Recipient should match PublicKey")
	}
	
	// Test that keys are not empty
	if keyPair.PrivateKey == "" {
		t.Error("Private key should not be empty")
	}
	if keyPair.PublicKey == "" {
		t.Error("Public key should not be empty")
	}
}

func TestAgeKeyGenerator_GenerateMultipleKeys(t *testing.T) {
	generator := NewAgeKeyGenerator()
	
	// Generate multiple keys and ensure they're different
	keys := make([]*AgeKeyPair, 5)
	for i := 0; i < 5; i++ {
		keyPair, err := generator.GenerateAgeKey()
		if err != nil {
			t.Fatalf("GenerateAgeKey() failed on iteration %d: %v", i, err)
		}
		keys[i] = keyPair
	}
	
	// Verify all keys are unique
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i].PrivateKey == keys[j].PrivateKey {
				t.Errorf("Generated duplicate private keys at indices %d and %d", i, j)
			}
			if keys[i].PublicKey == keys[j].PublicKey {
				t.Errorf("Generated duplicate public keys at indices %d and %d", i, j)
			}
		}
	}
}

func TestAgeKeyGenerator_GenerateFallbackKey(t *testing.T) {
	generator := NewAgeKeyGenerator()
	
	keyPair, err := generator.GenerateFallbackKey()
	if err != nil {
		t.Fatalf("GenerateFallbackKey() failed: %v", err)
	}
	
	if keyPair == nil {
		t.Fatal("GenerateFallbackKey() returned nil key pair")
	}
	
	// Should have same format as regular key generation
	if !strings.HasPrefix(keyPair.PrivateKey, "AGE-SECRET-KEY-1") {
		t.Errorf("Fallback private key should start with 'AGE-SECRET-KEY-1', got: %s", keyPair.PrivateKey[:20])
	}
	
	if !strings.HasPrefix(keyPair.PublicKey, "age1") {
		t.Errorf("Fallback public key should start with 'age1', got: %s", keyPair.PublicKey[:10])
	}
}

func TestAgeKeyGenerator_GenerateRandomPassword(t *testing.T) {
	generator := NewAgeKeyGenerator()
	
	tests := []struct {
		name   string
		length int
	}{
		{"default length", 0},  // Should use default of 32
		{"length 8", 8},
		{"length 16", 16},
		{"length 64", 64},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password, err := generator.GenerateRandomPassword(tt.length)
			if err != nil {
				t.Fatalf("GenerateRandomPassword(%d) failed: %v", tt.length, err)
			}
			
			expectedLength := tt.length
			if expectedLength <= 0 {
				expectedLength = 32 // Default length
			}
			
			if len(password) != expectedLength {
				t.Errorf("Expected password length %d, got %d", expectedLength, len(password))
			}
			
			// Verify password contains expected character set
			charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
			for _, char := range password {
				if !strings.ContainsRune(charset, char) {
					t.Errorf("Password contains unexpected character: %c", char)
				}
			}
		})
	}
}

func TestAgeKeyGenerator_GenerateRandomPasswordUniqueness(t *testing.T) {
	generator := NewAgeKeyGenerator()
	
	// Generate multiple passwords and ensure they're different
	passwords := make([]string, 10)
	for i := 0; i < 10; i++ {
		password, err := generator.GenerateRandomPassword(32)
		if err != nil {
			t.Fatalf("GenerateRandomPassword() failed on iteration %d: %v", i, err)
		}
		passwords[i] = password
	}
	
	// Verify all passwords are unique
	for i := 0; i < len(passwords); i++ {
		for j := i + 1; j < len(passwords); j++ {
			if passwords[i] == passwords[j] {
				t.Errorf("Generated duplicate passwords at indices %d and %d", i, j)
			}
		}
	}
}

func TestParseAgeKey(t *testing.T) {
	// First generate a key to test parsing
	generator := NewAgeKeyGenerator()
	originalKeyPair, err := generator.GenerateAgeKey()
	if err != nil {
		t.Fatalf("Failed to generate key for testing: %v", err)
	}
	
	// Test parsing the generated private key
	parsedKeyPair, err := ParseAgeKey(originalKeyPair.PrivateKey)
	if err != nil {
		t.Fatalf("ParseAgeKey() failed: %v", err)
	}
	
	if parsedKeyPair == nil {
		t.Fatal("ParseAgeKey() returned nil key pair")
	}
	
	// Verify the parsed key matches the original
	if parsedKeyPair.PrivateKey != originalKeyPair.PrivateKey {
		t.Error("Parsed private key doesn't match original")
	}
	
	if parsedKeyPair.PublicKey != originalKeyPair.PublicKey {
		t.Error("Parsed public key doesn't match original")
	}
	
	if parsedKeyPair.Recipient != originalKeyPair.Recipient {
		t.Error("Parsed recipient doesn't match original")
	}
}

func TestParseAgeKey_InvalidKey(t *testing.T) {
	tests := []struct {
		name       string
		privateKey string
	}{
		{"empty string", ""},
		{"invalid format", "not-a-valid-key"},
		{"wrong prefix", "INVALID-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{"too short", "AGE-SECRET-KEY-1ABC"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAgeKey(tt.privateKey)
			if err == nil {
				t.Errorf("ParseAgeKey(%q) should have failed but didn't", tt.privateKey)
			}
		})
	}
}

func TestGenerateKeyWithTimestamp(t *testing.T) {
	prefix := "test-key"
	
	keyName, keyPair, err := GenerateKeyWithTimestamp(prefix)
	if err != nil {
		t.Fatalf("GenerateKeyWithTimestamp() failed: %v", err)
	}
	
	if keyName == "" {
		t.Error("Key name should not be empty")
	}
	
	if !strings.HasPrefix(keyName, prefix+"-") {
		t.Errorf("Key name should start with '%s-', got: %s", prefix, keyName)
	}
	
	if keyPair == nil {
		t.Fatal("Key pair should not be nil")
	}
	
	// Verify the key pair is valid
	if !strings.HasPrefix(keyPair.PrivateKey, "AGE-SECRET-KEY-1") {
		t.Error("Generated key pair should have valid private key format")
	}
	
	if !strings.HasPrefix(keyPair.PublicKey, "age1") {
		t.Error("Generated key pair should have valid public key format")
	}
}

func TestGenerateKeyWithTimestamp_Format(t *testing.T) {
	prefix := "formattest"  // Use prefix without hyphens to simplify testing
	
	keyName, _, err := GenerateKeyWithTimestamp(prefix)
	if err != nil {
		t.Fatalf("GenerateKeyWithTimestamp() failed: %v", err)
	}
	
	// Verify the key name starts with the prefix
	if !strings.HasPrefix(keyName, prefix+"-") {
		t.Errorf("Key name should start with '%s-', got: %s", prefix, keyName)
	}
	
	// Extract the timestamp part (everything after the last hyphen)
	lastHyphen := strings.LastIndex(keyName, "-")
	if lastHyphen == -1 {
		t.Errorf("Key name should contain a hyphen, got: %s", keyName)
		return
	}
	
	timestampPart := keyName[lastHyphen+1:]
	if timestampPart == "" {
		t.Error("Timestamp part should not be empty")
	}
	
	// Verify it's a reasonable timestamp (should be numeric)
	for _, char := range timestampPart {
		if char < '0' || char > '9' {
			t.Errorf("Timestamp part should be numeric, got: %s", timestampPart)
			break
		}
	}
}