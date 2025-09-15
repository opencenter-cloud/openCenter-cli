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

Initializes a new cluster configuration file with default values. The command is non-interactive and requires a cluster name.

**Usage**
```bash
openCenter cluster init <name> [flags]
```

**Flags**
| Flag | Description |
| --- | --- |
| `-t, --template <type>` | Cluster template type: `openstack` (default), `kind`, `vmware`, `baremetal`, `talos`. Each template provides infrastructure-specific default configurations. |
| `--force` | Overwrite the configuration file if it already exists. |
| `--strict`| Fail the command if the resulting configuration is not valid. |
| `--no-sops-keygen` | Do not auto-generate a SOPS age key when `secrets.sops_age_key_file` is unset. |
| `--full-schema` | Include all default keys in the generated YAML, including values that reference Terraform locals. Without this flag, entries whose values contain `local.` are omitted for brevity. |

**Dynamic Flags**

The `init` command accepts additional flags to override any field in the configuration using dot notation. Unknown flags are interpreted as field assignments and applied to the in-memory config before saving. Supported value types include strings, integers, and booleans. Map fields accept simple `--map.key=value` assignments.

**Flow**
- Resolve config directory: honors `--config-dir` or `OPENCENTER_CONFIG_DIR`; defaults to `~/.config/openCenter`.
- Create defaults: start from a spec-aligned default config for `<name>`.
- Apply overrides: parse unknown `--field.path=value` flags and set matching fields.
- Guard existing files: if `<config-dir>/<name>.yaml` exists and `--force` is not set, abort.
- Optional strict validation: if `--strict`, validate the config and abort on errors (errors printed to stderr).
- Optional SOPS key generation: if `secrets.sops_age_key_file` is empty and `--no-sops-keygen` is not set, generate an Age key at `<config-dir>/sops/age/keys/<name>-key.txt` (0600) and set the field.
- Save configuration: write `<config-dir>/<name>.yaml` with 0600 permissions.
- Output: print `Created cluster configuration <name>` on success.
  - By default, the generated YAML omits `iac.main` and `iac.modules` entries whose values include `local.` references. Pass `--full-schema` to include them.

**Outcomes**
- A new config file exists at `<config-dir>/<name>.yaml` populated with defaults plus any overrides.
- If generated, an Age key file exists at `<config-dir>/sops/age/keys/<name>-key.txt`, and `secrets.sops_age_key_file` points to it.
- The active cluster is not changed; set it with `openCenter cluster select <name>` if desired.
- Validation runs only when `--strict` is provided; otherwise, you can validate later with `openCenter cluster validate <name>` or `openCenter cluster info --validate`.

**Examples**
```bash
# Create a new OpenStack cluster (default template)
./openCenter cluster init my-cluster \
  --gitops.git_dir=/tmp/my-cluster \
  --gitops.git_url=git@github.com:my-org/my-cluster.git

# Create a local Kind cluster for development
./openCenter cluster init dev-cluster --template=kind \
  --gitops.git_dir=/tmp/dev-cluster

# Create a VMware vSphere cluster
./openCenter cluster init prod-cluster -t vmware \
  --gitops.git_dir=/tmp/prod-cluster \
  --gitops.git_url=git@github.com:my-org/prod-cluster.git

# Create a bare metal cluster
./openCenter cluster init baremetal-cluster --template=baremetal \
  --gitops.git_dir=/tmp/baremetal-cluster

# Create a Talos cluster for secure, minimal Kubernetes
./openCenter cluster init talos-cluster --template=talos \
  --gitops.git_dir=/tmp/talos-cluster

# Create without auto-generating a SOPS key
./openCenter cluster init my-cluster --no-sops-keygen

# Create and fail fast on invalid combinations
./openCenter cluster init strict-cluster --strict \
  --iac.networking.use_octavia=false
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

Provisions and configures the cluster from the cluster directory `gitops.git_dir/infrastructure/clusters/<name>`.

Flow
- `terraform init`
- `terraform apply -auto-approve`
- `KUBECONFIG=./kubeconfig.yaml kubectl get nodes`
- `helm repo add projectcalico https://docs.tigera.io/calico/charts`
- `helm upgrade --install calico projectcalico/tigera-operator --namespace tigera-operator -f ../../../applications/overlays/<name>/services/calico/helm-values/override_values.yaml --create-namespace`
- `terraform apply -auto-approve`
- `ANSIBLE_INVENTORY=$PWD/inventory/inventory.yaml ansible-playbook -f 10 -b upgrade-cluster.yml -e @../inventory/k8s_hardening.yml` (runs from the `kubespray/` subdirectory and uses `venv/bin` if present)

Logging
- Logs all commands and output to `bootstrap.log` in the cluster directory by default.
- Use `--log` to customize the log path.

**Usage**
```bash
openCenter cluster bootstrap [name] [flags]
```

**Flags**
| Flag | Description | Default |
| --- | --- | --- |
| `--dry-run` | Show planned actions without executing. | `false` |
| `--kubeconfig <path>` | Path to kubeconfig used by kubectl (relative to cluster dir). | `./kubeconfig.yaml` |
| `--log <path>` | Log file path. | `<git_dir>/infrastructure/clusters/<name>/bootstrap.log` |

**Examples**
```bash
# Execute full bootstrap flow for the active cluster, logging to default path
./openCenter cluster bootstrap

# Preview actions only
./openCenter cluster bootstrap --dry-run

# Use an explicit kubeconfig and custom log location
./openCenter cluster bootstrap dev \
  --kubeconfig ./out/kubeconfig.yaml \
  --log /tmp/dev-bootstrap.log
```

Requirements
- Tools on PATH: `terraform`, `kubectl`, `helm`, `ansible-playbook`.
- Cluster directory exists (generated by `openCenter cluster setup`).

Notes
- The Helm values file path is resolved under `applications/overlays/<name>/...` using the current cluster name.
- If `venv/bin/` exists at the cluster directory, it is prepended to `PATH` for the Ansible step.

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

## `openCenter secrets`

The `secrets` command group provides helpers for working with secret management tools used alongside GitOps.

Running `openCenter secrets` by itself will display a help message listing available subcommands.

### `openCenter secrets sops-keygen`

Generates a SOPS (age) secret key file suitable for use with SOPS. The file is written with `0600` permissions and contains a key string starting with `AGE-SECRET-KEY-1`.

Note: This helper generates a placeholder key compatible with SOPS. You may replace it with a key generated by the `age-keygen` tool at any time.

**Usage**
```bash
openCenter secrets sops-keygen --out <path>
```

**Flags**
| Flag | Description |
| --- | --- |
| `--out <path>` | Path to write the age key file (required). |

**Example**
```bash
./openCenter secrets sops-keygen --out ~/.config/sops/age/keys.txt

# Configure your cluster to point at the key
# secrets:
#   sops_age_key_file: ~/.config/sops/age/keys.txt
```
