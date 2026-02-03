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
)

// Severity represents the severity level of a validation issue.
type Severity string

const (
	// SeverityError indicates a validation error that must be fixed.
	SeverityError Severity = "error"
	// SeverityWarning indicates a validation warning that should be reviewed.
	SeverityWarning Severity = "warning"
	// SeverityInfo indicates informational validation feedback.
	SeverityInfo Severity = "info"
)

// Priority constants for validator execution ordering.
//
// Validators with lower priority values execute first. This allows fast,
// simple validators to run before slow, complex validators.
//
// Priority ranges:
//   - 0-99: High priority (fast validators like format checks)
//   - 100-199: Normal priority (standard business logic validators)
//   - 200-299: Low priority (slow validators like network checks, file I/O)
const (
	// PriorityHigh is for fast validators that should run first.
	// Examples: format checks, simple pattern matching, length validation
	PriorityHigh = 50

	// PriorityNormal is for standard validators with moderate complexity.
	// Examples: business logic validation, cross-field validation
	PriorityNormal = 100

	// PriorityLow is for slow validators that should run last.
	// Examples: network connectivity checks, file I/O, external API calls
	PriorityLow = 200
)

// ValidationIssue represents a single validation issue.
//
// Each issue contains:
//   - Severity: Error, warning, or info
//   - Field: Path to the field that failed (e.g., "cluster.name")
//   - Message: Human-readable description
//   - Code: Optional error code for programmatic handling
//   - Suggestions: Helpful suggestions for fixing the issue
//   - Context: Additional metadata about the failure
//
// Example:
//
//	issue := &validation.ValidationIssue{
//	    Severity: validation.SeverityError,
//	    Field: "cluster.name",
//	    Message: "name must be lowercase",
//	    Code: "INVALID_NAME_FORMAT",
//	    Suggestions: []string{
//	        "Use lowercase letters only",
//	        "Example: my-cluster",
//	    },
//	}
type ValidationIssue struct {
	// Severity indicates the severity level of the issue.
	Severity Severity
	// Field is the field path that failed validation (e.g., "cluster.name").
	Field string
	// Message is a human-readable description of the issue.
	Message string
	// Code is an optional error code for programmatic handling.
	Code string
	// Suggestions contains helpful suggestions for fixing the issue.
	Suggestions []string
	// Context contains additional context about the validation failure.
	Context map[string]interface{}
}

// Error implements the error interface for ValidationIssue.
func (v *ValidationIssue) Error() string {
	if v.Code != "" {
		return fmt.Sprintf("[%s] %s: %s", v.Code, v.Field, v.Message)
	}
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// ValidationResult represents the result of a validation operation.
//
// A validation result contains:
//   - Valid: Overall validation status (false if any errors)
//   - Errors: Critical issues that must be fixed
//   - Warnings: Issues that should be reviewed but don't block
//   - Info: Informational messages
//
// Example usage:
//
//	result := validation.NewValidationResult()
//	result.AddError("cluster.name", "name is required")
//	result.AddWarning("cluster.region", "region not specified, using default")
//
//	if !result.Valid {
//	    for _, err := range result.Errors {
//	        fmt.Printf("Error: %s\n", err.Message)
//	        for _, suggestion := range err.Suggestions {
//	            fmt.Printf("  Suggestion: %s\n", suggestion)
//	        }
//	    }
//	}
type ValidationResult struct {
	// Valid indicates whether the validation passed.
	Valid bool
	// Errors contains all validation errors.
	Errors []*ValidationIssue
	// Warnings contains all validation warnings.
	Warnings []*ValidationIssue
	// Info contains informational messages.
	Info []*ValidationIssue
}

// NewValidationResult creates a new ValidationResult with pre-allocated slices.
//
// Pre-allocates capacity for 4 errors, 4 warnings, and 4 info messages to
// reduce allocations during validation. The result is marked as valid by default.
//
// Example:
//
//	result := validation.NewValidationResult()
//	result.AddError("field", "error message")
//	return result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:    true,
		Errors:   make([]*ValidationIssue, 0, 4), // Pre-allocate for 4 errors
		Warnings: make([]*ValidationIssue, 0, 4), // Pre-allocate for 4 warnings
		Info:     make([]*ValidationIssue, 0, 4), // Pre-allocate for 4 info messages
	}
}

// HasErrors returns true if the result contains any errors.
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if the result contains any warnings.
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// HasIssues returns true if the result contains any errors or warnings.
func (r *ValidationResult) HasIssues() bool {
	return r.HasErrors() || r.HasWarnings()
}

// AddError adds an error to the validation result.
//
// Errors indicate critical issues that must be fixed before proceeding.
// Adding an error sets Valid to false.
//
// Parameters:
//   - field: Field path (e.g., "cluster.name")
//   - message: Human-readable error description
//   - suggestions: Optional suggestions for fixing the error
//
// Example:
//
//	result.AddError("cluster.name", "name is required")
//	result.AddError("cluster.region", "invalid region", "Use us-east-1 or us-west-2")
func (r *ValidationResult) AddError(field, message string, suggestions ...string) {
	r.Errors = append(r.Errors, &ValidationIssue{
		Severity:    SeverityError,
		Field:       field,
		Message:     message,
		Suggestions: suggestions,
	})
	r.Valid = false
}

