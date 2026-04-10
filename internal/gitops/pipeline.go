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
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/metrics"
)

// PipelineGenerator implements GitOpsGenerator using a staged pipeline approach.
// Each stage executes independently and can be rolled back on failure.
type PipelineGenerator struct {
	// stages contains all generation stages in execution order
	stages []GenerationStage

	// workspace is the current workspace for generation
	workspace *GitOpsWorkspace

	// workspaceManager manages workspace lifecycle
	workspaceManager WorkspaceManager

	// options configures the generation process
	options GenerationOptions

	// progressCallback is called to report progress
	progressCallback ProgressCallback

	// completedStages tracks which stages have completed successfully
	completedStages []string

	// currentStage is the stage currently being executed
	currentStage string
}

// GenerationStage defines the interface for a single generation stage.
// Each stage is responsible for a specific aspect of GitOps repository generation.
type GenerationStage interface {
	// Name returns the unique name of this stage
	Name() string

	// Description returns a human-readable description of what this stage does
	Description() string

	// Dependencies returns the names of stages that must complete before this stage
	Dependencies() []string

	// Execute performs the stage's generation operations
	Execute(ctx context.Context, workspace *GitOpsWorkspace) error

	// Rollback undoes the changes made by this stage
	Rollback(ctx context.Context, workspace *GitOpsWorkspace) error

	// Validate checks if the stage completed successfully
	Validate(ctx context.Context, workspace *GitOpsWorkspace) error

	// DryRun returns a plan of what this stage would do without executing
	DryRun(ctx context.Context, cfg v2.Config) (*StagePlan, error)
}

// NewPipelineGenerator creates a new pipeline-based GitOps generator.
func NewPipelineGenerator(workspaceManager WorkspaceManager, stages []GenerationStage) *PipelineGenerator {
	return &PipelineGenerator{
		stages:           stages,
		workspaceManager: workspaceManager,
		options:          DefaultGenerationOptions(),
		completedStages:  make([]string, 0),
	}
}

// NewPipelineGeneratorWithOptions creates a new pipeline generator with custom options.
func NewPipelineGeneratorWithOptions(workspaceManager WorkspaceManager, stages []GenerationStage, options GenerationOptions) *PipelineGenerator {
	return &PipelineGenerator{
		stages:           stages,
		workspaceManager: workspaceManager,
		options:          options,
		progressCallback: options.ProgressCallback,
		completedStages:  make([]string, 0),
	}
}

