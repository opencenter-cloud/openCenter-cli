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
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestErrorTypes verifies all error types are properly defined
func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		expected  string
	}{
		{"ValidationError", ValidationError, "validation"},
		{"PathError", PathError, "path"},
		{"PermissionError", PermissionError, "permission"},
		{"TemplateError", TemplateError, "template"},
		{"SOPSError", SOPSError, "sops"},
		{"ConfigError", ConfigError, "config"},
		{"NetworkError", NetworkError, "network"},
		{"FileError", FileError, "file"},
		{"SystemError", SystemError, "system"},
		{"UserError", UserError, "user"},
		{"CloudError", CloudError, "cloud"},
		{"CredentialError", CredentialError, "credential"},
		{"ServiceError", ServiceError, "service"},
		{"GenerationError", GenerationError, "generation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.errorType) != tt.expected {
				t.Errorf("ErrorType %s = %v, want %v", tt.name, tt.errorType, tt.expected)
			}
		})
	}
}

// TestStructuredError verifies StructuredError implements error interface
func TestStructuredError(t *testing.T) {
	tests := []struct {
		name     string
		err      *StructuredError
		expected string
	}{
		{
			name: "error with field",
			err: &StructuredError{
				Type:    ValidationError,
				Field:   "cluster_name",
				Message: "invalid cluster name",
			},
			expected: "cluster_name: invalid cluster name",
		},
		{
			name: "error without field",
			err: &StructuredError{
				Type:    SystemError,
				Message: "system failure",
			},
			expected: "system failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("StructuredError.Error() = %v, want %v", tt.err.Error(), tt.expected)
			}
		})
	}
}

// TestStructuredErrorUnwrap verifies Unwrap functionality
func TestStructuredErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &StructuredError{
		Type:    ValidationError,
		Message: "validation failed",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

// TestErrorCollection verifies ErrorCollection functionality
func TestErrorCollection(t *testing.T) {
	tests := []struct {
		name     string
		errors   []error
		expected string
	}{
		{
			name:     "no errors",
			errors:   []error{},
			expected: "no errors",
		},
		{
			name:     "single error",
			errors:   []error{errors.New("error 1")},
			expected: "error 1",
		},
		{
			name:     "multiple errors",
			errors:   []error{errors.New("error 1"), errors.New("error 2")},
			expected: "multiple errors occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &ErrorCollection{Errors: tt.errors}
			if ec.Error() != tt.expected {
				t.Errorf("ErrorCollection.Error() = %v, want %v", ec.Error(), tt.expected)
			}
		})
	}
}

// TestValidationResult verifies ValidationResult functionality
func TestValidationResult(t *testing.T) {
	t.Run("no errors or warnings", func(t *testing.T) {
		vr := &ValidationResult{Valid: true}
		if vr.HasErrors() {
			t.Error("HasErrors() = true, want false")
		}
		if vr.HasWarnings() {
			t.Error("HasWarnings() = true, want false")
		}
		if vr.ToError() != nil {
			t.Error("ToError() should return nil for valid result")
		}
	})

	t.Run("with errors", func(t *testing.T) {
		vr := &ValidationResult{
			Valid: false,
			Errors: []*StructuredError{
				{Type: ValidationError, Message: "error 1"},
			},
		}
		if !vr.HasErrors() {
			t.Error("HasErrors() = false, want true")
		}
		if vr.ToError() == nil {
			t.Error("ToError() should return error when errors exist")
		}
	})

	t.Run("with warnings", func(t *testing.T) {
		vr := &ValidationResult{
			Valid: true,
			Warnings: []*StructuredError{
				{Type: ValidationError, Message: "warning 1"},
			},
		}
		if !vr.HasWarnings() {
			t.Error("HasWarnings() = false, want true")
		}
	})
}

// TestDefaultErrorHandler verifies error handler functionality
func TestDefaultErrorHandler(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	t.Run("HandleError with nil", func(t *testing.T) {
		result := handler.HandleError(nil)
		if result != nil {
			t.Error("HandleError(nil) should return nil")
		}
	})

	t.Run("HandleError with structured error", func(t *testing.T) {
		original := &StructuredError{
			Type:    ValidationError,
			Message: "test error",
		}
		result := handler.HandleError(original)
		if result != original {
			t.Error("HandleError should return same structured error")
		}
	})

	t.Run("HandleError with regular error", func(t *testing.T) {
		err := errors.New("validation failed")
		result := handler.HandleError(err)
		if result == nil {
			t.Fatal("HandleError should return structured error")
		}
		if result.Type != ValidationError {
			t.Errorf("HandleError type = %v, want %v", result.Type, ValidationError)
		}
	})

	t.Run("FormatError", func(t *testing.T) {
		err := &StructuredError{
			Type:        ValidationError,
			Field:       "test_field",
			Message:     "test message",
			Suggestions: []string{"suggestion 1", "suggestion 2"},
		}
		formatted := handler.FormatError(err)
		if formatted == "" {
			t.Error("FormatError should return non-empty string")
		}
	})

	t.Run("IsRetryable", func(t *testing.T) {
		retryableErr := errors.New("connection timeout")
		if !handler.IsRetryable(retryableErr) {
			t.Error("timeout error should be retryable")
		}

		nonRetryableErr := errors.New("invalid input")
		if handler.IsRetryable(nonRetryableErr) {
			t.Error("validation error should not be retryable")
		}
	})

	t.Run("GetSuggestions with permission denied", func(t *testing.T) {
		err := errors.New("permission denied")
		suggestions := handler.GetSuggestions(err)
		if len(suggestions) == 0 {
			t.Error("GetSuggestions should return suggestions for permission denied")
		}
	})

	t.Run("GetSuggestions with file not found", func(t *testing.T) {
		err := errors.New("no such file or directory")
		suggestions := handler.GetSuggestions(err)
		if len(suggestions) == 0 {
			t.Error("GetSuggestions should return suggestions for file not found")
		}
	})

	t.Run("GetSuggestions with invalid input", func(t *testing.T) {
		err := errors.New("invalid configuration")
		suggestions := handler.GetSuggestions(err)
		if len(suggestions) == 0 {
			t.Error("GetSuggestions should return suggestions for invalid input")
		}
	})

	t.Run("GetSuggestions with connection error", func(t *testing.T) {
		err := errors.New("connection refused")
		suggestions := handler.GetSuggestions(err)
		if len(suggestions) == 0 {
			t.Error("GetSuggestions should return suggestions for connection error")
		}
	})

	t.Run("GetSuggestions with timeout", func(t *testing.T) {
		err := errors.New("operation timeout")
		suggestions := handler.GetSuggestions(err)
		if len(suggestions) == 0 {
			t.Error("GetSuggestions should return suggestions for timeout")
		}
	})
}

