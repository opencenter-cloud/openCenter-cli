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

package validators

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockFileInfo implements fs.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// mockFileSystem implements fs.FileSystem for testing
type mockFileSystem struct {
	files       map[string][]byte
	permissions map[string]os.FileMode
	readError   error
	statError   error
}

func newMockFileSystem() *mockFileSystem {
	return &mockFileSystem{
		files:       make(map[string][]byte),
		permissions: make(map[string]os.FileMode),
	}
}

func (m *mockFileSystem) addFile(path string, content []byte, perm os.FileMode) {
	m.files[path] = content
	m.permissions[path] = perm
}

func (m *mockFileSystem) ReadFile(path string) ([]byte, error) {
	if m.readError != nil {
		return nil, m.readError
	}
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.files[path] = data
	m.permissions[path] = perm
	return nil
}

func (m *mockFileSystem) WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return m.WriteFile(path, data, perm)
}

func (m *mockFileSystem) Exists(path string) bool {
	_, ok := m.files[path]
	return ok
}

func (m *mockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (m *mockFileSystem) Remove(path string) error {
	delete(m.files, path)
	delete(m.permissions, path)
	return nil
}

func (m *mockFileSystem) Stat(path string) (fs.FileInfo, error) {
	if m.statError != nil {
		return nil, m.statError
	}
	if _, ok := m.files[path]; ok {
		perm := m.permissions[path]
		if perm == 0 {
			perm = 0644 // Default permission
		}
		return &mockFileInfo{
			name:    filepath.Base(path),
			size:    int64(len(m.files[path])),
			mode:    perm,
			modTime: time.Now(),
			isDir:   false,
		}, nil
	}
	return nil, os.ErrNotExist
}

func TestSOPSKeyValidator_Name(t *testing.T) {
	mockFS := newMockFileSystem()
	validator := NewSOPSKeyValidator(mockFS)

	if got := validator.Name(); got != "sops-key" {
		t.Errorf("Name() = %q, want %q", got, "sops-key")
	}
}

func TestSOPSKeyValidator_ValidKey(t *testing.T) {
	mockFS := newMockFileSystem()
	validKey := "AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE"
	mockFS.addFile("/path/to/key.txt", []byte(validKey), 0600)

	validator := NewSOPSKeyValidator(mockFS)
	result, err := validator.Validate(context.Background(), "/path/to/key.txt")

	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if !result.Valid {
		t.Errorf("Validate() result.Valid = false, want true")
		for _, e := range result.Errors {
			t.Errorf("  Error: %s: %s", e.Field, e.Message)
		}
	}

	if result.HasWarnings() {
		t.Errorf("Validate() has warnings, want none")
		for _, w := range result.Warnings {
			t.Errorf("  Warning: %s: %s", w.Field, w.Message)
		}
	}
}

func TestSOPSKeyValidator_InvalidDataType(t *testing.T) {
	mockFS := newMockFileSystem()
	validator := NewSOPSKeyValidator(mockFS)

	// Pass an integer instead of string
	result, err := validator.Validate(context.Background(), 123)

	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if result.Valid {
		t.Errorf("Validate() result.Valid = true, want false")
	}

	if !result.HasErrors() {
		t.Errorf("Validate() has no errors, want error about invalid data type")
	}

	if len(result.Errors) > 0 {
		if !strings.Contains(result.Errors[0].Message, "invalid data type") {
			t.Errorf("Error message = %q, want to contain 'invalid data type'", result.Errors[0].Message)
		}
	}
}

func TestSOPSKeyValidator_MissingFile(t *testing.T) {
	mockFS := newMockFileSystem()
	validator := NewSOPSKeyValidator(mockFS)

	result, err := validator.Validate(context.Background(), "/nonexistent/key.txt")

	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if result.Valid {
		t.Errorf("Validate() result.Valid = true, want false")
	}

	if !result.HasErrors() {
		t.Fatalf("Validate() has no errors, want error about missing file")
	}

	// Check error message
	if !strings.Contains(result.Errors[0].Message, "not found") {
		t.Errorf("Error message = %q, want to contain 'not found'", result.Errors[0].Message)
	}

	// Check suggestions
	if len(result.Errors[0].Suggestions) == 0 {
		t.Errorf("Error has no suggestions, want at least one")
	}

	// Verify suggestions are actionable
	foundAgeKeygen := false
	for _, s := range result.Errors[0].Suggestions {
		if strings.Contains(s, "age-keygen") {
			foundAgeKeygen = true
			break
		}
	}
	if !foundAgeKeygen {
		t.Errorf("Suggestions don't include age-keygen command")
	}
}

func TestSOPSKeyValidator_UnreadableFile(t *testing.T) {
	mockFS := newMockFileSystem()
	mockFS.addFile("/path/to/key.txt", []byte("content"), 0600)
	mockFS.readError = os.ErrPermission

	validator := NewSOPSKeyValidator(mockFS)
	result, err := validator.Validate(context.Background(), "/path/to/key.txt")

	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if result.Valid {
		t.Errorf("Validate() result.Valid = true, want false")
	}

	if !result.HasErrors() {
		t.Fatalf("Validate() has no errors, want error about unreadable file")
	}

	// Check error message
	if !strings.Contains(result.Errors[0].Message, "cannot read") {
		t.Errorf("Error message = %q, want to contain 'cannot read'", result.Errors[0].Message)
	}

	// Check suggestions mention permissions
	foundPermissionSuggestion := false
	for _, s := range result.Errors[0].Suggestions {
		if strings.Contains(s, "permissions") {
			foundPermissionSuggestion = true
			break
		}
	}
	if !foundPermissionSuggestion {
		t.Errorf("Suggestions don't mention permissions")
	}
}

func TestSOPSKeyValidator_InvalidFormat(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "wrong prefix",
			content: "INVALID-SECRET-KEY-1234567890",
		},
		{
			name:    "no prefix",
			content: "1234567890ABCDEF",
		},
		{
			name:    "empty file",
			content: "",
		},
		{
			name:    "random text",
			content: "this is not a valid age key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := newMockFileSystem()
			mockFS.addFile("/path/to/key.txt", []byte(tt.content), 0600)

			validator := NewSOPSKeyValidator(mockFS)
			result, err := validator.Validate(context.Background(), "/path/to/key.txt")

			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}

			if result.Valid {
				t.Errorf("Validate() result.Valid = true, want false for invalid format")
			}

			if !result.HasErrors() {
				t.Fatalf("Validate() has no errors, want error about invalid format")
			}

			// Check error message mentions format
			if !strings.Contains(result.Errors[0].Message, "format") {
				t.Errorf("Error message = %q, want to contain 'format'", result.Errors[0].Message)
			}

			// Check suggestions mention AGE-SECRET-KEY-
			foundFormatSuggestion := false
			for _, s := range result.Errors[0].Suggestions {
				if strings.Contains(s, "AGE-SECRET-KEY-") {
					foundFormatSuggestion = true
					break
				}
			}
			if !foundFormatSuggestion {
				t.Errorf("Suggestions don't mention AGE-SECRET-KEY- format")
			}
		})
	}
}

