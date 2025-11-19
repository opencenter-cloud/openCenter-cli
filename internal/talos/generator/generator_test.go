package generator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
	"gopkg.in/yaml.v3"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)
	if g == nil {
		t.Fatal("New() returned nil")
	}
}

func TestGenerateClusterConfig(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)
	artifacts, err := g.GenerateClusterConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("GenerateClusterConfig() error = %v", err)
	}

	if artifacts == nil {
		t.Fatal("GenerateClusterConfig() returned nil artifacts")
	}

	// Verify machine configs were generated
	if len(artifacts.TalosMachineConfigs) == 0 {
		t.Error("No machine configs generated")
	}

	// Verify expected node types
	expectedTypes := []talos.NodeType{
		talos.NodeTypeControlPlane,
		talos.NodeTypeWorker,
		talos.NodeTypeBastion,
	}

	for _, nodeType := range expectedTypes {
		if _, exists := artifacts.TalosMachineConfigs[nodeType]; !exists {
			t.Errorf("Missing machine config for node type: %s", nodeType)
		}
	}

	// Verify other artifacts
	if artifacts.PulumiStack == nil {
		t.Error("Pulumi stack not generated")
	}

	if artifacts.WireGuardConfig == nil {
		t.Error("WireGuard config not generated")
	}

	if artifacts.NetworkTopology == nil {
		t.Error("Network topology not generated")
	}

	if len(artifacts.SecurityGroups) == 0 {
		t.Error("Security groups not generated")
	}

	if artifacts.SOPSConfig == nil {
		t.Error("SOPS config not generated")
	}
}

func TestGenerateClusterConfig_NilConfig(t *testing.T) {
	g := New(&config.Config{})
	_, err := g.GenerateClusterConfig(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil config, got nil")
	}
}

func TestGenerateTalosMachineConfig(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)

	tests := []struct {
		name     string
		nodeType talos.NodeType
	}{
		{"control-plane", talos.NodeTypeControlPlane},
		{"worker", talos.NodeTypeWorker},
		{"bastion", talos.NodeTypeBastion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machineConfig, err := g.GenerateTalosMachineConfig(context.Background(), tt.nodeType)
			if err != nil {
				t.Fatalf("GenerateTalosMachineConfig() error = %v", err)
			}

			if len(machineConfig) == 0 {
				t.Error("Generated machine config is empty")
			}

			// Verify it's valid YAML
			var parsed map[string]interface{}
			if err := yaml.Unmarshal(machineConfig, &parsed); err != nil {
				t.Errorf("Generated machine config is not valid YAML: %v", err)
			}
		})
	}
}

func TestGeneratePulumiStack(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)
	stackConfig, err := g.GeneratePulumiStack(context.Background(), cfg)
	if err != nil {
		t.Fatalf("GeneratePulumiStack() error = %v", err)
	}

	if len(stackConfig) == 0 {
		t.Error("Generated Pulumi stack config is empty")
	}

	// Verify it's valid YAML
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(stackConfig, &parsed); err != nil {
		t.Errorf("Generated Pulumi stack config is not valid YAML: %v", err)
	}

	// Verify required fields
	if _, ok := parsed["name"]; !ok {
		t.Error("Pulumi stack config missing 'name' field")
	}

	if _, ok := parsed["config"]; !ok {
		t.Error("Pulumi stack config missing 'config' field")
	}
}

func TestGenerateWireGuardConfig(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)
	wgConfig, err := g.GenerateWireGuardConfig(context.Background())
	if err != nil {
		t.Fatalf("GenerateWireGuardConfig() error = %v", err)
	}

	if wgConfig == nil {
		t.Fatal("Generated WireGuard config is nil")
	}

	if wgConfig.ServerPublicKey == "" {
		t.Error("WireGuard config missing server public key")
	}

	if wgConfig.ServerPrivateKey == "" {
		t.Error("WireGuard config missing server private key")
	}

	if wgConfig.ServerAddress == "" {
		t.Error("WireGuard config missing server address")
	}

	if wgConfig.ServerPort == 0 {
		t.Error("WireGuard config has invalid server port")
	}

	if len(wgConfig.Peers) == 0 {
		t.Error("WireGuard config has no peers")
	}
}

func TestGenerateNetworkTopology(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)
	topology, err := g.GenerateNetworkTopology(context.Background(), cfg)
	if err != nil {
		t.Fatalf("GenerateNetworkTopology() error = %v", err)
	}

	if topology == nil {
		t.Fatal("Generated network topology is nil")
	}

	// Verify networks
	if topology.ManagementNetwork.Name == "" {
		t.Error("Management network missing name")
	}

	if topology.ControlNetwork.Name == "" {
		t.Error("Control network missing name")
	}

	if topology.DataNetwork.Name == "" {
		t.Error("Data network missing name")
	}

	// Verify router
	if topology.Router.Name == "" {
		t.Error("Router missing name")
	}

	// Verify routes
	if len(topology.Routes) == 0 {
		t.Error("No routes configured")
	}
}

func TestGenerateSecurityGroups(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)
	securityGroups, err := g.GenerateSecurityGroups(context.Background(), cfg)
	if err != nil {
		t.Fatalf("GenerateSecurityGroups() error = %v", err)
	}

	if len(securityGroups) == 0 {
		t.Fatal("No security groups generated")
	}

	// Verify each security group has required fields
	for _, sg := range securityGroups {
		if sg.Name == "" {
			t.Error("Security group missing name")
		}

		if sg.Description == "" {
			t.Error("Security group missing description")
		}

		if len(sg.Rules) == 0 {
			t.Errorf("Security group %s has no rules", sg.Name)
		}
	}
}

func TestGenerateGitOpsStructure(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "gitops-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)
	if err := g.GenerateGitOpsStructure(context.Background(), tmpDir); err != nil {
		t.Fatalf("GenerateGitOpsStructure() error = %v", err)
	}

	// Verify key directories exist
	requiredDirs := []string{
		"clusters",
		"infrastructure",
		"infrastructure/talos",
		"applications",
	}

	for _, dir := range requiredDirs {
		fullPath := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Required directory %s does not exist", dir)
		}
	}

	// Verify SOPS config exists
	sopsPath := filepath.Join(tmpDir, ".sops.yaml")
	if _, err := os.Stat(sopsPath); os.IsNotExist(err) {
		t.Error("SOPS config file does not exist")
	}
}

func TestGenerateGitOpsStructure_EmptyPath(t *testing.T) {
	cfg := &config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name: "test-cluster",
			},
		},
	}

	g := New(cfg)
	err := g.GenerateGitOpsStructure(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty path, got nil")
	}
}
