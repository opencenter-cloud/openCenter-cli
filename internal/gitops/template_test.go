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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeletRotateServerCertsRendering(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "gitops-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := newDefault("test-cluster")
	cfg.OpenCenter.Meta.Organization = "test-org"
	cfg.OpenCenter.GitOps.Repository.LocalDir = tmpDir

	// Render the infrastructure cluster template
	err = RenderInfrastructureCluster(cfg)
	require.NoError(t, err)

	// Read the rendered main.tf file
	mainTfPath := filepath.Join(tmpDir, "infrastructure", "clusters", "test-cluster", "main.tf")
	content, err := os.ReadFile(mainTfPath)
	require.NoError(t, err)

	mainTfContent := string(content)

	// kubelet_rotate_server_certificates must be false for initial bootstrap because
	// no CNI is installed by Kubespray (CNI is deployed via GitOps after kubeconfig
	// normalization). Kubelet cert rotation requires node Ready, which needs a CNI.
	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = false",
		"Expected kubelet_rotate_server_certificates to be false in locals block (CNI not yet installed)")

	// Check that it's passed to the kubespray-cluster module
	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = local.kubelet_rotate_server_certificates",
		"Expected kubelet_rotate_server_certificates to be passed to kubespray-cluster module")

	t.Logf("Rendered locals block:\n%s", extractSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
	t.Logf("Rendered module block:\n%s", extractModuleSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
}

func TestKubeletRotateServerCertsDefaultValue(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "gitops-test-default-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := newDefault("test-cluster-default")
	cfg.OpenCenter.Meta.Organization = "test-org"
	cfg.OpenCenter.GitOps.Repository.LocalDir = tmpDir

	// Render the infrastructure cluster template
	err = RenderInfrastructureCluster(cfg)
	require.NoError(t, err)

	// Read the rendered main.tf file
	mainTfPath := filepath.Join(tmpDir, "infrastructure", "clusters", "test-cluster-default", "main.tf")
	content, err := os.ReadFile(mainTfPath)
	require.NoError(t, err)

	mainTfContent := string(content)

	// Must be false: CNI is not installed during initial Kubespray run, so kubelet
	// certificate rotation would fail (nodes never reach Ready without a CNI).
	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = false",
		"Expected kubelet_rotate_server_certificates to be false by default (CNI not yet installed)")

	t.Logf("Rendered locals block (default/unset case):\n%s", extractSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
}

func TestOpenStackUsesGitOpsOwnedExternalCCM(t *testing.T) {
	cfg := mustNewGitOpsTestConfig("openstack-gitops-ccm", "openstack")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()

	require.NoError(t, RenderInfrastructureCluster(cfg))
	mainTFPath := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	content, err := os.ReadFile(mainTFPath)
	require.NoError(t, err)

	mainTF := string(content)
	assert.Contains(t, mainTF, `kubelet_cloud_provider                  = "external"`)
	assert.Contains(t, mainTF, `external_cloud_provider                 = "manual"`,
		"Kubespray requires manual mode when GitOps owns OpenStack CCM")
	assert.NotContains(t, mainTF, `external_cloud_provider                 = "openstack"`,
		"Kubespray must not deploy a second OpenStack CCM")
}

func TestRenderInfrastructureClusterLifecycleContract(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		mode       string
		autoDeploy bool
		wantDeploy string
	}{
		{name: "openstack-legacy-enabled", provider: "openstack", mode: "legacy", autoDeploy: true, wantDeploy: "true"},
		{name: "openstack-legacy-disabled", provider: "openstack", mode: "legacy", autoDeploy: false, wantDeploy: "false"},
		{name: "openstack-cli", provider: "openstack", mode: "cli", autoDeploy: true, wantDeploy: "true"},
		{name: "baremetal-legacy-enabled", provider: "baremetal", mode: "legacy", autoDeploy: true, wantDeploy: "true"},
		{name: "baremetal-legacy-disabled", provider: "baremetal", mode: "legacy", autoDeploy: false, wantDeploy: "false"},
		{name: "baremetal-cli", provider: "baremetal", mode: "cli", autoDeploy: true, wantDeploy: "true"},
		{name: "vmware-legacy-enabled", provider: "vmware", mode: "legacy", autoDeploy: true, wantDeploy: "true"},
		{name: "vmware-legacy-disabled", provider: "vmware", mode: "legacy", autoDeploy: false, wantDeploy: "false"},
		{name: "vmware-cli", provider: "vmware", mode: "cli", autoDeploy: true, wantDeploy: "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustNewGitOpsTestConfig("deploy-cluster-"+tt.name, tt.provider)
			cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
			cfg.Deployment.AutoDeploy = tt.autoDeploy

			require.NoError(t, RenderInfrastructureCluster(cfg))

			mainTFPath := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
			content, err := os.ReadFile(mainTFPath)
			require.NoError(t, err)

			mainTF := string(content)
			assert.Contains(t, mainTF, "deploy_cluster                          = var.opencenter_lifecycle_mode == \"cli\" ? false : "+tt.wantDeploy)
			assert.Contains(t, mainTF, "deploy_cluster                          = local.deploy_cluster")

			variables, err := os.ReadFile(filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName(), "variables.tf"))
			require.NoError(t, err)
			assert.Contains(t, string(variables), `variable "opencenter_lifecycle_mode"`)
			assert.Contains(t, string(variables), `default = "legacy"`)
			assert.Contains(t, string(variables), `contains(["legacy", "cli"], var.opencenter_lifecycle_mode)`)

			outputs, err := os.ReadFile(filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName(), "outputs.tf"))
			require.NoError(t, err)
			for _, outputName := range []string{
				"opencenter_kubespray_inventory_path",
				"opencenter_kubespray_lifecycle_contract_version",
				"opencenter_kubespray_api_address",
				"opencenter_kubespray_api_port",
			} {
				assert.Contains(t, string(outputs), `output "`+outputName+`"`)
			}
			assert.Contains(t, string(outputs), `var.opencenter_lifecycle_mode == "legacy"`)
			assert.Contains(t, string(outputs), `try(module.kubespray-cluster.k8s_api_address, null)`)
			assert.Contains(t, string(outputs), `try(module.kubespray-cluster.k8s_api_port, null)`)
			assert.NotContains(t, string(outputs), `module.kubespray-cluster.api_address`)
			assert.NotContains(t, string(outputs), `module.kubespray-cluster.api_port`)
		})
	}
}

