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

package security

import (
	"regexp"
	"strings"
	"testing"
)

func TestDefaultCredentialMasker_MaskString_AWSAccessKey(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "AWS access key in plain text",
			input:    "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			expected: "AWS_ACCESS_KEY_ID=AKIA****MPLE",
		},
		{
			name:     "AWS access key in log message",
			input:    "Using credentials AKIAIOSFODNN7EXAMPLE for authentication",
			expected: "Using credentials AKIA****MPLE for authentication",
		},
		{
			name:     "Multiple AWS access keys",
			input:    "Key1: AKIAIOSFODNN7EXAMPLE, Key2: AKIAJ7EXAMPLE1234567",
			expected: "Key1: AKIA****MPLE, Key2: AKIA****4567",
		},
		{
			name:     "No AWS access key",
			input:    "This is a normal log message",
			expected: "This is a normal log message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)
			if result != tt.expected {
				t.Errorf("MaskString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultCredentialMasker_MaskString_AgeSecretKey(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Age secret key in plain text",
			input:    "AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE",
			expected: "AGE-SECRET-KEY-****",
		},
		{
			name:     "Age secret key in log message",
			input:    "Generated key: AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE",
			expected: "Generated key: AGE-SECRET-KEY-****",
		},
		{
			name:     "No Age secret key",
			input:    "This is a normal log message",
			expected: "This is a normal log message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)
			if result != tt.expected {
				t.Errorf("MaskString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultCredentialMasker_MaskString_Passwords(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	tests := []struct {
		name          string
		input         string
		shouldContain string
		shouldNotContain string
	}{
		{
			name:             "Password with equals",
			input:            "password=mysecretpass123",
			shouldContain:    "password=",
			shouldNotContain: "mysecretpass123",
		},
		{
			name:             "Password with colon",
			input:            "password: mysecretpass123",
			shouldContain:    "password:",
			shouldNotContain: "mysecretpass123",
		},
		{
			name:             "PASSWORD in uppercase",
			input:            "PASSWORD=MySecretPass456",
			shouldContain:    "PASSWORD=",
			shouldNotContain: "MySecretPass456",
		},
		{
			name:             "pwd abbreviation",
			input:            "pwd=shortpwd",
			shouldContain:    "pwd=",
			shouldNotContain: "shortpwd",
		},
		{
			name:             "Password with quotes",
			input:            `password="quoted_password"`,
			shouldContain:    "password=",
			shouldNotContain: "quoted_password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)
			if !strings.Contains(result, tt.shouldContain) {
				t.Errorf("MaskString() result should contain %q, got %v", tt.shouldContain, result)
			}
			if strings.Contains(result, tt.shouldNotContain) {
				t.Errorf("MaskString() result should not contain %q, got %v", tt.shouldNotContain, result)
			}
		})
	}
}

func TestDefaultCredentialMasker_MaskString_Tokens(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	tests := []struct {
		name          string
		input         string
		shouldContain string
		shouldNotContain string
	}{
		{
			name:             "Token with equals",
			input:            "token=abc123def456ghi789",
			shouldContain:    "token=",
			shouldNotContain: "abc123def456ghi789",
		},
		{
			name:             "Bearer token",
			input:            "Authorization: bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			shouldContain:    "bearer",
			shouldNotContain: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:             "TOKEN in uppercase",
			input:            "TOKEN=MyTokenValue123",
			shouldContain:    "TOKEN=",
			shouldNotContain: "MyTokenValue123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)
			if !strings.Contains(result, tt.shouldContain) {
				t.Errorf("MaskString() result should contain %q, got %v", tt.shouldContain, result)
			}
			if strings.Contains(result, tt.shouldNotContain) {
				t.Errorf("MaskString() result should not contain %q, got %v", tt.shouldNotContain, result)
			}
		})
	}
}

func TestDefaultCredentialMasker_MaskString_PrivateKeys(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	privateKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890abcdefghijklmnopqrstuvwxyz
ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnop
-----END RSA PRIVATE KEY-----`

	result := masker.MaskString(privateKey)

	if !strings.Contains(result, "-----BEGIN PRIVATE KEY-----") {
		t.Errorf("MaskString() should preserve BEGIN marker")
	}
	if !strings.Contains(result, "-----END PRIVATE KEY-----") {
		t.Errorf("MaskString() should preserve END marker")
	}
	if !strings.Contains(result, "***MASKED***") {
		t.Errorf("MaskString() should mask the key content")
	}
	if strings.Contains(result, "MIIEpAIBAAKCAQEA") {
		t.Errorf("MaskString() should not contain original key content")
	}
}

func TestDefaultCredentialMasker_MaskString_EmptyInput(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	result := masker.MaskString("")
	if result != "" {
		t.Errorf("MaskString(\"\") = %v, want empty string", result)
	}
}

func TestDefaultCredentialMasker_MaskBytes(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "AWS access key in bytes",
			input:    []byte("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"),
			expected: []byte("AWS_ACCESS_KEY_ID=AKIA****MPLE"),
		},
		{
			name:     "Empty byte slice",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "No credentials in bytes",
			input:    []byte("This is a normal message"),
			expected: []byte("This is a normal message"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskBytes(tt.input)
			if string(result) != string(tt.expected) {
				t.Errorf("MaskBytes() = %v, want %v", string(result), string(tt.expected))
			}
		})
	}
}

func TestDefaultCredentialMasker_RegisterPattern(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	// Register a custom pattern for a fictional credential type
	customPattern := regexp.MustCompile(`CUSTOM-KEY-[A-Z0-9]{20}`)
	masker.RegisterPattern("custom_key", customPattern)

	// Test that the custom pattern is registered
	// The custom pattern should be in the patterns map
	// Note: The current implementation doesn't automatically mask custom patterns
	// This test verifies the pattern is registered
	if masker.patterns["custom_key"] == nil {
		t.Errorf("RegisterPattern() did not register the custom pattern")
	}
}

func TestDefaultCredentialMasker_GetMaskedCount(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	// Initially, count should be 0
	if count := masker.GetMaskedCount(); count != 0 {
		t.Errorf("GetMaskedCount() = %v, want 0", count)
	}

	// Mask a string with credentials
	masker.MaskString("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE")

	// Count should be incremented
	if count := masker.GetMaskedCount(); count != 1 {
		t.Errorf("GetMaskedCount() = %v, want 1", count)
	}

	// Mask another string with credentials
	masker.MaskString("password=mysecret")

	// Count should be incremented again
	if count := masker.GetMaskedCount(); count != 2 {
		t.Errorf("GetMaskedCount() = %v, want 2", count)
	}

	// Mask a string without credentials
	masker.MaskString("This is a normal message")

	// Count should not change
	if count := masker.GetMaskedCount(); count != 2 {
		t.Errorf("GetMaskedCount() = %v, want 2 (should not increment for non-credential strings)", count)
	}
}

func TestDefaultCredentialMasker_ConcurrentAccess(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	// Test concurrent access to MaskString and GetMaskedCount
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			masker.MaskString("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE")
			masker.GetMaskedCount()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify that the count is correct (should be 10)
	if count := masker.GetMaskedCount(); count != 10 {
		t.Errorf("GetMaskedCount() = %v, want 10 (concurrent access test)", count)
	}
}

func TestDefaultCredentialMasker_MultipleCredentialsInSingleString(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	input := `Configuration:
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
password=mysecretpassword
token=abc123def456ghi789jkl012
AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE`

	result := masker.MaskString(input)

	// Verify that all credentials are masked
	if strings.Contains(result, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("Result should not contain original AWS access key")
	}
	if strings.Contains(result, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY") {
		t.Errorf("Result should not contain original AWS secret key")
	}
	if strings.Contains(result, "mysecretpassword") {
		t.Errorf("Result should not contain original password")
	}
	if strings.Contains(result, "abc123def456ghi789jkl012") {
		t.Errorf("Result should not contain original token")
	}
	if strings.Contains(result, "1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE") {
		t.Errorf("Result should not contain original Age secret key")
	}

	// Verify that masked markers are present
	if !strings.Contains(result, "***MASKED***") {
		t.Errorf("Result should contain masked markers")
	}
}

func TestDefaultCredentialMasker_OpenStackCredentials(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	tests := []struct {
		name          string
		input         string
		shouldContain string
		shouldNotContain string
	}{
		{
			name:             "OpenStack application credential secret",
			input:            "application_credential_secret=abcdef1234567890abcdef",
			shouldContain:    "application_credential_secret=",
			shouldNotContain: "abcdef1234567890abcdef",
		},
		{
			name:             "OpenStack app cred with underscores",
			input:            "application-credential-secret=xyz789012345678901234",
			shouldContain:    "application-credential-secret=",
			shouldNotContain: "xyz789012345678901234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)
			if !strings.Contains(result, tt.shouldContain) {
				t.Errorf("MaskString() result should contain %q, got %v", tt.shouldContain, result)
			}
			if strings.Contains(result, tt.shouldNotContain) {
				t.Errorf("MaskString() result should not contain %q, got %v", tt.shouldNotContain, result)
			}
		})
	}
}

func TestDefaultCredentialMasker_GenericAPIKeys(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	tests := []struct {
		name          string
		input         string
		shouldContain string
		shouldNotContain string
	}{
		{
			name:             "API key with equals (32+ chars)",
			input:            "api_key=abcdef1234567890abcdef1234567890",
			shouldContain:    "api_key=",
			shouldNotContain: "abcdef1234567890abcdef1234567890",
		},
		{
			name:             "API key with hyphen (32+ chars)",
			input:            "api-key=xyz789012345678901234567890123456",
			shouldContain:    "api-key=",
			shouldNotContain: "xyz789012345678901234567890123456",
		},
		{
			name:             "APIKEY in uppercase (32+ chars)",
			input:            "APIKEY=ABC123DEF456GHI789JKL012MNO345PQR",
			shouldContain:    "APIKEY=",
			shouldNotContain: "ABC123DEF456GHI789JKL012MNO345PQR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)
			if !strings.Contains(result, tt.shouldContain) {
				t.Errorf("MaskString() result should contain %q, got %v", tt.shouldContain, result)
			}
			if strings.Contains(result, tt.shouldNotContain) {
				t.Errorf("MaskString() result should not contain %q, got %v", tt.shouldNotContain, result)
			}
		})
	}
}

func TestDefaultCredentialMasker_EdgeCases(t *testing.T) {
	masker := NewDefaultCredentialMasker()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Very short AWS-like key (should still mask)",
			input:    "AKIATEST",
			expected: "AKIATEST", // Too short to match pattern
		},
		{
			name:     "AWS key at start of string",
			input:    "AKIAIOSFODNN7EXAMPLE is the key",
			expected: "AKIA****MPLE is the key",
		},
		{
			name:     "AWS key at end of string",
			input:    "The key is AKIAIOSFODNN7EXAMPLE",
			expected: "The key is AKIA****MPLE",
		},
		{
			name:     "Password with very short value",
			input:    "password=ab",
			expected: "password=ab", // Too short to match pattern (min 3 chars)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)
			if result != tt.expected {
				t.Errorf("MaskString() = %v, want %v", result, tt.expected)
			}
		})
	}
}
