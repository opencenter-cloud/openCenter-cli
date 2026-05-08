// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
)

func setupServiceTestEnv(t *testing.T, clusterName string) (string, func()) {
	t.Helper()
	cfgDir := t.TempDir()

	// Manually manage environment to avoid t.Setenv issues with subtests
	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	oldActiveCluster := os.Getenv("OPENCENTER_CLUSTER")
	oldSessionFile := os.Getenv("OPENCENTER_SESSION_FILE")
	oldSessionID := os.Getenv("OPENCENTER_SESSION_ID")
	oldClustersDir := os.Getenv("OPENCENTER_CLUSTERS_DIR")
	oldGitOpsDir := os.Getenv("OPENCENTER_GITOPS_DIR")
	oldClusterStateDir := os.Getenv("OPENCENTER_CLUSTER_STATE_DIR")
	oldSecretsDir := os.Getenv("OPENCENTER_SECRETS_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir)
	os.Setenv("OPENCENTER_CLUSTERS_DIR", filepath.Join(cfgDir, "clusters"))
	os.Unsetenv("OPENCENTER_GITOPS_DIR")
	os.Unsetenv("OPENCENTER_CLUSTER_STATE_DIR")
	os.Unsetenv("OPENCENTER_SECRETS_DIR")
	os.Unsetenv("OPENCENTER_CLUSTER")
	os.Unsetenv("OPENCENTER_SESSION_FILE")
	os.Unsetenv("OPENCENTER_SESSION_ID")
	resetCommandStateForTests()

	cleanup := func() {
		if oldConfigDir != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
		if oldActiveCluster != "" {
			os.Setenv("OPENCENTER_CLUSTER", oldActiveCluster)
		} else {
			os.Unsetenv("OPENCENTER_CLUSTER")
		}
		if oldSessionFile != "" {
			os.Setenv("OPENCENTER_SESSION_FILE", oldSessionFile)
		} else {
			os.Unsetenv("OPENCENTER_SESSION_FILE")
		}
		if oldSessionID != "" {
			os.Setenv("OPENCENTER_SESSION_ID", oldSessionID)
		} else {
			os.Unsetenv("OPENCENTER_SESSION_ID")
		}
		if oldClustersDir != "" {
			os.Setenv("OPENCENTER_CLUSTERS_DIR", oldClustersDir)
		} else {
			os.Unsetenv("OPENCENTER_CLUSTERS_DIR")
		}
		if oldGitOpsDir != "" {
			os.Setenv("OPENCENTER_GITOPS_DIR", oldGitOpsDir)
		} else {
			os.Unsetenv("OPENCENTER_GITOPS_DIR")
		}
		if oldClusterStateDir != "" {
			os.Setenv("OPENCENTER_CLUSTER_STATE_DIR", oldClusterStateDir)
		} else {
			os.Unsetenv("OPENCENTER_CLUSTER_STATE_DIR")
		}
		if oldSecretsDir != "" {
			os.Setenv("OPENCENTER_SECRETS_DIR", oldSecretsDir)
		} else {
			os.Unsetenv("OPENCENTER_SECRETS_DIR")
		}
		resetCommandStateForTests()
	}

	_, clusterPaths := createClusterDirectoriesForTest(t, cfgDir, clusterName, "opencenter")

	// Create a basic v2 config using the org-based layout expected by PathResolver.
	cfgPtr, err := v2.NewV2Default(clusterName, "openstack")
	if err != nil {
		cleanup()
		t.Fatalf("create native v2 config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = "opencenter"
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir

	ctx := context.Background()
	if err := saveConfig(ctx, cfg); err != nil {
		cleanup()
		t.Fatalf("failed to save initial config: %v", err)
	}

	// Set active cluster
	if err := setActiveCluster(clusterName); err != nil {
		cleanup()
		t.Fatalf("failed to set active cluster: %v", err)
	}

	return cfgDir, cleanup
}

func uniqueServiceTestCluster(base, testName string) string {
	name := base + "-" + strings.ReplaceAll(testName, " ", "-")
	if len(name) > 63 {
		return name[:63]
	}
	return name
}

func TestClusterServiceEnable(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		serviceName string
		args        []string
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, cfg *v2.Config)
	}{
		{
			name:        "enable simple service",
			clusterName: "test-cluster",
			serviceName: "prometheus",
			args:        []string{"prometheus"},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				if svc, exists := cfg.OpenCenter.Services["prometheus"]; !exists {
					t.Error("expected prometheus service to exist")
				} else {
					if !isEnabled(svc) {
						t.Error("expected prometheus service to be enabled")
					}
				}
			},
		},
		{
			name:        "enable managed service",
			clusterName: "test-cluster",
			serviceName: "custom-app",
			args:        []string{"custom-app", "--managed"},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				if svc, exists := cfg.OpenCenter.ManagedServices["custom-app"]; !exists {
					t.Error("expected custom-app managed service to exist")
				} else {
					if !isEnabled(svc) {
						t.Error("expected custom-app managed service to be enabled")
					}
				}
			},
		},
		{
			name:        "enable already enabled service",
			clusterName: "test-cluster",
			serviceName: "prometheus",
			args:        []string{"prometheus"},
			expectError: true,
			errorMsg:    "already enabled",
			validate:    nil,
		},
		{
			name:        "force re-enable already enabled service",
			clusterName: "test-cluster",
			serviceName: "prometheus",
			args:        []string{"prometheus", "--force"},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				if svc, exists := cfg.OpenCenter.Services["prometheus"]; !exists {
					t.Error("expected prometheus service to exist")
				} else {
					if !isEnabled(svc) {
						t.Error("expected prometheus service to be enabled")
					}
				}
			},
		},
		{
			name:        "re-enable disabled service preserves config",
			clusterName: "test-cluster",
			serviceName: "cert-manager",
			args:        []string{"cert-manager"},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				svc, exists := cfg.OpenCenter.Services["cert-manager"]
				if !exists {
					t.Fatal("expected cert-manager service to exist")
				}
				certManager, ok := svc.(*services.CertManagerConfig)
				if !ok {
					t.Fatalf("expected *services.CertManagerConfig, got %T", svc)
				}
				if !certManager.Enabled {
					t.Error("expected cert-manager service to be enabled")
				}
				if certManager.Email != "preserved@example.com" {
					t.Errorf("expected existing email to be preserved, got %q", certManager.Email)
				}
			},
		},
		{
			name:        "enable service without required parameter",
			clusterName: "test-cluster",
			serviceName: "cert-manager",
			args:        []string{"cert-manager"},
			expectError: true,
			errorMsg:    "missing required parameter 'email'",
			validate:    nil,
		},
		{
			name:        "enable service without required secret",
			clusterName: "test-cluster",
			serviceName: "loki",
			args:        []string{"loki"},
			expectError: true,
			errorMsg:    "missing required Swift credentials",
			validate:    nil,
		},
		{
			name:        "enable service with explicit cluster flag",
			clusterName: "explicit-test",
			serviceName: "grafana",
			args:        []string{"grafana", "--cluster=explicit-test-enable-service-with-explicit-cluster-flag"},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				if _, exists := cfg.OpenCenter.Services["grafana"]; !exists {
					t.Error("expected grafana service to exist")
				}
			},
		},
		{
			name:        "enable service with missing dependency",
			clusterName: "test-cluster",
			serviceName: "weave-gitops",
			args:        []string{"weave-gitops"},
			expectError: true,
			errorMsg:    "service 'weave-gitops' requires 'fluxcd' to be enabled",
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique cluster name per test to avoid state pollution.
			uniqueCluster := uniqueServiceTestCluster(tt.clusterName, tt.name)
			_, cleanup := setupServiceTestEnv(t, uniqueCluster)
			defer cleanup()

			// Pre-populate config if testing "already enabled" or "force re-enable" scenario
			if strings.Contains(tt.name, "already enabled") || strings.Contains(tt.name, "force re-enable") {
				cfg, err := loadConfig(context.Background(), uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(v2.ServiceMap)
				}
				cfg.OpenCenter.Services[tt.serviceName] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			}

			if strings.Contains(tt.name, "re-enable disabled service") {
				cfg, err := loadConfig(context.Background(), uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{
					BaseConfig: services.BaseConfig{Enabled: false},
					Email:      "preserved@example.com",
				}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			}

			if strings.Contains(tt.name, "missing dependency") {
				cfg, err := loadConfig(context.Background(), uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				cfg.OpenCenter.Services["fluxcd"] = &services.DefaultServiceConfig{
					BaseConfig: services.BaseConfig{Enabled: false},
				}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			}

			if strings.Contains(tt.name, "without required parameter") {
				cfg, err := loadConfig(context.Background(), uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{
					BaseConfig: services.BaseConfig{Enabled: false},
				}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			}

			if strings.Contains(tt.name, "without required secret") {
				cfg, err := loadConfig(context.Background(), uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				cfg.OpenCenter.Services["loki"] = &services.LokiConfig{
					BaseConfig: services.BaseConfig{Enabled: false},
				}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			}

			cmd := newClusterServiceEnableCmd()
			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(errOut)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none. Output: %s", out.String())
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v\nOutput: %s\nError output: %s", err, out.String(), errOut.String())
				return
			}

			// Validate output message
			output := out.String()
			if !strings.Contains(output, "Successfully enabled") {
				t.Errorf("expected success message in output, got: %s", output)
			}

			// Run custom validation if provided
			if tt.validate != nil {
				cfg, err := loadConfig(context.Background(), uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config for validation: %v", err)
				}
				tt.validate(t, &cfg)
			}
		})
	}
}

