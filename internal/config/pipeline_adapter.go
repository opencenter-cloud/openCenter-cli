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

package config

import (
	"context"
)

// PipelineAdapter is a temporary stub for the validation pipeline adapter.
// TODO: Implement the full validation pipeline architecture as described in docs/dev/validation-pipeline.md
type PipelineAdapter struct {
	// Placeholder for future implementation
}

// NewPipelineAdapter creates a new pipeline adapter.
func NewPipelineAdapter() *PipelineAdapter {
	return &PipelineAdapter{}
}

// Validate performs validation using the pipeline.
// This is a stub implementation that returns a valid result.
func (pa *PipelineAdapter) Validate(ctx context.Context, config *Config) *ConfigValidationResult {
	// TODO: Implement full validation pipeline
	return &ConfigValidationResult{
		Valid:    true,
		Errors:   []ConfigValidationError{},
		Warnings: []ConfigValidationError{},
	}
}

// ValidateStructure validates the basic structure.
// This is a stub implementation that returns a valid result.
func (pa *PipelineAdapter) ValidateStructure(ctx context.Context, config *Config) *ConfigValidationResult {
	// TODO: Implement structural validation
	return &ConfigValidationResult{
		Valid:    true,
		Errors:   []ConfigValidationError{},
		Warnings: []ConfigValidationError{},
	}
}

// ValidateSemantics validates semantic correctness.
// This is a stub implementation that returns a valid result.
func (pa *PipelineAdapter) ValidateSemantics(ctx context.Context, config *Config) *ConfigValidationResult {
	// TODO: Implement semantic validation
	return &ConfigValidationResult{
		Valid:    true,
		Errors:   []ConfigValidationError{},
		Warnings: []ConfigValidationError{},
	}
}
