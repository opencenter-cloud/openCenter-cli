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
)

// DefaultErrorWrapper implements ErrorWrapper interface
type DefaultErrorWrapper struct{}

// NewDefaultErrorWrapper creates a new default error wrapper
func NewDefaultErrorWrapper() *DefaultErrorWrapper {
	return &DefaultErrorWrapper{}
}

// WrapError wraps an error with additional context message
func (w *DefaultErrorWrapper) WrapError(err error, message string) error {
	if err == nil {
		return nil
	}

	// If it's already a structured error, preserve its structure
	if structuredErr, ok := err.(*StructuredError); ok {
		return &StructuredError{
			Type:        structuredErr.Type,
			Field:       structuredErr.Field,
			Message:     message + ": " + structuredErr.Message,
			Cause:       structuredErr.Cause,
			Suggestions: structuredErr.Suggestions,
			Context:     structuredErr.Context,
			Retryable:   structuredErr.Retryable,
		}
	}

	return fmt.Errorf("%s: %w", message, err)
}

// WrapErrorWithContext wraps an error with context information
func (w *DefaultErrorWrapper) WrapErrorWithContext(ctx context.Context, err error, message string) error {
	if err == nil {
		return nil
	}

	wrappedErr := w.WrapError(err, message)

	// Add context information if available
	if structuredErr, ok := wrappedErr.(*StructuredError); ok {
		if structuredErr.Context == nil {
			structuredErr.Context = make(map[string]interface{})
		}

		// Add context values if available
		if ctx != nil {
			if requestID := ctx.Value("request_id"); requestID != nil {
				structuredErr.Context["request_id"] = requestID
			}
			if userID := ctx.Value("user_id"); userID != nil {
				structuredErr.Context["user_id"] = userID
			}
			if operation := ctx.Value("operation"); operation != nil {
				structuredErr.Context["operation"] = operation
			}
		}
	}

	return wrappedErr
}

// WrapErrorWithType wraps an error with a specific error type
func (w *DefaultErrorWrapper) WrapErrorWithType(err error, errorType ErrorType, message string) error {
	if err == nil {
		return nil
	}

	// Create a structured error with the specified type
	structuredErr := &StructuredError{
		Type:      errorType,
		Message:   message,
		Cause:     err,
		Retryable: false,
	}

	// If the original error is structured, preserve some of its properties
	if originalStructured, ok := err.(*StructuredError); ok {
		structuredErr.Field = originalStructured.Field
		structuredErr.Context = originalStructured.Context
		structuredErr.Retryable = originalStructured.Retryable

		// Combine suggestions
		structuredErr.Suggestions = append(structuredErr.Suggestions, originalStructured.Suggestions...)
	}

	// Add type-specific suggestions
	handler := NewDefaultErrorHandler()
	typeSuggestions := handler.suggestionMap[errorType]
	structuredErr.Suggestions = append(structuredErr.Suggestions, typeSuggestions...)

	return structuredErr
}

// UnwrapError unwraps an error to get the underlying cause
func (w *DefaultErrorWrapper) UnwrapError(err error) error {
	if err == nil {
		return nil
	}

	// If it's a structured error, return the cause
	if structuredErr, ok := err.(*StructuredError); ok {
		return structuredErr.Cause
	}

	// Use standard unwrapping
	type unwrapper interface {
		Unwrap() error
	}

	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}

	return err
}

// WrapWithField wraps an error with a field context
func WrapWithField(err error, field string) error {
	if err == nil {
		return nil
	}

	if structuredErr, ok := err.(*StructuredError); ok {
		structuredErr.Field = field
		return structuredErr
	}

	return &StructuredError{
		Type:      ValidationError,
		Field:     field,
		Message:   err.Error(),
		Cause:     err,
		Retryable: false,
	}
}

// WrapWithSuggestions wraps an error with additional suggestions
func WrapWithSuggestions(err error, suggestions ...string) error {
	if err == nil {
		return nil
	}

	if structuredErr, ok := err.(*StructuredError); ok {
		structuredErr.Suggestions = append(structuredErr.Suggestions, suggestions...)
		return structuredErr
	}

	handler := NewDefaultErrorHandler()
	structuredErr := handler.HandleError(err)
	structuredErr.Suggestions = append(structuredErr.Suggestions, suggestions...)

	return structuredErr
}

// WrapWithContext wraps an error with additional context information
func WrapWithContext(err error, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	if structuredErr, ok := err.(*StructuredError); ok {
		if structuredErr.Context == nil {
			structuredErr.Context = make(map[string]interface{})
		}
		for k, v := range context {
			structuredErr.Context[k] = v
		}
		return structuredErr
	}

	handler := NewDefaultErrorHandler()
	structuredErr := handler.HandleError(err)
	structuredErr.Context = context

	return structuredErr
}

// Chain creates a chain of wrapped errors
func Chain(errors ...error) error {
	var result error

	for i, err := range errors {
		if err == nil {
			continue
		}

		if result == nil {
			result = err
		} else {
			wrapper := NewDefaultErrorWrapper()
			result = wrapper.WrapError(result, fmt.Sprintf("error %d", i+1))
		}
	}

	return result
}

// Annotate adds an annotation to an error without changing its type
func Annotate(err error, annotation string) error {
	if err == nil {
		return nil
	}

	if structuredErr, ok := err.(*StructuredError); ok {
		// Add annotation to context
		if structuredErr.Context == nil {
			structuredErr.Context = make(map[string]interface{})
		}

		// Add to existing annotations or create new
		if existing, ok := structuredErr.Context["annotations"].([]string); ok {
			structuredErr.Context["annotations"] = append(existing, annotation)
		} else {
			structuredErr.Context["annotations"] = []string{annotation}
		}

		return structuredErr
	}

	return fmt.Errorf("%s (%s)", err.Error(), annotation)
}
