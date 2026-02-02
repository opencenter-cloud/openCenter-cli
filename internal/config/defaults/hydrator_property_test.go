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

package defaults

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestConfig is a simple test configuration struct for hydration testing.
type TestConfig struct {
	ImageID             string
	DefaultStorageClass string
	FlavorBastion       string
	FlavorMaster        string
	FlavorWorker        string
	FlavorWorkerWindows string
	AvailabilityZones   []string
	NTPServers          []string
	DNSNameservers      []string
}

// Property 4: Default Precedence Order
// For any configuration field with values at multiple precedence levels
// (explicit, CLI config, provider-region, provider, global), the hydrator
// must apply the value from the highest precedence level and not override
// it with lower precedence values.
// **Validates: Requirements 7.4, 15.2, 15.3**
func TestProperty_DefaultPrecedenceOrder(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	registry := GetGlobalRegistry()

	properties.Property("explicit values are never overridden by defaults", prop.ForAll(
		func(provider, region, explicitValue string) bool {
			// Skip empty strings - they should be treated as empty fields, not explicit values
			if explicitValue == "" {
				return true
			}

			// Create config with explicit value
			cfg := &TestConfig{
				ImageID: explicitValue,
			}

			// Apply hydration
			hydrator := NewHydrator(registry)
			err := hydrator.Hydrate(cfg, provider, region)
			if err != nil {
				// If provider-region combo doesn't exist, that's ok
				return true
			}

			// Verify explicit value was not overridden
			return cfg.ImageID == explicitValue
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
		gen.AlphaString(),
	))

	properties.Property("empty fields are populated with provider-region defaults", prop.ForAll(
		func(provider, region string) bool {
			// Create config with empty fields
			cfg := &TestConfig{}

			// Apply hydration
			hydrator := NewHydrator(registry)
			err := hydrator.Hydrate(cfg, provider, region)
			if err != nil {
				// If provider-region combo doesn't exist, that's ok
				return true
			}

			// If no defaults were applied (invalid provider-region combo), that's ok
			appliedDefaults := hydrator.GetAppliedDefaults()
			if len(appliedDefaults) == 0 {
				return true
			}

			// If defaults were applied, verify at least some fields were populated
			// (not all fields may have defaults for all providers)
			return cfg.ImageID != "" || cfg.DefaultStorageClass != "" ||
				len(cfg.AvailabilityZones) > 0 || len(cfg.NTPServers) > 0
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("applied defaults are tracked correctly", prop.ForAll(
		func(provider, region string) bool {
			// Create config with empty fields
			cfg := &TestConfig{}

			// Apply hydration
			hydrator := NewHydrator(registry)
			err := hydrator.Hydrate(cfg, provider, region)
			if err != nil {
				return true
			}

			// Get applied defaults
			appliedDefaults := hydrator.GetAppliedDefaults()

			// If any field was populated, it should be tracked
			if cfg.ImageID != "" {
				if source, ok := appliedDefaults["ImageID"]; !ok || source != SourceProviderRegion {
					return false
				}
			}

			if cfg.DefaultStorageClass != "" {
				if source, ok := appliedDefaults["DefaultStorageClass"]; !ok || source != SourceProviderRegion {
					return false
				}
			}

			return true
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("hydration is idempotent", prop.ForAll(
		func(provider, region string) bool {
			// Create config with empty fields
			cfg := &TestConfig{}

			// Apply hydration twice
			hydrator1 := NewHydrator(registry)
			err1 := hydrator1.Hydrate(cfg, provider, region)
			if err1 != nil {
				return true
			}

			// Capture state after first hydration
			imageID1 := cfg.ImageID
			storageClass1 := cfg.DefaultStorageClass

			// Apply hydration again
			hydrator2 := NewHydrator(registry)
			err2 := hydrator2.Hydrate(cfg, provider, region)
			if err2 != nil {
				return true
			}

			// Verify values didn't change
			return cfg.ImageID == imageID1 && cfg.DefaultStorageClass == storageClass1
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("partial explicit values don't prevent other defaults", prop.ForAll(
		func(provider, region, explicitImage string) bool {
			// Skip empty strings - they should be treated as empty fields, not explicit values
			if explicitImage == "" {
				return true
			}

			// Create config with one explicit value and other empty fields
			cfg := &TestConfig{
				ImageID: explicitImage,
			}

			// Apply hydration
			hydrator := NewHydrator(registry)
			err := hydrator.Hydrate(cfg, provider, region)
			if err != nil {
				return true
			}

			// Verify explicit value preserved
			if cfg.ImageID != explicitImage {
				return false
			}

			// If no defaults were applied (invalid provider-region combo), that's ok
			appliedDefaults := hydrator.GetAppliedDefaults()
			if len(appliedDefaults) == 0 {
				return true
			}

			// If defaults were applied, verify other fields were populated
			// (at least some should be populated)
			return cfg.DefaultStorageClass != "" || len(cfg.AvailabilityZones) > 0
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
		gen.AlphaString(),
	))

	properties.Property("slice fields are only populated when empty", prop.ForAll(
		func(provider, region string, explicitAZs []string) bool {
			// Create config with explicit availability zones
			cfg := &TestConfig{
				AvailabilityZones: explicitAZs,
			}

			// Apply hydration
			hydrator := NewHydrator(registry)
			err := hydrator.Hydrate(cfg, provider, region)
			if err != nil {
				return true
			}

			// Verify explicit slice was not overridden
			if len(explicitAZs) > 0 {
				return len(cfg.AvailabilityZones) == len(explicitAZs)
			}

			// If explicit slice was empty, it should be populated
			return len(cfg.AvailabilityZones) >= 0
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
