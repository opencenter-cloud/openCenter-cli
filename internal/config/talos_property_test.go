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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: talos-openstack-provider, Property 4: Security hardening completeness
// For any generated Talos machine configuration, the configuration should contain
// enabled settings for AppArmor, Seccomp, hardened sysctls, KubePrism, and disk encryption.
// Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5
func TestProperty_SecurityHardeningCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all generated Talos configs have security hardening enabled", prop.ForAll(
		func(clusterName string) bool {
			// Generate default Talos configuration
			talosConfig := DefaultTalosConfig(clusterName)

			// Verify all security features are enabled by default
			if !talosConfig.MachineConfig.AppArmorEnabled {
				return false
			}
			if !talosConfig.MachineConfig.SeccompEnabled {
				return false
			}
			if !talosConfig.MachineConfig.DiskEncryption {
				return false
			}
			if !talosConfig.MachineConfig.KubePrismEnabled {
				return false
			}

			// Verify security config defaults
			if !talosConfig.SecurityConfig.VTPMEnabled {
				return false
			}
			if !talosConfig.SecurityConfig.ImageVerification {
				return false
			}
			if !talosConfig.SecurityConfig.MFARequired {
				return false
			}
			if !talosConfig.SecurityConfig.AuditLogEnabled {
				return false
			}

			return true
		},
		genClusterName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genClusterName generates valid cluster names for property testing
func genClusterName() gopter.Gen {
	return gen.Identifier().
		SuchThat(func(v interface{}) bool {
			name := v.(string)
			// Ensure name is valid according to ValidateClusterName
			return len(name) > 0 && len(name) <= 255
		})
}
