# Progress Reporting for GitOps Generation


## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Progress Reporter Options](#progress-reporter-options)
- [Integration with Pipeline Generator](#integration-with-pipeline-generator)
- [Output Examples](#output-examples)
- [Stage Icons](#stage-icons)
- [Error Handling](#error-handling)
- [Thread Safety](#thread-safety)
- [CLI Integration](#cli-integration)
- [Testing](#testing)
- [Best Practices](#best-practices)
- [Future Enhancements](#future-enhancements)
This document describes how to use the progress reporting system for GitOps repository generation.

## Overview

The progress reporting system provides real-time feedback to users during GitOps generation operations. It displays:

- Stage names with visual icons
- Progress bars showing completion percentage
- Status messages for each operation
- Elapsed time (in verbose mode)
- Completion summary with file counts

## Quick Start

### Basic Usage

```go
import (
    "os"
    "github.com/rackerlabs/openCenter-cli/internal/gitops"
)

// Create a progress reporter
reporter := gitops.NewDefaultProgressReporter(os.Stdout)

// Create your pipeline generator
generator := gitops.NewPipelineGenerator(workspaceManager, stages)

// Set the progress callback
generator.SetProgressCallback(reporter.Callback())

// Generate with progress reporting
err := generator.Generate(ctx, cfg)
if err != nil {
    reporter.Error(err)
    return err
}

// Report completion
reporter.Complete(duration, filesGenerated)
```

## Progress Reporter Options

### Default Reporter

The default reporter provides colored output with progress bars:

```go
reporter := gitops.NewDefaultProgressReporter(os.Stdout)
```

### Custom Options

For more control, use `NewProgressReporter` with options:

```go
reporter := gitops.NewProgressReporter(gitops.ProgressReporterOptions{
    Writer:    os.Stdout,
    Quiet:     false,      // Set to true to suppress all output
    Verbose:   true,       // Set to true for detailed timing information
    UseColors: true,       // Set to false to disable ANSI colors
})
```

### Simple Reporter

For minimal output (CI/CD environments):

```go
reporter := gitops.NewSimpleProgressReporter(os.Stdout)
```

This only reports stage start and completion, without progress bars.

## Integration with Pipeline Generator

### Using GenerationOptions

```go
options := gitops.GenerationOptions{
    DryRun:           false,
    SkipValidation:   false,
    CleanupOnError:   true,
    ProgressCallback: reporter.Callback(),
    Verbose:          false,
}

generator := gitops.NewPipelineGeneratorWithOptions(
    workspaceManager,
    stages,
    options,
)
```

### Setting Callback After Creation

```go
generator := gitops.NewPipelineGenerator(workspaceManager, stages)
generator.SetProgressCallback(reporter.Callback())
```

## Output Examples

### Normal Mode

```
🚀 Initialization
  [████████████████████] 100% - Initialization complete

📁 Base Structure
  [██████████░░░░░░░░░░]  50% - Copying base files
  [████████████████████] 100% - Base structure complete

🏗️ Infrastructure
  [████████████████████] 100% - Infrastructure complete

✓ Completion
  [████████████████████] 100% - Generation completed

✓ Generation completed successfully!
  Duration: 5.2s
  Files generated: 42
```

### Verbose Mode

```
🚀 Initialization
  [████████████████████] 100% - Initialization complete (elapsed: 100ms)

📁 Base Structure
  [██████████░░░░░░░░░░]  50% - Copying base files (elapsed: 2.5s)
  [████████████████████] 100% - Base structure complete (elapsed: 5.0s)
```

### Quiet Mode

No output is produced (useful for scripting).

### Simple Mode

```
Starting: initialization
Completed: initialization
Starting: base-structure
Completed: base-structure
```

## Stage Icons

The progress reporter uses visual icons for different stages:

- 🚀 `initialization` - Starting generation
- ✓ `completion` - Generation complete
- ✓ `validation` - Validation stage
- 📁 `base`, `base-structure` - Directory structure
- 🏗️ `infrastructure` - Infrastructure templates
- ⚙️ `services`, `config` - Service and configuration stages
- ▶ Default icon for other stages

## Error Handling

When an error occurs during generation:

```go
err := generator.Generate(ctx, cfg)
if err != nil {
    reporter.Error(err)
    // Handle error...
}
```

Output:
```
✗ Generation failed: stage infrastructure failed: template not found
```

## Thread Safety

The progress reporter is thread-safe and can be called from multiple goroutines concurrently. This is important when stages execute operations in parallel.

## CLI Integration

When integrating with CLI commands, consider:

1. **Detect TTY**: Use colors only when output is a terminal
2. **Respect Flags**: Honor `--quiet` and `--verbose` flags
3. **Error Output**: Send errors to stderr, progress to stdout

Example:

```go
import (
    "os"
    "golang.org/x/term"
)

func createProgressReporter(quiet, verbose bool) *gitops.ProgressReporter {
    // Detect if stdout is a terminal
    useColors := term.IsTerminal(int(os.Stdout.Fd()))
    
    return gitops.NewProgressReporter(gitops.ProgressReporterOptions{
        Writer:    os.Stdout,
        Quiet:     quiet,
        Verbose:   verbose,
        UseColors: useColors,
    })
}
```

## Testing

The progress reporter includes comprehensive tests. See `progress_test.go` for examples of:

- Basic progress reporting
- Quiet mode
- Verbose mode
- Thread safety
- Integration scenarios

## Best Practices

1. **Always report completion**: Call `reporter.Complete()` after successful generation
2. **Always report errors**: Call `reporter.Error()` when generation fails
3. **Use appropriate mode**: Quiet for scripts, verbose for debugging, normal for interactive use
4. **Respect user preferences**: Allow users to control verbosity through CLI flags
5. **Test with different outputs**: Verify behavior with TTY and non-TTY outputs

## Future Enhancements

Potential improvements for the progress reporting system:

- Progress estimation based on file counts
- Cancellation support with progress updates
- Progress persistence for long-running operations
- Integration with structured logging systems
- Customizable progress bar styles
