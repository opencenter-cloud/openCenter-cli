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

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/config/defaults"
	"github.com/rackerlabs/opencenter-cli/internal/config/v2"
)

// newClusterMigrateConfigCmd creates the cluster migrate-config command.
// Requirements: 12.1, 12.8
func newClusterMigrateConfigCmd() *cobra.Command {
	var outputPath string
	var showReport bool

	cmd := &cobra.Command{
		Use:   "migrate-config <input-config>",
		Short: "Migrate v1 configuration to v2 format",
		Long: `Migrate a v1 cluster configuration to v2 format.

This command converts a v1 configuration file to the new v2 schema format,
relocating fields to their new locations and applying hydration to make
implicit defaults explicit.

The migration process:
1. Loads the v1 configuration
2. Applies hydration to make implicit defaults explicit
3. Relocates fields to their v2 locations
4. Validates the migrated configuration
5. Writes the v2 configuration to the output file

Field relocations:
- VRRP IP: cluster.networking.vrrp_ip → infrastructure.networking.vrrp_ip
- Networking: cluster.networking → infrastructure.networking
- Compute: cluster.kubernetes → infrastructure.compute
- Storage: opencenter.storage → infrastructure.storage
- SSH: cluster → infrastructure.ssh

Examples:
  # Migrate a v1 config to v2 (output to same directory with -v2 suffix)
  opencenter cluster migrate-config cluster-config.yaml

  # Migrate with custom output path
  opencenter cluster migrate-config cluster-config.yaml --output cluster-config-v2.yaml

  # Show migration report
  opencenter cluster migrate-config cluster-config.yaml --report
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputPath := args[0]

			// Default output path if not specified
			if outputPath == "" {
				ext := filepath.Ext(inputPath)
				base := inputPath[:len(inputPath)-len(ext)]
				outputPath = base + "-v2" + ext
			}

			return runMigrateConfig(inputPath, outputPath, showReport)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for v2 configuration (default: <input>-v2.yaml)")
	cmd.Flags().BoolVarP(&showReport, "report", "r", false, "Show migration report")

	return cmd
}

// runMigrateConfig executes the configuration migration.
// Requirements: 12.1, 12.8
func runMigrateConfig(inputPath, outputPath string, showReport bool) error {
	// Load v1 configuration
	fmt.Printf("Loading v1 configuration from %s...\n", inputPath)
	v1Config, err := loadV1Config(inputPath)
	if err != nil {
		return fmt.Errorf("failed to load v1 configuration: %w", err)
	}

	// Create hydrator with default registry
	registry := defaults.NewRegistry()
	hydrator := defaults.NewHydrator(registry)

	// Create migrator
	migrator := v2.NewMigrator(hydrator)

	// Migrate configuration
	fmt.Println("Migrating configuration to v2 format...")
	v2Config, err := migrator.Migrate(v1Config)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Validate migration
	fmt.Println("Validating migrated configuration...")
	if err := migrator.ValidateMigration(v1Config, v2Config); err != nil {
		return fmt.Errorf("migration validation failed: %w", err)
	}

	// Write v2 configuration
	fmt.Printf("Writing v2 configuration to %s...\n", outputPath)
	if err := writeV2Config(v2Config, outputPath); err != nil {
		return fmt.Errorf("failed to write v2 configuration: %w", err)
	}

	// Generate and display migration report if requested
	if showReport {
		fmt.Println("\n=== Migration Report ===")
		report, err := migrator.GenerateMigrationReport(v1Config, v2Config)
		if err != nil {
			return fmt.Errorf("failed to generate migration report: %w", err)
		}

		displayMigrationReport(report)
	}

	fmt.Printf("\n✓ Successfully migrated configuration to v2 format\n")
	fmt.Printf("  Input:  %s\n", inputPath)
	fmt.Printf("  Output: %s\n", outputPath)

	return nil
}

// loadV1Config loads a v1 configuration from a file.
func loadV1Config(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Verify it's a v1 config
	if cfg.SchemaVersion != "" && cfg.SchemaVersion != "1.0" {
		if cfg.SchemaVersion == "2.0" {
			return nil, fmt.Errorf("configuration is already v2.0 - no migration needed\n\nTo validate your v2 configuration, use:\n  opencenter cluster validate %s", path)
		}
		return nil, fmt.Errorf("input configuration is not v1 (schema_version: %s)", cfg.SchemaVersion)
	}

	return &cfg, nil
}

// writeV2Config writes a v2 configuration to a file.
func writeV2Config(cfg *v2.Config, path string) error {
	// Ensure output directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// displayMigrationReport displays the migration report to the user.
func displayMigrationReport(report *v2.MigrationReport) {
	// Display moved fields
	if len(report.MovedFields) > 0 {
		fmt.Println("\nField Relocations:")
		for oldPath, newPath := range report.MovedFields {
			fmt.Printf("  %s → %s\n", oldPath, newPath)
		}
	}

	// Display applied defaults
	if len(report.AppliedDefaults) > 0 {
		fmt.Println("\nApplied Defaults:")
		for field, source := range report.AppliedDefaults {
			fmt.Printf("  %s: %s\n", field, source)
		}
	}

	// Display warnings
	if len(report.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, warning := range report.Warnings {
			fmt.Printf("  ⚠ %s\n", warning)
		}
	}
}
