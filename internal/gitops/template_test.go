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

	// Check that kubelet_rotate_server_certificates is enabled in locals
	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = true",
		"Expected kubelet_rotate_server_certificates to be true in locals block")

	// Check that it's passed to the kubespray-cluster module
	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = local.kubelet_rotate_server_certificates",
		"Expected kubelet_rotate_server_certificates to be passed to kubespray-cluster module")

	t.Logf("Rendered locals block (true case):\n%s", extractSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
	t.Logf("Rendered module block (true case):\n%s", extractModuleSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
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

	assert.Contains(t, mainTfContent, "kubelet_rotate_server_certificates      = true",
		"Expected kubelet_rotate_server_certificates to remain enabled by default")

	t.Logf("Rendered locals block (default/unset case):\n%s", extractSnippet(mainTfContent, "kubelet_rotate_server_certificates"))
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
