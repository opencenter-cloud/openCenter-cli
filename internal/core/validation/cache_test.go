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
	"time"
)

func TestValidationCache_GetSet(t *testing.T) {
	cache := NewValidationCache(5 * time.Minute)

	// Create a validation result
	result := NewValidationResult()
	result.AddError("field", "error message")

	// Cache miss - should return nil
	if got := cache.Get("test-validator", "test-data"); got != nil {
		t.Errorf("Expected cache miss, got result: %v", got)
	}

	// Set cache entry
	cache.Set("test-validator", "test-data", result, 0)

	// Cache hit - should return result
	got := cache.Get("test-validator", "test-data")
	if got == nil {
		t.Fatal("Expected cache hit, got nil")
	}

	if got.Valid != result.Valid {
		t.Errorf("Expected Valid=%v, got %v", result.Valid, got.Valid)
	}

	if len(got.Errors) != len(result.Errors) {
		t.Errorf("Expected %d errors, got %d", len(result.Errors), len(got.Errors))
	}
}

func TestValidationCache_DifferentData(t *testing.T) {
	cache := NewValidationCache(5 * time.Minute)

	result1 := NewValidationResult()
	result1.AddError("field1", "error1")

	result2 := NewValidationResult()
	result2.AddError("field2", "error2")

	// Cache results for different data
	cache.Set("test-validator", "data1", result1, 0)
	cache.Set("test-validator", "data2", result2, 0)

	// Verify different data produces different cache entries
	got1 := cache.Get("test-validator", "data1")
	if got1 == nil || len(got1.Errors) == 0 || got1.Errors[0].Field != "field1" {
		t.Error("Expected result1 for data1")
	}

	got2 := cache.Get("test-validator", "data2")
	if got2 == nil || len(got2.Errors) == 0 || got2.Errors[0].Field != "field2" {
		t.Error("Expected result2 for data2")
	}
}