// Generate executes the complete GitOps generation pipeline.
// Stages are executed in order, with automatic rollback on failure.
func (pg *PipelineGenerator) Generate(ctx context.Context, cfg v2.Config) error {
	startTime := time.Now()
	var generationErr error
	clusterName := cfg.OpenCenter.Meta.Name
	filesGenerated := 0

	// Record metrics at the end
	defer func() {
		duration := time.Since(startTime)
		// Count files in workspace if generation succeeded
		if generationErr == nil && pg.workspace != nil {
			filesGenerated = pg.workspace.GetFileCount()
		}
		// Record metric using global collector
		metrics.RecordGitOpsGeneration(clusterName, duration, filesGenerated, generationErr == nil, generationErr)
	}()

	// Validate options
	if err := pg.options.Validate(); err != nil {
		generationErr = fmt.Errorf("invalid generation options: %w", err)
		return generationErr
	}

	// Create workspace
	workspace, err := pg.workspaceManager.CreateWorkspace(ctx, cfg)
	if err != nil {
		generationErr = fmt.Errorf("failed to create workspace: %w", err)
		return generationErr
	}
	pg.workspace = workspace

	// Ensure workspace cleanup on error if configured
	defer func() {
		if generationErr != nil && pg.options.CleanupOnError {
			if cleanupErr := pg.workspaceManager.CleanupWorkspace(ctx, workspace); cleanupErr != nil {
				fmt.Printf("Warning: failed to cleanup workspace after error: %v\n", cleanupErr)
			}
		}
	}()

	// Report initial progress
	pg.reportProgress("initialization", 0, "Starting GitOps generation")

	// Validate stage dependencies
	if err := pg.validateStageDependencies(); err != nil {
		generationErr = fmt.Errorf("invalid stage dependencies: %w", err)
		return generationErr
	}

	// Execute stages in order
	totalStages := len(pg.stages)
	for i, stage := range pg.stages {
		pg.currentStage = stage.Name()

		// Calculate progress percentage
		progress := int(float64(i) / float64(totalStages) * 100)
		pg.reportProgress(stage.Name(), progress, fmt.Sprintf("Executing stage: %s", stage.Description()))

		// Create checkpoint before stage execution
		checkpointID := fmt.Sprintf("stage-%s-%d", stage.Name(), time.Now().UnixNano())
		if _, err := workspace.CreateCheckpoint(checkpointID); err != nil {
			generationErr = fmt.Errorf("failed to create checkpoint for stage %s: %w", stage.Name(), err)
			return generationErr
		}

		// Execute stage
		if err := stage.Execute(ctx, workspace); err != nil {
			pg.reportProgress(stage.Name(), progress, fmt.Sprintf("Stage failed: %s", err.Error()))

			// Rollback all completed stages in reverse order
			if rollbackErr := pg.rollbackCompletedStages(ctx); rollbackErr != nil {
				generationErr = fmt.Errorf("stage %s failed and rollback failed: %w (original error: %v)", stage.Name(), rollbackErr, err)
				return generationErr
			}

			generationErr = fmt.Errorf("stage %s failed: %w", stage.Name(), err)
			return generationErr
		}

		// Mark stage as completed (before validation so it gets rolled back if validation fails)
		pg.completedStages = append(pg.completedStages, stage.Name())

		// Validate stage completion
		if !pg.options.SkipValidation {
			if err := stage.Validate(ctx, workspace); err != nil {
				pg.reportProgress(stage.Name(), progress, fmt.Sprintf("Stage validation failed: %s", err.Error()))

				// Rollback all completed stages (including this one)
				if rollbackErr := pg.rollbackCompletedStages(ctx); rollbackErr != nil {
					generationErr = fmt.Errorf("stage %s validation failed and rollback failed: %w (original error: %v)", stage.Name(), rollbackErr, err)
					return generationErr
				}

				generationErr = fmt.Errorf("stage %s validation failed: %w", stage.Name(), err)
				return generationErr
			}
		}

		// Clean up checkpoint (keep only the most recent one)
		if err := workspace.DeleteCheckpoint(checkpointID); err != nil {
			// Log warning but don't fail
			fmt.Printf("Warning: failed to delete checkpoint %s: %v\n", checkpointID, err)
		}
	}

	// Report completion
	duration := time.Since(startTime)
	pg.reportProgress("completion", 100, fmt.Sprintf("Generation completed successfully in %s", duration))

	return nil
}

// GenerateDryRun performs a dry-run of the generation process.
// It executes all stages with a dry-run workspace that records operations without
// making filesystem changes, providing an accurate preview of what would be generated.
func (pg *PipelineGenerator) GenerateDryRun(ctx context.Context, cfg v2.Config) (*GenerationPlan, error) {
	startTime := time.Now()

	// Create a dry-run workspace that tracks operations without filesystem changes
	dryRunWS := NewDryRunWorkspace(cfg)

	// Convert to GitOpsWorkspace interface for stage execution
	workspace := &GitOpsWorkspace{
		ID:           dryRunWS.ID,
		RootDir:      dryRunWS.RootDir,
		TempDir:      dryRunWS.TempDir,
		Config:       dryRunWS.Config,
		Metadata:     dryRunWS.Metadata,
		Checkpoints:  dryRunWS.Checkpoints,
		createdAt:    dryRunWS.createdAt,
		lastModified: dryRunWS.lastModified,
	}

	// Store reference to dry-run workspace in metadata for stages to access
	workspace.SetMetadata("dryrun_workspace", dryRunWS)
	workspace.SetMetadata("is_dryrun", true)

	plan := &GenerationPlan{
		Stages:           make([]StagePlan, 0, len(pg.stages)),
		TotalFiles:       0,
		TotalDirectories: 0,
		Warnings:         make([]string, 0),
	}

	// Execute each stage in dry-run mode
	for _, stage := range pg.stages {
		// Report progress if callback is set
		pg.reportProgress(stage.Name(), 0, fmt.Sprintf("Dry-run: %s", stage.Description()))

		// Execute the stage with the dry-run workspace
		// Stages should detect dry-run mode and use DryRunAtomicWriter
		if err := stage.Execute(ctx, workspace); err != nil {
			return nil, fmt.Errorf("dry-run failed for stage %s: %w", stage.Name(), err)
		}

		// Collect the operations performed by this stage
		stageFiles := make([]string, 0)
		stageDirs := make([]string, 0)

		// Get operations from dry-run workspace
		for _, op := range dryRunWS.GetOperations() {
			if op.Stage == stage.Name() {
				switch op.Type {
				case OpWriteFile:
					stageFiles = append(stageFiles, op.Path)
				case OpCreateDir:
					stageDirs = append(stageDirs, op.Path)
				}
			}
		}

		// Create stage plan from actual execution
		stagePlan := StagePlan{
			Name:         stage.Name(),
			Description:  stage.Description(),
			Files:        stageFiles,
			Directories:  stageDirs,
			Dependencies: stage.Dependencies(),
		}

		plan.Stages = append(plan.Stages, stagePlan)
	}

	// Get final counts from dry-run workspace
	plan.TotalFiles = dryRunWS.GetFileCount()
	plan.TotalDirectories = dryRunWS.GetDirectoryCount()

	// Calculate actual duration
	duration := time.Since(startTime)
	plan.EstimatedDuration = duration.String()

	// Add warnings if any
	if plan.TotalFiles == 0 {
		plan.Warnings = append(plan.Warnings, "No files would be generated")
	}

	return plan, nil
}

