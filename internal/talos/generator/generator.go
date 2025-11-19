package generator

import (
	"context"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// generator implements the Generator interface.
type generator struct {
	config *config.Config
}

// New creates a new Generator instance.
func New(cfg *config.Config) talos.Generator {
	return &generator{
		config: cfg,
	}
}

// GenerateClusterConfig creates complete cluster configuration.
func (g *generator) GenerateClusterConfig(ctx context.Context, cfg *config.Config) (*talos.ClusterArtifacts, error) {
	if cfg == nil {
		return nil, talos.NewConfigurationError(
			"INVALID_CONFIG",
			"configuration cannot be nil",
			nil,
		)
	}

	artifacts := &talos.ClusterArtifacts{
		TalosMachineConfigs: make(map[talos.NodeType][]byte),
		GitOpsStructure:     make(map[string][]byte),
	}

	// Generate machine configs for each node type
	nodeTypes := []talos.NodeType{
		talos.NodeTypeControlPlane,
		talos.NodeTypeWorker,
		talos.NodeTypeBastion,
	}

	for _, nodeType := range nodeTypes {
		machineConfig, err := g.GenerateTalosMachineConfig(ctx, nodeType)
		if err != nil {
			return nil, fmt.Errorf("failed to generate machine config for %s: %w", nodeType, err)
		}
		artifacts.TalosMachineConfigs[nodeType] = machineConfig
	}

	// Generate Pulumi stack configuration
	pulumiStack, err := g.GeneratePulumiStack(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Pulumi stack: %w", err)
	}
	artifacts.PulumiStack = pulumiStack

	// Generate WireGuard configuration
	wireGuardConfig, err := g.GenerateWireGuardConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate WireGuard config: %w", err)
	}
	artifacts.WireGuardConfig = wireGuardConfig

	// Generate network topology
	networkTopology, err := g.GenerateNetworkTopology(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate network topology: %w", err)
	}
	artifacts.NetworkTopology = networkTopology

	// Generate security groups
	securityGroups, err := g.GenerateSecurityGroups(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate security groups: %w", err)
	}
	artifacts.SecurityGroups = securityGroups

	// Generate SOPS configuration
	sopsConfig, err := g.generateSOPSConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SOPS config: %w", err)
	}
	artifacts.SOPSConfig = sopsConfig

	return artifacts, nil
}

// GenerateTalosMachineConfig is implemented in machine_config.go

// GeneratePulumiStack is implemented in pulumi_stack.go

// GenerateWireGuardConfig is implemented in wireguard.go

// GenerateNetworkTopology is implemented in network_topology.go

// GenerateSecurityGroups is implemented in security_groups.go

// GenerateGitOpsStructure is implemented in gitops_structure.go

// generateSOPSConfig creates SOPS encryption configuration.
func (g *generator) generateSOPSConfig(ctx context.Context, cfg *config.Config) ([]byte, error) {
	// Basic SOPS configuration template
	sopsConfig := `creation_rules:
  - path_regex: .*\.(yaml|yml|json)$
    encrypted_regex: ^(data|stringData|password|token|key|secret)$
    barbican: true
`
	return []byte(sopsConfig), nil
}
