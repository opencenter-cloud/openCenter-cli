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

	corevalidation "github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// ClusterConfigValidator implements the ConfigValidatorInterface for comprehensive configuration validation.
// It now uses the ValidationEngine from internal/core/validation for unified validation.
type ClusterConfigValidator struct {
	autoRepair       bool
	pipelineAdapter  *PipelineAdapter
	suggestionEngine *SuggestionEngine
	validationEngine *corevalidation.ValidationEngine
}

// NewConfigValidator creates a new configuration validator.
func NewConfigValidator(autoRepair bool) *ClusterConfigValidator {
	return &ClusterConfigValidator{
		autoRepair:       autoRepair,
		pipelineAdapter:  NewPipelineAdapter(),
		suggestionEngine: NewSuggestionEngine(),
		validationEngine: corevalidation.NewValidationEngine(),
	}
}

// Validate performs comprehensive validation on a configuration.
func (cv *ClusterConfigValidator) Validate(ctx context.Context, config *Config) *ConfigValidationResult {
	return cv.pipelineAdapter.Validate(ctx, config)
}

// ValidateStructure validates the basic structure of a configuration.
func (cv *ClusterConfigValidator) ValidateStructure(ctx context.Context, config *Config) *ConfigValidationResult {
	return cv.pipelineAdapter.ValidateStructure(ctx, config)
}

// ValidateSemantics validates the semantic correctness of a configuration.
func (cv *ClusterConfigValidator) ValidateSemantics(ctx context.Context, config *Config) *ConfigValidationResult {
	return cv.pipelineAdapter.ValidateSemantics(ctx, config)
}

// ValidateNetworking validates network plugin configuration.
func (cv *ClusterConfigValidator) ValidateNetworking(ctx context.Context, config *Config) *ConfigValidationResult {
	return cv.pipelineAdapter.ValidateNetworking(ctx, config)
}

// ValidateCloudProvider validates cloud provider specific configuration.
func (cv *ClusterConfigValidator) ValidateCloudProvider(ctx context.Context, config *Config) *ConfigValidationResult {
	return cv.pipelineAdapter.ValidateCloudProvider(ctx, config)
}

// SetAutoRepair enables or disables automatic repair of configuration issues.
func (cv *ClusterConfigValidator) SetAutoRepair(autoRepair bool) {
	cv.autoRepair = autoRepair
}

// IsAutoRepairEnabled returns whether automatic repair is enabled.
func (cv *ClusterConfigValidator) IsAutoRepairEnabled() bool {
	return cv.autoRepair
}

// GetSuggestionEngine returns the suggestion engine for generating helpful suggestions.
func (cv *ClusterConfigValidator) GetSuggestionEngine() *SuggestionEngine {
	return cv.suggestionEngine
}

// GetValidationEngine returns the core validation engine.
func (cv *ClusterConfigValidator) GetValidationEngine() *corevalidation.ValidationEngine {
	return cv.validationEngine
}

// enhanceSuggestions enhances existing suggestions with context-aware recommendations.
func (cv *ClusterConfigValidator) enhanceSuggestions(field string, value interface{}, existingSuggestions []string) []string {
	// Get field-specific suggestions from the engine
	fieldSuggestions := cv.suggestionEngine.GetSuggestionsForField(field, value)

	// Combine existing and new suggestions, avoiding duplicates
	suggestionSet := make(map[string]bool)
	enhanced := make([]string, 0, len(existingSuggestions)+len(fieldSuggestions))

	// Add existing suggestions first
	for _, s := range existingSuggestions {
		if !suggestionSet[s] {
			suggestionSet[s] = true
			enhanced = append(enhanced, s)
		}
	}

	// Add field-specific suggestions
	for _, s := range fieldSuggestions {
		if !suggestionSet[s] {
			suggestionSet[s] = true
			enhanced = append(enhanced, s)
		}
	}

	return enhanced
}
