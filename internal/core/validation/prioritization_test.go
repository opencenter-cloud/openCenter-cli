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
	"sync"
	"testing"
)

// TestValidatorPrioritization tests that validators execute in priority order.
//
// Requirements: 8.9
func TestValidatorPrioritization(t *testing.T) {
	engine := NewValidationEngine()

	// Track execution order
	var executionOrder []string
	var mu sync.Mutex

	// Create validators with different priorities
	highPriorityValidator := &mockValidator{
		name:     "high-priority",
		priority: PriorityHigh,
		result: &ValidationResult{
			Valid: true,
		},
	}

	normalPriorityValidator := &mockValidator{
		name:     "normal-priority",
		priority: PriorityNormal,
		result: &ValidationResult{
			Valid: true,
		},
	}

	lowPriorityValidator := &mockValidator{
		name:     "low-priority",
		priority: PriorityLow,
		result: &ValidationResult{
			Valid: true,
		},
	}

	// Wrap validators to track execution order
	highPriorityValidator.result = nil
	normalPriorityValidator.result = nil
	lowPriorityValidator.result = nil

	// Create tracking validators
	trackingHigh := NewValidatorFuncWithPriority("high-priority", PriorityHigh, func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "high-priority")
		mu.Unlock()
		return NewValidationResult(), nil
	})

	trackingNormal := NewValidatorFuncWithPriority("normal-priority", PriorityNormal, func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "normal-priority")
		mu.Unlock()
		return NewValidationResult(), nil
	})

	trackingLow := NewValidatorFuncWithPriority("low-priority", PriorityLow, func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "low-priority")
		mu.Unlock()
		return NewValidationResult(), nil
	})

	// Register validators in random order (not priority order)
	if err := engine.Register(trackingNormal); err != nil {
		t.Fatalf("failed to register normal priority validator: %v", err)
	}
	if err := engine.Register(trackingLow); err != nil {
		t.Fatalf("failed to register low priority validator: %v", err)
	}
	if err := engine.Register(trackingHigh); err != nil {
		t.Fatalf("failed to register high priority validator: %v", err)
	}

	// Execute validators using ValidateAll
	ctx := context.Background()
	names := []string{"normal-priority", "low-priority", "high-priority"}
	_, err := engine.ValidateAll(ctx, names, "test-value")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}

	// Verify execution order: high -> normal -> low
	expectedOrder := []string{"high-priority", "normal-priority", "low-priority"}
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("expected %d validators to execute, got %d", len(expectedOrder), len(executionOrder))
	}

	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Errorf("execution order mismatch at position %d: expected %s, got %s", i, expected, executionOrder[i])
		}
	}
}

// TestValidatorPrioritization_CustomPriorities tests validators with custom priority values.
func TestValidatorPrioritization_CustomPriorities(t *testing.T) {
	engine := NewValidationEngine()

	// Track execution order
	var executionOrder []string
	var mu sync.Mutex

	// Register validators in random order
	for _, v := range []struct {
		name     string
		priority int
	}{
		{"validator-150", 150},
		{"validator-10", 10},
		{"validator-200", 200},
		{"validator-50", 50},
		{"validator-100", 100},
	} {
		name := v.name // Capture for closure
		validator := NewValidatorFuncWithPriority(name, v.priority, func(ctx context.Context, value interface{}) (*ValidationResult, error) {
			mu.Lock()
			executionOrder = append(executionOrder, name)
			mu.Unlock()
			return NewValidationResult(), nil
		})
		if err := engine.Register(validator); err != nil {
			t.Fatalf("failed to register validator %s: %v", v.name, err)
		}
	}

	// Execute all validators
	ctx := context.Background()
	names := []string{"validator-150", "validator-10", "validator-200", "validator-50", "validator-100"}
	_, err := engine.ValidateAll(ctx, names, "test-value")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}

	// Verify execution order: 10 -> 50 -> 100 -> 150 -> 200
	expectedOrder := []string{"validator-10", "validator-50", "validator-100", "validator-150", "validator-200"}
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("expected %d validators to execute, got %d", len(expectedOrder), len(executionOrder))
	}

	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Errorf("execution order mismatch at position %d: expected %s, got %s", i, expected, executionOrder[i])
		}
	}
}