// AddWarning adds a warning to the validation result.
func (r *ValidationResult) AddWarning(field, message string, suggestions ...string) {
	r.Warnings = append(r.Warnings, &ValidationIssue{
		Severity:    SeverityWarning,
		Field:       field,
		Message:     message,
		Suggestions: suggestions,
	})
}

// AddInfo adds an informational message to the validation result.
func (r *ValidationResult) AddInfo(field, message string) {
	r.Info = append(r.Info, &ValidationIssue{
		Severity: SeverityInfo,
		Field:    field,
		Message:  message,
	})
}

// Merge merges another ValidationResult into this one.
//
// This method combines errors, warnings, and info messages from another result.
// If the other result is invalid, this result becomes invalid.
//
// Pre-allocates capacity to avoid multiple reallocations when merging large results.
//
// Parameters:
//   - other: ValidationResult to merge (nil-safe)
//
// Example:
//
//	result1 := validator1.Validate(ctx, value)
//	result2 := validator2.Validate(ctx, value)
//	result1.Merge(result2) // Combine results
func (r *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}

	// Pre-allocate if needed to avoid multiple reallocations
	if len(other.Errors) > 0 {
		if cap(r.Errors)-len(r.Errors) < len(other.Errors) {
			newErrors := make([]*ValidationIssue, len(r.Errors), len(r.Errors)+len(other.Errors))
			copy(newErrors, r.Errors)
			r.Errors = newErrors
		}
		r.Errors = append(r.Errors, other.Errors...)
	}

	if len(other.Warnings) > 0 {
		if cap(r.Warnings)-len(r.Warnings) < len(other.Warnings) {
			newWarnings := make([]*ValidationIssue, len(r.Warnings), len(r.Warnings)+len(other.Warnings))
			copy(newWarnings, r.Warnings)
			r.Warnings = newWarnings
		}
		r.Warnings = append(r.Warnings, other.Warnings...)
	}

	if len(other.Info) > 0 {
		if cap(r.Info)-len(r.Info) < len(other.Info) {
			newInfo := make([]*ValidationIssue, len(r.Info), len(r.Info)+len(other.Info))
			copy(newInfo, r.Info)
			r.Info = newInfo
		}
		r.Info = append(r.Info, other.Info...)
	}

	if !other.Valid {
		r.Valid = false
	}
}

// Validator defines the interface for all validators.
//
// Validators must:
//   - Have a unique name (returned by Name())
//   - Implement Validate() to perform validation logic
//   - Be thread-safe (can be called concurrently)
//   - Return ValidationResult with errors, warnings, and suggestions
//   - Define a priority for execution ordering (lower values run first)
//
// Example implementation:
//
//	type MyValidator struct{}
//
//	func (v *MyValidator) Name() string {
//	    return "my-validator"
//	}
//
//	func (v *MyValidator) Priority() int {
//	    return PriorityNormal
//	}
//
//	func (v *MyValidator) Validate(ctx context.Context, value interface{}) (*ValidationResult, error) {
//	    result := validation.NewValidationResult()
//	    // Perform validation...
//	    if invalid {
//	        result.AddError("field", "error message", "suggestion")
//	    }
//	    return result, nil
//	}
type Validator interface {
	// Name returns the unique name of the validator.
	//
	// The name must be unique within a ValidationEngine and is used to
	// identify the validator when calling Validate() or ValidateAll().
	//
	// Convention: Use lowercase with hyphens (e.g., "cluster-name", "config-syntax")
	//
	// Returns:
	//   - string: Unique validator name
	Name() string

	// Priority returns the execution priority of the validator.
	//
	// Validators with lower priority values execute first. This allows fast,
	// simple validators (like format checks) to run before slow, complex
	// validators (like network checks or file I/O).
	//
	// Standard priority levels:
	//   - PriorityHigh (0-99): Fast validators (format checks, simple rules)
	//   - PriorityNormal (100-199): Standard validators (business logic)
	//   - PriorityLow (200-299): Slow validators (network, file I/O)
	//
	// Returns:
	//   - int: Priority value (lower values execute first)
	Priority() int

	// Validate performs validation on the given value.
	// The context can be used for cancellation and passing metadata.
	//
	// Implementations should:
	//   - Be thread-safe (can be called concurrently)
	//   - Respect context cancellation
	//   - Return ValidationResult with errors, warnings, and suggestions
	//   - Not panic (return errors instead)
	//
	// Parameters:
	//   - ctx: Context for cancellation and metadata
	//   - value: Value to validate (type depends on validator)
	//
	// Returns:
	//   - *ValidationResult: Validation result with errors/warnings
	//   - error: Execution error (not validation failure)
	Validate(ctx context.Context, value interface{}) (*ValidationResult, error)
}

// ValidatorFunc is a function type that implements the Validator interface.
type ValidatorFunc struct {
	name     string
	priority int
	fn       func(ctx context.Context, value interface{}) (*ValidationResult, error)
}

