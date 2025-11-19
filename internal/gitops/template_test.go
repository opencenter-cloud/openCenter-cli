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

package gitops

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func TestRenderInfrastructureClusterWithDefaults(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("default-test")
	cfg.OpenCenter.Cluster.ClusterName = "default-test"
	cfg.OpenCenter.GitOps.GitDir = dst

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)

	// Test default values from main-stage.tf
	expectedDefaults := map[string]string{
		"subnet_nodes":                       "10.2.184.0/22",
		"worker_count":                       "2",
		"master_count":                       "3",
		"kubernetes_version":                 "1.31.4",
		"flavor_master":                      "gp.0.4.4",
		"flavor_worker":                      "gp.0.4.8",
		"worker_node_bfv_volume_size":        "40",
		"worker_node_bfv_volume_type":        "HA-Standard",
		"kubelet_rotate_server_certificates": "true",
		"dns_nameservers":                    `["8.8.8.8", "8.8.4.4"]`,
		"node_worker":                        "-wn",
		"node_master":                        "-cp",
	}

	for key, expected := range expectedDefaults {
		if !strings.Contains(content, expected) {
			t.Errorf("rendered main.tf missing expected default %s = %s\ncontent:\n%s", key, expected, content)
		}
	}
}

func TestRenderInfrastructureClusterConditionalOIDC(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("oidc-test")
	cfg.OpenCenter.Cluster.ClusterName = "oidc-test"
	cfg.OpenCenter.GitOps.GitDir = dst

	// Enable OIDC
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.Enabled = true
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCURL = "https://auth.example.com/realms/test"
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCClientID = "test-client"

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)

	// Test OIDC configuration is included when enabled
	oidcSettings := []string{
		"kube_oidc_auth_enabled",
		"kube_oidc_url",
		"kube_oidc_client_id",
		"kube_oidc_ca_file",
		"kube_oidc_username_claim",
		"kube_oidc_username_prefix",
		"kube_oidc_groups_claim",
		"kube_oidc_groups_prefix",
	}

	for _, setting := range oidcSettings {
		if !strings.Contains(content, setting) {
			t.Errorf("rendered main.tf missing OIDC setting %s when OIDC is enabled\ncontent:\n%s", setting, content)
		}
	}
}

func TestRenderInfrastructureClusterConditionalOIDCDisabled(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("no-oidc-test")
	cfg.OpenCenter.Cluster.ClusterName = "no-oidc-test"
	cfg.OpenCenter.GitOps.GitDir = dst

	// Explicitly disable OIDC
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.Enabled = false

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)

	// Test OIDC configuration is not included when disabled
	oidcSettings := []string{
		"kube_oidc_auth_enabled",
		"kube_oidc_url",
		"kube_oidc_client_id",
		"kube_oidc_ca_file",
		"kube_oidc_username_claim",
		"kube_oidc_username_prefix",
		"kube_oidc_groups_claim",
		"kube_oidc_groups_prefix",
	}

	for _, setting := range oidcSettings {
		if strings.Contains(content, setting) {
			t.Errorf("rendered main.tf should not contain OIDC setting %s when OIDC is disabled", setting)
		}
	}
}

func TestRenderInfrastructureClusterConditionalWindows(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("windows-test")
	cfg.OpenCenter.Cluster.ClusterName = "windows-test"
	cfg.OpenCenter.GitOps.GitDir = dst

	// Enable Windows workers
	cfg.OpenCenter.Cluster.Kubernetes.WorkerCountWindows = 2
	cfg.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled = true
	cfg.OpenCenter.Cluster.Kubernetes.WindowsWorkers.WindowsUser = "Administrator"
	cfg.OpenCenter.Cluster.Kubernetes.WindowsWorkers.WindowsAdminPassword = "SecretPassword123!"

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)

	// Test Windows configuration is included when enabled
	windowsSettings := []string{
		"size_worker_windows",
		"windows_admin_password",
		"worker_node_bfv_size_windows",
		"worker_node_bfv_type_windows",
		"windows_nodes",
	}

	for _, setting := range windowsSettings {
		if !strings.Contains(content, setting) {
			t.Errorf("rendered main.tf missing Windows setting %s when Windows workers are enabled\ncontent:\n%s", setting, content)
		}
	}
}

