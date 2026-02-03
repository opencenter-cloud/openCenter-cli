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

package validators_test

import (
	"context"
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
)

// Example_serviceValidator demonstrates basic service validation.
func Example_serviceValidator() {
	// Create a service validator for Loki
	validator := validators.NewServiceValidator("loki")

	// Validate a service configuration
	config := map[string]interface{}{
		"enabled":   true,
		"namespace": "loki-system",
		"name":      "loki",
	}

	result, err := validator.Validate(context.Background(), config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if result.Valid {
		fmt.Println("Service configuration is valid")
	} else {
		fmt.Println("Service configuration is invalid")
		for _, e := range result.Errors {
			fmt.Printf("Error: %s - %s\n", e.Field, e.Message)
		}
	}

	// Output:
	// Service configuration is valid
}

// Example_serviceValidator_invalidNamespace demonstrates namespace validation errors.
func Example_serviceValidator_invalidNamespace() {
	validator := validators.NewServiceValidator("loki")

	// Invalid namespace with uppercase letters
	config := map[string]interface{}{
		"enabled":   true,
		"namespace": "Loki-System",
		"name":      "loki",
	}

	result, _ := validator.Validate(context.Background(), config)

	if !result.Valid {
		for _, e := range result.Errors {
			fmt.Printf("%s: %s\n", e.Field, e.Message)
			if len(e.Suggestions) > 0 {
				fmt.Printf("Suggestion: %s\n", e.Suggestions[0])
			}
		}
	}

	// Output:
	// service.namespace: invalid namespace format: Loki-System
	// Suggestion: Convert namespace to lowercase
}

// Example_serviceValidator_withExtension demonstrates using an extension validator.
func Example_serviceValidator_withExtension() {
	// Create base service validator
	validator := validators.NewServiceValidator("loki")

	// Create an extension validator for Loki-specific validation
	lokiValidator := validation.NewValidatorFunc("loki-storage", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := validation.NewValidationResult()

		// Check for Loki-specific fields
		if m, ok := value.(map[string]interface{}); ok {
			if storageType, ok := m["storage_type"].(string); ok {
				if storageType != "s3" && storageType != "swift" {
					result.AddError("storage_type",
						fmt.Sprintf("invalid storage type: %s", storageType),
						"Use 's3' or 'swift' for Loki storage")
				}
			}
		}

		return result, nil
	})

	// Set the extension validator
	validator.SetExtensionValidator(lokiValidator)

	// Validate with extension
	config := map[string]interface{}{
		"enabled":      true,
		"namespace":    "loki-system",
		"name":         "loki",
		"storage_type": "invalid",
	}

	result, _ := validator.Validate(context.Background(), config)

	if !result.Valid {
		for _, e := range result.Errors {
			fmt.Printf("%s: %s\n", e.Field, e.Message)
		}
	}

	// Output:
	// storage_type: invalid storage type: invalid
}

// Example_serviceValidator_namingConvention demonstrates the naming convention.
func Example_serviceValidator_namingConvention() {
	// Service validators follow the "service:{service_name}" convention
	lokiValidator := validators.NewServiceValidator("loki")
	prometheusValidator := validators.NewServiceValidator("prometheus")
	certManagerValidator := validators.NewServiceValidator("cert-manager")

	fmt.Printf("Loki validator name: %s\n", lokiValidator.Name())
	fmt.Printf("Prometheus validator name: %s\n", prometheusValidator.Name())
	fmt.Printf("Cert-manager validator name: %s\n", certManagerValidator.Name())

	// Output:
	// Loki validator name: service:loki
	// Prometheus validator name: service:prometheus
	// Cert-manager validator name: service:cert-manager
}

// Example_serviceValidator_baseServiceConfig demonstrates using BaseServiceConfig.
func Example_serviceValidator_baseServiceConfig() {
	validator := validators.NewServiceValidator("prometheus")

	// Use BaseServiceConfig struct
	config := &validators.BaseServiceConfig{
		Enabled:   true,
		Namespace: "prometheus-system",
		Name:      "prometheus",
	}

	result, _ := validator.Validate(context.Background(), config)

	if result.Valid {
		fmt.Println("Configuration is valid")
	}

	// Output:
	// Configuration is valid
}

// Example_serviceValidator_disabledService demonstrates validation of disabled services.
func Example_serviceValidator_disabledService() {
	validator := validators.NewServiceValidator("loki")

	config := map[string]interface{}{
		"enabled":   false,
		"namespace": "loki-system",
		"name":      "loki",
	}

	result, _ := validator.Validate(context.Background(), config)

	// Disabled services are still valid
	fmt.Printf("Valid: %v\n", result.Valid)

	// But they have info messages
	if len(result.Info) > 0 {
		fmt.Printf("Info: %s\n", result.Info[0].Message)
	}

	// Output:
	// Valid: true
	// Info: service 'loki' is disabled
}

// Example_serviceValidator_suggestions demonstrates error suggestions.
func Example_serviceValidator_suggestions() {
	validator := validators.NewServiceValidator("loki")

	// Configuration with multiple errors
	config := map[string]interface{}{
		"enabled":   true,
		"namespace": "loki_system", // Underscore instead of hyphen
		"name":      "",            // Empty name
	}

	result, _ := validator.Validate(context.Background(), config)

	if !result.Valid {
		for _, e := range result.Errors {
			fmt.Printf("Error: %s\n", e.Message)
			fmt.Println("Suggestions:")
			for _, s := range e.Suggestions {
				fmt.Printf("  - %s\n", s)
			}
			fmt.Println()
		}
	}

	// Output:
	// Error: invalid namespace format: loki_system
	// Suggestions:
	//   - Replace underscores with hyphens
	//   - Namespace must contain only lowercase letters, numbers, and hyphens
	//   - Namespace must start and end with alphanumeric character
	//
	// Error: service name is required
	// Suggestions:
	//   - Provide a name for the service
	//   - Example: name: loki
	//
}
