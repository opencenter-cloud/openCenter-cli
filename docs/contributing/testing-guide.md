---
id: testing-guide
title: "Testing Guide"
sidebar_label: Testing Guide
description: Write and run unit, BDD, and property-based tests for openCenter-cli.
doc_type: how-to
audience: "developers"
tags: [contributing]
---
# Testing Guide

**Purpose:** For developers, shows how to write and run tests for openCenter-cli.

## Test Types

openCenter-cli uses four types of tests:

1. **Unit Tests** - Test individual functions and packages
2. **BDD Tests** - Test user-facing workflows with Gherkin scenarios
3. **Property Tests** - Test invariants with generated inputs
4. **Integration Tests** - Test complete workflows end-to-end

## Running Tests

### Run All Unit Tests

```bash
# Run all unit tests in internal/ packages
mise run test
```

Expected output: All tests pass (1-2 minutes)

### Run BDD Tests

```bash
# Run all BDD scenarios (excluding @wip)
mise run godog
```

Expected output: All scenarios pass (2-3 minutes)

### Run WIP Scenarios Only

```bash
# Run only @wip tagged scenarios during development
mise run godog-wip
```

Use `@wip` tag for scenarios you’re actively working on.

### Run Property Tests

```bash
# Run all property-based tests
mise run test-properties
```

### Run Specific Test Suites

```bash
# Security tests only
mise run test-security

# Integration tests only
mise run test-integration

# V2 configuration tests only
mise run test-v2
```

### Run Tests for Specific Package

```bash
# Test single package
go test -v ./internal/config

# Test with coverage
go test -v -cover ./internal/config

# Test specific function
go test -v ./internal/config -run TestValidation
```

## Writing Unit Tests

### Test File Structure

Create test files alongside source files:

```
internal/config/
├── config.go
├── config_test.go          # Unit tests
└── config_property_test.go # Property tests
```

### Basic Unit Test

```go
// internal/config/validator_test.go
package config

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestValidateClusterName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {
            name:    "valid cluster name",
            input:   "my-cluster",
            wantErr: false,
        },
        {
            name:    "invalid characters",
            input:   "my_cluster!",
            wantErr: true,
        },
        {
            name:    "too long",
            input:   "this-cluster-name-is-way-too-long-and-exceeds-limits",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateClusterName(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Test with Fixtures

```go
func TestLoadConfig(t *testing.T) {
    // Load test fixture
    data, err := os.ReadFile("testdata/valid-config.yaml")
    assert.NoError(t, err)

    // Parse configuration
    var cfg Config
    err = yaml.Unmarshal(data, &cfg)
    assert.NoError(t, err)

    // Verify expected values
    assert.Equal(t, "test-cluster", cfg.ClusterName)
    assert.Equal(t, "openstack", cfg.Provider)
}
```

### Test with Mocks

```go
type mockCloudProvider struct {
    preflightCalled bool
    preflightErrors []string
}

func (m *mockCloudProvider) Preflight(config map[string]any) []string {
    m.preflightCalled = true
    return m.preflightErrors
}

func TestPreflightCheck(t *testing.T) {
    mock := &mockCloudProvider{
        preflightErrors: []string{"auth failed"},
    }

    errors := RunPreflight(mock, map[string]any{})

    assert.True(t, mock.preflightCalled)
    assert.Len(t, errors, 1)
    assert.Contains(t, errors[0], "auth failed")
}
```

## Writing BDD Tests

### Feature File Structure

Create feature files in `tests/features/`:

```gherkin
# tests/features/cluster_init.feature
Feature: Cluster Initialization
  As a platform engineer
  I want to initialize cluster configurations
  So that I can deploy Kubernetes clusters

  Background:
    Given I have a clean test environment

  Scenario: Initialize cluster with defaults
    When I run "opencenter cluster init demo --org my-org"
    Then the command should succeed
    And a configuration file should exist at "my-org/.demo-config.yaml"
    And the configuration should have provider "openstack"

  Scenario: Initialize cluster with custom provider
    When I run "opencenter cluster init demo --org my-org --type aws"
    Then the command should succeed
    And the configuration should have provider "aws"

  @wip
  Scenario: Initialize cluster with invalid name
    When I run "opencenter cluster init invalid_name --org my-org"
    Then the command should fail
    And the error should contain "invalid cluster name"
```

### Step Definitions

Implement steps in `tests/features/steps/`:

```go
// tests/features/steps/cluster_steps.go
package steps

import (
    "github.com/cucumber/godog"
)

func (s *TestSuite) iRunCommand(cmd string) error {
    s.lastCommand = cmd
    s.lastOutput, s.lastError = s.runCommand(cmd)
    return nil
}

func (s *TestSuite) theCommandShouldSucceed() error {
    if s.lastError != nil {
        return fmt.Errorf("command failed: %v\nOutput: %s",
            s.lastError, s.lastOutput)
    }
    return nil
}

func (s *TestSuite) aConfigurationFileShouldExistAt(path string) error {
    fullPath := filepath.Join(s.configDir, path)
    if _, err := os.Stat(fullPath); os.IsNotExist(err) {
        return fmt.Errorf("configuration file not found: %s", fullPath)
    }
    return nil
}

