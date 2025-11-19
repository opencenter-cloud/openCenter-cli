package generator

import (
	"context"
	"net"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// Feature: talos-openstack-provider, Property 13: Network segmentation
// For any provisioned cluster, exactly three networks should be created
// (management, control, data) with distinct CIDR ranges and explicit routing rules.
// Validates: Requirements 5.1, 5.2, 5.3, 5.4
func TestProperty_NetworkSegmentation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all clusters have three distinct networks",
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

				// Generate network topology
				topology, err := g.GenerateNetworkTopology(context.Background(), cfg)
				if err != nil {
					t.Logf("Failed to generate network topology: %v", err)
					return false
				}

				// Verify three networks exist
				networks := []talos.Network{
					topology.ManagementNetwork,
					topology.ControlNetwork,
					topology.DataNetwork,
				}

				hasThreeNetworks := len(networks) == 3
				hasDistinctCIDRs := validateDistinctNetworkCIDRs(networks)
				hasExplicitRoutes := len(topology.Routes) > 0
				hasRouter := topology.Router.Name != ""

				if !hasThreeNetworks {
					t.Logf("Expected 3 networks, got %d", len(networks))
				}
				if !hasDistinctCIDRs {
					t.Logf("Networks do not have distinct CIDRs")
				}
				if !hasExplicitRoutes {
					t.Logf("No explicit routes configured")
				}
				if !hasRouter {
					t.Logf("No router configured")
				}

				return hasThreeNetworks && hasDistinctCIDRs && hasExplicitRoutes && hasRouter
			},
			gen.Identifier(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// validateDistinctNetworkCIDRs checks that all networks have distinct, non-overlapping CIDRs.
func validateDistinctNetworkCIDRs(networks []talos.Network) bool {
	if len(networks) != 3 {
		return false
	}

	// Parse all CIDRs
	cidrs := make([]*net.IPNet, len(networks))
	for i, network := range networks {
		_, ipNet, err := net.ParseCIDR(network.CIDR)
		if err != nil {
			return false
		}
		cidrs[i] = ipNet
	}

	// Check that all CIDRs are distinct (no overlaps)
	for i := 0; i < len(cidrs); i++ {
		for j := i + 1; j < len(cidrs); j++ {
			// Check if CIDRs overlap
			if cidrs[i].Contains(cidrs[j].IP) || cidrs[j].Contains(cidrs[i].IP) {
				return false
			}
		}
	}

	// Verify each network has required fields
	for _, network := range networks {
		if network.Name == "" {
			return false
		}
		if network.CIDR == "" {
			return false
		}
		if network.Gateway == "" {
			return false
		}
		// Verify gateway is within the CIDR
		_, ipNet, _ := net.ParseCIDR(network.CIDR)
		gatewayIP := net.ParseIP(network.Gateway)
		if !ipNet.Contains(gatewayIP) {
			return false
		}
	}

	return true
}