func TestSOPSKeyValidator_InsecurePermissions(t *testing.T) {
	tests := []struct {
		name string
		perm os.FileMode
		want bool // want warning
	}{
		{
			name: "secure permissions 0600",
			perm: 0600,
			want: false,
		},
		{
			name: "insecure permissions 0644",
			perm: 0644,
			want: true,
		},
		{
			name: "insecure permissions 0666",
			perm: 0666,
			want: true,
		},
		{
			name: "insecure permissions 0777",
			perm: 0777,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := newMockFileSystem()
			validKey := "AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE"
			mockFS.addFile("/path/to/key.txt", []byte(validKey), tt.perm)

			validator := NewSOPSKeyValidator(mockFS)
			result, err := validator.Validate(context.Background(), "/path/to/key.txt")

			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}

			if !result.Valid {
				t.Errorf("Validate() result.Valid = false, want true (permissions should be warning, not error)")
			}

			hasWarning := result.HasWarnings()
			if hasWarning != tt.want {
				t.Errorf("Validate() has warning = %v, want %v", hasWarning, tt.want)
			}

			if tt.want && len(result.Warnings) > 0 {
				// Check warning message mentions permissions
				if !strings.Contains(result.Warnings[0].Message, "permissions") {
					t.Errorf("Warning message = %q, want to contain 'permissions'", result.Warnings[0].Message)
				}

				// Check suggestions mention chmod
				foundChmodSuggestion := false
				for _, s := range result.Warnings[0].Suggestions {
					if strings.Contains(s, "chmod 600") {
						foundChmodSuggestion = true
						break
					}
				}
				if !foundChmodSuggestion {
					t.Errorf("Suggestions don't mention chmod 600")
				}
			}
		})
	}
}

func TestSOPSKeyValidator_KeyWithWhitespace(t *testing.T) {
	mockFS := newMockFileSystem()
	// Key with leading/trailing whitespace (should be trimmed)
	validKey := "\n  AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE  \n"
	mockFS.addFile("/path/to/key.txt", []byte(validKey), 0600)

	validator := NewSOPSKeyValidator(mockFS)
	result, err := validator.Validate(context.Background(), "/path/to/key.txt")

	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if !result.Valid {
		t.Errorf("Validate() result.Valid = false, want true (whitespace should be trimmed)")
		for _, e := range result.Errors {
			t.Errorf("  Error: %s: %s", e.Field, e.Message)
		}
	}
}

func TestSOPSKeyValidator_StatError(t *testing.T) {
	mockFS := newMockFileSystem()
	validKey := "AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE"
	mockFS.addFile("/path/to/key.txt", []byte(validKey), 0600)
	mockFS.statError = os.ErrPermission

	validator := NewSOPSKeyValidator(mockFS)
	result, err := validator.Validate(context.Background(), "/path/to/key.txt")

	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	// Should still pass validation even if stat fails
	// (we can't check permissions, but the key format is valid)
	if !result.Valid {
		t.Errorf("Validate() result.Valid = false, want true (stat error should not fail validation)")
	}

	// Should not have warnings if stat fails
	if result.HasWarnings() {
		t.Errorf("Validate() has warnings, want none (can't check permissions if stat fails)")
	}
}
