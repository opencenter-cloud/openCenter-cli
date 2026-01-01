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

package flags

import (
	"fmt"
	"strings"
)

// ErrorType represents the category of configuration error
type ErrorType string

const (
	ErrorTypeSyntax     ErrorType = "syntax"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeType       ErrorType = "type"
	ErrorTypeConflict   ErrorType = "conflict"
	ErrorTypeTemplate   ErrorType = "template"
	ErrorTypePath       ErrorType = "path"
	ErrorTypePermission ErrorType = "permission"
)

// ConfigError represents a configuration-related error with detailed context
type ConfigError struct {
	Type        ErrorType     `json:"type"`
	Code        string        `json:"code"`
	Message     string        `json:"message"`
	Path        string        `json:"path,omitempty"`
	Suggestions []string      `json:"suggestions,omitempty"`
	Context     *ErrorContext `json:"context,omitempty"`
	Cause       error         `json:"-"` // Original error that caused this
}

// ErrorContext provides additional error information
type ErrorContext struct {
	Line        int      `json:"line,omitempty"`
	Column      int      `json:"column,omitempty"`
	Source      string   `json:"source,omitempty"`
	RelatedPath []string `json:"related_paths,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	Value       string   `json:"value,omitempty"`
	ExpectedType string  `json:"expected_type,omitempty"`
}

// Error implements the error interface
func (e *ConfigError) Error() string {
	var parts []string
	
	// Add error type and code
	if e.Code != "" {
		parts = append(parts, fmt.Sprintf("[%s:%s]", e.Type, e.Code))
	} else {
		parts = append(parts, fmt.Sprintf("[%s]", e.Type))
	}
	
	// Add path if available
	if e.Path != "" {
		parts = append(parts, fmt.Sprintf("at path '%s'", e.Path))
	}
	
	// Add main message
	parts = append(parts, e.Message)
	
	// Add context information
	if e.Context != nil {
		if e.Context.Value != "" {
			parts = append(parts, fmt.Sprintf("(value: %q)", e.Context.Value))
		}
		if e.Context.ExpectedType != "" {
			parts = append(parts, fmt.Sprintf("(expected type: %s)", e.Context.ExpectedType))
		}
	}
	
	return strings.Join(parts, " ")
}

// Unwrap returns the underlying error
func (e *ConfigError) Unwrap() error {
	return e.Cause
}

// WithSuggestions adds suggestions to the error
func (e *ConfigError) WithSuggestions(suggestions ...string) *ConfigError {
	e.Suggestions = append(e.Suggestions, suggestions...)
	return e
}

// WithContext adds context information to the error
func (e *ConfigError) WithContext(context *ErrorContext) *ConfigError {
	e.Context = context
	return e
}

// WithCause sets the underlying cause of the error
func (e *ConfigError) WithCause(cause error) *ConfigError {
	e.Cause = cause
	return e
}

// ErrorBuilder helps construct ConfigError instances
type ErrorBuilder struct {
	errorType ErrorType
	code      string
	path      string
}

// NewErrorBuilder creates a new error builder
func NewErrorBuilder(errorType ErrorType) *ErrorBuilder {
	return &ErrorBuilder{errorType: errorType}
}

// WithCode sets the error code
func (b *ErrorBuilder) WithCode(code string) *ErrorBuilder {
	b.code = code
	return b
}

// WithPath sets the configuration path
func (b *ErrorBuilder) WithPath(path string) *ErrorBuilder {
	b.path = path
	return b
}

// Build creates a ConfigError with the specified message
func (b *ErrorBuilder) Build(message string, args ...interface{}) *ConfigError {
	return &ConfigError{
		Type:    b.errorType,
		Code:    b.code,
		Path:    b.path,
		Message: fmt.Sprintf(message, args...),
	}
}

// Predefined error builders for common error types
var (
	SyntaxError     = NewErrorBuilder(ErrorTypeSyntax)
	ValidationError = NewErrorBuilder(ErrorTypeValidation)
	TypeError       = NewErrorBuilder(ErrorTypeType)
	ConflictError   = NewErrorBuilder(ErrorTypeConflict)
	TemplateError   = NewErrorBuilder(ErrorTypeTemplate)
	PathError       = NewErrorBuilder(ErrorTypePath)
	PermissionError = NewErrorBuilder(ErrorTypePermission)
)

// Error message templates with suggestions
var ErrorTemplates = map[string]ErrorTemplate{
	"invalid_array_index": {
		Message: "Array index %d is out of bounds for array of length %d",
		Suggestions: []string{
			"Use --array-append to add new elements",
			"Check that the array has been initialized",
			"Array indices start at 0",
		},
		Examples: []string{
			"--field[0].value=data",
			"--array-append field=newvalue",
		},
	},
	"invalid_json_syntax": {
		Message: "Invalid JSON syntax in flag '%s': %s",
		Suggestions: []string{
			"Check that all quotes are properly escaped",
			"Ensure JSON objects use double quotes for keys",
			"Validate JSON syntax using a JSON validator",
		},
		Examples: []string{
			`--json-set 'path={"key": "value"}'`,
			`--json-set 'config={"enabled": true, "count": 5}'`,
		},
	},
	"template_variable_undefined": {
		Message: "Template variable '%s' is undefined in path '%s'",
		Suggestions: []string{
			"Define the variable with --template-var %s=value",
			"Check for typos in the variable name",
			"Use {{.VAR}} syntax for template variables",
		},
		Examples: []string{
			"--template-var NAME=myapp",
			"--template-var ENV=production",
		},
	},
	"conflicting_flags": {
		Message: "Conflicting values for path '%s': flag '%s' sets '%v', flag '%s' sets '%v'",
		Suggestions: []string{
			"Later flags take precedence over earlier ones",
			"Remove duplicate flag definitions",
			"Use a single flag to set the value",
		},
		Examples: []string{
			"Remove one of the conflicting flags",
			"Combine values using JSON or YAML flags",
		},
	},
	"invalid_field_name": {
		Message: "Field '%s' not found in struct '%s'",
		Suggestions: []string{
			"Check the field name spelling",
			"Use the YAML tag name if different from field name",
			"Check the configuration schema documentation",
		},
		Examples: []string{
			"--opencenter.meta.organization=myorg",
			"--opencenter.cluster.kubernetes.version=1.31.4",
		},
	},
	"type_conversion_failed": {
		Message: "Cannot convert value '%s' to type %s",
		Suggestions: []string{
			"Check the expected data type for this field",
			"Use proper format for the target type",
			"For boolean fields, use 'true' or 'false'",
			"For numeric fields, use valid number format",
		},
		Examples: []string{
			"--field.enabled=true",
			"--field.count=42",
			"--field.name=string-value",
		},
	},
	"path_syntax_error": {
		Message: "Invalid path syntax: %s",
		Suggestions: []string{
			"Use dot notation for nested fields: field.subfield",
			"Use bracket notation for arrays: field[0].subfield",
			"Field names must start with letter or underscore",
			"Array indices must be non-negative integers",
		},
		Examples: []string{
			"--opencenter.meta.organization=myorg",
			"--items[0].name=first-item",
			"--servers.0.hostname=server1",
		},
	},
}

// ErrorTemplate defines a template for error messages
type ErrorTemplate struct {
	Message     string
	Suggestions []string
	Examples    []string
}

// CreateTemplatedError creates a ConfigError using a predefined template
func CreateTemplatedError(templateKey string, errorType ErrorType, path string, args ...interface{}) *ConfigError {
	template, exists := ErrorTemplates[templateKey]
	if !exists {
		return &ConfigError{
			Type:    errorType,
			Code:    templateKey,
			Path:    path,
			Message: fmt.Sprintf("Unknown error template: %s", templateKey),
		}
	}
	
	return &ConfigError{
		Type:        errorType,
		Code:        templateKey,
		Path:        path,
		Message:     fmt.Sprintf(template.Message, args...),
		Suggestions: template.Suggestions,
		Context: &ErrorContext{
			Examples: template.Examples,
		},
	}
}

// ErrorReporter collects and formats multiple errors
type ErrorReporter struct {
	errors []*ConfigError
}

// NewErrorReporter creates a new error reporter
func NewErrorReporter() *ErrorReporter {
	return &ErrorReporter{
		errors: make([]*ConfigError, 0),
	}
}

// Add adds an error to the reporter
func (r *ErrorReporter) Add(err *ConfigError) {
	r.errors = append(r.errors, err)
}

// AddError adds a regular error as a ConfigError
func (r *ErrorReporter) AddError(err error, errorType ErrorType, path string) {
	configErr := &ConfigError{
		Type:    errorType,
		Path:    path,
		Message: err.Error(),
		Cause:   err,
	}
	r.errors = append(r.errors, configErr)
}

// HasErrors returns true if there are any errors
func (r *ErrorReporter) HasErrors() bool {
	return len(r.errors) > 0
}

// Errors returns all collected errors
func (r *ErrorReporter) Errors() []*ConfigError {
	return r.errors
}

// Error returns a formatted string with all errors
func (r *ErrorReporter) Error() string {
	if len(r.errors) == 0 {
		return "no errors"
	}
	
	if len(r.errors) == 1 {
		return r.errors[0].Error()
	}
	
	var parts []string
	parts = append(parts, fmt.Sprintf("Multiple configuration errors (%d):", len(r.errors)))
	
	for i, err := range r.errors {
		parts = append(parts, fmt.Sprintf("  %d. %s", i+1, err.Error()))
		
		// Add suggestions for the first few errors
		if i < 3 && len(err.Suggestions) > 0 {
			parts = append(parts, "     Suggestions:")
			for _, suggestion := range err.Suggestions {
				parts = append(parts, fmt.Sprintf("     - %s", suggestion))
			}
		}
	}
	
	return strings.Join(parts, "\n")
}

// Clear removes all errors from the reporter
func (r *ErrorReporter) Clear() {
	r.errors = r.errors[:0]
}

// GetErrorsByType returns errors of a specific type
func (r *ErrorReporter) GetErrorsByType(errorType ErrorType) []*ConfigError {
	var filtered []*ConfigError
	for _, err := range r.errors {
		if err.Type == errorType {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

// GetErrorsByPath returns errors for a specific path
func (r *ErrorReporter) GetErrorsByPath(path string) []*ConfigError {
	var filtered []*ConfigError
	for _, err := range r.errors {
		if err.Path == path {
			filtered = append(filtered, err)
		}
	}
	return filtered
}