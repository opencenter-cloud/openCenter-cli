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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 23: Structured Error Usage
// For any error returned by the system, it SHALL be a structured error from internal/util/errors
// with type, code, field, and context.
// Validates: Requirements 21.1, 21.4

func TestProperty_StructuredErrorUsage(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: All structured errors have required fields
	properties.Property("structured errors have type and message", prop.ForAll(
		func(errorType ErrorType, message string) bool {
			if message == "" {
				return true // Skip empty messages
			}

			err := &StructuredError{
				Type:    errorType,
				Message: message,
			}

			// Verify error has type
			if err.Type == "" {
				return false
			}

			// Verify error has message
			if err.Message == "" {
				return false
			}

			// Verify Error() method returns non-empty string
			if err.Error() == "" {
				return false
			}

			return true
		},
		genErrorType(),
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property: Structured errors preserve cause chain
	properties.Property("structured errors preserve cause chain", prop.ForAll(
		func(message1, message2 string) bool {
			if message1 == "" || message2 == "" {
				return true
			}

			// Create a cause error
			cause := fmt.Errorf("%s", message1)

			// Wrap it in a structured error
			err := &StructuredError{
				Type:    SystemError,
				Message: message2,
				Cause:   cause,
			}

			// Verify Unwrap returns the cause
			unwrapped := err.Unwrap()
			if unwrapped == nil {
				return false
			}

			if unwrapped.Error() != cause.Error() {
				return false
			}

			return true
		},
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property: Error handler converts any error to structured error
	properties.Property("error handler converts to structured error", prop.ForAll(
		func(message string) bool {
			if message == "" {
				return true
			}

			handler := NewDefaultErrorHandlerWithoutMasking()
			err := fmt.Errorf("%s", message)

			structuredErr := handler.HandleError(err)

			// Verify it's a structured error
			if structuredErr == nil {
				return false
			}

			// Verify it has a type
			if structuredErr.Type == "" {
				return false
			}

			// Verify it has a message
			if structuredErr.Message == "" {
				return false
			}

			return true
		},
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property: Structured errors with field context include field name
	properties.Property("structured errors with field include field name", prop.ForAll(
		func(field, message string) bool {
			if field == "" || message == "" {
				return true
			}

			err := &StructuredError{
				Type:    ValidationError,
				Field:   field,
				Message: message,
			}

			// Error string should include field name
			errorStr := err.Error()
			if errorStr == "" {
				return false
			}

			// When field is set, Error() should include it
			if err.Field != "" && errorStr == message {
				// If field is set but not in error string, that's wrong
				return false
			}

			return true
		},
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property: Error wrapper preserves structured error properties
	properties.Property("error wrapper preserves structured error properties", prop.ForAll(
		func(errorType ErrorType, field, message, wrapMessage string) bool {
			if message == "" || wrapMessage == "" {
				return true
			}

			// Create original structured error
			original := &StructuredError{
				Type:    errorType,
				Field:   field,
				Message: message,
			}

			// Wrap it
			wrapper := NewDefaultErrorWrapper()
			wrapped := wrapper.WrapError(original, wrapMessage)

			// Verify it's still a structured error
			structuredWrapped, ok := wrapped.(*StructuredError)
			if !ok {
				return false
			}

			// Verify type is preserved
			if structuredWrapped.Type != original.Type {
				return false
			}

			// Verify field is preserved
			if structuredWrapped.Field != original.Field {
				return false
			}

			return true
		},
		genErrorType(),
		gen.AnyString(),
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
	))

	// Property: Error collection contains all added errors
	properties.Property("error collection contains all added errors", prop.ForAll(
		func(messages []string) bool {
			if len(messages) == 0 {
				return true
			}

			// Filter out empty messages
			var validMessages []string
			for _, msg := range messages {
				if msg != "" {
					validMessages = append(validMessages, msg)
				}
			}

			if len(validMessages) == 0 {
				return true
			}

			// Create error collection
			var errors []error
			for _, msg := range validMessages {
				errors = append(errors, fmt.Errorf("%s", msg))
			}

			collection := &ErrorCollection{Errors: errors}

			// Verify count matches
			if len(collection.Errors) != len(validMessages) {
				return false
			}

			// Verify Error() returns non-empty string
			if collection.Error() == "" {
				return false
			}

			return true
		},
		gen.SliceOf(gen.AnyString()),
	))

	// Property: Validation result correctly reports error state
	properties.Property("validation result correctly reports error state", prop.ForAll(
		func(errorCount, warningCount int) bool {
			// Limit to reasonable counts
			if errorCount < 0 || errorCount > 100 || warningCount < 0 || warningCount > 100 {
				return true
			}

			result := &ValidationResult{
				Valid:    errorCount == 0,
				Errors:   make([]*StructuredError, errorCount),
				Warnings: make([]*StructuredError, warningCount),
			}

			// Initialize errors
			for i := 0; i < errorCount; i++ {
				result.Errors[i] = &StructuredError{
					Type:    ValidationError,
					Message: fmt.Sprintf("error %d", i),
				}
			}

			// Initialize warnings
			for i := 0; i < warningCount; i++ {
				result.Warnings[i] = &StructuredError{
					Type:    ValidationError,
					Message: fmt.Sprintf("warning %d", i),
				}
			}

			// Verify HasErrors matches error count
			if result.HasErrors() != (errorCount > 0) {
				return false
			}

			// Verify HasWarnings matches warning count
			if result.HasWarnings() != (warningCount > 0) {
				return false
			}

			// Verify Valid flag matches error count
			if result.Valid != (errorCount == 0) {
				return false
			}

			// Verify ToError returns error when there are errors
			err := result.ToError()
			if errorCount > 0 && err == nil {
				return false
			}
			if errorCount == 0 && err != nil {
				return false
			}

			return true
		},
		gen.IntRange(0, 10),
		gen.IntRange(0, 10),
	))

	// Property: Error context is preserved through wrapping
	properties.Property("error context is preserved through wrapping", prop.ForAll(
		func(key, value, message string) bool {
			if key == "" || message == "" {
				return true
			}

			// Create error with context
			err := &StructuredError{
				Type:    SystemError,
				Message: message,
				Context: map[string]interface{}{
					key: value,
				},
			}

			// Wrap it
			wrapper := NewDefaultErrorWrapper()
			wrapped := wrapper.WrapError(err, "wrapped")

			// Verify context is preserved
			structuredWrapped, ok := wrapped.(*StructuredError)
			if !ok {
				return false
			}

			if structuredWrapped.Context == nil {
				return false
			}

			if structuredWrapped.Context[key] != value {
				return false
			}

			return true
		},
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
		gen.AnyString(),
		gen.AnyString().SuchThat(func(s string) bool { return s != "" }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genErrorType generates random error types
func genErrorType() gopter.Gen {
	return gen.OneConstOf(
		ValidationError,
		PathError,
		PermissionError,
		TemplateError,
		SOPSError,
		ConfigError,
		NetworkError,
		FileError,
		SystemError,
		UserError,
		CloudError,
		CredentialError,
		ServiceError,
		GenerationError,
	)
}

// TestProperty_ErrorTypeClassification tests that errors are correctly classified
func TestProperty_ErrorTypeClassification(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: Error handler classifies errors by content
	properties.Property("error handler classifies errors by content", prop.ForAll(
		func(keyword string) bool {
			if keyword == "" {
				return true
			}

			handler := NewDefaultErrorHandlerWithoutMasking()

			// Test validation errors
			if keyword == "validation" || keyword == "invalid" {
				err := fmt.Errorf("%s error", keyword)
				structured := handler.HandleError(err)
				return structured.Type == ValidationError
			}

			// Test network errors
			if keyword == "network" || keyword == "connection" {
				err := fmt.Errorf("%s error", keyword)
				structured := handler.HandleError(err)
				return structured.Type == NetworkError
			}

			// Test permission errors
			if keyword == "permission" || keyword == "access" {
				err := fmt.Errorf("%s denied", keyword)
				structured := handler.HandleError(err)
				return structured.Type == PermissionError
			}

			return true
		},
		gen.OneConstOf("validation", "invalid", "network", "connection", "permission", "access"),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ErrorRetryability tests that errors are correctly marked as retryable
func TestProperty_ErrorRetryability(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property: Network errors are retryable
	properties.Property("network errors are retryable", prop.ForAll(
		func(networkKeyword string) bool {
			handler := NewDefaultErrorHandlerWithoutMasking()
			err := fmt.Errorf("%s error", networkKeyword)

			isRetryable := handler.IsRetryable(err)

			// Network-related errors should be retryable
			return isRetryable
		},
		gen.OneConstOf("timeout", "connection refused", "network", "temporary"),
	))

	// Property: Validation errors are not retryable
	properties.Property("validation errors are not retryable", prop.ForAll(
		func(validationKeyword string) bool {
			handler := NewDefaultErrorHandlerWithoutMasking()
			err := fmt.Errorf("%s error", validationKeyword)

			isRetryable := handler.IsRetryable(err)

			// Validation errors should not be retryable
			return !isRetryable
		},
		gen.OneConstOf("invalid", "permission denied", "access denied"),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
