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

package config_test

import (
	"fmt"
	"os"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// Example_featureFlagsStructuredLogging demonstrates how feature flag evaluation
// produces structured logs that can be used for monitoring and debugging.
func Example_featureFlagsStructuredLogging() {
	// Set up feature flags
	os.Setenv(config.EnvUseNewTemplateEngine, "true")
	os.Setenv(config.EnvUsePipelineGenerator, "false")

	// Initialize logging (in production, this would be done at startup)
	loggingConfig := config.DefaultCLIConfig().Logging
	loggingConfig.Level = "info"
	loggingConfig.Format = "json"
	config.InitializeLogging(&loggingConfig)

	// Get feature flags instance
	ff := config.GetFeatureFlags()

	// Check feature flags - this will produce structured logs
	templateEngineEnabled := ff.UseNewTemplateEngine()
	pipelineGeneratorEnabled := ff.UsePipelineGenerator()

	fmt.Printf("Template Engine: %v\n", templateEngineEnabled)
	fmt.Printf("Pipeline Generator: %v\n", pipelineGeneratorEnabled)

	// The structured logs will include:
	// - component: "feature_flags"
	// - operation: "evaluation"
	// - feature_name: "new template engine" or "pipeline generator"
	// - env_var: the environment variable name
	// - enabled: true or false
	// - source: "environment", "all_new_features", or "default"

	// Clean up
	os.Unsetenv(config.EnvUseNewTemplateEngine)
	os.Unsetenv(config.EnvUsePipelineGenerator)

	// Output:
	// Template Engine: true
	// Pipeline Generator: false
}

// Example_featureFlagsDebugMode demonstrates how to enable debug mode
// for feature flag evaluation, which provides additional stderr output.
func Example_featureFlagsDebugMode() {
	// Enable debug mode
	os.Setenv(config.EnvFeatureFlagDebug, "true")
	os.Setenv(config.EnvUseNewTemplateEngine, "true")

	// Get feature flags instance
	ff := config.GetFeatureFlags()

	// This will produce both structured logs AND stderr output
	enabled := ff.UseNewTemplateEngine()

	fmt.Printf("Enabled: %v\n", enabled)

	// Debug mode produces stderr output like:
	// [FEATURE FLAG] new template engine is enabled (OPENCENTER_USE_NEW_TEMPLATE_ENGINE, source: environment)

	// Clean up
	os.Unsetenv(config.EnvFeatureFlagDebug)
	os.Unsetenv(config.EnvUseNewTemplateEngine)

	// Output:
	// Enabled: true
}

// Example_featureFlagsGetStatus demonstrates how to get the current status
// of all feature flags for monitoring and debugging.
func Example_featureFlagsGetStatus() {
	// Clear any previous state
	os.Unsetenv(config.EnvUseNewTemplateEngine)
	os.Unsetenv(config.EnvUsePipelineGenerator)
	os.Unsetenv(config.EnvUseNewConfigBuilder)
	os.Unsetenv(config.EnvUseServiceRegistry)
	os.Unsetenv(config.EnvEnableAllNewFeatures)

	// Set up some feature flags
	os.Setenv(config.EnvUseNewTemplateEngine, "true")
	os.Setenv(config.EnvUsePipelineGenerator, "true")

	// Get feature flags instance and clear cache to pick up new env vars
	ff := config.GetFeatureFlags()
	ff.ClearCache()

	// Get status of all flags
	status := ff.GetStatus()

	fmt.Printf("New Template Engine: %v\n", status["new_template_engine"])
	fmt.Printf("Pipeline Generator: %v\n", status["pipeline_generator"])
	fmt.Printf("New Config Builder: %v\n", status["new_config_builder"])
	fmt.Printf("Service Registry: %v\n", status["service_registry"])

	// Clean up
	os.Unsetenv(config.EnvUseNewTemplateEngine)
	os.Unsetenv(config.EnvUsePipelineGenerator)

	// Output:
	// New Template Engine: true
	// Pipeline Generator: true
	// New Config Builder: false
	// Service Registry: false
}
