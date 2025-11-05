# CLI Command Reference

This document provides a detailed reference for every command available in the `openCenter` CLI.

## Global Flags

These flags are available on all `openCenter` commands.

| Flag | Description | Default | Environment Variable |
| --- | --- | --- | --- |
| `--config-dir <path>` | Specifies the directory where cluster configuration files are stored. | `~/.config/openCenter` | `OPENCENTER_CONFIG_DIR` |

### Environment Variables
- `OPENCENTER_CONFIG_DIR`: Overrides the configuration directory. Passing `--config-dir` sets this variable for the process.
- `OPENCENTER_PLUGINS_DIR`: Directory searched first for plugin executables before `$OPENCENTER_CONFIG_DIR/plugins` and `PATH`.

---

## `openCenter cluster`

The `cluster` command is the main entry point for managing cluster configurations. It serves as a parent command for many subcommands.

Running `openCenter cluster` by itself will display a help message listing all available subcommands.

### `openCenter cluster list`

(Alias: `ls`)

Lists all available cluster configurations found in the configuration directory.

**Usage**
```bash
openCenter cluster list [flags]
```

**Flags**
| Flag | Description |
| --- | --- |
| `--json` | Output the list of clusters in JSON format. |

**Example**
```bash
$ ./openCenter cluster list
dev
prod

$ ./openCenter cluster ls --json
[
  "dev",
  "prod"
]
```

---

## `openCenter plugins`

Manage external plugins.

Plugins are executables named `openCenter-<name>` discovered at runtime and exposed as `openCenter <name>`. See `docs/reference/plugins.md` for discovery rules and examples.

### `openCenter plugins list`

Lists discovered plugins and their paths.

Usage
```bash
openCenter plugins list
```

Example
```bash
openCenter plugins list
# kubectl	/Users/alice/.config/openCenter/plugins/openCenter-kubectl
```
No flags

---

### `openCenter cluster select [name]`

Selects a cluster to be the "active" context for other commands.

If `[name]` is provided, it sets that cluster as active. If no name is provided, it launches an interactive menu to choose from the list of available clusters.

**Usage**
```bash
openCenter cluster select [name]
```

**Example**
```bash
# Select a cluster by name
./openCenter cluster select dev

# Select a cluster interactively
./openCenter cluster select
```

---

### `openCenter cluster current`

Displays the name of the currently active cluster.

**Usage**
```bash
openCenter cluster current
```

**Example**
```bash
$ ./openCenter cluster current
dev
```

---

### `openCenter cluster info [name]`

Displays the full, parsed configuration for a cluster.

If `[name]` is provided, it shows the configuration for that cluster. If no name is provided, it shows the configuration for the active cluster.

**Usage**
```bash
openCenter cluster info [name] [flags]
```

**Flags**
| Flag | Description |
| --- | --- |
| `--json` | Output the full configuration in JSON format. |

**Example**
```bash
# Show info for the active cluster
./openCenter cluster info

# Show info for a specific cluster as JSON
./openCenter cluster info dev --json
```

---

### `openCenter cluster init <name>`

Initializes a new cluster configuration file with default values. The command is non-interactive and requires a cluster name. Uses the OpenStack provider by default.

**Usage**
```bash
openCenter cluster init <name> [flags]
```

**Flags**
| Flag | Description |
| --- | --- |
| `--force` | Overwrite the configuration file if it already exists. |
| `--strict`| Fail the command if the resulting configuration is not valid. |
| `--no-sops-keygen` | Do not auto-generate a SOPS age key when `secrets.sops_age_key_file` is unset. |

**Dynamic Configuration Flags**

The `init` command accepts additional flags to override any field in the configuration using dot notation. Unknown flags are interpreted as field assignments and applied to the in-memory config before saving. Supported value types include strings, integers, and booleans.

**Configuration Structure**

The configuration follows this structure:
- `opencenter.*` - Main cluster configuration
  - `opencenter.provider` - Cloud provider (openstack, aws, kind, vmware, etc.)
  - `opencenter.cluster.*` - Cluster metadata and Kubernetes settings
  - `opencenter.gitops.*` - GitOps repository configuration
  - `opencenter.cloud.*` - Cloud provider-specific settings
  - `opencenter.services.*` - Service enablement flags
- `opentofu.*` - OpenTofu/Terraform backend configuration
- `secrets.*` - Secret management settings

**Common Configuration Fields**

