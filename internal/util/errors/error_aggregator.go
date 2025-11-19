/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errors

import (
	"fmt"
	"strings"
)

// DefaultErrorAggregator implements ErrorAggregator interface
type DefaultErrorAggregator struct {
	errors  []error
	handler ErrorHandler
}

// NewDefaultErrorAggregator creates a new default error aggregator
func NewDefaultErrorAggregator() *DefaultErrorAggregator {
	return &DefaultErrorAggregator{
		errors:  make([]error, 0),
		handler: NewDefaultErrorHandler(),
	}
}

// AddError adds an error to the aggregator
func (a *DefaultErrorAggregator) AddError(err error) {
	if err != nil {
		a.errors = append(a.errors, err)
	}
}

// AddErrorWithContext adds an error with field context to the aggregator
func (a *DefaultErrorAggregator) AddErrorWithContext(field string, err error) {
	if err != nil {
		wrappedErr := WrapWithField(err, field)
		a.errors = append(a.errors, wrappedErr)
	}
}

// HasErrors returns true if there are any errors in the aggregator
func (a *DefaultErrorAggregator) HasErrors() bool {
	return len(a.errors) > 0
}

// GetErrors returns all errors in the aggregator
func (a *DefaultErrorAggregator) GetErrors() []error {
	return a.errors
}

// GetErrorsByType returns errors of a specific type
func (a *DefaultErrorAggregator) GetErrorsByType(errorType ErrorType) []error {
	var filteredErrors []error

	for _, err := range a.errors {
		if structuredErr, ok := err.(*StructuredError); ok {
			if structuredErr.Type == errorType {
				filteredErrors = append(filteredErrors, err)
			}
		} else {
			// For non-structured errors, try to determine type
			structuredErr := a.handler.HandleError(err)
			if structuredErr.Type == errorType {
				filteredErrors = append(filteredErrors, err)
			}
		}
	}

	return filteredErrors
}

// Clear removes all errors from the aggregator
func (a *DefaultErrorAggregator) Clear() {
	a.errors = make([]error, 0)
}

// ToError converts the aggregated errors to a single error
func (a *DefaultErrorAggregator) ToError() error {
	if len(a.errors) == 0 {
		return nil
	}

	if len(a.errors) == 1 {
		return a.errors[0]
	}

	return &ErrorCollection{Errors: a.errors}
}

// Count returns the number of errors in the aggregator
func (a *DefaultErrorAggregator) Count() int {
	return len(a.errors)
}

// CountByType returns the number of errors of a specific type
func (a *DefaultErrorAggregator) CountByType(errorType ErrorType) int {
	return len(a.GetErrorsByType(errorType))
}

