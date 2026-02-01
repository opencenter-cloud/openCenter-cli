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

func BenchmarkValidationEngine_SingleValidator(b *testing.B) {
	engine := NewValidationEngine()
	validator := &mockValidator{
		name:   "test",
		result: &ValidationResult{Valid: true},
	}
	engine.Register(validator)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Validate(ctx, "test", "my-cluster")
	}
}

func BenchmarkValidationEngine_MultipleValidators(b *testing.B) {
	engine := NewValidationEngine()
	engine.Register(&mockValidator{name: "test1", result: &ValidationResult{Valid: true}})
	engine.Register(&mockValidator{name: "test2", result: &ValidationResult{Valid: true}})
	engine.Register(&mockValidator{name: "test3", result: &ValidationResult{Valid: true}})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ValidateAll(ctx, []string{"test1", "test2", "test3"}, "test-value")
	}
}

func BenchmarkValidationEngine_ParallelValidation(b *testing.B) {
	engine := NewValidationEngine()
	engine.Register(&mockValidator{name: "test1", result: &ValidationResult{Valid: true}})
	engine.Register(&mockValidator{name: "test2", result: &ValidationResult{Valid: true}})
	engine.Register(&mockValidator{name: "test3", result: &ValidationResult{Valid: true}})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ValidateParallel(ctx, []string{"test1", "test2", "test3"}, "test-value")
	}
}

func BenchmarkSuggestionEngine_EnhanceResult(b *testing.B) {
	engine := NewSuggestionEngine()
	result := &ValidationResult{
		Valid: false,
		Errors: []*ValidationIssue{
			{Field: "test", Message: "invalid value 'tst'"},
		},
	}
	context := map[string]interface{}{
		"valid_values": []string{"test", "prod", "dev"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a copy to avoid modifying the original
		resultCopy := &ValidationResult{
			Valid: result.Valid,
			Errors: []*ValidationIssue{
				{Field: result.Errors[0].Field, Message: result.Errors[0].Message},
			},
		}
		engine.EnhanceResult(resultCopy, context)
	}
}

func BenchmarkLevenshteinDistance(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = levenshteinDistance("kitten", "sitting")
	}
}
