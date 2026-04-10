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

package gitops_test

import (
	"context"
	"fmt"
	"os"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
)

// ExampleProgressReporter demonstrates how to use the progress reporter
// with the pipeline generator to provide user feedback during GitOps generation.
func ExampleProgressReporter() {
	// Create a progress reporter that writes to stdout
	reporter := gitops.NewDefaultProgressReporter(os.Stdout)

	// Create a workspace manager and stages (simplified for example)
	workspaceManager := gitops.NewWorkspaceManager("/tmp/gitops-workspace")
	stages := []gitops.GenerationStage{
		// Add your stages here
	}

	// Create pipeline generator with progress callback
	generator := gitops.NewPipelineGenerator(workspaceManager, stages)
	generator.SetProgressCallback(reporter.Callback())

	// Create configuration
	cfg := v2.Config{
		// Configure your cluster here
	}

	// Generate GitOps repository with progress reporting
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)
	if err != nil {
		reporter.Error(err)
		return
	}

	// Report completion
	reporter.Complete(0, 42) // duration and file count would be tracked
}

// ExampleProgressReporter_verbose demonstrates verbose progress reporting
// with detailed timing information.
func ExampleProgressReporter_verbose() {
	// Create a verbose progress reporter
	reporter := gitops.NewProgressReporter(gitops.ProgressReporterOptions{
		Writer:    os.Stdout,
		Verbose:   true,
		UseColors: true,
	})

	// Use the reporter's callback with the generator
	callback := reporter.Callback()

	// Simulate progress updates
	callback("initialization", 0, "Starting GitOps generation")
	callback("initialization", 100, "Initialization complete")

	callback("base-structure", 0, "Creating directory structure")
	callback("base-structure", 50, "Copying base files")
	callback("base-structure", 100, "Base structure complete")

	callback("completion", 100, "Generation completed")
}

// ExampleProgressReporter_quiet demonstrates quiet mode for non-interactive use.
func ExampleProgressReporter_quiet() {
	// Create a quiet progress reporter (no output)
	reporter := gitops.NewProgressReporter(gitops.ProgressReporterOptions{
		Writer: os.Stdout,
		Quiet:  true, // Suppresses all output
	})

	// Progress updates will be silently ignored
	callback := reporter.Callback()
	callback("stage", 50, "This will not be displayed")
}

// ExampleSimpleProgressReporter demonstrates the simple progress reporter
// for minimal output in CI/CD environments.
func ExampleSimpleProgressReporter() {
	// Create a simple progress reporter
	reporter := gitops.NewSimpleProgressReporter(os.Stdout)

	// Use the reporter's callback
	callback := reporter.Callback()

	// Only stage start and completion are reported
	callback("initialization", 0, "Starting")
	callback("initialization", 50, "In progress") // Not displayed
	callback("initialization", 100, "Done")

	// Output:
	// Starting: initialization
	// Completed: initialization
}

// ExamplePipelineGenerator_withProgress demonstrates integrating progress
// reporting into a complete GitOps generation workflow.
func ExamplePipelineGenerator_withProgress() {
	// Create progress reporter
	reporter := gitops.NewProgressReporter(gitops.ProgressReporterOptions{
		Writer:    os.Stdout,
		Verbose:   false,
		UseColors: true,
	})

	// Create workspace manager
	tempDir := "/tmp/gitops-example"
	workspaceManager := gitops.NewWorkspaceManager(tempDir)

	// Create generation stages (simplified)
	stages := []gitops.GenerationStage{
		// Your stages would be defined here
	}

	// Create generator with options including progress callback
	options := gitops.GenerationOptions{
		DryRun:           false,
		SkipValidation:   false,
		CleanupOnError:   true,
		ProgressCallback: reporter.Callback(),
		Verbose:          false,
	}

	generator := gitops.NewPipelineGeneratorWithOptions(workspaceManager, stages, options)

	// Create configuration
	cfg := v2.Config{
		// Configure your cluster
	}

	// Generate with progress reporting
	ctx := context.Background()
	err := generator.Generate(ctx, cfg)
	if err != nil {
		reporter.Error(err)
		fmt.Printf("Generation failed: %v\n", err)
		return
	}

	// Report successful completion
	reporter.Complete(0, 0) // Would use actual duration and file count
}
