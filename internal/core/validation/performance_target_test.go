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

// TestValidationEngine_PerformanceTarget verifies that validation meets the <300ms target
// for a typical cluster validation scenario with multiple validators.
func TestValidationEngine_PerformanceTarget(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register 20 validators to simulate a realistic cluster validation scenario
	// (schema, business rules, provider-specific, services, etc.)
	for i := 0; i < 20; i++ {
		name := "validator-" + string(rune('a'+i))
		engine.Register(&mockValidator{
			name:   name,
			result: &ValidationResult{Valid: true},
		})
	}

	validators := make([]string, 20)
	for i := 0; i < 20; i++ {
		validators[i] = "validator-" + string(rune('a'+i))
	}

	// Run validation 100 times to get average
	const iterations = 100
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_, err := engine.ValidateAll(ctx, validators, "test-cluster")
		if err != nil {
			t.Fatalf("Validation failed: %v", err)
		}
	}

	elapsed := time.Since(start)
	avgTime := elapsed / iterations

	t.Logf("Average validation time: %v", avgTime)
	t.Logf("Total time for %d iterations: %v", iterations, elapsed)

	// Target: <300ms for full cluster validation
	// Current implementation should be much faster (< 1ms per validation)
	target := 300 * time.Millisecond
	if avgTime > target {
		t.Errorf("Validation time %v exceeds target %v", avgTime, target)
	}

	// Log performance margin
	margin := float64(target-avgTime) / float64(target) * 100
	t.Logf("Performance margin: %.1f%% faster than target", margin)
}

// TestValidationEngine_PerformanceTargetWithErrors verifies performance with validation errors
func TestValidationEngine_PerformanceTargetWithErrors(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register validators that return errors
	for i := 0; i < 20; i++ {
		name := "validator-" + string(rune('a'+i))
		engine.Register(&mockValidator{
			name: name,
			result: &ValidationResult{
				Valid: false,
				Errors: []*ValidationIssue{
					{Field: "field1", Message: "error1"},
					{Field: "field2", Message: "error2"},
				},
			},
		})
	}

	validators := make([]string, 20)
	for i := 0; i < 20; i++ {
		validators[i] = "validator-" + string(rune('a'+i))
	}

	// Run validation 100 times
	const iterations = 100
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_, err := engine.ValidateAll(ctx, validators, "test-cluster")
		if err != nil {
			t.Fatalf("Validation failed: %v", err)
		}
	}

	elapsed := time.Since(start)
	avgTime := elapsed / iterations

	t.Logf("Average validation time (with errors): %v", avgTime)
	t.Logf("Total time for %d iterations: %v", iterations, elapsed)

	// Target: <300ms even with errors
	target := 300 * time.Millisecond
	if avgTime > target {
		t.Errorf("Validation time %v exceeds target %v", avgTime, target)
	}

	// Log performance margin
	margin := float64(target-avgTime) / float64(target) * 100
	t.Logf("Performance margin: %.1f%% faster than target", margin)
}

// TestValidationEngine_PerformanceTargetParallel verifies parallel validation performance
func TestValidationEngine_PerformanceTargetParallel(t *testing.T) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register 20 validators
	for i := 0; i < 20; i++ {
		name := "validator-" + string(rune('a'+i))
		engine.Register(&mockValidator{
			name:   name,
			result: &ValidationResult{Valid: true},
		})
	}

	validators := make([]string, 20)
	for i := 0; i < 20; i++ {
		validators[i] = "validator-" + string(rune('a'+i))
	}

	// Run parallel validation 100 times
	const iterations = 100
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_, err := engine.ValidateParallel(ctx, validators, "test-cluster")
		if err != nil {
			t.Fatalf("Validation failed: %v", err)
		}
	}

	elapsed := time.Since(start)
	avgTime := elapsed / iterations

	t.Logf("Average parallel validation time: %v", avgTime)
	t.Logf("Total time for %d iterations: %v", iterations, elapsed)

	// Target: <300ms for parallel validation
	target := 300 * time.Millisecond
	if avgTime > target {
		t.Errorf("Parallel validation time %v exceeds target %v", avgTime, target)
	}

	// Log performance margin
	margin := float64(target-avgTime) / float64(target) * 100
	t.Logf("Performance margin: %.1f%% faster than target", margin)
}

// BenchmarkValidationEngine_PerformanceTarget benchmarks the performance target scenario
func BenchmarkValidationEngine_PerformanceTarget(b *testing.B) {
	engine := NewValidationEngine()
	ctx := context.Background()

	// Register 20 validators
	for i := 0; i < 20; i++ {
		name := "validator-" + string(rune('a'+i))
		engine.Register(&mockValidator{
			name:   name,
			result: &ValidationResult{Valid: true},
		})
	}

	validators := make([]string, 20)
	for i := 0; i < 20; i++ {
		validators[i] = "validator-" + string(rune('a'+i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ValidateAll(ctx, validators, "test-cluster")
	}
}