// TestDefaultErrorAggregator verifies error aggregator functionality
func TestDefaultErrorAggregator(t *testing.T) {
	t.Run("AddError and GetErrors", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		if agg.HasErrors() {
			t.Error("new aggregator should have no errors")
		}

		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		agg.AddError(err1)
		agg.AddError(err2)

		if !agg.HasErrors() {
			t.Error("aggregator should have errors")
		}

		if agg.Count() != 2 {
			t.Errorf("Count() = %d, want 2", agg.Count())
		}

		errs := agg.GetErrors()
		if len(errs) != 2 {
			t.Errorf("GetErrors() length = %d, want 2", len(errs))
		}
	})

	t.Run("AddError with nil", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		agg.AddError(nil)
		if agg.HasErrors() {
			t.Error("adding nil should not add error")
		}
	})

	t.Run("AddErrorWithContext", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		err := errors.New("test error")
		agg.AddErrorWithContext("test_field", err)

		if !agg.HasErrors() {
			t.Error("aggregator should have errors")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		agg.AddError(errors.New("error"))
		agg.Clear()

		if agg.HasErrors() {
			t.Error("Clear() should remove all errors")
		}
	})

	t.Run("ToError", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		if agg.ToError() != nil {
			t.Error("ToError() should return nil when no errors")
		}

		agg.AddError(errors.New("error 1"))
		if agg.ToError() == nil {
			t.Error("ToError() should return error when errors exist")
		}
	})

	t.Run("GetErrorsByType", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		agg.AddError(&StructuredError{Type: ValidationError, Message: "validation error"})
		agg.AddError(&StructuredError{Type: ConfigError, Message: "config error"})

		validationErrors := agg.GetErrorsByType(ValidationError)
		if len(validationErrors) != 1 {
			t.Errorf("GetErrorsByType(ValidationError) length = %d, want 1", len(validationErrors))
		}
	})

	t.Run("CountByType", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		agg.AddError(&StructuredError{Type: ValidationError, Message: "validation error 1"})
		agg.AddError(&StructuredError{Type: ValidationError, Message: "validation error 2"})
		agg.AddError(&StructuredError{Type: ConfigError, Message: "config error"})

		validationCount := agg.CountByType(ValidationError)
		if validationCount != 2 {
			t.Errorf("CountByType(ValidationError) = %d, want 2", validationCount)
		}

		configCount := agg.CountByType(ConfigError)
		if configCount != 1 {
			t.Errorf("CountByType(ConfigError) = %d, want 1", configCount)
		}

		systemCount := agg.CountByType(SystemError)
		if systemCount != 0 {
			t.Errorf("CountByType(SystemError) = %d, want 0", systemCount)
		}
	})

	t.Run("GetSummary", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()

		// Test with no errors
		summary := agg.GetSummary()
		if summary != "No errors" {
			t.Errorf("GetSummary() with no errors = %q, want %q", summary, "No errors")
		}

		// Test with single error
		agg.AddError(errors.New("single error"))
		summary = agg.GetSummary()
		if summary == "" {
			t.Error("GetSummary() with single error should not be empty")
		}

		// Test with multiple errors of different types
		agg.Clear()
		agg.AddError(&StructuredError{Type: ValidationError, Message: "validation error 1"})
		agg.AddError(&StructuredError{Type: ValidationError, Message: "validation error 2"})
		agg.AddError(&StructuredError{Type: ConfigError, Message: "config error"})

		summary = agg.GetSummary()
		if summary == "" {
			t.Error("GetSummary() with multiple errors should not be empty")
		}
		if !strings.Contains(summary, "Found 3 errors") {
			t.Errorf("GetSummary() should contain error count, got: %s", summary)
		}
	})
}

// TestValidationAggregator verifies validation aggregator functionality
func TestValidationAggregator(t *testing.T) {
	t.Run("AddWarning", func(t *testing.T) {
		agg := NewValidationAggregator()
		if agg.HasWarnings() {
			t.Error("new aggregator should have no warnings")
		}

		agg.AddWarning(errors.New("warning"))
		if !agg.HasWarnings() {
			t.Error("aggregator should have warnings")
		}
	})

	t.Run("AddWarningWithContext", func(t *testing.T) {
		agg := NewValidationAggregator()
		agg.AddWarningWithContext("test_field", errors.New("warning"))

		if !agg.HasWarnings() {
			t.Error("aggregator should have warnings")
		}

		warnings := agg.GetWarnings()
		if len(warnings) != 1 {
			t.Errorf("GetWarnings() length = %d, want 1", len(warnings))
		}
	})

	t.Run("GetWarnings", func(t *testing.T) {
		agg := NewValidationAggregator()
		agg.AddWarning(errors.New("warning 1"))
		agg.AddWarning(errors.New("warning 2"))

		warnings := agg.GetWarnings()
		if len(warnings) != 2 {
			t.Errorf("GetWarnings() length = %d, want 2", len(warnings))
		}
	})

	t.Run("ToValidationResult", func(t *testing.T) {
		agg := NewValidationAggregator()
		agg.AddError(errors.New("error"))
		agg.AddWarning(errors.New("warning"))

		result := agg.ToValidationResult()
		if result.Valid {
			t.Error("result should be invalid when errors exist")
		}
		if len(result.Errors) != 1 {
			t.Errorf("result.Errors length = %d, want 1", len(result.Errors))
		}
		if len(result.Warnings) != 1 {
			t.Errorf("result.Warnings length = %d, want 1", len(result.Warnings))
		}
	})

	t.Run("ClearAll", func(t *testing.T) {
		agg := NewValidationAggregator()
		agg.AddError(errors.New("error"))
		agg.AddWarning(errors.New("warning"))
		agg.ClearAll()

		if agg.HasErrors() || agg.HasWarnings() {
			t.Error("ClearAll() should remove all errors and warnings")
		}
	})
}

// TestDefaultErrorWrapper verifies error wrapper functionality
func TestDefaultErrorWrapper(t *testing.T) {
	wrapper := NewDefaultErrorWrapper()

	t.Run("WrapError with nil", func(t *testing.T) {
		result := wrapper.WrapError(nil, "context")
		if result != nil {
			t.Error("WrapError(nil) should return nil")
		}
	})

	t.Run("WrapError with regular error", func(t *testing.T) {
		err := errors.New("original error")
		wrapped := wrapper.WrapError(err, "additional context")

		if wrapped == nil {
			t.Fatal("WrapError should return error")
		}

		if wrapped.Error() == err.Error() {
			t.Error("wrapped error should include additional context")
		}
	})

	t.Run("WrapError with structured error", func(t *testing.T) {
		err := &StructuredError{
			Type:    ValidationError,
			Message: "original message",
		}
		wrapped := wrapper.WrapError(err, "context")

		structuredWrapped, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}

		if structuredWrapped.Type != ValidationError {
			t.Error("wrapped error should preserve type")
		}
	})

	t.Run("WrapErrorWithContext", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "request_id", "12345")
		err := errors.New("test error")
		wrapped := wrapper.WrapErrorWithContext(ctx, err, "context")

		if wrapped == nil {
			t.Fatal("WrapErrorWithContext should return error")
		}
	})

	t.Run("WrapErrorWithContext with nil error", func(t *testing.T) {
		ctx := context.Background()
		wrapped := wrapper.WrapErrorWithContext(ctx, nil, "context")

		if wrapped != nil {
			t.Error("WrapErrorWithContext(nil) should return nil")
		}
	})

	t.Run("WrapErrorWithContext with structured error", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "user_id", "user123")
		err := &StructuredError{Type: ConfigError, Message: "config error"}
		wrapped := wrapper.WrapErrorWithContext(ctx, err, "additional context")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.Type != ConfigError {
			t.Error("wrapped error should preserve type")
		}
	})

	t.Run("WrapErrorWithType", func(t *testing.T) {
		err := errors.New("test error")
		wrapped := wrapper.WrapErrorWithType(err, ConfigError, "config failed")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}

		if structuredErr.Type != ConfigError {
			t.Errorf("error type = %v, want %v", structuredErr.Type, ConfigError)
		}
	})

	t.Run("WrapErrorWithType with nil", func(t *testing.T) {
		wrapped := wrapper.WrapErrorWithType(nil, ConfigError, "config failed")

		if wrapped != nil {
			t.Error("WrapErrorWithType(nil) should return nil")
		}
	})

	t.Run("WrapErrorWithType with structured error", func(t *testing.T) {
		err := &StructuredError{Type: ValidationError, Message: "validation error"}
		wrapped := wrapper.WrapErrorWithType(err, ConfigError, "wrapped as config error")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.Type != ConfigError {
			t.Error("wrapped error should have new type")
		}
	})

	t.Run("UnwrapError", func(t *testing.T) {
		original := errors.New("original")
		wrapped := fmt.Errorf("wrapped: %w", original)

		unwrapped := wrapper.UnwrapError(wrapped)
		if unwrapped != original {
			t.Error("UnwrapError should return original error")
		}
	})

	t.Run("UnwrapError with nil", func(t *testing.T) {
		unwrapped := wrapper.UnwrapError(nil)
		if unwrapped != nil {
			t.Error("UnwrapError(nil) should return nil")
		}
	})

	t.Run("UnwrapError with non-wrapped error", func(t *testing.T) {
		err := errors.New("simple error")
		unwrapped := wrapper.UnwrapError(err)
		// UnwrapError returns the error itself if it doesn't have an Unwrap method
		if unwrapped != err {
			t.Error("UnwrapError should return the error itself for non-wrapped errors")
		}
	})
}

