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

package config

import (
	stderrors "errors"
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
)

// Configuration-specific error creation functions.
// These functions provide convenient wrappers around the generic error types
// from internal/util/errors, adding configuration-specific context and suggestions.

// NewFileError creates a file operation error with configuration context.
//
// This error type is used when file operations fail during configuration
// loading, saving, or deletion. It includes the file path and operation
// context to help users diagnose and fix the issue.
//
// Parameters:
//   - operation: The file operation that failed (e.g., "read", "write", "delete")
//   - path: The file path that was being accessed
//   - cause: The underlying error that caused the failure
//
// Returns:
//   - *errors.StructuredError: A structured error with file context
//
// Example:
//
//	err := NewFileError("read", "/path/to/config.yaml", os.ErrNotExist)
//	// Error: file operation failed: read
//	// Path: /path/to/config.yaml
//	// Suggestions:
//	//   - Verify the path exists: ls -la /path/to/config.yaml
//	//   - Check file permissions and ownership
func NewFileError(operation, path string, cause error) *errors.StructuredError {
	suggestions := []string{
		fmt.Sprintf("Verify the path exists: ls -la %s", path),
		"Check file permissions and ownership",
		"Ensure parent directories exist",
		"Check disk space: df -h",
	}

	// Add operation-specific suggestions
	switch operation {
	case "read":
		suggestions = append(suggestions, "Ensure the file is readable: chmod +r "+path)
	case "write", "save":
		suggestions = append(suggestions, "Ensure the directory is writable: chmod +w "+path)
		suggestions = append(suggestions, "Check if the file is locked by another process")
	case "delete":
		suggestions = append(suggestions, "Ensure you have delete permissions on the directory")
	}

	return &errors.StructuredError{
		Type:        errors.FileError,
		Message:     fmt.Sprintf("file operation failed: %s", operation),
		Cause:       cause,
		Operation:   operation,
		FilePath:    path,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"path": path, "operation": operation},
		Retryable:   isRetryableFileError(cause),
	}
}

