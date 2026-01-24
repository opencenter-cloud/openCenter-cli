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

package services

import (
	"strings"
	"testing"
)

func TestDependencyValidator_ValidateDependencies(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]any
		wantErrs []string
	}{
		{
			name: "weave-gitops enabled without fluxcd",
			services: map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{
				"service 'weave-gitops' requires 'fluxcd' to be enabled",
			},
		},
		{
			name: "weave-gitops enabled with fluxcd",
			services: map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
			},
			wantErrs: []string{},
		},
		{
			name: "weave-gitops disabled without fluxcd",
			services: map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{},
		},
		{
			name: "headlamp enabled without keycloak",
			services: map[string]any{
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{
				"service 'headlamp' requires 'keycloak' to be enabled",
			},
		},
		{
			name: "headlamp enabled with keycloak",
			services: map[string]any{
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
			},
			wantErrs: []string{},
		},
		{
			name: "multiple dependency violations",
			services: map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{
				"service 'weave-gitops' requires 'fluxcd' to be enabled",
				"service 'headlamp' requires 'keycloak' to be enabled",
			},
		},
		{
			name: "all services disabled",
			services: map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{},
		},
		{
			name: "service not in map",
			services: map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
				// fluxcd not in map at all
			},
			wantErrs: []string{
				"service 'weave-gitops' requires 'fluxcd' to be enabled",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDependencyValidator()
			errors := validator.ValidateDependencies(tt.services)

			if len(errors) != len(tt.wantErrs) {
				t.Errorf("ValidateDependencies() got %d errors, want %d errors", len(errors), len(tt.wantErrs))
				t.Logf("Got errors: %v", errors)
				t.Logf("Want errors: %v", tt.wantErrs)
				return
			}

			for i, wantErr := range tt.wantErrs {
				if !strings.Contains(errors[i], wantErr) {
					t.Errorf("ValidateDependencies() error[%d] = %q, want to contain %q", i, errors[i], wantErr)
				}
			}
		})
	}
}

func TestDependencyValidator_ValidateHeadlampOIDC(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]any
		wantErrs []string
	}{
		{
			name: "headlamp with OIDC issuer URL but no keycloak",
			services: map[string]any{
				"headlamp": &HeadlampConfig{
					BaseConfig:    BaseConfig{Enabled: true},
					OIDCIssuerURL: "https://keycloak.example.com/realms/myrealm",
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{
				"service 'headlamp' has OIDC configured but requires 'keycloak' to be enabled",
			},
		},
		{
			name: "headlamp with OIDC client ID but no keycloak",
			services: map[string]any{
				"headlamp": &HeadlampConfig{
					BaseConfig:   BaseConfig{Enabled: true},
					OIDCClientID: "headlamp-client",
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{
				"service 'headlamp' has OIDC configured but requires 'keycloak' to be enabled",
			},
		},
		{
			name: "headlamp with OIDC and keycloak enabled",
			services: map[string]any{
				"headlamp": &HeadlampConfig{
					BaseConfig:    BaseConfig{Enabled: true},
					OIDCIssuerURL: "https://keycloak.example.com/realms/myrealm",
					OIDCClientID:  "headlamp-client",
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
			},
			wantErrs: []string{},
		},
		{
			name: "headlamp without OIDC and no keycloak",
			services: map[string]any{
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{},
		},
		{
			name: "headlamp disabled with OIDC configured",
			services: map[string]any{
				"headlamp": &HeadlampConfig{
					BaseConfig:    BaseConfig{Enabled: false},
					OIDCIssuerURL: "https://keycloak.example.com/realms/myrealm",
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			},
			wantErrs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDependencyValidator()
			errors := validator.ValidateHeadlampOIDC(tt.services)

			if len(errors) != len(tt.wantErrs) {
				t.Errorf("ValidateHeadlampOIDC() got %d errors, want %d errors", len(errors), len(tt.wantErrs))
				t.Logf("Got errors: %v", errors)
				t.Logf("Want errors: %v", tt.wantErrs)
				return
			}

			for i, wantErr := range tt.wantErrs {
				if !strings.Contains(errors[i], wantErr) {
					t.Errorf("ValidateHeadlampOIDC() error[%d] = %q, want to contain %q", i, errors[i], wantErr)
				}
			}
		})
	}
}

func TestDependencyValidator_GetDependencyGraph(t *testing.T) {
	validator := NewDependencyValidator()
	graph := validator.GetDependencyGraph()

	if len(graph) == 0 {
		t.Error("GetDependencyGraph() returned empty graph")
	}

	// Check that expected dependencies are present
	foundWeaveGitOps := false
	foundHeadlamp := false

	for _, dep := range graph {
		if dep.Service == "weave-gitops" {
			foundWeaveGitOps = true
			if len(dep.Dependencies) == 0 {
				t.Error("weave-gitops should have dependencies")
			}
			if dep.Reason == "" {
				t.Error("weave-gitops dependency should have a reason")
			}
		}
		if dep.Service == "headlamp" {
			foundHeadlamp = true
			if len(dep.Dependencies) == 0 {
				t.Error("headlamp should have dependencies")
			}
			if dep.Reason == "" {
				t.Error("headlamp dependency should have a reason")
			}
		}
	}

	if !foundWeaveGitOps {
		t.Error("weave-gitops dependency not found in graph")
	}
	if !foundHeadlamp {
		t.Error("headlamp dependency not found in graph")
	}
}

func TestDependencyValidator_AddDependency(t *testing.T) {
	validator := NewDependencyValidator()
	initialCount := len(validator.GetDependencyGraph())

	newDep := ServiceDependency{
		Service:      "test-service",
		Dependencies: []string{"test-dependency"},
		Reason:       "Test reason",
	}

	validator.AddDependency(newDep)

	graph := validator.GetDependencyGraph()
	if len(graph) != initialCount+1 {
		t.Errorf("AddDependency() did not add dependency, got %d dependencies, want %d", len(graph), initialCount+1)
	}

	// Verify the new dependency was added
	found := false
	for _, dep := range graph {
		if dep.Service == "test-service" {
			found = true
			if len(dep.Dependencies) != 1 || dep.Dependencies[0] != "test-dependency" {
				t.Error("AddDependency() did not add correct dependencies")
			}
			if dep.Reason != "Test reason" {
				t.Error("AddDependency() did not add correct reason")
			}
		}
	}

	if !found {
		t.Error("AddDependency() did not add the dependency to the graph")
	}
}

func TestIsServiceEnabled(t *testing.T) {
	tests := []struct {
		name        string
		services    map[string]any
		serviceName string
		want        bool
	}{
		{
			name: "service enabled",
			services: map[string]any{
				"test": &BaseConfig{Enabled: true},
			},
			serviceName: "test",
			want:        true,
		},
		{
			name: "service disabled",
			services: map[string]any{
				"test": &BaseConfig{Enabled: false},
			},
			serviceName: "test",
			want:        false,
		},
		{
			name: "service not in map",
			services: map[string]any{
				"other": &BaseConfig{Enabled: true},
			},
			serviceName: "test",
			want:        false,
		},
		{
			name: "service with embedded BaseConfig",
			services: map[string]any{
				"test": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: true},
				},
			},
			serviceName: "test",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isServiceEnabled(tt.services, tt.serviceName)
			if got != tt.want {
				t.Errorf("isServiceEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
