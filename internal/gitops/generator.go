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

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// GitOpsGenerator defines the interface for GitOps repository generation.
// Implementations should support staged execution with rollback capabilities.
type GitOpsGenerator interface {
	// Generate creates a complete GitOps repository based on the provided configuration.
	// The generation process executes in discrete stages with automatic rollback on failure.
	Generate(ctx context.Context, cfg v2.Config) error

	// GenerateDryRun previews the generation process without making filesystem changes.
	// It returns a plan describing what would be generated.
	GenerateDryRun(ctx context.Context, cfg v2.Config) (*GenerationPlan, error)

	// Rollback restores the workspace to a previous checkpoint state.
	// This is typically called automatically on stage failure but can be invoked manually.
	Rollback(ctx context.Context, checkpointID string) error

	// GetWorkspace returns the current workspace being used for generation.
	GetWorkspace() *GitOpsWorkspace

	// SetProgressCallback sets a callback function for progress reporting.
	SetProgressCallback(callback ProgressCallback)
}

// GenerationPlan describes the planned generation operations without executing them.
// This is used for dry-run mode to preview what would be generated.
type GenerationPlan struct {
	// Stages lists all stages that would be executed
	Stages []StagePlan

	// TotalFiles is the estimated number of files that would be generated
	TotalFiles int

	// TotalDirectories is the estimated number of directories that would be created
	TotalDirectories int

	// EstimatedDuration is an estimate of how long generation would take
	EstimatedDuration string

	// Warnings contains any warnings about the planned generation
	Warnings []string
}

// StagePlan describes a single stage in the generation plan.
type StagePlan struct {
	// Name is the stage name
	Name string

	// Description describes what the stage does
	Description string

	// Files lists files that would be created or modified
	Files []string

	// Directories lists directories that would be created
	Directories []string

	// Dependencies lists stages that must complete before this stage
	Dependencies []string
}

// ProgressCallback is called during generation to report progress.
// It receives the current stage name, progress percentage (0-100), and a status message.
type ProgressCallback func(stage string, progress int, message string)

// GenerationResult contains the results of a generation operation.
type GenerationResult struct {
	// Success indicates whether generation completed successfully
	Success bool

	// WorkspaceID is the ID of the workspace used for generation
	WorkspaceID string

	// StagesCompleted is the number of stages that completed successfully
	StagesCompleted int

	// TotalStages is the total number of stages
	TotalStages int

	// FilesGenerated is the number of files created
	FilesGenerated int

	// Error contains any error that occurred during generation
	Error error

	// Duration is how long generation took
	Duration string
}

// GenerationOptions configures the generation process.
type GenerationOptions struct {
	// DryRun enables dry-run mode (no filesystem changes)
	DryRun bool

	// SkipValidation skips validation stages
	SkipValidation bool

	// OutputDir specifies where to generate the repository (overrides default)
	OutputDir string

	// CleanupOnError determines whether to clean up workspace on error
	CleanupOnError bool

	// ProgressCallback is called to report progress
	ProgressCallback ProgressCallback

	// Verbose enables verbose logging
	Verbose bool
}

// DefaultGenerationOptions returns the default generation options.
func DefaultGenerationOptions() GenerationOptions {
	return GenerationOptions{
		DryRun:           false,
		SkipValidation:   false,
		OutputDir:        "",
		CleanupOnError:   true,
		ProgressCallback: nil,
		Verbose:          false,
	}
}

// Validate checks if the generation options are valid.
func (opts GenerationOptions) Validate() error {
	// Add validation logic as needed
	return nil
}

// String returns a string representation of the generation plan.
func (plan *GenerationPlan) String() string {
	return fmt.Sprintf("GenerationPlan{Stages: %d, Files: %d, Directories: %d}",
		len(plan.Stages), plan.TotalFiles, plan.TotalDirectories)
}

// String returns a string representation of the generation result.
func (result *GenerationResult) String() string {
	status := "success"
	if !result.Success {
		status = "failed"
	}
	return fmt.Sprintf("GenerationResult{Status: %s, Stages: %d/%d, Files: %d, Duration: %s}",
		status, result.StagesCompleted, result.TotalStages, result.FilesGenerated, result.Duration)
}
