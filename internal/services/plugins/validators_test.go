/*
Copyright 2025 Victor Palma <victor.palma@rackspace.com>
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

package plugins

import (
	"context"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
)

func TestCertManagerValidator(t *testing.T) {
	validator := NewCertManagerValidator()

	tests := []struct {
		name        string
		config      *services.CertManagerConfig
		wantValid   bool
		wantErrors  int
		errorFields []string
	}{
		{
			name: "valid config with https letsencrypt server",
			config: &services.CertManagerConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
				Email:             "admin@example.com",
			},
			wantValid:  true,
			wantErrors: 0,
		},
		{
			name: "invalid letsencrypt server without https",
			config: &services.CertManagerConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				LetsEncryptServer: "http://acme-v02.api.letsencrypt.org/directory",
				Email:             "admin@example.com",
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"cert-manager.letsencrypt_server"},
		},
		{
			name: "invalid email format",
			config: &services.CertManagerConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
				Email:             "invalid-email",
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"cert-manager.email"},
		},
		{
			name: "empty config is valid",
			config: &services.CertManagerConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
			},
			wantValid:  true,
			wantErrors: 0,
		},
		{
			name: "multiple validation errors",
			config: &services.CertManagerConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				LetsEncryptServer: "ftp://invalid-protocol.com",
				Email:             "not-an-email",
			},
			wantValid:   false,
			wantErrors:  2,
			errorFields: []string{"cert-manager.letsencrypt_server", "cert-manager.email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(context.Background(), tt.config)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("Validate() errors = %d, want %d", len(result.Errors), tt.wantErrors)
				for _, err := range result.Errors {
					t.Logf("  Error: %s - %s", err.Field, err.Message)
				}
			}

			// Check that expected error fields are present
			for _, expectedField := range tt.errorFields {
				found := false
				for _, err := range result.Errors {
					if err.Field == expectedField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error for field %s not found", expectedField)
				}
			}
		})
	}
}

func TestCertManagerValidator_InvalidType(t *testing.T) {
	validator := NewCertManagerValidator()

	result, err := validator.Validate(context.Background(), "invalid-type")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Valid {
		t.Error("Validate() should fail for invalid config type")
	}

	if len(result.Errors) == 0 {
		t.Error("Validate() should return error for invalid config type")
	}
}

func TestKeycloakValidator(t *testing.T) {
	validator := NewKeycloakValidator()

	tests := []struct {
		name        string
		config      *services.KeycloakConfig
		wantValid   bool
		wantErrors  int
		wantWarnings int
		errorFields []string
	}{
		{
			name: "valid production config",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances:       2,
				StartOptimized:  true,
				FrontendURL:     "https://keycloak.example.com",
				LogLevel:        "INFO",
				LogFormat:       "json",
				CacheStack:      "kubernetes",
				DBPoolMinSize:   5,
				DBPoolMaxSize:   20,
				MinReplicas:     2,
				MaxReplicas:     10,
			},
			wantValid:  true,
			wantErrors: 0,
		},
		{
			name: "production mode requires 2+ instances",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances:      1,
				StartOptimized: true,
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.instances"},
		},
		{
			name: "instances must be at least 1",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances: 0,
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.instances"},
		},
		{
			name: "min_replicas cannot exceed max_replicas",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances:   2,
				MinReplicas: 5,
				MaxReplicas: 3,
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.autoscaling"},
		},
		{
			name: "min_replicas < 2 generates warning",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances:   2,
				MinReplicas: 1,
				MaxReplicas: 5,
			},
			wantValid:    true,
			wantErrors:   0,
			wantWarnings: 1,
		},
		{
			name: "db_pool_min_size cannot exceed db_pool_max_size",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances:     2,
				DBPoolMinSize: 20,
				DBPoolMaxSize: 10,
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.db_pool"},
		},
		{
			name: "frontend_url must be valid URL",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances:   2,
				FrontendURL: "not-a-url",
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.frontend_url"},
		},
		{
			name: "production mode requires https frontend_url",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances:      2,
				StartOptimized: true,
				FrontendURL:    "http://keycloak.example.com",
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.frontend_url"},
		},
		{
			name: "invalid log level",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances: 2,
				LogLevel:  "INVALID",
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.log_level"},
		},
		{
			name: "invalid log format",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances: 2,
				LogFormat: "xml",
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.log_format"},
		},
		{
			name: "invalid cache stack",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				Instances:  2,
				CacheStack: "redis",
			},
			wantValid:   false,
			wantErrors:  1,
			errorFields: []string{"keycloak.cache_stack"},
		},
		{
			name: "disabled service skips validation",
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: false,
				},
				Instances:      0,
				StartOptimized: true,
			},
			wantValid:  true,
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(context.Background(), tt.config)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("Validate() errors = %d, want %d", len(result.Errors), tt.wantErrors)
				for _, err := range result.Errors {
					t.Logf("  Error: %s - %s", err.Field, err.Message)
				}
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("Validate() warnings = %d, want %d", len(result.Warnings), tt.wantWarnings)
			}

			// Check that expected error fields are present
			for _, expectedField := range tt.errorFields {
				found := false
				for _, err := range result.Errors {
					if err.Field == expectedField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error for field %s not found", expectedField)
				}
			}
		})
	}
}

func TestKeycloakValidator_InvalidType(t *testing.T) {
	validator := NewKeycloakValidator()

	result, err := validator.Validate(context.Background(), "invalid-type")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Valid {
		t.Error("Validate() should fail for invalid config type")
	}

	if len(result.Errors) == 0 {
		t.Error("Validate() should return error for invalid config type")
	}
}

func TestValidatorNames(t *testing.T) {
	tests := []struct {
		name      string
		validator interface{ Name() string }
		wantName  string
	}{
		{
			name:      "CertManagerValidator",
			validator: NewCertManagerValidator(),
			wantName:  "service:cert-manager",
		},
		{
			name:      "KeycloakValidator",
			validator: NewKeycloakValidator(),
			wantName:  "service:keycloak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.validator.Name(); got != tt.wantName {
				t.Errorf("Name() = %v, want %v", got, tt.wantName)
			}
		})
	}
}