// TestValidatorPrioritization_SamePriority tests validators with the same priority.
func TestValidatorPrioritization_SamePriority(t *testing.T) {
	engine := NewValidationEngine()

	// Track execution
	var executionCount int
	var mu sync.Mutex

	// Create validators with same priority
	for i := 1; i <= 3; i++ {
		name := "validator-" + string(rune('0'+i))
		validator := NewValidatorFuncWithPriority(name, PriorityNormal, func(ctx context.Context, value interface{}) (*ValidationResult, error) {
			mu.Lock()
			executionCount++
			mu.Unlock()
			return NewValidationResult(), nil
		})
		if err := engine.Register(validator); err != nil {
			t.Fatalf("failed to register validator %s: %v", name, err)
		}
	}

	// Execute all validators
	ctx := context.Background()
	names := []string{"validator-1", "validator-2", "validator-3"}
	_, err := engine.ValidateAll(ctx, names, "test-value")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}

	// Verify all validators executed
	if executionCount != 3 {
		t.Errorf("expected 3 validators to execute, got %d", executionCount)
	}
}

// TestSortValidatorsByPriority tests the sorting helper function.
func TestSortValidatorsByPriority(t *testing.T) {
	tests := []struct {
		name     string
		input    []Validator
		expected []string // Expected order of validator names
	}{
		{
			name: "already sorted",
			input: []Validator{
				&mockValidator{name: "high", priority: PriorityHigh},
				&mockValidator{name: "normal", priority: PriorityNormal},
				&mockValidator{name: "low", priority: PriorityLow},
			},
			expected: []string{"high", "normal", "low"},
		},
		{
			name: "reverse order",
			input: []Validator{
				&mockValidator{name: "low", priority: PriorityLow},
				&mockValidator{name: "normal", priority: PriorityNormal},
				&mockValidator{name: "high", priority: PriorityHigh},
			},
			expected: []string{"high", "normal", "low"},
		},
		{
			name: "random order",
			input: []Validator{
				&mockValidator{name: "normal", priority: PriorityNormal},
				&mockValidator{name: "high", priority: PriorityHigh},
				&mockValidator{name: "low", priority: PriorityLow},
			},
			expected: []string{"high", "normal", "low"},
		},
		{
			name: "custom priorities",
			input: []Validator{
				&mockValidator{name: "v200", priority: 200},
				&mockValidator{name: "v10", priority: 10},
				&mockValidator{name: "v100", priority: 100},
				&mockValidator{name: "v50", priority: 50},
			},
			expected: []string{"v10", "v50", "v100", "v200"},
		},
		{
			name: "same priorities",
			input: []Validator{
				&mockValidator{name: "alpha", priority: 100},
				&mockValidator{name: "bravo", priority: 100},
				&mockValidator{name: "charlie", priority: 100},
			},
			expected: []string{"alpha", "bravo", "charlie"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the test input
			validators := make([]Validator, len(tt.input))
			copy(validators, tt.input)

			// Sort validators
			sortValidatorsByPriority(validators)

			// Verify order
			if len(validators) != len(tt.expected) {
				t.Fatalf("expected %d validators, got %d", len(tt.expected), len(validators))
			}

			for i, expected := range tt.expected {
				if validators[i].Name() != expected {
					t.Errorf("position %d: expected %s, got %s", i, expected, validators[i].Name())
				}
			}
		})
	}
}

// TestValidatorPrioritization_FastValidatorsFirst tests that fast validators run before slow ones.
func TestValidatorPrioritization_FastValidatorsFirst(t *testing.T) {
	engine := NewValidationEngine()

	// Track execution order
	var executionOrder []string
	var mu sync.Mutex

	// Create fast validator (format check)
	fastValidator := NewValidatorFuncWithPriority("format-check", PriorityHigh, func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "format-check")
		mu.Unlock()
		return NewValidationResult(), nil
	})

	// Create slow validator (file I/O)
	slowValidator := NewValidatorFuncWithPriority("file-check", PriorityLow, func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "file-check")
		mu.Unlock()
		return NewValidationResult(), nil
	})

	// Register validators
	if err := engine.Register(slowValidator); err != nil {
		t.Fatalf("failed to register slow validator: %v", err)
	}
	if err := engine.Register(fastValidator); err != nil {
		t.Fatalf("failed to register fast validator: %v", err)
	}

	// Execute validators
	ctx := context.Background()
	names := []string{"file-check", "format-check"}
	_, err := engine.ValidateAll(ctx, names, "test-value")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}

	// Verify fast validator ran first
	if len(executionOrder) != 2 {
		t.Fatalf("expected 2 validators to execute, got %d", len(executionOrder))
	}

	if executionOrder[0] != "format-check" {
		t.Errorf("expected format-check to run first, got %s", executionOrder[0])
	}

	if executionOrder[1] != "file-check" {
		t.Errorf("expected file-check to run second, got %s", executionOrder[1])
	}
}
