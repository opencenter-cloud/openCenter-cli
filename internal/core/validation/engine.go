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

package validation

import (
	"context"
	"fmt"
	"sync"
)

// ValidationEngine provides a unified validation system with pluggable validators.
//
// The engine manages a registry of validators and provides methods to execute
// them individually, sequentially, or in parallel. It also integrates a
// suggestion engine that enhances validation results with helpful suggestions.
//
// Thread Safety:
//
// ValidationEngine is thread-safe and can be used concurrently from multiple
// goroutines. All public methods use appropriate locking.
//
// Example usage:
//
//	engine := validation.NewValidationEngine()
//	engine.Register(validators.NewClusterNameValidator())
//
//	result, err := engine.Validate(ctx, "cluster-name", "my-cluster")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if !result.Valid {
//	    for _, err := range result.Errors {
//	        fmt.Printf("Error: %s\n", err.Message)
//	    }
//	}
//
// For multiple validators:
//
//	result, err := engine.ValidateAll(ctx, []string{"cluster-name", "config"}, value)
//
// For parallel execution:
//
//	result, err := engine.ValidateParallel(ctx, []string{"validator1", "validator2"}, value)
type ValidationEngine struct {
	registry         *Registry
	suggestionEngine *SuggestionEngine
	mu               sync.RWMutex
}

// NewValidationEngine creates a new validation engine.
//
// The engine is created with an empty registry and a default suggestion engine.
// Validators must be registered before use.
//
// Example:
//
//	engine := validation.NewValidationEngine()
//	engine.Register(validators.NewClusterNameValidator())
//	engine.Register(validators.NewConfigValidator())
func NewValidationEngine() *ValidationEngine {
	return &ValidationEngine{
		registry:         NewRegistry(),
		suggestionEngine: NewSuggestionEngine(),
	}
}

// Register registers a validator with the engine.
//
// The validator's Name() must be unique. Attempting to register a validator
// with a duplicate name returns an error.
//
// Parameters:
//   - validator: The validator to register
//
// Returns:
//   - error: Registration failure (duplicate name)
//
// Example:
//
//	err := engine.Register(validators.NewClusterNameValidator())
//	if err != nil {
//	    return fmt.Errorf("failed to register validator: %w", err)
//	}
func (e *ValidationEngine) Register(validator Validator) error {
	return e.registry.Register(validator)
}

// MustRegister registers a validator and panics if registration fails.
func (e *ValidationEngine) MustRegister(validator Validator) {
	e.registry.MustRegister(validator)
}

// Unregister removes a validator from the engine.
func (e *ValidationEngine) Unregister(name string) error {
	return e.registry.Unregister(name)
}

// Has checks if a validator is registered.
func (e *ValidationEngine) Has(name string) bool {
	return e.registry.Has(name)
}

// List returns all registered validator names.
func (e *ValidationEngine) List() []string {
	return e.registry.List()
}

// Validate validates a value using a specific validator.
//
// This method:
//  1. Looks up the validator by name
//  2. Executes the validator
//  3. Enhances the result with suggestions
//  4. Returns the validation result
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - validatorName: Name of the registered validator
//   - value: Value to validate (type depends on validator)
//
// Returns:
//   - *ValidationResult: Validation result with errors, warnings, and suggestions
//   - error: Validator not found or validation execution error
//
// Example:
//
//	result, err := engine.Validate(ctx, "cluster-name", "my-cluster")
//	if err != nil {
//	    return fmt.Errorf("validation failed: %w", err)
//	}
//	if !result.Valid {
//	    fmt.Println("Validation errors:")
//	    for _, err := range result.Errors {
//	        fmt.Printf("  - %s: %s\n", err.Field, err.Message)
//	    }
//	}
func (e *ValidationEngine) Validate(ctx context.Context, validatorName string, value interface{}) (*ValidationResult, error) {
	return e.ValidateWithOptions(ctx, validatorName, value, DefaultValidationOptions())
}