// TestCreateHelperFunctions verifies error creation helper functions
func TestCreateHelperFunctions(t *testing.T) {
	t.Run("CreateValidationError", func(t *testing.T) {
		err := CreateValidationError("field", "message", "suggestion")
		if err.Type != ValidationError {
			t.Errorf("error type = %v, want %v", err.Type, ValidationError)
		}
		if err.Field != "field" {
			t.Errorf("error field = %v, want %v", err.Field, "field")
		}
	})

	t.Run("CreatePathError", func(t *testing.T) {
		cause := errors.New("cause")
		err := CreatePathError("/test/path", "path error", cause)
		if err.Type != PathError {
			t.Errorf("error type = %v, want %v", err.Type, PathError)
		}
		if err.Cause != cause {
			t.Error("error should preserve cause")
		}
	})

	t.Run("CreatePermissionError", func(t *testing.T) {
		cause := errors.New("cause")
		err := CreatePermissionError("resource", "read", cause)
		if err.Type != PermissionError {
			t.Errorf("error type = %v, want %v", err.Type, PermissionError)
		}
	})

	t.Run("CreateSOPSError", func(t *testing.T) {
		cause := errors.New("cause")
		err := CreateSOPSError("encrypt", "encryption failed", cause)
		if err.Type != SOPSError {
			t.Errorf("error type = %v, want %v", err.Type, SOPSError)
		}
	})

	t.Run("CreateConfigError", func(t *testing.T) {
		cause := errors.New("cause")
		err := CreateConfigError("field", "config error", cause)
		if err.Type != ConfigError {
			t.Errorf("error type = %v, want %v", err.Type, ConfigError)
		}
	})

	t.Run("CreateCloudError", func(t *testing.T) {
		cause := errors.New("cause")
		err := CreateCloudError("openstack", "create", "creation failed", cause)
		if err.Type != CloudError {
			t.Errorf("error type = %v, want %v", err.Type, CloudError)
		}
		if !err.Retryable {
			t.Error("cloud errors should be retryable")
		}
	})

	t.Run("CreateCredentialError", func(t *testing.T) {
		cause := errors.New("cause")
		err := CreateCredentialError("AWS", "access_key", "invalid credentials", cause)
		if err.Type != CredentialError {
			t.Errorf("error type = %v, want %v", err.Type, CredentialError)
		}
		if err.Retryable {
			t.Error("credential errors should not be retryable")
		}
	})

	t.Run("CreateServiceError", func(t *testing.T) {
		cause := errors.New("cause")
		err := CreateServiceError("prometheus", "install", "installation failed", cause)
		if err.Type != ServiceError {
			t.Errorf("error type = %v, want %v", err.Type, ServiceError)
		}
	})

	t.Run("CreateGenerationError", func(t *testing.T) {
		cause := errors.New("cause")
		err := CreateGenerationError("template-rendering", "template not found", cause)
		if err.Type != GenerationError {
			t.Errorf("error type = %v, want %v", err.Type, GenerationError)
		}
	})
}

// TestMultiFieldAggregator verifies multi-field aggregator functionality
func TestMultiFieldAggregator(t *testing.T) {
	t.Run("AddFieldError", func(t *testing.T) {
		agg := NewMultiFieldAggregator()
		agg.AddFieldError("field1", errors.New("error 1"))
		agg.AddFieldError("field2", errors.New("error 2"))

		if !agg.HasFieldErrors("field1") {
			t.Error("should have errors for field1")
		}
		if !agg.HasFieldErrors("field2") {
			t.Error("should have errors for field2")
		}
		if agg.HasFieldErrors("field3") {
			t.Error("should not have errors for field3")
		}
	})

	t.Run("GetFieldErrors", func(t *testing.T) {
		agg := NewMultiFieldAggregator()
		agg.AddFieldError("field1", errors.New("error 1"))
		agg.AddFieldError("field1", errors.New("error 2"))

		errors := agg.GetFieldErrors("field1")
		if len(errors) != 2 {
			t.Errorf("GetFieldErrors length = %d, want 2", len(errors))
		}
	})

	t.Run("GetAllFieldErrors", func(t *testing.T) {
		agg := NewMultiFieldAggregator()
		agg.AddFieldError("field1", errors.New("error 1"))
		agg.AddFieldError("field2", errors.New("error 2"))
		agg.AddFieldError("field2", errors.New("error 3"))

		allErrors := agg.GetAllFieldErrors()
		if len(allErrors) != 2 {
			t.Errorf("GetAllFieldErrors() should have 2 fields, got %d", len(allErrors))
		}
		if len(allErrors["field1"]) != 1 {
			t.Errorf("field1 should have 1 error, got %d", len(allErrors["field1"]))
		}
		if len(allErrors["field2"]) != 2 {
			t.Errorf("field2 should have 2 errors, got %d", len(allErrors["field2"]))
		}
	})

	t.Run("HasAnyErrors", func(t *testing.T) {
		agg := NewMultiFieldAggregator()
		if agg.HasAnyErrors() {
			t.Error("new aggregator should have no errors")
		}

		agg.AddFieldError("field1", errors.New("error"))
		if !agg.HasAnyErrors() {
			t.Error("aggregator should have errors")
		}
	})

	t.Run("ToError", func(t *testing.T) {
		agg := NewMultiFieldAggregator()
		if agg.ToError() != nil {
			t.Error("ToError() should return nil when no errors")
		}

		agg.AddFieldError("field1", errors.New("error"))
		if agg.ToError() == nil {
			t.Error("ToError() should return error when errors exist")
		}
	})
}

