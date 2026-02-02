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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty_AllV1ConfigsRejected verifies that all v1 configurations are rejected
// with a clear error message, regardless of their content.
//
// Property 1: V1 Config Rejection
// Validates: Requirements 1.1, 1.5
//
// This property-based test generates various v1 configurations and verifies that:
// 1. All v1 configs (with explicit "1.0" or missing version) are rejected
// 2. The error is a V1ConfigError
// 3. The error message contains migration guidance
func TestProperty_AllV1ConfigsRejected(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("all v1 configs are rejected with clear error", prop.ForAll(
		func(clusterName string, orgName string, hasExplicitVersion bool) bool {
			// Generate v1 config
			var v1Config string
			if hasExplicitVersion {
				v1Config = fmt.Sprintf(`schema_version: "1.0"
opencenter:
  meta:
    cluster_name: %s
    organization: %s
`, clusterName, orgName)
			} else {
				// Missing version defaults to v1
				v1Config = fmt.Sprintf(`opencenter:
  meta:
    cluster_name: %s
    organization: %s
`, clusterName, orgName)
			}

			// Write to temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config.yaml")
			if err := os.WriteFile(configPath, []byte(v1Config), 0600); err != nil {
				t.Logf("Failed to write config: %v", err)
				return false
			}

			// Attempt to load
			manager := NewConfigManager()
			_, err := manager.Load(configPath, LoadOptions{})

			// Must error
			if err == nil {
				t.Logf("Expected error for v1 config, got nil")
				return false
			}

			// Must be V1ConfigError
			var v1Err *V1ConfigError
			if !errors.As(err, &v1Err) {
				t.Logf("Expected V1ConfigError, got: %T", err)
				return false
			}

			// Error must contain migration guidance
			errMsg := err.Error()
			requiredStrings := []string{
				"v1 configurations are not supported",
				"v2.0.0",
				"Install opencenter v1.x",
				"opencenter cluster migrate-config",
			}

			for _, required := range requiredStrings {
				if !contains(errMsg, required) {
					t.Logf("Error message missing %q: %s", required, errMsg)
					return false
				}
			}

			return true
		},
		genValidClusterName(),
		genValidOrgName(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestProperty_V1ConfigsAlwaysRejectedRegardlessOfContent verifies that v1 configs
// are rejected even with various valid content structures.
func TestProperty_V1ConfigsAlwaysRejectedRegardlessOfContent(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("v1 configs rejected regardless of content", prop.ForAll(
		func(clusterName string, provider string, region string) bool {
			// Generate v1 config with various content
			v1Config := fmt.Sprintf(`schema_version: "1.0"
opencenter:
  meta:
    cluster_name: %s
    organization: test-org
  cluster:
    provider: %s
    region: %s
`, clusterName, provider, region)

			// Write to temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config.yaml")
			if err := os.WriteFile(configPath, []byte(v1Config), 0600); err != nil {
				return false
			}

			// Attempt to load
			manager := NewConfigManager()
			_, err := manager.Load(configPath, LoadOptions{})

			// Must error with V1ConfigError
			if err == nil {
				return false
			}

			var v1Err *V1ConfigError
			return errors.As(err, &v1Err)
		},
		genValidClusterName(),
		gen.OneConstOf("openstack", "aws", "vsphere", "kind"),
		gen.OneConstOf("ord1", "dfw1", "iad1", "us-east-1", "us-west-2"),
	))

	properties.TestingRun(t)
}

// TestProperty_V1RejectionIsConsistent verifies that v1 rejection is consistent
// across multiple load attempts.
func TestProperty_V1RejectionIsConsistent(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("v1 rejection is consistent across loads", prop.ForAll(
		func(clusterName string) bool {
			v1Config := fmt.Sprintf(`schema_version: "1.0"
opencenter:
  meta:
    cluster_name: %s
`, clusterName)

			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config.yaml")
			if err := os.WriteFile(configPath, []byte(v1Config), 0600); err != nil {
				return false
			}

			manager := NewConfigManager()

			// Load multiple times
			for i := 0; i < 3; i++ {
				_, err := manager.Load(configPath, LoadOptions{})
				if err == nil {
					return false
				}

				var v1Err *V1ConfigError
				if !errors.As(err, &v1Err) {
					return false
				}
			}

			return true
		},
		genValidClusterName(),
	))

	properties.TestingRun(t)
}

// TestProperty_IsV1ConfigDetection verifies the isV1Config helper function
// correctly identifies v1 configurations.
func TestProperty_IsV1ConfigDetection(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("isV1Config correctly identifies v1 configs", prop.ForAll(
		func(clusterName string, hasVersion bool) bool {
			var configData string
			if hasVersion {
				configData = fmt.Sprintf(`schema_version: "1.0"
opencenter:
  meta:
    cluster_name: %s
`, clusterName)
			} else {
				configData = fmt.Sprintf(`opencenter:
  meta:
    cluster_name: %s
`, clusterName)
			}

			// Should always detect as v1
			return isV1Config([]byte(configData))
		},
		genValidClusterName(),
		gen.Bool(),
	))

	properties.Property("isV1Config correctly rejects v2 configs", prop.ForAll(
		func(clusterName string) bool {
			configData := fmt.Sprintf(`schema_version: "2.0"
opencenter:
  meta:
    cluster_name: %s
`, clusterName)

			// Should not detect as v1
			return !isV1Config([]byte(configData))
		},
		genValidClusterName(),
	))

	properties.TestingRun(t)
}

// Helper generators

// genValidClusterName generates valid cluster names
func genValidClusterName() gopter.Gen {
	return gen.IntRange(1, 100).Map(func(i int) string {
		return fmt.Sprintf("cluster-%d", i)
	})
}

// genValidOrgName generates valid organization names
func genValidOrgName() gopter.Gen {
	return gen.IntRange(1, 100).Map(func(i int) string {
		return fmt.Sprintf("org-%d", i)
	})
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
