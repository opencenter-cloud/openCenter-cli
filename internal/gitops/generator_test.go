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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStage is a mock implementation of GenerationStage for testing.
type mockStage struct {
	name         string
	description  string
	dependencies []string
	executeFunc  func(ctx context.Context, workspace *GitOpsWorkspace) error
	rollbackFunc func(ctx context.Context, workspace *GitOpsWorkspace) error
	validateFunc func(ctx context.Context, workspace *GitOpsWorkspace) error
	dryRunFunc   func(ctx context.Context, cfg v2.Config) (*StagePlan, error)
	executed     bool
	rolledBack   bool
	validated    bool
}

func newMockStage(name, description string, dependencies []string) *mockStage {
	return &mockStage{
		name:         name,
		description:  description,
		dependencies: dependencies,
		executeFunc: func(ctx context.Context, workspace *GitOpsWorkspace) error {
			return nil
		},
		rollbackFunc: func(ctx context.Context, workspace *GitOpsWorkspace) error {
			return nil
		},
		validateFunc: func(ctx context.Context, workspace *GitOpsWorkspace) error {
			return nil
		},
		dryRunFunc: func(ctx context.Context, cfg v2.Config) (*StagePlan, error) {
			return &StagePlan{
				Name:         name,
				Description:  description,
				Files:        []string{},
				Directories:  []string{},
				Dependencies: dependencies,
			}, nil
		},
	}
}

func (ms *mockStage) Name() string {
	return ms.name
}

func (ms *mockStage) Description() string {
	return ms.description
}

func (ms *mockStage) Dependencies() []string {
	return ms.dependencies
}

func (ms *mockStage) Execute(ctx context.Context, workspace *GitOpsWorkspace) error {
	ms.executed = true
	return ms.executeFunc(ctx, workspace)
}

func (ms *mockStage) Rollback(ctx context.Context, workspace *GitOpsWorkspace) error {
	ms.rolledBack = true
	return ms.rollbackFunc(ctx, workspace)
}

func (ms *mockStage) Validate(ctx context.Context, workspace *GitOpsWorkspace) error {
	ms.validated = true
	return ms.validateFunc(ctx, workspace)
}

func (ms *mockStage) DryRun(ctx context.Context, cfg v2.Config) (*StagePlan, error) {
	return ms.dryRunFunc(ctx, cfg)
}

func TestPipelineGenerator_Generate_Success(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create mock stages
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage2 := newMockStage("stage2", "Second stage", []string{"stage1"})
	stage3 := newMockStage("stage3", "Third stage", []string{"stage2"})

	stages := []GenerationStage{stage1, stage2, stage3}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Create test configuration
	cfg := createTestConfig()

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify success
	require.NoError(t, err)
	assert.True(t, stage1.executed, "Stage 1 should be executed")
	assert.True(t, stage2.executed, "Stage 2 should be executed")
	assert.True(t, stage3.executed, "Stage 3 should be executed")
	assert.True(t, stage1.validated, "Stage 1 should be validated")
	assert.True(t, stage2.validated, "Stage 2 should be validated")
	assert.True(t, stage3.validated, "Stage 3 should be validated")
	assert.False(t, stage1.rolledBack, "Stage 1 should not be rolled back")
	assert.False(t, stage2.rolledBack, "Stage 2 should not be rolled back")
	assert.False(t, stage3.rolledBack, "Stage 3 should not be rolled back")

	// Verify workspace was created
	workspace := generator.GetWorkspace()
	require.NotNil(t, workspace)
	assert.DirExists(t, workspace.RootDir)

	// Verify completed stages
	completedStages := generator.GetCompletedStages()
	assert.Len(t, completedStages, 3)
	assert.Equal(t, []string{"stage1", "stage2", "stage3"}, completedStages)
}

func TestPipelineGenerator_Generate_StageFailure_TriggersRollback(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create mock stages
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage2 := newMockStage("stage2", "Second stage (fails)", []string{"stage1"})
	stage3 := newMockStage("stage3", "Third stage", []string{"stage2"})

	// Make stage2 fail
	stage2.executeFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		return assert.AnError
	}

	stages := []GenerationStage{stage1, stage2, stage3}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Create test configuration
	cfg := createTestConfig()

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stage2 failed")

	// Verify execution order
	assert.True(t, stage1.executed, "Stage 1 should be executed")
	assert.True(t, stage2.executed, "Stage 2 should be executed (and failed)")
	assert.False(t, stage3.executed, "Stage 3 should not be executed")

	// Verify rollback was triggered
	assert.True(t, stage1.rolledBack, "Stage 1 should be rolled back")
	assert.False(t, stage2.rolledBack, "Stage 2 should not be rolled back (it failed)")
	assert.False(t, stage3.rolledBack, "Stage 3 should not be rolled back (never executed)")

	// Verify no completed stages
	completedStages := generator.GetCompletedStages()
	assert.Len(t, completedStages, 0, "No stages should be marked as completed after rollback")
}

