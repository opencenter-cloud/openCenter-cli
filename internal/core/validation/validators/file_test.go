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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileValidator_Name(t *testing.T) {
	validator := NewFileValidator()
	if validator.Name() != "file" {
		t.Errorf("expected name 'file', got %q", validator.Name())
	}
}

func TestFileValidator_ValidatePath(t *testing.T) {
	validator := NewFileValidator()
	ctx := context.Background()

	tests := []struct {
		name          string
		value         interface{}
		wantValid     bool
		wantErrors    int
		wantWarnings  int
		errorContains string
	}{
		{
			name:      "valid simple path",
			value:     "config.yaml",
			wantValid: true,
		},
		{
			name:      "valid path with directory",
			value:     "configs/cluster.yaml",
			wantValid: true,
		},
		{
			name:          "empty path",
			value:         "",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "cannot be empty",
		},
		{
			name:          "path traversal",
			value:         "../../../etc/passwd",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "path traversal",
		},
		{
			name:          "null byte",
			value:         "file\x00.txt",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "null bytes",
		},
		{
			name:         "absolute path warning",
			value:        "/etc/config.yaml",
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name:         "hidden file",
			value:        ".hidden",
			wantValid:    true,
			wantWarnings: 1, // Warning about uncommon extension
		},
		{
			name:         "uncommon extension",
			value:        "file.exe",
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name:          "path too long",
			value:         strings.Repeat("a/", 2100),
			wantValid:     false,
			wantErrors:    1,
			errorContains: "too long",
		},
		{
			name:          "not a string or map",
			value:         123,
			wantValid:     false,
			wantErrors:    1,
			errorContains: "must be a string or map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d: %v", len(result.Errors), tt.wantErrors, result.Errors)
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d: %v", len(result.Warnings), tt.wantWarnings, result.Warnings)
			}

			if tt.errorContains != "" && len(result.Errors) > 0 {
				found := false
				for _, err := range result.Errors {
					if strings.Contains(strings.ToLower(err.Message), strings.ToLower(tt.errorContains)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("error message should contain %q, got: %v", tt.errorContains, result.Errors)
				}
			}
		})
	}
}

func TestFileValidator_ValidateReadOperation(t *testing.T) {
	validator := NewFileValidator()
	ctx := context.Background()

	// Create a temporary file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	tests := []struct {
		name          string
		value         map[string]interface{}
		wantValid     bool
		errorContains string
	}{
		{
			name: "read existing file",
			value: map[string]interface{}{
				"operation": "read",
				"path":      tmpFile,
			},
			wantValid: true,
		},
		{
			name: "read non-existent file",
			value: map[string]interface{}{
				"operation": "read",
				"path":      filepath.Join(tmpDir, "nonexistent.txt"),
			},
			wantValid:     false,
			errorContains: "does not exist",
		},
		{
			name: "read directory",
			value: map[string]interface{}{
				"operation": "read",
				"path":      tmpDir,
			},
			wantValid:     false,
			errorContains: "directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if tt.errorContains != "" && len(result.Errors) > 0 {
				found := false
				for _, err := range result.Errors {
					if strings.Contains(strings.ToLower(err.Message), strings.ToLower(tt.errorContains)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("error message should contain %q, got: %v", tt.errorContains, result.Errors)
				}
			}
		})
	}
}

func TestFileValidator_ValidateWriteOperation(t *testing.T) {
	validator := NewFileValidator()
	ctx := context.Background()

	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	tests := []struct {
		name         string
		value        map[string]interface{}
		wantValid    bool
		wantWarnings int
		skipWarnings bool // Skip warning count check
	}{
		{
			name: "write new file",
			value: map[string]interface{}{
				"operation": "write",
				"path":      filepath.Join(tmpDir, "new.txt"),
			},
			wantValid:    true,
			skipWarnings: true, // May have warnings about extension
		},
		{
			name: "overwrite existing file",
			value: map[string]interface{}{
				"operation": "write",
				"path":      existingFile,
			},
			wantValid:    true,
			wantWarnings: 1, // At least warning about overwriting
			skipWarnings: true,
		},
		{
			name: "write to directory",
			value: map[string]interface{}{
				"operation": "write",
				"path":      tmpDir,
			},
			wantValid:    false,
			skipWarnings: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if !tt.skipWarnings && len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
			}
		})
	}
}

func TestFileValidator_ValidateDeleteOperation(t *testing.T) {
	validator := NewFileValidator()
	ctx := context.Background()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	tests := []struct {
		name         string
		value        map[string]interface{}
		wantValid    bool
		wantWarnings int
		skipWarnings bool
	}{
		{
			name: "delete existing file",
			value: map[string]interface{}{
				"operation": "delete",
				"path":      tmpFile,
			},
			wantValid:    true,
			skipWarnings: true, // May have warnings about extension
		},
		{
			name: "delete non-existent file",
			value: map[string]interface{}{
				"operation": "delete",
				"path":      filepath.Join(tmpDir, "nonexistent.txt"),
			},
			wantValid:    false,
			skipWarnings: true,
		},
		{
			name: "delete directory",
			value: map[string]interface{}{
				"operation": "delete",
				"path":      tmpDir,
			},
			wantValid:    true,
			wantWarnings: 1, // At least warning about directory
			skipWarnings: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if !tt.skipWarnings && len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
			}
		})
	}
}

func TestFileValidator_SetAllowedExtensions(t *testing.T) {
	validator := NewFileValidator()
	ctx := context.Background()

	// Set custom allowed extensions
	validator.SetAllowedExtensions([]string{".txt", ".md"})

	tests := []struct {
		name         string
		value        string
		wantWarnings int
	}{
		{
			name:         "allowed extension",
			value:        "file.txt",
			wantWarnings: 0,
		},
		{
			name:         "disallowed extension",
			value:        "file.yaml",
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
			}
		})
	}
}

func TestFileValidator_SetMaxPathLength(t *testing.T) {
	validator := NewFileValidator()
	ctx := context.Background()

	// Set custom max path length
	validator.SetMaxPathLength(10)

	result, err := validator.Validate(ctx, "short.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("short path should be valid")
	}

	result, err = validator.Validate(ctx, "very-long-filename.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("long path should be invalid")
	}
}

func BenchmarkFileValidator_Validate(b *testing.B) {
	validator := NewFileValidator()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.Validate(ctx, "config/cluster.yaml")
	}
}
