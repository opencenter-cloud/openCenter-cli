---
doc_type: how-to
---

# Developer Guide


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Prerequisites](#prerequisites)
- [Initial Setup](#initial-setup)
- [Development Workflow](#development-workflow)
- [Project Structure](#project-structure)
- [Common Tasks](#common-tasks)
- [Architecture Overview](#architecture-overview)
- [Testing Strategy](#testing-strategy)
- [Configuration Management](#configuration-management)
- [Error Handling](#error-handling)
- [Logging](#logging)
- [Performance Considerations](#performance-considerations)
- [Security Considerations](#security-considerations)
- [Debugging](#debugging)
- [Contributing](#contributing)
- [Release Process](#release-process)
- [See Also](#see-also)
This guide covers setting up your development environment, building openCenter, running tests, and common development workflows.

## Who this is for

Developers contributing to openCenter or extending it with custom providers and plugins.

## Prerequisites

- **Mise**: Tool version manager ([installation guide](https://mise.jdx.dev/getting-started.html))
- **Git**: Version control
- **Go**: Managed automatically by Mise

## Initial Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/rackerlabs/openCenter-cli.git
   cd openCenter-cli
   ```

2. **Install development tools**:
   ```bash
   mise install
   ```
   
   This installs Go, kubectl, kind, helm, and other tools defined in `.mise.toml`.

3. **Build the CLI**:
   ```bash
   mise run build
   ```
   
   The binary is created at `bin/openCenter` with version metadata from git.

4. **Verify the build**:
   ```bash
   ./bin/openCenter version
   ```

## Development Workflow

### Building

Build the binary with version information:
```bash
mise run build
```

Build for specific platforms:
```bash
mise run build-linux        # Linux AMD64
mise run build-all          # All platforms
```

The build injects version metadata via ldflags:
- `version`: Git tag or "0.0.1"
- `gitCommit`: Current commit hash
- `gitBranch`: Current branch name
- `buildDate`: ISO 8601 timestamp

### Testing

Run unit tests:
```bash
mise run test
```

Run BDD tests:
```bash
mise run godog              # All non-@wip scenarios
mise run godog-wip          # Only @wip scenarios
```

Run specific test suites:
```bash
mise run test-security      # Security component tests
mise run test-integration   # Operational readiness tests
```

Run tests for specific priorities:
```bash
mise run test-priority1     # Config loader tests
mise run test-priority2     # GitOps idempotency tests
```

### Code Quality

Format code:
```bash
mise run fmt
```

Run linter:
```bash
mise run lint
```

Tidy dependencies:
```bash
mise run tidy
```

### Schema Changes

When modifying configuration schema:
```bash
mise run schema-verify
```

This comprehensive task:
1. Builds the project
2. Generates JSON schema
3. Tests cluster init with new schema
4. Runs validation
5. Executes unit and BDD tests
6. Compares with reference schema

### Local Testing Environment

Start local Gitea for testing:
```bash
mise run gitea-up           # Setup and configure
mise run gitea-cleanup      # Cleanup
```

Create Kind cluster for testing:
```bash
mise run kind-cluster-no-cni
```

### Credentials Management

Export credentials for active cluster:
```bash
mise run export-aws-creds       # AWS credentials
mise run export-os-creds        # OpenStack credentials
mise run export-all-creds       # All credentials
```

Setup development environment:
```bash
mise run dev-env-setup          # Export all credentials
mise run dev-env-clean          # Clear credentials
```

## Project Structure

See [architecture.md](./architecture.md) for detailed codebase organization.

Quick overview:
- `cmd/`: CLI commands (Cobra)
- `internal/`: Core implementation
- `tests/`: BDD test scenarios
- `testdata/`: Test fixtures
- `schema/`: Generated JSON schemas
- `docs/`: Documentation

## Common Tasks

### Adding a New Command

1. Create `cmd/<command>_<subcommand>.go`
2. Implement `new<Command><Subcommand>Cmd()` returning `*cobra.Command`
3. Register in parent command's `AddCommand()`
4. Add tests in `tests/features/`
5. Update documentation

### Adding a New Provider

1. Create `internal/cloud/<provider>/`
2. Implement `preflight.go` with provider checks
3. Add provider configuration in `internal/config/types_infrastructure.go`
4. Update schema in `internal/config/schema.go`
5. Add validation in `internal/config/<provider>_validator.go`
6. Add tests

### Adding a New Mise Task

When adding functionality that requires commands:

1. **Always create a mise task** in `.mise.toml`
2. Use descriptive kebab-case names
3. Document the task purpose with a comment
4. Chain related tasks using dependencies

Example:
```toml
[tasks]
# Run integration tests with local cluster
test-integration = [
  "mise run kind-cluster-no-cni",
  "go test ./tests/integration/... -v"
]
```

Never suggest raw commands - always wrap in mise tasks for consistency.

## Architecture Overview

openCenter follows a layered architecture:

**Command Layer** (`cmd/`): Cobra commands, flag parsing, user interaction

**Business Logic** (`internal/`):
- `config/`: Configuration management and validation
- `gitops/`: GitOps repository scaffolding
- `sops/`: Secrets encryption with SOPS/Age
- `cloud/`: Provider-specific integrations
- `provision/`: Infrastructure provisioning (Terraform/Ansible/Pulumi)
- `template/`: Template engine with sandboxing
- `security/`: Input validation, credential masking, audit logging
- `resilience/`: Retry logic, circuit breakers, locking
- `operations/`: Drift detection, backup management

**Testing** (`internal/testing/`): Test framework, generators, mocks

For detailed architecture, see [architecture.md](./architecture.md).

## Testing Strategy

openCenter uses multiple testing approaches:

**Unit Tests**: Standard Go tests in `internal/` packages
- Test individual functions and components
- Run with `mise run test`
- Files: `*_test.go`

**Property-Based Tests**: Generative testing with gopter
- Test properties that should hold for all inputs
- Files: `*_property_test.go`
- Example: `internal/config/migration_property_test.go`

**BDD Tests**: Behavior-driven tests with Godog
- Gherkin scenarios in `tests/features/`
- Run with `mise run godog`
- Use `@wip` tag during development

**Integration Tests**: Full workflow validation
- Files: `*_integration_test.go`
- Test complete command flows

See [testing/README.md](./testing/README.md) for detailed testing guide.

## Configuration Management

Configuration files use organization-based structure:

```
~/.config/openCenter/clusters/
└── <organization>/
    ├── .<cluster>-config.yaml
    ├── infrastructure/
    │   └── clusters/<cluster>/
    ├── applications/
    │   └── overlays/<cluster>/
    └── secrets/
        ├── age/keys/
        └── ssh/
```

The `ConfigManager` handles:
- Loading from YAML
- Schema validation
- Environment variable expansion
- Runtime overrides via `--set` flag
- Migration between schema versions

## Error Handling

Use error wrapping for context:
```go
if err != nil {
    return fmt.Errorf("failed to load cluster: %w", err)
}
```

Exit codes:
- `0`: Success
- `1`: Error occurred

## Logging

Log levels: `debug`, `info`, `warn` (default), `error`

Set via:
- `--log-level` flag
- `--verbose` flag (sets debug)
- `OPENCENTER_DEBUG=1` environment variable

Log formats: `text` (default), `json`, `yaml`

## Performance Considerations

- Configuration loaded once at startup
- Plugin discovery cached
- Template rendering cached when enabled
- Lazy loading where possible
- File operations minimized

## Security Considerations

- Configuration files: 0600 permissions
- Secrets never logged (credential masking)
- SOPS integration for encryption
- Input validation on all user input
- Path traversal prevention
- Template sandboxing prevents code execution

## Debugging

Enable debug mode:
```bash
export OPENCENTER_DEBUG=1
./bin/openCenter cluster validate my-cluster
```

Debug artifacts created in debug mode:
- Detailed logs
- Intermediate files
- Validation reports

## Contributing

See [contributing.md](./contributing.md) for:
- Code style guidelines
- Commit message conventions
- Pull request process
- Review checklist

## Release Process

See [release-process.md](./release-process.md) for:
- Versioning strategy
- Release checklist
- Build and distribution
- Changelog generation

## See Also

- [Architecture Documentation](./architecture.md) - Detailed codebase architecture
- [Testing Guide](./testing/README.md) - Comprehensive testing documentation
- [Contributing Guidelines](./contributing.md) - How to contribute
- [Release Process](./release-process.md) - Release procedures
- [Configuration Reference](../reference/configuration.md) - Configuration schema
- [Plugin Development](../how-to/plugin-development.md) - Creating plugins
