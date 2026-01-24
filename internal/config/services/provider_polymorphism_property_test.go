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

package services

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: v2-cluster-config-schema, Property 7: Service Provider Polymorphism
// **Validates: Requirements 8.2, 8.3, 8.4, 8.5, 8.6**
//
// For any infrastructure provider, when a service requires provider-specific configuration
// (e.g., cert-manager DNS challenge), the system must auto-select a compatible service
// provider based on the infrastructure provider (AWS→route53, OpenStack+Designate→designate,
// OpenStack-Designate→cloudflare) and validate that user overrides are compatible with the
// infrastructure provider.
func TestProperty_ServiceProviderPolymorphism(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property 1: Auto-selection produces compatible providers
	properties.Property("auto-selected providers are compatible with infrastructure", prop.ForAll(
		func(infraProvider InfrastructureProvider, useDesignate bool) bool {
			registry := GetProviderRegistry()
			clusterName := "test-cluster"

			// Set Designate availability for OpenStack
			if infraProvider == ProviderOpenStack {
				registry.SetDesignateAvailability(clusterName, useDesignate)
			}

			// Test cert-manager DNS provider auto-selection
			dnsProvider, err := registry.AutoSelectProvider("cert-manager", "dns", infraProvider, clusterName)
			if err != nil {
				return false
			}

			// Verify the auto-selected provider is compatible
			if err := registry.ValidateCompatibility("cert-manager", "dns", infraProvider, dnsProvider); err != nil {
				return false
			}

			// Test storage providers only for providers that have storage compatibility defined
			// BareMetal and VSphere don't have storage compatibility entries in the registry
			if infraProvider == ProviderBareMetal || infraProvider == ProviderVSphere {
				return true
			}

			// Test loki storage provider auto-selection
			lokiProvider, err := registry.GetDefaultProvider("loki", "storage", infraProvider)
			if err != nil {
				return false
			}

			// Verify the auto-selected provider is compatible
			if err := registry.ValidateCompatibility("loki", "storage", infraProvider, lokiProvider); err != nil {
				return false
			}

			// Test tempo storage provider auto-selection
			tempoProvider, err := registry.GetDefaultProvider("tempo", "storage", infraProvider)
			if err != nil {
				return false
			}

			// Verify the auto-selected provider is compatible
			if err := registry.ValidateCompatibility("tempo", "storage", infraProvider, tempoProvider); err != nil {
				return false
			}

			// Test velero storage provider auto-selection
			veleroProvider, err := registry.GetDefaultProvider("velero", "storage", infraProvider)
			if err != nil {
				return false
			}

			// Verify the auto-selected provider is compatible
			return registry.ValidateCompatibility("velero", "storage", infraProvider, veleroProvider) == nil
		},
		genInfrastructureProvider(),
		gen.Bool(),
	))

	// Property 2: OpenStack with Designate selects Designate for DNS
	properties.Property("OpenStack with Designate auto-selects Designate DNS provider", prop.ForAll(
		func() bool {
			registry := GetProviderRegistry()
			clusterName := "test-cluster-designate"

			// Enable Designate for OpenStack
			registry.SetDesignateAvailability(clusterName, true)

			// Auto-select DNS provider
			dnsProvider, err := registry.AutoSelectProvider("cert-manager", "dns", ProviderOpenStack, clusterName)
			if err != nil {
				return false
			}

			// Should select Designate
			return dnsProvider == DNSProviderDesignate
		},
	))

	// Property 3: OpenStack without Designate selects Cloudflare for DNS
	properties.Property("OpenStack without Designate auto-selects Cloudflare DNS provider", prop.ForAll(
		func() bool {
			registry := GetProviderRegistry()
			clusterName := "test-cluster-no-designate"

			// Disable Designate for OpenStack
			registry.SetDesignateAvailability(clusterName, false)

			// Auto-select DNS provider
			dnsProvider, err := registry.AutoSelectProvider("cert-manager", "dns", ProviderOpenStack, clusterName)
			if err != nil {
				return false
			}

			// Should select Cloudflare
			return dnsProvider == DNSProviderCloudflare
		},
	))

	// Property 4: AWS auto-selects Route53 for DNS
	properties.Property("AWS auto-selects Route53 DNS provider", prop.ForAll(
		func() bool {
			registry := GetProviderRegistry()
			clusterName := "test-cluster-aws"

			// Auto-select DNS provider for AWS
			dnsProvider, err := registry.AutoSelectProvider("cert-manager", "dns", ProviderAWS, clusterName)
			if err != nil {
				return false
			}

			// Should select Route53
			return dnsProvider == DNSProviderRoute53
		},
	))

	// Property 5: Incompatible provider combinations are rejected
	properties.Property("incompatible provider combinations are rejected", prop.ForAll(
		func(infraProvider InfrastructureProvider) bool {
			registry := GetProviderRegistry()

			// Test incompatible combinations based on infrastructure provider
			var incompatibleProvider ServiceProviderType
			var serviceName, providerType string

			switch infraProvider {
			case ProviderAWS:
				// Route53 should not work on OpenStack
				serviceName = "cert-manager"
				providerType = "dns"
				incompatibleProvider = DNSProviderDesignate
			case ProviderOpenStack:
				// Designate should not work on AWS
				serviceName = "cert-manager"
				providerType = "dns"
				incompatibleProvider = DNSProviderRoute53
			case ProviderGCP:
				// Route53 should not work on GCP
				serviceName = "cert-manager"
				providerType = "dns"
				incompatibleProvider = DNSProviderRoute53
			case ProviderAzure:
				// Route53 should not work on Azure
				serviceName = "cert-manager"
				providerType = "dns"
				incompatibleProvider = DNSProviderRoute53
			case ProviderBareMetal, ProviderVSphere:
				// Provider-specific DNS should not work on bare metal/vsphere
				serviceName = "cert-manager"
				providerType = "dns"
				incompatibleProvider = DNSProviderRoute53
			default:
				return true
			}

			// Validation should fail for incompatible combination
			err := registry.ValidateCompatibility(serviceName, providerType, infraProvider, incompatibleProvider)
			return err != nil
		},
		genInfrastructureProvider(),
	))

	// Property 6: User overrides are validated for compatibility
	properties.Property("user overrides are validated for compatibility", prop.ForAll(
		func(infraProvider InfrastructureProvider) bool {
			validator := NewServiceProviderValidator()
			clusterName := "test-cluster-override"

			// Create service configurations with explicit provider overrides
			services := make(map[string]interface{})

			// Get a compatible provider for this infrastructure
			registry := GetProviderRegistry()
			compatibleProviders := registry.GetCompatibleProviders("cert-manager", "dns", infraProvider)
			if len(compatibleProviders) == 0 {
				return true // Skip if no compatible providers
			}

			// Create cert-manager config with compatible provider
			certMgr := &CertManagerConfig{
				BaseConfig: BaseConfig{
					Enabled: true,
				},
				DNSProvider: string(compatibleProviders[0]),
			}
			services["cert-manager"] = certMgr

			// Validate - should succeed with compatible provider
			errors := validator.ValidateServiceProviders(services, string(infraProvider), false, clusterName)
			return len(errors) == 0
		},
		genInfrastructureProvider(),
	))

	// Property 7: Storage provider defaults match infrastructure
	properties.Property("storage provider defaults match infrastructure", prop.ForAll(
		func(infraProvider InfrastructureProvider) bool {
			registry := GetProviderRegistry()

			// Skip BareMetal and VSphere as they don't have storage compatibility defined yet
			if infraProvider == ProviderBareMetal || infraProvider == ProviderVSphere {
				return true
			}

			// Get default storage providers
			lokiProvider, err := registry.GetDefaultProvider("loki", "storage", infraProvider)
			if err != nil {
				return false
			}

			tempoProvider, err := registry.GetDefaultProvider("tempo", "storage", infraProvider)
			if err != nil {
				return false
			}

			veleroProvider, err := registry.GetDefaultProvider("velero", "storage", infraProvider)
			if err != nil {
				return false
			}

			// Verify expected defaults based on infrastructure
			switch infraProvider {
			case ProviderAWS:
				return lokiProvider == StorageProviderS3 &&
					tempoProvider == StorageProviderS3 &&
					veleroProvider == StorageProviderS3
			case ProviderOpenStack:
				return lokiProvider == StorageProviderSwift &&
					tempoProvider == StorageProviderSwift &&
					veleroProvider == StorageProviderSwift
			case ProviderGCP:
				return lokiProvider == StorageProviderGCS &&
					tempoProvider == StorageProviderGCS &&
					veleroProvider == StorageProviderGCS
			case ProviderAzure:
				return lokiProvider == StorageProviderAzure &&
					tempoProvider == StorageProviderAzure &&
					veleroProvider == StorageProviderAzure
			default:
				return false
			}
		},
		genInfrastructureProvider(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper generators for property tests

// genInfrastructureProvider generates valid infrastructure provider values
func genInfrastructureProvider() gopter.Gen {
	return gen.OneConstOf(
		ProviderOpenStack,
		ProviderAWS,
		ProviderGCP,
		ProviderAzure,
		ProviderBareMetal,
		ProviderVSphere,
	)
}
