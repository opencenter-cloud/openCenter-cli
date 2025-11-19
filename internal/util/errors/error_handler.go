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
	"os"
	"strings"
)

// DefaultErrorHandler implements ErrorHandler interface
type DefaultErrorHandler struct {
	suggestionMap map[ErrorType][]string
}

// NewDefaultErrorHandler creates a new default error handler
func NewDefaultErrorHandler() *DefaultErrorHandler {
	handler := &DefaultErrorHandler{
		suggestionMap: make(map[ErrorType][]string),
	}
	handler.initializeSuggestions()
	return handler
}

// HandleError converts an error to a structured error
func (h *DefaultErrorHandler) HandleError(err error) *StructuredError {
	if err == nil {
		return nil
	}

	// Check if it's already a structured error
	if structuredErr, ok := err.(*StructuredError); ok {
		return structuredErr
	}

	// Determine error type based on error message
	errorType := h.determineErrorType(err)

	return &StructuredError{
		Type:        errorType,
		Message:     err.Error(),
		Cause:       err,
		Suggestions: h.GetSuggestions(err),
		Retryable:   h.IsRetryable(err),
	}
}

// FormatError formats an error for display
func (h *DefaultErrorHandler) FormatError(err error) string {
	if err == nil {
		return ""
	}

	structuredErr := h.HandleError(err)

	var parts []string

	// Add error type if not user error
	if structuredErr.Type != UserError {
		parts = append(parts, fmt.Sprintf("[%s]", strings.ToUpper(string(structuredErr.Type))))
	}

	// Add field if present
	if structuredErr.Field != "" {
		parts = append(parts, fmt.Sprintf("Field '%s':", structuredErr.Field))
	}

	// Add message
	parts = append(parts, structuredErr.Message)

	result := strings.Join(parts, " ")

	// Add suggestions if available
	if len(structuredErr.Suggestions) > 0 {
		result += "\n\nSuggestions:"
		for _, suggestion := range structuredErr.Suggestions {
			result += "\n  - " + suggestion
		}
	}

	return result
}

// GetSuggestions returns suggestions for resolving an error
func (h *DefaultErrorHandler) GetSuggestions(err error) []string {
	if err == nil {
		return nil
	}

	errorType := h.determineErrorType(err)
	suggestions := h.suggestionMap[errorType]

	// Add specific suggestions based on error content
	errorMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errorMsg, "permission denied"):
		suggestions = append(suggestions, "Check file and directory permissions")
		suggestions = append(suggestions, "Ensure you have write access to the target directory")

	case strings.Contains(errorMsg, "no such file or directory"):
		suggestions = append(suggestions, "Verify the file or directory path exists")
		suggestions = append(suggestions, "Check for typos in the path")

	case strings.Contains(errorMsg, "invalid"):
		suggestions = append(suggestions, "Review the input format and requirements")
		suggestions = append(suggestions, "Check the documentation for valid values")

	case strings.Contains(errorMsg, "connection"):
		suggestions = append(suggestions, "Check your network connectivity")
		suggestions = append(suggestions, "Verify firewall settings")

	case strings.Contains(errorMsg, "timeout"):
		suggestions = append(suggestions, "Retry the operation")
		suggestions = append(suggestions, "Check if the service is responsive")
	}

	return suggestions
}

// IsRetryable determines if an error is retryable
func (h *DefaultErrorHandler) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := strings.ToLower(err.Error())

	// Network-related errors are usually retryable
	if strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "network") ||
		strings.Contains(errorMsg, "temporary") {
		return true
	}

	// File system errors that might be temporary
	if strings.Contains(errorMsg, "resource temporarily unavailable") ||
		strings.Contains(errorMsg, "device busy") {
		return true
	}

	// Validation and permission errors are usually not retryable
	if strings.Contains(errorMsg, "invalid") ||
		strings.Contains(errorMsg, "permission denied") ||
		strings.Contains(errorMsg, "access denied") {
		return false
	}

	return false
}

// determineErrorType determines the error type based on error content
func (h *DefaultErrorHandler) determineErrorType(err error) ErrorType {
	if err == nil {
		return SystemError
	}

	errorMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errorMsg, "validation") || strings.Contains(errorMsg, "invalid"):
		return ValidationError

	case strings.Contains(errorMsg, "path") || strings.Contains(errorMsg, "directory") || strings.Contains(errorMsg, "file"):
		return PathError

	case strings.Contains(errorMsg, "permission") || strings.Contains(errorMsg, "access"):
		return PermissionError

	case strings.Contains(errorMsg, "template"):
		return TemplateError

	case strings.Contains(errorMsg, "sops") || strings.Contains(errorMsg, "encryption"):
		return SOPSError

	case strings.Contains(errorMsg, "config"):
		return ConfigError

	case strings.Contains(errorMsg, "network") || strings.Contains(errorMsg, "connection"):
		return NetworkError

	case strings.Contains(errorMsg, "cloud") || strings.Contains(errorMsg, "aws") || strings.Contains(errorMsg, "openstack"):
		return CloudError

	case strings.Contains(errorMsg, "credential") || strings.Contains(errorMsg, "authentication") || strings.Contains(errorMsg, "unauthorized"):
		return CredentialError

	default:
		return SystemError
	}
}

