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

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if registry.validators == nil {
		t.Error("registry.validators is nil")
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	err := registry.Register(validator)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !registry.Has("test") {
		t.Error("validator not found after registration")
	}
}

func TestRegistry_RegisterNil(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(nil)
	if err == nil {
		t.Error("Expected error when registering nil validator")
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewRegistry()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	err := registry.Register(validator)
	if err != nil {
		t.Fatalf("First Register failed: %v", err)
	}

	err = registry.Register(validator)
	if err == nil {
		t.Error("Expected error when registering duplicate validator")
	}
}

func TestRegistry_MustRegister(t *testing.T) {
	registry := NewRegistry()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	// Should not panic
	registry.MustRegister(validator)

	if !registry.Has("test") {
		t.Error("validator not found after MustRegister")
	}
}

func TestRegistry_MustRegisterPanic(t *testing.T) {
	registry := NewRegistry()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	registry.MustRegister(validator)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when MustRegister duplicate validator")
		}
	}()

	registry.MustRegister(validator)
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	registry.MustRegister(validator)

	err := registry.Unregister("test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if registry.Has("test") {
		t.Error("validator still found after unregistration")
	}
}

func TestRegistry_UnregisterNotFound(t *testing.T) {
	registry := NewRegistry()

	err := registry.Unregister("nonexistent")
	if err == nil {
		t.Error("Expected error when unregistering nonexistent validator")
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	registry.MustRegister(validator)

	retrieved := registry.Get("test")
	if retrieved == nil {
		t.Error("Get returned nil for registered validator")
	}

	if retrieved.Name() != "test" {
		t.Errorf("Expected name %q, got %q", "test", retrieved.Name())
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	registry := NewRegistry()

	retrieved := registry.Get("nonexistent")
	if retrieved != nil {
		t.Error("Expected nil for nonexistent validator")
	}
}

func TestRegistry_Has(t *testing.T) {
	registry := NewRegistry()

	if registry.Has("test") {
		t.Error("Expected false for nonexistent validator")
	}

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	registry.MustRegister(validator)

	if !registry.Has("test") {
		t.Error("Expected true for registered validator")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	validator1 := NewValidatorFunc("test1", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	validator2 := NewValidatorFunc("test2", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	registry.MustRegister(validator1)
	registry.MustRegister(validator2)

	names := registry.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 validators, got %d", len(names))
	}

	// Check that both names are present
	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}

	if !found["test1"] || !found["test2"] {
		t.Error("Expected both test1 and test2 in list")
	}
}

func TestRegistry_Count(t *testing.T) {
	registry := NewRegistry()

	if registry.Count() != 0 {
		t.Errorf("Expected count 0, got %d", registry.Count())
	}

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	registry.MustRegister(validator)

	if registry.Count() != 1 {
		t.Errorf("Expected count 1, got %d", registry.Count())
	}
}

func TestRegistry_Clear(t *testing.T) {
	registry := NewRegistry()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	registry.MustRegister(validator)

	if registry.Count() != 1 {
		t.Error("Expected 1 validator before clear")
	}

	registry.Clear()

	if registry.Count() != 0 {
		t.Error("Expected 0 validators after clear")
	}
}

func TestRegistry_Clone(t *testing.T) {
	registry := NewRegistry()

	validator := NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	registry.MustRegister(validator)

	clone := registry.Clone()

	if clone.Count() != registry.Count() {
		t.Error("Clone has different count than original")
	}

	if !clone.Has("test") {
		t.Error("Clone does not have validator from original")
	}

	// Modify clone
	clone.Clear()

	if registry.Count() == 0 {
		t.Error("Original registry was modified when clone was cleared")
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Clear global registry before test
	Clear()

	validator := NewValidatorFunc("global-test", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		return &ValidationResult{Valid: true}, nil
	})

	err := Register(validator)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !Has("global-test") {
		t.Error("validator not found in global registry")
	}

	retrieved := Get("global-test")
	if retrieved == nil {
		t.Error("Get returned nil from global registry")
	}

	names := List()
	if len(names) == 0 {
		t.Error("List returned empty from global registry")
	}

	count := Count()
	if count == 0 {
		t.Error("Count returned 0 from global registry")
	}

	err = Unregister("global-test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if Has("global-test") {
		t.Error("validator still found after unregistration from global registry")
	}

	// Clean up
	Clear()
}
