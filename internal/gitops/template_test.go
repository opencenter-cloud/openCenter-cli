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

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeletRotateServerCertsRendering(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "gitops-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test configuration with KubeletRotateServerCerts set to true
	cfg := &v2.Config{
		SchemaVersion: "v2.0.0",
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			GitOps: config.GitOpsConfig{
				GitDir: tmpDir,
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
				Kubernetes: config.KubernetesConfig{
					KubeletRotateServerCerts: true,
				},
			},
		},
	}

	// Render the infrastructure cluster template
	err = RenderInfrastructureCluster(*cfg)
	require.NoError(t, err)

	// Read the rendered main.tf file
	mainTfPath := filepath.Join(tmpDir, "infrastructure", "clusters", "test-cluster", "main.tf")
	content, err := os.ReadFile(mainTfPath)
	require.NoError(t, err)

	mainTfContent := string(content)

	// Check that kubelet_rotate_server_certificates is set to true in locals
	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = true",
		"Expected kubelet_rotate_server_certificates to be true in locals block")

	// Check that it's passed to the kubespray-cluster module
	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = local.kubelet_rotate_server_certificates",
		"Expected kubelet_rotate_server_certificates to be passed to kubespray-cluster module")

	// Test with KubeletRotateServerCerts set to false
	cfg.OpenCenter.Cluster.Kubernetes.KubeletRotateServerCerts = false
	cfg.OpenCenter.Cluster.ClusterName = "test-cluster-false"

	err = RenderInfrastructureCluster(*cfg)
	require.NoError(t, err)

	mainTfPathFalse := filepath.Join(tmpDir, "infrastructure", "clusters", "test-cluster-false", "main.tf")
	contentFalse, err := os.ReadFile(mainTfPathFalse)
	require.NoError(t, err)

	mainTfContentFalse := string(contentFalse)

	// Check that kubelet_rotate_server_certificates is set to false in locals
	assert.Contains(t, mainTfContentFalse, "kubelet_rotate_server_certificates      = false",
		"Expected kubelet_rotate_server_certificates to be false in locals block")

	// Print a snippet of the rendered content for debugging
	t.Logf("Rendered locals block (true case):\n%s", extractSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
	t.Logf("Rendered module block (true case):\n%s", extractModuleSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
	t.Logf("Rendered locals block (false case):\n%s", extractSnippet(mainTfContentFalse, "kubelet_rotate_server_certificates"))
}

func TestKubeletRotateServerCertsDefaultValue(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "gitops-test-default-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test configuration WITHOUT explicitly setting KubeletRotateServerCerts
	// When not set, it will be the Go zero value (false)
	// Users should explicitly set this in their config or use cluster init which sets defaults
	cfg := &v2.Config{
		SchemaVersion: "v2.0.0",
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster-default",
				Organization: "test-org",
			},
			GitOps: config.GitOpsConfig{
				GitDir: tmpDir,
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster-default",
				Kubernetes:  config.KubernetesConfig{
					// KubeletRotateServerCerts not set - will be false (Go zero value)
					// In production, cluster init sets this to true
				},
			},
		},
	}

	// Render the infrastructure cluster template
	err = RenderInfrastructureCluster(*cfg)
	require.NoError(t, err)

	// Read the rendered main.tf file
	mainTfPath := filepath.Join(tmpDir, "infrastructure", "clusters", "test-cluster-default", "main.tf")
	content, err := os.ReadFile(mainTfPath)
	require.NoError(t, err)

	mainTfContent := string(content)

	// When not explicitly set, the Go zero value (false) is used
	// This is expected behavior - users should explicitly set this value or use cluster init
	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = false",
		"Expected kubelet_rotate_server_certificates to be false when not explicitly set (Go zero value)")

	t.Logf("Rendered locals block (default/unset case):\n%s", extractSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
	t.Logf("Note: In production, 'cluster init' sets KubeletRotateServerCerts to true by default")
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
