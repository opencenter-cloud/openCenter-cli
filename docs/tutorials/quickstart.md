# Quickstart: From Zero to GitOps

This tutorial guides you through the entire `openCenter` workflow, from initializing a new cluster configuration to bootstrapping a ready-to-use GitOps repository. By the end, you will have a declarative cluster configuration and a local Git repository that is ready to be pushed to a remote.

This guide follows a "happy path" but also demonstrates how `openCenter`'s built-in validation helps you catch common errors early.

## Who is this for?

*   **New users** of `openCenter`.
*   **Platform engineers** who want to quickly scaffold a new cluster environment.

## What you'll achieve

*   Initialize a new cluster configuration file.
*   Validate and correct the configuration.
*   Generate a complete GitOps repository from embedded templates.
*   Bootstrap the cluster by running Terraform, kubectl, Helm, and Ansible from the generated GitOps directory.

---

### Prerequisites

Before you begin, ensure you have the following tools installed on your system:

*   [**Mise**](https://mise.jdx.dev/): For managing project-specific tool versions.
*   [**Podman**](https://podman.io/get-started) or [**OrbStack**](https://orbstack.dev/): For container management.
*   **Git**: For version control.

### Step 1: Install Project Tools

Once you have cloned the `openCenter` repository, the first step is to install the necessary tools pinned in the project's configuration. `mise` handles this for you.

```bash
# This command reads the .mise.toml file and installs
# the correct versions of Go, Godog, etc.
mise install
```

### Step 2: Build the `openCenter` CLI

Next, compile the `openCenter` binary. The `build` task in `.mise.toml` runs the `go build` command.

```bash
# This creates the `openCenter` executable in the root directory.
mise run build
```

You should now be ableto run the CLI:

```bash
./openCenter --version
```

### Step 3: Initialize a New Cluster

The `cluster init` command creates a new YAML configuration file with sensible defaults. You can specify different infrastructure templates depending on your target environment.

For this tutorial, we'll create an OpenStack cluster configuration (the default template):

```bash
./openCenter cluster init demo
```

For other infrastructure types, you can specify a template:

```bash
# Local development with Kind
./openCenter cluster init dev-demo --template=kind

# VMware vSphere environment
./openCenter cluster init prod-demo --template=vmware

# Bare metal servers
./openCenter cluster init metal-demo --template=baremetal

# Talos Linux for secure, minimal Kubernetes
./openCenter cluster init talos-demo --template=talos
```

This command creates a new file at `~/.config/openCenter/demo.yaml`. This file is the single source of truth for your cluster, with infrastructure-specific defaults based on the selected template.

Tip
- If `secrets.sops_age_key_file` is not set in your config at init-time, `openCenter` automatically generates a proper Age key at `~/.config/openCenter/sops/age/keys/demo-key.txt` (permissions `0600`) and saves the updated path into the config. The generated key is compatible with standard Age encryption tools.
 - To disable auto-generation, pass `--no-sops-keygen` to the init command.

### Step 4: Configure and Validate

The default configuration needs a few more details before it's ready.

1.  **Set the GitOps Repository Path**: Open `~/.config/openCenter/demo.yaml` in your favorite editor and set the `gitops.git_dir` and `gitops.git_url` fields. The `git_dir` is where `openCenter` will create the local Git repository, and `git_url` is the remote destination.

    ```yaml
    # ~/.config/openCenter/demo.yaml
    cluster_name: demo
    gitops:
      git_dir: /tmp/opencenter-demo-repo # A temporary local path for this tutorial
      git_url: git@github.com:your-org/your-repo.git # Your remote Git repository URL
    # ... rest of the config
    ```

2.  **Configure Networking**: For this tutorial, we will configure the cluster to use VRRP for the control plane load balancer instead of Octavia. This requires a specific IP address.

    Update the `iac.networking` section in your `demo.yaml`:
    ```yaml
    # ~/.config/openCenter/demo.yaml
    iac:
      # ...
      networking:
        use_octavia: false
        vrrp_enabled: true
        vrrp_ip: "" # Leave this blank for now to see validation fail
      # ...
    ```

3.  **Run the Validator**: `openCenter` includes a `validate` command to check for common configuration errors. Let's run it on our current configuration.

    ```bash
    ./openCenter cluster validate demo
    ```

    Because `vrrp_ip` is required when `use_octavia` is `false`, the command will fail with a helpful error message:

    ```text
    [ERROR] iac.networking.use_octavia=false requires vrrp_ip to be set
    ```

4.  **Fix and Re-validate**: Now, edit `demo.yaml` again and provide a valid IP address for `vrrp_ip`.

    ```yaml
    # ~/.config/openCenter/demo.yaml
    iac:
      # ...
      networking:
        use_octavia: false
        vrrp_enabled: true
        vrrp_ip: "10.0.0.10" # Add the required IP
      # ...
    ```

    Run the validator again:
    ```bash
    ./openCenter cluster validate demo
    ```

    This time, the command should succeed, confirming your configuration is valid.

### Step 5: Select the Active Cluster

Most `openCenter` commands operate on an "active" cluster. The `cluster select` command sets this context.

```bash
./openCenter cluster select demo
```

You can always check which cluster is currently active with `openCenter cluster current`.

### Step 6: Generate the GitOps Repository

Now that the configuration is valid and the cluster is selected, you can generate the GitOps repository. The `cluster setup` command materializes all the embedded templates into the path you specified in `gitops.git_dir`.

The `--render` flag ensures that any `.tmpl` files are processed using the values from your `demo.yaml` file.

```bash
./openCenter cluster setup --render
```

After this command completes, the `/tmp/opencenter-demo-repo` directory will contain a fully-formed GitOps repository, initialized as a Git repo.

Key outputs to expect
- OpenTofu files under `infrastructure/clusters/demo/`:
  - `main.tf`: rendered from your configuration’s `iac.main` (locals) and `iac.modules` (module blocks).
  - `provider.tf`: configures the OpenTofu backend using your `opentofu.backend` and (if set) `opencenter` credentials.

### Step 7: Bootstrap the Cluster

With the GitOps repo generated, you can now provision and configure the cluster directly from the cluster directory using `openCenter cluster bootstrap`.

What it does (in order)
- Runs `terraform init` and `terraform apply -auto-approve`.
- Runs `kubectl get nodes` with `KUBECONFIG=./kubeconfig.yaml` to verify connectivity.
- Adds the Calico Helm repo and installs/updates Calico via `helm upgrade --install` using your overlay values under `applications/overlays/<name>/...`.
- Runs `terraform apply -auto-approve` again to reconcile any changes.
- Runs an Ansible playbook from `kubespray/` with `ANSIBLE_INVENTORY=$PWD/inventory/inventory.yaml` to apply hardening (`-e @../inventory/k8s_hardening.yml`). If a Python virtualenv exists at `venv/bin/`, it is used automatically.

All commands and output are logged to `bootstrap.log` in the cluster directory by default.

**Usage**
```bash
# Execute the full bootstrap flow for the active cluster
./openCenter cluster bootstrap

# Preview actions without executing
./openCenter cluster bootstrap --dry-run

# Use a custom kubeconfig and log path
./openCenter cluster bootstrap demo \
  --kubeconfig ./out/kubeconfig.yaml \
  --log /tmp/demo-bootstrap.log
```

### What's Next?

Congratulations! You have successfully created, configured, and bootstrapped a cluster configuration with `openCenter`.

From here, you can explore more advanced topics:

*   **Run Tests**: Learn how to run the built-in BDD tests in our [How-To Guide on Running Tests](../how-to/run-tests.md).
*   **Explore the CLI**: Dive deeper into all available commands in the [CLI Reference](../reference/cli.md).
*   **Understand the Config**: See a full list of all configuration options in the [Configuration Reference](../reference/config.md).
*   **Learn the Architecture**: Get a high-level overview of how `openCenter` works in the [Architecture Explanation](../explanation/architecture.md).