// TestWrapperHelperFunctions verifies wrapper helper functions
func TestWrapperHelperFunctions(t *testing.T) {
	t.Run("WrapWithField", func(t *testing.T) {
		err := errors.New("test error")
		wrapped := WrapWithField(err, "test_field")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.Field != "test_field" {
			t.Errorf("field = %v, want %v", structuredErr.Field, "test_field")
		}
	})

	t.Run("WrapWithField with nil", func(t *testing.T) {
		wrapped := WrapWithField(nil, "test_field")
		if wrapped != nil {
			t.Error("WrapWithField(nil) should return nil")
		}
	})

	t.Run("WrapWithField with structured error", func(t *testing.T) {
		original := &StructuredError{Type: ValidationError, Message: "test"}
		wrapped := WrapWithField(original, "new_field")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.Field != "new_field" {
			t.Errorf("field = %v, want %v", structuredErr.Field, "new_field")
		}
	})

	t.Run("WrapWithSuggestions", func(t *testing.T) {
		err := errors.New("test error")
		wrapped := WrapWithSuggestions(err, "suggestion 1", "suggestion 2")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if len(structuredErr.Suggestions) < 2 {
			t.Error("should have at least 2 suggestions")
		}
	})

	t.Run("WrapWithSuggestions with nil", func(t *testing.T) {
		wrapped := WrapWithSuggestions(nil, "suggestion")
		if wrapped != nil {
			t.Error("WrapWithSuggestions(nil) should return nil")
		}
	})

	t.Run("WrapWithContext", func(t *testing.T) {
		err := errors.New("test error")
		context := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}
		wrapped := WrapWithContext(err, context)

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.Context["key1"] != "value1" {
			t.Error("context should be preserved")
		}
	})

	t.Run("WrapWithContext with nil", func(t *testing.T) {
		context := map[string]interface{}{"key": "value"}
		wrapped := WrapWithContext(nil, context)
		if wrapped != nil {
			t.Error("WrapWithContext(nil) should return nil")
		}
	})

	t.Run("Chain", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		err3 := errors.New("error 3")

		chained := Chain(err1, err2, err3)
		if chained == nil {
			t.Fatal("Chain should return error")
		}
	})

	t.Run("Chain with nil errors", func(t *testing.T) {
		err1 := errors.New("error 1")
		chained := Chain(err1, nil, nil)
		if chained == nil {
			t.Fatal("Chain should return error")
		}
	})

	t.Run("Chain with all nil", func(t *testing.T) {
		chained := Chain(nil, nil, nil)
		if chained != nil {
			t.Error("Chain with all nil should return nil")
		}
	})

	t.Run("Annotate", func(t *testing.T) {
		err := errors.New("test error")
		annotated := Annotate(err, "annotation 1")

		if annotated == nil {
			t.Fatal("Annotate should return error")
		}
	})

	t.Run("Annotate with nil", func(t *testing.T) {
		annotated := Annotate(nil, "annotation")
		if annotated != nil {
			t.Error("Annotate(nil) should return nil")
		}
	})

	t.Run("Annotate with multiple annotations", func(t *testing.T) {
		err := errors.New("test error")
		annotated := Annotate(err, "annotation 1")
		annotated = Annotate(annotated, "annotation 2")
		annotated = Annotate(annotated, "annotation 3")

		if annotated == nil {
			t.Fatal("Annotate should return error")
		}
	})
}

// TestHelperErrorChecks verifies error checking helper functions
func TestHelperErrorChecks(t *testing.T) {
	t.Run("IsFileNotFoundError", func(t *testing.T) {
		// Note: This won't actually be os.ErrNotExist, but we can test the function exists
		if IsFileNotFoundError(nil) {
			t.Error("IsFileNotFoundError(nil) should return false")
		}
	})

	t.Run("IsPermissionError", func(t *testing.T) {
		if IsPermissionError(nil) {
			t.Error("IsPermissionError(nil) should return false")
		}
	})

	t.Run("IsTimeoutError", func(t *testing.T) {
		timeoutErr := errors.New("operation timeout")
		if !IsTimeoutError(timeoutErr) {
			t.Error("IsTimeoutError should detect timeout errors")
		}

		if IsTimeoutError(nil) {
			t.Error("IsTimeoutError(nil) should return false")
		}

		normalErr := errors.New("normal error")
		if IsTimeoutError(normalErr) {
			t.Error("IsTimeoutError should return false for non-timeout errors")
		}
	})
}

// TestFileContextFunctions verifies file context wrapper functions
func TestFileContextFunctions(t *testing.T) {
	t.Run("WrapWithFileContext", func(t *testing.T) {
		err := errors.New("test error")
		wrapped := WrapWithFileContext(err, "/path/to/file.yaml", 42)

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.FilePath != "/path/to/file.yaml" {
			t.Errorf("FilePath = %v, want %v", structuredErr.FilePath, "/path/to/file.yaml")
		}
		if structuredErr.LineNumber != 42 {
			t.Errorf("LineNumber = %v, want %v", structuredErr.LineNumber, 42)
		}
	})

	t.Run("WrapWithFileContext with nil", func(t *testing.T) {
		wrapped := WrapWithFileContext(nil, "/path/to/file.yaml", 42)
		if wrapped != nil {
			t.Error("WrapWithFileContext(nil) should return nil")
		}
	})

	t.Run("WrapWithFileContext with structured error", func(t *testing.T) {
		original := &StructuredError{Type: ValidationError, Message: "test"}
		wrapped := WrapWithFileContext(original, "/path/to/file.yaml", 42)

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.FilePath != "/path/to/file.yaml" {
			t.Error("FilePath should be set")
		}
		if structuredErr.LineNumber != 42 {
			t.Error("LineNumber should be set")
		}
	})

	t.Run("WrapWithFileContextAndColumn", func(t *testing.T) {
		err := errors.New("test error")
		wrapped := WrapWithFileContextAndColumn(err, "/path/to/file.yaml", 42, 15)

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.FilePath != "/path/to/file.yaml" {
			t.Errorf("FilePath = %v, want %v", structuredErr.FilePath, "/path/to/file.yaml")
		}
		if structuredErr.LineNumber != 42 {
			t.Errorf("LineNumber = %v, want %v", structuredErr.LineNumber, 42)
		}
		if structuredErr.ColumnNumber != 15 {
			t.Errorf("ColumnNumber = %v, want %v", structuredErr.ColumnNumber, 15)
		}
	})

	t.Run("WrapWithFileContextAndColumn with nil", func(t *testing.T) {
		wrapped := WrapWithFileContextAndColumn(nil, "/path/to/file.yaml", 42, 15)
		if wrapped != nil {
			t.Error("WrapWithFileContextAndColumn(nil) should return nil")
		}
	})

	t.Run("WrapWithOperation", func(t *testing.T) {
		err := errors.New("test error")
		wrapped := WrapWithOperation(err, "cluster_init")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.Operation != "cluster_init" {
			t.Errorf("Operation = %v, want %v", structuredErr.Operation, "cluster_init")
		}
	})

	t.Run("WrapWithOperation with nil", func(t *testing.T) {
		wrapped := WrapWithOperation(nil, "cluster_init")
		if wrapped != nil {
			t.Error("WrapWithOperation(nil) should return nil")
		}
	})

	t.Run("WrapWithOperation with structured error", func(t *testing.T) {
		original := &StructuredError{Type: ValidationError, Message: "test"}
		wrapped := WrapWithOperation(original, "cluster_init")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.Operation != "cluster_init" {
			t.Error("Operation should be set")
		}
	})

	t.Run("WrapWithFullContext", func(t *testing.T) {
		err := errors.New("test error")
		wrapped := WrapWithFullContext(err, "/path/to/file.yaml", 42, 15, "cluster_init")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}
		if structuredErr.FilePath != "/path/to/file.yaml" {
			t.Errorf("FilePath = %v, want %v", structuredErr.FilePath, "/path/to/file.yaml")
		}
		if structuredErr.LineNumber != 42 {
			t.Errorf("LineNumber = %v, want %v", structuredErr.LineNumber, 42)
		}
		if structuredErr.ColumnNumber != 15 {
			t.Errorf("ColumnNumber = %v, want %v", structuredErr.ColumnNumber, 15)
		}
		if structuredErr.Operation != "cluster_init" {
			t.Errorf("Operation = %v, want %v", structuredErr.Operation, "cluster_init")
		}
	})

	t.Run("WrapWithFullContext with nil", func(t *testing.T) {
		wrapped := WrapWithFullContext(nil, "/path/to/file.yaml", 42, 15, "cluster_init")
		if wrapped != nil {
			t.Error("WrapWithFullContext(nil) should return nil")
		}
	})
}

