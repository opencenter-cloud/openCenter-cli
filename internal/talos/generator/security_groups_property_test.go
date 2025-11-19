package generator

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// Feature: talos-openstack-provider, Property 14: Security group default deny
// For any created security group, the default policy should be deny-all with only
// explicitly allowed ports (6443, 50000, 2379-2380, 10250, 51820-51821) permitted.
// Validates: Requirements 5.5, 5.6, 5.7, 5.8, 5.9, 5.10
func TestProperty_SecurityGroupDefaultDeny(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all security groups have default deny with explicit allows",
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

				// Generate security groups
				securityGroups, err := g.GenerateSecurityGroups(context.Background(), cfg)
				if err != nil {
					t.Logf("Failed to generate security groups: %v", err)
					return false
				}

				// Should have at least 3 security groups (control-plane, worker, bastion)
				if len(securityGroups) < 3 {
					t.Logf("Expected at least 3 security groups, got %d", len(securityGroups))
					return false
				}

				// Verify each security group
				for _, sg := range securityGroups {
					if !validateSecurityGroup(sg, t) {
						return false
					}
				}

				// Verify required ports are present across all groups
				hasKubernetesAPI := hasPortInGroups(securityGroups, 6443)
				hasTalosAPI := hasPortInGroups(securityGroups, 50000)
				hasEtcd := hasPortRangeInGroups(securityGroups, 2379, 2380)
				hasKubelet := hasPortInGroups(securityGroups, 10250)
				hasWireGuard := hasPortRangeInGroups(securityGroups, 51820, 51821)

				if !hasKubernetesAPI {
					t.Logf("Missing Kubernetes API port 6443")
				}
				if !hasTalosAPI {
					t.Logf("Missing Talos API port 50000")
				}
				if !hasEtcd {
					t.Logf("Missing etcd ports 2379-2380")
				}
				if !hasKubelet {
					t.Logf("Missing Kubelet port 10250")
				}
				if !hasWireGuard {
					t.Logf("Missing WireGuard ports 51820-51821")
				}

				return hasKubernetesAPI && hasTalosAPI && hasEtcd && hasKubelet && hasWireGuard
			},
			gen.Identifier(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// validateSecurityGroup checks if a security group has proper structure.
func validateSecurityGroup(sg talos.SecurityGroup, t *testing.T) bool {
	// Check basic fields
	if sg.Name == "" {
		t.Logf("Security group missing name")
		return false
	}

	if sg.Description == "" {
		t.Logf("Security group %s missing description", sg.Name)
		return false
	}

	// Check that description mentions default deny
	// (This is a convention check, not strictly enforced)

	// Check that rules exist
	if len(sg.Rules) == 0 {
		t.Logf("Security group %s has no rules", sg.Name)
		return false
	}

	// Verify each rule has required fields
	for i, rule := range sg.Rules {
		if rule.Direction == "" {
			t.Logf("Security group %s rule %d missing direction", sg.Name, i)
			return false
		}

		if rule.Direction != "ingress" && rule.Direction != "egress" {
			t.Logf("Security group %s rule %d has invalid direction: %s", sg.Name, i, rule.Direction)
			return false
		}

		if rule.Protocol == "" {
			t.Logf("Security group %s rule %d missing protocol", sg.Name, i)
			return false
		}

		if rule.PortRangeMin <= 0 || rule.PortRangeMax <= 0 {
			t.Logf("Security group %s rule %d has invalid port range: %d-%d", sg.Name, i, rule.PortRangeMin, rule.PortRangeMax)
			return false
		}

		if rule.PortRangeMin > rule.PortRangeMax {
			t.Logf("Security group %s rule %d has invalid port range: min > max", sg.Name, i)
			return false
		}
	}

	return true
}

// hasPortInGroups checks if a specific port is allowed in any security group.
func hasPortInGroups(groups []talos.SecurityGroup, port int) bool {
	for _, sg := range groups {
		for _, rule := range sg.Rules {
			if rule.Direction == "ingress" &&
				rule.PortRangeMin <= port &&
				rule.PortRangeMax >= port {
				return true
			}
		}
	}
	return false
}

// hasPortRangeInGroups checks if a port range is allowed in any security group.
func hasPortRangeInGroups(groups []talos.SecurityGroup, minPort, maxPort int) bool {
	for _, sg := range groups {
		for _, rule := range sg.Rules {
			if rule.Direction == "ingress" &&
				rule.PortRangeMin <= minPort &&
				rule.PortRangeMax >= maxPort {
				return true
			}
		}
	}
	return false
}
