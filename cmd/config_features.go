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

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func newConfigFeaturesCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "features",
		Short: "Display feature flag status",
		Long: `Display the current status of feature flags that control system behavior.

Feature flags allow gradual migration from legacy systems to new implementations
with the ability to rollback if issues are discovered. This command shows which
features are currently enabled and provides guidance on how to use them.

Available feature flags:
  - OPENCENTER_USE_NEW_TEMPLATE_ENGINE: New template engine with caching
  - OPENCENTER_USE_PIPELINE_GENERATOR: Pipeline-based GitOps generation
  - OPENCENTER_USE_NEW_CONFIG_BUILDER: Type-safe configuration builder
  - OPENCENTER_USE_SERVICE_REGISTRY: Plugin-based service registry
  - OPENCENTER_ENABLE_ALL_NEW_FEATURES: Enable all new features at once
  - OPENCENTER_FEATURE_FLAG_DEBUG: Enable debug logging for feature flags

Set feature flags using environment variables:
  export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
  export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true

Valid values: "true", "1", "yes", "on" (case-insensitive)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ff := config.GetFeatureFlags()
			status := ff.GetStatus()

			switch outputFormat {
			case "json":
				return printFeaturesJSON(cmd, status)
			case "table":
				return printFeaturesTable(cmd, status)
			case "env":
				return printFeaturesEnv(cmd, status)
			default:
				return fmt.Errorf("unsupported output format: %s (use 'json', 'table', or 'env')", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: json, table, or env")

	return cmd
}

// printFeaturesJSON prints feature flag status as JSON
func printFeaturesJSON(cmd *cobra.Command, status map[string]bool) error {
	// Create a more detailed structure for JSON output
	output := map[string]interface{}{
		"features": map[string]interface{}{
			"new_template_engine": map[string]interface{}{
				"enabled":     status["new_template_engine"],
				"env_var":     config.EnvUseNewTemplateEngine,
				"description": "Enhanced template engine with caching and better error messages",
			},
			"pipeline_generator": map[string]interface{}{
				"enabled":     status["pipeline_generator"],
				"env_var":     config.EnvUsePipelineGenerator,
				"description": "Pipeline-based GitOps generation with rollback and progress reporting",
			},
			"new_config_builder": map[string]interface{}{
				"enabled":     status["new_config_builder"],
				"env_var":     config.EnvUseNewConfigBuilder,
				"description": "Type-safe fluent configuration builder",
			},
			"service_registry": map[string]interface{}{
				"enabled":     status["service_registry"],
				"env_var":     config.EnvUseServiceRegistry,
				"description": "Plugin-based service registry with dependency resolution",
			},
		},
		"global": map[string]interface{}{
			"all_new_features": map[string]interface{}{
				"enabled":     status["all_new_features"],
				"env_var":     config.EnvEnableAllNewFeatures,
				"description": "Enable all new features at once",
			},
			"debug_enabled": map[string]interface{}{
				"enabled":     status["debug_enabled"],
				"env_var":     config.EnvFeatureFlagDebug,
				"description": "Enable debug logging for feature flag evaluation",
			},
		},
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// printFeaturesTable prints feature flag status as a formatted table
func printFeaturesTable(cmd *cobra.Command, status map[string]bool) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "FEATURE\tSTATUS\tENVIRONMENT VARIABLE\tDESCRIPTION")
	fmt.Fprintln(w, "-------\t------\t--------------------\t-----------")

	// Individual features
	printFeatureRow(w, "Template Engine", status["new_template_engine"],
		config.EnvUseNewTemplateEngine,
		"Enhanced template engine with caching")

	printFeatureRow(w, "Pipeline Generator", status["pipeline_generator"],
		config.EnvUsePipelineGenerator,
		"Pipeline-based GitOps generation")

	printFeatureRow(w, "Config Builder", status["new_config_builder"],
		config.EnvUseNewConfigBuilder,
		"Type-safe configuration builder")

	printFeatureRow(w, "Service Registry", status["service_registry"],
		config.EnvUseServiceRegistry,
		"Plugin-based service registry")

	// Separator
	fmt.Fprintln(w, "")

	// Global flags
	printFeatureRow(w, "All New Features", status["all_new_features"],
		config.EnvEnableAllNewFeatures,
		"Enable all new features at once")

	printFeatureRow(w, "Debug Logging", status["debug_enabled"],
		config.EnvFeatureFlagDebug,
		"Feature flag debug logging")

	return nil
}

// printFeatureRow prints a single feature row in the table
func printFeatureRow(w *tabwriter.Writer, name string, enabled bool, envVar, description string) {
	statusStr := "disabled"
	if enabled {
		statusStr = "enabled"
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, statusStr, envVar, description)
}

// printFeaturesEnv prints feature flag status as environment variable export statements
func printFeaturesEnv(cmd *cobra.Command, status map[string]bool) error {
	fmt.Fprintln(cmd.OutOrStdout(), "# Feature Flag Environment Variables")
	fmt.Fprintln(cmd.OutOrStdout(), "# Copy and paste these commands to enable/disable features")
	fmt.Fprintln(cmd.OutOrStdout(), "")

	printEnvExport(cmd, config.EnvUseNewTemplateEngine, status["new_template_engine"],
		"Enhanced template engine with caching")
	printEnvExport(cmd, config.EnvUsePipelineGenerator, status["pipeline_generator"],
		"Pipeline-based GitOps generation")
	printEnvExport(cmd, config.EnvUseNewConfigBuilder, status["new_config_builder"],
		"Type-safe configuration builder")
	printEnvExport(cmd, config.EnvUseServiceRegistry, status["service_registry"],
		"Plugin-based service registry")

	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "# Global flags")

	printEnvExport(cmd, config.EnvEnableAllNewFeatures, status["all_new_features"],
		"Enable all new features at once")
	printEnvExport(cmd, config.EnvFeatureFlagDebug, status["debug_enabled"],
		"Feature flag debug logging")

	return nil
}

// printEnvExport prints an environment variable export statement
func printEnvExport(cmd *cobra.Command, envVar string, enabled bool, description string) {
	value := "false"
	if enabled {
		value = "true"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "# %s\n", description)
	fmt.Fprintf(cmd.OutOrStdout(), "export %s=%s\n", envVar, value)
	fmt.Fprintln(cmd.OutOrStdout(), "")
}

func init() {
	// Check if we should print feature flag status on startup
	if os.Getenv(config.EnvFeatureFlagDebug) != "" {
		config.GetFeatureFlags().PrintStatus()
	}
}
