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
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFeatureFlagStructuredLogging tests that feature flag evaluation produces structured logs
func TestFeatureFlagStructuredLogging(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{
		EnvUseNewTemplateEngine: os.Getenv(EnvUseNewTemplateEngine),
		EnvUsePipelineGenerator: os.Getenv(EnvUsePipelineGenerator),
		EnvUseNewConfigBuilder:  os.Getenv(EnvUseNewConfigBuilder),
		EnvUseServiceRegistry:   os.Getenv(EnvUseServiceRegistry),
		EnvEnableAllNewFeatures: os.Getenv(EnvEnableAllNewFeatures),
		EnvFeatureFlagDebug:     os.Getenv(EnvFeatureFlagDebug),
	}
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Clear all environment variables
	for key := range originalEnv {
		os.Unsetenv(key)
	}

	// Reset global state
	globalFeatureFlags = nil
	once = sync.Once{}

	// Create a buffer to capture log output
	var logBuffer bytes.Buffer
	testLogger := logrus.New()
	testLogger.SetOutput(&logBuffer)
	testLogger.SetFormatter(&logrus.JSONFormatter{})
	testLogger.SetLevel(logrus.DebugLevel)

	// Override the global logger for testing
	Logger = testLogger

	// Get feature flags instance (this will trigger initialization logging)
	ff := GetFeatureFlags()
	ff.logger = testLogger

	// Clear the buffer to start fresh
	logBuffer.Reset()

	// Test 1: Evaluate a feature flag and check structured logging
	t.Run("feature flag evaluation logging", func(t *testing.T) {
		logBuffer.Reset()
		os.Setenv(EnvUseNewTemplateEngine, "true")
		ff.ClearCache()

		enabled := ff.UseNewTemplateEngine()
		assert.True(t, enabled)

		// Parse the log output
		logLines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
		require.Greater(t, len(logLines), 0, "Expected at least one log line")

		// Find the evaluation log entry
		var evalLog map[string]interface{}
		for _, line := range logLines {
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
				if logEntry["operation"] == "evaluation" {
					evalLog = logEntry
					break
				}
			}
		}

		require.NotNil(t, evalLog, "Expected to find evaluation log entry")
		assert.Equal(t, "feature_flags", evalLog["component"])
		assert.Equal(t, "evaluation", evalLog["operation"])
		assert.Equal(t, "new template engine", evalLog["feature_name"])
		assert.Equal(t, EnvUseNewTemplateEngine, evalLog["env_var"])
		assert.Equal(t, true, evalLog["enabled"])
		assert.Equal(t, "environment", evalLog["source"])
	})

	// Test 2: Check initialization logging
	t.Run("initialization logging", func(t *testing.T) {
		logBuffer.Reset()
		os.Setenv(EnvUseNewTemplateEngine, "true")
		os.Setenv(EnvUsePipelineGenerator, "false")

		// Reset to trigger initialization
		globalFeatureFlags = nil
		once = sync.Once{}

		ff := GetFeatureFlags()
		ff.logger = testLogger

		// Parse the log output
		logLines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
		require.Greater(t, len(logLines), 0, "Expected at least one log line")

		// Find the initialization log entry
		var initLog map[string]interface{}
		for _, line := range logLines {
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
				if logEntry["operation"] == "initialization" {
					initLog = logEntry
					break
				}
			}
		}

		require.NotNil(t, initLog, "Expected to find initialization log entry")
		assert.Equal(t, "feature_flags", initLog["component"])
		assert.Equal(t, "initialization", initLog["operation"])
		assert.Contains(t, initLog, "new_template_engine")
		assert.Contains(t, initLog, "pipeline_generator")
		assert.Contains(t, initLog, "new_config_builder")
		assert.Contains(t, initLog, "service_registry")
	})

	// Test 3: Check cache clear logging
	t.Run("cache clear logging", func(t *testing.T) {
		logBuffer.Reset()
		os.Setenv(EnvEnableAllNewFeatures, "true")

		ff.ClearCache()

		// Parse the log output
		logLines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
		require.Greater(t, len(logLines), 0, "Expected at least one log line")

		// Find the cache clear log entry
		var clearLog map[string]interface{}
		for _, line := range logLines {
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
				if logEntry["operation"] == "cache_clear" {
					clearLog = logEntry
					break
				}
			}
		}

		require.NotNil(t, clearLog, "Expected to find cache_clear log entry")
		assert.Equal(t, "feature_flags", clearLog["component"])
		assert.Equal(t, "cache_clear", clearLog["operation"])
		assert.Contains(t, clearLog, "all_new_features_before")
		assert.Contains(t, clearLog, "all_new_features_after")
		assert.Contains(t, clearLog, "debug_enabled_before")
		assert.Contains(t, clearLog, "debug_enabled_after")
	})

	// Test 4: Check source tracking (environment vs all_new_features vs default)
	t.Run("source tracking", func(t *testing.T) {
		testCases := []struct {
			name           string
			envVars        map[string]string
			expectedSource string
		}{
			{
				name: "environment source",
				envVars: map[string]string{
					EnvUseNewTemplateEngine: "true",
				},
				expectedSource: "environment",
			},
			{
				name: "all_new_features source",
				envVars: map[string]string{
					EnvEnableAllNewFeatures: "true",
				},
				expectedSource: "all_new_features",
			},
			{
				name:           "default source",
				envVars:        map[string]string{},
				expectedSource: "default",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Clear environment
				for key := range originalEnv {
					os.Unsetenv(key)
				}

				// Set test environment
				for key, value := range tc.envVars {
					os.Setenv(key, value)
				}

				// Reset and get new instance
				globalFeatureFlags = nil
				once = sync.Once{}
				logBuffer.Reset()

				ff := GetFeatureFlags()
				ff.logger = testLogger

				// Clear buffer and cache, then evaluate flag
				logBuffer.Reset()
				ff.ClearCache()
				logBuffer.Reset() // Clear again after cache clear logging
				ff.UseNewTemplateEngine()

				// Parse the log output
				logLines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
				require.Greater(t, len(logLines), 0, "Expected at least one log line")

				// Find the evaluation log entry
				var evalLog map[string]interface{}
				for _, line := range logLines {
					var logEntry map[string]interface{}
					if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
						if logEntry["operation"] == "evaluation" {
							evalLog = logEntry
							break
						}
					}
				}

				require.NotNil(t, evalLog, "Expected to find evaluation log entry")
				assert.Equal(t, tc.expectedSource, evalLog["source"])
			})
		}
	})

	// Test 5: Check debug mode stderr output
	t.Run("debug mode stderr output", func(t *testing.T) {
		// Clear environment
		for key := range originalEnv {
			os.Unsetenv(key)
		}

		os.Setenv(EnvFeatureFlagDebug, "true")
		os.Setenv(EnvUseNewTemplateEngine, "true")

		// Reset and get new instance
		globalFeatureFlags = nil
		once = sync.Once{}

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		ff := GetFeatureFlags()
		ff.logger = testLogger
		ff.ClearCache()
		ff.UseNewTemplateEngine()

		w.Close()
		os.Stderr = oldStderr

		var stderrBuf bytes.Buffer
		stderrBuf.ReadFrom(r)
		stderrOutput := stderrBuf.String()

		// Check that stderr contains the expected output
		assert.Contains(t, stderrOutput, "[FEATURE FLAG]")
		assert.Contains(t, stderrOutput, "new template engine")
		assert.Contains(t, stderrOutput, "enabled")
		assert.Contains(t, stderrOutput, "source:")
	})
}

