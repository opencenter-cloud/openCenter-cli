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
	"strings"
	"sync"
)

// Feature flag environment variables for controlling system behavior during migration.
// These flags allow gradual migration from legacy systems to new implementations
// with the ability to rollback if issues are discovered.
const (
	// EnvUseNewTemplateEngine controls whether to use the new template engine (true)
	// or the legacy direct text/template implementation (false/unset).
	// This allows gradual migration and rollback capability for template rendering.
	EnvUseNewTemplateEngine = "OPENCENTER_USE_NEW_TEMPLATE_ENGINE"

	// EnvUsePipelineGenerator controls whether to use the new pipeline-based
	// GitOps generation system (true) or the legacy generation functions (false/unset).
	// This allows gradual migration and rollback capability for GitOps generation.
	EnvUsePipelineGenerator = "OPENCENTER_USE_PIPELINE_GENERATOR"

	// EnvUseNewConfigBuilder controls whether to use the new fluent configuration
	// builder (true) or the legacy reflection-based approach (false/unset).
	// This allows gradual migration and rollback capability for configuration building.
	EnvUseNewConfigBuilder = "OPENCENTER_USE_NEW_CONFIG_BUILDER"

	// EnvUseServiceRegistry controls whether to use the new service registry and
	// plugin system (true) or the legacy hardcoded service handling (false/unset).
	// This allows gradual migration and rollback capability for service management.
	EnvUseServiceRegistry = "OPENCENTER_USE_SERVICE_REGISTRY"

	// EnvEnableAllNewFeatures is a convenience flag that enables all new systems
	// at once. This is useful for testing the complete refactored system.
	// Individual flags take precedence over this global flag.
	EnvEnableAllNewFeatures = "OPENCENTER_ENABLE_ALL_NEW_FEATURES"

	// EnvFeatureFlagDebug enables debug logging for feature flag evaluation.
	// This helps troubleshoot which systems are being used during execution.
	EnvFeatureFlagDebug = "OPENCENTER_FEATURE_FLAG_DEBUG"
)

// FeatureFlags provides a centralized interface for checking feature flag status.
// This allows consistent feature flag evaluation across the codebase and provides
// a single place to manage feature flag logic.
type FeatureFlags struct {
	mu                    sync.RWMutex
	cache                 map[string]bool
	debugEnabled          bool
	allNewFeaturesEnabled bool
}

var (
	// globalFeatureFlags is the singleton instance used throughout the application
	globalFeatureFlags *FeatureFlags
	once               sync.Once
)

// GetFeatureFlags returns the global feature flags instance.
// This uses lazy initialization to ensure the instance is created only once.
func GetFeatureFlags() *FeatureFlags {
	once.Do(func() {
		globalFeatureFlags = &FeatureFlags{
			cache:                 make(map[string]bool),
			debugEnabled:          parseBoolEnv(EnvFeatureFlagDebug),
			allNewFeaturesEnabled: parseBoolEnv(EnvEnableAllNewFeatures),
		}
	})
	return globalFeatureFlags
}

// UseNewTemplateEngine returns true if the new template engine should be used.
func (ff *FeatureFlags) UseNewTemplateEngine() bool {
	return ff.isEnabled(EnvUseNewTemplateEngine, "new template engine")
}

// UsePipelineGenerator returns true if the new pipeline-based GitOps generator should be used.
func (ff *FeatureFlags) UsePipelineGenerator() bool {
	return ff.isEnabled(EnvUsePipelineGenerator, "pipeline generator")
}

// UseNewConfigBuilder returns true if the new fluent configuration builder should be used.
func (ff *FeatureFlags) UseNewConfigBuilder() bool {
	return ff.isEnabled(EnvUseNewConfigBuilder, "new config builder")
}

// UseServiceRegistry returns true if the new service registry and plugin system should be used.
func (ff *FeatureFlags) UseServiceRegistry() bool {
	return ff.isEnabled(EnvUseServiceRegistry, "service registry")
}

// isEnabled checks if a feature flag is enabled, with caching and debug logging.
func (ff *FeatureFlags) isEnabled(envVar, featureName string) bool {
	// Check cache first
	ff.mu.RLock()
	if cached, ok := ff.cache[envVar]; ok {
		ff.mu.RUnlock()
		return cached
	}
	ff.mu.RUnlock()

	// Evaluate the flag
	enabled := ff.evaluateFlag(envVar)

	// Cache the result
	ff.mu.Lock()
	ff.cache[envVar] = enabled
	ff.mu.Unlock()

	// Debug logging
	if ff.debugEnabled {
		status := "disabled"
		if enabled {
			status = "enabled"
		}
		// Use stderr to avoid interfering with command output
		os.Stderr.WriteString("[FEATURE FLAG] " + featureName + " is " + status + " (" + envVar + ")\n")
	}

	return enabled
}

// evaluateFlag evaluates a feature flag based on environment variables.
// It checks the specific flag first, then falls back to the global "all new features" flag.
func (ff *FeatureFlags) evaluateFlag(envVar string) bool {
	// Check specific flag first
	if value := os.Getenv(envVar); value != "" {
		return parseBoolEnv(envVar)
	}

	// Fall back to global "all new features" flag
	return ff.allNewFeaturesEnabled
}

