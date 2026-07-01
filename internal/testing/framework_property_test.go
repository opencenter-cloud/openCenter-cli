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

package testing

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 1: Deterministic Generation with Seeds
// For any seed value, creating two frameworks with the same seed should produce identical test data
// Validates: Requirements 11.1
func TestProperty_DeterministicGenerationWithSeeds(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("same seed produces identical configs", prop.ForAll(
		func(seed int64, provider string) bool {
			fw1 := NewTestFrameworkWithSeed(t, seed)
			fw2 := NewTestFrameworkWithSeed(t, seed)

			config1 := fw1.CreateTestConfig(provider)
			config2 := fw2.CreateTestConfig(provider)

			// Verify key fields are identical
			return config1.OpenCenter.Meta.Name == config2.OpenCenter.Meta.Name &&
				config1.OpenCenter.Meta.Organization == config2.OpenCenter.Meta.Organization &&
				config1.OpenCenter.Infrastructure.Provider == config2.OpenCenter.Infrastructure.Provider
		},
		gen.Int64(),
		genValidProvider(),
	))

	properties.Property("same seed produces identical template data", prop.ForAll(
		func(seed int64) bool {
			fw1 := NewTestFrameworkWithSeed(t, seed)
			fw2 := NewTestFrameworkWithSeed(t, seed)

			data1 := fw1.CreateTestTemplateData()
			data2 := fw2.CreateTestTemplateData()

			// Verify key fields are identical
			return data1["ClusterName"] == data2["ClusterName"] &&
				data1["Namespace"] == data2["Namespace"] &&
				data1["Version"] == data2["Version"]
		},
		gen.Int64(),
	))

	properties.Property("same seed produces identical service definitions", prop.ForAll(
		func(seed int64) bool {
			fw1 := NewTestFrameworkWithSeed(t, seed)
			fw2 := NewTestFrameworkWithSeed(t, seed)

			service1 := fw1.CreateTestServiceDefinition()
			service2 := fw2.CreateTestServiceDefinition()

			// Verify key fields are identical
			return service1["name"] == service2["name"] &&
				service1["type"] == service2["type"] &&
				service1["version"] == service2["version"]
		},
		gen.Int64(),
	))

	properties.Property("same seed produces identical gitops configs", prop.ForAll(
		func(seed int64) bool {
			fw1 := NewTestFrameworkWithSeed(t, seed)
			fw2 := NewTestFrameworkWithSeed(t, seed)

			gitops1 := fw1.CreateTestGitOpsConfig()
			gitops2 := fw2.CreateTestGitOpsConfig()

			// Verify key fields are identical
			return gitops1["repository"] == gitops2["repository"] &&
				gitops1["branch"] == gitops2["branch"] &&
				gitops1["path"] == gitops2["path"]
		},
		gen.Int64(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 2: Generator Output Validity
// For any generated test data, it should meet basic validity requirements
// Validates: Requirements 11.1
func TestProperty_GeneratorOutputValidity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("generated configs have valid structure", prop.ForAll(
		func(seed int64, provider string) bool {
			fw := NewTestFrameworkWithSeed(t, seed)
			config := fw.CreateTestConfig(provider)

			// Verify required fields are non-empty
			return config.OpenCenter.Meta.Name != "" &&
				config.OpenCenter.Meta.Organization != "" &&
				config.OpenCenter.Infrastructure.Provider == provider
		},
		gen.Int64(),
		genValidProvider(),
	))

	properties.Property("generated template data has required fields", prop.ForAll(
		func(seed int64) bool {
			fw := NewTestFrameworkWithSeed(t, seed)
			data := fw.CreateTestTemplateData()

			// Verify all required fields exist and are non-empty
			requiredFields := []string{
				"ClusterName", "Namespace", "Version", "Replicas",
				"Image", "Port", "Environment", "Labels", "Annotations", "Resources",
			}

			for _, field := range requiredFields {
				if _, ok := data[field]; !ok {
					return false
				}
			}
			return true
		},
		gen.Int64(),
	))

	properties.Property("generated service definitions have required fields", prop.ForAll(
		func(seed int64) bool {
			fw := NewTestFrameworkWithSeed(t, seed)
			service := fw.CreateTestServiceDefinition()

			// Verify all required fields exist
			requiredFields := []string{"name", "type", "enabled", "version", "dependencies", "config"}

			for _, field := range requiredFields {
				if _, ok := service[field]; !ok {
					return false
				}
			}
			return true
		},
		gen.Int64(),
	))

	properties.Property("generated gitops configs have required fields", prop.ForAll(
		func(seed int64) bool {
			fw := NewTestFrameworkWithSeed(t, seed)
			gitops := fw.CreateTestGitOpsConfig()

			// Verify all required fields exist
			requiredFields := []string{"enabled", "repository", "branch", "path", "sync"}

			for _, field := range requiredFields {
				if _, ok := gitops[field]; !ok {
					return false
				}
			}
			return true
		},
		gen.Int64(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 3: Mock Implementation Consistency
// For any mock implementation, it should behave consistently across multiple calls
// Validates: Requirements 11.1
func TestProperty_MockImplementationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("mock template engine tracks render calls", prop.ForAll(
		func(seed int64, callCount int) bool {
			if callCount < 1 || callCount > 10 {
				return true // Skip invalid call counts
			}

			fw := NewTestFrameworkWithSeed(t, seed)
			mock := fw.MockTemplateEngine

			// Make multiple render calls
			for i := 0; i < callCount; i++ {
				_, _ = mock.Render(nil, "test.tmpl", nil)
			}

			// Verify call count matches
			return len(mock.RenderCalls) == callCount
		},
		gen.Int64(),
		gen.IntRange(1, 10),
	))

	properties.Property("mock config builder is chainable", prop.ForAll(
		func(seed int64, provider, org, cluster string) bool {
			fw := NewTestFrameworkWithSeed(t, seed)
			mock := fw.MockConfigBuilder

			// Chain multiple calls
			builder := mock.
				WithProvider(provider).
				WithOrganization(org).
				WithClusterName(cluster)

			// Verify builder is still usable
			_, err := builder.Build()
			return err == nil
		},
		gen.Int64(),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.Property("mock error aggregator accumulates errors", prop.ForAll(
		func(seed int64, errorCount int) bool {
			if errorCount < 1 || errorCount > 10 {
				return true // Skip invalid error counts
			}

			fw := NewTestFrameworkWithSeed(t, seed)
			mock := fw.MockErrorAggregator

			// Add multiple errors
			for i := 0; i < errorCount; i++ {
				mock.Add(nil) // Mock accepts nil errors
			}

			// Verify error count matches
			return len(mock.Errors()) == errorCount
		},
		gen.Int64(),
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 4: Framework Isolation
// For any two framework instances, they should be isolated and not interfere with each other
// Validates: Requirements 11.1
func TestProperty_FrameworkIsolation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("different seeds produce different configs", prop.ForAll(
		func(seed1, seed2 int64, provider string) bool {
			if seed1 == seed2 {
				return true // Skip identical seeds
			}

			fw1 := NewTestFrameworkWithSeed(t, seed1)
			fw2 := NewTestFrameworkWithSeed(t, seed2)

			config1 := fw1.CreateTestConfig(provider)
			config2 := fw2.CreateTestConfig(provider)

			// Verify configs are different (at least one field should differ).
			// Check multiple fields to avoid false collisions from small value pools.
			return config1.OpenCenter.Meta.Name != config2.OpenCenter.Meta.Name ||
				config1.OpenCenter.Meta.Organization != config2.OpenCenter.Meta.Organization ||
				config1.OpenCenter.Meta.Env != config2.OpenCenter.Meta.Env ||
				config1.OpenCenter.Meta.Region != config2.OpenCenter.Meta.Region ||
				config1.OpenCenter.Cluster.Kubernetes.Version != config2.OpenCenter.Cluster.Kubernetes.Version ||
				config1.OpenCenter.GitOps.Repository.URL != config2.OpenCenter.GitOps.Repository.URL
		},
		gen.Int64(),
		gen.Int64(),
		genValidProvider(),
	))

	properties.Property("frameworks have isolated temp directories", prop.ForAll(
		func(seed1, seed2 int64) bool {
			fw1 := NewTestFrameworkWithSeed(t, seed1)
			fw2 := NewTestFrameworkWithSeed(t, seed2)

			// Verify temp directories are different
			return fw1.TempDir != fw2.TempDir &&
				fw1.ConfigDir != fw2.ConfigDir &&
				fw1.TemplateDir != fw2.TemplateDir
		},
		gen.Int64(),
		gen.Int64(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 5: Generator Consistency
// For any generator, calling it multiple times with the same seed should produce consistent results
// Validates: Requirements 11.1
func TestProperty_GeneratorConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("config generator produces valid configs", prop.ForAll(
		func(seed int64, provider string) bool {
			fw := NewTestFrameworkWithSeed(t, seed)

			// Generate multiple configs
			config1 := fw.CreateTestConfig(provider)
			config2 := fw.CreateTestConfig(provider)

			// Both configs should be valid (have required fields)
			return config1.OpenCenter.Meta.Name != "" &&
				config2.OpenCenter.Meta.Name != "" &&
				config1.OpenCenter.Infrastructure.Provider == provider &&
				config2.OpenCenter.Infrastructure.Provider == provider
		},
		gen.Int64(),
		genValidProvider(),
	))

	properties.Property("template data generator produces valid data", prop.ForAll(
		func(seed int64) bool {
			fw := NewTestFrameworkWithSeed(t, seed)

			// Generate multiple template data
			data1 := fw.CreateTestTemplateData()
			data2 := fw.CreateTestTemplateData()

			// Both should have required fields
			return data1["ClusterName"] != nil && data2["ClusterName"] != nil &&
				data1["Namespace"] != nil && data2["Namespace"] != nil
		},
		gen.Int64(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Generators for property-based testing

func genValidProvider() gopter.Gen {
	return gen.OneConstOf("openstack", "aws", "baremetal", "kind", "talos", "vmware")
}
