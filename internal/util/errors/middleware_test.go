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
	"testing"
)

// MockLogger implements the Logger interface for testing
type MockLogger struct {
	ErrorCalls []LogCall
	WarnCalls  []LogCall
	InfoCalls  []LogCall
	DebugCalls []LogCall
}

type LogCall struct {
	Message        string
	KeysAndValues  []interface{}
}

func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.ErrorCalls = append(m.ErrorCalls, LogCall{Message: msg, KeysAndValues: keysAndValues})
}

func (m *MockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.WarnCalls = append(m.WarnCalls, LogCall{Message: msg, KeysAndValues: keysAndValues})
}

func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.InfoCalls = append(m.InfoCalls, LogCall{Message: msg, KeysAndValues: keysAndValues})
}

func (m *MockLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.DebugCalls = append(m.DebugCalls, LogCall{Message: msg, KeysAndValues: keysAndValues})
}

func TestErrorMiddleware_Handle(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		fn        func() error
		wantErr   bool
		wantLog   bool
	}{
		{
			name:      "successful operation",
			operation: "test_op",
			fn:        func() error { return nil },
			wantErr:   false,
			wantLog:   false,
		},
		{
			name:      "operation with error",
			operation: "test_op",
			fn:        func() error { return fmt.Errorf("test error") },
			wantErr:   true,
			wantLog:   true,
		},
		{
			name:      "operation with structured error",
			operation: "test_op",
			fn: func() error {
				return &StructuredError{
					Type:    ValidationError,
					Message: "validation failed",
				}
			},
			wantErr: true,
			wantLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &MockLogger{}
			middleware := NewErrorMiddleware(logger)
			ctx := context.Background()

			err := middleware.Handle(ctx, tt.operation, tt.fn)

			if (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantLog {
				totalLogs := len(logger.ErrorCalls) + len(logger.WarnCalls)
				if totalLogs == 0 {
					t.Errorf("Expected error to be logged, but no logs found")
				}
			}
		})
	}
}

func TestErrorMiddleware_HandleError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		err       error
		wantType  ErrorType
	}{
		{
			name:      "nil error",
			operation: "test_op",
			err:       nil,
			wantType:  "",
		},
		{
			name:      "regular error",
			operation: "test_op",
			err:       fmt.Errorf("test error"),
			wantType:  SystemError,
		},
		{
			name:      "structured error",
			operation: "test_op",
			err: &StructuredError{
				Type:    ValidationError,
				Message: "validation failed",
			},
			wantType: ValidationError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &MockLogger{}
			middleware := NewErrorMiddleware(logger)
			ctx := context.Background()

			result := middleware.HandleError(ctx, tt.operation, tt.err)

			if tt.err == nil {
				if result != nil {
					t.Errorf("HandleError() with nil error should return nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("HandleError() returned nil, expected error")
				return
			}

			structuredErr, ok := result.(*StructuredError)
			if !ok {
				t.Errorf("HandleError() should return StructuredError, got %T", result)
				return
			}

			if structuredErr.Type != tt.wantType {
				t.Errorf("HandleError() type = %v, want %v", structuredErr.Type, tt.wantType)
			}

			if structuredErr.Operation != tt.operation {
				t.Errorf("HandleError() operation = %v, want %v", structuredErr.Operation, tt.operation)
			}
		})
	}
}

func TestErrorMiddleware_WrapCommand(t *testing.T) {
	logger := &MockLogger{}
	middleware := NewErrorMiddleware(logger)

	t.Run("successful command", func(t *testing.T) {
		wrapped := middleware.WrapCommand("test_op", func(ctx context.Context) error {
			return nil
		})

		err := wrapped(context.Background())
		if err != nil {
			t.Errorf("WrapCommand() error = %v, want nil", err)
		}
	})

	t.Run("command with error", func(t *testing.T) {
		wrapped := middleware.WrapCommand("test_op", func(ctx context.Context) error {
			return fmt.Errorf("test error")
		})

		err := wrapped(context.Background())
		if err == nil {
			t.Errorf("WrapCommand() error = nil, want error")
		}
	})
}