// TestFeatureFlagLoggingFields tests that all expected fields are present in logs
func TestFeatureFlagLoggingFields(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{
		EnvUseNewTemplateEngine: os.Getenv(EnvUseNewTemplateEngine),
		EnvUsePipelineGenerator: os.Getenv(EnvUsePipelineGenerator),
		EnvUseNewConfigBuilder:  os.Getenv(EnvUseNewConfigBuilder),
		EnvUseServiceRegistry:   os.Getenv(EnvUseServiceRegistry),
		EnvEnableAllNewFeatures: os.Getenv(EnvEnableAllNewFeatures),
		EnvFeatureFlagDebug:     os.Getenv(EnvFeatureFlagDebug),
	}
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Clear all environment variables
	for key := range originalEnv {
		os.Unsetenv(key)
	}

	// Reset global state
	globalFeatureFlags = nil
	once = sync.Once{}

	// Create a buffer to capture log output
	var logBuffer bytes.Buffer
	testLogger := logrus.New()
	testLogger.SetOutput(&logBuffer)
	testLogger.SetFormatter(&logrus.JSONFormatter{})
	testLogger.SetLevel(logrus.DebugLevel)

	// Override the global logger for testing
	Logger = testLogger

	os.Setenv(EnvUseNewTemplateEngine, "true")

	ff := GetFeatureFlags()
	ff.logger = testLogger

	logBuffer.Reset()
	ff.ClearCache()
	logBuffer.Reset() // Clear again after cache clear logging
	ff.UseNewTemplateEngine()

	// Parse the log output
	logLines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
	require.Greater(t, len(logLines), 0, "Expected at least one log line")

	// Find the evaluation log entry
	var evalLog map[string]interface{}
	for _, line := range logLines {
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
			if logEntry["operation"] == "evaluation" {
				evalLog = logEntry
				break
			}
		}
	}

	require.NotNil(t, evalLog, "Expected to find evaluation log entry")

	// Check all required fields are present
	requiredFields := []string{
		"component",
		"operation",
		"feature_name",
		"env_var",
		"enabled",
		"source",
		"level",
		"msg",
		"time",
	}

	for _, field := range requiredFields {
		assert.Contains(t, evalLog, field, "Expected log to contain field: %s", field)
	}
}