func InitializeScenario(ctx *godog.ScenarioContext) {
    suite := &TestSuite{}

    ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
        return ctx, suite.setup()
    })

    ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
        return ctx, suite.teardown()
    })

    ctx.Step(`^I run "([^"]*)"$`, suite.iRunCommand)
    ctx.Step(`^the command should succeed$`, suite.theCommandShouldSucceed)
    ctx.Step(`^a configuration file should exist at "([^"]*)"$`,
        suite.aConfigurationFileShouldExistAt)
}
```

### Running Specific Scenarios

```bash
# Run scenarios with specific tag
go test ./... -v args --godog.tags=@priority1 --godog.paths=tests/features

# Run specific feature file
go test ./... -v args --godog.paths=tests/features/cluster_init.feature

# Run scenario by name (partial match)
go test ./... -v args --godog.tags="@cluster_init"
```

## Writing Property Tests

Property tests verify invariants hold for many generated inputs.

### Basic Property Test

```go
// internal/config/validator_property_test.go
package config

import (
    "testing"
    "github.com/leanovate/gopter"
    "github.com/leanovate/gopter/gen"
    "github.com/leanovate/gopter/prop"
)

func TestPropertyClusterNameValidation(t *testing.T) {
    properties := gopter.NewProperties(nil)

    properties.Property("valid cluster names always pass validation",
        prop.ForAll(
            func(name string) bool {
                // Generate valid cluster name
                validName := generateValidClusterName(name)
                err := ValidateClusterName(validName)
                return err == nil
            },
            gen.AlphaString(),
        ))

    properties.Property("names with invalid characters always fail",
        prop.ForAll(
            func(name string) bool {
                // Add invalid character
                invalidName := name + "!"
                err := ValidateClusterName(invalidName)
                return err != nil
            },
            gen.AlphaString().SuchThat(func(s string) bool {
                return len(s) > 0 && len(s) < 50
            }),
        ))

    properties.TestingRun(t)
}
```

### Property Test for Idempotency

```go
func TestPropertyConfigMarshalUnmarshal(t *testing.T) {
    properties := gopter.NewProperties(nil)

    properties.Property("marshal then unmarshal preserves config",
        prop.ForAll(
            func(cfg *Config) bool {
                // Marshal to YAML
                data, err := yaml.Marshal(cfg)
                if err != nil {
                    return false
                }

                // Unmarshal back
                var cfg2 Config
                err = yaml.Unmarshal(data, &cfg2)
                if err != nil {
                    return false
                }

                // Compare (should be equal)
                return reflect.DeepEqual(cfg, &cfg2)
            },
            genConfig(),
        ))

    properties.TestingRun(t)
}

// Generator for Config struct
func genConfig() gopter.Gen {
    return gopter.CombineGens(
        gen.AlphaString(),
        gen.OneConstOf("openstack", "aws", "vmware"),
        gen.IntRange(1, 10),
    ).Map(func(values []interface{}) *Config {
        return &Config{
            ClusterName: values[0].(string),
            Provider:    values[1].(string),
            MasterCount: values[2].(int),
        }
    })
}
```

## Test Coverage

### Generate Coverage Report

```bash
# Run tests with coverage
go test -v -coverprofile=coverage.out ./internal/...

# View coverage in terminal
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
```

### Coverage Targets

Aim for:

* **Critical paths**: 90%+ coverage (validation, security, secrets)
* **Business logic**: 80%+ coverage (config, gitops, providers)
* **Utilities**: 70%+ coverage (helpers, formatters)

## Test Best Practices

### Do

* **Test behavior, not implementation** - Test what the code does, not how
* **Use table-driven tests** - Test multiple cases efficiently
* **Test edge cases** - Empty strings, nil values, boundary conditions
* **Test error paths** - Verify errors are returned correctly
* **Use descriptive test names** - `TestValidateClusterName_WithInvalidCharacters`
* **Keep tests fast** - Unit tests should run in milliseconds
* **Use fixtures** - Store test data in `testdata/` directory
* **Clean up after tests** - Remove temporary files and directories

### Don’t

* **Don’t test external services** - Mock cloud providers, APIs
* **Don’t test third-party libraries** - Trust they work
* **Don’t make tests depend on each other** - Each test should be independent
* **Don’t use real credentials** - Use test fixtures or mocks
* **Don’t skip cleanup** - Always clean up in `defer` or `After` hooks
* **Don’t test private functions directly** - Test through public API

## Debugging Tests

### Run Single Test with Verbose Output

```bash
go test -v ./internal/config -run TestValidateClusterName
```

### Run with Debug Logging

```bash
OPENCENTER_DEBUG=true go test -v ./internal/config
```

### Use Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug test
dlv test ./internal/config -- -test.run TestValidateClusterName
```

### Print Debug Information

```go
func TestSomething(t *testing.T) {
    result := DoSomething()

    // Print for debugging
    t.Logf("Result: %+v", result)

    assert.Equal(t, expected, result)
}
```

## Continuous Integration

Tests run automatically on:

* Every pull request
* Every commit to main branch
* Nightly builds

CI runs:

```bash
mise run build
mise run test
mise run godog
```

All tests must pass before merge.

---

## Evidence

This documentation is based on the following repository files:

* Test execution: `.mise.toml:64-67,69-72,74-77,79-82,84-87` (test tasks)
* Testing strategy: `.kiro/steering/tech.md:125-135`
* BDD tests: `tests/features/*.feature` (20+ feature files)
* Step definitions: `tests/features/steps/` directory
* Unit tests: `internal/**/*_test.go` (276 test files)
* Property tests: `internal/**/*_property_test.go` files
* Test utilities: `internal/testutil/` directory
* Pre-commit workflow: `.kiro/steering/tech.md:103-118`