// TestTemplateErrorCreation verifies template error creation with file context
func TestTemplateErrorCreation(t *testing.T) {
	t.Run("CreateTemplateError", func(t *testing.T) {
		cause := errors.New("syntax error")
		err := CreateTemplateError("/templates/cluster.yaml", 42, "invalid template syntax", cause)

		if err.Type != TemplateError {
			t.Errorf("error type = %v, want %v", err.Type, TemplateError)
		}
		if err.FilePath != "/templates/cluster.yaml" {
			t.Errorf("FilePath = %v, want %v", err.FilePath, "/templates/cluster.yaml")
		}
		if err.LineNumber != 42 {
			t.Errorf("LineNumber = %v, want %v", err.LineNumber, 42)
		}
		if err.Operation != "template_rendering" {
			t.Errorf("Operation = %v, want %v", err.Operation, "template_rendering")
		}
		if err.Cause != cause {
			t.Error("Cause should be preserved")
		}
	})

	t.Run("CreateTemplateErrorWithColumn", func(t *testing.T) {
		cause := errors.New("syntax error")
		err := CreateTemplateErrorWithColumn("/templates/cluster.yaml", 42, 15, "invalid template syntax", cause)

		if err.Type != TemplateError {
			t.Errorf("error type = %v, want %v", err.Type, TemplateError)
		}
		if err.FilePath != "/templates/cluster.yaml" {
			t.Errorf("FilePath = %v, want %v", err.FilePath, "/templates/cluster.yaml")
		}
		if err.LineNumber != 42 {
			t.Errorf("LineNumber = %v, want %v", err.LineNumber, 42)
		}
		if err.ColumnNumber != 15 {
			t.Errorf("ColumnNumber = %v, want %v", err.ColumnNumber, 15)
		}
		if err.Operation != "template_rendering" {
			t.Errorf("Operation = %v, want %v", err.Operation, "template_rendering")
		}
	})
}

// TestGenerationErrorWithFile verifies generation error creation with file context
func TestGenerationErrorWithFile(t *testing.T) {
	t.Run("CreateGenerationError", func(t *testing.T) {
		cause := errors.New("template not found")
		err := CreateGenerationError("template-rendering", "template file missing", cause)

		if err.Type != GenerationError {
			t.Errorf("error type = %v, want %v", err.Type, GenerationError)
		}
		if err.Operation != "gitops_generation" {
			t.Errorf("Operation = %v, want %v", err.Operation, "gitops_generation")
		}
	})

	t.Run("CreateGenerationErrorWithFile", func(t *testing.T) {
		cause := errors.New("template not found")
		err := CreateGenerationErrorWithFile("template-rendering", "/gitops/templates/app.yaml", "template file missing", cause)

		if err.Type != GenerationError {
			t.Errorf("error type = %v, want %v", err.Type, GenerationError)
		}
		if err.FilePath != "/gitops/templates/app.yaml" {
			t.Errorf("FilePath = %v, want %v", err.FilePath, "/gitops/templates/app.yaml")
		}
		if err.Operation != "gitops_generation" {
			t.Errorf("Operation = %v, want %v", err.Operation, "gitops_generation")
		}
	})
}

// TestFormatErrorWithContext verifies error formatting includes file context and operation
func TestFormatErrorWithContext(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	t.Run("FormatError with file context", func(t *testing.T) {
		err := &StructuredError{
			Type:       ValidationError,
			Message:    "invalid value",
			FilePath:   "/config/cluster.yaml",
			LineNumber: 42,
		}
		formatted := handler.FormatError(err)

		if !strings.Contains(formatted, "/config/cluster.yaml") {
			t.Error("formatted error should include file path")
		}
		if !strings.Contains(formatted, ":42") {
			t.Error("formatted error should include line number")
		}
	})

	t.Run("FormatError with file context and column", func(t *testing.T) {
		err := &StructuredError{
			Type:         ValidationError,
			Message:      "invalid value",
			FilePath:     "/config/cluster.yaml",
			LineNumber:   42,
			ColumnNumber: 15,
		}
		formatted := handler.FormatError(err)

		if !strings.Contains(formatted, "/config/cluster.yaml") {
			t.Error("formatted error should include file path")
		}
		if !strings.Contains(formatted, ":42:15") {
			t.Error("formatted error should include line and column numbers")
		}
	})

	t.Run("FormatError with operation", func(t *testing.T) {
		err := &StructuredError{
			Type:      ValidationError,
			Message:   "invalid value",
			Operation: "cluster_init",
		}
		formatted := handler.FormatError(err)

		if !strings.Contains(formatted, "cluster_init") {
			t.Error("formatted error should include operation")
		}
	})

	t.Run("FormatError with full context", func(t *testing.T) {
		err := &StructuredError{
			Type:         ValidationError,
			Field:        "cluster_name",
			Message:      "invalid value",
			FilePath:     "/config/cluster.yaml",
			LineNumber:   42,
			ColumnNumber: 15,
			Operation:    "cluster_init",
			Suggestions:  []string{"Use alphanumeric characters only"},
		}
		formatted := handler.FormatError(err)

		if !strings.Contains(formatted, "cluster_init") {
			t.Error("formatted error should include operation")
		}
		if !strings.Contains(formatted, "/config/cluster.yaml:42:15") {
			t.Error("formatted error should include full file context")
		}
		if !strings.Contains(formatted, "cluster_name") {
			t.Error("formatted error should include field")
		}
		if !strings.Contains(formatted, "Suggestions:") {
			t.Error("formatted error should include suggestions")
		}
	})

	t.Run("FormatError without context", func(t *testing.T) {
		err := &StructuredError{
			Type:    ValidationError,
			Message: "invalid value",
		}
		formatted := handler.FormatError(err)

		if formatted == "" {
			t.Error("formatted error should not be empty")
		}
		// Should still format correctly without context
		if !strings.Contains(formatted, "invalid value") {
			t.Error("formatted error should include message")
		}
	})
}

