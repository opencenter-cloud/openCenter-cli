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
	"fmt"
	"runtime/debug"

	"github.com/rackerlabs/openCenter-cli/internal/security"
)

// ErrorMiddleware provides consistent error handling across all commands
// Requirements: 21.5
type ErrorMiddleware struct {
	handler *DefaultErrorHandler
	masker  security.CredentialMasker
	logger  Logger
}

// Logger interface for logging errors
type Logger interface {
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// NewErrorMiddleware creates a new error middleware
func NewErrorMiddleware(logger Logger) *ErrorMiddleware {
	return &ErrorMiddleware{
		handler: NewDefaultErrorHandler(),
		masker:  security.NewDefaultCredentialMasker(),
		logger:  logger,
	}
}

// Handle wraps a command function with error handling middleware
// Provides: panic recovery, credential masking, logging
// Requirements: 21.5
func (m *ErrorMiddleware) Handle(ctx context.Context, operation string, fn func() error) error {
	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			maskedStack := m.masker.MaskString(stack)
			
			m.logger.Error("Command panic recovered",
				"operation", operation,
				"panic", r,
				"stack", maskedStack,
			)
			
			// Create a structured error for the panic
			err := &StructuredError{
				Type:      SystemError,
				Message:   fmt.Sprintf("panic recovered: %v", r),
				Operation: operation,
				Context: map[string]interface{}{
					"panic": r,
				},
				Retryable: false,
			}
			
			// Add correlation ID if available
			if correlationID := ctx.Value("correlation_id"); correlationID != nil {
				err.Context["correlation_id"] = correlationID
			}
		}
	}()
	
	// Execute the function
	err := fn()
	if err != nil {
		// Handle the error
		return m.HandleError(ctx, operation, err)
	}
	
	return nil
}

// HandleError processes an error with masking and logging
// Requirements: 21.5
func (m *ErrorMiddleware) HandleError(ctx context.Context, operation string, err error) error {
	if err == nil {
		return nil
	}
	
	// Convert to structured error if needed
	structuredErr := m.handler.HandleError(err)
	
	// Add operation context
	if structuredErr.Operation == "" {
		structuredErr.Operation = operation
	}
	
	// Add correlation ID from context if available
	if structuredErr.Context == nil {
		structuredErr.Context = make(map[string]interface{})
	}
	if correlationID := ctx.Value("correlation_id"); correlationID != nil {
		structuredErr.Context["correlation_id"] = correlationID
	}
	
	// Mask credentials in the error
	structuredErr.Message = m.masker.MaskString(structuredErr.Message)
	
	// Log the error
	m.logError(ctx, structuredErr)
	
	return structuredErr
}

// logError logs a structured error with appropriate level
func (m *ErrorMiddleware) logError(ctx context.Context, err *StructuredError) {
	fields := []interface{}{
		"type", err.Type,
		"operation", err.Operation,
		"retryable", err.Retryable,
	}
	
	if err.Field != "" {
		fields = append(fields, "field", err.Field)
	}
	
	if err.FilePath != "" {
		fields = append(fields, "file", err.FilePath)
		if err.LineNumber > 0 {
			fields = append(fields, "line", err.LineNumber)
		}
	}
	
	// Add correlation ID if available
	if correlationID := ctx.Value("correlation_id"); correlationID != nil {
		fields = append(fields, "correlation_id", correlationID)
	}
	
	// Add context fields
	for key, value := range err.Context {
		fields = append(fields, key, value)
	}
	
	// Log at appropriate level based on error type
	switch err.Type {
	case ValidationError, UserError:
		m.logger.Warn(err.Message, fields...)
	case SystemError, NetworkError, CloudError:
		m.logger.Error(err.Message, fields...)
	default:
		m.logger.Error(err.Message, fields...)
	}
}

// WrapCommand wraps a command function with full error handling
// This is a convenience function for use in Cobra commands
// Requirements: 21.5
func (m *ErrorMiddleware) WrapCommand(operation string, fn func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return m.Handle(ctx, operation, func() error {
			return fn(ctx)
		})
	}
}

// WrapCommandWithArgs wraps a command function that takes arguments
// Requirements: 21.5
func (m *ErrorMiddleware) WrapCommandWithArgs(operation string, fn func(ctx context.Context, args []string) error) func(ctx context.Context, args []string) error {
	return func(ctx context.Context, args []string) error {
		return m.Handle(ctx, operation, func() error {
			return fn(ctx, args)
		})
	}
}

// RecoverPanic recovers from a panic and converts it to an error
// This can be used in goroutines or other contexts where defer is needed
// Requirements: 21.5
func (m *ErrorMiddleware) RecoverPanic(ctx context.Context, operation string) error {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		maskedStack := m.masker.MaskString(stack)
		
		m.logger.Error("Panic recovered",
			"operation", operation,
			"panic", r,
			"stack", maskedStack,
		)
		
		return &StructuredError{
			Type:      SystemError,
			Message:   fmt.Sprintf("panic recovered: %v", r),
			Operation: operation,
			Context: map[string]interface{}{
				"panic": r,
			},
			Retryable: false,
		}
	}
	return nil
}

// HandleValidationErrors handles multiple validation errors
// Requirements: 21.5
func (m *ErrorMiddleware) HandleValidationErrors(ctx context.Context, operation string, errs []*StructuredError) error {
	if len(errs) == 0 {
		return nil
	}
	
	// Mask all error messages
	for _, err := range errs {
		err.Message = m.masker.MaskString(err.Message)
		if err.Operation == "" {
			err.Operation = operation
		}
	}
	
	// Log validation errors
	m.logger.Warn("Validation errors",
		"operation", operation,
		"error_count", len(errs),
	)
	
	// Return as error collection
	var errors []error
	for _, err := range errs {
		errors = append(errors, err)
	}
	
	return &ErrorCollection{Errors: errors}
}

// IsRetryable checks if an error is retryable
// Requirements: 21.5
func (m *ErrorMiddleware) IsRetryable(err error) bool {
	return m.handler.IsRetryable(err)
}

// GetSuggestions gets fix suggestions for an error
// Requirements: 21.5
func (m *ErrorMiddleware) GetSuggestions(err error) []string {
	return m.handler.GetSuggestions(err)
}
