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
	"fmt"
	"log"
)

// DeprecationWarning represents a deprecation warning for v1 configurations.
type DeprecationWarning struct {
	Field   string
	Message string
	Action  string
}

// BackwardCompatibilityManager manages backward compatibility between v1 and v2 schemas.
// Requirements: 13.1, 13.5, 13.6, 13.7
type BackwardCompatibilityManager struct {
	warnings []DeprecationWarning
}

// NewBackwardCompatibilityManager creates a new backward compatibility manager.
func NewBackwardCompatibilityManager() *BackwardCompatibilityManager {
	return &BackwardCompatibilityManager{
		warnings: []DeprecationWarning{},
	}
}

// CheckV1Deprecations checks a v1 configuration for deprecated features and generates warnings.
// Requirements: 13.5, 13.6
func (bcm *BackwardCompatibilityManager) CheckV1Deprecations(cfg *Config) []DeprecationWarning {
	bcm.warnings = []DeprecationWarning{}

	// Check for v1 schema version
	if cfg.SchemaVersion == "" || cfg.SchemaVersion == "1.0" {
		bcm.warnings = append(bcm.warnings, DeprecationWarning{
			Field:   "schema_version",
			Message: "v1 configuration schema is deprecated and will be removed in a future release",
			Action:  "Migrate to v2 using: opencenter cluster migrate-config <config-file>",
		})
	}

	// Check for deprecated field locations
	if cfg.OpenCenter.Cluster.Networking.VRRPIP != "" {
		bcm.warnings = append(bcm.warnings, DeprecationWarning{
			Field:   "opencenter.cluster.networking.vrrp_ip",
			Message: "VRRP IP location is deprecated in v1",
			Action:  "In v2, this field is located at infrastructure.networking.vrrp_ip",
		})
	}

	if cfg.OpenCenter.Cluster.Kubernetes.FlavorMaster != "" ||
		cfg.OpenCenter.Cluster.Kubernetes.FlavorWorker != "" {
		bcm.warnings = append(bcm.warnings, DeprecationWarning{
			Field:   "opencenter.cluster.kubernetes.flavor_*",
			Message: "Flavor configuration location is deprecated in v1",
			Action:  "In v2, flavor fields are located at infrastructure.compute.flavor_*",
		})
	}

	if cfg.OpenCenter.Storage.DefaultStorageClass != "" {
		bcm.warnings = append(bcm.warnings, DeprecationWarning{
			Field:   "opencenter.storage",
			Message: "Storage configuration location is deprecated in v1",
			Action:  "In v2, storage configuration is located at infrastructure.storage",
		})
	}

	return bcm.warnings
}

// DisplayDeprecationWarnings displays deprecation warnings to the user.
// Requirements: 13.5
func (bcm *BackwardCompatibilityManager) DisplayDeprecationWarnings() {
	if len(bcm.warnings) == 0 {
		return
	}

	log.Println("⚠️  Deprecation Warnings:")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, warning := range bcm.warnings {
		log.Printf("\n  Field: %s", warning.Field)
		log.Printf("  Warning: %s", warning.Message)
		log.Printf("  Action: %s\n", warning.Action)
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("\nTo migrate to v2, run: opencenter cluster migrate-config <config-file>")
	log.Println("For more information, see: docs/cluster-config/migration-guide.md")
}

// GetMigrationTimeline returns the migration timeline and deprecation schedule.
// Requirements: 13.6, 13.7
func GetMigrationTimeline() string {
	return `
Migration Timeline and Deprecation Schedule
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Phase 1: Dual Support (Current)
  - Both v1 and v2 schemas are fully supported
  - v1 configurations work with deprecation warnings
  - Migration tool available: opencenter cluster migrate-config
  - Duration: 6 months from v2 release

Phase 2: v2 Default (Planned)
  - New clusters default to v2 schema
  - v1 configurations still supported with warnings
  - Migration strongly recommended
  - Duration: 6 months

Phase 3: v1 Deprecation (Future)
  - v1 schema support removed
  - Only v2 configurations accepted
  - Migration required for all clusters
  - Timeline: 12 months from v2 release

Migration Resources:
  - Migration Guide: docs/cluster-config/migration-guide.md
  - v2 Reference: docs/cluster-config/v2-reference.md
  - Migration Command: opencenter cluster migrate-config <config-file>
  - Support: https://github.com/rackerlabs/opencenter-cli/issues

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`
}

// ValidateV1Compatibility validates that a v1 configuration can still be used.
// Requirements: 13.1, 13.7
func ValidateV1Compatibility(cfg *Config) error {
	// Check if v1 support is still enabled
	// In the future, this would return an error when v1 is no longer supported

	// For now, v1 is fully supported
	if cfg.SchemaVersion == "" || cfg.SchemaVersion == "1.0" {
		// Generate and display deprecation warnings
		bcm := NewBackwardCompatibilityManager()
		bcm.CheckV1Deprecations(cfg)
		bcm.DisplayDeprecationWarnings()
		return nil
	}

	return nil
}

// SupportsBothVersions returns true if the system currently supports both v1 and v2.
// Requirements: 13.1
func SupportsBothVersions() bool {
	// Currently in Phase 1: Dual Support
	return true
}

// GetSupportedVersions returns a list of currently supported schema versions.
// Requirements: 13.1
func GetSupportedVersions() []string {
	if SupportsBothVersions() {
		return []string{"1.0", "2.0"}
	}
	return []string{"2.0"}
}

// IsVersionSupported checks if a given schema version is supported.
func IsVersionSupported(version string) bool {
	supported := GetSupportedVersions()
	for _, v := range supported {
		if v == version {
			return true
		}
	}
	return false
}

// GetRecommendedVersion returns the recommended schema version for new configurations.
func GetRecommendedVersion() string {
	// During Phase 1, v1 is still default for backward compatibility
	// In Phase 2, this would return "2.0"
	return "1.0"
}

// ShouldMigrateToV2 determines if a configuration should be migrated to v2.
func ShouldMigrateToV2(cfg *Config) bool {
	// Recommend migration for v1 configurations
	return cfg.SchemaVersion == "" || cfg.SchemaVersion == "1.0"
}

// GetMigrationCommand returns the command to migrate a configuration to v2.
func GetMigrationCommand(configPath string) string {
	return fmt.Sprintf("opencenter cluster migrate-config %s", configPath)
}
