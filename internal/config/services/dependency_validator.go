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
	"fmt"
)

// ServiceDependency represents a dependency relationship between services
type ServiceDependency struct {
	Service      string   // The service that has dependencies
	Dependencies []string // Services that must be enabled for this service to work
	Reason       string   // Human-readable explanation of why the dependency exists
}

// serviceDependencyGraph defines the dependency relationships between services
var serviceDependencyGraph = []ServiceDependency{
	{
		Service:      "weave-gitops",
		Dependencies: []string{"fluxcd"},
		Reason:       "Weave GitOps requires FluxCD to be enabled for GitOps functionality",
	},
	{
		Service:      "headlamp",
		Dependencies: []string{"keycloak"},
		Reason:       "Headlamp requires Keycloak for OIDC authentication when OIDC is configured",
	},
}

// DependencyValidator validates service dependencies
type DependencyValidator struct {
	dependencies []ServiceDependency
}

// NewDependencyValidator creates a new service dependency validator
func NewDependencyValidator() *DependencyValidator {
	return &DependencyValidator{
		dependencies: serviceDependencyGraph,
	}
}

// ValidateDependencies checks if all service dependencies are satisfied
// Returns a list of error messages for missing dependencies
func (v *DependencyValidator) ValidateDependencies(services map[string]any) []string {
	var errors []string

	// Check each dependency rule
	for _, dep := range v.dependencies {
		// Check if the service is enabled
		if !isServiceEnabled(services, dep.Service) {
			continue // Service is not enabled, no need to check dependencies
		}

		// Check if all dependencies are enabled
		for _, requiredService := range dep.Dependencies {
			if !isServiceEnabled(services, requiredService) {
				errors = append(errors, fmt.Sprintf(
					"service '%s' requires '%s' to be enabled: %s",
					dep.Service,
					requiredService,
					dep.Reason,
				))
			}
		}
	}

	return errors
}

// ValidateHeadlampOIDC performs special validation for Headlamp OIDC configuration
// Headlamp only requires Keycloak when OIDC is actually configured
func (v *DependencyValidator) ValidateHeadlampOIDC(services map[string]any) []string {
	var errors []string

	// Check if headlamp is enabled
	if !isServiceEnabled(services, "headlamp") {
		return errors
	}

	// Get headlamp configuration
	headlampConfig, ok := services["headlamp"]
	if !ok {
		return errors
	}

	// Check if OIDC is configured (has issuer URL or client ID)
	hasOIDC := false
	if config, ok := headlampConfig.(*HeadlampConfig); ok {
		hasOIDC = config.OIDCIssuerURL != "" || config.OIDCClientID != ""
	}

	// If OIDC is configured, ensure keycloak is enabled
	if hasOIDC && !isServiceEnabled(services, "keycloak") {
		errors = append(errors, fmt.Sprintf(
			"service 'headlamp' has OIDC configured but requires 'keycloak' to be enabled for authentication",
		))
	}

	return errors
}

// isServiceEnabled checks if a service is enabled in the service map
func isServiceEnabled(services map[string]any, serviceName string) bool {
	service, exists := services[serviceName]
	if !exists {
		return false
	}

	// Try to get enabled status from BaseConfig
	switch s := service.(type) {
	case *BaseConfig:
		return s.Enabled
	case BaseConfig:
		return s.Enabled
	default:
		// Try to use reflection to get Enabled field
		// All service configs embed BaseConfig, so they should have Enabled field
		if enabler, ok := service.(interface{ IsEnabled() bool }); ok {
			return enabler.IsEnabled()
		}
	}

	return false
}

// GetDependencyGraph returns the service dependency graph for inspection
func (v *DependencyValidator) GetDependencyGraph() []ServiceDependency {
	return v.dependencies
}

// AddDependency adds a new dependency rule to the validator
// This allows for dynamic dependency registration
func (v *DependencyValidator) AddDependency(dep ServiceDependency) {
	v.dependencies = append(v.dependencies, dep)
}
