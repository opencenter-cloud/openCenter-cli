---
doc_type: how-to
---

# Contributing to openCenter


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Style Guidelines](#code-style-guidelines)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Adding New Features](#adding-new-features)
- [Documentation Guidelines](#documentation-guidelines)
- [Common Pitfalls](#common-pitfalls)
- [Getting Help](#getting-help)
- [Code of Conduct](#code-of-conduct)
- [Recognition](#recognition)
- [See Also](#see-also)
This guide covers how to contribute code, documentation, and tests to openCenter.

## Who this is for

Anyone who wants to contribute to openCenter, whether fixing bugs, adding features, or improving documentation.

## Getting Started

### Prerequisites

- **Mise**: Tool version manager ([installation guide](https://mise.jdx.dev/getting-started.html))
- **Git**: Version control
- **GitHub account**: For pull requests

### Fork and Clone

1. **Fork the repository** on GitHub

2. **Clone your fork**:
   ```bash
   git clone https://github.com/your-username/openCenter-cli.git
   cd openCenter-cli
   ```

3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/rackerlabs/openCenter-cli.git
   ```

4. **Install tools**:
   ```bash
   mise install
   ```

5. **Build and test**:
   ```bash
   mise run build
   mise run test
   mise run godog
   ```

## Development Workflow

### Create a Branch

Create a feature branch from `main`:

```bash
git checkout main
git pull upstream main
git checkout -b feature/my-feature
```

Branch naming conventions:
- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation changes
- `refactor/description` - Code refactoring
- `test/description` - Test additions/fixes

### Make Changes

1. **Write code** following style guidelines (see below)

2. **Add tests**:
   - Unit tests for new functions
   - BDD tests for new commands or workflows
   - Property tests for critical logic

3. **Run tests**:
   ```bash
   mise run test
   mise run godog
   ```

4. **Format code**:
   ```bash
   mise run fmt
   ```

5. **Run linter** (if available):
   ```bash
   mise run lint
   ```

### Commit Changes

Use Conventional Commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples**:
```bash
git commit -m "feat(cluster): add support for AWS provider"
git commit -m "fix(config): resolve path resolution bug for Windows"
git commit -m "docs(dev): update architecture documentation"
git commit -m "test(gitops): add property tests for template rendering"
```

**Commit message guidelines**:
- Use present tense ("add feature" not "added feature")
- Use imperative mood ("move cursor to..." not "moves cursor to...")
- First line should be 50 characters or less
- Reference issues and pull requests when relevant

### Push Changes

```bash
git push origin feature/my-feature
```

### Create Pull Request

1. **Go to GitHub** and create a pull request from your branch to `main`

2. **Fill out the PR template**:
   - Clear description of changes
   - Link to related issues
   - Test commands run
   - Screenshots (if UI changes)

3. **Wait for review** and address feedback

## Code Style Guidelines

### Go Code Style

Follow standard Go conventions:

**Formatting**:
- Use `gofmt` (run `mise run fmt`)
- Use tabs for indentation
- Line length: aim for 100 characters, max 120

**Naming**:
- `CamelCase` for exported identifiers
- `mixedCase` for unexported identifiers
- Acronyms should be all caps: `HTTPServer`, `URLPath`
- Interface names: `Reader`, `Writer`, `Manager`

**Comments**:
- Package comment in `doc.go` or first file
- Exported functions must have doc comments
- Doc comments start with the function name
- Use complete sentences

**Example**:
```go
// ConfigManager manages cluster configuration lifecycle.
// It handles loading, validation, and persistence of configuration.
type ConfigManager struct {
    loader    Loader
    validator Validator
}

// Load reads configuration from the specified path.
// It returns an error if the file does not exist or is invalid.
func (cm *ConfigManager) Load(path string) (*Config, error) {
    // Implementation
}
```

**Error Handling**:
- Always check errors
- Wrap errors with context using `fmt.Errorf` with `%w`
- Return errors, don't panic (except in init or truly unrecoverable situations)

```go
if err := loader.Load(path); err != nil {
    return fmt.Errorf("failed to load config from %s: %w", path, err)
}
```

**Testing**:
- Test files: `*_test.go`
- Test functions: `TestFunctionName`
- Use table-driven tests for multiple scenarios
- Use `t.Helper()` in test helpers

```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  Config
        wantErr bool
    }{
        {
            name:    "valid config",
            config:  validConfig(),
            wantErr: false,
        },
        {
            name:    "missing cluster name",
            config:  configWithoutName(),
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validator.Validate(tt.config)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Documentation Style

Follow Diátaxis framework (see [documentation.md](../../.kiro/steering/documentation.md)):

- **Tutorials**: Learning-oriented, step-by-step
- **How-to guides**: Task-oriented, problem-solving
- **Reference**: Information-oriented, precise
- **Explanation**: Understanding-oriented, conceptual

**Markdown conventions**:
- Use ATX-style headers (`#`, `##`, `###`)
- Code blocks with language identifiers
- Use relative links for internal docs
- Include `doc_type` metadata at top

### Commit Message Style

**Structure**:
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Subject line**:
- 50 characters or less
- Lowercase (except proper nouns)
- No period at end
- Imperative mood

**Body** (optional):
- Wrap at 72 characters
- Explain what and why, not how
- Separate from subject with blank line

**Footer** (optional):
- Reference issues: `Fixes #123`, `Closes #456`
- Breaking changes: `BREAKING CHANGE: description`

## Testing Requirements

### Unit Tests

Required for:
- New functions in `internal/` packages
- Bug fixes
- Refactoring

Run with:
```bash
mise run test
```

### BDD Tests

Required for:
- New commands
- New workflows
- User-facing features

Add scenarios to `tests/features/*.feature`:

```gherkin
Feature: Cluster Initialization
  Scenario: Initialize cluster with custom organization
    When I run "openCenter cluster init my-cluster --opencenter.meta.organization=my-org"
    Then a cluster configuration "my-cluster" should exist
    And the cluster configuration "my-cluster" should have "opencenter.meta.organization" set to "my-org"
```

Run with:
```bash
mise run godog
```

Use `@wip` tag during development:
```bash
mise run godog-wip
```

### Property-Based Tests

Recommended for:
- Data transformations
- Serialization/deserialization
- Configuration migration
- Complex validation logic

Example:
```go
func TestConfigMigrationProperty(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("migration preserves cluster name", prop.ForAll(
        func(cfg config.Config) bool {
            migrated := migrator.Migrate(cfg)
            return cfg.OpenCenter.Meta.Name == migrated.OpenCenter.Meta.Name
        },
        generators.Config(),
    ))
    
    properties.TestingRun(t)
}
```

### Test Coverage

Aim for:
- 80%+ coverage for new code
- 100% coverage for critical paths (security, validation)
- All exported functions tested

Check coverage:
```bash
go test -cover ./internal/...
```

## Pull Request Process

### Before Submitting

**Checklist**:
- [ ] Code follows style guidelines
- [ ] Tests added and passing
- [ ] Documentation updated
- [ ] Commit messages follow conventions
- [ ] Code formatted (`mise run fmt`)
- [ ] No linter errors (`mise run lint`)
- [ ] Branch is up to date with main

### PR Description

Include:
- **Summary**: What does this PR do?
- **Motivation**: Why is this change needed?
- **Changes**: List of changes made
- **Testing**: How was this tested?
- **Related Issues**: Link to issues

**Template**:
```markdown
## Summary
Brief description of changes

## Motivation
Why this change is needed

## Changes
- Change 1
- Change 2
- Change 3

## Testing
- [ ] Unit tests pass (`mise run test`)
- [ ] BDD tests pass (`mise run godog`)
- [ ] Manual testing performed

## Related Issues
Fixes #123
```

### Review Process

1. **Automated checks** run (tests, linting)
2. **Maintainer review** (1-2 reviewers)
3. **Address feedback** by pushing new commits
4. **Approval** from maintainer
5. **Merge** (squash and merge)

### Addressing Feedback

- Push new commits to your branch
- Don't force-push after review starts
- Respond to comments
- Mark conversations as resolved when addressed

## Adding New Features

### New Command

1. **Create command file**: `cmd/<command>_<subcommand>.go`
2. **Implement command**:
   ```go
   func newClusterMyCommandCmd() *cobra.Command {
       cmd := &cobra.Command{
           Use:   "my-command",
           Short: "Brief description",
           Long:  "Detailed description",
           RunE:  runClusterMyCommand,
       }
       return cmd
   }
   ```
3. **Register command**: Add to parent in `cmd/<command>.go`
4. **Add tests**: BDD scenarios in `tests/features/`
5. **Update docs**: Add to reference documentation

### New Provider

1. **Create provider package**: `internal/cloud/<provider>/`
2. **Implement preflight checks**: `preflight.go`
3. **Add configuration types**: `internal/config/types_infrastructure.go`
4. **Update schema**: `internal/config/schema.go`
5. **Add validation**: `internal/config/<provider>_validator.go`
6. **Add provisioning**: `internal/provision/<provider>/`
7. **Add tests**: Unit and BDD tests
8. **Update docs**: Provider-specific documentation

### New Mise Task

When adding functionality requiring commands:

1. **Add task to `.mise.toml`**:
   ```toml
   [tasks]
   # Run integration tests with local cluster
   test-integration = [
     "mise run kind-cluster-no-cni",
     "go test ./tests/integration/... -v"
   ]
   ```

2. **Document the task** in relevant documentation

3. **Never suggest raw commands** - always use mise tasks

## Documentation Guidelines

### When to Update Docs

Update documentation when:
- Adding new commands
- Changing command behavior
- Adding new configuration options
- Changing workflows
- Adding new providers
- Fixing bugs that affect documented behavior

### Where to Update

- **Reference docs** (`docs/reference/`): Command reference, configuration schema
- **How-to guides** (`docs/how-to/`): Task-oriented guides
- **Developer docs** (`docs/dev/`): Architecture, contributing, testing
- **README**: High-level overview, quick start

### Documentation Style

- Use clear, concise language
- Include code examples
- Use relative links for internal docs
- Test all commands and examples
- Follow Diátaxis framework

## Common Pitfalls

### Don't

- ❌ Commit directly to `main`
- ❌ Force-push after review starts
- ❌ Include unrelated changes in PR
- ❌ Skip tests
- ❌ Ignore linter warnings
- ❌ Use raw commands instead of mise tasks
- ❌ Commit secrets or credentials
- ❌ Break existing tests

### Do

- ✅ Create feature branches
- ✅ Write tests for new code
- ✅ Follow code style guidelines
- ✅ Use Conventional Commits
- ✅ Update documentation
- ✅ Use mise tasks for all commands
- ✅ Encrypt secrets with SOPS
- ✅ Keep PRs focused and small

## Getting Help

- **Questions**: Open a discussion on GitHub
- **Bugs**: Open an issue with reproduction steps
- **Features**: Open an issue with use case description
- **Security**: Email security@rackspace.com (do not open public issue)

## Code of Conduct

Be respectful and professional:
- Welcome newcomers
- Be patient with questions
- Provide constructive feedback
- Focus on the code, not the person
- Assume good intentions

## Recognition

Contributors are recognized in:
- Release notes
- Contributors list
- Git history

Thank you for contributing to openCenter!

## See Also

- [Developer Guide](./README.md) - Development setup and workflows
- [Architecture Documentation](./architecture.md) - Codebase architecture
- [Testing Guide](./testing/README.md) - Testing strategies
- [Release Process](./release-process.md) - Release procedures
