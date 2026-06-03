---
id: contributing
title: "Contributing to openCenter-cli"
sidebar_label: Contributing to openCenter-cli
description: How to contribute code, tests, and documentation to openCenter-cli, covering setup through pull request review.
doc_type: how-to
audience: "contributors"
tags: [contributing]
---
# Contributing to openCenter-cli

**Purpose:** For contributors, shows how to contribute code, tests, and documentation to the openCenter-cli project.

## Prerequisites

Before contributing, you need:

* Git installed and configured
* GitHub account with SSH key configured
* Mise installed (see [Development Setup](development-setup.md))
* Familiarity with Go and Kubernetes concepts

## Fork and Clone

1. Fork the repository on GitHub:

   ```bash
   # Navigate to https://github.com/opencenter-cloud/openCenter-cli
   # Click "Fork" button
   ```
2. Clone your fork:

   ```bash
   git clone git@github.com:YOUR-USERNAME/openCenter-cli.git
   cd openCenter-cli
   ```
3. Add upstream remote:

   ```bash
   git remote add upstream git@github.com:opencenter-cloud/openCenter-cli.git
   ```
4. Install development tools:

   ```bash
   mise install
   ```
5. Build the project:

   ```bash
   mise run build
   ```

## Making Changes

### Create a Branch

Always create a feature branch for your changes:

```bash
# Update your fork
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feat/my-feature
```

Branch naming conventions:

* `feat/` - New features
* `fix/` - Bug fixes
* `docs/` - Documentation changes
* `refactor/` - Code refactoring
* `test/` - Test additions or fixes

### Write Code

Follow the coding standards defined in `.kiro/steering/tech.md`:

1. **Formatting**: Code must be formatted with `gofmt`

   ```bash
   mise run fmt
   ```
2. **Import organization**: Standard library → External → Internal

   ```go
   import (
       "fmt"
       "os"

       "github.com/spf13/cobra"
       "gopkg.in/yaml.v3"

       "github.com/opencenter-cloud/opencenter-cli/internal/config"
   )
   ```
3. **Naming conventions**:
   * Exported: `CamelCase`
   * Unexported: `mixedCase`
   * Test functions: `TestXxx`
   * Commands: `newCluster<Action>Cmd()`
4. **Error handling**: Always wrap errors with context

   ```go
   if err != nil {
       return fmt.Errorf("failed to load config: %w", err)
   }
   ```

### Write Tests

All behavior changes require tests. See [Testing Guide](testing-guide.md) for details.

**Required tests:**

* Unit tests for new functions (`*_test.go`)
* BDD tests for user-facing features (`tests/features/*.feature`)
* Property tests for critical logic (`*_property_test.go`)

**Run tests before committing:**

```bash
# Unit tests
mise run test

# BDD tests
mise run godog

# All tests
mise run test && mise run godog
```

### Update Documentation

Documentation changes are required for:

* New commands or flags
* Configuration schema changes
* New features or workflows
* Breaking changes

Update relevant files in `docs/`:

* `docs/getting-started/` - Learning-oriented guides
* `docs/operations/` - Task-oriented guides
* `docs/reference/` - Information-oriented specs
* `docs/concepts/` - Understanding-oriented concepts

## Pre-Commit Checklist

Before committing, always run:

```bash
# 1. Build to verify compilation
mise run build

# 2. Format code
mise run fmt

# 3. Run tests
mise run test

# 4. Run BDD tests
mise run godog

# 5. Tidy dependencies (if you added/removed imports)
mise run tidy
```

If you modified configuration schema:

```bash
# Comprehensive schema verification
mise run schema-verify
```

## Commit Messages

Use Conventional Commits format:

```
<type>: <description>

[optional body]

[optional footer]
```

**Types:**

* `feat:` - New feature
* `fix:` - Bug fix
* `docs:` - Documentation changes
* `refactor:` - Code refactoring
* `test:` - Test additions or fixes
* `chore:` - Build process or tooling changes