| Field Path | Description | Example |
| --- | --- | --- |
| `opencenter.provider` | Cloud provider | `openstack`, `kind`, `aws` |
| `opencenter.cluster.cluster_name` | Cluster name | `my-cluster` |
| `opencenter.gitops.git_url` | GitOps repository URL | `git@github.com:org/repo.git` |
| `opencenter.gitops.git_dir` | Local git directory | `./testdata/repo-local` |
| `opencenter.gitops.git_ssh_key` | SSH private key path | `~/.ssh/gitea_key` |
| `opencenter.kubernetes.master_count` | Number of master nodes | `3` |
| `opencenter.kubernetes.worker_count` | Number of worker nodes | `4` |
| `opencenter.services.cert-manager` | Enable cert-manager | `true`/`false` |
| `opentofu.backend.type` | Backend type | `local`, `s3` |
| `secrets.sops_age_key_file` | SOPS age key file path | `~/.config/sops/age/keys.txt` |

**Flow**
- Resolve config directory: honors `--config-dir` or `OPENCENTER_CONFIG_DIR`; defaults to `~/.config/openCenter`.
- Create defaults: start from a default OpenStack configuration for `<name>`.
- Apply overrides: parse unknown `--field.path=value` flags and set matching fields.
- Guard existing files: if `<config-dir>/<name>.yaml` exists and `--force` is not set, abort.
- Optional strict validation: if `--strict`, validate the config and abort on errors (errors printed to stderr).
- Optional SOPS key generation: if `secrets.sops_age_key_file` is empty and `--no-sops-keygen` is not set, generate an Age key at `<config-dir>/sops/age/keys/<name>-key.txt` (0600) and set the field.
- Save configuration: write `<config-dir>/<name>.yaml` with 0600 permissions.
- Output: print `Created cluster configuration <name>` on success.

**Outcomes**
- A new config file exists at `<config-dir>/<name>.yaml` populated with defaults plus any overrides.
- If generated, an Age key file exists at `<config-dir>/sops/age/keys/<name>-key.txt`, and `secrets.sops_age_key_file` points to it.
- The active cluster is not changed; set it with `openCenter cluster select <name>` if desired.
- Validation runs only when `--strict` is provided; otherwise, you can validate later with `openCenter cluster validate <name>`.

**Examples**
```bash
# Create a new OpenStack cluster with GitOps configuration
./openCenter cluster init my-cluster \
  --opencenter.gitops.git_dir=/tmp/my-cluster \
  --opencenter.gitops.git_url=git@github.com:my-org/my-cluster.git

# Create a kind cluster for local development
./openCenter cluster init kind-test \
  --opencenter.provider=kind \
  --opencenter.gitops.git_url=git@localhost:3001:newuser/test-repo.git \
  --opencenter.gitops.git_dir=./testdata/repo-kind-local \
  --opencenter.gitops.git_ssh_key=~/.ssh/gitea_newuser_key

# Create an AWS cluster with custom worker count
./openCenter cluster init aws-prod \
  --opencenter.provider=aws \
  --opencenter.cluster.kubernetes.worker_count=10 \
  --opencenter.cloud.aws.region=us-west-2

# Create without auto-generating a SOPS key
./openCenter cluster init my-cluster --no-sops-keygen

# Create and fail fast on validation errors
./openCenter cluster init strict-cluster --strict \
  --opencenter.services.cert-manager=false
```

---

### `openCenter cluster validate [name]`

Validates the configuration for a given cluster against a set of rules.

**Usage**
```bash
openCenter cluster validate [name]
```

**Example**
```bash
./openCenter cluster validate my-cluster
```

---

### `openCenter cluster preflight [name]`

Runs a series of preflight checks to ensure the environment is ready for setup and bootstrapping. This includes checking for required tools (like `git`, `kubectl`) and provider-specific configurations.

**Usage**
```bash
openCenter cluster preflight [name]
```

---

### `openCenter cluster setup`

Generates the GitOps repository for the active cluster by materializing embedded templates into the `gitops.git_dir` path.

What it generates (high level)
- OpenTofu IaC: `infrastructure/clusters/<name>/main.tf` rendered from `iac.main` (locals) and `iac.modules` (module blocks).
- OpenTofu backend: `infrastructure/clusters/<name>/provider.tf` configuring the OpenTofu backend from `opentofu.backend` and `opencenter` creds.
- Other base files: copied or rendered from embedded templates under `internal/gitops/gitops-base-dir`.

**Usage**
```bash
openCenter cluster setup [flags]
```

**Flags**
| Flag | Description |
| --- | --- |
| `--render` | Process `.tmpl` files using values from the cluster configuration. |
| `--force` | Overwrite the `git_dir` if it already exists. |

**Example**
```bash
# Set up the repo, processing templates
./openCenter cluster setup --render
```

Notes
- `main.tf` is always produced from the structured configuration (`iac.main` and `iac.modules`). The older `iac.main_tf` string is no longer used during setup.

