---
id: mise-tasks
title: "Mise Tasks"
sidebar_label: Mise Tasks
description: Complete reference of available mise tasks for development, testing, and building.
doc_type: reference
audience: "developers"
tags: [mise, tasks, build, development]
---

# Mise Tasks

**Purpose:** For developers, provides complete reference of available mise tasks for development, testing, and building.

This reference lists all mise tasks defined in `.mise.toml` for openCenter CLI development.

## Overview

Mise is the task automation tool for openCenter CLI. All development operations use mise tasks instead of raw commands for consistency and discoverability.

**Critical Rule:** Always use `mise run <task>` instead of raw commands (go, terraform, etc.).

**Evidence:** `.kiro/steering/tech.md:1-2`, `.kiro/steering/product.md:23-28`

## Build Tasks

### build

Build binary with version information.

**Usage:**
```bash
mise run build
```

**What it does:**
- Compiles Go code
- Injects version information via ldflags
- Creates binary in `bin/opencenter`

**Output:** `bin/opencenter` (current platform)

**Evidence:** `.kiro/steering/tech.md:52`

### build-linux

Build binary for Linux platform.

**Usage:**
```bash
mise run build-linux
```

**What it does:**
- Cross-compiles for Linux amd64
- Injects version information
- Creates binary in `bin/opencenter-linux-amd64`

**Output:** `bin/opencenter-linux-amd64`

**Evidence:** `.kiro/steering/tech.md:55`

### build-all

Build binaries for the supported release platforms.

**Usage:**
```bash
mise run build-all
```

**What it does:**
- Builds for Linux and macOS
- Creates binaries for amd64 and arm64
- Injects version information

**Output:**
- `bin/opencenter-linux-amd64`
- `bin/opencenter-linux-arm64`
- `bin/opencenter-darwin-amd64`
- `bin/opencenter-darwin-arm64`

For official releases, push a `v*` tag and let `.github/workflows/release.yml` publish the signed artifacts. Use `mise run release` only for local preflight builds.

**Evidence:** `.kiro/steering/tech.md:58`

## Test Tasks

### test

Run the deterministic GA test gate.

**Usage:**
```bash
mise run test
```

**What it does:**
- Runs the default CLI/config/cloud verification lane
- Covers `./internal/config/...`, `./cmd/...`, and `./internal/cloud/...`
- Excludes perf-tagged coverage

### property

Run opt-in property-based coverage.

**Usage:**
```bash
mise run property
```

**What it does:**
- Runs tests selected by `-run "TestProperty"`
- Keeps exploratory/property coverage separate from the default gate

### integration

Run the opt-in integration lane.

**Usage:**
```bash
mise run integration
```

**What it does:**
- Runs integration-oriented command and subsystem tests
- Intended for deeper pre-release verification

### perf

Run perf-tagged checks.

**Usage:**
```bash
mise run perf
```

**What it does:**
- Runs `perf` build-tag coverage such as memory-regression checks
- Keeps noisy or long-running performance tests out of the default lane

### godog

Run BDD tests (non-WIP scenarios).

**Usage:**
```bash
mise run godog
```

**What it does:**
- Runs Gherkin scenarios in `tests/features/`
- Excludes scenarios tagged with `@wip`
- Executes all production-ready scenarios

**Evidence:** `.kiro/steering/tech.md:67`

### godog-wip

Run WIP (work-in-progress) BDD scenarios only.

**Usage:**
```bash
mise run godog-wip
```

**What it does:**
- Runs only scenarios tagged with `@wip`
- Used during feature development
- Excludes production scenarios

**Evidence:** `.kiro/steering/tech.md:70`

## Code Quality Tasks

### fmt

Format code with gofmt.

**Usage:**
```bash
mise run fmt
```

**What it does:**
- Runs `gofmt -w` on all Go files
- Formats code to Go standards
- Must be run before committing

**Evidence:** `.kiro/steering/tech.md:73`

### tidy

Tidy Go module dependencies.

**Usage:**
```bash
mise run tidy
```

**What it does:**
- Runs `go mod tidy`
- Removes unused dependencies
- Updates `go.mod` and `go.sum`

**Evidence:** `.kiro/steering/tech.md:76`

### upgrade-deps

Upgrade all dependencies to latest versions.

**Usage:**
```bash
mise run upgrade-deps
```

**What it does:**
- Updates all dependencies to latest compatible versions
- Runs `go get -u ./...`
- Updates `go.mod` and `go.sum`

**Evidence:** `.kiro/steering/tech.md:79`

## Schema Tasks

### schema

Generate JSON schema from Go types.

**Usage:**
```bash
mise run schema
```

**What it does:**
- Generates `schema/cluster.schema.json`
- Extracts schema from Go struct tags
- Used for configuration validation

**Evidence:** `.kiro/steering/tech.md:82`

### schema-verify

Comprehensive schema verification.

**Usage:**
```bash
mise run schema-verify
```

**What it does:**
- Validates schema structure
- Checks schema completeness
- Verifies schema examples
- Tests schema against fixtures

**Evidence:** `.kiro/steering/tech.md:91`

## Validation Tasks

### validate

Validate cluster configuration.

