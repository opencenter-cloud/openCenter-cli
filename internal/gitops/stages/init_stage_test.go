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

package stages

import (
	"context"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitStage_Execute(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager and workspace
	workspaceManager := gitops.NewWorkspaceManager(tempDir)
	cfg := createTestConfig("openstack")
	ctx := context.Background()

	workspace, err := workspaceManager.CreateWorkspace(ctx, cfg)
	require.NoError(t, err)
	defer workspaceManager.CleanupWorkspace(ctx, workspace)

	// Create init stage
	stage := NewInitStage()

	// Execute stage
	err = stage.Execute(ctx, workspace)
	require.NoError(t, err)

	// Verify directories were created
	assert.True(t, workspace.Exists("infrastructure"))
	assert.True(t, workspace.Exists("infrastructure/clusters"))
	assert.True(t, workspace.Exists("applications"))
	assert.True(t, workspace.Exists("applications/base"))
	assert.True(t, workspace.Exists("applications/overlays"))
	assert.True(t, workspace.Exists(".flux-system"))

	// Verify files were created
	assert.True(t, workspace.Exists("README.md"))
	assert.True(t, workspace.Exists(".gitignore"))
}

func TestInitStage_Validate(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager and workspace
	workspaceManager := gitops.NewWorkspaceManager(tempDir)
	cfg := createTestConfig("openstack")
	ctx := context.Background()

	workspace, err := workspaceManager.CreateWorkspace(ctx, cfg)
	require.NoError(t, err)
	defer workspaceManager.CleanupWorkspace(ctx, workspace)

	// Create init stage
	stage := NewInitStage()

	// Execute stage
	err = stage.Execute(ctx, workspace)
	require.NoError(t, err)

	// Validate stage
	err = stage.Validate(ctx, workspace)
	assert.NoError(t, err)
}

func TestInitStage_Validate_MissingDirectory(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager and workspace
	workspaceManager := gitops.NewWorkspaceManager(tempDir)
	cfg := createTestConfig("openstack")
	ctx := context.Background()

	workspace, err := workspaceManager.CreateWorkspace(ctx, cfg)
	require.NoError(t, err)
	defer workspaceManager.CleanupWorkspace(ctx, workspace)

	// Create init stage
	stage := NewInitStage()

	// Don't execute stage, just validate (should fail)
	err = stage.Validate(ctx, workspace)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required directory not found")
}

func TestInitStage_Rollback(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager and workspace
	workspaceManager := gitops.NewWorkspaceManager(tempDir)
	cfg := createTestConfig("openstack")
	ctx := context.Background()

	workspace, err := workspaceManager.CreateWorkspace(ctx, cfg)
	require.NoError(t, err)
	defer workspaceManager.CleanupWorkspace(ctx, workspace)

	// Create init stage
	stage := NewInitStage()

	// Execute stage
	err = stage.Execute(ctx, workspace)
	require.NoError(t, err)

	// Verify files exist
	assert.True(t, workspace.Exists("README.md"))
	assert.True(t, workspace.Exists("infrastructure"))

	// Rollback stage
	err = stage.Rollback(ctx, workspace)
	require.NoError(t, err)

	// Verify files were removed
	assert.False(t, workspace.Exists("README.md"))
	assert.False(t, workspace.Exists("infrastructure"))
	assert.False(t, workspace.Exists("applications"))
}

func TestInitStage_DryRun(t *testing.T) {
	// Create init stage
	stage := NewInitStage()

	// Create test configuration
	cfg := createTestConfig("openstack")

	// Execute dry-run
	ctx := context.Background()
	plan, err := stage.DryRun(ctx, cfg)

	// Verify success
	require.NoError(t, err)
	require.NotNil(t, plan)

	// Verify plan contents
	assert.Equal(t, "init", plan.Name)
	assert.NotEmpty(t, plan.Description)
	assert.Len(t, plan.Files, 2) // README.md and .gitignore
	assert.Len(t, plan.Directories, 6)
	assert.Empty(t, plan.Dependencies)
}

func TestInitStage_Properties(t *testing.T) {
	// Create init stage
	stage := NewInitStage()

	// Verify properties
	assert.Equal(t, "init", stage.Name())
	assert.NotEmpty(t, stage.Description())
	assert.Empty(t, stage.Dependencies())
}

// createTestConfig creates a minimal test configuration.
func createTestConfig(provider string) v2.Config {
	return v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: config.Infrastructure{
				Provider: provider,
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
		},
	}
}
