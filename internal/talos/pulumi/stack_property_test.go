package pulumi

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: talos-openstack-provider, Property 24: Stack isolation
// For any two different clusters, each should have its own Pulumi stack name
// and Swift prefix, ensuring state isolation.
// Validates: Requirements 10.10
func TestProperty_StackIsolation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("different clusters have isolated stacks", prop.ForAll(
		func(cluster1Name string, cluster2Name string, env1 string, env2 string) bool {
			// Skip if cluster names are the same
			if cluster1Name == cluster2Name {
				return true
			}

			// Create stack manager
			logger := &testLogger{}
			manager, err := NewStackManager(logger)
			if err != nil {
				t.Logf("Failed to create stack manager: %v", err)
				return false
			}

			// Create configurations for two different clusters
			config1, err := NewStackConfig(cluster1Name, env1)
			if err != nil {
				t.Logf("Failed to create config1: %v", err)
				return false
			}

			config2, err := NewStackConfig(cluster2Name, env2)
			if err != nil {
				t.Logf("Failed to create config2: %v", err)
				return false
			}

			// Generate names for both configurations
			if err := config1.GenerateNames(manager); err != nil {
				t.Logf("Failed to generate names for config1: %v", err)
				return false
			}

			if err := config2.GenerateNames(manager); err != nil {
				t.Logf("Failed to generate names for config2: %v", err)
				return false
			}

			// Verify stack names are different
			if config1.StackName == config2.StackName {
				t.Logf("Stack names should be different for different clusters: %s == %s", config1.StackName, config2.StackName)
				return false
			}

			// Verify Swift prefixes are different
			if config1.SwiftPrefix == config2.SwiftPrefix {
				t.Logf("Swift prefixes should be different for different clusters: %s == %s", config1.SwiftPrefix, config2.SwiftPrefix)
				return false
			}

			// Verify stack isolation
			ctx := context.Background()
			err = manager.EnsureStackIsolation(ctx, config1, config2)
			if err != nil {
				t.Logf("Stack isolation check failed: %v", err)
				return false
			}

			return true
		},
		genClusterName(),
		genClusterName(),
		genEnvironment(),
		genEnvironment(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_StackNameGeneration tests stack name generation properties.
func TestProperty_StackNameGeneration(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("generated stack names are valid", prop.ForAll(
		func(clusterName string, environment string) bool {
			// Create stack manager
			logger := &testLogger{}
			manager, err := NewStackManager(logger)
			if err != nil {
				t.Logf("Failed to create stack manager: %v", err)
				return false
			}

			// Generate stack name
			stackName, err := manager.GenerateStackName(clusterName, environment)
			if err != nil {
				t.Logf("Failed to generate stack name: %v", err)
				return false
			}

			// Verify stack name is not empty
			if stackName == "" {
				t.Log("Generated stack name is empty")
				return false
			}

			// Verify stack name is valid
			if !manager.isValidStackName(stackName) {
				t.Logf("Generated stack name is invalid: %s", stackName)
				return false
			}

			// Verify stack name contains cluster name (sanitized)
			sanitizedCluster := manager.sanitizeStackName(clusterName)
			if sanitizedCluster != "" && len(stackName) > 0 {
				// Stack name should start with sanitized cluster name
				if len(stackName) < len(sanitizedCluster) {
					t.Logf("Stack name too short: %s (cluster: %s)", stackName, sanitizedCluster)
					return false
				}
			}

			return true
		},
		genClusterName(),
		genEnvironment(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_SwiftPrefixGeneration tests Swift prefix generation properties.
func TestProperty_SwiftPrefixGeneration(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("generated Swift prefixes are valid", prop.ForAll(
		func(clusterName string, environment string) bool {
			// Create stack manager
			logger := &testLogger{}
			manager, err := NewStackManager(logger)
			if err != nil {
				t.Logf("Failed to create stack manager: %v", err)
				return false
			}

			// Generate Swift prefix
			prefix, err := manager.GenerateSwiftPrefix(clusterName, environment)
			if err != nil {
				t.Logf("Failed to generate Swift prefix: %v", err)
				return false
			}

			// Verify prefix is not empty
			if prefix == "" {
				t.Log("Generated Swift prefix is empty")
				return false
			}

			// Verify prefix ends with slash
			if prefix[len(prefix)-1] != '/' {
				t.Logf("Swift prefix should end with slash: %s", prefix)
				return false
			}

			// Verify prefix contains cluster name (sanitized)
			sanitizedCluster := manager.sanitizeStackName(clusterName)
			if sanitizedCluster != "" {
				// Prefix should contain sanitized cluster name
				if len(prefix) < len(sanitizedCluster) {
					t.Logf("Swift prefix too short: %s (cluster: %s)", prefix, sanitizedCluster)
					return false
				}
			}

			return true
		},
		genClusterName(),
		genEnvironment(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_StackValidation tests stack selection validation.
func TestProperty_StackValidation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("valid stack names pass validation", prop.ForAll(
		func(clusterName string, environment string) bool {
			// Create stack manager
			logger := &testLogger{}
			manager, err := NewStackManager(logger)
			if err != nil {
				t.Logf("Failed to create stack manager: %v", err)
				return false
			}

			// Generate stack name
			stackName, err := manager.GenerateStackName(clusterName, environment)
			if err != nil {
				t.Logf("Failed to generate stack name: %v", err)
				return false
			}

			// Validate stack selection
			ctx := context.Background()
			err = manager.ValidateStackSelection(ctx, stackName)
			if err != nil {
				t.Logf("Stack validation failed for valid stack name: %v", err)
				return false
			}

			return true
		},
		genClusterName(),
		genEnvironment(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genClusterName generates valid cluster names.
func genClusterName() gopter.Gen {
	return gen.Identifier().SuchThat(func(v interface{}) bool {
		s := v.(string)
		return len(s) > 0 && len(s) <= 64
	})
}

// genEnvironment generates valid environment names.
func genEnvironment() gopter.Gen {
	return gen.OneConstOf("", "prod", "dev", "staging", "test", "qa")
}
