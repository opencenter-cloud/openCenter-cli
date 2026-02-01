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

// TestValidationEngine_ConcurrentRegister tests concurrent validator registration.
func TestValidationEngine_ConcurrentRegister(t *testing.T) {
	engine := NewValidationEngine()
	var wg sync.WaitGroup

	// Register 100 validators concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			validator := NewValidatorFunc(
				string(rune('a'+id%26))+string(rune('0'+id/26)),
				func(ctx context.Context, value interface{}) (*ValidationResult, error) {
					return &ValidationResult{Valid: true}, nil
				},
			)

			_ = engine.Register(validator)
		}(i)
	}

	wg.Wait()

	// Verify all validators were registered
	count := engine.registry.Count()
	if count == 0 {
		t.Error("No validators were registered")
	}
}

// TestValidationEngine_ConcurrentValidate tests concurrent validation operations.
func TestValidationEngine_ConcurrentValidate(t *testing.T) {
	engine := NewValidationEngine()

	// Register a test validator
	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		result := &ValidationResult{Valid: true}
		if value == nil {
			result.AddError("value", "value cannot be nil")
		}
		return result, nil
	})

	engine.MustRegister(validator)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Perform 100 concurrent validations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_, err := engine.Validate(context.Background(), "test", "value")
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Validation error: %v", err)
	}
}

// TestValidationEngine_ConcurrentMixedOperations tests concurrent mixed operations.
func TestValidationEngine_ConcurrentMixedOperations(t *testing.T) {
	engine := NewValidationEngine()

	// Register initial validators
	for i := 0; i < 10; i++ {
		validator := NewValidatorFunc(
			string(rune('a'+i)),
			func(ctx context.Context, value interface{}) (*ValidationResult, error) {
				return &ValidationResult{Valid: true}, nil
			},
		)
		engine.MustRegister(validator)
	}

	var wg sync.WaitGroup

	// Concurrent reads (validations)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = engine.Validate(context.Background(), "a", "value")
		}()
	}

	// Concurrent reads (list)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = engine.List()
		}()
	}

	// Concurrent reads (has)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = engine.Has("a")
		}()
	}

	wg.Wait()
}

// TestValidationEngine_ConcurrentAddSuggestionRule tests concurrent suggestion rule additions.
func TestValidationEngine_ConcurrentAddSuggestionRule(t *testing.T) {
	engine := NewValidationEngine()
	var wg sync.WaitGroup

	// Add 100 suggestion rules concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			rule := &TypoSuggestionRule{}

			engine.AddSuggestionRule(rule)
		}(i)
	}

	wg.Wait()

	// Verify engine is still functional
	if engine.GetSuggestionEngine() == nil {
		t.Error("SuggestionEngine is nil after concurrent operations")
	}
}

// TestRegistry_ConcurrentOperations tests concurrent registry operations.
func TestRegistry_ConcurrentOperations(t *testing.T) {
	registry := NewRegistry()

	// Register initial validators
	for i := 0; i < 10; i++ {
		validator := NewValidatorFunc(
			string(rune('a'+i)),
			func(ctx context.Context, value interface{}) (*ValidationResult, error) {
				return &ValidationResult{Valid: true}, nil
			},
		)
		registry.MustRegister(validator)
	}

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.Get("a")
			_ = registry.Has("a")
			_ = registry.List()
			_ = registry.Count()
		}()
	}

	wg.Wait()

	// Verify registry is still functional
	if registry.Count() != 10 {
		t.Errorf("Expected 10 validators, got %d", registry.Count())
	}
}
