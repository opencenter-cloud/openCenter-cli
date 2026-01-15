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

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// usePipelineGenerator checks if the new pipeline-based generation system should be used.
// This delegates to the centralized feature flag system.
func usePipelineGenerator() bool {
	return config.UsePipelineGenerator()
}

// GenerateGitOpsRepository is a compatibility wrapper that provides a unified interface
// for GitOps repository generation. It automatically selects between the legacy system
// and the new pipeline-based system based on the feature flag.
//
// This function maintains backward compatibility while allowing gradual migration to
// the new pipeline system. When OPENCENTER_USE_PIPELINE_GENERATOR=true, it uses the
// new PipelineGenerator. Otherwise, it uses the legacy generation functions.
//
// Usage:
//
//	if err := gitops.GenerateGitOpsRepository(ctx, cfg); err != nil {
//	    return fmt.Errorf("failed to generate GitOps repository: %w", err)
//	}
//
// Legacy equivalent:
//
//	if err := gitops.CopyBase(cfg, true); err != nil {
//	    return err
//	}
//	if err := gitops.RenderClusterApps(cfg); err != nil {
//	    return err
//	}
//	if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
//	    return err
//	}
func GenerateGitOpsRepository(ctx context.Context, cfg config.Config) error {
	if usePipelineGenerator() {
		// Use new pipeline-based generation system
		// TODO: Implement when PipelineGenerator is available (Task 4.2)
		// generator := NewPipelineGenerator()
		// return generator.Generate(ctx, cfg)
		
		// For now, fall back to legacy system even when flag is set
		// This will be updated once the pipeline system is implemented
		return generateGitOpsRepositoryLegacy(cfg)
	}

	// Use legacy generation system
	return generateGitOpsRepositoryLegacy(cfg)
}

// generateGitOpsRepositoryLegacy uses the existing generation functions to create
// a GitOps repository. This maintains the current behavior and serves as the
// compatibility layer during migration.
func generateGitOpsRepositoryLegacy(cfg config.Config) error {
	// Copy base GitOps structure (always render for generation)
	if err := CopyBase(cfg, true); err != nil {
		return err
	}

	// Render cluster-specific applications
	if err := RenderClusterApps(cfg); err != nil {
		return err
	}

	// Render infrastructure templates
	if err := RenderInfrastructureCluster(cfg); err != nil {
		return err
	}

	return nil
}

// GenerateGitOpsRepositoryWithOptions provides more control over the generation process
// with explicit options. This is useful for commands that need specific behavior like
// dry-run mode or custom output directories.
//
// When the pipeline system is enabled, this uses GenerationOptions. Otherwise, it
// uses the legacy system with the provided render flag.
//
// Usage:
//
//	opts := gitops.GenerationOptions{
//	    DryRun: true,
//	    Verbose: true,
//	}
//	if err := gitops.GenerateGitOpsRepositoryWithOptions(ctx, cfg, opts); err != nil {
//	    return err
//	}
func GenerateGitOpsRepositoryWithOptions(ctx context.Context, cfg config.Config, opts GenerationOptions) error {
	if usePipelineGenerator() {
		// Use new pipeline-based generation system with options
		// TODO: Implement when PipelineGenerator is available (Task 4.2)
		// generator := NewPipelineGenerator()
		// generator.SetOptions(opts)
		// if opts.DryRun {
		//     plan, err := generator.GenerateDryRun(ctx, cfg)
		//     if err != nil {
		//         return err
		//     }
		//     // Print plan details
		//     fmt.Println(plan.String())
		//     return nil
		// }
		// return generator.Generate(ctx, cfg)
		
		// For now, fall back to legacy system
		return generateGitOpsRepositoryLegacy(cfg)
	}

	// Use legacy generation system
	// Note: Legacy system doesn't support all options, so we just use the basic flow
	return generateGitOpsRepositoryLegacy(cfg)
}

// LegacyGenerationWrapper wraps the existing generation functions to maintain
// backward compatibility. This allows existing code to continue working without
// modification while we migrate to the new pipeline system.
//
// Deprecated: Use GenerateGitOpsRepository instead. This wrapper exists only for
// backward compatibility during the migration period.
type LegacyGenerationWrapper struct {
	config config.Config
}

// NewLegacyGenerationWrapper creates a new wrapper for legacy generation functions.
//
// Deprecated: Use GenerateGitOpsRepository instead.
func NewLegacyGenerationWrapper(cfg config.Config) *LegacyGenerationWrapper {
	return &LegacyGenerationWrapper{config: cfg}
}

// Generate executes the legacy generation flow.
//
// Deprecated: Use GenerateGitOpsRepository instead.
func (w *LegacyGenerationWrapper) Generate() error {
	return generateGitOpsRepositoryLegacy(w.config)
}

// CopyBase wraps the existing CopyBase function for backward compatibility.
//
// Deprecated: Use GenerateGitOpsRepository instead.
func (w *LegacyGenerationWrapper) CopyBase(render bool) error {
	return CopyBase(w.config, render)
}

// RenderClusterApps wraps the existing RenderClusterApps function for backward compatibility.
//
// Deprecated: Use GenerateGitOpsRepository instead.
func (w *LegacyGenerationWrapper) RenderClusterApps() error {
	return RenderClusterApps(w.config)
}

// RenderInfrastructureCluster wraps the existing RenderInfrastructureCluster function for backward compatibility.
//
// Deprecated: Use GenerateGitOpsRepository instead.
func (w *LegacyGenerationWrapper) RenderInfrastructureCluster() error {
	return RenderInfrastructureCluster(w.config)
}

// MigrationGuide provides documentation for migrating from legacy to pipeline-based generation.
const MigrationGuide = `
GitOps Generation Migration Guide
==================================

The openCenter CLI is migrating from a legacy generation system to a new pipeline-based
system that provides better error handling, rollback capabilities, and progress reporting.

Current Status:
- Legacy system: Fully functional (default)
- Pipeline system: In development (Tasks 4.1-4.3)

Migration Path:
1. All existing code continues to work without modification
2. Set OPENCENTER_USE_PIPELINE_GENERATOR=true to test the new system (when available)
3. Once validated, the new system will become the default
4. Legacy system will be deprecated and eventually removed

Code Migration:

Before (Legacy):
    if err := gitops.CopyBase(cfg, true); err != nil {
        return err
    }
    if err := gitops.RenderClusterApps(cfg); err != nil {
        return err
    }
    if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
        return err
    }

After (Unified):
    if err := gitops.GenerateGitOpsRepository(ctx, cfg); err != nil {
        return err
    }

With Options:
    opts := gitops.GenerationOptions{
        DryRun: true,
        Verbose: true,
    }
    if err := gitops.GenerateGitOpsRepositoryWithOptions(ctx, cfg, opts); err != nil {
        return err
    }

Benefits of New System:
- Atomic operations with automatic rollback on failure
- Progress reporting for long-running operations
- Dry-run mode for previewing changes
- Better error messages with context
- Workspace management with checkpointing
- Staged execution with validation

For more information, see:
- Design document: .kiro/specs/configuration-system-refactor/design.md
- Tasks document: .kiro/specs/configuration-system-refactor/tasks.md
`
