# How-To: Run the BDD Test Suite

`openCenter` uses Behavior-Driven Development (BDD) to ensure the CLI behaves as expected. The tests are written in Gherkin (`.feature` files) and executed with Godog. These tests serve as both a safety net against regressions and as living documentation for the tool's functionality.

This guide explains how to run the test suite.

## Who is this for?

*   **Contributors** who want to validate their changes before submitting a pull request.
*   **Developers** who want to understand the expected behavior of a specific command.

## What you'll achieve

*   Run the entire BDD test suite.
*   Run specific feature files or scenarios for targeted testing.
*   Understand the basic structure of the tests.

---

### Prerequisites

*   You have cloned the `openCenter` repository.
*   You have [Mise](https://mise.jdx.dev/) installed.
*   You have installed the project's development tools:

    ```bash
    mise install
    ```

### Running the Test Suite

The primary way to run the BDD tests is by using the `godog` task defined in the project's `.mise.toml` file.

```bash
# This is the primary command for running all regression tests.
mise run godog
```

This task executes the Godog test suite via `go test`. By default, it runs all `.feature` files in the `tests/features/` directory.

### Running Specific Features or Tags

The `godog` task accepts arguments passed to the underlying runner. You can:

- Run a specific feature file via paths:
  ```bash
  mise run godog -- --godog.paths=tests/features/gitops_setup.feature
  ```

- Run by tag (recommended):
  ```bash
  # Only configuration flows (list/select/info/init/update)
  mise run godog -- --godog.tags=@config

  # Only GitOps flows (setup/bootstrap)
  mise run godog -- --godog.tags=@gitops

  # Only secrets/SOPS tests
  mise run godog -- --godog.tags=@secrets

  # Schema generation
  mise run godog -- --godog.tags=@schema
  ```

Note the `--` which separates the `mise` command from arguments for the task.

There is also a `test` task, which runs unit tests for the internal packages:
```bash
mise run test
```

### Understanding the Test Structure

The tests are organized in the `tests/features/` directory by behavior:

- `config_*.feature`: configuration flows (list/select/info/init/update)
- `gitops_*.feature`: GitOps lifecycle (setup, bootstrap)
 
- `schema.feature`: JSON schema generation
- `secrets.feature` / `secrets_sops.feature`: SOPS/age helpers
- `preflight.feature`: provider checks
- `destroy.feature`: teardown safety

The `tests/features/steps/` directory contains the Go step definitions used by all features.

By reading the `.feature` files, you can get a clear understanding of what `openCenter` is designed to do and how it should behave in various situations.
