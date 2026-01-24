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

package config

import (
	"strings"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
)

func TestMultiLayerValidator_ValidateServices(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		wantErrs []string
	}{
		{
			name: "weave-gitops requires fluxcd",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: map[string]any{
						"weave-gitops": &services.WeaveGitOpsConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
						"fluxcd": &services.DefaultServiceConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
					},
				},
			},
			wantErrs: []string{
				"weave-gitops",
			},
		},
		{
			name: "headlamp requires keycloak",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: map[string]any{
						"headlamp": &services.HeadlampConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
						"keycloak": &services.KeycloakConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
					},
				},
			},
			wantErrs: []string{
				"headlamp",
			},
		},
		{
			name: "headlamp with OIDC requires keycloak",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: map[string]any{
						"headlamp": &services.HeadlampConfig{
							BaseConfig:    services.BaseConfig{Enabled: true},
							OIDCIssuerURL: "https://keycloak.example.com/realms/myrealm",
						},
						"keycloak": &services.KeycloakConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
					},
				},
			},
			wantErrs: []string{
				"headlamp",
				"keycloak",
			},
		},
		{
			name: "all dependencies satisfied",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: map[string]any{
						"weave-gitops": &services.WeaveGitOpsConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
						"fluxcd": &services.DefaultServiceConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
						"headlamp": &services.HeadlampConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
						"keycloak": &services.KeycloakConfig{
							BaseConfig: services.BaseConfig{Enabled: true},
						},
					},
				},
			},
			wantErrs: []string{},
		},
		{
			name: "services disabled - no validation errors",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Services: map[string]any{
						"weave-gitops": &services.WeaveGitOpsConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
						"fluxcd": &services.DefaultServiceConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
						"headlamp": &services.HeadlampConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
						"keycloak": &services.KeycloakConfig{
							BaseConfig: services.BaseConfig{Enabled: false},
						},
					},
				},
			},
			wantErrs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewMultiLayerValidator()
			errors := validator.ValidateServices(tt.config)

			if len(errors) != len(tt.wantErrs) {
				t.Errorf("ValidateServices() got %d errors, want %d errors", len(errors), len(tt.wantErrs))
				for _, err := range errors {
					t.Logf("Got error: %s", err.Message)
				}
				return
			}

			for i, wantErr := range tt.wantErrs {
				found := false
				for _, err := range errors {
					if strings.Contains(err.Message, wantErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateServices() error[%d] should contain %q, but not found in any error", i, wantErr)
					for _, err := range errors {
						t.Logf("Got error: %s", err.Message)
					}
				}
			}
		})
	}
}

func TestMultiLayerValidator_Validate_ServiceDependencies(t *testing.T) {
	// Test that service dependency validation is included in the full validation pipeline
	config := &Config{
		OpenCenter: SimplifiedOpenCenter{
			Services: map[string]any{
				"weave-gitops": &services.WeaveGitOpsConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				},
				"fluxcd": &services.DefaultServiceConfig{
					BaseConfig: services.BaseConfig{Enabled: false},
				},
			},
		},
	}

	validator := NewMultiLayerValidator()
	errors := validator.Validate(config)

	// Should have at least one error for the missing fluxcd dependency
	foundServiceError := false
	for _, err := range errors {
		if err.Code == "E014" && strings.Contains(err.Message, "weave-gitops") && strings.Contains(err.Message, "fluxcd") {
			foundServiceError = true
			break
		}
	}

	if !foundServiceError {
		t.Error("Validate() should include service dependency validation errors")
		for _, err := range errors {
			t.Logf("Got error: [%s] %s: %s", err.Code, err.Field, err.Message)
		}
	}
}

func TestMultiLayerValidator_ServiceDependencyErrorCode(t *testing.T) {
	// Verify that service dependency errors use the correct error code
	config := &Config{
		OpenCenter: SimplifiedOpenCenter{
			Services: map[string]any{
				"weave-gitops": &services.WeaveGitOpsConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				},
				"fluxcd": &services.DefaultServiceConfig{
					BaseConfig: services.BaseConfig{Enabled: false},
				},
			},
		},
	}

	validator := NewMultiLayerValidator()
	errors := validator.ValidateServices(config)

	if len(errors) == 0 {
		t.Fatal("ValidateServices() should return errors for missing dependencies")
	}

	for _, err := range errors {
		if err.Code != "E014" {
			t.Errorf("Service dependency error should have code E014, got %s", err.Code)
		}
	}
}