// TestDetermineErrorType verifies error type detection from error messages
func TestDetermineErrorType(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	tests := []struct {
		name     string
		errMsg   string
		expected ErrorType
	}{
		{"validation error", "validation failed", ValidationError},
		{"invalid error", "invalid input", ValidationError},
		{"path error", "path not found", PathError},
		{"directory error", "directory does not exist", PathError},
		{"file error", "file missing", PathError},
		{"permission error", "permission denied", PermissionError},
		{"access error", "access denied", PermissionError},
		{"template error", "template syntax error", TemplateError},
		{"sops error", "sops encryption failed", SOPSError},
		{"encryption error", "encryption error", SOPSError},
		{"config error", "config loading failed", ConfigError},
		{"network error", "network timeout", NetworkError},
		{"connection error", "connection refused", NetworkError},
		{"cloud error", "cloud API failed", CloudError},
		{"aws error", "aws service error", CloudError},
		{"openstack error", "openstack authentication failed", CloudError},
		{"credential error", "credential expired", CredentialError},
		{"authentication error", "authentication failed", CredentialError},
		{"unauthorized error", "unauthorized request", CredentialError},
		{"service error", "service unavailable", ServiceError},
		{"plugin error", "plugin load failed", ServiceError},
		{"generation error", "generation failed", GenerationError},
		{"gitops error", "gitops setup failed", GenerationError},
		{"workspace error", "workspace creation failed", GenerationError},
		{"system error", "unknown error", SystemError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			errorType := handler.determineErrorType(err)
			if errorType != tt.expected {
				t.Errorf("determineErrorType(%q) = %v, want %v", tt.errMsg, errorType, tt.expected)
			}
		})
	}
}

// TestGetSuggestionsComprehensive verifies suggestions for all error types
func TestGetSuggestionsComprehensive(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	tests := []struct {
		name           string
		errMsg         string
		shouldHaveSugg bool
	}{
		{"yaml syntax error", "invalid yaml syntax", true},
		{"disk full error", "no space left on device", true},
		{"disk full error 2", "disk full", true},
		{"dns error", "dns resolution failed", true},
		{"network interface error", "network interface down", true},
		{"routing error", "routing table error", true},
		{"generic network error", "network error occurred", true},
		{"device busy error", "device busy", true},
		{"resource unavailable error", "resource temporarily unavailable", true},
		{"unknown error", "something went wrong", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			suggestions := handler.GetSuggestions(err)
			if tt.shouldHaveSugg && len(suggestions) == 0 {
				t.Errorf("GetSuggestions(%q) should return suggestions", tt.errMsg)
			}
		})
	}
}

// TestIsRetryableComprehensive verifies retryable detection for various errors
func TestIsRetryableComprehensive(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	tests := []struct {
		name      string
		errMsg    string
		retryable bool
	}{
		{"timeout error", "operation timeout", true},
		{"timed out error", "operation timeout occurred", true},
		{"connection refused", "connection refused", true},
		{"network error", "network unreachable", true},
		{"temporary error", "temporary failure", true},
		{"resource unavailable", "resource temporarily unavailable", true},
		{"device busy", "device busy", true},
		{"invalid input", "invalid configuration", false},
		{"permission denied", "permission denied", false},
		{"access denied", "access denied", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			retryable := handler.IsRetryable(err)
			if retryable != tt.retryable {
				t.Errorf("IsRetryable(%q) = %v, want %v", tt.errMsg, retryable, tt.retryable)
			}
		})
	}
}

// TestErrorAggregatorEdgeCases verifies edge cases in error aggregation
func TestErrorAggregatorEdgeCases(t *testing.T) {
	t.Run("GetErrorsByType with regular errors", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		agg.AddError(errors.New("validation failed"))
		agg.AddError(errors.New("config error"))

		validationErrors := agg.GetErrorsByType(ValidationError)
		if len(validationErrors) == 0 {
			t.Error("should detect validation errors from regular errors")
		}
	})

	t.Run("ToError with multiple errors", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		agg.AddError(errors.New("error 1"))
		agg.AddError(errors.New("error 2"))
		agg.AddError(errors.New("error 3"))

		err := agg.ToError()
		if err == nil {
			t.Fatal("ToError should return error")
		}

		if _, ok := err.(*ErrorCollection); !ok {
			t.Error("ToError with multiple errors should return ErrorCollection")
		}
	})

	t.Run("GetSummary with many errors", func(t *testing.T) {
		agg := NewDefaultErrorAggregator()
		for i := 0; i < 10; i++ {
			agg.AddError(&StructuredError{
				Type:    ValidationError,
				Message: fmt.Sprintf("validation error %d", i),
			})
		}

		summary := agg.GetSummary()
		if !strings.Contains(summary, "and") {
			t.Error("summary should indicate there are more errors")
		}
	})
}

// TestValidationResultEdgeCases verifies edge cases in validation results
func TestValidationResultEdgeCases(t *testing.T) {
	t.Run("ToError with multiple errors", func(t *testing.T) {
		vr := &ValidationResult{
			Valid: false,
			Errors: []*StructuredError{
				{Type: ValidationError, Message: "error 1"},
				{Type: ValidationError, Message: "error 2"},
				{Type: ValidationError, Message: "error 3"},
			},
		}

		err := vr.ToError()
		if err == nil {
			t.Fatal("ToError should return error")
		}

		if _, ok := err.(*ErrorCollection); !ok {
			t.Error("ToError with multiple errors should return ErrorCollection")
		}
	})
}

// TestValidationAggregatorEdgeCases verifies edge cases in validation aggregation
func TestValidationAggregatorEdgeCases(t *testing.T) {
	t.Run("ToValidationResult with regular errors", func(t *testing.T) {
		agg := NewValidationAggregator()
		agg.AddError(errors.New("validation error"))
		agg.AddWarning(errors.New("warning"))

		result := agg.ToValidationResult()
		if len(result.Errors) == 0 {
			t.Error("should convert regular errors to structured errors")
		}
		if len(result.Warnings) == 0 {
			t.Error("should convert regular warnings to structured errors")
		}
	})
}

// TestMultiFieldAggregatorEdgeCases verifies edge cases in multi-field aggregation
func TestMultiFieldAggregatorEdgeCases(t *testing.T) {
	t.Run("AddFieldError with nil", func(t *testing.T) {
		agg := NewMultiFieldAggregator()
		agg.AddFieldError("field1", nil)

		if agg.HasFieldErrors("field1") {
			t.Error("should not add nil errors")
		}
	})

	t.Run("GetFieldErrors for non-existent field", func(t *testing.T) {
		agg := NewMultiFieldAggregator()
		errors := agg.GetFieldErrors("non_existent")

		if errors != nil {
			t.Error("should return nil for non-existent field")
		}
	})

	t.Run("ToError with single error", func(t *testing.T) {
		agg := NewMultiFieldAggregator()
		agg.AddFieldError("field1", errors.New("error"))

		err := agg.ToError()
		if err == nil {
			t.Fatal("ToError should return error")
		}

		// Single error should not be wrapped in ErrorCollection
		if _, ok := err.(*ErrorCollection); ok {
			t.Error("ToError with single error should not return ErrorCollection")
		}
	})
}