// TestFeatureFlagActiveCount tests that the active feature count is logged correctly
func TestFeatureFlagActiveCount(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{
		EnvUseNewTemplateEngine: os.Getenv(EnvUseNewTemplateEngine),
		EnvUsePipelineGenerator: os.Getenv(EnvUsePipelineGenerator),
		EnvUseNewConfigBuilder:  os.Getenv(EnvUseNewConfigBuilder),
		EnvUseServiceRegistry:   os.Getenv(EnvUseServiceRegistry),
		EnvEnableAllNewFeatures: os.Getenv(EnvEnableAllNewFeatures),
		EnvFeatureFlagDebug:     os.Getenv(EnvFeatureFlagDebug),
	}
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	testCases := []struct {
		name          string
		envVars       map[string]string
		expectedCount float64
	}{
		{
			name:          "no features enabled",
			envVars:       map[string]string{},
			expectedCount: 0,
		},
		{
			name: "one feature enabled",
			envVars: map[string]string{
				EnvUseNewTemplateEngine: "true",
			},
			expectedCount: 1,
		},
		{
			name: "two features enabled",
			envVars: map[string]string{
				EnvUseNewTemplateEngine: "true",
				EnvUsePipelineGenerator: "true",
			},
			expectedCount: 2,
		},
		{
			name: "all features enabled via individual flags",
			envVars: map[string]string{
				EnvUseNewTemplateEngine: "true",
				EnvUsePipelineGenerator: "true",
				EnvUseNewConfigBuilder:  "true",
				EnvUseServiceRegistry:   "true",
			},
			expectedCount: 4,
		},
		{
			name: "all features enabled via global flag",
			envVars: map[string]string{
				EnvEnableAllNewFeatures: "true",
			},
			expectedCount: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear environment
			for key := range originalEnv {
				os.Unsetenv(key)
			}

			// Set test environment
			for key, value := range tc.envVars {
				os.Setenv(key, value)
			}

			// Reset global state
			globalFeatureFlags = nil
			once = sync.Once{}

			// Create a buffer to capture log output
			var logBuffer bytes.Buffer
			testLogger := logrus.New()
			testLogger.SetOutput(&logBuffer)
			testLogger.SetFormatter(&logrus.JSONFormatter{})
			testLogger.SetLevel(logrus.InfoLevel)

			// Override the global logger for testing
			Logger = testLogger

			// Get feature flags instance (this will trigger initialization logging)
			ff := GetFeatureFlags()
			ff.logger = testLogger

			// Parse the log output
			logLines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
			require.Greater(t, len(logLines), 0, "Expected at least one log line")

			// Find the summary log entry
			var summaryLog map[string]interface{}
			for _, line := range logLines {
				var logEntry map[string]interface{}
				if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
					if _, hasActiveFeatures := logEntry["active_features"]; hasActiveFeatures {
						summaryLog = logEntry
						break
					}
				}
			}

			require.NotNil(t, summaryLog, "Expected to find summary log entry")
			assert.Equal(t, tc.expectedCount, summaryLog["active_features"])
			assert.Equal(t, float64(4), summaryLog["total_features"])
		})
	}
}
