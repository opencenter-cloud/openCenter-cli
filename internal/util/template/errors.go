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

package template

import (
	"fmt"
	"strings"
)

// TemplateErrorType represents different types of template errors
type TemplateErrorType string

const (
	// ErrorTypeNotFound indicates a template was not found
	ErrorTypeNotFound TemplateErrorType = "template_not_found"
	// ErrorTypeParsing indicates a template parsing error
	ErrorTypeParsing TemplateErrorType = "template_parsing"
	// ErrorTypeExecution indicates a template execution error
	ErrorTypeExecution TemplateErrorType = "template_execution"
	// ErrorTypeValidation indicates a template validation error
	ErrorTypeValidation TemplateErrorType = "template_validation"
	// ErrorTypeDataValidation indicates a data validation error
	ErrorTypeDataValidation TemplateErrorType = "data_validation"
	// ErrorTypeInitialization indicates an initialization error
	ErrorTypeInitialization TemplateErrorType = "initialization"
	// ErrorTypeNetworkPlugin indicates a network plugin error
	ErrorTypeNetworkPlugin TemplateErrorType = "network_plugin"
)

// TemplateError represents a comprehensive template error with context and suggestions
type TemplateError struct {
	Type        TemplateErrorType
	Template    string
	Field       string
	Message     string
	Cause       error
	Suggestions []string
}

// Error implements the error interface
func (e *TemplateError) Error() string {
	var parts []string

	if e.Template != "" {
		parts = append(parts, fmt.Sprintf("template '%s'", e.Template))
	}

	if e.Field != "" {
		parts = append(parts, fmt.Sprintf("field '%s'", e.Field))
	}

	parts = append(parts, e.Message)

	result := strings.Join(parts, ": ")

	if e.Cause != nil {
		result = fmt.Sprintf("%s: %v", result, e.Cause)
	}

	return result
}

// Unwrap returns the underlying cause error
func (e *TemplateError) Unwrap() error {
	return e.Cause
}

// WithSuggestions adds suggestions to the error
func (e *TemplateError) WithSuggestions(suggestions ...string) *TemplateError {
	e.Suggestions = append(e.Suggestions, suggestions...)
	return e
}

// GetSuggestions returns error suggestions
func (e *TemplateError) GetSuggestions() []string {
	return e.Suggestions
}

// NewTemplateError creates a new template error
func NewTemplateError(errorType TemplateErrorType, template, message string, cause error) *TemplateError {
	return &TemplateError{
		Type:     errorType,
		Template: template,
		Message:  message,
		Cause:    cause,
	}
}

// NewTemplateNotFoundError creates a template not found error
func NewTemplateNotFoundError(templateName string, availableTemplates []string) *TemplateError {
	err := &TemplateError{
		Type:     ErrorTypeNotFound,
		Template: templateName,
		Message:  "template not found",
	}

	if len(availableTemplates) > 0 {
		err.WithSuggestions(
			"Available templates: "+strings.Join(availableTemplates, ", "),
			"Check template name spelling",
			"Ensure template is properly loaded",
		)
	} else {
		err.WithSuggestions(
			"No templates are currently loaded",
			"Initialize the template engine with templates",
			"Check template loading configuration",
		)
	}

	return err
}

// NewTemplateParsingError creates a template parsing error
func NewTemplateParsingError(templateName string, cause error) *TemplateError {
	return &TemplateError{
		Type:     ErrorTypeParsing,
		Template: templateName,
		Message:  "failed to parse template",
		Cause:    cause,
		Suggestions: []string{
			"Check template syntax",
			"Verify template function usage",
			"Ensure proper template delimiters",
		},
	}
}

// NewTemplateExecutionError creates a template execution error
func NewTemplateExecutionError(templateName string, cause error) *TemplateError {
	return &TemplateError{
		Type:     ErrorTypeExecution,
		Template: templateName,
		Message:  "failed to execute template",
		Cause:    cause,
		Suggestions: []string{
			"Check template data structure",
			"Verify all required fields are present",
			"Ensure template functions are available",
			"Check for nil pointer access in template",
		},
	}
}

// NewDataValidationError creates a data validation error
func NewDataValidationError(templateName, field string, cause error) *TemplateError {
	return &TemplateError{
		Type:     ErrorTypeDataValidation,
		Template: templateName,
		Field:    field,
		Message:  "data validation failed",
		Cause:    cause,
		Suggestions: []string{
			"Check data structure matches template requirements",
			"Ensure all required fields are provided",
			"Verify field types are correct",
		},
	}
}

// NewNetworkPluginError creates a network plugin error
func NewNetworkPluginError(pluginType string, cause error) *TemplateError {
	return &TemplateError{
		Type:    ErrorTypeNetworkPlugin,
		Field:   pluginType,
		Message: "network plugin error",
		Cause:   cause,
		Suggestions: []string{
			"Check network plugin configuration",
			"Verify plugin type is supported",
			"Ensure plugin-specific fields are correct",
		},
	}
}

// NewInitializationError creates an initialization error
func NewInitializationError(component string, cause error) *TemplateError {
	return &TemplateError{
		Type:    ErrorTypeInitialization,
		Field:   component,
		Message: "initialization failed",
		Cause:   cause,
		Suggestions: []string{
			"Check component dependencies",
			"Verify initialization order",
			"Ensure required resources are available",
		},
	}
}

// WrapTemplateError wraps an existing error as a template error
func WrapTemplateError(errorType TemplateErrorType, template, message string, cause error) *TemplateError {
	return &TemplateError{
		Type:     errorType,
		Template: template,
		Message:  message,
		Cause:    cause,
	}
}

// IsTemplateError checks if an error is a template error
func IsTemplateError(err error) bool {
	_, ok := err.(*TemplateError)
	return ok
}

// GetTemplateError extracts a template error from an error
func GetTemplateError(err error) (*TemplateError, bool) {
	if templateErr, ok := err.(*TemplateError); ok {
		return templateErr, true
	}
	return nil, false
}
