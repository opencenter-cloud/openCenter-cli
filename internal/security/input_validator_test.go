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
	"fmt"
	"strings"
	"testing"
)

func TestValidateClusterName(t *testing.T) {
	validator := NewDefaultInputValidator()

	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{
			name:      "valid simple name",
			input:     "my-cluster",
			wantError: false,
		},
		{
			name:      "valid with underscores",
			input:     "my_cluster_01",
			wantError: false,
		},
		{
			name:      "valid with numbers",
			input:     "cluster123",
			wantError: false,
		},
		{
			name:      "valid max length",
			input:     "a" + strings.Repeat("b", 62),
			wantError: false,
		},
		{
			name:      "valid mixed case",
			input:     "MyCluster-01",
			wantError: false,
		},

		// Invalid cases - path traversal
		{
			name:      "contains path traversal",
			input:     "../etc/passwd",
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name:      "contains forward slash",
			input:     "my/cluster",
			wantError: true,
			errorMsg:  "path separators",
		},
		{
			name:      "contains backslash",
			input:     "my\\cluster",
			wantError: true,
			errorMsg:  "path separators",
		},

		// Invalid cases - pattern violations
		{
			name:      "empty name",
			input:     "",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "starts with hyphen",
			input:     "-cluster",
			wantError: true,
			errorMsg:  "must start with alphanumeric",
		},
		{
			name:      "starts with underscore",
			input:     "_cluster",
			wantError: true,
			errorMsg:  "must start with alphanumeric",
		},
		{
			name:      "too long",
			input:     strings.Repeat("a", 64),
			wantError: true,
			errorMsg:  "max 63 characters",
		},
		{
			name:      "contains special characters",
			input:     "my-cluster!",
			wantError: true,
			errorMsg:  "must start with alphanumeric",
		},
		{
			name:      "contains spaces",
			input:     "my cluster",
			wantError: true,
			errorMsg:  "must start with alphanumeric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateClusterName(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateClusterName() expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateClusterName() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateClusterName() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateOrganizationName(t *testing.T) {
	validator := NewDefaultInputValidator()

	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{
			name:      "valid simple name",
			input:     "my-org",
			wantError: false,
		},
		{
			name:      "valid with underscores",
			input:     "my_org_01",
			wantError: false,
		},

		// Invalid cases
		{
			name:      "empty name",
			input:     "",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "contains path traversal",
			input:     "../etc",
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name:      "contains forward slash",
			input:     "my/org",
			wantError: true,
			errorMsg:  "path separators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateOrganizationName(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateOrganizationName() expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateOrganizationName() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateOrganizationName() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	validator := NewDefaultInputValidator()

	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{
			name:      "valid relative path",
			input:     "config/cluster.yaml",
			wantError: false,
		},
		{
			name:      "valid absolute path",
			input:     "/home/user/.config/openCenter/cluster.yaml",
			wantError: false,
		},
		{
			name:      "valid simple filename",
			input:     "cluster.yaml",
			wantError: false,
		},

		// Invalid cases
		{
			name:      "empty path",
			input:     "",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "contains path traversal",
			input:     "../../../etc/passwd",
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name:      "relative path with traversal",
			input:     "config/../../etc/passwd",
			wantError: true,
			errorMsg:  "path traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePath(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidatePath() expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidatePath() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePath() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	validator := NewDefaultInputValidator()

	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{
			name:      "valid HTTPS URL",
			input:     "https://example.com/api",
			wantError: false,
		},
		{
			name:      "valid localhost HTTP",
			input:     "http://localhost:8080",
			wantError: false,
		},
		{
			name:      "valid 127.0.0.1 HTTP",
			input:     "http://127.0.0.1:5000",
			wantError: false,
		},
		{
			name:      "valid private IP HTTP",
			input:     "http://192.168.1.100:5000",
			wantError: false,
		},
		{
			name:      "valid 10.x.x.x HTTP",
			input:     "http://10.0.0.1:5000",
			wantError: false,
		},
		{
			name:      "valid 172.16-31.x.x HTTP",
			input:     "http://172.16.0.1:5000",
			wantError: false,
		},

		// Invalid cases
		{
			name:      "empty URL",
			input:     "",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "external HTTP URL",
			input:     "http://example.com/api",
			wantError: true,
			errorMsg:  "must use HTTPS",
		},
		{
			name:      "invalid scheme",
			input:     "ftp://example.com",
			wantError: true,
			errorMsg:  "unsupported URL scheme",
		},
		{
			name:      "malformed URL",
			input:     "not a url",
			wantError: true,
			errorMsg:  "invalid URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateURL(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateURL() expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateURL() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateURL() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateEnvironmentVariable(t *testing.T) {
	validator := NewDefaultInputValidator()

	tests := []struct {
		name      string
		varName   string
		varValue  string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{
			name:      "valid EDITOR - vim",
			varName:   "EDITOR",
			varValue:  "vim",
			wantError: false,
		},
		{
			name:      "valid EDITOR - code",
			varName:   "EDITOR",
			varValue:  "code",
			wantError: false,
		},
		{
			name:      "valid EDITOR - nano",
			varName:   "EDITOR",
			varValue:  "nano",
			wantError: false,
		},
		{
			name:      "valid EDITOR - empty",
			varName:   "EDITOR",
			varValue:  "",
			wantError: false,
		},
		{
			name:      "valid regular variable",
			varName:   "MY_VAR",
			varValue:  "some-value",
			wantError: false,
		},

		// Invalid cases
		{
			name:      "empty variable name",
			varName:   "",
			varValue:  "value",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "EDITOR with semicolon",
			varName:   "EDITOR",
			varValue:  "vim; rm -rf /",
			wantError: true,
			errorMsg:  "shell metacharacter",
		},
		{
			name:      "EDITOR with pipe",
			varName:   "EDITOR",
			varValue:  "vim | cat",
			wantError: true,
			errorMsg:  "shell metacharacter",
		},
		{
			name:      "EDITOR with backtick",
			varName:   "EDITOR",
			varValue:  "vim`whoami`",
			wantError: true,
			errorMsg:  "shell metacharacter",
		},
		{
			name:      "EDITOR not in whitelist",
			varName:   "EDITOR",
			varValue:  "malicious-editor",
			wantError: true,
			errorMsg:  "not in the safe editors whitelist",
		},
		{
			name:      "regular variable with semicolon",
			varName:   "MY_VAR",
			varValue:  "value; rm -rf /",
			wantError: true,
			errorMsg:  "shell metacharacter",
		},
		{
			name:      "regular variable with pipe",
			varName:   "MY_VAR",
			varValue:  "value | cat",
			wantError: true,
			errorMsg:  "shell metacharacter",
		},
		{
			name:      "regular variable with ampersand",
			varName:   "MY_VAR",
			varValue:  "value & background",
			wantError: true,
			errorMsg:  "shell metacharacter",
		},
		{
			name:      "regular variable with dollar",
			varName:   "MY_VAR",
			varValue:  "value$injection",
			wantError: true,
			errorMsg:  "shell metacharacter",
		},
		{
			name:      "regular variable with newline",
			varName:   "MY_VAR",
			varValue:  "value\ninjection",
			wantError: true,
			errorMsg:  "shell metacharacter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateEnvironmentVariable(tt.varName, tt.varValue)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateEnvironmentVariable() expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateEnvironmentVariable() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateEnvironmentVariable() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestSanitizeShellInput(t *testing.T) {
	validator := NewDefaultInputValidator()

	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
		errorMsg  string
	}{
		// Valid cases that get sanitized
		{
			name:      "empty input",
			input:     "",
			want:      "",
			wantError: false,
		},
		{
			name:      "simple text",
			input:     "hello",
			want:      "hello",
			wantError: false,
		},
		{
			name:      "text with dollar sign",
			input:     "value$test",
			want:      "value\\$test",
			wantError: false,
		},
		{
			name:      "text with angle brackets",
			input:     "a<b>c",
			want:      "a\\<b\\>c",
			wantError: false,
		},
		{
			name:      "text with parentheses",
			input:     "func(arg)",
			want:      "func\\(arg\\)",
			wantError: false,
		},
		{
			name:      "text with braces",
			input:     "var{value}",
			want:      "var\\{value\\}",
			wantError: false,
		},

		// Invalid cases that should be rejected
		{
			name:      "contains semicolon",
			input:     "cmd; rm -rf /",
			want:      "",
			wantError: true,
			errorMsg:  "dangerous shell metacharacter",
		},
		{
			name:      "contains pipe",
			input:     "cmd | cat",
			want:      "",
			wantError: true,
			errorMsg:  "dangerous shell metacharacter",
		},
		{
			name:      "contains ampersand",
			input:     "cmd & background",
			want:      "",
			wantError: true,
			errorMsg:  "dangerous shell metacharacter",
		},
		{
			name:      "contains backtick",
			input:     "cmd`whoami`",
			want:      "",
			wantError: true,
			errorMsg:  "dangerous shell metacharacter",
		},
		{
			name:      "contains newline",
			input:     "cmd\ninjection",
			want:      "",
			wantError: true,
			errorMsg:  "dangerous shell metacharacter",
		},
		{
			name:      "contains carriage return",
			input:     "cmd\rinjection",
			want:      "",
			wantError: true,
			errorMsg:  "dangerous shell metacharacter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validator.SanitizeShellInput(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("SanitizeShellInput() expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("SanitizeShellInput() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("SanitizeShellInput() unexpected error = %v", err)
				}
				if got != tt.want {
					t.Errorf("SanitizeShellInput() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "test_field",
		Value:   "test_value",
		Message: "test message",
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "test_field") {
		t.Errorf("ValidationError.Error() should contain field name")
	}
	if !strings.Contains(errStr, "test message") {
		t.Errorf("ValidationError.Error() should contain message")
	}
	if !strings.Contains(errStr, "test_value") {
		t.Errorf("ValidationError.Error() should contain value")
	}
}

func TestValidationErrorLongValue(t *testing.T) {
	longValue := strings.Repeat("a", 100)
	err := &ValidationError{
		Field:   "test_field",
		Value:   longValue,
		Message: "test message",
	}

	errStr := err.Error()
	// Should be truncated
	if len(errStr) > 200 {
		t.Errorf("ValidationError.Error() should truncate long values")
	}
	if strings.Contains(errStr, longValue) {
		t.Errorf("ValidationError.Error() should not contain full long value")
	}
}

func TestIsValidationError(t *testing.T) {
	validationErr := &ValidationError{
		Field:   "test",
		Value:   "value",
		Message: "message",
	}

	if !IsValidationError(validationErr) {
		t.Errorf("IsValidationError() should return true for ValidationError")
	}

	regularErr := fmt.Errorf("regular error")
	if IsValidationError(regularErr) {
		t.Errorf("IsValidationError() should return false for regular error")
	}
}

// Benchmark tests
func BenchmarkValidateClusterName(b *testing.B) {
	validator := NewDefaultInputValidator()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateClusterName("my-cluster-123")
	}
}

func BenchmarkValidatePath(b *testing.B) {
	validator := NewDefaultInputValidator()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidatePath("/home/user/.config/openCenter/cluster.yaml")
	}
}

func BenchmarkValidateURL(b *testing.B) {
	validator := NewDefaultInputValidator()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateURL("https://example.com/api/v1")
	}
}

func BenchmarkSanitizeShellInput(b *testing.B) {
	validator := NewDefaultInputValidator()
	for i := 0; i < b.N; i++ {
		_, _ = validator.SanitizeShellInput("some input with $special (chars)")
	}
}
