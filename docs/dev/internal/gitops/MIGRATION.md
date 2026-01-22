# GitOps Generation System Migration Guide


## Table of Contents

- [Overview](#overview)
- [Current Status](#current-status)
- [Migration Timeline](#migration-timeline)
- [Code Migration Examples](#code-migration-examples)
- [Feature Flag Usage](#feature-flag-usage)
- [API Reference](#api-reference)
- [Benefits of the New System](#benefits-of-the-new-system)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [Related Documentation](#related-documentation)
- [Questions?](#questions)
- [Version History](#version-history)
## Overview

The openCenter CLI is migrating from a legacy generation system to a new pipeline-based system that provides:

- **Atomic operations** with automatic rollback on failure
- **Progress reporting** for long-running operations  
- **Dry-run mode** for previewing changes without applying them
- **Better error messages** with context and suggestions
- **Workspace management** with checkpointing and recovery
- **Staged execution** with validation at each step

## Current Status

| Component | Status | Notes |
|-----------|--------|-------|
| Legacy System | ✅ Fully Functional | Default, production-ready |
| Compatibility Layer | ✅ Complete | Maintains backward compatibility |
| Pipeline System | 🚧 In Development | Tasks 4.1-4.3 in progress |
| Workspace Management | 🚧 Partial | Basic implementation exists |
| Generation Stages | ⏳ Planned | Task 4.3 |
| MCP Integration | ⏳ Planned | Task 5.1-5.4 |

## Migration Timeline

### Phase 1: Compatibility Layer (Current)
- ✅ All existing code continues to work without modification
- ✅ New unified interface available: `GenerateGitOpsRepository()`
- ✅ Feature flag mechanism in place for testing
- ✅ Comprehensive test coverage

### Phase 2: Pipeline Implementation (In Progress)
- 🚧 Implement workspace management (Task 4.1)
- 🚧 Implement pipeline-based generation (Task 4.2)
- 🚧 Implement generation stages (Task 4.3)
- 🚧 Enable feature flag testing

### Phase 3: Validation & Testing
- ⏳ Comprehensive integration testing
- ⏳ Performance benchmarking
- ⏳ Output validation (ensure identical results)
- ⏳ User acceptance testing

### Phase 4: Gradual Rollout
- ⏳ Enable pipeline system by default
- ⏳ Monitor for issues
- ⏳ Deprecate legacy system
- ⏳ Remove legacy code

## Code Migration Examples

### Before (Legacy System)

```go
// Existing code - still works!
if err := gitops.CopyBase(cfg, true); err != nil {
    return fmt.Errorf("failed to copy base: %w", err)
}

if err := gitops.RenderClusterApps(cfg); err != nil {
    return fmt.Errorf("failed to render cluster apps: %w", err)
}

if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
    return fmt.Errorf("failed to render infrastructure: %w", err)
}
```

### After (Unified Interface)

```go
// New unified interface - recommended for new code
ctx := context.Background()
if err := gitops.GenerateGitOpsRepository(ctx, cfg); err != nil {
    return fmt.Errorf("failed to generate GitOps repository: %w", err)
}
```

### With Options (Advanced)

```go
// Use options for more control
ctx := context.Background()
opts := gitops.GenerationOptions{
    DryRun:  true,  // Preview changes without applying
    Verbose: true,  // Enable detailed logging
}

if err := gitops.GenerateGitOpsRepositoryWithOptions(ctx, cfg, opts); err != nil {
    return fmt.Errorf("failed to generate GitOps repository: %w", err)
}
```

### Using the Wrapper (Deprecated)

```go
// Wrapper interface - for gradual migration
wrapper := gitops.NewLegacyGenerationWrapper(cfg)
if err := wrapper.Generate(); err != nil {
    return fmt.Errorf("failed to generate: %w", err)
}
```

## Feature Flag Usage

The system uses the `OPENCENTER_USE_PIPELINE_GENERATOR` environment variable to control which generation system is used.

### Testing the New System

```bash
# Enable the new pipeline system (when available)
export OPENCENTER_USE_PIPELINE_GENERATOR=true

# Run your commands as normal
openCenter cluster init my-cluster
openCenter cluster render my-cluster

# Disable to revert to legacy system
unset OPENCENTER_USE_PIPELINE_GENERATOR
```

### In Code

```go
// Check which system is active
if gitops.UsePipelineGenerator() {
    fmt.Println("Using new pipeline system")
} else {
    fmt.Println("Using legacy system")
}
```

## API Reference

### GenerateGitOpsRepository

```go
func GenerateGitOpsRepository(ctx context.Context, cfg config.Config) error
```

Unified interface for GitOps repository generation. Automatically selects between legacy and pipeline systems based on feature flag.

**Parameters:**
- `ctx`: Context for cancellation and timeout
- `cfg`: Cluster configuration

**Returns:**
- `error`: Any error that occurred during generation

**Example:**
```go
ctx := context.Background()
if err := gitops.GenerateGitOpsRepository(ctx, cfg); err != nil {
    log.Fatalf("Generation failed: %v", err)
}
```

### GenerateGitOpsRepositoryWithOptions

```go
func GenerateGitOpsRepositoryWithOptions(
    ctx context.Context,
    cfg config.Config,
    opts GenerationOptions,
) error
```

Generate GitOps repository with explicit options for more control.

**Parameters:**
- `ctx`: Context for cancellation and timeout
- `cfg`: Cluster configuration
- `opts`: Generation options

**Returns:**
- `error`: Any error that occurred during generation

**Example:**
```go
opts := gitops.GenerationOptions{
    DryRun:           true,
    SkipValidation:   false,
    OutputDir:        "/custom/path",
    CleanupOnError:   true,
    Verbose:          true,
}

ctx := context.Background()
if err := gitops.GenerateGitOpsRepositoryWithOptions(ctx, cfg, opts); err != nil {
    log.Fatalf("Generation failed: %v", err)
}
```

### GenerationOptions

```go
type GenerationOptions struct {
    DryRun           bool              // Preview without applying changes
    SkipValidation   bool              // Skip validation stages
    OutputDir        string            // Custom output directory
    CleanupOnError   bool              // Clean up workspace on error
    ProgressCallback ProgressCallback  // Progress reporting callback
    Verbose          bool              // Enable verbose logging
}
```

**Methods:**
- `DefaultGenerationOptions()`: Returns default options
- `Validate()`: Validates the options

### LegacyGenerationWrapper (Deprecated)

```go
type LegacyGenerationWrapper struct {
    config config.Config
}

func NewLegacyGenerationWrapper(cfg config.Config) *LegacyGenerationWrapper
func (w *LegacyGenerationWrapper) Generate() error
func (w *LegacyGenerationWrapper) CopyBase(render bool) error
func (w *LegacyGenerationWrapper) RenderClusterApps() error
func (w *LegacyGenerationWrapper) RenderInfrastructureCluster() error
```

Wrapper for legacy generation functions. Deprecated - use `GenerateGitOpsRepository` instead.

## Benefits of the New System

### 1. Atomic Operations
- All file operations are atomic
- Automatic rollback on failure
- No partial writes or corrupted state

### 2. Progress Reporting
```go
opts := gitops.GenerationOptions{
    ProgressCallback: func(stage string, progress int, message string) {
        fmt.Printf("[%s] %d%% - %s\n", stage, progress, message)
    },
}
```

### 3. Dry-Run Mode
```go
opts := gitops.GenerationOptions{
    DryRun: true,
}
// Preview what would be generated without making changes
```

### 4. Better Error Messages
```
Error: Failed to render template 'main.tf.tpl'
  Location: infrastructure/clusters/my-cluster/main.tf
  Line: 42
  Context: Invalid variable reference '${var.missing_var}'
  Suggestion: Add 'missing_var' to your configuration or check for typos
```

### 5. Workspace Management
- Isolated workspace for each generation
- Checkpointing at each stage
- Easy rollback to any checkpoint
- Automatic cleanup

### 6. Staged Execution
1. **Base Structure** - Create directory layout
2. **Infrastructure** - Generate provider-specific templates
3. **Services** - Generate enabled service configurations
4. **Configuration** - Create cluster-specific configs
5. **Validation** - Verify repository completeness

## Testing

### Running Tests

```bash
# Test compatibility layer
go test -v ./internal/gitops -run TestGenerateGitOpsRepository

# Test backward compatibility
go test -v ./internal/gitops -run TestGenerateGitOpsRepositoryBackwardCompatibility

# Test feature flag
go test -v ./internal/gitops -run TestUsePipelineGenerator

# Run all gitops tests
go test -v ./internal/gitops
```

### Writing Tests

```go
func TestMyFeature(t *testing.T) {
    // Create test configuration
    cfg := config.NewDefault("test-cluster")
    cfg.OpenCenter.GitOps.GitDir = t.TempDir()
    
    // Test generation
    ctx := context.Background()
    if err := gitops.GenerateGitOpsRepository(ctx, cfg); err != nil {
        t.Fatalf("Generation failed: %v", err)
    }
    
    // Verify results
    // ...
}
```

## Troubleshooting

### Issue: Generation fails with "workspace not found"
**Solution:** Ensure the GitOps directory is set in your configuration:
```go
cfg.OpenCenter.GitOps.GitDir = "/path/to/gitops"
```

### Issue: Feature flag not working
**Solution:** Check the environment variable is set correctly:
```bash
echo $OPENCENTER_USE_PIPELINE_GENERATOR
# Should output: true
```

### Issue: Different output between legacy and new system
**Solution:** This is a bug - please report it! The systems should produce identical output.

### Issue: Tests failing after migration
**Solution:** Ensure you're using the compatibility layer correctly:
```go
// Old way - still works
gitops.CopyBase(cfg, true)

// New way - recommended
gitops.GenerateGitOpsRepository(ctx, cfg)
```

## Contributing

When contributing to the GitOps generation system:

1. **Maintain backward compatibility** - existing code must continue to work
2. **Add tests** - both unit and integration tests
3. **Update documentation** - keep this guide current
4. **Follow the migration plan** - coordinate with ongoing work
5. **Test both systems** - verify legacy and pipeline systems work

## Related Documentation

- [Design Document](../../.kiro/specs/configuration-system-refactor/design.md)
- [Requirements Document](../../.kiro/specs/configuration-system-refactor/requirements.md)
- [Tasks Document](../../.kiro/specs/configuration-system-refactor/tasks.md)
- [Architecture Overview](../../docs/architecture.md)

## Questions?

For questions or issues:
1. Check this migration guide
2. Review the design document
3. Check existing tests for examples
4. Ask in the team chat or create an issue

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-01-14 | Initial compatibility layer implementation |
| 1.1 | TBD | Pipeline system implementation |
| 2.0 | TBD | Pipeline system becomes default |
| 3.0 | TBD | Legacy system removed |
