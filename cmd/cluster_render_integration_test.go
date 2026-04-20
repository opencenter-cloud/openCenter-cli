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
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// TestRenderClusterTemplatesIntegration tests the render functions
// to ensure they work with the unified GitOps generation interface.
func TestRenderClusterTemplatesIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create test configuration
	cfgPtr, err := v2.NewV2Default("test-render-integration", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.GitOps.Repository.LocalDir = tempDir

	// Create a mock cobra command for output
	cmd := newClusterRenderCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetContext(context.Background())

	// Test rendering all services
	if err := renderAllServices(&cfg, false, false, cmd); err != nil {
		t.Fatalf("renderAllServices failed: %v\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
	}

	// Verify that base files were created
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		t.Errorf("Expected .gitignore to be created at %s", gitignorePath)
	}

	// Verify base directory structure
	appsPath := filepath.Join(tempDir, "applications")
	if _, err := os.Stat(appsPath); os.IsNotExist(err) {
		t.Errorf("Expected applications directory to be created at %s", appsPath)
	}

	// Verify that cluster apps were rendered
	clusterAppsPath := filepath.Join(tempDir, "applications", "overlays", "test-render-integration")
	if _, err := os.Stat(clusterAppsPath); os.IsNotExist(err) {
		t.Errorf("Expected cluster apps directory to be created at %s", clusterAppsPath)
	}

	// Verify output contains success message
	output := stdout.String()
	if output == "" {
		t.Error("Expected output to contain rendering information")
	}
}
