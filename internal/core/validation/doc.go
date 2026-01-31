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

// Package validation provides a unified validation system with pluggable validators.
//
// The validation package implements a flexible validation engine that supports:
//   - Pluggable validators through the Validator interface
//   - Thread-safe validator registration and lookup
//   - Context-aware validation with cancellation support
//   - Automatic suggestion generation for validation errors
//   - Parallel validation for independent validators
//
// # Architecture
//
// The package consists of four main components:
//
//  1. ValidationEngine: The main validation orchestrator
//  2. Registry: Thread-safe validator registration and lookup
//  3. SuggestionEngine: Automatic suggestion generation for errors
//  4. Validator interface: Contract for all validators
//
// # Basic Usage
//
// Create a validation engine and register validators:
//
//	engine := validation.NewValidationEngine()
//
//	// Register a custom validator
//	validator := validation.NewValidatorFunc("cluster-name", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
//	    name, ok := value.(string)
//	    if !ok {
//	        result := &validation.ValidationResult{Valid: false}
//	        result.AddError("cluster.name", "value must be a string")
//	        return result, nil
//	    }
//
//	    result := &validation.ValidationResult{Valid: true}
//	    if len(name) == 0 {
//	        result.AddError("cluster.name", "cluster name cannot be empty")
//	    }
//	    return result, nil
//	})
//
//	engine.Register(validator)
//
//	// Validate a value
//	result, err := engine.Validate(context.Background(), "cluster-name", "my-cluster")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if !result.Valid {
//	    for _, issue := range result.Errors {
//	        fmt.Printf("Error: %s - %s\n", issue.Field, issue.Message)
//	        for _, suggestion := range issue.Suggestions {
//	            fmt.Printf("  Suggestion: %s\n", suggestion)
//	        }
//	    }
//	}
//
// # Multiple Validators
//
// Validate using multiple validators sequentially:
//
//	result, err := engine.ValidateAll(ctx, []string{"cluster-name", "cluster-version"}, config)
//
// Or validate in parallel for independent validators:
//
//	result, err := engine.ValidateParallel(ctx, []string{"cluster-name", "cluster-version"}, config)
//
// # Validation Options
//
// Control validation behavior with options:
//
//	opts := &validation.ValidationOptions{
//	    StopOnFirstError: true,
//	    IncludeWarnings:  true,
//	    Context: map[string]interface{}{
//	        "valid_values": []string{"dev", "staging", "prod"},
//	    },
//	}
//
//	result, err := engine.ValidateWithOptions(ctx, "environment", "production", opts)
//
// # Suggestion Engine
//
// The suggestion engine automatically generates helpful suggestions for validation errors:
//
//   - Typo detection using Levenshtein distance
//   - Context-aware suggestions based on field names
//   - Custom suggestion rules
//
// Add custom suggestion rules:
//
//	type CustomRule struct{}
//
//	func (r *CustomRule) Name() string { return "custom" }
//
//	func (r *CustomRule) Generate(issue *validation.ValidationIssue, context map[string]interface{}) []string {
//	    // Generate custom suggestions
//	    return []string{"Custom suggestion"}
//	}
//
//	engine.AddSuggestionRule(&CustomRule{})
//
// # Global Registry
//
// Use the global registry for application-wide validators:
//
//	validation.MustRegister(myValidator)
//	result, err := validation.Validate(ctx, "my-validator", value)
//
// # Thread Safety
//
// All operations are thread-safe:
//   - Validator registration uses RWMutex
//   - Parallel validation uses goroutines safely
//   - Registry operations are protected
//
// # Performance
//
// Target performance characteristics:
//   - Validator lookup: <100μs
//   - Single validation: <100μs (validator-dependent)
//   - Parallel validation: ~1/N of sequential (N validators)
//   - Suggestion generation: <10μs per issue
//
// # Best Practices
//
//  1. Register validators during initialization
//  2. Use parallel validation for independent validators
//  3. Provide context for better suggestions
//  4. Use StopOnFirstError for fail-fast validation
//  5. Implement Validator interface for complex validation logic
//  6. Use ValidatorFunc for simple validation functions
//
// # Example: Complete Validation Flow
//
//	// Create engine
//	engine := validation.NewValidationEngine()
//
//	// Register validators
//	engine.MustRegister(clusterNameValidator)
//	engine.MustRegister(clusterVersionValidator)
//	engine.MustRegister(networkConfigValidator)
//
//	// Validate configuration
//	opts := validation.DefaultValidationOptions()
//	opts.Context["valid_providers"] = []string{"openstack", "aws", "gcp"}
//
//	result, err := engine.ValidateAllWithOptions(
//	    ctx,
//	    []string{"cluster-name", "cluster-version", "network-config"},
//	    config,
//	    opts,
//	)
//
//	if err != nil {
//	    return fmt.Errorf("validation failed: %w", err)
//	}
//
//	if !result.Valid {
//	    // Handle validation errors
//	    for _, issue := range result.Errors {
//	        fmt.Printf("[%s] %s: %s\n", issue.Severity, issue.Field, issue.Message)
//	        for _, suggestion := range issue.Suggestions {
//	            fmt.Printf("  → %s\n", suggestion)
//	        }
//	    }
//	    return fmt.Errorf("configuration validation failed")
//	}
//
//	// Handle warnings
//	for _, issue := range result.Warnings {
//	    fmt.Printf("Warning: %s - %s\n", issue.Field, issue.Message)
//	}
package validation
