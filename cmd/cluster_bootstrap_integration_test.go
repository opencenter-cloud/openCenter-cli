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
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	configdefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
)

// TestClusterBootstrapIntegration tests the cluster bootstrap command with DI container
// broken: full-suite run fails on generated GitOps source contracts (repo casing, ref strategy,
// sync interval, and cert-manager kustomization indentation); see docs/test-results.md.
func TestClusterBootstrapIntegration(t *testing.T) {
	dir, stateDir, clusterDir := prepareKindBootstrapFixture(t, "kind-bootstrap-int")

	cmd := newClusterBootstrapCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"kind-bootstrap-int"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("bootstrap command failed: %v\nstderr: %s", err, stderr.String())
	}

	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig.yaml")
	if _, err := os.Stat(kubeconfigPath); err != nil {
		t.Fatalf("expected kubeconfig at cluster-owned path: %v", err)
	}

	stateRoot := filepath.Join(dir, ".local", "state", "opencenter")
	logDir := filepath.Join(stateRoot, "logs", "bootstrap", "opencenter", "kind-bootstrap-int")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("expected bootstrap log directory at %s: %v", logDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one bootstrap log in %s", logDir)
	}

	statePath := filepath.Join(stateRoot, "bootstrap", "opencenter", "kind-bootstrap-int", "state.json")
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("expected bootstrap state cleanup at %s, got err=%v", statePath, err)
	}

	kindLog, err := os.ReadFile(filepath.Join(stateDir, "kind.log"))
	if err != nil {
		t.Fatalf("read fake kind log: %v", err)
	}
	if !strings.Contains(string(kindLog), "kind create cluster --name kind-bootstrap-int --config "+filepath.Join(clusterDir, "kind-config.yaml")) {
		t.Fatalf("expected create cluster invocation in log\nlog:\n%s", string(kindLog))
	}
	if !strings.Contains(string(kindLog), "kind export kubeconfig --name kind-bootstrap-int --kubeconfig "+kubeconfigPath) {
		t.Fatalf("expected export kubeconfig invocation in log\nlog:\n%s", string(kindLog))
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".kind-bootstrap-int-config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "stage: bootstrap") || !strings.Contains(string(data), "status: success") {
		t.Fatalf("expected bootstrap success lifecycle state\nconfig:\n%s", string(data))
	}
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

	// Create a schema-valid native v2 config file at the org root path expected by PathResolver.
	configPath := filepath.Join(orgDir, "."+clusterName+"-config.yaml")
	cfgPtr, err := v2.NewV2Default(clusterName, "kind")
	if err != nil {
		t.Fatalf("create v2 config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.GitDir = filepath.Join(dir, "clusters", organization)

	loader := v2.NewConfigLoader(configdefaults.NewRegistry())
	if err := loader.SaveToFile(&cfg, configPath); err != nil {
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
// broken: full-suite run fails on generated GitOps source contracts (repo casing, ref strategy,
// sync interval, and cert-manager kustomization indentation); see docs/test-results.md.
func TestClusterBootstrapWithExistingCluster(t *testing.T) {
	_, stateDir, _ := prepareKindBootstrapFixture(t, "kind-existing-int")

	firstRun := newClusterBootstrapCmd()
	firstRun.SetOut(&bytes.Buffer{})
	firstRun.SetErr(&bytes.Buffer{})
	firstRun.SetArgs([]string{"kind-existing-int"})
	if err := firstRun.Execute(); err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(stateDir, "kind.log"), nil, 0o644); err != nil {
		t.Fatalf("reset fake kind log: %v", err)
	}
	resetCommandStateForTests()

	secondRun := newClusterBootstrapCmd()
	var stdout, stderr bytes.Buffer
	secondRun.SetOut(&stdout)
	secondRun.SetErr(&stderr)
	secondRun.SetArgs([]string{"kind-existing-int", "--restart"})
	if err := secondRun.Execute(); err != nil {
		t.Fatalf("second bootstrap failed: %v\nstderr: %s", err, stderr.String())
	}

	kindLog, err := os.ReadFile(filepath.Join(stateDir, "kind.log"))
	if err != nil {
		t.Fatalf("read fake kind log: %v", err)
	}
	logText := string(kindLog)
	if strings.Contains(logText, "kind create cluster") {
		t.Fatalf("expected rerun bootstrap to skip cluster creation\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "kind get clusters") {
		t.Fatalf("expected rerun bootstrap to check for existing clusters\nlog:\n%s", logText)
	}
	if !strings.Contains(logText, "kind export kubeconfig") {
		t.Fatalf("expected rerun bootstrap to export kubeconfig\nlog:\n%s", logText)
	}
}

func TestKindLifecycleSmoke(t *testing.T) {
	if os.Getenv("OPENCENTER_RUN_KIND_SMOKE") == "" {
		t.Skip("set OPENCENTER_RUN_KIND_SMOKE=1 to run the real Kind lifecycle smoke test")
	}
	for _, bin := range []string{"kind", "kubectl", "git"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}

	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	clusterName := "kind-smoke-int"

	initCmd := newClusterInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{clusterName, "--type", "kind", "--no-keygen"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v", err)
	}

	resetCommandStateForTests()

	setupCmd := newClusterSetupCmd()
	setupCmd.SetOut(&bytes.Buffer{})
	setupCmd.SetErr(&bytes.Buffer{})
	setupCmd.SetArgs([]string{clusterName})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("cluster setup failed: %v", err)
	}

	resetCommandStateForTests()

	bootstrapCmd := newClusterBootstrapCmd()
	bootstrapCmd.SetOut(&bytes.Buffer{})
	bootstrapCmd.SetErr(&bytes.Buffer{})
	bootstrapCmd.SetArgs([]string{clusterName})
	if err := bootstrapCmd.Execute(); err != nil {
		t.Fatalf("cluster bootstrap failed: %v", err)
	}

	resetCommandStateForTests()

	statusCmd := newClusterStatusCmd()
	var statusOut bytes.Buffer
	statusCmd.SetOut(&statusOut)
	statusCmd.SetErr(&bytes.Buffer{})
	statusCmd.SetArgs([]string{clusterName})
	if err := statusCmd.Execute(); err != nil {
		t.Fatalf("cluster status failed: %v", err)
	}
	if !strings.Contains(statusOut.String(), "Kind Status:") {
		t.Fatalf("expected kind status output\noutput:\n%s", statusOut.String())
	}

	resetCommandStateForTests()
	t.Setenv("OPENCENTER_TEST_MODE", "1")

	destroyCmd := newClusterDestroyCmd()
	destroyCmd.SetOut(&bytes.Buffer{})
	destroyCmd.SetErr(&bytes.Buffer{})
	destroyCmd.SetArgs([]string{clusterName, "--force"})
	if err := destroyCmd.Execute(); err != nil {
		t.Fatalf("cluster destroy failed: %v", err)
	}
}

func prepareKindBootstrapFixture(t *testing.T, clusterName string) (string, string, string) {
	t.Helper()

	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeGitBinary(t, binDir)
	installFakeKindBinary(t, binDir)
	installFakeKubectlBinary(t, binDir)
	prependTestPath(t, binDir)

	initCmd := newClusterInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{clusterName, "--type", "kind", "--no-keygen"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v", err)
	}

	resetCommandStateForTests()

	setupCmd := newClusterSetupCmd()
	setupCmd.SetOut(&bytes.Buffer{})
	setupCmd.SetErr(&bytes.Buffer{})
	setupCmd.SetArgs([]string{clusterName})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("cluster setup failed: %v", err)
	}

	resetCommandStateForTests()

	clusterDir := filepath.Join(dir, "clusters", "opencenter", "infrastructure", "clusters", clusterName)
	return dir, stateDir, clusterDir
}