func TestValidationCache_Expiration(t *testing.T) {
	cache := NewValidationCache(100 * time.Millisecond)

	result := NewValidationResult()
	result.AddError("field", "error")

	// Set cache entry with short TTL
	cache.Set("test-validator", "test-data", result, 100*time.Millisecond)

	// Immediate cache hit
	if got := cache.Get("test-validator", "test-data"); got == nil {
		t.Error("Expected cache hit immediately after set")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Cache miss after expiration
	if got := cache.Get("test-validator", "test-data"); got != nil {
		t.Error("Expected cache miss after expiration")
	}
}

func TestValidationCache_Invalidate(t *testing.T) {
	cache := NewValidationCache(5 * time.Minute)

	result := NewValidationResult()
	cache.Set("test-validator", "test-data", result, 0)

	// Verify cache hit
	if got := cache.Get("test-validator", "test-data"); got == nil {
		t.Fatal("Expected cache hit before invalidation")
	}

	// Invalidate entry
	cache.Invalidate("test-validator", "test-data")

	// Verify cache miss after invalidation
	if got := cache.Get("test-validator", "test-data"); got != nil {
		t.Error("Expected cache miss after invalidation")
	}
}

func TestValidationCache_InvalidateAll(t *testing.T) {
	cache := NewValidationCache(5 * time.Minute)

	result := NewValidationResult()
	cache.Set("test-validator", "data1", result, 0)
	cache.Set("test-validator", "data2", result, 0)
	cache.Set("other-validator", "data3", result, 0)

	// Invalidate all entries for test-validator
	cache.InvalidateAll("test-validator")

	// Verify test-validator entries are gone
	if got := cache.Get("test-validator", "data1"); got != nil {
		t.Error("Expected cache miss for data1 after InvalidateAll")
	}
	if got := cache.Get("test-validator", "data2"); got != nil {
		t.Error("Expected cache miss for data2 after InvalidateAll")
	}

	// Verify other-validator entry still exists
	if got := cache.Get("other-validator", "data3"); got == nil {
		t.Error("Expected cache hit for other-validator after InvalidateAll")
	}
}

func TestValidationCache_Clear(t *testing.T) {
	cache := NewValidationCache(5 * time.Minute)

	result := NewValidationResult()
	cache.Set("validator1", "data1", result, 0)
	cache.Set("validator2", "data2", result, 0)

	// Verify entries exist
	if cache.Size() != 2 {
		t.Errorf("Expected 2 entries, got %d", cache.Size())
	}

	// Clear cache
	cache.Clear()

	// Verify all entries are gone
	if cache.Size() != 0 {
		t.Errorf("Expected 0 entries after Clear, got %d", cache.Size())
	}
}

func TestValidationCache_CleanExpired(t *testing.T) {
	cache := NewValidationCache(100 * time.Millisecond)

	result := NewValidationResult()
	
	// Add entries with different TTLs
	cache.Set("validator1", "data1", result, 50*time.Millisecond)  // Expires soon
	cache.Set("validator2", "data2", result, 500*time.Millisecond) // Expires later

	// Wait for first entry to expire
	time.Sleep(100 * time.Millisecond)

	// Clean expired entries
	removed := cache.CleanExpired()

	if removed != 1 {
		t.Errorf("Expected 1 entry removed, got %d", removed)
	}

	// Verify first entry is gone
	if got := cache.Get("validator1", "data1"); got != nil {
		t.Error("Expected cache miss for expired entry")
	}

	// Verify second entry still exists
	if got := cache.Get("validator2", "data2"); got == nil {
		t.Error("Expected cache hit for non-expired entry")
	}
}

func TestValidationCache_Stats(t *testing.T) {
	cache := NewValidationCache(100 * time.Millisecond)

	result := NewValidationResult()
	
	// Add entries
	cache.Set("validator1", "data1", result, 50*time.Millisecond)
	cache.Set("validator2", "data2", result, 500*time.Millisecond)

	// Check initial stats
	stats := cache.Stats()
	if stats.Total != 2 {
		t.Errorf("Expected 2 total entries, got %d", stats.Total)
	}
	if stats.Expired != 0 {
		t.Errorf("Expected 0 expired entries, got %d", stats.Expired)
	}

	// Wait for first entry to expire
	time.Sleep(100 * time.Millisecond)

	// Check stats after expiration
	stats = cache.Stats()
	if stats.Total != 2 {
		t.Errorf("Expected 2 total entries, got %d", stats.Total)
	}
	if stats.Expired != 1 {
		t.Errorf("Expected 1 expired entry, got %d", stats.Expired)
	}
}

func TestValidationCache_DisabledCaching(t *testing.T) {
	// Create cache with 0 TTL (disabled)
	cache := NewValidationCache(0)

	result := NewValidationResult()
	result.AddError("field", "error")

	// Try to set cache entry
	cache.Set("test-validator", "test-data", result, 0)

	// Verify caching is disabled (always cache miss)
	if got := cache.Get("test-validator", "test-data"); got != nil {
		t.Error("Expected cache miss when caching is disabled")
	}
}

func TestValidationEngine_Caching(t *testing.T) {
	engine := NewValidationEngine()

	// Create a validator that counts invocations
	invocations := 0
	validator := NewValidatorFunc("test-validator", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		invocations++
		result := NewValidationResult()
		result.AddError("field", "error")
		return result, nil
	})

	engine.Register(validator)

	ctx := context.Background()

	// First validation - should invoke validator
	result1, err := engine.Validate(ctx, "test-validator", "test-data")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if invocations != 1 {
		t.Errorf("Expected 1 invocation, got %d", invocations)
	}

	// Second validation with same data - should use cache
	result2, err := engine.Validate(ctx, "test-validator", "test-data")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if invocations != 1 {
		t.Errorf("Expected 1 invocation (cached), got %d", invocations)
	}

	// Verify results are equivalent
	if result1.Valid != result2.Valid {
		t.Error("Cached result differs from original")
	}

	// Third validation with different data - should invoke validator
	_, err = engine.Validate(ctx, "test-validator", "different-data")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if invocations != 2 {
		t.Errorf("Expected 2 invocations, got %d", invocations)
	}
}

func TestValidationEngine_CacheInvalidation(t *testing.T) {
	engine := NewValidationEngine()

	invocations := 0
	validator := NewValidatorFunc("test-validator", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		invocations++
		return NewValidationResult(), nil
	})

	engine.Register(validator)

	ctx := context.Background()

	// First validation
	_, err := engine.Validate(ctx, "test-validator", "test-data")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Invalidate cache
	engine.InvalidateCache("test-validator", "test-data")

	// Second validation - should invoke validator again
	_, err = engine.Validate(ctx, "test-validator", "test-data")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if invocations != 2 {
		t.Errorf("Expected 2 invocations after cache invalidation, got %d", invocations)
	}
}

func TestValidationEngine_CachingDisabled(t *testing.T) {
	// Create engine with caching disabled
	engine := NewValidationEngineWithCache(0)

	invocations := 0
	validator := NewValidatorFunc("test-validator", func(ctx context.Context, value interface{}) (*ValidationResult, error) {
		invocations++
		return NewValidationResult(), nil
	})

	engine.Register(validator)

	ctx := context.Background()

	// Multiple validations with same data
	for i := 0; i < 3; i++ {
		_, err := engine.Validate(ctx, "test-validator", "test-data")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	// Verify validator was invoked every time (no caching)
	if invocations != 3 {
		t.Errorf("Expected 3 invocations with caching disabled, got %d", invocations)
	}
}