func TestPipelineGenerator_Generate_ValidationFailure_TriggersRollback(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create mock stages
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage2 := newMockStage("stage2", "Second stage (validation fails)", []string{"stage1"})

	// Make stage2 validation fail
	stage2.validateFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		return assert.AnError
	}

	stages := []GenerationStage{stage1, stage2}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Create test configuration
	cfg := createTestConfig()

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")

	// Verify execution
	assert.True(t, stage1.executed, "Stage 1 should be executed")
	assert.True(t, stage2.executed, "Stage 2 should be executed")
	assert.True(t, stage2.validated, "Stage 2 validation should be attempted")

	// Verify rollback was triggered
	assert.True(t, stage1.rolledBack, "Stage 1 should be rolled back")
	assert.True(t, stage2.rolledBack, "Stage 2 should be rolled back")
}

func TestPipelineGenerator_GenerateDryRun(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create mock stages with dry-run plans
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage1.dryRunFunc = func(ctx context.Context, cfg v2.Config) (*StagePlan, error) {
		return &StagePlan{
			Name:         "stage1",
			Description:  "First stage",
			Files:        []string{"file1.yaml", "file2.yaml"},
			Directories:  []string{"dir1"},
			Dependencies: []string{},
		}, nil
	}

	stage2 := newMockStage("stage2", "Second stage", []string{"stage1"})
	stage2.dryRunFunc = func(ctx context.Context, cfg v2.Config) (*StagePlan, error) {
		return &StagePlan{
			Name:         "stage2",
			Description:  "Second stage",
			Files:        []string{"file3.yaml"},
			Directories:  []string{"dir2", "dir3"},
			Dependencies: []string{"stage1"},
		}, nil
	}

	stages := []GenerationStage{stage1, stage2}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Create test configuration
	cfg := createTestConfig()

	// Execute dry-run
	ctx := context.Background()
	plan, err := generator.GenerateDryRun(ctx, cfg)

	// Verify success
	require.NoError(t, err)
	require.NotNil(t, plan)

	// Verify plan contents
	assert.Len(t, plan.Stages, 2)
	assert.NotEmpty(t, plan.EstimatedDuration)

	// Verify stages were executed (in dry-run mode)
	assert.True(t, stage1.executed, "Stage 1 should be executed in dry-run mode")
	assert.True(t, stage2.executed, "Stage 2 should be executed in dry-run mode")

	// CRITICAL: Verify no actual filesystem changes occurred
	// This is the key test for Property 17: Dry-Run Filesystem Safety
	workspaceDir := filepath.Join(tempDir, "gitops-workspaces")

	// Check if workspace directory exists
	if _, err := os.Stat(workspaceDir); err == nil {
		// If it exists, verify it's empty or only contains dry-run workspaces
		entries, err := os.ReadDir(workspaceDir)
		require.NoError(t, err)

		for _, entry := range entries {
			// Dry-run workspaces should have "dryrun-" prefix
			assert.Contains(t, entry.Name(), "dryrun-",
				"Only dry-run workspaces should exist, found: %s", entry.Name())

			// Verify the dry-run workspace directory is empty (no actual files created)
			workspacePath := filepath.Join(workspaceDir, entry.Name())
			wsEntries, err := os.ReadDir(workspacePath)
			if err == nil {
				assert.Empty(t, wsEntries,
					"Dry-run workspace should not contain any actual files or directories")
			}
		}
	}

	// Verify no files were created in temp directory
	tempEntries, err := os.ReadDir(tempDir)
	require.NoError(t, err)

	// Should only have the gitops-workspaces directory (if any)
	for _, entry := range tempEntries {
		if entry.Name() != "gitops-workspaces" {
			t.Errorf("Unexpected entry in temp directory: %s", entry.Name())
		}
	}
}

func TestPipelineGenerator_ValidateStageDependencies_Success(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create stages with valid dependencies
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage2 := newMockStage("stage2", "Second stage", []string{"stage1"})
	stage3 := newMockStage("stage3", "Third stage", []string{"stage1", "stage2"})

	stages := []GenerationStage{stage1, stage2, stage3}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Validate dependencies
	err := generator.validateStageDependencies()

	// Verify success
	assert.NoError(t, err)
}

func TestPipelineGenerator_ValidateStageDependencies_MissingDependency(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create stages with missing dependency
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage2 := newMockStage("stage2", "Second stage", []string{"nonexistent"})

	stages := []GenerationStage{stage1, stage2}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Validate dependencies
	err := generator.validateStageDependencies()

	// Verify failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-existent")
}

func TestPipelineGenerator_ValidateStageDependencies_WrongOrder(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create stages with dependency in wrong order
	stage1 := newMockStage("stage1", "First stage", []string{"stage2"})
	stage2 := newMockStage("stage2", "Second stage", []string{})

	stages := []GenerationStage{stage1, stage2}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Validate dependencies
	err := generator.validateStageDependencies()

	// Verify failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must come before")
}