func TestErrorMiddleware_WrapCommandWithArgs(t *testing.T) {
	logger := &MockLogger{}
	middleware := NewErrorMiddleware(logger)

	t.Run("successful command with args", func(t *testing.T) {
		wrapped := middleware.WrapCommandWithArgs("test_op", func(ctx context.Context, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("expected 2 args, got %d", len(args))
			}
			return nil
		})

		err := wrapped(context.Background(), []string{"arg1", "arg2"})
		if err != nil {
			t.Errorf("WrapCommandWithArgs() error = %v, want nil", err)
		}
	})

	t.Run("command with args and error", func(t *testing.T) {
		wrapped := middleware.WrapCommandWithArgs("test_op", func(ctx context.Context, args []string) error {
			return fmt.Errorf("test error")
		})

		err := wrapped(context.Background(), []string{"arg1"})
		if err == nil {
			t.Errorf("WrapCommandWithArgs() error = nil, want error")
		}
	})
}

func TestErrorMiddleware_HandleValidationErrors(t *testing.T) {
	logger := &MockLogger{}
	middleware := NewErrorMiddleware(logger)
	ctx := context.Background()

	t.Run("no errors", func(t *testing.T) {
		err := middleware.HandleValidationErrors(ctx, "test_op", nil)
		if err != nil {
			t.Errorf("HandleValidationErrors() with no errors should return nil, got %v", err)
		}
	})

	t.Run("single error", func(t *testing.T) {
		errs := []*StructuredError{
			{
				Type:    ValidationError,
				Message: "validation failed",
			},
		}

		err := middleware.HandleValidationErrors(ctx, "test_op", errs)
		if err == nil {
			t.Errorf("HandleValidationErrors() should return error")
		}

		if len(logger.WarnCalls) == 0 {
			t.Errorf("Expected validation errors to be logged")
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		errs := []*StructuredError{
			{
				Type:    ValidationError,
				Message: "error 1",
			},
			{
				Type:    ValidationError,
				Message: "error 2",
			},
		}

		err := middleware.HandleValidationErrors(ctx, "test_op", errs)
		if err == nil {
			t.Errorf("HandleValidationErrors() should return error")
		}

		collection, ok := err.(*ErrorCollection)
		if !ok {
			t.Errorf("Expected ErrorCollection, got %T", err)
		}

		if len(collection.Errors) != 2 {
			t.Errorf("Expected 2 errors in collection, got %d", len(collection.Errors))
		}
	})
}

func TestErrorMiddleware_IsRetryable(t *testing.T) {
	logger := &MockLogger{}
	middleware := NewErrorMiddleware(logger)

	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{
			name: "network timeout",
			err:  fmt.Errorf("network timeout"),
			want: true,
		},
		{
			name: "connection refused",
			err:  fmt.Errorf("connection refused"),
			want: true,
		},
		{
			name: "validation error",
			err:  fmt.Errorf("invalid input"),
			want: false,
		},
		{
			name: "permission denied",
			err:  fmt.Errorf("permission denied"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := middleware.IsRetryable(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorMiddleware_GetSuggestions(t *testing.T) {
	logger := &MockLogger{}
	middleware := NewErrorMiddleware(logger)

	t.Run("error with suggestions", func(t *testing.T) {
		err := fmt.Errorf("permission denied")
		suggestions := middleware.GetSuggestions(err)

		if len(suggestions) == 0 {
			t.Errorf("Expected suggestions for permission error, got none")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		suggestions := middleware.GetSuggestions(nil)
		if suggestions != nil {
			t.Errorf("Expected nil suggestions for nil error, got %v", suggestions)
		}
	})
}

func TestErrorMiddleware_ContextPropagation(t *testing.T) {
	logger := &MockLogger{}
	middleware := NewErrorMiddleware(logger)

	t.Run("correlation ID propagation", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "correlation_id", "test-123")

		err := middleware.HandleError(ctx, "test_op", fmt.Errorf("test error"))

		structuredErr, ok := err.(*StructuredError)
		if !ok {
			t.Errorf("Expected StructuredError, got %T", err)
			return
		}

		if structuredErr.Context == nil {
			t.Errorf("Expected context to be set")
			return
		}

		correlationID, ok := structuredErr.Context["correlation_id"]
		if !ok {
			t.Errorf("Expected correlation_id in context")
			return
		}

		if correlationID != "test-123" {
			t.Errorf("Expected correlation_id = test-123, got %v", correlationID)
		}
	})
}
