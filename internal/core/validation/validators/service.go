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
	"fmt"
	"regexp"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// ServiceConfig represents the common service configuration fields.
// This is a minimal interface that service configurations must satisfy.
type ServiceConfig interface {
	GetEnabled() bool
	GetNamespace() string
	GetName() string
}

// ServiceValidator validates service configurations.
//
// The validator checks:
//   - Common service fields (name, namespace, enabled)
//   - Service-specific validation via extension validators
//   - Naming conventions and format requirements
//
// Naming Convention:
//   - Base validator: "service"
//   - Service-specific: "service:{service_name}" (e.g., "service:loki", "service:prometheus")
//
// Example usage:
//
//	validator := validators.NewServiceValidator("loki")
//	result, err := validator.Validate(ctx, serviceConfig)
type ServiceValidator struct {
	serviceName       string
	extensionValidator validation.Validator
	namespacePattern  *regexp.Regexp
	namePattern       *regexp.Regexp
}

// NewServiceValidator creates a new service validator.
//
// Parameters:
//   - serviceName: Name of the service (e.g., "loki", "prometheus", "cert-manager")
//
// Returns:
//   - *ServiceValidator: New service validator instance
//
// Example:
//
//	validator := validators.NewServiceValidator("loki")
func NewServiceValidator(serviceName string) *ServiceValidator {
	return &ServiceValidator{
		serviceName: serviceName,
		// Kubernetes namespace naming rules: lowercase alphanumeric and hyphens, max 63 chars
		namespacePattern: regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`),
		// Service name pattern: lowercase alphanumeric, hyphens, and underscores
		namePattern: regexp.MustCompile(`^[a-z0-9]([-_a-z0-9]*[a-z0-9])?$`),
	}
}

// Name returns the validator name following the "service:{service_name}" convention.
//
// Returns:
//   - string: Validator name (e.g., "service:loki", "service:prometheus")
func (v *ServiceValidator) Name() string {
	if v.serviceName == "" {
		return "service"
	}
	return fmt.Sprintf("service:%s", v.serviceName)
}

// Priority returns the validator priority.
// Service validation involves business logic checks, so it has normal priority.
func (v *ServiceValidator) Priority() int {
	return validation.PriorityNormal
}

// SetExtensionValidator sets a service-specific extension validator.
//
// Extension validators can provide additional validation logic specific to
// a particular service type (e.g., Loki storage validation, Prometheus retention).
//
// Parameters:
//   - validator: Service-specific validator (nil to remove)
//
// Example:
//
//	lokiValidator := NewLokiStorageValidator()
//	serviceValidator.SetExtensionValidator(lokiValidator)
func (v *ServiceValidator) SetExtensionValidator(validator validation.Validator) {
	v.extensionValidator = validator
}

// Validate validates a service configuration.
//
// The value should be one of:
//   - A map with "enabled", "namespace", and "name" keys
//   - A struct implementing ServiceConfig interface
//   - A struct with Enabled, Namespace, and Name fields
//
// Parameters:
//   - ctx: Context for cancellation and metadata
//   - value: Service configuration to validate
//
// Returns:
//   - *ValidationResult: Validation result with errors/warnings
//   - error: Execution error (not validation failure)
//
// Example:
//
//	config := map[string]interface{}{
//	    "enabled": true,
//	    "namespace": "loki-system",
//	    "name": "loki",
//	}
//	result, err := validator.Validate(ctx, config)
func (v *ServiceValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := validation.NewValidationResult()

	// Extract service configuration fields
	enabled, namespace, name, err := v.extractServiceFields(value)
	if err != nil {
		result.AddError("service", err.Error(),
			"Provide a valid service configuration",
			"Service configuration must have 'enabled', 'namespace', and 'name' fields")
		return result, nil
	}

	// Validate common service fields
	v.validateEnabled(result, enabled)
	v.validateNamespace(result, namespace)
	v.validateName(result, name)

	// Run extension validator if configured
	if v.extensionValidator != nil {
		extensionResult, err := v.extensionValidator.Validate(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("extension validator failed: %w", err)
		}
		result.Merge(extensionResult)
	}

	return result, nil
}

// extractServiceFields extracts common service fields from various input types.
func (v *ServiceValidator) extractServiceFields(value interface{}) (enabled bool, namespace, name string, err error) {
	// Try ServiceConfig interface first
	if sc, ok := value.(ServiceConfig); ok {
		return sc.GetEnabled(), sc.GetNamespace(), sc.GetName(), nil
	}

	// Try map[string]interface{}
	if m, ok := value.(map[string]interface{}); ok {
		enabled, _ = m["enabled"].(bool)
		namespace, _ = m["namespace"].(string)
		name, _ = m["name"].(string)
		return enabled, namespace, name, nil
	}

	// Try struct with reflection-like field access
	// This handles structs with Enabled, Namespace, Name fields
	switch v := value.(type) {
	case struct {
		Enabled   bool
		Namespace string
		Name      string
	}:
		return v.Enabled, v.Namespace, v.Name, nil
	}

	return false, "", "", fmt.Errorf("invalid service configuration type: %T", value)
}

// validateEnabled validates the enabled field.
func (v *ServiceValidator) validateEnabled(result *validation.ValidationResult, enabled bool) {
	// Enabled is a boolean, so it's always valid
	// We just provide info about the service state
	if !enabled {
		result.AddInfo("service.enabled",
			fmt.Sprintf("service '%s' is disabled", v.serviceName))
	}
}

// validateNamespace validates the namespace field.
func (v *ServiceValidator) validateNamespace(result *validation.ValidationResult, namespace string) {
	if namespace == "" {
		result.AddWarning("service.namespace",
			"namespace not specified, will use default",
			"Specify a namespace for better organization",
			fmt.Sprintf("Example: namespace: %s-system", v.serviceName))
		return
	}

	// Check length (Kubernetes limit is 63 characters)
	if len(namespace) > 63 {
		result.AddError("service.namespace",
			fmt.Sprintf("namespace too long: %d characters (max 63)", len(namespace)),
			"Shorten the namespace name to 63 characters or less",
			"Use abbreviations or shorter identifiers")
		return
	}

	// Check pattern (lowercase alphanumeric and hyphens)
	if !v.namespacePattern.MatchString(namespace) {
		suggestions := []string{
			"Namespace must contain only lowercase letters, numbers, and hyphens",
			"Namespace must start and end with alphanumeric character",
		}

		if strings.ToLower(namespace) != namespace {
			suggestions = append([]string{"Convert namespace to lowercase"}, suggestions...)
		}
		if strings.HasPrefix(namespace, "-") || strings.HasSuffix(namespace, "-") {
			suggestions = append([]string{"Remove leading or trailing hyphens"}, suggestions...)
		}
		if strings.Contains(namespace, "_") {
			suggestions = append([]string{"Replace underscores with hyphens"}, suggestions...)
		}

		result.AddError("service.namespace",
			fmt.Sprintf("invalid namespace format: %s", namespace),
			suggestions...)
	}
}

// validateName validates the service name field.
func (v *ServiceValidator) validateName(result *validation.ValidationResult, name string) {
	if name == "" {
		result.AddError("service.name",
			"service name is required",
			"Provide a name for the service",
			fmt.Sprintf("Example: name: %s", v.serviceName))
		return
	}

	// Check length (reasonable limit for service names)
	if len(name) > 63 {
		result.AddError("service.name",
			fmt.Sprintf("service name too long: %d characters (max 63)", len(name)),
			"Shorten the service name to 63 characters or less",
			"Use abbreviations or shorter identifiers")
		return
	}

	// Check pattern (lowercase alphanumeric, hyphens, and underscores)
	if !v.namePattern.MatchString(name) {
		suggestions := []string{
			"Service name must contain only lowercase letters, numbers, hyphens, and underscores",
			"Service name must start and end with alphanumeric character",
		}

		if strings.ToLower(name) != name {
			suggestions = append([]string{"Convert name to lowercase"}, suggestions...)
		}
		if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
			suggestions = append([]string{"Remove leading or trailing hyphens"}, suggestions...)
		}

		result.AddError("service.name",
			fmt.Sprintf("invalid service name format: %s", name),
			suggestions...)
	}

	// Warn if name doesn't match service type
	if v.serviceName != "" && name != v.serviceName && !strings.Contains(name, v.serviceName) {
		result.AddWarning("service.name",
			fmt.Sprintf("service name '%s' doesn't match service type '%s'", name, v.serviceName),
			fmt.Sprintf("Consider using '%s' as the service name for consistency", v.serviceName))
	}
}

// BaseServiceConfig is a basic implementation of ServiceConfig for testing.
type BaseServiceConfig struct {
	Enabled   bool
	Namespace string
	Name      string
}

// GetEnabled returns the enabled status.
func (c *BaseServiceConfig) GetEnabled() bool {
	return c.Enabled
}

// GetNamespace returns the namespace.
func (c *BaseServiceConfig) GetNamespace() string {
	return c.Namespace
}

// GetName returns the service name.
func (c *BaseServiceConfig) GetName() string {
	return c.Name
}