// Rollback restores the workspace to a previous checkpoint.
func (pg *PipelineGenerator) Rollback(ctx context.Context, checkpointID string) error {
	if pg.workspace == nil {
		return fmt.Errorf("no active workspace")
	}

	return pg.workspace.RestoreCheckpoint(checkpointID)
}

// GetWorkspace returns the current workspace.
func (pg *PipelineGenerator) GetWorkspace() *GitOpsWorkspace {
	return pg.workspace
}

// SetProgressCallback sets the progress callback function.
func (pg *PipelineGenerator) SetProgressCallback(callback ProgressCallback) {
	pg.progressCallback = callback
}

// rollbackCompletedStages rolls back all completed stages in reverse order.
func (pg *PipelineGenerator) rollbackCompletedStages(ctx context.Context) error {
	// Rollback in reverse order
	for i := len(pg.completedStages) - 1; i >= 0; i-- {
		stageName := pg.completedStages[i]

		// Find the stage
		var stage GenerationStage
		for _, s := range pg.stages {
			if s.Name() == stageName {
				stage = s
				break
			}
		}

		if stage == nil {
			return fmt.Errorf("stage not found for rollback: %s", stageName)
		}

		// Rollback the stage
		pg.reportProgress(stageName, 0, fmt.Sprintf("Rolling back stage: %s", stageName))
		if err := stage.Rollback(ctx, pg.workspace); err != nil {
			return fmt.Errorf("failed to rollback stage %s: %w", stageName, err)
		}
	}

	// Clear completed stages
	pg.completedStages = make([]string, 0)

	return nil
}

// validateStageDependencies checks that all stage dependencies are satisfied.
func (pg *PipelineGenerator) validateStageDependencies() error {
	// Build a map of stage names for quick lookup
	stageNames := make(map[string]bool)
	for _, stage := range pg.stages {
		stageNames[stage.Name()] = true
	}

	// Build a map of stage positions
	stagePositions := make(map[string]int)
	for i, stage := range pg.stages {
		stagePositions[stage.Name()] = i
	}

	// Check each stage's dependencies
	for i, stage := range pg.stages {
		for _, dep := range stage.Dependencies() {
			// Check if dependency exists
			if !stageNames[dep] {
				return fmt.Errorf("stage %s depends on non-existent stage: %s", stage.Name(), dep)
			}

			// Check if dependency comes before this stage
			depPosition, exists := stagePositions[dep]
			if !exists || depPosition >= i {
				return fmt.Errorf("stage %s depends on %s which must come before it", stage.Name(), dep)
			}
		}
	}

	return nil
}

// reportProgress calls the progress callback if one is set.
func (pg *PipelineGenerator) reportProgress(stage string, progress int, message string) {
	if pg.progressCallback != nil {
		pg.progressCallback(stage, progress, message)
	}

	if pg.options.Verbose {
		fmt.Printf("[%s] %d%% - %s\n", stage, progress, message)
	}
}

// GetCompletedStages returns the list of stages that have completed successfully.
func (pg *PipelineGenerator) GetCompletedStages() []string {
	return append([]string{}, pg.completedStages...)
}

// GetCurrentStage returns the name of the stage currently being executed.
func (pg *PipelineGenerator) GetCurrentStage() string {
	return pg.currentStage
}

// GetTotalStages returns the total number of stages in the pipeline.
func (pg *PipelineGenerator) GetTotalStages() int {
	return len(pg.stages)
}
