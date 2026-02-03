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

package examples

import (
	"context"
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
)

// SecurityValidatorExample demonstrates how security validators always run
// and cannot be bypassed.
func SecurityValidatorExample() {
	// Create a validation engine
	engine := validation.NewValidationEngine()

	// Register a security validator - it will always run
	securityValidator := validators.NewSecurityValidator()
	engine.MustRegisterSecurityValidator(securityValidator)

	// Register a regular validator
	clusterValidator := validators.NewClusterNameValidator()
	engine.MustRegister(clusterValidator)

	ctx := context.Background()

	// Example 1: Validate with malicious input
	fmt.Println("=== Example 1: Malicious Input ===")
	maliciousInput := map[string]interface{}{
		"type":  "shell-input",
		"value": "test; rm -rf /",
	}

	result, err := engine.Validate(ctx, "cluster-name", maliciousInput)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !result.Valid {
		fmt.Println("Validation failed (as expected):")
		for _, issue := range result.Errors {
			fmt.Printf("  - %s: %s\n", issue.Field, issue.Message)
		}
	}

	// Example 2: Validate with safe input
	fmt.Println("\n=== Example 2: Safe Input ===")
	safeInput := map[string]interface{}{
		"type":  "shell-input",
		"value": "my-cluster",
	}

	result, err = engine.Validate(ctx, "cluster-name", safeInput)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if result.Valid {
		fmt.Println("Validation passed!")
	} else {
		fmt.Println("Validation failed:")
		for _, issue := range result.Errors {
			fmt.Printf("  - %s: %s\n", issue.Field, issue.Message)
		}
	}

	// Example 3: Security validator runs even when not explicitly requested
	fmt.Println("\n=== Example 3: Security Validator Always Runs ===")
	fmt.Println("Requesting only 'cluster-name' validator, but security validator runs automatically")

	maliciousInput2 := map[string]interface{}{
		"type":  "shell-input",
		"value": "test | cat /etc/passwd",
	}

	result, err = engine.Validate(ctx, "cluster-name", maliciousInput2)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !result.Valid {
		fmt.Println("Security validator detected malicious input:")
		for _, issue := range result.Errors {
			fmt.Printf("  - %s: %s\n", issue.Field, issue.Message)
		}
	}

	// Example 4: List security validators
	fmt.Println("\n=== Example 4: List Security Validators ===")
	secValidators := engine.ListSecurityValidators()
	fmt.Printf("Registered security validators: %v\n", secValidators)
}
