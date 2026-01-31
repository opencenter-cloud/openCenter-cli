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

package validation

import (
	"context"
	"testing"
)

func TestNewValidationEngine(t *testing.T) {
	engine := NewValidationEngine()
	if engine == nil {
		t.Fatal("NewValidationEngine returned nil")
	}

	if engine.registry == nil {
		t.Error("engine.registry is nil")
	}

	if engine.suggestionEngine == nil {
		t.Error("engine.suggestionEngine is nil")
	}
}

func TestValidationEngine_Register(t *testing.T) {
	engine := NewValidationEngine()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		result := &ValidationResult{Valid: true}
		return result, nil
	})

	err := engine.Register(validator)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !engine.Has("test") {
		t.Error("validator not found after registration")
	}
}

func TestValidationEngine_RegisterDuplicate(t *testing.T) {
	engine := NewValidationEngine()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		result := &ValidationResult{Valid: true}
		return result, nil
	})

	err := engine.Register(validator)
	if err != nil {
		t.Fatalf("First Register failed: %v", err)
	}

	err = engine.Register(validator)
	if err == nil {
		t.Error("Expected error when registering duplicate validator")
	}
}

func TestValidationEngine_Validate(t *testing.T) {
	engine := NewValidationEngine()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		result := &ValidationResult{Valid: true}
		if value == nil {
			result.AddError("value", "value cannot be nil")
		}
		return result, nil
	})

	engine.MustRegister(validator)

	// Test valid value
	result, err := engine.Validate(context.Background(), "test", "valid")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if !result.Valid {
		t.Error("Expected valid result")
	}

	// Test invalid value
	result, err = engine.Validate(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid result")
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
}

func TestValidationEngine_ValidateNotFound(t *testing.T) {
	engine := NewValidationEngine()

	_, err := engine.Validate(context.Background(), "nonexistent", "value")
	if err == nil {
		t.Error("Expected error for nonexistent validator")
	}
}

func TestValidationEngine_ValidateAll(t *testing.T) {
	engine := NewValidationEngine()

	validator1 := NewValidatorFunc("test1", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		result := &ValidationResult{Valid: true}
		return result, nil
	})

	validator2 := NewValidatorFunc("test2", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		result := &ValidationResult{Valid: true}
		result.AddWarning("field", "this is a warning")
		return result, nil
	})

	engine.MustRegister(validator1)
	engine.MustRegister(validator2)

	result, err := engine.ValidateAll(context.Background(), []string{"test1", "test2"}, "value")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}

	if !result.Valid {
		t.Error("Expected valid result")
	}

	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}
}

func TestValidationEngine_ValidateParallel(t *testing.T) {
	engine := NewValidationEngine()

	validator1 := NewValidatorFunc("test1", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		result := &ValidationResult{Valid: true}
		return result, nil
	})

	validator2 := NewValidatorFunc("test2", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		result := &ValidationResult{Valid: true}
		return result, nil
	})

	engine.MustRegister(validator1)
	engine.MustRegister(validator2)

	result, err := engine.ValidateParallel(context.Background(), []string{"test1", "test2"}, "value")
	if err != nil {
		t.Fatalf("ValidateParallel failed: %v", err)
	}

	if !result.Valid {
		t.Error("Expected valid result")
	}
}

func TestValidationEngine_List(t *testing.T) {
	engine := NewValidationEngine()

	validator1 := NewValidatorFunc("test1", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	validator2 := NewValidatorFunc("test2", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	engine.MustRegister(validator1)
	engine.MustRegister(validator2)

	names := engine.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 validators, got %d", len(names))
	}
}

func TestValidationEngine_Unregister(t *testing.T) {
	engine := NewValidationEngine()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	engine.MustRegister(validator)

	if !engine.Has("test") {
		t.Error("validator not found after registration")
	}

	err := engine.Unregister("test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if engine.Has("test") {
		t.Error("validator still found after unregistration")
	}
}
