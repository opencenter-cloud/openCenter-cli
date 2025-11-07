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

---

## Development Workflow Tasks

### Local Gitea Setup

These tasks help set up a local Gitea instance for development and testing GitOps workflows.

#### `gitea-setup`

Starts a local Gitea instance using Docker/Podman.

**Command**
```bash
mise run gitea-setup
```

**Description**
This task launches a local Gitea server accessible at `https://localhost:3001` for development and testing purposes.

---

#### `gitea-configure`

Configures the Gitea instance with users, tokens, and repositories.

**Command**
```bash
mise run gitea-configure
```

**Description**
This task creates admin and user accounts, generates API tokens, and sets up a test repository.

---

#### `gitea-up`

Complete Gitea setup (combines setup and configure).

**Command**
```bash
mise run gitea-up
```

**Description**
Runs both `gitea-setup` and `gitea-configure` in sequence to provide a fully configured local Gitea instance.

---

#### `gitea-cleanup`

Cleans up local Gitea resources.

**Command**
```bash
mise run gitea-cleanup
```

**Description**
Destroys the local Gitea instance and removes generated token files.

---

### Kind Cluster Tasks

#### `kind-cluster-no-cni`

Creates a kind cluster with no CNI installed using experimental podman support.

**Command**
```bash
mise run kind-cluster-no-cni
```

**Description**
Creates a multi-node kind cluster (1 control-plane, 3 workers) with:
- No default CNI installation (`disableDefaultCNI: true`)
- Experimental podman support
- Standard pod and service subnets
- Automatic kubeconfig export

**When to use it**
Use this to create a local Kubernetes cluster for testing CNI installations or GitOps workflows.

---

## Quick Getting Started Guide

For rapid local development with GitOps workflows:

### 1. Set up local Gitea server
```bash
# Start and configure local Gitea with users and repositories
mise run gitea-up
```

### 2. Generate and upload SSH keys
```bash
# Create SSH key and upload to Gitea newuser account
./hack/gitea-local/setup-ssh-key.sh
```

### 3. Create a kind cluster configuration
```bash
# Create a kind cluster config pointing to local Gitea repo
./bin/openCenter cluster init kind_test \
  --opencenter.provider=kind \
  --opencenter.gitops.git_url=git@localhost:3001:newuser/test-repo.git \
  --opencenter.gitops.git_dir=testdata/repo-kind-local-test \
  --opencenter.gitops.git_ssh_key=/Users/$(whoami)/.ssh/gitea_newuser_key \
  --force
```

### 4. Create kind cluster (optional)
```bash
# Create actual kind cluster for testing
mise run kind-cluster-no-cni
```

This setup provides a complete local development environment with:
- Local Gitea server with SSH access
- Cluster configuration pointing to local Git repository
- Optional local Kubernetes cluster for testing

### Sources
*   `README.md`
*   `tests/features/`
*   `.mise.toml`
*   `hack/gitea-local/`
