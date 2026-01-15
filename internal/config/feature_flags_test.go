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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureFlags_DefaultBehavior(t *testing.T) {
	// Clear all feature flag environment variables
	clearFeatureFlagEnvVars(t)

	ff := GetFeatureFlags()
	ff.ClearCache()

	// All flags should be disabled by default
	assert.False(t, ff.UseNewTemplateEngine(), "Template engine should be disabled by default")
	assert.False(t, ff.UsePipelineGenerator(), "Pipeline generator should be disabled by default")
	assert.False(t, ff.UseNewConfigBuilder(), "Config builder should be disabled by default")
	assert.False(t, ff.UseServiceRegistry(), "Service registry should be disabled by default")
}

func TestFeatureFlags_IndividualFlags(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		value    string
		checkFn  func(*FeatureFlags) bool
		expected bool
	}{
		{
			name:     "template_engine_enabled",
			envVar:   EnvUseNewTemplateEngine,
			value:    "true",
			checkFn:  func(ff *FeatureFlags) bool { return ff.UseNewTemplateEngine() },
			expected: true,
		},
		{
			name:     "template_engine_disabled",
			envVar:   EnvUseNewTemplateEngine,
			value:    "false",
			checkFn:  func(ff *FeatureFlags) bool { return ff.UseNewTemplateEngine() },
			expected: false,
		},
		{
			name:     "pipeline_generator_enabled",
			envVar:   EnvUsePipelineGenerator,
			value:    "true",
			checkFn:  func(ff *FeatureFlags) bool { return ff.UsePipelineGenerator() },
			expected: true,
		},
		{
			name:     "config_builder_enabled",
			envVar:   EnvUseNewConfigBuilder,
			value:    "1",
			checkFn:  func(ff *FeatureFlags) bool { return ff.UseNewConfigBuilder() },
			expected: true,
		},
		{
			name:     "service_registry_enabled",
			envVar:   EnvUseServiceRegistry,
			value:    "yes",
			checkFn:  func(ff *FeatureFlags) bool { return ff.UseServiceRegistry() },
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearFeatureFlagEnvVars(t)
			t.Setenv(tt.envVar, tt.value)

			ff := GetFeatureFlags()
			ff.ClearCache()

			result := tt.checkFn(ff)
			assert.Equal(t, tt.expected, result, "Feature flag %s should be %v", tt.envVar, tt.expected)
		})
	}
}

func TestFeatureFlags_AllNewFeatures(t *testing.T) {
	clearFeatureFlagEnvVars(t)
	t.Setenv(EnvEnableAllNewFeatures, "true")

	ff := GetFeatureFlags()
	ff.ClearCache()

	// All features should be enabled when global flag is set
	assert.True(t, ff.UseNewTemplateEngine(), "Template engine should be enabled with global flag")
	assert.True(t, ff.UsePipelineGenerator(), "Pipeline generator should be enabled with global flag")
	assert.True(t, ff.UseNewConfigBuilder(), "Config builder should be enabled with global flag")
	assert.True(t, ff.UseServiceRegistry(), "Service registry should be enabled with global flag")
}

func TestFeatureFlags_IndividualOverridesGlobal(t *testing.T) {
	clearFeatureFlagEnvVars(t)
	t.Setenv(EnvEnableAllNewFeatures, "true")
	t.Setenv(EnvUseNewTemplateEngine, "false")

	ff := GetFeatureFlags()
	ff.ClearCache()

	// Individual flag should override global flag
	assert.False(t, ff.UseNewTemplateEngine(), "Individual flag should override global flag")
	assert.True(t, ff.UsePipelineGenerator(), "Other features should still be enabled by global flag")
}

func TestFeatureFlags_CaseInsensitive(t *testing.T) {
	testCases := []string{
		"true", "TRUE", "True", "TrUe",
		"yes", "YES", "Yes",
		"on", "ON", "On",
		"1",
	}

	for _, value := range testCases {
		t.Run("value_"+value, func(t *testing.T) {
			clearFeatureFlagEnvVars(t)
			t.Setenv(EnvUseNewTemplateEngine, value)

			ff := GetFeatureFlags()
			ff.ClearCache()

			assert.True(t, ff.UseNewTemplateEngine(),
				"Value %q should enable feature (case-insensitive)", value)
		})
	}
}

