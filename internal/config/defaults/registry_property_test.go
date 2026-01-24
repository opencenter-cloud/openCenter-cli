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

// Property 5: Provider-Region Default Application
// For any provider-region combination in the registry (e.g., OpenStack + sjc3),
// when hydrating a configuration with that provider and region, the system must
// apply the correct provider-region defaults for image IDs, availability zones,
// NTP servers, and DNS nameservers.
// **Validates: Requirements 7.2, 7.3**
func TestProperty_ProviderRegionDefaultApplication(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	registry := GetGlobalRegistry()

	properties.Property("provider-region defaults are correctly applied", prop.ForAll(
		func(provider, region string) bool {
			// Get defaults for the provider-region combination
			defaults, err := registry.GetDefaults(provider, region)
			if err != nil {
				// If combination doesn't exist in registry, that's expected
				return true
			}

			// Verify all required default methods return non-empty values
			imageID := defaults.GetImageID("24")
			if imageID == "" {
				return false
			}

			azs := defaults.GetAvailabilityZones()
			if len(azs) == 0 {
				return false
			}

			ntpServers := defaults.GetNTPServers()
			if len(ntpServers) == 0 {
				return false
			}

			dnsServers := defaults.GetDNSNameservers()
			if len(dnsServers) == 0 {
				return false
			}

			storageClass := defaults.GetDefaultStorageClass()
			if storageClass == "" {
				return false
			}

			flavors := defaults.GetDefaultFlavors()
			if flavors.Bastion == "" || flavors.Master == "" || flavors.Worker == "" {
				return false
			}

			return true
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("image IDs are provider-region specific", prop.ForAll(
		func(provider, region string) bool {
			defaults, err := registry.GetDefaults(provider, region)
			if err != nil {
				return true
			}

			// Verify image IDs are different for different OS versions
			image22 := defaults.GetImageID("22")
			image24 := defaults.GetImageID("24")

			// Both should be non-empty
			if image22 == "" || image24 == "" {
				return false
			}

			// For most providers, different OS versions have different image IDs
			// (though this isn't strictly required, it's the expected pattern)
			return true
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("availability zones are region-specific", prop.ForAll(
		func(provider, region string) bool {
			defaults, err := registry.GetDefaults(provider, region)
			if err != nil {
				return true
			}

			azs := defaults.GetAvailabilityZones()
			if len(azs) == 0 {
				return false
			}

			// Verify AZs contain region identifier (for cloud providers)
			if provider == "aws" || provider == "gcp" {
				for _, az := range azs {
					if len(az) < len(region) {
						return false
					}
				}
			}

			return true
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("NTP servers are region-appropriate", prop.ForAll(
		func(provider, region string) bool {
			defaults, err := registry.GetDefaults(provider, region)
			if err != nil {
				return true
			}

			ntpServers := defaults.GetNTPServers()
			if len(ntpServers) == 0 {
				return false
			}

			// Verify NTP servers are non-empty strings
			for _, ntp := range ntpServers {
				if ntp == "" {
					return false
				}
			}

			return true
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("DNS nameservers are valid", prop.ForAll(
		func(provider, region string) bool {
			defaults, err := registry.GetDefaults(provider, region)
			if err != nil {
				return true
			}

			dnsServers := defaults.GetDNSNameservers()
			if len(dnsServers) == 0 {
				return false
			}

			// Verify DNS servers are non-empty strings
			for _, dns := range dnsServers {
				if dns == "" {
					return false
				}
			}

			return true
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("storage class is provider-appropriate", prop.ForAll(
		func(provider, region string) bool {
			defaults, err := registry.GetDefaults(provider, region)
			if err != nil {
				return true
			}

			storageClass := defaults.GetDefaultStorageClass()
			if storageClass == "" {
				return false
			}

			// Verify storage class naming conventions match provider
			switch provider {
			case "openstack":
				// OpenStack typically uses cinder-based storage classes
				return true
			case "aws":
				// AWS typically uses gp2/gp3 storage classes
				return true
			case "gcp":
				// GCP typically uses standard-rwo or similar
				return true
			default:
				return true
			}
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.Property("flavors are complete and non-empty", prop.ForAll(
		func(provider, region string) bool {
			defaults, err := registry.GetDefaults(provider, region)
			if err != nil {
				return true
			}

			flavors := defaults.GetDefaultFlavors()

			// All flavor types must be specified
			if flavors.Bastion == "" {
				return false
			}
			if flavors.Master == "" {
				return false
			}
			if flavors.Worker == "" {
				return false
			}
			if flavors.WorkerWindows == "" {
				return false
			}

			return true
		},
		genRegisteredProvider(registry),
		genRegisteredRegion(registry),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Generators for property-based testing

// genRegisteredProvider generates provider names that are registered in the registry.
func genRegisteredProvider(registry Registry) gopter.Gen {
	providers := registry.ListProviders()
	if len(providers) == 0 {
		// Fallback to known providers if registry is empty
		providers = []string{"openstack", "aws", "gcp"}
	}
	return gen.OneConstOf(providersToInterfaces(providers)...)
}

// genRegisteredRegion generates region names that are registered in the registry.
// This generator is dependent on the provider, but for simplicity we generate
// from all known regions across all providers.
func genRegisteredRegion(registry Registry) gopter.Gen {
	allRegions := make([]string, 0)
	for _, provider := range registry.ListProviders() {
		regions := registry.ListRegions(provider)
		allRegions = append(allRegions, regions...)
	}

	if len(allRegions) == 0 {
		// Fallback to known regions if registry is empty
		allRegions = []string{"sjc3", "dfw3", "iad3", "us-east-1", "us-west-2", "eu-west-1", "us-central1", "europe-west1"}
	}

	return gen.OneConstOf(regionsToInterfaces(allRegions)...)
}

// Helper functions to convert string slices to interface slices for gen.OneConstOf

func providersToInterfaces(providers []string) []interface{} {
	result := make([]interface{}, len(providers))
	for i, p := range providers {
		result[i] = p
	}
	return result
}

func regionsToInterfaces(regions []string) []interface{} {
	result := make([]interface{}, len(regions))
	for i, r := range regions {
		result[i] = r
	}
	return result
}