// NewValidationError creates a validation error with field context.
//
// This error type is used when configuration validation fails. It includes
// the field name and specific validation failure details to help users
// correct their configuration.
//
// Parameters:
//   - field: The configuration field that failed validation
//   - message: A description of the validation failure
//   - cause: The underlying error (can be nil)
//
// Returns:
//   - *errors.StructuredError: A structured error with validation context
//
// Example:
//
//	err := NewValidationError("cluster.name", "cluster name cannot be empty", nil)
//	// Error: Field 'cluster.name': cluster name cannot be empty
//	// Suggestions:
//	//   - Run: opencenter cluster validate to check configuration
//	//   - Generate schema with: opencenter config ide --schema-only
func NewValidationError(field, message string, cause error) *errors.StructuredError {
	suggestions := []string{
		"Run: opencenter cluster validate to check configuration",
		"Generate schema with: opencenter config ide --schema-only",
		"Check documentation at: https://docs.opencenter.io",
		"Edit configuration: opencenter cluster edit",
	}

	return &errors.StructuredError{
		Type:        errors.ValidationError,
		Field:       field,
		Message:     message,
		Cause:       cause,
		Operation:   "validation",
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// NewPathError creates a path resolution error with cluster context.
//
// This error type is used when path resolution fails for a cluster
// configuration. It includes the cluster name and organization to help
// users understand which configuration could not be found.
//
// Parameters:
//   - clusterName: The name of the cluster being accessed
//   - organization: The organization name (can be empty)
//   - cause: The underlying error that caused the path resolution failure
//
// Returns:
//   - *errors.StructuredError: A structured error with path context
//
// Example:
//
//	err := NewPathError("my-cluster", "my-org", os.ErrNotExist)
//	// Error: failed to resolve path for cluster "my-cluster"
//	// Suggestions:
//	//   - Verify cluster exists: opencenter cluster list
//	//   - Check organization name is correct
func NewPathError(clusterName, organization string, cause error) *errors.StructuredError {
	message := fmt.Sprintf("failed to resolve path for cluster %q", clusterName)
	if organization != "" {
		message += fmt.Sprintf(" in organization %q", organization)
	}

	suggestions := []string{
		"Verify cluster exists: opencenter cluster list",
		"Check organization name is correct",
		"Ensure configuration directory exists: ~/.config/opencenter/clusters",
		"Initialize cluster if needed: opencenter cluster init " + clusterName,
	}

	context := map[string]interface{}{
		"cluster_name": clusterName,
	}
	if organization != "" {
		context["organization"] = organization
	}

	return &errors.StructuredError{
		Type:        errors.PathError,
		Message:     message,
		Cause:       cause,
		Operation:   "path_resolution",
		Suggestions: suggestions,
		Context:     context,
		Retryable:   false,
	}
}

// NewParseError creates a YAML parsing error with file context.
//
// This error type is used when YAML parsing fails during configuration
// loading. It includes file path and line/column information when available
// to help users locate and fix syntax errors.
//
// Parameters:
//   - filePath: The path to the file being parsed
//   - lineNumber: The line number where parsing failed (0 if unknown)
//   - columnNumber: The column number where parsing failed (0 if unknown)
//   - cause: The underlying parsing error
//
// Returns:
//   - *errors.StructuredError: A structured error with parse context
//
// Example:
//
//	err := NewParseError("/path/to/config.yaml", 42, 15, yamlErr)
//	// Error: failed to parse YAML configuration
//	// File: /path/to/config.yaml:42:15
//	// Suggestions:
//	//   - Validate YAML syntax with: yamllint /path/to/config.yaml
//	//   - Check for proper indentation (use spaces, not tabs)
func NewParseError(filePath string, lineNumber, columnNumber int, cause error) *errors.StructuredError {
	message := "failed to parse YAML configuration"
	if cause != nil {
		message += ": " + cause.Error()
	}

	suggestions := []string{
		fmt.Sprintf("Validate YAML syntax with: yamllint %s", filePath),
		"Check for proper indentation (use spaces, not tabs)",
		"Ensure all quotes and brackets are properly closed",
		"Verify special characters are properly escaped",
		"Check for duplicate keys in the YAML structure",
	}

	return &errors.StructuredError{
		Type:         errors.ConfigError,
		Message:      message,
		Cause:        cause,
		Operation:    "yaml_parsing",
		FilePath:     filePath,
		LineNumber:   lineNumber,
		ColumnNumber: columnNumber,
		Suggestions:  suggestions,
		Context:      map[string]interface{}{"file": filePath},
		Retryable:    false,
	}
}

// NewConfigError creates a general configuration error.
//
// This error type is used for general configuration-related errors that
// don't fit into the more specific categories (file, validation, path, parse).
//
// Parameters:
//   - operation: The operation that failed (e.g., "load", "save", "delete")
//   - message: A description of what went wrong
//   - cause: The underlying error (can be nil)
//
// Returns:
//   - *errors.StructuredError: A structured error with configuration context
//
// Example:
//
//	err := NewConfigError("load", "configuration is corrupted", nil)
//	// Error: configuration operation failed: load
//	// Message: configuration is corrupted
func NewConfigError(operation, message string, cause error) *errors.StructuredError {
	suggestions := []string{
		"Run: opencenter cluster validate to check configuration",
		"View current config: opencenter cluster describe",
		"Edit configuration: opencenter cluster edit",
		"Check documentation at: https://docs.opencenter.io",
	}

	return &errors.StructuredError{
		Type:        errors.ConfigError,
		Message:     message,
		Cause:       cause,
		Operation:   operation,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// isRetryableFileError determines if a file error can be retried.
//
// Some file errors are temporary and can be resolved by retrying the
// operation (e.g., resource temporarily unavailable, too many open files).
// Others are permanent and require user intervention (e.g., permission denied,
// file not found).
//
// Parameters:
//   - err: The error to check
//
// Returns:
//   - bool: true if the error is retryable, false otherwise
func isRetryableFileError(err error) bool {
	if err == nil {
		return false
	}

	// Use the utility function from the errors package
	// This checks for patterns like "resource temporarily unavailable",
	// "too many open files", "device busy", etc.
	return errors.IsTimeoutError(err) || isTemporaryError(err)
}

// isTemporaryError checks if an error is temporary.
func isTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	// Check if the error implements the Temporary interface
	type temporary interface {
		Temporary() bool
	}

	if te, ok := err.(temporary); ok {
		return te.Temporary()
	}

	return false
}

// WrapFileError wraps an existing error as a file error with additional context.
//
// This is useful when you want to add file operation context to an error
// that was returned from a lower-level function.
//
// Parameters:
//   - err: The error to wrap
//   - operation: The file operation being performed
//   - path: The file path being accessed
//
// Returns:
//   - error: The wrapped error with file context
//
// Example:
//
//	data, err := os.ReadFile(path)
//	if err != nil {
//	    return nil, WrapFileError(err, "read", path)
//	}
func WrapFileError(err error, operation, path string) error {
	if err == nil {
		return nil
	}

	// If it's already a structured error, preserve it
	if se, ok := err.(*errors.StructuredError); ok {
		se.Operation = operation
		se.FilePath = path
		return se
	}

	return NewFileError(operation, path, err)
}

// WrapValidationError wraps an existing error as a validation error.
//
// Parameters:
//   - err: The error to wrap
//   - field: The field that failed validation
//
// Returns:
//   - error: The wrapped error with validation context
func WrapValidationError(err error, field string) error {
	if err == nil {
		return nil
	}

	// If it's already a structured error, preserve it
	if se, ok := err.(*errors.StructuredError); ok {
		se.Field = field
		se.Type = errors.ValidationError
		return se
	}

	return NewValidationError(field, err.Error(), err)
}

// WrapPathError wraps an existing error as a path error.
//
// Parameters:
//   - err: The error to wrap
//   - clusterName: The cluster name
//   - organization: The organization name
//
// Returns:
//   - error: The wrapped error with path context
func WrapPathError(err error, clusterName, organization string) error {
	if err == nil {
		return nil
	}

	// If it's already a structured error, preserve it
	if se, ok := err.(*errors.StructuredError); ok {
		se.Type = errors.PathError
		if se.Context == nil {
			se.Context = make(map[string]interface{})
		}
		se.Context["cluster_name"] = clusterName
		if organization != "" {
			se.Context["organization"] = organization
		}
		return se
	}

	return NewPathError(clusterName, organization, err)
}

// WrapParseError wraps an existing error as a parse error.
//
// Parameters:
//   - err: The error to wrap
//   - filePath: The file being parsed
//   - lineNumber: The line number where parsing failed
//   - columnNumber: The column number where parsing failed
//
// Returns:
//   - error: The wrapped error with parse context
func WrapParseError(err error, filePath string, lineNumber, columnNumber int) error {
	if err == nil {
		return nil
	}

	// If it's already a structured error, preserve it
	if se, ok := err.(*errors.StructuredError); ok {
		se.FilePath = filePath
		se.LineNumber = lineNumber
		se.ColumnNumber = columnNumber
		return se
	}

	return NewParseError(filePath, lineNumber, columnNumber, err)
}

// IsFileNotFoundError checks if an error is a "file not found" error.
//
// This is useful for distinguishing between different types of file errors
// and providing appropriate error handling.
//
// Parameters:
//   - err: The error to check
//
// Returns:
//   - bool: true if the error indicates a file was not found
//
// Example:
//
//	config, err := manager.Load(ctx, "my-cluster")
//	if IsFileNotFoundError(err) {
//	    // Handle missing configuration
//	    return initializeNewConfig()
//	}
func IsFileNotFoundError(err error) bool {
	return errors.IsFileNotFoundError(err)
}

// IsValidationError checks if an error is a validation error.
//
// Parameters:
//   - err: The error to check
//
// Returns:
//   - bool: true if the error is a validation error
func IsValidationError(err error) bool {
	if se, ok := err.(*errors.StructuredError); ok {
		return se.Type == errors.ValidationError
	}
	return false
}

// IsPathError checks if an error is a path resolution error.
//
// Parameters:
//   - err: The error to check
//
// Returns:
//   - bool: true if the error is a path error
func IsPathError(err error) bool {
	if se, ok := err.(*errors.StructuredError); ok {
		return se.Type == errors.PathError
	}
	return false
}

// IsParseError checks if an error is a YAML parsing error.
//
// Parameters:
//   - err: The error to check
//
// Returns:
//   - bool: true if the error is a parse error
func IsParseError(err error) bool {
	if se, ok := err.(*errors.StructuredError); ok {
		return se.Type == errors.ConfigError && se.Operation == "yaml_parsing"
	}
	return false
}

// GetErrorField extracts the field name from a validation error.
//
// Parameters:
//   - err: The error to extract from
//
// Returns:
//   - string: The field name, or empty string if not a validation error
func GetErrorField(err error) string {
	if se, ok := err.(*errors.StructuredError); ok {
		return se.Field
	}
	return ""
}

// GetErrorFilePath extracts the file path from a file or parse error.
//
// Parameters:
//   - err: The error to extract from
//
// Returns:
//   - string: The file path, or empty string if not available
func GetErrorFilePath(err error) string {
	if se, ok := err.(*errors.StructuredError); ok {
		return se.FilePath
	}
	return ""
}

// GetErrorSuggestions extracts suggestions from a structured error.
//
// Parameters:
//   - err: The error to extract from
//
// Returns:
//   - []string: The suggestions, or empty slice if not available
func GetErrorSuggestions(err error) []string {
	if se, ok := err.(*errors.StructuredError); ok {
		return se.Suggestions
	}
	return nil
}

// ConfigNotFoundError is a sentinel error type indicating that a cluster
// configuration directory or file does not exist. Commands that encounter
// this error should exit with code 3 so CI/CD pipelines can distinguish
// missing-configuration failures from general errors.
type ConfigNotFoundError struct {
	ClusterName string
	Err         error
}

// Error implements the error interface.
func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("cluster configuration not found: %s", e.ClusterName)
}

// Unwrap returns the underlying error for errors.Is/errors.As compatibility.
func (e *ConfigNotFoundError) Unwrap() error {
	return e.Err
}

// NewConfigNotFoundError creates a ConfigNotFoundError for the given cluster name.
func NewConfigNotFoundError(clusterName string, cause error) *ConfigNotFoundError {
	return &ConfigNotFoundError{
		ClusterName: clusterName,
		Err:         cause,
	}
}

// IsConfigNotFoundError checks whether err (or any error in its chain)
// is a ConfigNotFoundError.
func IsConfigNotFoundError(err error) bool {
	var target *ConfigNotFoundError
	return stderrors.As(err, &target)
}