func TestClusterServiceDisable(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		serviceName string
		args        []string
		setupFunc   func(t *testing.T, clusterName string)
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, cfg *v2.Config)
	}{
		{
			name:        "disable enabled service",
			clusterName: "test-cluster",
			serviceName: "prometheus",
			args:        []string{"prometheus"},
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(v2.ServiceMap)
				}
				cfg.OpenCenter.Services["prometheus"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				if svc, exists := cfg.OpenCenter.Services["prometheus"]; !exists {
					t.Error("expected prometheus service to still exist")
				} else if isEnabled(svc) {
					t.Error("expected prometheus service to be disabled")
				}
			},
		},
		{
			name:        "disable enabled managed service",
			clusterName: "test-cluster",
			serviceName: "custom-app",
			args:        []string{"custom-app", "--managed"},
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.ManagedServices == nil {
					cfg.OpenCenter.ManagedServices = make(v2.ServiceMap)
				}
				cfg.OpenCenter.ManagedServices["custom-app"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				if svc, exists := cfg.OpenCenter.ManagedServices["custom-app"]; !exists {
					t.Error("expected custom-app managed service to still exist")
				} else if isEnabled(svc) {
					t.Error("expected custom-app managed service to be disabled")
				}
			},
		},
		{
			name:        "disable non-existent service",
			clusterName: "test-cluster",
			serviceName: "nonexistent",
			args:        []string{"nonexistent"},
			setupFunc:   nil,
			expectError: true,
			errorMsg:    "not found",
			validate:    nil,
		},
		{
			name:        "disable already disabled service",
			clusterName: "test-cluster",
			serviceName: "prometheus",
			args:        []string{"prometheus"},
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(v2.ServiceMap)
				}
				cfg.OpenCenter.Services["prometheus"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: true,
			errorMsg:    "already disabled",
			validate:    nil,
		},
		{
			name:        "disable service with explicit cluster flag",
			clusterName: "explicit-test",
			serviceName: "grafana",
			args:        []string{"grafana", "--cluster=explicit-test-disable-service-with-explicit-cluster-flag"},
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(v2.ServiceMap)
				}
				cfg.OpenCenter.Services["grafana"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				if svc, exists := cfg.OpenCenter.Services["grafana"]; !exists {
					t.Error("expected grafana service to still exist")
				} else if isEnabled(svc) {
					t.Error("expected grafana service to be disabled")
				}
			},
		},
		{
			name:        "disable service with enabled dependents",
			clusterName: "test-cluster",
			serviceName: "keycloak",
			args:        []string{"keycloak"},
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				cfg.OpenCenter.Services["headlamp"] = &services.HeadlampConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				}
				cfg.OpenCenter.Services["keycloak"] = &services.KeycloakConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: true,
			errorMsg:    "service 'headlamp' requires 'keycloak' to be enabled",
			validate:    nil,
		},
		{
			name:        "disable service with render removes manifests",
			clusterName: "test-cluster",
			serviceName: "cert-manager",
			args:        []string{"cert-manager", "--render"},
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
					Email:      "render@example.com",
				}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
				if err := gitops.RenderClusterApps(cfg); err != nil {
					t.Fatalf("failed to render cluster apps: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, cfg *v2.Config) {
				if svc, exists := cfg.OpenCenter.Services["cert-manager"]; !exists {
					t.Error("expected cert-manager service to still exist")
				} else if isEnabled(svc) {
					t.Error("expected cert-manager service to be disabled")
				}

				clusterRoot := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName())
				paths := []string{
					filepath.Join(clusterRoot, "services", "cert-manager"),
					filepath.Join(clusterRoot, "services", "sources", "opencenter-cert-manager.yaml"),
					filepath.Join(clusterRoot, "services", "fluxcd", "cert-manager.yaml"),
				}
				for _, path := range paths {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Errorf("expected rendered path %s to be removed, got err=%v", path, err)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique cluster name per test to avoid state pollution.
			uniqueCluster := uniqueServiceTestCluster(tt.clusterName, tt.name)
			_, cleanup := setupServiceTestEnv(t, uniqueCluster)
			defer cleanup()

			// Run setup function if provided
			if tt.setupFunc != nil {
				tt.setupFunc(t, uniqueCluster)
			}

			cmd := newClusterServiceDisableCmd()
			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(errOut)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none. Output: %s", out.String())
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v\nOutput: %s\nError output: %s", err, out.String(), errOut.String())
				return
			}

			// Validate output message
			output := out.String()
			if !strings.Contains(output, "Successfully disabled") {
				t.Errorf("expected success message in output, got: %s", output)
			}

			// Run custom validation if provided
			if tt.validate != nil {
				cfg, err := loadConfig(context.Background(), uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config for validation: %v", err)
				}
				tt.validate(t, &cfg)
			}
		})
	}
}

