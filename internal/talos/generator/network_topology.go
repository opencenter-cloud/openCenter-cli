package generator

import (
	"context"
	"fmt"
	"net"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// GenerateNetworkTopology creates network definitions with three-zone architecture:
// management, control, and data networks with routers and routes.
func (g *generator) GenerateNetworkTopology(ctx context.Context, cfg *config.Config) (*talos.NetworkTopology, error) {
	if cfg == nil {
		return nil, talos.NewConfigurationError(
			"MISSING_CONFIG",
			"configuration cannot be nil",
			nil,
		)
	}

	// Get Talos configuration
	talosConfig := talos.DefaultTalosConfig()

	// Build network topology
	topology := &talos.NetworkTopology{
		ManagementNetwork: g.buildManagementNetwork(talosConfig),
		ControlNetwork:    g.buildControlNetwork(talosConfig),
		DataNetwork:       g.buildDataNetwork(talosConfig),
		Router:            g.buildRouter(cfg),
		Routes:            g.buildRoutes(talosConfig),
	}

	// Validate that networks have distinct CIDRs
	if err := g.validateDistinctCIDRs(topology); err != nil {
		return nil, talos.NewConfigurationError(
			"INVALID_NETWORK_CONFIG",
			"network CIDRs must be distinct",
			err,
		)
	}

	return topology, nil
}

// buildManagementNetwork creates the management network configuration.
func (g *generator) buildManagementNetwork(talosConfig *talos.TalosConfig) talos.Network {
	cidr := talosConfig.NetworkConfig.ManagementSubnet
	gateway := calculateGateway(cidr)

	return talos.Network{
		Name:    "management",
		CIDR:    cidr,
		Gateway: gateway,
		DNSServers: []string{
			"8.8.8.8",
			"8.8.4.4",
		},
		AllocationPool: calculateAllocationPool(cidr),
	}
}

// buildControlNetwork creates the control plane network configuration.
func (g *generator) buildControlNetwork(talosConfig *talos.TalosConfig) talos.Network {
	cidr := talosConfig.NetworkConfig.ControlSubnet
	gateway := calculateGateway(cidr)

	return talos.Network{
		Name:    "control",
		CIDR:    cidr,
		Gateway: gateway,
		DNSServers: []string{
			"8.8.8.8",
			"8.8.4.4",
		},
		AllocationPool: calculateAllocationPool(cidr),
	}
}

// buildDataNetwork creates the data plane network configuration.
func (g *generator) buildDataNetwork(talosConfig *talos.TalosConfig) talos.Network {
	cidr := talosConfig.NetworkConfig.DataSubnet
	gateway := calculateGateway(cidr)

	return talos.Network{
		Name:    "data",
		CIDR:    cidr,
		Gateway: gateway,
		DNSServers: []string{
			"8.8.8.8",
			"8.8.4.4",
		},
		AllocationPool: calculateAllocationPool(cidr),
	}
}

// buildRouter creates the router configuration.
func (g *generator) buildRouter(cfg *config.Config) talos.Router {
	clusterName := cfg.OpenCenter.Meta.Name
	if clusterName == "" {
		clusterName = "talos-cluster"
	}

	return talos.Router{
		Name:              fmt.Sprintf("%s-router", clusterName),
		ExternalNetworkID: "public", // This should be configurable
	}
}

// buildRoutes creates explicit routing rules between networks.
func (g *generator) buildRoutes(talosConfig *talos.TalosConfig) []talos.Route {
	return []talos.Route{
		{
			Destination: talosConfig.NetworkConfig.ControlSubnet,
			NextHop:     calculateGateway(talosConfig.NetworkConfig.ManagementSubnet),
		},
		{
			Destination: talosConfig.NetworkConfig.DataSubnet,
			NextHop:     calculateGateway(talosConfig.NetworkConfig.ManagementSubnet),
		},
		{
			Destination: talosConfig.NetworkConfig.ManagementSubnet,
			NextHop:     calculateGateway(talosConfig.NetworkConfig.ControlSubnet),
		},
	}
}

// calculateGateway calculates the gateway IP for a given CIDR.
// Uses the first usable IP in the subnet.
func calculateGateway(cidr string) string {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}

	// Get the first IP in the network
	ip = ip.Mask(ipNet.Mask)

	// Increment to get the first usable IP (gateway)
	ip[len(ip)-1]++

	return ip.String()
}

// calculateAllocationPool calculates the IP allocation pool for a given CIDR.
// Reserves the first 10 IPs for infrastructure and uses the rest for allocation.
func calculateAllocationPool(cidr string) talos.AllocationPool {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return talos.AllocationPool{}
	}

	// Get the first IP in the network
	ip = ip.Mask(ipNet.Mask)

	// Start allocation from .11 (reserve .1-.10 for infrastructure)
	startIP := make(net.IP, len(ip))
	copy(startIP, ip)
	startIP[len(startIP)-1] += 11

	// End allocation at the last usable IP (broadcast - 1)
	endIP := make(net.IP, len(ip))
	copy(endIP, ip)
	for i := range endIP {
		endIP[i] |= ^ipNet.Mask[i]
	}
	endIP[len(endIP)-1]-- // Subtract 1 for broadcast

	return talos.AllocationPool{
		Start: startIP.String(),
		End:   endIP.String(),
	}
}

// validateDistinctCIDRs ensures all networks have distinct, non-overlapping CIDRs.
func (g *generator) validateDistinctCIDRs(topology *talos.NetworkTopology) error {
	networks := []talos.Network{
		topology.ManagementNetwork,
		topology.ControlNetwork,
		topology.DataNetwork,
	}

	// Parse all CIDRs
	cidrs := make([]*net.IPNet, len(networks))
	for i, network := range networks {
		_, ipNet, err := net.ParseCIDR(network.CIDR)
		if err != nil {
			return fmt.Errorf("invalid CIDR %s: %w", network.CIDR, err)
		}
		cidrs[i] = ipNet
	}

	// Check for overlaps
	for i := 0; i < len(cidrs); i++ {
		for j := i + 1; j < len(cidrs); j++ {
			if cidrsOverlap(cidrs[i], cidrs[j]) {
				return fmt.Errorf("networks %s and %s overlap", networks[i].Name, networks[j].Name)
			}
		}
	}

	return nil
}

// cidrsOverlap checks if two CIDRs overlap.
func cidrsOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}