func TestPipelineGenerator_ProgressCallback(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create mock stages
	stage1 := newMockStage("stage1", "First stage", []string{})
	stages := []GenerationStage{stage1}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Track progress callbacks
	var progressCalls []string
	generator.SetProgressCallback(func(stage string, progress int, message string) {
		progressCalls = append(progressCalls, stage)
	})

	// Create test configuration
	cfg := createTestConfig()

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify success
	require.NoError(t, err)

	// Verify progress callbacks were made
	assert.NotEmpty(t, progressCalls)
	assert.Contains(t, progressCalls, "initialization")
	assert.Contains(t, progressCalls, "stage1")
	assert.Contains(t, progressCalls, "completion")
}

func TestPipelineGenerator_MultipleStagesRollbackInReverseOrder(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Track rollback order
	var rollbackOrder []string

	// Create mock stages
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage1.rollbackFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		rollbackOrder = append(rollbackOrder, "stage1")
		return nil
	}

	stage2 := newMockStage("stage2", "Second stage", []string{"stage1"})
	stage2.rollbackFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		rollbackOrder = append(rollbackOrder, "stage2")
		return nil
	}

	stage3 := newMockStage("stage3", "Third stage", []string{"stage2"})
	stage3.rollbackFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		rollbackOrder = append(rollbackOrder, "stage3")
		return nil
	}

	stage4 := newMockStage("stage4", "Fourth stage (fails)", []string{"stage3"})
	stage4.executeFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		return assert.AnError
	}

	stages := []GenerationStage{stage1, stage2, stage3, stage4}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Create test configuration
	cfg := createTestConfig()

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stage4 failed")

	// Verify all previous stages were executed
	assert.True(t, stage1.executed, "Stage 1 should be executed")
	assert.True(t, stage2.executed, "Stage 2 should be executed")
	assert.True(t, stage3.executed, "Stage 3 should be executed")
	assert.True(t, stage4.executed, "Stage 4 should be executed (and failed)")

	// Verify rollback happened in reverse order
	assert.True(t, stage1.rolledBack, "Stage 1 should be rolled back")
	assert.True(t, stage2.rolledBack, "Stage 2 should be rolled back")
	assert.True(t, stage3.rolledBack, "Stage 3 should be rolled back")
	assert.False(t, stage4.rolledBack, "Stage 4 should not be rolled back (it failed)")

	// Verify rollback order is reverse of execution order
	expectedRollbackOrder := []string{"stage3", "stage2", "stage1"}
	assert.Equal(t, expectedRollbackOrder, rollbackOrder, "Stages should be rolled back in reverse order")

	// Verify no completed stages remain
	completedStages := generator.GetCompletedStages()
	assert.Len(t, completedStages, 0, "No stages should be marked as completed after rollback")
}

func TestPipelineGenerator_RollbackFailureReportsError(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create mock stages
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage1.rollbackFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		return fmt.Errorf("rollback failed for stage1")
	}

	stage2 := newMockStage("stage2", "Second stage (fails)", []string{"stage1"})
	stage2.executeFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		return assert.AnError
	}

	stages := []GenerationStage{stage1, stage2}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Create test configuration
	cfg := createTestConfig()

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify failure includes both original error and rollback error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stage2 failed")
	assert.Contains(t, err.Error(), "rollback failed")
	assert.Contains(t, err.Error(), "stage1")
}

func TestPipelineGenerator_WorkspaceCleanupOnError(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create mock stage that fails
	stage1 := newMockStage("stage1", "First stage (fails)", []string{})
	stage1.executeFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		return assert.AnError
	}

	stages := []GenerationStage{stage1}

	// Create pipeline generator with cleanup on error
	options := DefaultGenerationOptions()
	options.CleanupOnError = true
	generator := NewPipelineGeneratorWithOptions(workspaceManager, stages, options)

	// Create test configuration
	cfg := createTestConfig()

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify failure
	require.Error(t, err)

	// Verify workspace was cleaned up
	workspace := generator.GetWorkspace()
	require.NotNil(t, workspace)

	// The workspace directory should not exist after cleanup
	_, statErr := os.Stat(workspace.RootDir)
	assert.True(t, os.IsNotExist(statErr), "Workspace directory should be cleaned up")
}

func TestPipelineGenerator_CheckpointCreation(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create workspace manager
	workspaceManager := NewWorkspaceManager(tempDir)

	// Create mock stage that creates a file
	stage1 := newMockStage("stage1", "First stage", []string{})
	stage1.executeFunc = func(ctx context.Context, workspace *GitOpsWorkspace) error {
		// Create a test file
		writer := NewAtomicWriter(workspace)
		return writer.WriteFileString("test.txt", "test content", 0o644)
	}

	stages := []GenerationStage{stage1}

	// Create pipeline generator
	generator := NewPipelineGenerator(workspaceManager, stages)

	// Create test configuration
	cfg := createTestConfig()

	// Execute generation
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)

	// Verify success
	require.NoError(t, err)

	// Verify file was created
	workspace := generator.GetWorkspace()
	require.NotNil(t, workspace)
	testFilePath := filepath.Join(workspace.RootDir, "test.txt")
	assert.FileExists(t, testFilePath)
}

// createTestConfig creates a minimal test configuration.
func createTestConfig() v2.Config {
	return v2.Config{
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
}