// broken: full-suite run resolves the integration-test cluster instead of reporting a
// missing active cluster; see docs/test-results.md.
func TestClusterServiceNoActiveCluster(t *testing.T) {
	cfgDir := t.TempDir()
	prepareCommandTestEnv(t, cfgDir)

	// Don't set an active cluster

	cmd := newClusterServiceEnableCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"prometheus"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no cluster is selected")
		return
	}

	if !strings.Contains(err.Error(), "no active cluster is set") {
		t.Errorf("expected missing active cluster error, got: %v", err)
	}
}

// broken: full-suite run resolves the integration-test cluster instead of the per-test
// fixture before enabling services; see docs/test-results.md.
func TestClusterServiceEnableDisableRoundtrip(t *testing.T) {
	clusterName := "roundtrip-cluster"
	_, cleanup := setupServiceTestEnv(t, clusterName)
	defer cleanup()

	// Enable a service
	enableCmd := newClusterServiceEnableCmd()
	enableOut := &bytes.Buffer{}
	enableCmd.SetOut(enableOut)
	enableCmd.SetArgs([]string{"prometheus"})

	if err := enableCmd.Execute(); err != nil {
		t.Fatalf("failed to enable service: %v", err)
	}

	// Verify it's enabled
	cfg, err := loadConfig(context.Background(), clusterName)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if svc, ok := cfg.OpenCenter.Services["prometheus"]; !ok || !isEnabled(svc) {
		t.Error("expected prometheus to be enabled after enable command")
	}

	// Disable the service
	disableCmd := newClusterServiceDisableCmd()
	disableOut := &bytes.Buffer{}
	disableCmd.SetOut(disableOut)
	disableCmd.SetArgs([]string{"prometheus"})

	if err := disableCmd.Execute(); err != nil {
		t.Fatalf("failed to disable service: %v", err)
	}

	// Verify it's disabled
	cfg, err = loadConfig(context.Background(), clusterName)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if svc, ok := cfg.OpenCenter.Services["prometheus"]; ok && isEnabled(svc) {
		t.Error("expected prometheus to be disabled after disable command")
	}

	// Re-enable the service
	enableCmd2 := newClusterServiceEnableCmd()
	enableOut2 := &bytes.Buffer{}
	enableCmd2.SetOut(enableOut2)
	enableCmd2.SetArgs([]string{"prometheus"})

	if err := enableCmd2.Execute(); err != nil {
		t.Fatalf("failed to re-enable service: %v", err)
	}

	// Verify it's enabled again
	cfg, err = loadConfig(context.Background(), clusterName)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if svc, ok := cfg.OpenCenter.Services["prometheus"]; !ok || !isEnabled(svc) {
		t.Error("expected prometheus to be enabled after re-enable command")
	}
}

