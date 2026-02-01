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
	"testing"
)

func TestConfigValidator_Name(t *testing.T) {
	validator := NewConfigValidator()
	if validator.Name() != "config" {
		t.Errorf("expected name 'config', got '%s'", validator.Name())
	}
}

func TestConfigValidator_ValidateEmail(t *testing.T) {
	validator := NewConfigValidator()
	ctx := context.Background()

	tests := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{
			name:      "valid email",
			value:     "admin@example.com",
			wantValid: true,
		},
		{
			name:      "valid email with subdomain",
			value:     "user@mail.example.com",
			wantValid: true,
		},
		{
			name:      "empty email",
			value:     "",
			wantValid: false,
		},
		{
			name:      "invalid format no @",
			value:     "adminexample.com",
			wantValid: false,
		},
		{
			name:      "invalid format no domain",
			value:     "admin@",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, map[string]interface{}{
				"type":  "email",
				"value": tt.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestConfigValidator_ValidateDomain(t *testing.T) {
	validator := NewConfigValidator()
	ctx := context.Background()

	tests := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{
			name:      "valid domain",
			value:     "example.com",
			wantValid: true,
		},
		{
			name:      "valid subdomain",
			value:     "sub.example.com",
			wantValid: true,
		},
		{
			name:      "empty domain",
			value:     "",
			wantValid: false,
		},
		{
			name:      "consecutive dots",
			value:     "example..com",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, map[string]interface{}{
				"type":  "domain",
				"value": tt.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestConfigValidator_ValidateURL(t *testing.T) {
	validator := NewConfigValidator()
	ctx := context.Background()

	tests := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{
			name:      "valid https url",
			value:     "https://example.com",
			wantValid: true,
		},
		{
			name:      "valid http localhost",
			value:     "http://localhost:8080",
			wantValid: true,
		},
		{
			name:      "empty url",
			value:     "",
			wantValid: false,
		},
		{
			name:      "no scheme",
			value:     "example.com",
			wantValid: false,
		},
		{
			name:      "invalid scheme",
			value:     "ftp://example.com",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, map[string]interface{}{
				"type":  "url",
				"value": tt.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestConfigValidator_ValidateIP(t *testing.T) {
	validator := NewConfigValidator()
	ctx := context.Background()

	tests := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{
			name:      "valid ipv4",
			value:     "192.168.1.1",
			wantValid: true,
		},
		{
			name:      "valid ipv6",
			value:     "2001:db8::1",
			wantValid: true,
		},
		{
			name:      "empty ip",
			value:     "",
			wantValid: false,
		},
		{
			name:      "invalid ip",
			value:     "256.256.256.256",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, map[string]interface{}{
				"type":  "ip",
				"value": tt.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestConfigValidator_ValidateCIDR(t *testing.T) {
	validator := NewConfigValidator()
	ctx := context.Background()

	tests := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{
			name:      "valid cidr",
			value:     "192.168.1.0/24",
			wantValid: true,
		},
		{
			name:      "valid ipv6 cidr",
			value:     "2001:db8::/32",
			wantValid: true,
		},
		{
			name:      "empty cidr",
			value:     "",
			wantValid: false,
		},
		{
			name:      "invalid cidr",
			value:     "192.168.1.0",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, map[string]interface{}{
				"type":  "cidr",
				"value": tt.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestConfigValidator_ValidatePort(t *testing.T) {
	validator := NewConfigValidator()
	ctx := context.Background()

	tests := []struct {
		name      string
		value     interface{}
		wantValid bool
	}{
		{
			name:      "valid port",
			value:     8080,
			wantValid: true,
		},
		{
			name:      "valid port as string",
			value:     "8080",
			wantValid: true,
		},
		{
			name:      "port too low",
			value:     0,
			wantValid: false,
		},
		{
			name:      "port too high",
			value:     65536,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, map[string]interface{}{
				"type":  "port",
				"value": tt.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func BenchmarkConfigValidator_ValidateEmail(b *testing.B) {
	validator := NewConfigValidator()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.Validate(ctx, map[string]interface{}{
			"type":  "email",
			"value": "admin@example.com",
		})
	}
}