// TestErrorWrapperEdgeCases verifies edge cases in error wrapping
func TestErrorWrapperEdgeCases(t *testing.T) {
	wrapper := NewDefaultErrorWrapper()

	t.Run("WrapErrorWithContext with nil context", func(t *testing.T) {
		err := errors.New("test error")
		wrapped := wrapper.WrapErrorWithContext(nil, err, "context")

		if wrapped == nil {
			t.Fatal("should wrap error even with nil context")
		}
	})

	t.Run("WrapErrorWithType preserves structured error properties", func(t *testing.T) {
		original := &StructuredError{
			Type:        ValidationError,
			Field:       "test_field",
			Message:     "original message",
			Context:     map[string]interface{}{"key": "value"},
			Retryable:   true,
			Suggestions: []string{"original suggestion"},
		}

		wrapped := wrapper.WrapErrorWithType(original, ConfigError, "wrapped message")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}

		if structuredErr.Field != "test_field" {
			t.Error("should preserve field")
		}
		if structuredErr.Context["key"] != "value" {
			t.Error("should preserve context")
		}
		if structuredErr.Retryable != true {
			t.Error("should preserve retryable flag")
		}
		if len(structuredErr.Suggestions) == 0 {
			t.Error("should preserve suggestions")
		}
	})

	t.Run("UnwrapError with non-unwrappable error", func(t *testing.T) {
		err := errors.New("simple error")
		unwrapped := wrapper.UnwrapError(err)

		if unwrapped != err {
			t.Error("should return same error for non-unwrappable errors")
		}
	})
}

// TestWrapperHelperEdgeCases verifies edge cases in wrapper helper functions
func TestWrapperHelperEdgeCases(t *testing.T) {
	t.Run("WrapWithSuggestions with structured error", func(t *testing.T) {
		original := &StructuredError{
			Type:        ValidationError,
			Message:     "test",
			Suggestions: []string{"original suggestion"},
		}

		wrapped := WrapWithSuggestions(original, "new suggestion 1", "new suggestion 2")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}

		if len(structuredErr.Suggestions) < 3 {
			t.Error("should append new suggestions to existing ones")
		}
	})

	t.Run("WrapWithContext with structured error", func(t *testing.T) {
		original := &StructuredError{
			Type:    ValidationError,
			Message: "test",
			Context: map[string]interface{}{"existing": "value"},
		}

		newContext := map[string]interface{}{
			"new_key": "new_value",
		}

		wrapped := WrapWithContext(original, newContext)

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}

		if structuredErr.Context["existing"] != "value" {
			t.Error("should preserve existing context")
		}
		if structuredErr.Context["new_key"] != "new_value" {
			t.Error("should add new context")
		}
	})

	t.Run("WrapWithContext with regular error", func(t *testing.T) {
		err := errors.New("test error")
		context := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}

		wrapped := WrapWithContext(err, context)

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}

		if structuredErr.Context["key1"] != "value1" {
			t.Error("should set context")
		}
	})

	t.Run("Annotate with structured error multiple times", func(t *testing.T) {
		original := &StructuredError{
			Type:    ValidationError,
			Message: "test",
			Context: map[string]interface{}{
				"annotations": []string{"annotation 1"},
			},
		}

		annotated := Annotate(original, "annotation 2")
		annotated = Annotate(annotated, "annotation 3")

		structuredErr, ok := annotated.(*StructuredError)
		if !ok {
			t.Fatal("annotated error should be StructuredError")
		}

		annotations, ok := structuredErr.Context["annotations"].([]string)
		if !ok {
			t.Fatal("annotations should be string slice")
		}

		if len(annotations) != 3 {
			t.Errorf("should have 3 annotations, got %d", len(annotations))
		}
	})

	t.Run("Annotate with regular error", func(t *testing.T) {
		err := errors.New("test error")
		annotated := Annotate(err, "annotation")

		if annotated == nil {
			t.Fatal("annotated error should not be nil")
		}

		if !strings.Contains(annotated.Error(), "annotation") {
			t.Error("annotated error should contain annotation")
		}
	})

	t.Run("WrapWithFileContextAndColumn with structured error", func(t *testing.T) {
		original := &StructuredError{
			Type:    ValidationError,
			Message: "test",
		}

		wrapped := WrapWithFileContextAndColumn(original, "/path/file.yaml", 10, 5)

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}

		if structuredErr.FilePath != "/path/file.yaml" {
			t.Error("should set file path")
		}
		if structuredErr.LineNumber != 10 {
			t.Error("should set line number")
		}
		if structuredErr.ColumnNumber != 5 {
			t.Error("should set column number")
		}
	})

	t.Run("WrapWithFullContext with structured error", func(t *testing.T) {
		original := &StructuredError{
			Type:    ValidationError,
			Message: "test",
		}

		wrapped := WrapWithFullContext(original, "/path/file.yaml", 10, 5, "test_operation")

		structuredErr, ok := wrapped.(*StructuredError)
		if !ok {
			t.Fatal("wrapped error should be StructuredError")
		}

		if structuredErr.FilePath != "/path/file.yaml" {
			t.Error("should set file path")
		}
		if structuredErr.LineNumber != 10 {
			t.Error("should set line number")
		}
		if structuredErr.ColumnNumber != 5 {
			t.Error("should set column number")
		}
		if structuredErr.Operation != "test_operation" {
			t.Error("should set operation")
		}
	})
}

// TestFormatErrorEdgeCases verifies edge cases in error formatting
func TestFormatErrorEdgeCases(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	t.Run("FormatError with UserError type", func(t *testing.T) {
		err := &StructuredError{
			Type:    UserError,
			Message: "user error message",
		}
		formatted := handler.FormatError(err)

		// UserError should not include type prefix
		if strings.Contains(formatted, "[USER]") {
			t.Error("UserError should not include type prefix")
		}
		if !strings.Contains(formatted, "user error message") {
			t.Error("should include message")
		}
	})

	t.Run("FormatError with file path but no line number", func(t *testing.T) {
		err := &StructuredError{
			Type:     ValidationError,
			Message:  "test error",
			FilePath: "/path/to/file.yaml",
		}
		formatted := handler.FormatError(err)

		if !strings.Contains(formatted, "/path/to/file.yaml") {
			t.Error("should include file path")
		}
	})
}

// TestGetSuggestionsAllErrorTypes verifies suggestions for all error message patterns
func TestGetSuggestionsAllErrorTypes(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	tests := []struct {
		name   string
		errMsg string
	}{
		{"permission with directory", "permission denied on directory"},
		{"file not found with config", "config file not found"},
		{"yaml validation", "invalid yaml syntax"},
		{"connection with systemctl", "connection refused to service"},
		{"timeout with ping", "operation timeout on network"},
		{"sops with age", "sops age key error"},
		{"template with variables", "template variable undefined"},
		{"openstack cloud", "openstack API error"},
		{"aws cloud", "aws credentials error"},
		{"cloud generic", "cloud provider error"},
		{"config edit", "config validation failed"},
		{"service dependency", "service dependency missing"},
		{"gitops workspace", "gitops workspace error"},
		{"disk space", "no space left on device"},
		{"network dns", "dns resolution failed"},
		{"network interface", "network interface error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			suggestions := handler.GetSuggestions(err)
			if len(suggestions) == 0 {
				t.Errorf("GetSuggestions(%q) should return suggestions", tt.errMsg)
			}
		})
	}
}

