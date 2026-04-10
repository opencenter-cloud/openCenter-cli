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

// TestPipelineIntegration_WithInitStage tests the complete pipeline with a real stage.
func TestPipelineIntegration_WithInitStage(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := gitops.NewWorkspaceManager(tempDir)

	// Create real stages
	initStage := NewInitStage()
	stageList := []gitops.GenerationStage{initStage}

	// Create pipeline generator
	generator := gitops.NewPipelineGenerator(workspaceManager, stageList)

	// Track progress
	var progressStages []string
	generator.SetProgressCallback(func(stage string, progress int, message string) {
		progressStages = append(progressStages, stage)
	})

	// Create test configuration
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
		},
	}

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify success
	require.NoError(t, err)

	// Verify workspace was created
	workspace := generator.GetWorkspace()
	require.NotNil(t, workspace)

	// Verify init stage created the expected structure
	assert.True(t, workspace.Exists("infrastructure"))
	assert.True(t, workspace.Exists("infrastructure/clusters"))
	assert.True(t, workspace.Exists("applications"))
	assert.True(t, workspace.Exists("applications/base"))
	assert.True(t, workspace.Exists("applications/overlays"))
	assert.True(t, workspace.Exists(".flux-system"))
	assert.True(t, workspace.Exists("README.md"))
	assert.True(t, workspace.Exists(".gitignore"))

	// Verify progress was reported
	assert.Contains(t, progressStages, "initialization")
	assert.Contains(t, progressStages, "init")
	assert.Contains(t, progressStages, "completion")

	// Verify completed stages
	completedStages := generator.GetCompletedStages()
	assert.Len(t, completedStages, 1)
	assert.Equal(t, "init", completedStages[0])
}

// TestPipelineIntegration_DryRun tests dry-run mode with real stages.
func TestPipelineIntegration_DryRun(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := gitops.NewWorkspaceManager(tempDir)

	// Create real stages
	initStage := NewInitStage()
	stageList := []gitops.GenerationStage{initStage}

	// Create pipeline generator
	generator := gitops.NewPipelineGenerator(workspaceManager, stageList)

	// Create test configuration
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
		},
	}

	// Execute dry-run
	ctx := context.Background()
	plan, err := generator.GenerateDryRun(ctx, cfg)

	// Verify success
	require.NoError(t, err)
	require.NotNil(t, plan)

	// Verify plan contents
	assert.Len(t, plan.Stages, 1)
	assert.Equal(t, "init", plan.Stages[0].Name)
	assert.Len(t, plan.Stages[0].Files, 2)
	assert.Len(t, plan.Stages[0].Directories, 6)
	assert.Equal(t, 2, plan.TotalFiles)
	assert.Equal(t, 6, plan.TotalDirectories)
}