func TestRenderInfrastructureClusterHardeningValues(t *testing.T) {
	for _, provider := range []string{"openstack", "baremetal", "vmware"} {
		t.Run(provider, func(t *testing.T) {
			cfg := mustNewGitOpsTestConfig("hardening-"+provider, provider)
			cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
			cfg.OpenCenter.Cluster.Kubernetes.Security.K8sHardening = false
			cfg.OpenCenter.Infrastructure.Networking.Security.OSHardening = false

			require.NoError(t, RenderInfrastructureCluster(cfg))
			mainTFPath := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
			content, err := os.ReadFile(mainTFPath)
			require.NoError(t, err)

			mainTF := string(content)
			assert.Contains(t, mainTF, "k8s_hardening_enabled                   = false")
			assert.Contains(t, mainTF, "os_hardening_enabled                    = false")
			assert.Contains(t, mainTF, "k8s_hardening_enabled                   = local.k8s_hardening_enabled")
			assert.Contains(t, mainTF, "os_hardening_enabled                    = local.os_hardening_enabled")
		})
	}
}

func TestRenderInfrastructureClusterCloudInitTimeout(t *testing.T) {
	for _, provider := range []string{"openstack", "baremetal", "vmware"} {
		t.Run(provider, func(t *testing.T) {
			cfg := mustNewGitOpsTestConfig("cloud-init-timeout-"+provider, provider)
			cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
			cfg.Deployment.Kubespray.CloudInitTimeout = "1h15m"

			require.NoError(t, RenderInfrastructureCluster(cfg))
			mainTFPath := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
			content, err := os.ReadFile(mainTFPath)
			require.NoError(t, err)

			mainTF := string(content)
			// cloud_init_timeout is consumed by the CLI-owned Kubespray lifecycle.
			// Do not pass it unconditionally to legacy/custom Terraform modules:
			// older module contracts do not declare this input and retain their own
			// cloud-init wait defaults.
			assert.NotContains(t, mainTF, "kubesprayCloudInitTimeoutSeconds")
			assert.NotContains(t, mainTF, "cloudinit_wait_timeout_seconds")
		})
	}
}

// extractModuleSnippet extracts lines from the kubespray-cluster module block
func extractModuleSnippet(content, searchTerm string) string {
	lines := strings.Split(content, "\n")
	inModule := false
	var moduleLines []string

	for _, line := range lines {
		if strings.Contains(line, "module \"kubespray-cluster\"") {
			inModule = true
		}
		if inModule {
			moduleLines = append(moduleLines, line)
			if strings.Contains(line, searchTerm) {
				// Get a few more lines after finding the term
				continue
			}
			// Stop after we've collected enough or reached the end of the module
			if len(moduleLines) > 50 || (len(moduleLines) > 5 && strings.TrimSpace(line) == "}") {
				break
			}
		}
	}

	// Find the specific line with our search term
	for i, line := range moduleLines {
		if strings.Contains(line, searchTerm) {
			start := max(0, i-2)
			end := min(len(moduleLines), i+3)
			return strings.Join(moduleLines[start:end], "\n")
		}
	}
	return "Not found in module block"
}

// extractSnippet extracts a few lines around the search term for debugging
func extractSnippet(content, searchTerm string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, searchTerm) {
			start := max(0, i-2)
			end := min(len(lines), i+3)
			return strings.Join(lines[start:end], "\n")
		}
	}
	return "Not found"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
