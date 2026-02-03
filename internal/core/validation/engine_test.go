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
	"errors"
	"testing"
)

// mockValidator is a mock validator for testing
type mockValidator struct {
	name     string
	priority int
	result   *ValidationResult
	err      error
}

func (m *mockValidator) Name() string {
	return m.name
}

func (m *mockValidator) Priority() int {
	if m.priority == 0 {
		return PriorityNormal
	}
	return m.priority
}

func (m *mockValidator) Validate(ctx context.Context, value interface{}) (*ValidationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestNewValidationEngine(t *testing.T) {
	engine := NewValidationEngine()
	if engine == nil {
		t.Fatal("NewValidationEngine returned nil")
	}
	if engine.registry == nil {
		t.Error("registry is nil")
	}
	if engine.suggestionEngine == nil {
		t.Error("suggestionEngine is nil")
	}
}

func TestValidationEngine_Register(t *testing.T) {
	engine := NewValidationEngine()

	validator := &mockValidator{name: "test"}
	err := engine.Register(validator)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !engine.Has("test") {
		t.Error("validator not registered")
	}
}

func TestValidationEngine_MustRegister(t *testing.T) {
	engine := NewValidationEngine()

	validator := &mockValidator{name: "test"}
	engine.MustRegister(validator)

	if !engine.Has("test") {
		t.Error("validator not registered")
	}

	// Test panic on duplicate
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustRegister should panic on duplicate")
		}
	}()
	engine.MustRegister(validator)
}

func TestValidationEngine_Unregister(t *testing.T) {
	engine := NewValidationEngine()

	validator := &mockValidator{name: "test"}
	engine.Register(validator)

	err := engine.Unregister("test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if engine.Has("test") {
		t.Error("validator still registered")
	}
}

func TestValidationEngine_List(t *testing.T) {
	engine := NewValidationEngine()

	engine.Register(&mockValidator{name: "test1"})
	engine.Register(&mockValidator{name: "test2"})

	list := engine.List()
	if len(list) != 2 {
		t.Errorf("expected 2 validators, got %d", len(list))
	}
}

func TestValidationEngine_Validate(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	result := &ValidationResult{Valid: true}
	validator := &mockValidator{name: "test", result: result}
	engine.Register(validator)

	got, err := engine.Validate(ctx, "test", "value")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !got.Valid {
		t.Error("expected valid result")
	}
}

func TestValidationEngine_ValidateNotFound(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	_, err := engine.Validate(ctx, "nonexistent", "value")
	if err == nil {
		t.Error("expected error for nonexistent validator")
	}
}

func TestValidationEngine_ValidateError(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	validator := &mockValidator{
		name: "test",
		err:  errors.New("validation error"),
	}
	engine.Register(validator)

	_, err := engine.Validate(ctx, "test", "value")
	if err == nil {
		t.Error("expected error from validator")
	}
}

func TestValidationEngine_ValidateAll(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register validators
	result1 := &ValidationResult{Valid: true}
	result2 := &ValidationResult{Valid: true}
	engine.Register(&mockValidator{name: "test1", result: result1})
	engine.Register(&mockValidator{name: "test2", result: result2})

	got, err := engine.ValidateAll(ctx, []string{"test1", "test2"}, "value")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}
	if !got.Valid {
		t.Error("expected valid result")
	}
}

func TestValidationEngine_ValidateAllWithError(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register validators with one error
	result1 := &ValidationResult{
		Valid:  false,
		Errors: []*ValidationIssue{{Field: "test", Message: "error"}},
	}
	result2 := &ValidationResult{Valid: true}
	engine.Register(&mockValidator{name: "test1", result: result1})
	engine.Register(&mockValidator{name: "test2", result: result2})

	got, err := engine.ValidateAll(ctx, []string{"test1", "test2"}, "value")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}
	if got.Valid {
		t.Error("expected invalid result")
	}
	if len(got.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(got.Errors))
	}
}

func TestValidationEngine_ValidateAllStopOnFirstError(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register validators with one error
	result1 := &ValidationResult{
		Valid:  false,
		Errors: []*ValidationIssue{{Field: "test", Message: "error"}},
	}
	result2 := &ValidationResult{Valid: true}
	engine.Register(&mockValidator{name: "test1", result: result1})
	engine.Register(&mockValidator{name: "test2", result: result2})

	opts := &ValidationOptions{StopOnFirstError: true}
	got, err := engine.ValidateAllWithOptions(ctx, []string{"test1", "test2"}, "value", opts)
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}
	if got.Valid {
		t.Error("expected invalid result")
	}
	// Should only have errors from test1
	if len(got.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(got.Errors))
	}
}

