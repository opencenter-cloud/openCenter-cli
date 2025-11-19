package generator

import (
	"context"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
	"gopkg.in/yaml.v3"
)

// GeneratePulumiStack creates Pulumi stack configuration with network topology,
// security groups, load balancers, and credential policies.
func (g *generator) GeneratePulumiStack(ctx context.Context, cfg *config.Config) ([]byte, error) {
	if cfg == nil {
		return nil, talos.NewConfigurationError(
			"MISSING_CONFIG",
			"configuration cannot be nil",
			nil,
		)
	}

	// Get Talos configuration
	talosConfig := talos.DefaultTalosConfig()

	// Build Pulumi stack configuration
	stackConfig := g.buildPulumiStackConfig(cfg, talosConfig)

	// Marshal to YAML
	data, err := yaml.Marshal(stackConfig)
	if err != nil {
		return nil, talos.NewConfigurationError(
			"MARSHAL_ERROR",
			"failed to marshal Pulumi stack config to YAML",
			err,
		)
	}

	return data, nil
}

// buildPulumiStackConfig constructs the Pulumi stack configuration structure.
func (g *generator) buildPulumiStackConfig(cfg *config.Config, talosConfig *talos.TalosConfig) map[string]interface{} {
	clusterName := cfg.OpenCenter.Meta.Name
	if clusterName == "" {
		clusterName = "talos-cluster"
	}

	stackConfig := map[string]interface{}{
		"name":        fmt.Sprintf("%s-stack", clusterName),
		"runtime":     "go",
		"description": fmt.Sprintf("Talos cluster infrastructure for %s", clusterName),
		"config": map[string]interface{}{
			// Network topology configuration
			"talos:managementSubnet": talosConfig.NetworkConfig.ManagementSubnet,
			"talos:controlSubnet":    talosConfig.NetworkConfig.ControlSubnet,
			"talos:dataSubnet":       talosConfig.NetworkConfig.DataSubnet,

			// Network configuration
			"talos:networks": g.buildPulumiNetworkConfig(talosConfig),

			// Router configuration
			"talos:routers": g.buildRouterConfig(clusterName),

			// Security group configuration
			"talos:securityGroups": g.buildSecurityGroupConfig(talosConfig),

			// Load balancer configuration
			"talos:loadBalancers": g.buildLoadBalancerConfig(clusterName, talosConfig),

			// Credential policy configuration
			"talos:credentialPolicies": g.buildCredentialPolicies(),

			// Image configuration
			"talos:imageURL":          talosConfig.ImageURL,
			"talos:imageSignature":    talosConfig.ImageSignature,
			"talos:imageVerification": talosConfig.SecurityConfig.ImageVerification,

			// Cluster configuration
			"talos:clusterName": clusterName,
			"talos:version":     talosConfig.Version,
		},
	}

	return stackConfig
}

// buildPulumiNetworkConfig creates network topology configuration for Pulumi.
func (g *generator) buildPulumiNetworkConfig(talosConfig *talos.TalosConfig) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "management",
			"cidr":        talosConfig.NetworkConfig.ManagementSubnet,
			"description": "Management network for bastion and WireGuard access",
		},
		{
			"name":        "control",
			"cidr":        talosConfig.NetworkConfig.ControlSubnet,
			"description": "Control plane network for Talos control nodes",
		},
		{
			"name":        "data",
			"cidr":        talosConfig.NetworkConfig.DataSubnet,
			"description": "Data plane network for worker nodes",
		},
	}
}

// buildRouterConfig creates router configuration.
func (g *generator) buildRouterConfig(clusterName string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":            fmt.Sprintf("%s-router", clusterName),
			"description":     "Main router for cluster networks",
			"externalNetwork": "public", // This should be configurable
			"routes": []map[string]interface{}{
				{
					"destination": "10.0.0.0/8",
					"description": "Internal routing",
				},
			},
		},
	}
}

// buildSecurityGroupConfig creates security group configuration.
func (g *generator) buildSecurityGroupConfig(talosConfig *talos.TalosConfig) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":          "control-plane",
			"description":   "Security group for control plane nodes",
			"defaultPolicy": "deny",
			"rules": []map[string]interface{}{
				{
					"direction":   "ingress",
					"protocol":    "tcp",
					"port":        6443,
					"description": "Kubernetes API server",
				},
				{
					"direction":   "ingress",
					"protocol":    "tcp",
					"port":        talosConfig.NetworkConfig.TalosAPIPort,
					"description": "Talos API",
				},
				{
					"direction":   "ingress",
					"protocol":    "tcp",
					"portRange":   "2379-2380",
					"description": "etcd",
				},
			},
		},
		{
			"name":          "worker",
			"description":   "Security group for worker nodes",
			"defaultPolicy": "deny",
			"rules": []map[string]interface{}{
				{
					"direction":   "ingress",
					"protocol":    "tcp",
					"port":        10250,
					"description": "Kubelet",
				},
			},
		},
		{
			"name":          "bastion",
			"description":   "Security group for bastion/WireGuard gateway",
			"defaultPolicy": "deny",
			"rules": []map[string]interface{}{
				{
					"direction":   "ingress",
					"protocol":    "udp",
					"portRange":   fmt.Sprintf("%d-%d", talosConfig.NetworkConfig.WireGuardPort, talosConfig.NetworkConfig.WireGuardPort+1),
					"description": "WireGuard VPN",
				},
			},
		},
	}
}

// buildLoadBalancerConfig creates load balancer configuration.
func (g *generator) buildLoadBalancerConfig(clusterName string, talosConfig *talos.TalosConfig) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        fmt.Sprintf("%s-control-plane-lb", clusterName),
			"description": "Load balancer for control plane API",
			"type":        "octavia", // Can fallback to HAProxy
			"port":        6443,
			"protocol":    "tcp",
			"healthCheck": map[string]interface{}{
				"protocol":           "https",
				"port":               6443,
				"path":               "/healthz",
				"interval":           10,
				"timeout":            5,
				"unhealthyThreshold": 3,
			},
		},
	}
}

// buildCredentialPolicies creates least-privilege credential policies.
func (g *generator) buildCredentialPolicies() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "talos-cluster-admin",
			"description": "Least-privilege policy for Talos cluster management",
			"permissions": []string{
				"compute:create",
				"compute:delete",
				"compute:read",
				"compute:update",
				"network:create",
				"network:delete",
				"network:read",
				"network:update",
				"volume:create",
				"volume:delete",
				"volume:read",
				"volume:update",
				"image:read",
				"loadbalancer:create",
				"loadbalancer:delete",
				"loadbalancer:read",
				"loadbalancer:update",
			},
		},
		{
			"name":        "talos-node",
			"description": "Least-privilege policy for Talos nodes",
			"permissions": []string{
				"compute:read",
				"network:read",
				"volume:read",
				"volume:attach",
				"volume:detach",
				"loadbalancer:read",
			},
		},
	}
}