**Usage:**
```bash
mise run validate
```

**What it does:**
- Validates configuration files
- Checks schema compliance
- Verifies business rules

**Evidence:** `.kiro/steering/tech.md:85`

### preflight

Run preflight checks before deployment.

**Usage:**
```bash
mise run preflight
```

**What it does:**
- Checks system requirements
- Verifies tool installations
- Validates credentials

**Evidence:** `.kiro/steering/tech.md:88`

## Documentation Tasks

### docs-gen

Generate CLI documentation.

**Usage:**
```bash
mise run docs-gen
```

**What it does:**
- Generates command documentation
- Creates markdown files for each command
- Updates CLI reference docs

**Evidence:** `.kiro/steering/tech.md:94`

## Cleanup Tasks

### clean

Clean build artifacts.

**Usage:**
```bash
mise run clean
```

**What it does:**
- Removes `bin/` directory
- Removes temporary files
- Cleans build cache

**Evidence:** `.kiro/steering/tech.md:97`

## Pre-Commit Workflow

Required tasks before committing:

```bash
# 1. Build to verify compilation
mise run build

# 2. Format code
mise run fmt

# 3. Run tests
mise run test

# 4. Run BDD tests (if applicable)
mise run godog
```

**Evidence:** `.kiro/steering/tech.md:103-118`

## Task Chaining

Mise tasks can be chained for complex workflows:

```bash
# Full pre-commit check
mise run fmt && mise run test && mise run godog && mise run build

# Schema update workflow
mise run schema && mise run schema-verify

# Full build and test
mise run build-all && mise run test && mise run godog
```

## Custom Task Creation

To add new tasks, edit `.mise.toml`:

```toml
[tasks]
# Task description
task-name = "command to run"

# Task with multiple commands
multi-step = [
  "command 1",
  "command 2",
  "command 3"
]

# Task with dependencies
dependent-task = { depends = ["task1", "task2"], run = "command" }
```

**Evidence:** `.kiro/steering/tech.md:120-135`

## Environment Variables

Mise tasks can use environment variables:

```bash
# Set environment variable for task
OPENCENTER_CONFIG_DIR=/tmp/config mise run validate

# Or export for multiple tasks
export OPENCENTER_CONFIG_DIR=/tmp/config
mise run validate
mise run build
```

## Task Output

Tasks output to stdout/stderr:

```bash
# Capture output
mise run build > build.log 2>&1

# Pipe output
mise run test | grep PASS

# Quiet mode (suppress output)
mise run build --quiet
```

## Common Task Combinations

### Development Workflow

```bash
# Start development
mise install  # Install tools
mise run build  # Build binary

# Make changes
vim internal/config/manager.go

# Test changes
mise run fmt  # Format
mise run test  # Unit tests
mise run build  # Verify compilation

# Commit
git add .
git commit -m "feat: add new feature"
```

### Schema Update Workflow

```bash
# Update schema
vim internal/config/types.go

# Regenerate schema
mise run schema

# Verify schema
mise run schema-verify

# Test with fixtures
mise run validate

# Commit
git add schema/cluster.schema.json internal/config/types.go
git commit -m "feat: update schema"
```

### Release Workflow

```bash
# Build all platforms
mise run build-all

# Run full test suite
mise run test
mise run godog

# Verify schema
mise run schema-verify

# Tag release
git tag v1.0.0
git push --tags
```

## Troubleshooting

### Task Not Found

**Error:** `Task 'xyz' not found`

**Solution:**

```bash
# List available tasks
mise tasks

# Check .mise.toml for task definition
cat .mise.toml | grep -A 5 "\[tasks\]"
```

### Task Fails

**Error:** Task exits with non-zero code

**Solution:**

```bash
# Run task with verbose output
mise run <task> --verbose

# Check task definition
mise tasks --json | jq '.[] | select(.name == "<task>")'

# Run underlying command directly (for debugging only)
# Check .mise.toml for actual command
```

### Mise Not Installed

**Error:** `mise: command not found`

**Solution:**

```bash
# Install mise
curl https://mise.run | sh

# Or with package manager
brew install mise  # macOS
apt install mise   # Ubuntu

# Verify installation
mise --version
```

## Best Practices

1. **Always use mise tasks:** Never use raw commands in documentation or scripts
2. **Run fmt before commit:** Ensure code is formatted
3. **Run tests before commit:** Catch errors early
4. **Use task chaining:** Combine tasks for complex workflows
5. **Document custom tasks:** Add comments in .mise.toml
6. **Keep tasks simple:** One task, one purpose
7. **Use descriptive names:** `test-integration` not `ti`

## Related Topics

- [Development Setup](../dev/development-setup.md) - Set up development environment
- [Testing Guide](../dev/testing-guide.md) - Write and run tests
- [Build System](../dev/build-system.md) - Understand mise-based build system
- [Contributing](../dev/contributing.md) - Contribute code

---

## Evidence

This reference is based on:

- Mise tasks: `.kiro/steering/tech.md:40-97`
- Pre-commit workflow: `.kiro/steering/tech.md:103-118`
- Task creation: `.kiro/steering/tech.md:120-135`
- Build system: `.kiro/steering/product.md:23-28`
