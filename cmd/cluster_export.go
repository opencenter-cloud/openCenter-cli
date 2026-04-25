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
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/spf13/cobra"
)

// newClusterExportCmd creates the "cluster export" command.
func newClusterExportCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export [name]",
		Short: "Export effective cluster configuration",
		Long: `Export the effective configuration including all applied defaults.

This command loads a cluster configuration, applies all defaults (provider-region,
provider, and global defaults), and exports the complete configuration with comments
indicating which values came from defaults vs explicit configuration.

The effective configuration shows:
  • All explicitly configured values
  • All applied defaults with their source (provider-region, provider, global)
  • Comments indicating default sources for transparency

This is useful for:
  • Understanding which defaults are being applied
  • Debugging configuration issues
  • Creating explicit configurations from defaults
  • Documentation and auditing

If no cluster name is provided, exports the currently active cluster.`,
		Example: `  # Export effective config for active cluster
  opencenter cluster export

  # Export effective config for specific cluster
  opencenter cluster export my-cluster

  # Export to specific file
  opencenter cluster export my-cluster -o /tmp/effective-config.yaml

  # Export with organization prefix
  opencenter cluster export myorg/my-cluster`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			name, err := resolveClusterNameForCommand(cmd, args, true)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Load config to get organization
			cfg, err := loadConfig(ctx, name)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Extract just the cluster name (without organization prefix)
			actualClusterName := extractClusterName(name)

			// Get configuration file path
			configPath, err := getConfigPath(ctx, actualClusterName, cfg.OpenCenter.Meta.Organization)
			if err != nil {
				return fmt.Errorf("failed to resolve configuration path: %w", err)
			}

			return exportV2EffectiveConfig(cmd, configPath, outputPath)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output-file", "o", "", "write effective configuration to this file instead of stdout")

	return cmd
}

// exportV2EffectiveConfig exports effective configuration for v2 schema.
// For v2, we use the loader's ExportEffectiveConfig method which includes
// detailed comments about default sources.
// Requirements: 15.7, 15.8
func exportV2EffectiveConfig(cmd *cobra.Command, configPath, outputPath string) error {
	// Create v2 loader with default registry
	registry := defaults.NewRegistry()
	loader := v2.NewConfigLoader(registry)

	// Load configuration (this applies defaults during loading)
	cfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Export effective configuration with applied defaults
	effectiveConfig, err := loader.ExportEffectiveConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to export effective configuration: %w", err)
	}

	if outputPath == "" {
		fmt.Fprint(cmd.OutOrStdout(), string(effectiveConfig))
		return nil
	}

	if err := os.WriteFile(outputPath, effectiveConfig, 0600); err != nil {
		return fmt.Errorf("failed to write effective configuration: %w", err)
	}

	// Get applied defaults for summary
	appliedDefaults := loader.GetAppliedDefaults()

	// Resolve absolute path for display
	absPath, _ := filepath.Abs(outputPath)

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Effective configuration exported to: %s\n", absPath)
	fmt.Fprintf(cmd.OutOrStdout(), "  Schema version: 2.0\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Cluster: %s\n", cfg.OpenCenter.Meta.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "  Provider: %s\n", cfg.OpenCenter.Infrastructure.Provider)
	fmt.Fprintf(cmd.OutOrStdout(), "  Region: %s\n", cfg.OpenCenter.Meta.Region)
	fmt.Fprintf(cmd.OutOrStdout(), "  Applied defaults: %d fields\n", len(appliedDefaults))

	// Show summary of default sources
	sourceCounts := make(map[defaults.DefaultSource]int)
	for _, source := range appliedDefaults {
		sourceCounts[source]++
	}

	if len(sourceCounts) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Default sources:\n")
		for source, count := range sourceCounts {
			fmt.Fprintf(cmd.OutOrStdout(), "    - %s: %d fields\n", source, count)
		}
	}

	return nil
}
