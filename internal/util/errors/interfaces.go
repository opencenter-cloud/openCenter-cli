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

import "context"

// ErrorType represents different categories of errors
type ErrorType string

const (
	ValidationError ErrorType = "validation"
	PathError       ErrorType = "path"
	PermissionError ErrorType = "permission"
	TemplateError   ErrorType = "template"
	SOPSError       ErrorType = "sops"
	ConfigError     ErrorType = "config"
	NetworkError    ErrorType = "network"
	FileError       ErrorType = "file"
	SystemError     ErrorType = "system"
	UserError       ErrorType = "user"
	CloudError      ErrorType = "cloud"
	CredentialError ErrorType = "credential"
	ServiceError    ErrorType = "service"
	GenerationError ErrorType = "generation"
	OperationalError ErrorType = "operational"
)

// ErrorHandler interface for handling and formatting errors
type ErrorHandler interface {
	HandleError(err error) *StructuredError
	FormatError(err error) string
	GetSuggestions(err error) []string
	IsRetryable(err error) bool
	CreateValidationError(field, message string, suggestions ...string) *StructuredError
	CreateFileError(operation, path string, cause error) *StructuredError
	CreateConfigError(message string, cause error) *StructuredError
	Wrap(err error, operation, context string) error
}

// ErrorWrapper interface for wrapping errors with context
type ErrorWrapper interface {
	WrapError(err error, message string) error
	WrapErrorWithContext(ctx context.Context, err error, message string) error
	WrapErrorWithType(err error, errorType ErrorType, message string) error
	UnwrapError(err error) error
}

// ErrorAggregator interface for collecting and managing multiple errors
type ErrorAggregator interface {
	AddError(err error)
	AddErrorWithContext(field string, err error)
	HasErrors() bool
	GetErrors() []error
	GetErrorsByType(errorType ErrorType) []error
	Clear()
	ToError() error
}

// StructuredError represents a structured error with additional context
type StructuredError struct {
	Type         ErrorType              `json:"type"`
	Field        string                 `json:"field,omitempty"`
	Message      string                 `json:"message"`
	Cause        error                  `json:"-"`
	Suggestions  []string               `json:"suggestions,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
	Retryable    bool                   `json:"retryable"`
	FilePath     string                 `json:"file_path,omitempty"`
	LineNumber   int                    `json:"line_number,omitempty"`
	ColumnNumber int                    `json:"column_number,omitempty"`
	Operation    string                 `json:"operation,omitempty"`
}

// Error implements the error interface
func (e *StructuredError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// Unwrap returns the underlying cause error
func (e *StructuredError) Unwrap() error {
	return e.Cause
}

// Is checks if the target error matches this error's type
func (e *StructuredError) Is(target error) bool {
	t, ok := target.(*StructuredError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// ErrorCollection represents a collection of errors
type ErrorCollection struct {
	Errors []error `json:"errors"`
}

// Error implements the error interface for ErrorCollection
func (ec *ErrorCollection) Error() string {
	if len(ec.Errors) == 0 {
		return "no errors"
	}
	if len(ec.Errors) == 1 {
		return ec.Errors[0].Error()
	}
	return "multiple errors occurred"
}

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	Valid    bool               `json:"valid"`
	Errors   []*StructuredError `json:"errors,omitempty"`
	Warnings []*StructuredError `json:"warnings,omitempty"`
}

// HasErrors returns true if there are validation errors
func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

// HasWarnings returns true if there are validation warnings
func (vr *ValidationResult) HasWarnings() bool {
	return len(vr.Warnings) > 0
}

// ToError converts the validation result to an error if there are errors
func (vr *ValidationResult) ToError() error {
	if !vr.HasErrors() {
		return nil
	}

	if len(vr.Errors) == 1 {
		return vr.Errors[0]
	}

	var errors []error
	for _, err := range vr.Errors {
		errors = append(errors, err)
	}

	return &ErrorCollection{Errors: errors}
}
