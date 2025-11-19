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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func setupServiceTestEnv(t *testing.T, clusterName string) (string, func()) {
	t.Helper()
	cfgDir := t.TempDir()

	// Manually manage environment to avoid t.Setenv issues with subtests
	oldEnv := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir)

	cleanup := func() {
		if oldEnv != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldEnv)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}

	// Create cluster directory structure
	clusterDir := filepath.Join(cfgDir, "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		cleanup()
		t.Fatalf("failed to create cluster directory: %v", err)
	}

	// Create a basic config file
	cfg := config.NewDefault(clusterName)
	cfg.OpenCenter.GitOps.GitDir = filepath.Join(cfgDir, "gitops")
	if err := config.Save(cfg); err != nil {
		cleanup()
		t.Fatalf("failed to save initial config: %v", err)
	}

	// Set active cluster
	if err := config.SetActive(clusterName); err != nil {
		cleanup()
		t.Fatalf("failed to set active cluster: %v", err)
	}

	return cfgDir, cleanup
}

func TestClusterServiceEnable(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		serviceName string
		args        []string
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, cfg *config.Config)
	}{
		{
			name:        "enable simple service",
			clusterName: "test-cluster",
			serviceName: "prometheus",
			args:        []string{"prometheus"},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if _, exists := cfg.OpenCenter.Services["prometheus"]; !exists {
					t.Error("expected prometheus service to exist")
				}
				if !cfg.OpenCenter.Services["prometheus"].Enabled {
					t.Error("expected prometheus service to be enabled")
				}
			},
		},
		{
			name:        "enable managed service",
			clusterName: "test-cluster",
			serviceName: "custom-app",
			args:        []string{"custom-app", "--managed"},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if _, exists := cfg.OpenCenter.ManagedService["custom-app"]; !exists {
					t.Error("expected custom-app managed service to exist")
				}
				if !cfg.OpenCenter.ManagedService["custom-app"].Enabled {
					t.Error("expected custom-app managed service to be enabled")
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
			errorMsg:    "missing required secret 'swift_password'",
			validate:    nil,
		},
		{
			name:        "enable service with explicit cluster flag",
			clusterName: "explicit-test",
			serviceName: "grafana",
			args:        []string{"grafana", "--cluster=explicit-test-enable-service-with-explicit-cluster-flag"},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if _, exists := cfg.OpenCenter.Services["grafana"]; !exists {
					t.Error("expected grafana service to exist")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique cluster name per test to avoid state pollution
			uniqueCluster := tt.clusterName + "-" + strings.ReplaceAll(tt.name, " ", "-")
			_, cleanup := setupServiceTestEnv(t, uniqueCluster)
			defer cleanup()

			// Pre-populate config if testing "already enabled" scenario
			if strings.Contains(tt.name, "already enabled") {
				cfg, err := config.Load(uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(map[string]config.ServiceCfg)
				}
				cfg.OpenCenter.Services[tt.serviceName] = config.ServiceCfg{Enabled: true}
				if err := config.Save(cfg); err != nil {
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
				cfg, err := config.Load(uniqueCluster)
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
		validate    func(t *testing.T, cfg *config.Config)
	}{
		{
			name:        "disable enabled service",
			clusterName: "test-cluster",
			serviceName: "prometheus",
			args:        []string{"prometheus"},
			setupFunc: func(t *testing.T, clusterName string) {
				cfg, err := config.Load(clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(map[string]config.ServiceCfg)
				}
				cfg.OpenCenter.Services["prometheus"] = config.ServiceCfg{Enabled: true}
				if err := config.Save(cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if svc, exists := cfg.OpenCenter.Services["prometheus"]; !exists {
					t.Error("expected prometheus service to still exist")
				} else if svc.Enabled {
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
				cfg, err := config.Load(clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.ManagedService == nil {
					cfg.OpenCenter.ManagedService = make(map[string]config.ServiceCfg)
				}
				cfg.OpenCenter.ManagedService["custom-app"] = config.ServiceCfg{Enabled: true}
				if err := config.Save(cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if svc, exists := cfg.OpenCenter.ManagedService["custom-app"]; !exists {
					t.Error("expected custom-app managed service to still exist")
				} else if svc.Enabled {
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
				cfg, err := config.Load(clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(map[string]config.ServiceCfg)
				}
				cfg.OpenCenter.Services["prometheus"] = config.ServiceCfg{Enabled: false}
				if err := config.Save(cfg); err != nil {
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
				cfg, err := config.Load(clusterName)
				if err != nil {
					t.Fatalf("failed to load config: %v", err)
				}
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(map[string]config.ServiceCfg)
				}
				cfg.OpenCenter.Services["grafana"] = config.ServiceCfg{Enabled: true}
				if err := config.Save(cfg); err != nil {
					t.Fatalf("failed to save config: %v", err)
				}
			},
			expectError: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if svc, exists := cfg.OpenCenter.Services["grafana"]; !exists {
					t.Error("expected grafana service to still exist")
				} else if svc.Enabled {
					t.Error("expected grafana service to be disabled")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique cluster name per test to avoid state pollution
			uniqueCluster := tt.clusterName + "-" + strings.ReplaceAll(tt.name, " ", "-")
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
				cfg, err := config.Load(uniqueCluster)
				if err != nil {
					t.Fatalf("failed to load config for validation: %v", err)
				}
				tt.validate(t, &cfg)
			}
		})
	}
}

func TestClusterServiceNoActiveCluster(t *testing.T) {
	cfgDir := t.TempDir()
	oldEnv := os.Getenv("OPENCENTER_CONFIG_DIR")
	os.Setenv("OPENCENTER_CONFIG_DIR", cfgDir)
	defer func() {
		if oldEnv != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldEnv)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
	}()

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

	if !strings.Contains(err.Error(), "no cluster selected") {
		t.Errorf("expected 'no cluster selected' error, got: %v", err)
	}
}

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
	cfg, err := config.Load(clusterName)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if !cfg.OpenCenter.Services["prometheus"].Enabled {
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
	cfg, err = config.Load(clusterName)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.OpenCenter.Services["prometheus"].Enabled {
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
	cfg, err = config.Load(clusterName)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if !cfg.OpenCenter.Services["prometheus"].Enabled {
		t.Error("expected prometheus to be enabled after re-enable command")
	}
}