// GetSummary returns a summary of all errors
func (a *DefaultErrorAggregator) GetSummary() string {
	if len(a.errors) == 0 {
		return "No errors"
	}

	if len(a.errors) == 1 {
		return a.handler.FormatError(a.errors[0])
	}

	// Group errors by type
	errorsByType := make(map[ErrorType][]error)
	for _, err := range a.errors {
		structuredErr := a.handler.HandleError(err)
		errorsByType[structuredErr.Type] = append(errorsByType[structuredErr.Type], err)
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Found %d errors:\n", len(a.errors)))

	for errorType, errors := range errorsByType {
		summary.WriteString(fmt.Sprintf("\n%s (%d):\n", strings.ToUpper(string(errorType)), len(errors)))
		for i, err := range errors {
			if i < 3 { // Show first 3 errors of each type
				summary.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
			} else if i == 3 {
				summary.WriteString(fmt.Sprintf("  ... and %d more\n", len(errors)-3))
				break
			}
		}
	}

	return summary.String()
}

// ValidationAggregator is a specialized aggregator for validation errors
type ValidationAggregator struct {
	*DefaultErrorAggregator
	warnings []error
}

// NewValidationAggregator creates a new validation aggregator
func NewValidationAggregator() *ValidationAggregator {
	return &ValidationAggregator{
		DefaultErrorAggregator: NewDefaultErrorAggregator(),
		warnings:               make([]error, 0),
	}
}

// AddWarning adds a warning to the aggregator
func (va *ValidationAggregator) AddWarning(err error) {
	if err != nil {
		va.warnings = append(va.warnings, err)
	}
}

// AddWarningWithContext adds a warning with field context
func (va *ValidationAggregator) AddWarningWithContext(field string, err error) {
	if err != nil {
		wrappedErr := WrapWithField(err, field)
		va.warnings = append(va.warnings, wrappedErr)
	}
}

// HasWarnings returns true if there are any warnings
func (va *ValidationAggregator) HasWarnings() bool {
	return len(va.warnings) > 0
}

// GetWarnings returns all warnings
func (va *ValidationAggregator) GetWarnings() []error {
	return va.warnings
}

// ToValidationResult converts the aggregated errors and warnings to a validation result
func (va *ValidationAggregator) ToValidationResult() *ValidationResult {
	result := &ValidationResult{
		Valid: !va.HasErrors(),
	}

	// Convert errors to structured errors
	for _, err := range va.GetErrors() {
		if structuredErr, ok := err.(*StructuredError); ok {
			result.Errors = append(result.Errors, structuredErr)
		} else {
			structuredErr := va.handler.HandleError(err)
			result.Errors = append(result.Errors, structuredErr)
		}
	}

	// Convert warnings to structured errors
	for _, warning := range va.warnings {
		if structuredErr, ok := warning.(*StructuredError); ok {
			result.Warnings = append(result.Warnings, structuredErr)
		} else {
			structuredErr := va.handler.HandleError(warning)
			result.Warnings = append(result.Warnings, structuredErr)
		}
	}

	return result
}

// ClearWarnings removes all warnings from the aggregator
func (va *ValidationAggregator) ClearWarnings() {
	va.warnings = make([]error, 0)
}

// ClearAll removes all errors and warnings from the aggregator
func (va *ValidationAggregator) ClearAll() {
	va.Clear()
	va.ClearWarnings()
}

// MultiFieldAggregator aggregates errors by field name
type MultiFieldAggregator struct {
	fieldErrors map[string]*DefaultErrorAggregator
	handler     ErrorHandler
}

// NewMultiFieldAggregator creates a new multi-field aggregator
func NewMultiFieldAggregator() *MultiFieldAggregator {
	return &MultiFieldAggregator{
		fieldErrors: make(map[string]*DefaultErrorAggregator),
		handler:     NewDefaultErrorHandler(),
	}
}

// AddFieldError adds an error for a specific field
func (mfa *MultiFieldAggregator) AddFieldError(field string, err error) {
	if err == nil {
		return
	}

	if mfa.fieldErrors[field] == nil {
		mfa.fieldErrors[field] = NewDefaultErrorAggregator()
	}

	mfa.fieldErrors[field].AddErrorWithContext(field, err)
}

// GetFieldErrors returns errors for a specific field
func (mfa *MultiFieldAggregator) GetFieldErrors(field string) []error {
	if aggregator, exists := mfa.fieldErrors[field]; exists {
		return aggregator.GetErrors()
	}
	return nil
}

// GetAllFieldErrors returns all field errors
func (mfa *MultiFieldAggregator) GetAllFieldErrors() map[string][]error {
	result := make(map[string][]error)
	for field, aggregator := range mfa.fieldErrors {
		result[field] = aggregator.GetErrors()
	}
	return result
}

// HasFieldErrors returns true if there are errors for the specified field
func (mfa *MultiFieldAggregator) HasFieldErrors(field string) bool {
	if aggregator, exists := mfa.fieldErrors[field]; exists {
		return aggregator.HasErrors()
	}
	return false
}

// HasAnyErrors returns true if there are any field errors
func (mfa *MultiFieldAggregator) HasAnyErrors() bool {
	for _, aggregator := range mfa.fieldErrors {
		if aggregator.HasErrors() {
			return true
		}
	}
	return false
}

// ToError converts all field errors to a single error
func (mfa *MultiFieldAggregator) ToError() error {
	if !mfa.HasAnyErrors() {
		return nil
	}

	var allErrors []error
	for _, aggregator := range mfa.fieldErrors {
		allErrors = append(allErrors, aggregator.GetErrors()...)
	}

	if len(allErrors) == 1 {
		return allErrors[0]
	}

	return &ErrorCollection{Errors: allErrors}
}