// broken: full-suite run resolves the integration-test cluster instead of the per-subtest
// fixture; failing subtests are tagged inline below and summarized in docs/test-results.md.
func TestClusterServiceStatus(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		setupFunc   func(t *testing.T, clusterName string)
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, output string)
	}{
		{
			// broken: full-suite run resolves integration-test instead of this subtest fixture.
			name:        "display status with no services",
			clusterName: "test-cluster",
			setupFunc:   nil,
			expectError: false,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "SERVICE NAME") {
					t.Error("expected header in output")
				}
				if !strings.Contains(output, "ENABLED") {
					t.Error("expected ENABLED column in output")
				}
				if !strings.Contains(output, "STATUS") {
					t.Error("expected STATUS column in output")
				}
			},
		},
		{
			// broken: full-suite run resolves integration-test instead of this subtest fixture.
			name:        "display status with enabled services",
			clusterName: "test-cluster",
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(v2.ServiceMap)
				}
				cfg.OpenCenter.Services["prometheus"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Status: "running"}}
				cfg.OpenCenter.Services["grafana"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false, Status: "pending"}}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "prometheus") {
					t.Error("expected prometheus in output")
				}
				if !strings.Contains(output, "grafana") {
					t.Error("expected grafana in output")
				}
				if !strings.Contains(output, "enabled") {
					t.Error("expected 'enabled' in output")
				}
				if !strings.Contains(output, "disabled") {
					t.Error("expected 'disabled' in output")
				}
				if !strings.Contains(output, "running") {
					t.Error("expected 'running' status in output")
				}
			},
		},
		{
			// broken: full-suite run resolves integration-test instead of this subtest fixture.
			name:        "display status with managed services",
			clusterName: "test-cluster",
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.ManagedServices == nil {
					cfg.OpenCenter.ManagedServices = make(v2.ServiceMap)
				}
				cfg.OpenCenter.ManagedServices["custom-app"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Status: "success"}}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "custom-app") {
					t.Error("expected custom-app in output")
				}
				if !strings.Contains(output, "managed") {
					t.Error("expected '(managed)' label in output")
				}
				if !strings.Contains(output, "success") {
					t.Error("expected 'success' status in output")
				}
			},
		},
		{
			// broken: full-suite run resolves integration-test instead of this subtest fixture.
			name:        "display status with empty status field",
			clusterName: "test-cluster",
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := loadConfig(context.Background(), clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(v2.ServiceMap)
				}
				cfg.OpenCenter.Services["loki"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Status: ""}}
				if err := saveConfig(context.Background(), cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "loki") {
					t.Error("expected loki in output")
				}
				// Should show "-" for empty status
				lines := strings.Split(output, "\n")
				found := false
				for _, line := range lines {
					if strings.Contains(line, "loki") {
						if strings.Contains(line, "-") {
							found = true
							break
						}
					}
				}
				if !found {
					t.Error("expected '-' for empty status field")
				}
			},
		},
		{
			// broken: full-suite run resolves integration-test instead of surfacing no active cluster.
			name:        "no active cluster",
			clusterName: "",
			setupFunc:   nil,
			expectError: true,
			errorMsg:    "no active cluster is set",
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cleanup func()
			if tt.clusterName != "" {
				uniqueCluster := tt.clusterName + "-" + strings.ReplaceAll(tt.name, " ", "-")
				_, cleanup = setupServiceTestEnv(t, uniqueCluster)
				defer cleanup()

				if tt.setupFunc != nil {
					tt.setupFunc(t, uniqueCluster)
				}
			} else {
				// Test case with no active cluster
				cfgDir := t.TempDir()
				oldEnv := os.Getenv("OPENCENTER_CONFIG_DIR")
				oldActiveCluster := os.Getenv("OPENCENTER_CLUSTER")
				oldSessionFile := os.Getenv("OPENCENTER_SESSION_FILE")
				oldSessionID := os.Getenv("OPENCENTER_SESSION_ID")
				oldClustersDir := os.Getenv("OPENCENTER_CLUSTERS_DIR")
				oldGitOpsDir := os.Getenv("OPENCENTER_GITOPS_DIR")
				oldClusterStateDir := os.Getenv("OPENCENTER_CLUSTER_STATE_DIR")
				oldSecretsDir := os.Getenv("OPENCENTER_SECRETS_DIR")
				os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir)
				os.Setenv("OPENCENTER_CLUSTERS_DIR", filepath.Join(cfgDir, "clusters"))
				os.Unsetenv("OPENCENTER_GITOPS_DIR")
				os.Unsetenv("OPENCENTER_CLUSTER_STATE_DIR")
				os.Unsetenv("OPENCENTER_SECRETS_DIR")
				os.Unsetenv("OPENCENTER_CLUSTER")
				os.Unsetenv("OPENCENTER_SESSION_FILE")
				os.Unsetenv("OPENCENTER_SESSION_ID")
				resetCommandStateForTests()
				cleanup = func() {
					if oldEnv != "" {
						os.Setenv("OPENCENTER_CONFIG_DIR", oldEnv)
					} else {
						os.Unsetenv("OPENCENTER_CONFIG_DIR")
					}
					if oldActiveCluster != "" {
						os.Setenv("OPENCENTER_CLUSTER", oldActiveCluster)
					} else {
						os.Unsetenv("OPENCENTER_CLUSTER")
					}
					if oldSessionFile != "" {
						os.Setenv("OPENCENTER_SESSION_FILE", oldSessionFile)
					} else {
						os.Unsetenv("OPENCENTER_SESSION_FILE")
					}
					if oldSessionID != "" {
						os.Setenv("OPENCENTER_SESSION_ID", oldSessionID)
					} else {
						os.Unsetenv("OPENCENTER_SESSION_ID")
					}
					if oldClustersDir != "" {
						os.Setenv("OPENCENTER_CLUSTERS_DIR", oldClustersDir)
					} else {
						os.Unsetenv("OPENCENTER_CLUSTERS_DIR")
					}
					if oldGitOpsDir != "" {
						os.Setenv("OPENCENTER_GITOPS_DIR", oldGitOpsDir)
					} else {
						os.Unsetenv("OPENCENTER_GITOPS_DIR")
					}
					if oldClusterStateDir != "" {
						os.Setenv("OPENCENTER_CLUSTER_STATE_DIR", oldClusterStateDir)
					} else {
						os.Unsetenv("OPENCENTER_CLUSTER_STATE_DIR")
					}
					if oldSecretsDir != "" {
						os.Setenv("OPENCENTER_SECRETS_DIR", oldSecretsDir)
					} else {
						os.Unsetenv("OPENCENTER_SECRETS_DIR")
					}
					resetCommandStateForTests()
				}
				defer cleanup()
			}

			cmd := newClusterServiceStatusCmd()
			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(errOut)

			err := cmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none. Output: %s", out.String())
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v\nOutput: %s\nError output: %s", err, out.String(), errOut.String())
				return
			}

			if tt.validate != nil {
				tt.validate(t, out.String())
			}
		})
	}
}
