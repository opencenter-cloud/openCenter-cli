/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUseNewTemplateEngine tests the feature flag detection logic.
func TestUseNewTemplateEngine(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "unset returns false",
			envValue: "",
			expected: false,
		},
		{
			name:     "empty string returns false",
			envValue: " ",
			expected: false,
		},
		{
			name:     "true returns true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "TRUE uppercase returns true",
			envValue: "TRUE",
			expected: true,
		},
		{
			name:     "True mixed case returns true",
			envValue: "True",
			expected: true,
		},
		{
			name:     "1 returns true",
			envValue: "1",
			expected: true,
		},
		{
			name:     "yes returns true",
			envValue: "yes",
			expected: true,
		},
		{
			name:     "YES uppercase returns true",
			envValue: "YES",
			expected: true,
		},
		{
			name:     "on returns true",
			envValue: "on",
			expected: true,
		},
		{
			name:     "ON uppercase returns true",
			envValue: "ON",
			expected: true,
		},
		{
			name:     "false returns false",
			envValue: "false",
			expected: false,
		},
		{
			name:     "0 returns false",
			envValue: "0",
			expected: false,
		},
		{
			name:     "no returns false",
			envValue: "no",
			expected: false,
		},
		{
			name:     "off returns false",
			envValue: "off",
			expected: false,
		},
		{
			name:     "invalid value returns false",
			envValue: "invalid",
			expected: false,
		},
		{
			name:     "random string returns false",
			envValue: "random",
			expected: false,
		},
		{
			name:     "whitespace around true is trimmed",
			envValue: "  true  ",
			expected: true,
		},
		{
			name:     "whitespace around yes is trimmed",
			envValue: " yes ",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				t.Setenv(EnvUseNewTemplateEngine, tt.envValue)
			} else {
				// Ensure it's unset
				os.Unsetenv(EnvUseNewTemplateEngine)
			}

			result := UseNewTemplateEngine()
			assert.Equal(t, tt.expected, result,
				"UseNewTemplateEngine() with env=%q should return %v", tt.envValue, tt.expected)
		})
	}
}

// TestEnvUseNewTemplateEngineConstant verifies the constant value is correct.
func TestEnvUseNewTemplateEngineConstant(t *testing.T) {
	assert.Equal(t, "OPENCENTER_USE_NEW_TEMPLATE_ENGINE", EnvUseNewTemplateEngine,
		"Environment variable constant should match expected value")
}

// TestFeatureFlagDefault verifies the default behavior when env var is not set.
func TestFeatureFlagDefault(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv(EnvUseNewTemplateEngine)

	// Default should be false (use legacy system)
	assert.False(t, UseNewTemplateEngine(),
		"Default behavior should use legacy system when feature flag is not set")
}

// TestFeatureFlagCaseInsensitive verifies case-insensitive matching.
func TestFeatureFlagCaseInsensitive(t *testing.T) {
	testCases := []string{
		"true", "TRUE", "True", "TrUe",
		"yes", "YES", "Yes", "YeS",
		"on", "ON", "On",
		"1",
	}

	for _, value := range testCases {
		t.Run(value, func(t *testing.T) {
			t.Setenv(EnvUseNewTemplateEngine, value)
			assert.True(t, UseNewTemplateEngine(),
				"Value %q should enable new template engine (case-insensitive)", value)
		})
	}
}

// TestFeatureFlagWhitespaceHandling verifies whitespace is properly trimmed.
func TestFeatureFlagWhitespaceHandling(t *testing.T) {
	testCases := []struct {
		value    string
		expected bool
	}{
		{"  true  ", true},
		{"\ttrue\t", true},
		{"\ntrue\n", true},
		{"  yes  ", true},
		{"  1  ", true},
		{"  false  ", false},
		{"  no  ", false},
		{"  ", false},
		{"\t\n", false},
	}

	for _, tc := range testCases {
		t.Run("whitespace_"+tc.value, func(t *testing.T) {
			t.Setenv(EnvUseNewTemplateEngine, tc.value)
			assert.Equal(t, tc.expected, UseNewTemplateEngine(),
				"Value %q should return %v after trimming whitespace", tc.value, tc.expected)
		})
	}
}
