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

// ValidationIssue represents a single validation issue.
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
func (r *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
	r.Info = append(r.Info, other.Info...)
	if !other.Valid {
		r.Valid = false
	}
}

// Validator defines the interface for all validators.
type Validator interface {
	// Name returns the unique name of the validator.
	Name() string
	// Validate performs validation on the given value.
	// The context can be used for cancellation and passing metadata.
	Validate(ctx context.Context, value interface{}) (*ValidationResult, error)
}

// ValidatorFunc is a function type that implements the Validator interface.
type ValidatorFunc struct {
	name string
	fn   func(ctx context.Context, value interface{}) (*ValidationResult, error)
}

// NewValidatorFunc creates a new ValidatorFunc.
func NewValidatorFunc(name string, fn func(ctx context.Context, value interface{}) (*ValidationResult, error)) *ValidatorFunc {
	return &ValidatorFunc{
		name: name,
		fn:   fn,
	}
}

// Name returns the validator name.
func (v *ValidatorFunc) Name() string {
	return v.name
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
