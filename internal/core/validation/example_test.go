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

package validation_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// Example demonstrates basic usage of the validation engine.
func Example() {
	// Create a validation engine
	engine := validation.NewValidationEngine()

	// Register a cluster name validator
	clusterNameValidator := validation.NewValidatorFunc("cluster-name", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		name, ok := value.(string)
		if !ok {
			result := &validation.ValidationResult{Valid: false}
			result.AddError("cluster.name", "value must be a string")
			return result, nil
		}

		result := &validation.ValidationResult{Valid: true}

		if len(name) == 0 {
			result.AddError("cluster.name", "cluster name cannot be empty")
		} else if len(name) > 63 {
			result.AddError("cluster.name", "cluster name too long (max 63 characters)")
		} else if !isValidClusterName(name) {
			result.AddError("cluster.name", "cluster name must contain only alphanumeric characters and hyphens")
		}

		return result, nil
	})

	engine.MustRegister(clusterNameValidator)

	// Validate a cluster name
	result, err := engine.Validate(context.Background(), "cluster-name", "my-cluster")
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	if result.Valid {
		fmt.Println("Validation passed")
	} else {
		fmt.Println("Validation failed")
		for _, issue := range result.Errors {
			fmt.Printf("  Error: %s\n", issue.Message)
		}
	}

	// Output:
	// Validation passed
}

// ExampleValidationEngine_ValidateAll demonstrates validating with multiple validators.
func ExampleValidationEngine_ValidateAll() {
	engine := validation.NewValidationEngine()

	// Register validators
	nameValidator := validation.NewValidatorFunc("name", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		if value == "" {
			result.AddError("name", "name is required")
		}
		return result, nil
	})

	versionValidator := validation.NewValidatorFunc("version", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		result.AddWarning("version", "version should follow semantic versioning")
		return result, nil
	})

	engine.MustRegister(nameValidator)
	engine.MustRegister(versionValidator)

	// Validate with multiple validators
	result, _ := engine.ValidateAll(context.Background(), []string{"name", "version"}, "my-cluster")

	fmt.Printf("Valid: %v\n", result.Valid)
	fmt.Printf("Errors: %d\n", len(result.Errors))
	fmt.Printf("Warnings: %d\n", len(result.Warnings))

	// Output:
	// Valid: true
	// Errors: 0
	// Warnings: 1
}

// ExampleSuggestionEngine demonstrates suggestion generation.
func ExampleSuggestionEngine() {
	engine := validation.NewSuggestionEngine()

	// Create a validation issue
	issue := &validation.ValidationIssue{
		Severity: validation.SeverityError,
		Field:    "provider",
		Message:  "invalid value 'openstck'",
	}

	// Provide context with valid values
	context := map[string]interface{}{
		"valid_values": []string{"openstack", "aws", "gcp"},
	}

	// Enhance the issue with suggestions
	result := &validation.ValidationResult{
		Valid:  false,
		Errors: []*validation.ValidationIssue{issue},
	}

	engine.EnhanceResult(result, context)

	// Print suggestions
	for _, suggestion := range result.Errors[0].Suggestions {
		if strings.Contains(suggestion, "openstack") {
			fmt.Println("Found suggestion for openstack")
		}
	}

	// Output:
	// Found suggestion for openstack
}

// isValidClusterName checks if a cluster name is valid.
func isValidClusterName(name string) bool {
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}