// initializeSuggestions initializes the suggestion map
func (h *DefaultErrorHandler) initializeSuggestions() {
	h.suggestionMap[ValidationError] = []string{
		"Check the input format and requirements",
		"Refer to the documentation for valid values",
		"Validate required fields are provided",
	}

	h.suggestionMap[PathError] = []string{
		"Verify the path exists and is accessible",
		"Check for typos in the path",
		"Ensure proper path separators are used",
	}

	h.suggestionMap[PermissionError] = []string{
		"Check file and directory permissions",
		"Ensure you have the necessary access rights",
		"Run with appropriate privileges if needed",
	}

	h.suggestionMap[TemplateError] = []string{
		"Verify template syntax is correct",
		"Check that all required variables are provided",
		"Ensure template file exists and is readable",
	}

	h.suggestionMap[SOPSError] = []string{
		"Verify SOPS is installed and in PATH",
		"Check that age keys are properly configured",
		"Ensure SOPS_AGE_KEY_FILE environment variable is set",
	}

	h.suggestionMap[ConfigError] = []string{
		"Validate configuration file syntax",
		"Check that all required configuration fields are provided",
		"Ensure configuration file is readable",
	}

	h.suggestionMap[NetworkError] = []string{
		"Check network connectivity",
		"Verify firewall settings",
		"Ensure the target service is accessible",
	}

	h.suggestionMap[FileError] = []string{
		"Check file exists and is readable",
		"Verify file permissions",
		"Ensure sufficient disk space",
	}

	h.suggestionMap[SystemError] = []string{
		"Check system resources",
		"Verify system dependencies are installed",
		"Review system logs for additional details",
	}

	h.suggestionMap[CloudError] = []string{
		"Verify cloud provider credentials",
		"Check cloud provider service availability",
		"Ensure proper network connectivity to cloud APIs",
	}

	h.suggestionMap[CredentialError] = []string{
		"Verify credentials are correctly configured",
		"Check credential expiration dates",
		"Ensure proper permissions for the credentials",
		"Use SOPS to encrypt sensitive credentials",
	}
}

// CreateValidationError creates a validation error with suggestions
func CreateValidationError(field, message string, suggestions ...string) *StructuredError {
	return &StructuredError{
		Type:        ValidationError,
		Field:       field,
		Message:     message,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// CreatePathError creates a path-related error with suggestions
func CreatePathError(path, message string, cause error) *StructuredError {
	suggestions := []string{
		"Verify the path exists: " + path,
		"Check path permissions",
		"Ensure parent directories exist",
	}

	return &StructuredError{
		Type:        PathError,
		Field:       "path",
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"path": path},
		Retryable:   false,
	}
}

// CreatePermissionError creates a permission-related error
func CreatePermissionError(resource, operation string, cause error) *StructuredError {
	suggestions := []string{
		fmt.Sprintf("Check permissions for %s", resource),
		fmt.Sprintf("Ensure you have %s access", operation),
		"Run with appropriate privileges if needed",
	}

	return &StructuredError{
		Type:        PermissionError,
		Field:       "permissions",
		Message:     fmt.Sprintf("Permission denied for %s operation on %s", operation, resource),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"resource": resource, "operation": operation},
		Retryable:   false,
	}
}

// CreateSOPSError creates a SOPS-related error with suggestions
func CreateSOPSError(operation, message string, cause error) *StructuredError {
	suggestions := []string{
		"Verify SOPS is installed and in PATH",
		"Check age key configuration",
		"Ensure SOPS_AGE_KEY_FILE environment variable is set",
		"Validate key file permissions",
	}

	return &StructuredError{
		Type:        SOPSError,
		Field:       "sops",
		Message:     fmt.Sprintf("SOPS %s failed: %s", operation, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"operation": operation},
		Retryable:   false,
	}
}

// CreateConfigError creates a configuration-related error
func CreateConfigError(field, message string, cause error) *StructuredError {
	suggestions := []string{
		"Validate configuration file syntax",
		"Check required fields are provided",
		"Refer to configuration documentation",
	}

	return &StructuredError{
		Type:        ConfigError,
		Field:       field,
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// IsFileNotFoundError checks if an error is a file not found error
func IsFileNotFoundError(err error) bool {
	return os.IsNotExist(err)
}

// IsPermissionError checks if an error is a permission error
func IsPermissionError(err error) bool {
	return os.IsPermission(err)
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

// CreateCloudError creates a cloud provider-related error
func CreateCloudError(provider, operation, message string, cause error) *StructuredError {
	suggestions := []string{
		fmt.Sprintf("Verify %s credentials are correctly configured", provider),
		fmt.Sprintf("Check %s service availability", provider),
		"Ensure proper network connectivity to cloud APIs",
		"Review cloud provider documentation for requirements",
	}

	return &StructuredError{
		Type:        CloudError,
		Field:       "cloud_provider",
		Message:     fmt.Sprintf("%s %s failed: %s", provider, operation, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"provider": provider, "operation": operation},
		Retryable:   true, // Cloud operations are often retryable
	}
}

// CreateCredentialError creates a credential-related error
func CreateCredentialError(credentialType, field, message string, cause error) *StructuredError {
	suggestions := []string{
		fmt.Sprintf("Verify %s credentials are correctly set", credentialType),
		"Check credential expiration dates",
		"Ensure proper permissions for the credentials",
		"Use SOPS to encrypt sensitive credentials",
		"Refer to cloud provider documentation for credential setup",
	}

	return &StructuredError{
		Type:        CredentialError,
		Field:       field,
		Message:     fmt.Sprintf("%s credential error: %s", credentialType, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"credential_type": credentialType},
		Retryable:   false, // Credential errors usually require manual fix
	}
}