// ValidateWithOptions validates a value using a specific validator with options.
func (e *ValidationEngine) ValidateWithOptions(ctx context.Context, validatorName string, value interface{}, opts *ValidationOptions) (*ValidationResult, error) {
	validator := e.registry.Get(validatorName)
	if validator == nil {
		return nil, fmt.Errorf("validator %q not found", validatorName)
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Enhance result with suggestions
	if opts.Context == nil {
		opts.Context = make(map[string]interface{})
	}
	e.suggestionEngine.EnhanceResult(result, opts.Context)

	// Filter warnings if not included
	if !opts.IncludeWarnings {
		result.Warnings = nil
	}

	return result, nil
}

// ValidateAll validates a value using multiple validators.
// Validation stops at the first error if StopOnFirstError is true in options.
//
// This method executes validators sequentially in the order provided.
// Use ValidateParallel for concurrent execution of independent validators.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - validatorNames: Names of validators to execute
//   - value: Value to validate
//
// Returns:
//   - *ValidationResult: Merged validation results from all validators
//   - error: Validator not found or execution error
//
// Example:
//
//	validators := []string{"cluster-name", "config", "security"}
//	result, err := engine.ValidateAll(ctx, validators, config)
//	if err != nil {
//	    return fmt.Errorf("validation failed: %w", err)
//	}
func (e *ValidationEngine) ValidateAll(ctx context.Context, validatorNames []string, value interface{}) (*ValidationResult, error) {
	return e.ValidateAllWithOptions(ctx, validatorNames, value, DefaultValidationOptions())
}

// ValidateAllWithOptions validates a value using multiple validators with options.
func (e *ValidationEngine) ValidateAllWithOptions(ctx context.Context, validatorNames []string, value interface{}, opts *ValidationOptions) (*ValidationResult, error) {
	result := NewValidationResult()

	for _, name := range validatorNames {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		validator := e.registry.Get(name)
		if validator == nil {
			result.AddError("validator", fmt.Sprintf("validator %q not found", name))
			if opts.StopOnFirstError {
				break
			}
			continue
		}

		validationResult, err := validator.Validate(ctx, value)
		if err != nil {
			result.AddError(name, fmt.Sprintf("validation error: %v", err))
			if opts.StopOnFirstError {
				break
			}
			continue
		}

		// Merge results
		result.Merge(validationResult)

		// Stop on first error if requested
		if opts.StopOnFirstError && result.HasErrors() {
			break
		}
	}

	// Enhance result with suggestions
	if opts.Context == nil {
		opts.Context = make(map[string]interface{})
	}
	e.suggestionEngine.EnhanceResult(result, opts.Context)

	// Filter warnings if not included
	if !opts.IncludeWarnings {
		result.Warnings = nil
	}

	return result, nil
}

// ValidateParallel validates a value using multiple validators in parallel.
// This is useful for independent validators that can run concurrently.
//
// All validators execute concurrently using goroutines. Results are collected
// and merged into a single ValidationResult. This method is faster than
// ValidateAll for independent validators but uses more resources.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - validatorNames: Names of validators to execute concurrently
//   - value: Value to validate
//
// Returns:
//   - *ValidationResult: Merged validation results from all validators
//   - error: Validator not found or execution error
//
// Example:
//
//	// These validators are independent and can run in parallel
//	validators := []string{"syntax", "schema", "security"}
//	result, err := engine.ValidateParallel(ctx, validators, config)
//	if err != nil {
//	    return fmt.Errorf("validation failed: %w", err)
//	}
func (e *ValidationEngine) ValidateParallel(ctx context.Context, validatorNames []string, value interface{}) (*ValidationResult, error) {
	return e.ValidateParallelWithOptions(ctx, validatorNames, value, DefaultValidationOptions())
}

// ValidateParallelWithOptions validates a value using multiple validators in parallel with options.
func (e *ValidationEngine) ValidateParallelWithOptions(ctx context.Context, validatorNames []string, value interface{}, opts *ValidationOptions) (*ValidationResult, error) {
	result := NewValidationResult()

	// Create channels for results
	type validationJob struct {
		name   string
		result *ValidationResult
		err    error
	}

	resultChan := make(chan validationJob, len(validatorNames))
	var wg sync.WaitGroup

	// Start validation goroutines
	for _, name := range validatorNames {
		wg.Add(1)
		go func(validatorName string) {
			defer wg.Done()

			validator := e.registry.Get(validatorName)
			if validator == nil {
				resultChan <- validationJob{
					name: validatorName,
					err:  fmt.Errorf("validator %q not found", validatorName),
				}
				return
			}

			validationResult, err := validator.Validate(ctx, value)
			resultChan <- validationJob{
				name:   validatorName,
				result: validationResult,
				err:    err,
			}
		}(name)
	}

	// Wait for all validations to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for job := range resultChan {
		if job.err != nil {
			result.AddError(job.name, fmt.Sprintf("validation error: %v", job.err))
			continue
		}

		if job.result != nil {
			result.Merge(job.result)
		}
	}

	// Enhance result with suggestions
	if opts.Context == nil {
		opts.Context = make(map[string]interface{})
	}
	e.suggestionEngine.EnhanceResult(result, opts.Context)

	// Filter warnings if not included
	if !opts.IncludeWarnings {
		result.Warnings = nil
	}

	return result, nil
}

// AddSuggestionRule adds a custom suggestion rule to the engine.
func (e *ValidationEngine) AddSuggestionRule(rule SuggestionRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.suggestionEngine.AddRule(rule)
}

// GetRegistry returns the validator registry.
func (e *ValidationEngine) GetRegistry() *Registry {
	return e.registry
}

// GetSuggestionEngine returns the suggestion engine.
func (e *ValidationEngine) GetSuggestionEngine() *SuggestionEngine {
	return e.suggestionEngine
}

// defaultEngine is the default global validation engine.
var defaultEngine = NewValidationEngine()

// DefaultEngine returns the default global validation engine.
func DefaultEngine() *ValidationEngine {
	return defaultEngine
}

// Validate validates a value using the default engine.
func Validate(ctx context.Context, validatorName string, value interface{}) (*ValidationResult, error) {
	return defaultEngine.Validate(ctx, validatorName, value)
}

// ValidateAll validates a value using multiple validators with the default engine.
func ValidateAll(ctx context.Context, validatorNames []string, value interface{}) (*ValidationResult, error) {
	return defaultEngine.ValidateAll(ctx, validatorNames, value)
}

// ValidateParallel validates a value using multiple validators in parallel with the default engine.
func ValidateParallel(ctx context.Context, validatorNames []string, value interface{}) (*ValidationResult, error) {
	return defaultEngine.ValidateParallel(ctx, validatorNames, value)
}