// TestAnnotateWithNilContext verifies Annotate with nil context
func TestAnnotateWithNilContext(t *testing.T) {
	original := &StructuredError{
		Type:    ValidationError,
		Message: "test",
		Context: nil,
	}

	annotated := Annotate(original, "first annotation")

	structuredErr, ok := annotated.(*StructuredError)
	if !ok {
		t.Fatal("annotated error should be StructuredError")
	}

	if structuredErr.Context == nil {
		t.Fatal("context should be initialized")
	}

	annotations, ok := structuredErr.Context["annotations"].([]string)
	if !ok {
		t.Fatal("annotations should be string slice")
	}

	if len(annotations) != 1 {
		t.Errorf("should have 1 annotation, got %d", len(annotations))
	}
}

// TestWrapWithContextNilContext verifies WrapWithContext with nil context on structured error
func TestWrapWithContextNilContext(t *testing.T) {
	original := &StructuredError{
		Type:    ValidationError,
		Message: "test",
		Context: nil,
	}

	newContext := map[string]interface{}{
		"key": "value",
	}

	wrapped := WrapWithContext(original, newContext)

	structuredErr, ok := wrapped.(*StructuredError)
	if !ok {
		t.Fatal("wrapped error should be StructuredError")
	}

	if structuredErr.Context == nil {
		t.Fatal("context should be initialized")
	}

	if structuredErr.Context["key"] != "value" {
		t.Error("should set context")
	}
}

// TestToValidationResultWithStructuredErrors verifies ToValidationResult with structured errors
func TestToValidationResultWithStructuredErrors(t *testing.T) {
	agg := NewValidationAggregator()

	// Add structured errors
	agg.AddError(&StructuredError{Type: ValidationError, Message: "error 1"})
	agg.AddError(&StructuredError{Type: ConfigError, Message: "error 2"})

	// Add structured warnings
	agg.AddWarning(&StructuredError{Type: ValidationError, Message: "warning 1"})
	agg.AddWarning(&StructuredError{Type: ConfigError, Message: "warning 2"})

	result := agg.ToValidationResult()

	if len(result.Errors) != 2 {
		t.Errorf("should have 2 errors, got %d", len(result.Errors))
	}

	if len(result.Warnings) != 2 {
		t.Errorf("should have 2 warnings, got %d", len(result.Warnings))
	}
}

// TestMultiFieldAggregatorToErrorMultiple verifies ToError with multiple field errors
func TestMultiFieldAggregatorToErrorMultiple(t *testing.T) {
	agg := NewMultiFieldAggregator()
	agg.AddFieldError("field1", errors.New("error 1"))
	agg.AddFieldError("field2", errors.New("error 2"))
	agg.AddFieldError("field3", errors.New("error 3"))

	err := agg.ToError()
	if err == nil {
		t.Fatal("ToError should return error")
	}

	if _, ok := err.(*ErrorCollection); !ok {
		t.Error("ToError with multiple errors should return ErrorCollection")
	}
}

// TestWrapErrorWithContextWithContextValues verifies WrapErrorWithContext with context values
func TestWrapErrorWithContextWithContextValues(t *testing.T) {
	wrapper := NewDefaultErrorWrapper()

	ctx := context.WithValue(context.Background(), "request_id", "req-123")
	ctx = context.WithValue(ctx, "user_id", "user-456")
	ctx = context.WithValue(ctx, "operation", "test_op")

	err := &StructuredError{
		Type:    ValidationError,
		Message: "test error",
		Context: nil,
	}

	wrapped := wrapper.WrapErrorWithContext(ctx, err, "additional context")

	structuredErr, ok := wrapped.(*StructuredError)
	if !ok {
		t.Fatal("wrapped error should be StructuredError")
	}

	if structuredErr.Context["request_id"] != "req-123" {
		t.Error("should include request_id from context")
	}
	if structuredErr.Context["user_id"] != "user-456" {
		t.Error("should include user_id from context")
	}
	if structuredErr.Context["operation"] != "test_op" {
		t.Error("should include operation from context")
	}
}

// TestUnwrapErrorWithStructuredError verifies UnwrapError with structured error
func TestUnwrapErrorWithStructuredError(t *testing.T) {
	wrapper := NewDefaultErrorWrapper()

	cause := errors.New("original cause")
	err := &StructuredError{
		Type:    ValidationError,
		Message: "wrapped error",
		Cause:   cause,
	}

	unwrapped := wrapper.UnwrapError(err)
	if unwrapped != cause {
		t.Error("should unwrap to original cause")
	}
}

// TestIsRetryableEdgeCases verifies edge cases in IsRetryable
func TestIsRetryableEdgeCases(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	t.Run("nil error", func(t *testing.T) {
		if handler.IsRetryable(nil) {
			t.Error("nil error should not be retryable")
		}
	})

	t.Run("empty error message", func(t *testing.T) {
		err := errors.New("")
		// Should not panic and should return false
		if handler.IsRetryable(err) {
			t.Error("empty error should not be retryable")
		}
	})
}

// TestDetermineErrorTypeEdgeCases verifies edge cases in determineErrorType
func TestDetermineErrorTypeEdgeCases(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	t.Run("nil error", func(t *testing.T) {
		errorType := handler.determineErrorType(nil)
		if errorType != SystemError {
			t.Errorf("nil error should return SystemError, got %v", errorType)
		}
	})

	t.Run("empty error message", func(t *testing.T) {
		err := errors.New("")
		errorType := handler.determineErrorType(err)
		if errorType != SystemError {
			t.Errorf("empty error should return SystemError, got %v", errorType)
		}
	})
}

// TestFormatErrorWithNilError verifies FormatError with nil error
func TestFormatErrorWithNilError(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	formatted := handler.FormatError(nil)
	if formatted != "" {
		t.Error("FormatError(nil) should return empty string")
	}
}

// TestGetSuggestionsWithStructuredError verifies GetSuggestions with structured error
func TestGetSuggestionsWithStructuredError(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	err := &StructuredError{
		Type:        ValidationError,
		Message:     "test error",
		Suggestions: []string{"existing suggestion 1", "existing suggestion 2"},
	}

	suggestions := handler.GetSuggestions(err)
	if len(suggestions) < 2 {
		t.Error("should return existing suggestions from structured error")
	}

	// Check that existing suggestions are preserved
	found := false
	for _, s := range suggestions {
		if s == "existing suggestion 1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should preserve existing suggestions")
	}
}

// TestGetSuggestionsWithNilError verifies GetSuggestions with nil error
func TestGetSuggestionsWithNilError(t *testing.T) {
	handler := NewDefaultErrorHandlerWithoutMasking()

	suggestions := handler.GetSuggestions(nil)
	if suggestions != nil {
		t.Error("GetSuggestions(nil) should return nil")
	}
}