func TestRenderInfrastructureClusterConditionalWindowsDisabled(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("no-windows-test")
	cfg.OpenCenter.Cluster.ClusterName = "no-windows-test"
	cfg.OpenCenter.GitOps.GitDir = dst

	// Disable Windows workers (default)
	cfg.OpenCenter.Cluster.Kubernetes.WorkerCountWindows = 0

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)

	// Test Windows configuration is not included when disabled
	windowsSettings := []string{
		"size_worker_windows",
		"windows_admin_password",
		"worker_node_bfv_size_windows",
		"worker_node_bfv_type_windows",
		"windows_nodes",
	}

	for _, setting := range windowsSettings {
		if strings.Contains(content, setting) {
			t.Errorf("rendered main.tf should not contain Windows setting %s when Windows workers are disabled", setting)
		}
	}
}

func TestRenderInfrastructureClusterConditionalCalico(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("calico-test")
	cfg.OpenCenter.Cluster.ClusterName = "calico-test"
	cfg.OpenCenter.GitOps.GitDir = dst

	// Enable Calico explicitly
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = true

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)

	// Test Calico module is included when enabled
	if !strings.Contains(content, `module "calico"`) {
		t.Errorf("rendered main.tf missing Calico module when Calico is enabled\ncontent:\n%s", content)
	}
}

func TestRenderInfrastructureClusterConditionalCalicoDisabled(t *testing.T) {
	dst := t.TempDir()
	cfg := config.NewDefault("no-calico-test")
	cfg.OpenCenter.Cluster.ClusterName = "no-calico-test"
	cfg.OpenCenter.GitOps.GitDir = dst

	// Disable Calico and enable Cilium instead
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = true

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)

	// Test Calico module is not included when disabled
	if strings.Contains(content, `module "calico"`) {
		t.Errorf("rendered main.tf should not contain Calico module when Calico is disabled")
	}
}

func TestRenderInfrastructureClusterNetworkPluginSelection(t *testing.T) {
	testCases := []struct {
		name           string
		setupConfig    func(*config.Config)
		expectedPlugin string
	}{
		{
			name: "calico enabled",
			setupConfig: func(cfg *config.Config) {
				cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = true
			},
			expectedPlugin: "calico",
		},
		{
			name: "cilium enabled",
			setupConfig: func(cfg *config.Config) {
				cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
				cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = true
			},
			expectedPlugin: "cilium",
		},
		{
			name: "kube-ovn enabled",
			setupConfig: func(cfg *config.Config) {
				cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
				cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = true
			},
			expectedPlugin: "kube-ovn",
		},
		{
			name: "default to calico",
			setupConfig: func(cfg *config.Config) {
				// Calico is enabled by default, ensure others are disabled
				cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = false
				cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = false
			},
			expectedPlugin: "calico",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			cfg := config.NewDefault("network-plugin-test")
			cfg.OpenCenter.Cluster.ClusterName = "network-plugin-test"
			cfg.OpenCenter.GitOps.GitDir = dst

			tc.setupConfig(&cfg)

			if err := RenderInfrastructureCluster(cfg); err != nil {
				t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
			}

			mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
			data, err := os.ReadFile(mainTF)
			if err != nil {
				t.Fatalf("failed to read rendered main.tf: %v", err)
			}
			content := string(data)

			expectedLine := `network_plugin = "` + tc.expectedPlugin + `"`
			// Use regex to handle variable whitespace in template formatting
			expectedPattern := `network_plugin\s*=\s*"` + tc.expectedPlugin + `"`
			matched, err := regexp.MatchString(expectedPattern, content)
			if err != nil {
				t.Fatalf("failed to compile regex pattern: %v", err)
			}
			if !matched {
				t.Errorf("rendered main.tf missing expected network plugin %s\ncontent:\n%s", expectedLine, content)
			}
		})
	}
}