func TestFeatureFlags_WhitespaceHandling(t *testing.T) {
	testCases := []struct {
		value    string
		expected bool
	}{
		{" true ", true},
		{" false ", false},
		{"  yes  ", true},
		{"\ttrue\t", true},
		{"\nfalse\n", false},
	}

	for _, tc := range testCases {
		t.Run("whitespace_"+tc.value, func(t *testing.T) {
			clearFeatureFlagEnvVars(t)
			t.Setenv(EnvUseNewTemplateEngine, tc.value)

			ff := GetFeatureFlags()
			ff.ClearCache()

			assert.Equal(t, tc.expected, ff.UseNewTemplateEngine(),
				"Value %q should return %v after trimming whitespace", tc.value, tc.expected)
		})
	}
}

func TestFeatureFlags_Caching(t *testing.T) {
	clearFeatureFlagEnvVars(t)
	t.Setenv(EnvUseNewTemplateEngine, "true")

	ff := GetFeatureFlags()
	ff.ClearCache()

	// First call should evaluate and cache
	result1 := ff.UseNewTemplateEngine()
	assert.True(t, result1)

	// Change environment variable (should not affect cached result)
	os.Setenv(EnvUseNewTemplateEngine, "false")

	// Second call should return cached result
	result2 := ff.UseNewTemplateEngine()
	assert.True(t, result2, "Should return cached result")

	// Clear cache and check again
	ff.ClearCache()
	result3 := ff.UseNewTemplateEngine()
	assert.False(t, result3, "Should return new result after cache clear")
}

func TestFeatureFlags_GetStatus(t *testing.T) {
	clearFeatureFlagEnvVars(t)
	t.Setenv(EnvUseNewTemplateEngine, "true")
	t.Setenv(EnvUsePipelineGenerator, "false")

	ff := GetFeatureFlags()
	ff.ClearCache()

	status := ff.GetStatus()

	assert.True(t, status["new_template_engine"], "Template engine should be enabled")
	assert.False(t, status["pipeline_generator"], "Pipeline generator should be disabled")
	assert.False(t, status["new_config_builder"], "Config builder should be disabled")
	assert.False(t, status["service_registry"], "Service registry should be disabled")
	assert.False(t, status["all_new_features"], "Global flag should be disabled")
	assert.False(t, status["debug_enabled"], "Debug should be disabled")
}

func TestFeatureFlags_PackageLevelFunctions(t *testing.T) {
	clearFeatureFlagEnvVars(t)
	t.Setenv(EnvUseNewTemplateEngine, "true")
	t.Setenv(EnvUsePipelineGenerator, "true")
	t.Setenv(EnvUseNewConfigBuilder, "true")
	t.Setenv(EnvUseServiceRegistry, "true")

	// Clear cache to ensure fresh evaluation
	GetFeatureFlags().ClearCache()

	// Test package-level convenience functions
	assert.True(t, UseNewTemplateEngine(), "Package-level function should work")
	assert.True(t, UsePipelineGenerator(), "Package-level function should work")
	assert.True(t, UseNewConfigBuilder(), "Package-level function should work")
	assert.True(t, UseServiceRegistry(), "Package-level function should work")
}

func TestFeatureFlags_DebugMode(t *testing.T) {
	clearFeatureFlagEnvVars(t)
	t.Setenv(EnvFeatureFlagDebug, "true")

	ff := GetFeatureFlags()
	ff.ClearCache()

	status := ff.GetStatus()
	assert.True(t, status["debug_enabled"], "Debug mode should be enabled")
}

func TestFeatureFlags_InvalidValues(t *testing.T) {
	invalidValues := []string{
		"invalid",
		"2",
		"maybe",
		"",
		"truee",
		"yess",
	}

	for _, value := range invalidValues {
		t.Run("invalid_"+value, func(t *testing.T) {
			clearFeatureFlagEnvVars(t)
			t.Setenv(EnvUseNewTemplateEngine, value)

			ff := GetFeatureFlags()
			ff.ClearCache()

			assert.False(t, ff.UseNewTemplateEngine(),
				"Invalid value %q should be treated as false", value)
		})
	}
}

func TestFeatureFlags_ConcurrentAccess(t *testing.T) {
	clearFeatureFlagEnvVars(t)
	t.Setenv(EnvUseNewTemplateEngine, "true")

	ff := GetFeatureFlags()
	ff.ClearCache()

	// Test concurrent access to feature flags
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = ff.UseNewTemplateEngine()
				_ = ff.UsePipelineGenerator()
				_ = ff.GetStatus()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state is still correct
	assert.True(t, ff.UseNewTemplateEngine())
}

func TestFeatureFlags_Singleton(t *testing.T) {
	// Get feature flags instance multiple times
	ff1 := GetFeatureFlags()
	ff2 := GetFeatureFlags()

	// Should be the same instance
	assert.Same(t, ff1, ff2, "GetFeatureFlags should return singleton instance")
}

func TestParseBoolEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		value    string
		expected bool
	}{
		{"true", "TEST_VAR", "true", true},
		{"TRUE", "TEST_VAR", "TRUE", true},
		{"1", "TEST_VAR", "1", true},
		{"yes", "TEST_VAR", "yes", true},
		{"on", "TEST_VAR", "on", true},
		{"false", "TEST_VAR", "false", false},
		{"0", "TEST_VAR", "0", false},
		{"no", "TEST_VAR", "no", false},
		{"off", "TEST_VAR", "off", false},
		{"invalid", "TEST_VAR", "invalid", false},
		{"empty", "TEST_VAR", "", false},
		{"unset", "TEST_VAR_UNSET", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				t.Setenv(tt.envVar, tt.value)
			} else if tt.name != "unset" {
				t.Setenv(tt.envVar, "")
			}

			result := parseBoolEnv(tt.envVar)
			assert.Equal(t, tt.expected, result,
				"parseBoolEnv(%q) with value %q should return %v", tt.envVar, tt.value, tt.expected)
		})
	}
}

// Helper function to clear all feature flag environment variables
func clearFeatureFlagEnvVars(t *testing.T) {
	t.Helper()
	
	// Unset all feature flag environment variables
	envVars := []string{
		EnvUseNewTemplateEngine,
		EnvUsePipelineGenerator,
		EnvUseNewConfigBuilder,
		EnvUseServiceRegistry,
		EnvEnableAllNewFeatures,
		EnvFeatureFlagDebug,
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}

	// Clear the feature flags cache
	if globalFeatureFlags != nil {
		globalFeatureFlags.ClearCache()
	}
}

// TestFeatureFlags_MigrationScenarios tests realistic migration scenarios
func TestFeatureFlags_MigrationScenarios(t *testing.T) {
	t.Run("phase1_all_legacy", func(t *testing.T) {
		clearFeatureFlagEnvVars(t)
		ff := GetFeatureFlags()
		ff.ClearCache()

		// Phase 1: All systems use legacy implementation
		assert.False(t, ff.UseNewTemplateEngine())
		assert.False(t, ff.UsePipelineGenerator())
		assert.False(t, ff.UseNewConfigBuilder())
		assert.False(t, ff.UseServiceRegistry())
	})

	t.Run("phase2_gradual_testing", func(t *testing.T) {
		clearFeatureFlagEnvVars(t)
		// Enable only template engine for testing
		t.Setenv(EnvUseNewTemplateEngine, "true")
		
		ff := GetFeatureFlags()
		ff.ClearCache()

		// Phase 2: Test one feature at a time
		assert.True(t, ff.UseNewTemplateEngine())
		assert.False(t, ff.UsePipelineGenerator())
		assert.False(t, ff.UseNewConfigBuilder())
		assert.False(t, ff.UseServiceRegistry())
	})

	t.Run("phase3_all_new", func(t *testing.T) {
		clearFeatureFlagEnvVars(t)
		t.Setenv(EnvEnableAllNewFeatures, "true")
		
		ff := GetFeatureFlags()
		ff.ClearCache()

		// Phase 3: All systems use new implementation
		assert.True(t, ff.UseNewTemplateEngine())
		assert.True(t, ff.UsePipelineGenerator())
		assert.True(t, ff.UseNewConfigBuilder())
		assert.True(t, ff.UseServiceRegistry())
	})

	t.Run("rollback_scenario", func(t *testing.T) {
		clearFeatureFlagEnvVars(t)
		// Enable all new features but disable one that has issues
		t.Setenv(EnvEnableAllNewFeatures, "true")
		t.Setenv(EnvUsePipelineGenerator, "false")
		
		ff := GetFeatureFlags()
		ff.ClearCache()

		// Rollback scenario: Disable problematic feature
		assert.True(t, ff.UseNewTemplateEngine())
		assert.False(t, ff.UsePipelineGenerator()) // Rolled back
		assert.True(t, ff.UseNewConfigBuilder())
		assert.True(t, ff.UseServiceRegistry())
	})
}

// TestFeatureFlags_Documentation verifies that documentation constants are defined
func TestFeatureFlags_Documentation(t *testing.T) {
	require.NotEmpty(t, MigrationGuide, "Migration guide should be defined")
	require.Contains(t, MigrationGuide, "Feature Flag Migration Guide", "Migration guide should have title")
	require.Contains(t, MigrationGuide, EnvUseNewTemplateEngine, "Migration guide should mention template engine flag")
	require.Contains(t, MigrationGuide, EnvUsePipelineGenerator, "Migration guide should mention pipeline generator flag")
}
