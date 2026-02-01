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

func TestClusterNameValidator_Name(t *testing.T) {
	validator := NewClusterNameValidator()
	if validator.Name() != "cluster-name" {
		t.Errorf("expected name 'cluster-name', got %q", validator.Name())
	}
}

func TestClusterNameValidator_Validate(t *testing.T) {
	validator := NewClusterNameValidator()
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
			name:      "valid simple name",
			value:     "my-cluster",
			wantValid: true,
		},
		{
			name:      "valid with underscores",
			value:     "my_cluster_01",
			wantValid: true,
		},
		{
			name:      "valid with dots",
			value:     "my.cluster.dev",
			wantValid: true,
		},
		{
			name:      "valid alphanumeric",
			value:     "cluster123",
			wantValid: true,
		},
		{
			name:          "empty string",
			value:         "",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "cannot be empty",
		},
		{
			name:          "not a string",
			value:         123,
			wantValid:     false,
			wantErrors:    1,
			errorContains: "must be a string",
		},
		{
			name:          "path traversal",
			value:         "../cluster",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "path traversal",
		},
		{
			name:          "forward slash",
			value:         "my/cluster",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "path separators",
		},
		{
			name:          "backslash",
			value:         "my\\cluster",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "path separators",
		},
		{
			name:          "too long",
			value:         strings.Repeat("a", 64),
			wantValid:     false,
			wantErrors:    1,
			errorContains: "too long",
		},
		{
			name:          "starts with hyphen",
			value:         "-cluster",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "invalid",
		},
		{
			name:          "starts with special char",
			value:         "!cluster",
			wantValid:     false,
			wantErrors:    1,
			errorContains: "invalid",
		},
		{
			name:         "leading hyphen warning",
			value:        "a-",
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name:         "consecutive hyphens",
			value:        "my--cluster",
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name:         "reserved name default",
			value:        "default",
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name:         "reserved name kube-system",
			value:        "kube-system",
			wantValid:    true,
			wantWarnings: 1,
		},
		{
			name:      "max length valid",
			value:     strings.Repeat("a", 63),
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

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d", len(result.Warnings), tt.wantWarnings)
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

func TestOrganizationNameValidator_Name(t *testing.T) {
	validator := NewOrganizationNameValidator()
	if validator.Name() != "organization-name" {
		t.Errorf("expected name 'organization-name', got %q", validator.Name())
	}
}

func TestOrganizationNameValidator_Validate(t *testing.T) {
	validator := NewOrganizationNameValidator()
	ctx := context.Background()

	tests := []struct {
		name      string
		value     interface{}
		wantValid bool
	}{
		{
			name:      "valid organization name",
			value:     "my-org",
			wantValid: true,
		},
		{
			name:      "empty string",
			value:     "",
			wantValid: false,
		},
		{
			name:      "path traversal",
			value:     "../org",
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

			// Check that field names are updated to "organization_name"
			for _, issue := range result.Errors {
				if issue.Field != "organization_name" {
					t.Errorf("expected field 'organization_name', got %q", issue.Field)
				}
				if strings.Contains(issue.Message, "cluster name") {
					t.Errorf("message should not contain 'cluster name': %s", issue.Message)
				}
			}
		})
	}
}

func BenchmarkClusterNameValidator_Validate(b *testing.B) {
	validator := NewClusterNameValidator()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.Validate(ctx, "my-cluster-01")
	}
}

func BenchmarkOrganizationNameValidator_Validate(b *testing.B) {
	validator := NewOrganizationNameValidator()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.Validate(ctx, "my-org")
	}
}
