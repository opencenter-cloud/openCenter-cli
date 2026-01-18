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

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
)

// TestProperty_BuilderMethodChaining validates that all builder methods return the builder
// for proper method chaining and that the configuration is updated correctly.
func TestProperty_BuilderMethodChaining(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("builder methods support chaining", prop.ForAll(
		func(clusterName, org, provider string) bool {
			builder := NewConfigBuilder(clusterName)

			// Chain multiple methods and verify builder is returned
			result := builder.
				WithOrganization(org).
				WithProvider(provider).
				WithEnvironment("test").
				WithRegion("us-east-1")

			// Verify result is still a ConfigBuilder
			_, ok := result.(ConfigBuilder)
			return ok
		},
		genValidClusterName(),
		genOrganization(),
		genProvider(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_BuilderImmutability validates that builder operations don't affect
// other builder instances (each builder is independent).
func TestProperty_BuilderImmutability(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("builder instances are independent", prop.ForAll(
		func(name1, name2, org1, org2 string) bool {
			// Create two independent builders with kind provider (no special requirements)
			builder1 := NewConfigBuilder(name1).
				WithOrganization(org1).
				WithProvider("kind").
				WithSubnetNodes("10.0.0.0/24").
				WithSubnetPods("10.244.0.0/16").
				WithSubnetServices("10.96.0.0/12")

			builder2 := NewConfigBuilder(name2).
				WithOrganization(org2).
				WithProvider("baremetal").
				WithSubnetNodes("10.1.0.0/24").
				WithSubnetPods("10.245.0.0/16").
				WithSubnetServices("10.97.0.0/12")

			// Build both configurations
			config1, err1 := builder1.
				WithMasterCount(3).
				WithWorkerCount(3).
				Build()

			config2, err2 := builder2.
				WithMasterCount(5).
				WithWorkerCount(5).
				Build()

			// Both should succeed
			if err1 != nil || err2 != nil {
				return false
			}

			// Verify configurations are independent
			return config1.OpenCenter.Meta.Name == name1 &&
				config2.OpenCenter.Meta.Name == name2 &&
				config1.OpenCenter.Meta.Organization == org1 &&
				config2.OpenCenter.Meta.Organization == org2 &&
				config1.OpenCenter.Infrastructure.Provider == "kind" &&
				config2.OpenCenter.Infrastructure.Provider == "baremetal"
		},
		genValidClusterName(),
		genValidClusterName(),
		genOrganization(),
		genOrganization(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ValidationConsistency validates that validation always produces
// the same results for the same configuration.
func TestProperty_ValidationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("validation is deterministic", prop.ForAll(
		func(clusterName, org, provider string, masterCount, workerCount int) bool {
			builder := NewConfigBuilder(clusterName).
				WithOrganization(org).
				WithProvider(provider).
				WithMasterCount(masterCount).
				WithWorkerCount(workerCount)

			// Run validation twice
			errors1 := builder.Validate()
			errors2 := builder.Validate()

			// Should produce same number of errors
			if len(errors1) != len(errors2) {
				return false
			}

			// Should produce same error fields
			for i := range errors1 {
				if errors1[i].Field != errors2[i].Field {
					return false
				}
			}

			return true
		},
		genValidClusterName(),
		genOrganization(),
		genProvider(),
		genNodeCount(),
		genNodeCount(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_RequiredFieldsValidation validates that explicitly clearing required fields
// produces validation errors.
func TestProperty_RequiredFieldsValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("clearing required fields produces errors", prop.ForAll(
		func(clusterName string) bool {
			// Create builder and explicitly clear required fields
			builder := NewConfigBuilder(clusterName).
				WithOrganization(""). // Clear the default
				WithProvider("")      // Clear the default

			// Validate
			errors := builder.Validate()

			// Check for specific required field errors
			hasOrgError := false
			hasProviderError := false

			for _, err := range errors {
				if err.Field == "opencenter.meta.organization" {
					hasOrgError = true
				}
				if err.Field == "opencenter.infrastructure.provider" {
					hasProviderError = true
				}
			}

			// Both org and provider errors must be present
			return hasOrgError && hasProviderError
		},
		genValidClusterName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_NodeCountValidation validates that invalid node counts
// always produce validation errors.
func TestProperty_NodeCountValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("invalid node counts produce errors", prop.ForAll(
		func(masterCount, workerCount int) bool {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org").
				WithProvider("openstack").
				WithMasterCount(masterCount).
				WithWorkerCount(workerCount)

			errors := builder.Validate()

			// If master count is < 1, should have error
			if masterCount < 1 {
				hasError := false
				for _, err := range errors {
					if err.Field == "opencenter.cluster.kubernetes.master_count" {
						hasError = true
						break
					}
				}
				if !hasError {
					return false
				}
			}

			// If worker count is negative, should have error
			if workerCount < 0 {
				hasError := false
				for _, err := range errors {
					if err.Field == "opencenter.cluster.kubernetes.worker_count" {
						hasError = true
						break
					}
				}
				if !hasError {
					return false
				}
			}

			return true
		},
		gen.IntRange(-5, 10),
		gen.IntRange(-5, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ProviderSpecificValidation validates that provider-specific
// validation rules are enforced correctly.
func TestProperty_ProviderSpecificValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("provider-specific validation is enforced", prop.ForAll(
		func(provider string) bool {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org").
				WithProvider(provider).
				WithMasterCount(3).
				WithWorkerCount(3).
				WithSubnetNodes("10.0.0.0/24").
				WithSubnetPods("10.244.0.0/16").
				WithSubnetServices("10.96.0.0/12")

			errors := builder.Validate()

			// OpenStack requires auth_url
			if provider == "openstack" {
				hasAuthURLError := false
				for _, err := range errors {
					if err.Field == "opencenter.infrastructure.cloud.openstack.auth_url" {
						hasAuthURLError = true
						break
					}
				}
				return hasAuthURLError
			}

			// AWS requires region
			if provider == "aws" {
				hasRegionError := false
				for _, err := range errors {
					if err.Field == "opencenter.infrastructure.cloud.aws.region" {
						hasRegionError = true
						break
					}
				}
				return hasRegionError
			}

			// Other providers don't have specific requirements
			return true
		},
		genProvider(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_BuildSuccessWithValidConfig validates that valid configurations
// always build successfully.
func TestProperty_BuildSuccessWithValidConfig(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("valid configurations build successfully", prop.ForAll(
		func(clusterName, org string, masterCount, workerCount int) bool {
			// Ensure valid counts
			// Master count must be odd (1, 3, 5, 7, 9) for etcd quorum
			if masterCount < 1 || masterCount%2 == 0 {
				masterCount = 2*masterCount + 1 // Convert to odd: 0->1, 1->3, 2->5, etc.
				if masterCount < 1 {
					masterCount = 1
				}
			}
			if workerCount < 0 {
				workerCount = 0
			}

			builder := NewConfigBuilder(clusterName).
				WithOrganization(org).
				WithProvider("kind"). // kind has no specific requirements
				WithMasterCount(masterCount).
				WithWorkerCount(workerCount).
				WithSubnetNodes("10.0.0.0/24").    // Required networking config
				WithSubnetPods("10.244.0.0/16").   // Required networking config
				WithSubnetServices("10.96.0.0/12") // Required networking config

			config, err := builder.Build()

			// Should build successfully
			if err != nil {
				return false
			}

			// Verify configuration values
			return config.OpenCenter.Meta.Name == clusterName &&
				config.OpenCenter.Meta.Organization == org &&
				config.OpenCenter.Infrastructure.Provider == "kind" &&
				config.OpenCenter.Cluster.Kubernetes.MasterCount == masterCount &&
				config.OpenCenter.Cluster.Kubernetes.WorkerCount == workerCount
		},
		genValidClusterName(),
		genOrganization(),
		gen.IntRange(1, 10),
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_MetadataTimestamps validates that metadata timestamps are
// set correctly during build.
func TestProperty_MetadataTimestamps(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("metadata timestamps are set on build", prop.ForAll(
		func(clusterName, org string) bool {
			builder := NewConfigBuilder(clusterName).
				WithOrganization(org).
				WithProvider("kind").
				WithMasterCount(1).
				WithWorkerCount(1).
				WithSubnetNodes("10.0.0.0/24").
				WithSubnetPods("10.244.0.0/16").
				WithSubnetServices("10.96.0.0/12")

			config, err := builder.Build()
			if err != nil {
				return false
			}

			// Timestamps should be set
			return !config.Metadata.CreatedAt.IsZero() &&
				!config.Metadata.UpdatedAt.IsZero() &&
				config.Metadata.UpdatedAt.After(config.Metadata.CreatedAt) ||
				config.Metadata.UpdatedAt.Equal(config.Metadata.CreatedAt)
		},
		genValidClusterName(),
		genOrganization(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_TagsAndAnnotations validates that tags and annotations
// are stored correctly.
func TestProperty_TagsAndAnnotations(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("tags and annotations are stored correctly", prop.ForAll(
		func(tagKey, tagValue, annoKey, annoValue string) bool {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org").
				WithProvider("kind").
				WithMasterCount(1).
				WithWorkerCount(1).
				WithSubnetNodes("10.0.0.0/24").
				WithSubnetPods("10.244.0.0/16").
				WithSubnetServices("10.96.0.0/12").
				WithTag(tagKey, tagValue).
				WithAnnotation(annoKey, annoValue)

			config, err := builder.Build()
			if err != nil {
				return false
			}

			// Verify tags and annotations
			return config.Metadata.Tags[tagKey] == tagValue &&
				config.Metadata.Annotations[annoKey] == annoValue
		},
		gen.AlphaString(),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ServiceConfiguration validates that service enable/disable
// operations work correctly.
func TestProperty_ServiceConfiguration(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("service configuration is applied correctly", prop.ForAll(
		func(enabled bool) bool {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org").
				WithProvider("kind").
				WithMasterCount(1).
				WithWorkerCount(1).
				WithSubnetNodes("10.0.0.0/24").
				WithSubnetPods("10.244.0.0/16").
				WithSubnetServices("10.96.0.0/12").
				WithService("cert-manager", enabled)

			config, err := builder.Build()
			if err != nil {
				return false
			}

			// Verify service configuration
			if svc, exists := config.OpenCenter.Services["cert-manager"]; exists {
				// Type assert to access Enabled field
				if certMgr, ok := svc.(*services.CertManagerConfig); ok {
					return certMgr.Enabled == enabled
				}
			}

			return false
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_OverrideStorage validates that overrides are stored correctly.
func TestProperty_OverrideStorage(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("overrides are stored correctly", prop.ForAll(
		func(path, value string) bool {
			builder := NewConfigBuilder("test-cluster").
				WithOrganization("test-org").
				WithProvider("kind").
				WithMasterCount(1).
				WithWorkerCount(1).
				WithSubnetNodes("10.0.0.0/24").
				WithSubnetPods("10.244.0.0/16").
				WithSubnetServices("10.96.0.0/12").
				WithOverride(path, value)

			config, err := builder.Build()
			if err != nil {
				return false
			}

			// Verify override is stored
			if storedValue, exists := config.Overrides[path]; exists {
				return storedValue == value
			}

			return false
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Generators for property-based testing
// Note: genClusterName is defined in talos_property_test.go and shared across tests

// genValidClusterName generates cluster names that pass ValidateClusterName
func genValidClusterName() gopter.Gen {
	return gen.RegexMatch("[a-zA-Z0-9][a-zA-Z0-9._-]{1,18}[a-zA-Z0-9]")
}

func genOrganization() gopter.Gen {
	return gen.RegexMatch("[a-zA-Z0-9][a-zA-Z0-9._-]{1,18}[a-zA-Z0-9]")
}

func genProvider() gopter.Gen {
	return gen.OneConstOf("openstack", "aws", "baremetal", "kind")
}

func genNodeCount() gopter.Gen {
	return gen.IntRange(0, 10)
}