// ClearCache clears the feature flag cache.
// This is useful for testing or when environment variables change at runtime.
func (ff *FeatureFlags) ClearCache() {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	ff.cache = make(map[string]bool)
	ff.allNewFeaturesEnabled = parseBoolEnv(EnvEnableAllNewFeatures)
	ff.debugEnabled = parseBoolEnv(EnvFeatureFlagDebug)
}

// GetStatus returns a map of all feature flags and their current status.
// This is useful for debugging and displaying feature flag status to users.
func (ff *FeatureFlags) GetStatus() map[string]bool {
	return map[string]bool{
		"new_template_engine":  ff.UseNewTemplateEngine(),
		"pipeline_generator":   ff.UsePipelineGenerator(),
		"new_config_builder":   ff.UseNewConfigBuilder(),
		"service_registry":     ff.UseServiceRegistry(),
		"all_new_features":     ff.allNewFeaturesEnabled,
		"debug_enabled":        ff.debugEnabled,
	}
}

// PrintStatus prints the current feature flag status to stderr.
// This is useful for debugging and troubleshooting.
func (ff *FeatureFlags) PrintStatus() {
	os.Stderr.WriteString("\n=== Feature Flag Status ===\n")
	status := ff.GetStatus()
	for name, enabled := range status {
		statusStr := "disabled"
		if enabled {
			statusStr = "enabled"
		}
		os.Stderr.WriteString(name + ": " + statusStr + "\n")
	}
	os.Stderr.WriteString("===========================\n\n")
}

// parseBoolEnv parses a boolean environment variable.
// Valid values for true: "true", "1", "yes", "on" (case-insensitive)
// Any other value or unset means false.
func parseBoolEnv(envVar string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(envVar)))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

// Package-level convenience functions for backward compatibility

// UseNewTemplateEngine returns true if the new template engine should be used.
// This is a convenience function that uses the global feature flags instance.
func UseNewTemplateEngine() bool {
	return GetFeatureFlags().UseNewTemplateEngine()
}

// UsePipelineGenerator returns true if the new pipeline-based GitOps generator should be used.
// This is a convenience function that uses the global feature flags instance.
func UsePipelineGenerator() bool {
	return GetFeatureFlags().UsePipelineGenerator()
}

// UseNewConfigBuilder returns true if the new fluent configuration builder should be used.
// This is a convenience function that uses the global feature flags instance.
func UseNewConfigBuilder() bool {
	return GetFeatureFlags().UseNewConfigBuilder()
}

// UseServiceRegistry returns true if the new service registry and plugin system should be used.
// This is a convenience function that uses the global feature flags instance.
func UseServiceRegistry() bool {
	return GetFeatureFlags().UseServiceRegistry()
}

// MigrationGuide provides documentation for using feature flags during migration.
const MigrationGuide = `
Feature Flag Migration Guide
=============================

The openCenter CLI uses feature flags to enable gradual migration from legacy
systems to new implementations. This allows testing new features in production
while maintaining the ability to quickly rollback if issues are discovered.

Available Feature Flags:
------------------------

1. OPENCENTER_USE_NEW_TEMPLATE_ENGINE
   Controls: Template rendering system
   Default: false (use legacy text/template)
   New: Enhanced template engine with caching and better error messages

2. OPENCENTER_USE_PIPELINE_GENERATOR
   Controls: GitOps repository generation
   Default: false (use legacy generation functions)
   New: Pipeline-based generation with rollback and progress reporting

3. OPENCENTER_USE_NEW_CONFIG_BUILDER
   Controls: Configuration building
   Default: false (use legacy reflection-based approach)
   New: Type-safe fluent builder with compile-time validation

4. OPENCENTER_USE_SERVICE_REGISTRY
   Controls: Service management
   Default: false (use legacy hardcoded services)
   New: Plugin-based service registry with dependency resolution

5. OPENCENTER_ENABLE_ALL_NEW_FEATURES
   Controls: All new systems at once
   Default: false
   Effect: Enables all new features (individual flags override this)

6. OPENCENTER_FEATURE_FLAG_DEBUG
   Controls: Feature flag debug logging
   Default: false
   Effect: Prints feature flag evaluation to stderr

Usage Examples:
---------------

Enable a single feature:
    export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
    openCenter cluster render

Enable all new features:
    export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
    openCenter cluster init my-cluster

Enable with debug logging:
    export OPENCENTER_FEATURE_FLAG_DEBUG=true
    export OPENCENTER_USE_PIPELINE_GENERATOR=true
    openCenter cluster render

Disable a specific feature when all are enabled:
    export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
    export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false
    openCenter cluster render

Valid Values:
-------------
To enable: "true", "1", "yes", "on" (case-insensitive)
To disable: Any other value or unset

Migration Timeline:
-------------------
Phase 1 (Current): All flags default to false, legacy systems active
Phase 2 (Testing): Flags can be enabled for testing, both systems available
Phase 3 (Transition): Flags default to true, new systems become default
Phase 4 (Cleanup): Legacy systems removed, flags no longer needed

Troubleshooting:
----------------
If you encounter issues with new features:
1. Disable the specific feature flag
2. Report the issue with debug logs (OPENCENTER_FEATURE_FLAG_DEBUG=true)
3. Continue using the legacy system while the issue is resolved

For more information:
- Design document: .kiro/specs/configuration-system-refactor/design.md
- Tasks document: .kiro/specs/configuration-system-refactor/tasks.md
`
