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

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
	"github.com/rackerlabs/opencenter-cli/internal/di"
)

// TestClusterBootstrapIntegration tests the cluster bootstrap command with DI container
// Note: This test verifies DI container setup and option parsing.
// Full end-to-end tests require complete path resolution migration (Phase 3).
func TestClusterBootstrapIntegration(t *testing.T) {
	t.Skip("Skipping full end-to-end test until path resolution migration is complete (Phase 3)")
}

// TestClusterBootstrapWithDIContainer tests that the DI container is properly set up
func TestClusterBootstrapWithDIContainer(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create DI container
	container := di.NewContainer()
	if err := setupBootstrapContainer(container); err != nil {
		t.Fatalf("failed to setup container: %v", err)
	}

	// Verify all services can be resolved
	var pathResolver *paths.PathResolver
	if err := container.ResolveAs("path-resolver", &pathResolver); err != nil {
		t.Errorf("failed to resolve path-resolver: %v", err)
	}
	if pathResolver == nil {
		t.Error("path-resolver is nil")
	}

	var validationEngine *validation.ValidationEngine
	if err := container.ResolveAs("validation-engine", &validationEngine); err != nil {
		t.Errorf("failed to resolve validation-engine: %v", err)
	}
	if validationEngine == nil {
		t.Error("validation-engine is nil")
	}

	var bootstrapService *cluster.BootstrapService
	if err := container.ResolveAs("bootstrap-service", &bootstrapService); err != nil {
		t.Errorf("failed to resolve bootstrap-service: %v", err)
	}
	if bootstrapService == nil {
		t.Error("bootstrap-service is nil")
	}
}

// TestClusterBootstrapServiceIntegration tests the BootstrapService directly
func TestClusterBootstrapServiceIntegration(t *testing.T) {
	// Set up temporary config directory
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create dependencies
	pathResolver := paths.NewPathResolver(filepath.Join(dir, "clusters"))
	validationEngine := validation.NewValidationEngine()

	// Register validators
	if err := validationEngine.Register(validators.NewClusterNameValidator()); err != nil {
		t.Fatalf("failed to register cluster name validator: %v", err)
	}
	if err := validationEngine.Register(validators.NewConfigValidator()); err != nil {
		t.Fatalf("failed to register config validator: %v", err)
	}

	// Create BootstrapService
	bootstrapService := cluster.NewBootstrapService(pathResolver, validationEngine)

	// Create a test cluster configuration
	clusterName := "test-service-cluster"
	organization := "opencenter"

	// Create cluster directory structure following org-based strategy
	orgDir := filepath.Join(dir, "clusters", organization)
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("failed to create cluster directory: %v", err)
	}

	// Create a minimal config file in the cluster directory
	configPath := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")
	configContent := `opencenter:
  meta:
    organization: ` + organization + `
    schema_version: "2.0"
  cluster:
    cluster_name: ` + clusterName + `
  infrastructure:
    provider: kind
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Test bootstrap with dry-run
	opts := cluster.BootstrapOptions{
		ClusterName:      clusterName,
		Organization:     organization,
		DryRun:           true,
		SkipValidation:   true, // Skip validation for faster test
		ContainerRuntime: "docker",
	}

	result, err := bootstrapService.Bootstrap(context.Background(), opts)
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("result is nil")
	}

	// In dry-run mode, infrastructure should not be provisioned
	if result.InfrastructureProvisioned {
		t.Error("infrastructure should not be provisioned in dry-run mode")
	}
	if result.ClusterDeployed {
		t.Error("cluster should not be deployed in dry-run mode")
	}
	if result.ClusterReady {
		t.Error("cluster should not be ready in dry-run mode")
	}
}

// TestClusterBootstrapOptions tests the parseBootstrapOptions function
func TestClusterBootstrapOptions(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		clusterName string
		expectError bool
		checkFunc   func(*testing.T, cluster.BootstrapOptions)
	}{
		{
			name:        "basic options",
			args:        []string{},
			clusterName: "test-cluster",
			expectError: false,
			checkFunc: func(t *testing.T, opts cluster.BootstrapOptions) {
				if opts.ClusterName != "test-cluster" {
					t.Errorf("expected cluster name 'test-cluster', got %q", opts.ClusterName)
				}
				if opts.DryRun {
					t.Error("expected dry-run to be false")
				}
			},
		},
		{
			name:        "dry-run option",
			args:        []string{"--dry-run"},
			clusterName: "test-cluster",
			expectError: false,
			checkFunc: func(t *testing.T, opts cluster.BootstrapOptions) {
				if !opts.DryRun {
					t.Error("expected dry-run to be true")
				}
			},
		},
		{
			name:        "container runtime option",
			args:        []string{"--container-runtime", "podman"},
			clusterName: "test-cluster",
			expectError: false,
			checkFunc: func(t *testing.T, opts cluster.BootstrapOptions) {
				if opts.ContainerRuntime != "podman" {
					t.Errorf("expected container runtime 'podman', got %q", opts.ContainerRuntime)
				}
			},
		},
		{
			name:        "restart option",
			args:        []string{"--restart"},
			clusterName: "test-cluster",
			expectError: false,
			checkFunc: func(t *testing.T, opts cluster.BootstrapOptions) {
				if !opts.Restart {
					t.Error("expected restart to be true")
				}
			},
		},
		{
			name:        "step option",
			args:        []string{"--step", "terraform-init"},
			clusterName: "test-cluster",
			expectError: false,
			checkFunc: func(t *testing.T, opts cluster.BootstrapOptions) {
				if opts.OnlyStep != "terraform-init" {
					t.Errorf("expected only-step 'terraform-init', got %q", opts.OnlyStep)
				}
			},
		},
		{
			name:        "from-step option",
			args:        []string{"--from-step", "terraform-apply"},
			clusterName: "test-cluster",
			expectError: false,
			checkFunc: func(t *testing.T, opts cluster.BootstrapOptions) {
				if opts.FromStep != "terraform-apply" {
					t.Errorf("expected from-step 'terraform-apply', got %q", opts.FromStep)
				}
			},
		},
		{
			name:        "mutually exclusive step and from-step",
			args:        []string{"--step", "terraform-init", "--from-step", "terraform-apply"},
			clusterName: "test-cluster",
			expectError: true,
			checkFunc:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := newClusterBootstrapCmd()
			cmd.SetArgs(tt.args)

			// Parse flags
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			// Parse options
			opts, err := parseBootstrapOptions(cmd, []string{}, tt.clusterName)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Run check function if provided
			if tt.checkFunc != nil {
				tt.checkFunc(t, opts)
			}
		})
	}
}

// TestClusterBootstrapWithExistingCluster tests bootstrap with an existing cluster
// Note: This test is skipped until path resolution migration is complete (Phase 3).
func TestClusterBootstrapWithExistingCluster(t *testing.T) {
	t.Skip("Skipping full end-to-end test until path resolution migration is complete (Phase 3)")
}