func TestValidationEngine_ValidateParallel(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register validators
	result1 := &ValidationResult{Valid: true}
	result2 := &ValidationResult{Valid: true}
	engine.Register(&mockValidator{name: "test1", result: result1})
	engine.Register(&mockValidator{name: "test2", result: result2})

	got, err := engine.ValidateParallel(ctx, []string{"test1", "test2"}, "value")
	if err != nil {
		t.Fatalf("ValidateParallel failed: %v", err)
	}
	if !got.Valid {
		t.Error("expected valid result")
	}
}

func TestValidationEngine_ValidateParallelWithError(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register validators with one error
	result1 := &ValidationResult{
		Valid:  false,
		Errors: []*ValidationIssue{{Field: "test", Message: "error"}},
	}
	result2 := &ValidationResult{Valid: true}
	engine.Register(&mockValidator{name: "test1", result: result1})
	engine.Register(&mockValidator{name: "test2", result: result2})

	got, err := engine.ValidateParallel(ctx, []string{"test1", "test2"}, "value")
	if err != nil {
		t.Fatalf("ValidateParallel failed: %v", err)
	}
	if got.Valid {
		t.Error("expected invalid result")
	}
	if len(got.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(got.Errors))
	}
}

func TestValidationEngine_ValidateWithOptions(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	result := &ValidationResult{
		Valid:    true,
		Warnings: []*ValidationIssue{{Field: "test", Message: "warning"}},
	}
	validator := &mockValidator{name: "test", result: result}
	engine.Register(validator)

	// Test with warnings included
	opts := &ValidationOptions{IncludeWarnings: true}
	got, err := engine.ValidateWithOptions(ctx, "test", "value", opts)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(got.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(got.Warnings))
	}

	// Test with warnings excluded
	opts = &ValidationOptions{IncludeWarnings: false}
	got, err = engine.ValidateWithOptions(ctx, "test", "value", opts)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(got.Warnings))
	}
}

func TestValidationEngine_AddSuggestionRule(t *testing.T) {
	engine := NewValidationEngine()

	// Create a custom suggestion rule
	rule := &mockSuggestionRule{name: "test"}
	engine.AddSuggestionRule(rule)

	// Verify rule was added (indirectly through suggestion engine)
	if engine.suggestionEngine == nil {
		t.Error("suggestion engine is nil")
	}
}

func TestValidationEngine_GetRegistry(t *testing.T) {
	engine := NewValidationEngine()
	registry := engine.GetRegistry()
	if registry == nil {
		t.Error("GetRegistry returned nil")
	}
}

func TestValidationEngine_GetSuggestionEngine(t *testing.T) {
	engine := NewValidationEngine()
	suggestionEngine := engine.GetSuggestionEngine()
	if suggestionEngine == nil {
		t.Error("GetSuggestionEngine returned nil")
	}
}

func TestDefaultEngine(t *testing.T) {
	engine := DefaultEngine()
	if engine == nil {
		t.Error("DefaultEngine returned nil")
	}
}

func TestValidate(t *testing.T) {
	// Use default engine
	ctx := context.Background()

	// Register a validator in the default engine
	result := &ValidationResult{Valid: true}
	validator := &mockValidator{name: "test-default", result: result}
	DefaultEngine().Register(validator)

	got, err := Validate(ctx, "test-default", "value")
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if !got.Valid {
		t.Error("expected valid result")
	}
}

func TestValidateAll_DefaultEngine(t *testing.T) {
	ctx := context.Background()

	// Register validators in the default engine
	result1 := &ValidationResult{Valid: true}
	result2 := &ValidationResult{Valid: true}
	DefaultEngine().Register(&mockValidator{name: "test-all-1", result: result1})
	DefaultEngine().Register(&mockValidator{name: "test-all-2", result: result2})

	got, err := ValidateAll(ctx, []string{"test-all-1", "test-all-2"}, "value")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}
	if !got.Valid {
		t.Error("expected valid result")
	}
}

func TestValidateParallel_DefaultEngine(t *testing.T) {
	ctx := context.Background()

	// Register validators in the default engine
	result1 := &ValidationResult{Valid: true}
	result2 := &ValidationResult{Valid: true}
	DefaultEngine().Register(&mockValidator{name: "test-parallel-1", result: result1})
	DefaultEngine().Register(&mockValidator{name: "test-parallel-2", result: result2})

	got, err := ValidateParallel(ctx, []string{"test-parallel-1", "test-parallel-2"}, "value")
	if err != nil {
		t.Fatalf("ValidateParallel failed: %v", err)
	}
	if !got.Valid {
		t.Error("expected valid result")
	}
}

// mockSuggestionRule is a mock suggestion rule for testing
type mockSuggestionRule struct {
	name string
}

func (m *mockSuggestionRule) Name() string {
	return m.name
}

func (m *mockSuggestionRule) Generate(issue *ValidationIssue, context map[string]interface{}) []string {
	return []string{"test suggestion"}
}
