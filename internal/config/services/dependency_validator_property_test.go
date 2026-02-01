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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: v2-cluster-config-schema, Property 14: Service Dependency Validation
// **Validates: Requirements 17.5**
//
// For any configuration with service dependencies (weave-gitops requires fluxcd,
// headlamp requires keycloak when OIDC is configured), the system must detect
// and report missing dependencies with clear error messages.
func TestProperty_ServiceDependencyValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property 1: Enabled services with missing dependencies are detected
	properties.Property("enabled services with missing dependencies are detected", prop.ForAll(
		func(enableWeaveGitOps, enableFluxCD, enableHeadlamp, enableKeycloak bool) bool {
			validator := NewDependencyValidator()

			// Build service map
			services := map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: enableWeaveGitOps},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: enableFluxCD},
				},
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: enableHeadlamp},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: enableKeycloak},
				},
			}

			errors := validator.ValidateDependencies(services)

			// Calculate expected errors
			expectedErrors := 0
			if enableWeaveGitOps && !enableFluxCD {
				expectedErrors++
			}
			if enableHeadlamp && !enableKeycloak {
				expectedErrors++
			}

			return len(errors) == expectedErrors
		},
		gen.Bool(),
		gen.Bool(),
		gen.Bool(),
		gen.Bool(),
	))

	// Property 2: Disabled services never produce dependency errors
	properties.Property("disabled services never produce dependency errors", prop.ForAll(
		func(enableFluxCD, enableKeycloak bool) bool {
			validator := NewDependencyValidator()

			// Build service map with dependent services disabled
			services := map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: enableFluxCD},
				},
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: enableKeycloak},
				},
			}

			errors := validator.ValidateDependencies(services)

			// Should have no errors since dependent services are disabled
			return len(errors) == 0
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property 3: All dependencies satisfied produces no errors
	properties.Property("all dependencies satisfied produces no errors", prop.ForAll(
		func(enableAll bool) bool {
			validator := NewDependencyValidator()

			// Build service map with all services having same enabled state
			services := map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: enableAll},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: enableAll},
				},
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: enableAll},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: enableAll},
				},
			}

			errors := validator.ValidateDependencies(services)

			// Should have no errors since dependencies match enabled state
			return len(errors) == 0
		},
		gen.Bool(),
	))

	// Property 4: Missing service in map is treated as disabled
	properties.Property("missing service in map is treated as disabled", prop.ForAll(
		func(enableWeaveGitOps, enableHeadlamp bool) bool {
			validator := NewDependencyValidator()

			// Build service map without dependency services
			services := map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: enableWeaveGitOps},
				},
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: enableHeadlamp},
				},
				// fluxcd and keycloak are missing
			}

			errors := validator.ValidateDependencies(services)

			// Calculate expected errors
			expectedErrors := 0
			if enableWeaveGitOps {
				expectedErrors++ // Missing fluxcd
			}
			if enableHeadlamp {
				expectedErrors++ // Missing keycloak
			}

			return len(errors) == expectedErrors
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property 5: Headlamp OIDC validation only triggers when OIDC is configured
	properties.Property("headlamp OIDC validation only triggers when OIDC is configured", prop.ForAll(
		func(enableHeadlamp, enableKeycloak bool, oidcIssuer, oidcClientID string) bool {
			validator := NewDependencyValidator()

			// Build service map
			services := map[string]any{
				"headlamp": &HeadlampConfig{
					BaseConfig:    BaseConfig{Enabled: enableHeadlamp},
					OIDCIssuerURL: oidcIssuer,
					OIDCClientID:  oidcClientID,
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: enableKeycloak},
				},
			}

			errors := validator.ValidateHeadlampOIDC(services)

			// Calculate expected errors
			hasOIDC := oidcIssuer != "" || oidcClientID != ""
			shouldError := enableHeadlamp && hasOIDC && !enableKeycloak

			if shouldError {
				return len(errors) > 0
			}
			return len(errors) == 0
		},
		gen.Bool(),
		gen.Bool(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property 6: Error messages contain service names
	properties.Property("error messages contain service names", prop.ForAll(
		func(enableWeaveGitOps, enableHeadlamp bool) bool {
			validator := NewDependencyValidator()

			// Build service map with dependencies missing
			services := map[string]any{
				"weave-gitops": &WeaveGitOpsConfig{
					BaseConfig: BaseConfig{Enabled: enableWeaveGitOps},
				},
				"fluxcd": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
				"headlamp": &HeadlampConfig{
					BaseConfig: BaseConfig{Enabled: enableHeadlamp},
				},
				"keycloak": &KeycloakConfig{
					BaseConfig: BaseConfig{Enabled: false},
				},
			}

			errors := validator.ValidateDependencies(services)

			// If no services are enabled, should have no errors
			if !enableWeaveGitOps && !enableHeadlamp {
				return len(errors) == 0
			}

			// If services are enabled, should have errors with service names
			if len(errors) == 0 {
				return false
			}

			// Check that at least one error message contains expected service names
			foundExpectedError := false
			for _, err := range errors {
				if enableWeaveGitOps {
					// Check for weave-gitops error
					if containsSubstring(err, "weave-gitops") && containsSubstring(err, "fluxcd") {
						foundExpectedError = true
					}
				}
				if enableHeadlamp {
					// Check for headlamp error
					if containsSubstring(err, "headlamp") && containsSubstring(err, "keycloak") {
						foundExpectedError = true
					}
				}
			}

			return foundExpectedError
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property 7: Adding new dependencies extends validation
	properties.Property("adding new dependencies extends validation", prop.ForAll(
		func(enableTestService, enableTestDep bool) bool {
			validator := NewDependencyValidator()

			// Add a new dependency
			validator.AddDependency(ServiceDependency{
				Service:      "test-service",
				Dependencies: []string{"test-dependency"},
				Reason:       "Test dependency",
			})

			// Build service map
			services := map[string]any{
				"test-service": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: enableTestService},
				},
				"test-dependency": &DefaultServiceConfig{
					BaseConfig: BaseConfig{Enabled: enableTestDep},
				},
			}

			errors := validator.ValidateDependencies(services)

			// Calculate expected errors
			expectedErrors := 0
			if enableTestService && !enableTestDep {
				expectedErrors++
			}

			return len(errors) == expectedErrors
		},
		gen.Bool(),
		gen.Bool(),
	))

	// Property 8: Dependency graph is queryable
	properties.Property("dependency graph is queryable and contains known dependencies", prop.ForAll(
		func() bool {
			validator := NewDependencyValidator()
			graph := validator.GetDependencyGraph()

			// Should have at least the known dependencies
			foundWeaveGitOps := false
			foundHeadlamp := false

			for _, dep := range graph {
				if dep.Service == "weave-gitops" {
					foundWeaveGitOps = true
					// Should have fluxcd as dependency
					hasFluxCD := false
					for _, d := range dep.Dependencies {
						if d == "fluxcd" {
							hasFluxCD = true
							break
						}
					}
					if !hasFluxCD {
						return false
					}
				}
				if dep.Service == "headlamp" {
					foundHeadlamp = true
					// Should have keycloak as dependency
					hasKeycloak := false
					for _, d := range dep.Dependencies {
						if d == "keycloak" {
							hasKeycloak = true
							break
						}
					}
					if !hasKeycloak {
						return false
					}
				}
			}

			return foundWeaveGitOps && foundHeadlamp
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
