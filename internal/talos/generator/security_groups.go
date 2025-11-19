package generator

import (
	"context"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// GenerateSecurityGroups creates security group rules with default-deny policies
// and explicit allow rules for required ports only.
func (g *generator) GenerateSecurityGroups(ctx context.Context, cfg *config.Config) ([]talos.SecurityGroup, error) {
	if cfg == nil {
		return nil, talos.NewConfigurationError(
			"MISSING_CONFIG",
			"configuration cannot be nil",
			nil,
		)
	}

	// Get Talos configuration
	talosConfig := talos.DefaultTalosConfig()

	// Build security groups
	securityGroups := []talos.SecurityGroup{
		g.buildControlPlaneSecurityGroup(talosConfig),
		g.buildWorkerSecurityGroup(talosConfig),
		g.buildBastionSecurityGroup(talosConfig),
	}

	return securityGroups, nil
}

// buildControlPlaneSecurityGroup creates security group for control plane nodes.
func (g *generator) buildControlPlaneSecurityGroup(talosConfig *talos.TalosConfig) talos.SecurityGroup {
	return talos.SecurityGroup{
		Name:        "talos-control-plane",
		Description: "Security group for Talos control plane nodes (default deny)",
		Rules: []talos.SecurityRule{
			// Kubernetes API server
			{
				Direction:      "ingress",
				Protocol:       "tcp",
				PortRangeMin:   6443,
				PortRangeMax:   6443,
				RemoteIPPrefix: "10.0.0.0/8", // Internal networks only
				Description:    "Kubernetes API server",
			},
			// Talos API
			{
				Direction:      "ingress",
				Protocol:       "tcp",
				PortRangeMin:   talosConfig.NetworkConfig.TalosAPIPort,
				PortRangeMax:   talosConfig.NetworkConfig.TalosAPIPort,
				RemoteIPPrefix: talosConfig.NetworkConfig.ManagementSubnet, // Only from management network
				Description:    "Talos API (restricted to management network)",
			},
			// etcd peer communication
			{
				Direction:     "ingress",
				Protocol:      "tcp",
				PortRangeMin:  2379,
				PortRangeMax:  2380,
				RemoteGroupID: "self", // Only from other control plane nodes
				Description:   "etcd peer communication",
			},
			// Kubelet
			{
				Direction:      "ingress",
				Protocol:       "tcp",
				PortRangeMin:   10250,
				PortRangeMax:   10250,
				RemoteIPPrefix: "10.0.0.0/8", // Internal networks only
				Description:    "Kubelet",
			},
			// KubePrism (if enabled)
			{
				Direction:      "ingress",
				Protocol:       "tcp",
				PortRangeMin:   7445,
				PortRangeMax:   7445,
				RemoteIPPrefix: "10.0.0.0/8", // Internal networks only
				Description:    "KubePrism internal load balancer",
			},
			// Allow all egress (default)
			{
				Direction:      "egress",
				Protocol:       "tcp",
				PortRangeMin:   1,
				PortRangeMax:   65535,
				RemoteIPPrefix: "0.0.0.0/0",
				Description:    "Allow all outbound TCP",
			},
			{
				Direction:      "egress",
				Protocol:       "udp",
				PortRangeMin:   1,
				PortRangeMax:   65535,
				RemoteIPPrefix: "0.0.0.0/0",
				Description:    "Allow all outbound UDP",
			},
		},
	}
}

// buildWorkerSecurityGroup creates security group for worker nodes.
func (g *generator) buildWorkerSecurityGroup(talosConfig *talos.TalosConfig) talos.SecurityGroup {
	return talos.SecurityGroup{
		Name:        "talos-worker",
		Description: "Security group for Talos worker nodes (default deny)",
		Rules: []talos.SecurityRule{
			// Kubelet
			{
				Direction:      "ingress",
				Protocol:       "tcp",
				PortRangeMin:   10250,
				PortRangeMax:   10250,
				RemoteIPPrefix: "10.0.0.0/8", // Internal networks only
				Description:    "Kubelet",
			},
			// NodePort services range
			{
				Direction:      "ingress",
				Protocol:       "tcp",
				PortRangeMin:   30000,
				PortRangeMax:   32767,
				RemoteIPPrefix: "10.0.0.0/8", // Internal networks only
				Description:    "NodePort services",
			},
			// Talos API (restricted)
			{
				Direction:      "ingress",
				Protocol:       "tcp",
				PortRangeMin:   talosConfig.NetworkConfig.TalosAPIPort,
				PortRangeMax:   talosConfig.NetworkConfig.TalosAPIPort,
				RemoteIPPrefix: talosConfig.NetworkConfig.ManagementSubnet, // Only from management network
				Description:    "Talos API (restricted to management network)",
			},
			// Allow all egress (default)
			{
				Direction:      "egress",
				Protocol:       "tcp",
				PortRangeMin:   1,
				PortRangeMax:   65535,
				RemoteIPPrefix: "0.0.0.0/0",
				Description:    "Allow all outbound TCP",
			},
			{
				Direction:      "egress",
				Protocol:       "udp",
				PortRangeMin:   1,
				PortRangeMax:   65535,
				RemoteIPPrefix: "0.0.0.0/0",
				Description:    "Allow all outbound UDP",
			},
		},
	}
}

// buildBastionSecurityGroup creates security group for bastion/WireGuard gateway.
func (g *generator) buildBastionSecurityGroup(talosConfig *talos.TalosConfig) talos.SecurityGroup {
	return talos.SecurityGroup{
		Name:        "talos-bastion",
		Description: "Security group for bastion/WireGuard gateway (default deny)",
		Rules: []talos.SecurityRule{
			// WireGuard VPN
			{
				Direction:      "ingress",
				Protocol:       "udp",
				PortRangeMin:   talosConfig.NetworkConfig.WireGuardPort,
				PortRangeMax:   talosConfig.NetworkConfig.WireGuardPort + 1,
				RemoteIPPrefix: "0.0.0.0/0", // Allow from anywhere (VPN entry point)
				Description:    fmt.Sprintf("WireGuard VPN (ports %d-%d)", talosConfig.NetworkConfig.WireGuardPort, talosConfig.NetworkConfig.WireGuardPort+1),
			},
			// SSH (for emergency access only, should be disabled in production)
			{
				Direction:      "ingress",
				Protocol:       "tcp",
				PortRangeMin:   22,
				PortRangeMax:   22,
				RemoteIPPrefix: "0.0.0.0/0", // Should be restricted to specific IPs in production
				Description:    "SSH (emergency access only - restrict in production)",
			},
			// Allow all egress (default)
			{
				Direction:      "egress",
				Protocol:       "tcp",
				PortRangeMin:   1,
				PortRangeMax:   65535,
				RemoteIPPrefix: "0.0.0.0/0",
				Description:    "Allow all outbound TCP",
			},
			{
				Direction:      "egress",
				Protocol:       "udp",
				PortRangeMin:   1,
				PortRangeMax:   65535,
				RemoteIPPrefix: "0.0.0.0/0",
				Description:    "Allow all outbound UDP",
			},
		},
	}
}
