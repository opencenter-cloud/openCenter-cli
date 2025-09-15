# Developer Tasks Reference

This project uses [Mise](https://mise.jdx.dev/) to manage project-specific tooling and to provide a consistent interface for common development tasks. These tasks are defined in the `.mise.toml` file at the root of the repository.

To run a task, use the `mise run` command followed by the task name.

## Task Reference

### `build`

Compiles the `openCenter` Go binary.

**Command**
```bash
mise run build
```

**Description**
This task runs `go build` to create the `openCenter` executable in the root of the repository.

**When to use it**
Run this after making any changes to the Go source code to ensure you have an up-to-date binary for testing.

---

### `test`

Runs the unit tests for the internal packages.

**Command**
```bash
mise run test
```

**Description**
This task runs `go test ./internal/...` to execute all unit tests within the `internal` directory.

**When to use it**
Run this to quickly check the core logic of the application.

---

### `godog`

Runs the full Behavior-Driven Development (BDD) test suite.

**Command**
```bash
mise run godog
```

**Description**
This is the main command for running all BDD regression tests. It uses `go test` to execute the Godog test suite defined in the `tests/features/` directory.

**When to use it**
Run this before submitting any code changes to ensure you haven't introduced any regressions.

---

### `schema`

Generates the JSON Schema for the cluster configuration.

**Command**
```bash
mise run schema
```

**Description**
This task is a convenient wrapper for the `openCenter cluster schema` command. It generates the schema and prints it to standard output.

**When to use it**
Use this to quickly generate the schema, for example, to pipe it to a file.

**Example**
```bash
mise run schema > schema/cluster.schema.json
```

---

### `preflight`

Runs the preflight checks for the active cluster.

**Command**
```bash
mise run preflight
```

**Description**
A wrapper for the `openCenter cluster preflight` command.

**When to use it**
Use this as a quick check to ensure your environment is ready before a `setup` or `bootstrap`.

---

### `validate`

Validates the configuration of the active cluster.

**Command**
```bash
mise run validate
```

**Description**
A wrapper for the `openCenter cluster validate` command.

**When to use it**
Use this to quickly check your configuration for errors after making changes.

### Sources
*   `README.md`
*   `tests/features/`
