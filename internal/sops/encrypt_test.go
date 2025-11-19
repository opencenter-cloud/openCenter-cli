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
	"strings"
	"testing"
)

func TestDefaultEncryptor_IsFileEncrypted(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name: "encrypted file with sops metadata",
			content: `apiVersion: v1
kind: Secret
metadata:
    name: test-secret
data:
    key: ENC[AES256_GCM,data:test,type:str]
sops:
    age:
        - recipient: age1test123
          enc: ENC[AES256_GCM,data:test,type:str]
    lastmodified: "2024-01-01T00:00:00Z"
    mac: ENC[AES256_GCM,data:test,type:str]
    version: 3.8.1`,
			expected: true,
		},
		{
			name: "unencrypted file",
			content: `apiVersion: v1
kind: Secret
metadata:
    name: test-secret
data:
    key: plaintext-value`,
			expected: false,
		},
		{
			name: "file with sops but no encryption keys",
			content: `apiVersion: v1
kind: Secret
sops:
    version: 3.8.1`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "test-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write test content
			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write test content: %v", err)
			}
			tmpFile.Close()

			e := NewDefaultEncryptor(nil, nil)
			result, err := e.IsFileEncrypted(tmpFile.Name())

			if err != nil {
				t.Errorf("IsFileEncrypted() error = %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("IsFileEncrypted() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultEncryptor_GetEncryptedContent(t *testing.T) {
	content := "test content"

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write test content
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	e := NewDefaultEncryptor(nil, nil)
	result, err := e.GetEncryptedContent(tmpFile.Name())

	if err != nil {
		t.Errorf("GetEncryptedContent() error = %v", err)
		return
	}

	if result != content {
		t.Errorf("GetEncryptedContent() = %v, want %v", result, content)
	}
}

func TestDefaultEncryptor_EncryptFile_FileNotFound(t *testing.T) {
	e := NewDefaultEncryptor([]string{"age1test123"}, nil)

	config := EncryptionConfig{
		AgeKeys: []string{"age1test123"},
		InPlace: true,
	}

	err := e.EncryptFile(context.Background(), "/nonexistent/file.yaml", config)
	if err == nil {
		t.Error("EncryptFile() should fail for non-existent file")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %v", err)
	}
}

func TestDefaultEncryptor_DecryptFile_FileNotFound(t *testing.T) {
	e := NewDefaultEncryptor(nil, nil)

	err := e.DecryptFile(context.Background(), "/nonexistent/file.yaml", "")
	if err == nil {
		t.Error("DecryptFile() should fail for non-existent file")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %v", err)
	}
}

func TestDefaultEncryptor_EncryptFiles(t *testing.T) {
	e := NewDefaultEncryptor([]string{"age1test123"}, nil)

	// Create temporary files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.yaml")
	file2 := filepath.Join(tmpDir, "file2.yaml")

	// Create test files
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := EncryptionConfig{
		AgeKeys: []string{"age1test123"},
		InPlace: true,
		DryRun:  true, // Use dry run to avoid actual SOPS execution
	}

	// This should not error even though SOPS is not available (dry run)
	err := e.EncryptFiles(context.Background(), []string{file1, file2}, config)
	if err != nil {
		t.Errorf("EncryptFiles() with dry run should not error: %v", err)
	}
}

func TestDefaultEncryptor_RotateKeys(t *testing.T) {
	e := NewDefaultEncryptor(nil, nil)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// This will fail because SOPS is not available, but we're testing the interface
	err = e.RotateKeys(context.Background(), tmpFile.Name(), []string{"age1newkey"}, nil)
	if err == nil {
		t.Error("RotateKeys() should fail when SOPS is not available")
	}
}

func TestCheckSOPSVersion(t *testing.T) {
	// This test checks the helper function
	version, err := checkSOPSVersion(context.Background())

	// We expect this to fail in test environment since SOPS CLI might not be installed
	// The important thing is that the method doesn't panic
	if err != nil {
		t.Logf("checkSOPSVersion() failed as expected in test environment: %v", err)
	} else {
		t.Logf("SOPS version: %s", version)
	}
}
