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
	"strings"
	"testing"
)

func TestSecurityValidator_Name(t *testing.T) {
	validator := NewSecurityValidator()
	if validator.Name() != "security" {
		t.Errorf("expected name 'security', got %q", validator.Name())
	}
}

func TestSecurityValidator_ValidateShellInput(t *testing.T) {
	validator := NewSecurityValidator()
	ctx := context.Background()

	tests := []struct {
		name          string
		value         map[string]interface{}
		wantValid     bool
		wantErrors    int
		wantWarnings  int
		errorContains string
	}{
		{
			name: "safe input",
			value: map[string]interface{}{
				"type":  "shell-input",
				"value": "my-cluster",
			},
			wantValid: true,
		},
		{
			name: "semicolon injection",
			value: map[string]interface{}{
				"type":  "shell-input",
				"value": "cluster; rm -rf /",
			},
			wantValid:     false,
			wantErrors:    1,
			errorContains: "dangerous",
		},
		{
			name: "pipe injection",
			value: map[string]interface{}{
				"type":  "shell-input",
				"value": "cluster | cat /etc/passwd",
			},
			wantValid:     false,
			wantErrors:    1,
			errorContains: "dangerous",
		},
		{
			name: "command substitution",
			value: map[string]interface{}{
				"type":  "shell-input",
				"value": "cluster$(whoami)",
			},
			wantValid:     false,
			wantErrors:    1,
			errorContains: "dangerous",
		},
		{
			name: "backtick substitution",
			value: map[string]interface{}{
				"type":  "shell-input",
				"value": "cluster`whoami`",
			},
			wantValid:     false,
			wantErrors:    1,
			errorContains: "dangerous",
		},
		{
			name: "empty input",
			value: map[string]interface{}{
				"type":  "shell-input",
				"value": "",
			},
			wantValid: true,
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

func TestSecurityValidator_ValidateEnvironmentVariable(t *testing.T) {
	validator := NewSecurityValidator()
	ctx := context.Background()

	tests := []struct {
		name         string
		value        map[string]interface{}
		wantValid    bool
		wantWarnings int
	}{
		{
			name: "valid env var",
			value: map[string]interface{}{
				"type":  "environment-variable",
				"name":  "MY_VAR",
				"value": "safe-value",
			},
			wantValid: true,
		},
		{
			name: "secret keyword warning",
			value: map[string]interface{}{
				"type":  "environment-variable",
				"name":  "API_SECRET",
				"value": "secret-value",
			},
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name: "empty name",
			value: map[string]interface{}{
				"type":  "environment-variable",
				"name":  "",
				"value": "value",
			},
			wantValid: false,
		},
		{
			name: "missing name",
			value: map[string]interface{}{
				"type":  "environment-variable",
				"value": "value",
			},
			wantValid: false,
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

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
			}
		})
	}
}

func TestSecurityValidator_ValidateEditor(t *testing.T) {
	validator := NewSecurityValidator()
	ctx := context.Background()

	tests := []struct {
		name         string
		value        map[string]interface{}
		wantValid    bool
		wantWarnings int
	}{
		{
			name: "safe editor",
			value: map[string]interface{}{
				"type":  "editor",
				"value": "vim",
			},
			wantValid: true,
		},
		{
			name: "safe editor with path",
			value: map[string]interface{}{
				"type":  "editor",
				"value": "/usr/bin/vim",
			},
			wantValid: true,
		},
		{
			name: "unsafe editor warning",
			value: map[string]interface{}{
				"type":  "editor",
				"value": "unknown-editor",
			},
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name: "editor with metacharacters",
			value: map[string]interface{}{
				"type":  "editor",
				"value": "vim; rm -rf /",
			},
			wantValid: false,
		},
		{
			name: "empty editor",
			value: map[string]interface{}{
				"type":  "editor",
				"value": "",
			},
			wantValid: true,
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

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
			}
		})
	}
}

func TestSecurityValidator_ValidateCommand(t *testing.T) {
	validator := NewSecurityValidator()
	ctx := context.Background()

	tests := []struct {
		name         string
		value        map[string]interface{}
		wantValid    bool
		wantWarnings int
	}{
		{
			name: "safe command",
			value: map[string]interface{}{
				"type":  "command",
				"value": "ls -la",
			},
			wantValid: true,
		},
		{
			name: "sudo warning",
			value: map[string]interface{}{
				"type":  "command",
				"value": "sudo apt-get update",
			},
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name: "dangerous rm command",
			value: map[string]interface{}{
				"type":  "command",
				"value": "rm -rf /",
			},
			wantValid: false,
		},
		{
			name: "command substitution",
			value: map[string]interface{}{
				"type":  "command",
				"value": "echo $(whoami)",
			},
			wantValid: false,
		},
		{
			name: "empty command",
			value: map[string]interface{}{
				"type":  "command",
				"value": "",
			},
			wantValid: false,
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

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
			}
		})
	}
}

func TestSecurityValidator_ValidateSecret(t *testing.T) {
	validator := NewSecurityValidator()
	ctx := context.Background()

	tests := []struct {
		name      string
		value     map[string]interface{}
		wantValid bool
	}{
		{
			name: "SOPS encrypted",
			value: map[string]interface{}{
				"type":  "secret",
				"value": "ENC[AES256_GCM,data:abc123,iv:def456,tag:ghi789,type:str]",
			},
			wantValid: true,
		},
		{
			name: "AWS access key pattern",
			value: map[string]interface{}{
				"type":  "secret",
				"value": "AKIAIOSFODNN7EXAMPLE",
			},
			wantValid: false,
		},
		{
			name: "GitHub token pattern",
			value: map[string]interface{}{
				"type":  "secret",
				"value": "ghp_1234567890abcdefghijklmnopqrstuv123456",
			},
			wantValid: false,
		},
		{
			name: "empty secret",
			value: map[string]interface{}{
				"type":  "secret",
				"value": "",
			},
			wantValid: false,
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
		})
	}
}

func TestSecurityValidator_SetSafeEditors(t *testing.T) {
	validator := NewSecurityValidator()
	ctx := context.Background()

	// Set custom safe editors
	validator.SetSafeEditors([]string{"custom-editor"})

	result, err := validator.Validate(ctx, map[string]interface{}{
		"type":  "editor",
		"value": "custom-editor",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("custom editor should be valid")
	}
	if len(result.Warnings) > 0 {
		t.Error("custom editor should not have warnings")
	}

	// vim should now be unsafe
	result, err = validator.Validate(ctx, map[string]interface{}{
		"type":  "editor",
		"value": "vim",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Error("vim should have warning after custom editors set")
	}
}

func BenchmarkSecurityValidator_ValidateShellInput(b *testing.B) {
	validator := NewSecurityValidator()
	ctx := context.Background()
	value := map[string]interface{}{
		"type":  "shell-input",
		"value": "my-cluster-01",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.Validate(ctx, value)
	}
}

func BenchmarkSecurityValidator_ValidateCommand(b *testing.B) {
	validator := NewSecurityValidator()
	ctx := context.Background()
	value := map[string]interface{}{
		"type":  "command",
		"value": "ls -la /tmp",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.Validate(ctx, value)
	}
}