// NewValidatorFunc creates a new ValidatorFunc with normal priority.
func NewValidatorFunc(name string, fn func(ctx context.Context, value interface{}) (*ValidationResult, error)) *ValidatorFunc {
	return &ValidatorFunc{
		name:     name,
		priority: PriorityNormal,
		fn:       fn,
	}
}

// NewValidatorFuncWithPriority creates a new ValidatorFunc with custom priority.
func NewValidatorFuncWithPriority(name string, priority int, fn func(ctx context.Context, value interface{}) (*ValidationResult, error)) *ValidatorFunc {
	return &ValidatorFunc{
		name:     name,
		priority: priority,
		fn:       fn,
	}
}

// Name returns the validator name.
func (v *ValidatorFunc) Name() string {
	return v.name
}

// Priority returns the validator priority.
func (v *ValidatorFunc) Priority() int {
	return v.priority
}

// Validate executes the validation function.
func (v *ValidatorFunc) Validate(ctx context.Context, value interface{}) (*ValidationResult, error) {
	return v.fn(ctx, value)
}

// ValidationOptions contains options for validation operations.
type ValidationOptions struct {
	// StopOnFirstError stops validation after the first error.
	StopOnFirstError bool
	// IncludeWarnings includes warnings in the validation result.
	IncludeWarnings bool
	// Context contains additional context for validation.
	Context map[string]interface{}
}

// DefaultValidationOptions returns the default validation options.
func DefaultValidationOptions() *ValidationOptions {
	return &ValidationOptions{
		StopOnFirstError: false,
		IncludeWarnings:  true,
		Context:          make(map[string]interface{}),
	}
}

// ToError converts ValidationResult to a StructuredError.
//
// This method provides compatibility with the error handling system by converting
// validation results into structured errors that can be handled uniformly.
//
// Behavior:
//   - Returns nil if Valid is true (no errors)
//   - Returns nil if there are no errors (even if Valid is false)
//   - Returns a single StructuredError if there's one error
//   - Returns a StructuredError with aggregated messages if there are multiple errors
//
// The returned StructuredError includes:
//   - Type: ValidationError
//   - Message: Aggregated error messages (semicolon-separated)
//   - Suggestions: All suggestions from all errors
//   - Field: Field from first error (if only one error)
//   - Context: Combined context from all errors
//   - Retryable: false (validation errors require user action)
//
// Example:
//
//	result := validator.Validate(ctx, value)
//	if err := result.ToError(); err != nil {
//	    return fmt.Errorf("validation failed: %w", err)
//	}
//
// Returns:
//   - error: StructuredError if validation failed, nil otherwise
func (r *ValidationResult) ToError() error {
	// Return nil if validation passed
	if r.Valid {
		return nil
	}

	// Return nil if there are no errors (shouldn't happen if Valid is false, but be defensive)
	if len(r.Errors) == 0 {
		return nil
	}

	// Import the errors package type
	// Note: We need to import "github.com/rackerlabs/opencenter-cli/internal/util/errors"
	// to use StructuredError

	// Single error case - return it directly with proper structure
	if len(r.Errors) == 1 {
		err := r.Errors[0]
		return &structuredError{
			Type:        "validation",
			Field:       err.Field,
			Message:     err.Message,
			Suggestions: err.Suggestions,
			Context:     err.Context,
			Retryable:   false,
		}
	}

	// Multiple errors - aggregate them
	var messages []string
	var allSuggestions []string
	combinedContext := make(map[string]interface{})

	for _, err := range r.Errors {
		// Aggregate messages
		if err.Field != "" {
			messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
		} else {
			messages = append(messages, err.Message)
		}

		// Aggregate suggestions (deduplicate)
		allSuggestions = append(allSuggestions, err.Suggestions...)

		// Merge context
		if err.Context != nil {
			for k, v := range err.Context {
				combinedContext[k] = v
			}
		}
	}

	// Deduplicate suggestions
	uniqueSuggestions := make([]string, 0, len(allSuggestions))
	seen := make(map[string]bool)
	for _, s := range allSuggestions {
		if !seen[s] {
			seen[s] = true
			uniqueSuggestions = append(uniqueSuggestions, s)
		}
	}

	// Create aggregated error
	aggregatedMessage := ""
	if len(messages) > 0 {
		aggregatedMessage = messages[0]
		if len(messages) > 1 {
			for _, msg := range messages[1:] {
				aggregatedMessage += "; " + msg
			}
		}
	}

	return &structuredError{
		Type:        "validation",
		Message:     aggregatedMessage,
		Suggestions: uniqueSuggestions,
		Context:     combinedContext,
		Retryable:   false,
	}
}

// structuredError is a local implementation of StructuredError to avoid circular imports.
// This matches the structure in internal/util/errors/interfaces.go
type structuredError struct {
	Type        string
	Field       string
	Message     string
	Suggestions []string
	Context     map[string]interface{}
	Retryable   bool
}

// Error implements the error interface
func (e *structuredError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}
