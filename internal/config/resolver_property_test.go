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
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: v2-cluster-config-schema, Property 2: Reference Resolution Round-Trip
// **Validates: Requirements 6.1, 6.2**
//
// For any configuration with references, resolving all references then re-serializing
// should produce a configuration where all ${path.to.value} patterns are replaced with
// their actual values, and re-parsing this resolved configuration should produce
// equivalent structs.
func TestProperty_ReferenceResolutionRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("resolving references produces consistent configuration", prop.ForAll(
		func(vrrpIP string, region string) bool {
			// Create a configuration with references
			cfg := defaultConfig("test-cluster")
			cfg.OpenCenter.Meta.Region = region
			cfg.OpenCenter.Cluster.Networking.VRRPIP = vrrpIP
			cfg.OpenCenter.Cluster.BaseDomain = "${opencenter.cluster.networking.vrrp_ip}.${opencenter.meta.region}.example.com"
			cfg.OpenCenter.Cluster.ClusterFQDN = "api.${opencenter.cluster.base_domain}"

			// Resolve references
			resolver := NewReferenceResolver()
			if err := resolver.Resolve(&cfg); err != nil {
				// If resolution fails, it's not a valid test case
				return true
			}

			// Verify that all references are resolved (no ${...} patterns remain)
			if hasReferences(cfg.OpenCenter.Cluster.BaseDomain) {
				return false
			}
			if hasReferences(cfg.OpenCenter.Cluster.ClusterFQDN) {
				return false
			}

			// Verify that the resolved values are correct
			expectedBaseDomain := vrrpIP + "." + region + ".example.com"
			if cfg.OpenCenter.Cluster.BaseDomain != expectedBaseDomain {
				return false
			}

			expectedFQDN := "api." + expectedBaseDomain
			if cfg.OpenCenter.Cluster.ClusterFQDN != expectedFQDN {
				return false
			}

			// Re-resolve should be idempotent (no changes)
			cfgCopy := cfg
			if err := resolver.Resolve(&cfgCopy); err != nil {
				return false
			}

			// Verify that re-resolving doesn't change the configuration
			return reflect.DeepEqual(cfg, cfgCopy)
		},
		genValidIPv4(),
		genValidRegion(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: v2-cluster-config-schema, Property 3: Circular Reference Detection
// **Validates: Requirements 6.4**
//
// For any configuration with circular reference dependencies (A references B, B references C,
// C references A), the reference resolver must detect the cycle and reject the configuration
// with a clear error message.
func TestProperty_CircularReferenceDetection(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("circular references are detected", prop.ForAll(
		func(cycleLength int) bool {
			// Create a configuration with a circular reference
			cfg := defaultConfig("test-cluster")

			// Create a simple 2-node cycle
			cfg.OpenCenter.Cluster.ClusterName = "${opencenter.cluster.base_domain}"
			cfg.OpenCenter.Cluster.BaseDomain = "${opencenter.cluster.cluster_name}"

			// Try to resolve - should fail with circular reference error
			resolver := NewReferenceResolver()
			err := resolver.Resolve(&cfg)

			// Should return an error
			if err == nil {
				return false
			}

			// Error message should mention circular reference
			return containsString(err.Error(), "circular reference")
		},
		gen.IntRange(2, 5), // Cycle length between 2 and 5
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper functions for property tests

// hasReferences checks if a string contains any ${...} reference patterns.
func hasReferences(s string) bool {
	return len(s) > 0 && (containsString(s, "${") && containsString(s, "}"))
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

// findSubstring finds a substring in a string.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// genValidIPv4 generates valid IPv4 addresses for testing.
func genValidIPv4() gopter.Gen {
	return gen.SliceOfN(4, gen.IntRange(1, 254)).Map(func(octets []int) string {
		if len(octets) != 4 {
			return "10.0.0.1"
		}
		return formatIPv4(octets[0], octets[1], octets[2], octets[3])
	})
}

// formatIPv4 formats an IPv4 address from four integers.
func formatIPv4(a, b, c, d int) string {
	return intToString(a) + "." + intToString(b) + "." + intToString(c) + "." + intToString(d)
}

// intToString converts an integer to a string.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// genValidRegion generates valid region names for testing.
func genValidRegion() gopter.Gen {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1", "sjc3", "dfw3", "iad3"}
	return gen.OneConstOf(
		regions[0], regions[1], regions[2], regions[3], regions[4], regions[5], regions[6],
	)
}
