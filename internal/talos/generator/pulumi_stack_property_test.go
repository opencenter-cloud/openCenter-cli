package generator

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/openCenter-cli/internal/config"
	"gopkg.in/yaml.v3"
)

// Feature: talos-openstack-provider, Property 5: Pulumi configuration completeness
// For any generated Pulumi stack configuration, the configuration should define
// all required infrastructure components: networks, routers, security groups,
// load balancers, and credential policies.
// Validates: Requirements 2.6, 2.7
func TestProperty_PulumiConfigurationCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all Pulumi stack configs have required components",
		prop.ForAll(
			func(clusterName string) bool {
				// Create a generator with minimal config
				cfg := &config.Config{
					OpenCenter: config.SimplifiedOpenCenter{
						Meta: config.ClusterMeta{
							Name: clusterName,
						},
					},
				}
				g := New(cfg)

				// Generate Pulumi stack config
				stackConfigBytes, err := g.GeneratePulumiStack(context.Background(), cfg)
				if err != nil {
					t.Logf("Failed to generate Pulumi stack config: %v", err)
					return false
				}

				// Parse the YAML to verify components
				var stackConfig map[string]interface{}
				if err := yaml.Unmarshal(stackConfigBytes, &stackConfig); err != nil {
					t.Logf("Failed to unmarshal Pulumi stack config: %v", err)
					return false
				}

				// Verify all required components are present
				hasNetworks := containsNetworks(stackConfig)
				hasRouters := containsRouters(stackConfig)
				hasSecurityGroups := containsSecurityGroups(stackConfig)
				hasLoadBalancers := containsLoadBalancers(stackConfig)
				hasCredentialPolicies := containsCredentialPolicies(stackConfig)

				if !hasNetworks {
					t.Logf("Missing networks configuration")
				}
				if !hasRouters {
					t.Logf("Missing routers configuration")
				}
				if !hasSecurityGroups {
					t.Logf("Missing security groups configuration")
				}
				if !hasLoadBalancers {
					t.Logf("Missing load balancers configuration")
				}
				if !hasCredentialPolicies {
					t.Logf("Missing credential policies configuration")
				}

				return hasNetworks && hasRouters && hasSecurityGroups && hasLoadBalancers && hasCredentialPolicies
			},
			gen.Identifier(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// containsNetworks checks if the Pulumi stack config has network definitions.
func containsNetworks(stackConfig map[string]interface{}) bool {
	configSection, ok := stackConfig["config"].(map[string]interface{})
	if !ok {
		return false
	}

	networksRaw, ok := configSection["talos:networks"]
	if !ok {
		return false
	}

	// Check if it's a slice
	networks, ok := networksRaw.([]interface{})
	if !ok {
		return false
	}

	// Should have at least 3 networks (management, control, data)
	if len(networks) < 3 {
		return false
	}

	// Verify each network has required fields
	for _, netRaw := range networks {
		net, ok := netRaw.(map[string]interface{})
		if !ok {
			return false
		}

		if _, hasName := net["name"]; !hasName {
			return false
		}
		if _, hasCIDR := net["cidr"]; !hasCIDR {
			return false
		}
	}

	return true
}

// containsRouters checks if the Pulumi stack config has router definitions.
func containsRouters(stackConfig map[string]interface{}) bool {
	configSection, ok := stackConfig["config"].(map[string]interface{})
	if !ok {
		return false
	}

	routersRaw, ok := configSection["talos:routers"]
	if !ok {
		return false
	}

	// Check if it's a slice
	routers, ok := routersRaw.([]interface{})
	if !ok {
		return false
	}

	// Should have at least 1 router
	if len(routers) < 1 {
		return false
	}

	// Verify router has required fields
	for _, routerRaw := range routers {
		router, ok := routerRaw.(map[string]interface{})
		if !ok {
			return false
		}

		if _, hasName := router["name"]; !hasName {
			return false
		}
	}

	return true
}

// containsSecurityGroups checks if the Pulumi stack config has security group definitions.
func containsSecurityGroups(stackConfig map[string]interface{}) bool {
	configSection, ok := stackConfig["config"].(map[string]interface{})
	if !ok {
		return false
	}

	securityGroupsRaw, ok := configSection["talos:securityGroups"]
	if !ok {
		return false
	}

	// Check if it's a slice
	securityGroups, ok := securityGroupsRaw.([]interface{})
	if !ok {
		return false
	}

	// Should have at least 3 security groups (control-plane, worker, bastion)
	if len(securityGroups) < 3 {
		return false
	}

	// Verify each security group has required fields
	for _, sgRaw := range securityGroups {
		sg, ok := sgRaw.(map[string]interface{})
		if !ok {
			return false
		}

		if _, hasName := sg["name"]; !hasName {
			return false
		}
		if _, hasDefaultPolicy := sg["defaultPolicy"]; !hasDefaultPolicy {
			return false
		}
		if _, hasRules := sg["rules"]; !hasRules {
			return false
		}
	}

	return true
}

// containsLoadBalancers checks if the Pulumi stack config has load balancer definitions.
func containsLoadBalancers(stackConfig map[string]interface{}) bool {
	configSection, ok := stackConfig["config"].(map[string]interface{})
	if !ok {
		return false
	}

	loadBalancersRaw, ok := configSection["talos:loadBalancers"]
	if !ok {
		return false
	}

	// Check if it's a slice
	loadBalancers, ok := loadBalancersRaw.([]interface{})
	if !ok {
		return false
	}

	// Should have at least 1 load balancer
	if len(loadBalancers) < 1 {
		return false
	}

	// Verify each load balancer has required fields
	for _, lbRaw := range loadBalancers {
		lb, ok := lbRaw.(map[string]interface{})
		if !ok {
			return false
		}

		if _, hasName := lb["name"]; !hasName {
			return false
		}
		if _, hasPort := lb["port"]; !hasPort {
			return false
		}
	}

	return true
}

// containsCredentialPolicies checks if the Pulumi stack config has credential policy definitions.
func containsCredentialPolicies(stackConfig map[string]interface{}) bool {
	configSection, ok := stackConfig["config"].(map[string]interface{})
	if !ok {
		return false
	}

	credentialPoliciesRaw, ok := configSection["talos:credentialPolicies"]
	if !ok {
		return false
	}

	// Check if it's a slice
	credentialPolicies, ok := credentialPoliciesRaw.([]interface{})
	if !ok {
		return false
	}

	// Should have at least 1 credential policy
	if len(credentialPolicies) < 1 {
		return false
	}

	// Verify each credential policy has required fields
	for _, cpRaw := range credentialPolicies {
		cp, ok := cpRaw.(map[string]interface{})
		if !ok {
			return false
		}

		if _, hasName := cp["name"]; !hasName {
			return false
		}
		if _, hasPermissions := cp["permissions"]; !hasPermissions {
			return false
		}
	}

	return true
}
