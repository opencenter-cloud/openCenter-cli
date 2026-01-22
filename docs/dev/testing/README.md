---
doc_type: how-to
---

# Testing Guide


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Testing Philosophy](#testing-philosophy)
- [Test Organization](#test-organization)
- [Unit Tests](#unit-tests)
- [Property-Based Tests](#property-based-tests)
- [BDD Tests](#bdd-tests)
- [Integration Tests](#integration-tests)
- [Test Fixtures](#test-fixtures)
- [Mocking](#mocking)
- [Test Coverage](#test-coverage)
- [CI Integration](#ci-integration)
- [Debugging Tests](#debugging-tests)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [See Also](#see-also)
This guide covers testing strategies, frameworks, and best practices for openCenter.

## Who this is for

Developers writing tests for openCenter, whether unit tests, BDD tests, property-based tests, or integration tests.

## Testing Philosophy

openCenter uses multiple testing strategies to ensure reliability:

1. **Unit Tests**: Test individual functions and components in isolation
2. **Property-Based Tests**: Test properties that should hold for all inputs
3. **BDD Tests**: Test complete workflows from user perspective
4. **Integration Tests**: Test component interactions

Each testing strategy serves a different purpose and catches different types of bugs.

## Test Organization

### Directory Structure

```
openCenter-cli/
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── config_test.go              # Unit tests
│   │   ├── migration_property_test.go  # Property tests
│   │   └── integration_test.go         # Integration tests
│   └── testing/
│       ├── framework.go                # Test framework
│       ├── generators.go               # Test data generators
│       └── mocks.go                    # Mock implementations
├── tests/
│   └── features/
│       ├── cluster_init.feature        # BDD scenarios
│       └── steps/                      # Step definitions
└── testdata/
    └── fixtures/                       # Test fixtures
```

### File Naming Conventions

- Unit tests: `*_test.go`
- Property tests: `*_property_test.go`
- Integration tests: `*_integration_test.go`
- BDD features: `*.feature`
- Test fixtures: `testdata/`

## Unit Tests

### Writing Unit Tests

Unit tests use Go's standard testing package:

```go
func TestConfigValidation(t *testing.T) {
    validator := config.NewValidator()
    cfg := config.Config{
        OpenCenter: config.SimplifiedOpenCenter{
            Meta: config.ClusterMeta{
                Name: "test-cluster",
            },
        },
    }
    
    err := validator.Validate(cfg)
    if err != nil {
        t.Errorf("expected no error, got %v", err)
    }
}
```

### Table-Driven Tests

Use table-driven tests for multiple scenarios:

```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  config.Config
        wantErr bool
        errMsg  string
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
            errMsg:  "cluster name is required",
        },
        {
            name:    "invalid kubernetes version",
            config:  configWithInvalidK8sVersion(),
            wantErr: true,
            errMsg:  "invalid kubernetes version",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validator.Validate(tt.config)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
                t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
            }
        })
    }
}
```

### Test Helpers

Use `t.Helper()` in test helper functions:

```go
func createTestConfig(t *testing.T, name string) config.Config {
    t.Helper()
    
    return config.Config{
        OpenCenter: config.SimplifiedOpenCenter{
            Meta: config.ClusterMeta{
                Name: name,
            },
        },
    }
}
```

### Running Unit Tests

Run all unit tests:
```bash
mise run test
```

Run tests for specific package:
```bash
go test ./internal/config
```

Run specific test:
```bash
go test ./internal/config -run TestConfigValidation
```

Run with coverage:
```bash
go test -cover ./internal/...
```

Generate coverage report:
```bash
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out
```

## Property-Based Tests

### What are Property-Based Tests?

Property-based tests verify that properties hold for all inputs, not just specific examples.

Instead of testing specific cases:
```go
// Example-based test
func TestReverse(t *testing.T) {
    assert.Equal(t, "cba", reverse("abc"))
    assert.Equal(t, "321", reverse("123"))
}
```

Test properties:
```go
// Property-based test
func TestReverseProperty(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("reversing twice returns original", prop.ForAll(
        func(s string) bool {
            return s == reverse(reverse(s))
        },
        gen.AnyString(),
    ))
    
    properties.TestingRun(t)
}
```

### Writing Property-Based Tests

openCenter uses [gopter](https://github.com/leanovate/gopter) for property-based testing:

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
    
    properties.Property("migration is idempotent", prop.ForAll(
        func(cfg config.Config) bool {
            once := migrator.Migrate(cfg)
            twice := migrator.Migrate(once)
            return reflect.DeepEqual(once, twice)
        },
        generators.Config(),
    ))
    
    properties.TestingRun(t)
}
```

### Test Data Generators

openCenter provides test data generators in `internal/testing/generators.go`:

```go
// Generate random valid configuration
cfg := generators.Config()

// Generate minimal configuration
cfg := generators.MinimalConfig("openstack")

// Generate complex configuration
cfg := generators.ComplexConfig("openstack")
```

### When to Use Property-Based Tests

Use property-based tests for:
- Data transformations (should preserve certain properties)
- Serialization/deserialization (round-trip should be identity)
- Configuration migration (should preserve data)
- Validation logic (valid input should always pass)
- Idempotent operations (applying twice should equal applying once)

### Running Property-Based Tests

Property-based tests run with regular unit tests:
```bash
mise run test
```

Run only property tests:
```bash
go test ./internal/... -run Property
```

## BDD Tests

### What are BDD Tests?

Behavior-Driven Development tests describe system behavior in plain language using Gherkin syntax.

### Writing BDD Tests

Create feature files in `tests/features/`:

```gherkin
Feature: Cluster Initialization
  As a user, I want to initialize a new cluster configuration
  so that I can start defining my cluster layout.

  Scenario: Initialize cluster with default settings
    When I run "openCenter cluster init test-cluster"
    Then a cluster configuration "test-cluster" should exist
    And the cluster configuration "test-cluster" should have "opencenter.cluster.cluster_name" set to "test-cluster"

  Scenario: Initialize cluster with custom organization
    When I run "openCenter cluster init my-cluster --opencenter.meta.organization=my-org"
    Then a cluster configuration "my-cluster" should exist
    And the cluster configuration "my-cluster" should have "opencenter.meta.organization" set to "my-org"
    And a directory "~/.config/openCenter/clusters/my-org" should exist
```

### Step Definitions

Implement step definitions in `tests/features/steps/`:

```go
func (s *Suite) iRunCommand(command string) error {
    cmd := exec.Command("sh", "-c", command)
    output, err := cmd.CombinedOutput()
    s.lastOutput = string(output)
    s.lastError = err
    return nil
}

func (s *Suite) aClusterConfigurationShouldExist(clusterName string) error {
    path := s.getConfigPath(clusterName)
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return fmt.Errorf("configuration file does not exist: %s", path)
    }
    return nil
}
```

### BDD Test Tags

Use tags to organize scenarios:

```gherkin
@priority1
Scenario: High priority test
  ...

@wip
Scenario: Work in progress
  ...

@slow
Scenario: Slow running test
  ...
```

### Running BDD Tests

Run all BDD tests:
```bash
mise run godog
```

Run only WIP scenarios:
```bash
mise run godog-wip
```

Run specific feature:
```bash
go test ./... -v args --godog.paths=tests/features/cluster_init.feature
```

Run scenarios with specific tag:
```bash
go test ./... -v args --godog.tags=@priority1 --godog.paths=tests/features
```

### BDD Best Practices

- **Use Given-When-Then structure**: Setup, action, assertion
- **Keep scenarios focused**: One behavior per scenario
- **Use descriptive names**: Scenario names should explain the behavior
- **Avoid implementation details**: Focus on user-visible behavior
- **Reuse step definitions**: Create reusable steps
- **Use tags**: Organize scenarios with tags

## Integration Tests

### What are Integration Tests?

Integration tests verify that multiple components work together correctly.

### Writing Integration Tests

```go
func TestGitOpsGenerationIntegration(t *testing.T) {
    // Setup test framework
    fw := testing.NewTestFramework(t)
    
    // Create test configuration
    cfg := fw.CreateTestConfig("openstack")
    
    // Create GitOps generator with real template engine
    generator := gitops.NewGenerator(fw.TemplateEngine)
    
    // Generate GitOps repository
    err := generator.Generate(cfg, fw.TempDir)
    if err != nil {
        t.Fatalf("failed to generate GitOps repository: %v", err)
    }
    
    // Verify directory structure
    fw.AssertDirExists(t, filepath.Join(fw.TempDir, "infrastructure"))
    fw.AssertDirExists(t, filepath.Join(fw.TempDir, "applications"))
    
    // Verify files were created
    fw.AssertFileExists(t, filepath.Join(fw.TempDir, "infrastructure", "clusters", cfg.OpenCenter.Meta.Name, "cluster.yaml"))
}
```

### Test Framework

openCenter provides a test framework in `internal/testing/framework.go`:

```go
// Create test framework
fw := testing.NewTestFramework(t)

// Access temporary directories
fw.TempDir      // Root temp directory
fw.ConfigDir    // Config directory
fw.TemplateDir  // Template directory

// Create test data
cfg := fw.CreateTestConfig("openstack")
data := fw.CreateTestTemplateData()

// Use mock implementations
fw.MockTemplateEngine
fw.MockConfigBuilder
fw.MockConfigValidator

// Assertions
fw.AssertFileExists(t, path)
fw.AssertDirExists(t, path)
```

### Running Integration Tests

Run all tests (includes integration tests):
```bash
mise run test
```

Run only integration tests:
```bash
go test ./internal/... -run Integration
```

## Test Fixtures

### Using Test Fixtures

Store test data in `testdata/` directories:

```
internal/config/testdata/
├── valid-config.yaml
├── invalid-config.yaml
└── minimal-config.yaml
```

Load fixtures in tests:

```go
func loadTestFixture(t *testing.T, name string) []byte {
    t.Helper()
    
    data, err := os.ReadFile(filepath.Join("testdata", name))
    if err != nil {
        t.Fatalf("failed to load fixture %s: %v", name, err)
    }
    
    return data
}
```

### Golden Files

Use golden files for expected output:

```go
func TestSchemaGeneration(t *testing.T) {
    schema := generateSchema()
    
    goldenFile := "testdata/schema.golden.json"
    
    if *update {
        // Update golden file
        os.WriteFile(goldenFile, schema, 0644)
    }
    
    expected, _ := os.ReadFile(goldenFile)
    
    if !bytes.Equal(schema, expected) {
        t.Errorf("schema does not match golden file")
    }
}
```

## Mocking

### Mock Implementations

openCenter provides mock implementations in `internal/testing/mocks.go`:

```go
// Use mock template engine
mockEngine := testing.NewMockTemplateEngine()
mockEngine.On("Render", mock.Anything, mock.Anything).Return("rendered", nil)

// Use mock config validator
mockValidator := testing.NewMockConfigValidator()
mockValidator.On("Validate", mock.Anything).Return(nil)
```

### Creating Mocks

Create mocks for interfaces:

```go
type MockLoader struct {
    mock.Mock
}

func (m *MockLoader) Load(path string) (*Config, error) {
    args := m.Called(path)
    return args.Get(0).(*Config), args.Error(1)
}
```

## Test Coverage

### Measuring Coverage

Run tests with coverage:
```bash
go test -cover ./internal/...
```

Generate coverage report:
```bash
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out
```

### Coverage Goals

- **Overall**: 80%+ coverage
- **Critical paths**: 100% coverage (security, validation)
- **New code**: 80%+ coverage
- **Bug fixes**: Add test that reproduces bug

## CI Integration

### Running Tests in CI

Tests run automatically on:
- Every commit to `main`
- Every pull request
- Every tag push

CI runs:
```bash
mise run test          # Unit tests
mise run godog         # BDD tests
mise run test-security # Security tests
```

### Test Parallelization

Tests run in parallel by default:
```bash
go test -parallel 4 ./internal/...
```

Disable parallelization for specific tests:
```go
func TestSequential(t *testing.T) {
    t.Parallel() // Remove this line
    // Test code
}
```

## Debugging Tests

### Verbose Output

Run tests with verbose output:
```bash
go test -v ./internal/config
```

### Print Debugging

Use `t.Log` for debugging:
```go
func TestSomething(t *testing.T) {
    t.Log("Debug information")
    t.Logf("Value: %v", value)
}
```

### Run Single Test

Run specific test:
```bash
go test ./internal/config -run TestConfigValidation
```

### Test Timeout

Set test timeout:
```bash
go test -timeout 30s ./internal/...
```

## Best Practices

### General

- **Test behavior, not implementation**: Focus on what, not how
- **Keep tests independent**: Tests should not depend on each other
- **Use descriptive names**: Test names should explain what is being tested
- **Test edge cases**: Test boundary conditions and error cases
- **Keep tests fast**: Slow tests discourage running them
- **Use table-driven tests**: Test multiple scenarios efficiently

### Unit Tests

- **Test one thing**: Each test should verify one behavior
- **Use mocks**: Isolate unit under test
- **Avoid external dependencies**: No network, file system, database
- **Test error paths**: Test both success and failure cases

### BDD Tests

- **Write from user perspective**: Focus on user-visible behavior
- **Keep scenarios simple**: One behavior per scenario
- **Use background**: Share common setup across scenarios
- **Avoid technical details**: Focus on business logic

### Property-Based Tests

- **Test invariants**: Properties that should always hold
- **Use generators**: Generate diverse test data
- **Start simple**: Begin with simple properties
- **Shrink failures**: Use shrinking to find minimal failing case

## Troubleshooting

### Tests Fail Locally

1. **Clean build**: `go clean -testcache`
2. **Update dependencies**: `mise run tidy`
3. **Check environment**: Verify environment variables
4. **Run verbose**: `go test -v` for more details

### Tests Fail in CI

1. **Check CI logs**: Review full output
2. **Reproduce locally**: Try to reproduce failure
3. **Check timing**: Look for race conditions
4. **Verify isolation**: Ensure tests don't interfere

### Flaky Tests

1. **Identify pattern**: When does it fail?
2. **Check timing**: Add timeouts or retries
3. **Check isolation**: Ensure proper cleanup
4. **Use deterministic data**: Avoid random data in tests

## See Also

- [Developer Guide](../README.md) - Development setup and workflows
- [Contributing Guidelines](../contributing.md) - How to contribute
- [Architecture Documentation](../architecture.md) - Codebase architecture
- [BDD Tests](./bdd-tests.md) - BDD test suite documentation
- [Sandbox Setup](./sandbox-setup.md) - Test sandbox environments