---

### `openCenter cluster render`

Renders the templates for the active cluster without performing the full setup (e.g., does not initialize a git repository). This is useful for inspecting the output of the templating engine.

**Usage**
```bash
openCenter cluster render
```

---

### `openCenter cluster bootstrap`

Runs provider-specific bootstrap actions for a cluster.

Flow
- Cloud providers (`openstack`, `aws`, `gcp`, `azure`): run `make` inside `gitops.git_dir/infrastructure/clusters/<name>` to provision infrastructure.
- `kind` provider: create a local kind cluster using the repository defaults (four nodes, networking disabled for CNI) and export the kubeconfig. The command honours `CONTAINER_RUNTIME` or `--container-runtime` (`docker` or `podman`).

Logging
- Logs all commands and output to `bootstrap.log` in the cluster directory by default when the GitOps directory exists.
- Use `--log` to customize the log path.

**Usage**
```bash
openCenter cluster bootstrap [name] [flags]
```

**Flags**
| Flag | Description | Default |
| --- | --- | --- |
| `--dry-run` | Show planned actions without executing. | `false` |
| `--kubeconfig <path>` | Path to kubeconfig exported during bootstrap (cloud providers). | `./kubeconfig.yaml` |
| `--log <path>` | Log file path. | `<git_dir>/infrastructure/clusters/<name>/bootstrap.log` |
| `--container-runtime <runtime>` | Container runtime for kind clusters (`docker` or `podman`). | `docker` (unless environment overrides) |

**Examples**
```bash
# Execute bootstrap for the active cluster (runs `make` for cloud providers)
./openCenter cluster bootstrap

# Preview the commands without executing them
./openCenter cluster bootstrap --dry-run

# Create a kind cluster using podman
./openCenter cluster bootstrap dev --container-runtime podman --dry-run
```

Requirements
- `make` for cloud providers.
- `kind` (plus the selected container runtime) when using the `kind` provider.
- Cluster directory exists for cloud providers (generated by `openCenter cluster setup`).

Notes
- When `CONTAINER_RUNTIME=podman` (as configured in `.mise.toml`), the bootstrap command automatically sets `KIND_EXPERIMENTAL_PROVIDER=podman` for kind cluster creation.
- Additional provider-specific behaviour can extend this command; unsupported providers will raise a clear error.

---

### `openCenter cluster schema`

Generates a JSON Schema (Draft 2020-12) for the cluster configuration. This schema can be used for IDE validation and autocompletion.

**Usage**
```bash
openCenter cluster schema [flags]
```

**Flags**
| Flag | Description |
| --- | --- |
| `--out <path>` | Path to write the schema file to. If omitted, prints to stdout. |
| `--pretty` | Indent the JSON output for readability. |

**Example**
```bash
./openCenter cluster schema --out schema/cluster.schema.json --pretty
```

### `openCenter cluster destroy`

Destroy a cluster.

**Usage**
```bash
openCenter cluster destroy
```

### Sources

*   `cmd/`
*   `tests/features/`
*   `README.md`

---

### `openCenter cluster update [name]`

Updates fields in an existing cluster configuration using dotted flags. If `[name]` is omitted, the active cluster is used.

This mirrors the dynamic flag behavior of `cluster init`, allowing you to script updates succinctly.

**Usage**
```bash
openCenter cluster update [name] [flags]
```

**Flags**
| Flag | Description |
| --- | --- |
| `--strict` | Fail the command if the resulting configuration is not valid. |

**Examples**
```bash
# Update a specific cluster
./openCenter cluster update test01 --iac.counts.master=3

# Update the active cluster
./openCenter cluster update --gitops.git_branch=main --ansible.enabled=false
```

---



## `openCenter sops`

The `sops` command group provides comprehensive SOPS (Secrets OPerationS) key management and automation for secure GitOps workflows. These commands handle the full lifecycle of Age encryption keys used for encrypting sensitive data in Kubernetes deployments.

For detailed documentation of all SOPS commands, see [`docs/cli/sops.md`](../cli/sops.md).

**Available Commands:**
- `generate-key` - Generate new Age key pair for SOPS encryption
- `rotate-key` - Rotate Age keys and re-encrypt existing secrets  
- `backup-key` - Create backup of Age keys and SOPS configuration
- `validate` - Validate Age key configuration and SOPS setup

**Quick Examples:**
```bash
# Generate a new SOPS key
openCenter sops generate-key

# Validate SOPS setup
openCenter sops validate

# Create a backup of keys
openCenter sops backup-key

# Rotate keys and re-encrypt files
openCenter sops rotate-key --search-path ./secrets
```
