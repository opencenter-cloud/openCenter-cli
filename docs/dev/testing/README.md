# Testing Documentation

This directory contains documentation for testing infrastructure and practices.

## Contents

- **[BDD Tests](./bdd-tests.md)** - Behavior-Driven Development test suite documentation
- **[Sandbox Setup](./sandbox-setup.md)** - Setting up test sandbox environments

## Testing Approach

openCenter uses multiple testing strategies:

1. **Unit Tests** - Go standard testing in `internal/` packages
   - Run with: `mise run test`
   - Coverage tracking enabled

2. **BDD Tests** - Gherkin scenarios in `tests/features/`
   - Run with: `mise run godog`
   - WIP scenarios: `mise run godog-wip`

3. **Property-Based Tests** - Generative testing for critical logic
   - Using gopter framework
   - Files named `*_property_test.go`

4. **Integration Tests** - Full workflow validation
   - Files named `*_integration_test.go`

## Test Fixtures

Test data and fixtures are located in:
- `testdata/` - Root-level test fixtures
- Package-specific `testdata/` directories

## Running Tests

```bash
# Run all unit tests
mise run test

# Run BDD tests
mise run godog

# Run WIP scenarios only
mise run godog-wip

# Run specific test
go test ./internal/config -run TestConfigValidation
```

## Writing Tests

Follow these guidelines:
- Use table-driven tests for multiple scenarios
- Keep test fixtures in `testdata/` directories
- Use descriptive test names: `TestFeature_Scenario_ExpectedBehavior`
- Add `@wip` tag to Gherkin scenarios during development

For more details, see [AGENTS.md](../../../AGENTS.md) testing guidelines.