**Examples:**

```
feat: add support for AWS provider

Implements AWS provider with EC2 instance provisioning,
VPC configuration, and IAM role management.

Closes #123
```

```
fix: correct VRRP validation logic

When use_octavia=false and vrrp_enabled=true, vrrp_ip
must be set. Previous validation was too permissive.

Fixes #456
```

```
docs: add tutorial for VMware deployment

Adds step-by-step guide for deploying clusters on
pre-provisioned VMware VMs.
```

## Submit Pull Request

1. Push your branch to your fork:

   ```bash
   git push origin feat/my-feature
   ```
2. Create pull request on GitHub:
   * Navigate to https://github.com/opencenter-cloud/openCenter-cli
   * Click "New Pull Request"
   * Select your fork and branch
   * Fill in PR template
3. PR description must include:
   * What changed and why
   * Test commands run
   * Documentation updates
   * Breaking changes (if any)
   * Related issues

**Example PR description:**

```markdown
## Changes

Adds support for AWS provider with EC2 provisioning.

## Testing

- `mise run test` - All unit tests pass
- `mise run godog` - All BDD tests pass
- Manual testing with AWS account in us-east-1

## Documentation

- Added `docs/getting-started/aws-deployment.md`
- Updated `docs/reference/providers.md`
- Updated `docs/reference/configuration-schema.md`

## Breaking Changes

None

## Related Issues

Closes #123
```

## Code Review Process

1. **Automated checks**: CI runs tests and linting
2. **Maintainer review**: At least one maintainer approval required
3. **Address feedback**: Make requested changes
4. **Merge**: Maintainer merges when approved

**Responding to feedback:**

```bash
# Make requested changes
git add .
git commit -m "fix: address review feedback"
git push origin feat/my-feature
```

## Common Contribution Types

### Adding a New Command

See [Code Structure](code-structure.md) for details.

1. Create `cmd/cluster_<action>.go`
2. Implement `newCluster<Action>Cmd()` function
3. Register in `cmd/cluster.go`
4. Add BDD tests in `tests/features/`
5. Update `docs/reference/cli-commands.md`

### Adding a New Provider

See [Adding Providers](adding-providers.md) for details.

1. Create `internal/cloud/<provider>/` directory
2. Implement preflight checks
3. Add provider defaults in `internal/config/defaults.go`
4. Add provider validation
5. Update documentation

### Adding a New Service

See [Adding Services](adding-services.md) for details.

1. Add service to `internal/config/defaults.go`
2. Create templates in `internal/gitops/gitops-base-dir/`
3. Add service validation
4. Update `docs/reference/platform-services.md`

### Fixing a Bug

1. Create failing test that reproduces bug
2. Fix the bug
3. Verify test now passes
4. Add regression test if needed

### Improving Documentation

1. Identify documentation gap or error
2. Update relevant documentation files
3. Follow Diátaxis framework (tutorial/how-to/reference/explanation)
4. Include evidence citations where applicable

## Getting Help

**Questions about contributing:**

* Open a discussion on GitHub
* Ask in pull request comments
* Review existing issues and PRs

**Found a bug:**

* Search existing issues first
* Open new issue with reproduction steps
* Include version: `opencenter version`

**Feature requests:**

* Open issue with "enhancement" label
* Describe use case and proposed solution
* Discuss before implementing large features

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (check LICENSE file in repository root).

---

## Evidence

This documentation is based on the following repository files:

* Contributing guide: `CONTRIBUTING.md:1-82`
* Development workflow: `.kiro/steering/tech.md:103-118`
* Coding standards: `.kiro/steering/tech.md:5-24`
* Commit conventions: `.kiro/steering/tech.md:26-29`
* Project structure: `.kiro/steering/structure.md:1-128`
* Build system: `.mise.toml:1-961` (mise tasks)
* Command structure: `cmd/` directory (70+ command files)