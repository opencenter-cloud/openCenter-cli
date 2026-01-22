# Getting Started with openCenter


## Table of Contents

- [What You'll Build](#what-youll-build)
- [Prerequisites](#prerequisites)
- [Step 1: Install openCenter](#step-1-install-opencenter)
- [Step 2: Initialize Your First Cluster](#step-2-initialize-your-first-cluster)
- [Step 3: Explore the Configuration File](#step-3-explore-the-configuration-file)
- [Step 4: Understand the Directory Structure](#step-4-understand-the-directory-structure)
- [Step 5: Validate the Configuration](#step-5-validate-the-configuration)
- [Step 6: Modify a Configuration Value](#step-6-modify-a-configuration-value)
- [Step 7: Check What Files Were Created](#step-7-check-what-files-were-created)
- [Verify Your Work](#verify-your-work)
- [What You Learned](#what-you-learned)
- [Next Steps](#next-steps)
- [Common Questions](#common-questions)
- [Troubleshooting](#troubleshooting)
**doc_type: tutorial**

Deploy your first Kubernetes cluster configuration in 15 minutes. You'll initialize a cluster, validate it, and understand the basic openCenter workflow.

## What You'll Build

By the end of this tutorial, you'll have:
- A validated cluster configuration file
- SOPS encryption keys for secrets management
- SSH key pairs for cluster access
- An understanding of openCenter's organization-based directory structure

You won't deploy actual infrastructure yet—this tutorial focuses on configuration and validation. For deployment, see the [OpenStack Deployment](openstack-deployment.md) or [Kind Local Development](kind-local-dev.md) tutorials.

## Prerequisites

Before starting, you need:
- **mise** installed ([mise.jdx.dev](https://mise.jdx.dev))
- **git** installed
- A text editor (vim, nano, VS Code, etc.)
- 10 minutes of time

No cloud provider account is required for this tutorial.

## Step 1: Install openCenter

Clone the repository and build the binary:

```bash
git clone https://github.com/rackerlabs/openCenter-cli.git
cd openCenter-cli
mise install
mise run build
```

You should see output like:

```
Built openCenter 0.0.1 (a1b2c3d)
```

The binary is now at `bin/openCenter`. Add it to your PATH or use the full path in commands.

## Step 2: Initialize Your First Cluster

Create a cluster configuration named `my-first-cluster`:

```bash
./bin/openCenter cluster init my-first-cluster
```

You'll see output showing what was created:

```
Generated ed25519 SSH key pair at ~/.config/openCenter/clusters/opencenter/secrets/ssh/my-first-cluster-dev-sjc3
Created cluster configuration in organization 'opencenter' at '~/.config/openCenter/clusters/opencenter/infrastructure/clusters/my-first-cluster'
GitOps repository root: ~/.config/openCenter/clusters/opencenter
SOPS key location: ~/.config/openCenter/clusters/opencenter/secrets/age/keys/my-first-cluster-key.txt
```

This command:
- Created a configuration file with sensible defaults
- Generated SOPS Age encryption keys for secrets
- Generated SSH key pairs for cluster access
- Initialized a git repository structure for GitOps

## Step 3: Explore the Configuration File

The configuration file is at:

```
~/.config/openCenter/clusters/opencenter/.my-first-cluster-config.yaml
```

Open it in your editor:

```bash
cat ~/.config/openCenter/clusters/opencenter/.my-first-cluster-config.yaml
```

You'll see a YAML structure with these main sections:

- **opencenter.meta**: Cluster metadata (name, environment, region, organization)
- **opencenter.cluster**: Kubernetes settings (version, node counts, networking)
- **opencenter.infrastructure**: Cloud provider configuration (OpenStack by default)
- **opencenter.gitops**: GitOps repository settings
- **opencenter.services**: Enabled services (Calico, FluxCD, etc.)
- **secrets**: Paths to encryption keys and credentials

The defaults create a 3-master, 2-worker OpenStack cluster with Calico networking.

## Step 4: Understand the Directory Structure

openCenter uses an organization-based structure:

```
~/.config/openCenter/clusters/
└── opencenter/                          # Organization name
    ├── .my-first-cluster-config.yaml    # Cluster configuration
    ├── infrastructure/
    │   └── clusters/
    │       └── my-first-cluster/        # Cluster-specific files
    ├── applications/
    │   └── overlays/
    │       └── my-first-cluster/        # Application manifests
    └── secrets/
        ├── age/
        │   └── keys/
        │       └── my-first-cluster-key.txt  # SOPS encryption key
        └── ssh/
            └── my-first-cluster-dev-sjc3     # SSH keys
```

This structure supports:
- Multiple clusters per organization
- Shared GitOps repository root
- Centralized secrets management
- Easy multi-cluster workflows

## Step 5: Validate the Configuration

Check that your configuration is valid:

```bash
./bin/openCenter cluster validate my-first-cluster
```

You should see:

```
Validation successful.
```

If validation fails, you'll see specific error messages. Common issues:
- Missing required fields
- Invalid CIDR ranges
- Conflicting network settings

The validator checks:
- Schema compliance
- Required field presence
- Cross-field dependencies
- Network configuration validity
- SOPS key availability

## Step 6: Modify a Configuration Value

Try changing the Kubernetes version using the command line:

```bash
./bin/openCenter cluster init my-first-cluster \
  --force \
  --opencenter.cluster.kubernetes.version=1.31.4
```

The `--force` flag overwrites the existing configuration. The dot notation lets you set any configuration value.

Validate again:

```bash
./bin/openCenter cluster validate my-first-cluster
```

## Step 7: Check What Files Were Created

List the files openCenter created:

```bash
# Configuration file
ls -la ~/.config/openCenter/clusters/opencenter/.my-first-cluster-config.yaml

# SOPS encryption key
ls -la ~/.config/openCenter/clusters/opencenter/secrets/age/keys/my-first-cluster-key.txt

# SSH keys
ls -la ~/.config/openCenter/clusters/opencenter/secrets/ssh/my-first-cluster-dev-sjc3*

# GitOps repository
ls -la ~/.config/openCenter/clusters/opencenter/.git
```

All files exist and have appropriate permissions:
- Configuration: `0600` (read/write for owner only)
- SOPS key: `0600`
- SSH private key: `0600`
- SSH public key: `0644`

## Verify Your Work

Run these checks to confirm everything worked:

1. **Configuration exists and is valid:**
   ```bash
   ./bin/openCenter cluster validate my-first-cluster
   ```

2. **SOPS key is readable:**
   ```bash
   cat ~/.config/openCenter/clusters/opencenter/secrets/age/keys/my-first-cluster-key.txt
   ```
   You should see an Age key starting with `AGE-SECRET-KEY-`.

3. **SSH keys are properly formatted:**
   ```bash
   ssh-keygen -l -f ~/.config/openCenter/clusters/opencenter/secrets/ssh/my-first-cluster-dev-sjc3.pub
   ```
   You should see key fingerprint information.

4. **Git repository initialized:**
   ```bash
   git -C ~/.config/openCenter/clusters/opencenter status
   ```
   You should see git status output.

## What You Learned

You now understand:

1. **Configuration-first workflow**: openCenter starts with a declarative YAML file
2. **Organization structure**: Clusters are organized by organization for multi-cluster management
3. **Automatic key generation**: SOPS and SSH keys are created automatically
4. **Validation**: Configuration is validated before deployment
5. **GitOps foundation**: The directory structure is ready for GitOps workflows

## Next Steps

Now that you have a valid configuration, you can:

1. **Deploy locally**: Try the [Kind Local Development](kind-local-dev.md) tutorial to test without cloud resources
2. **Deploy to OpenStack**: Follow the [OpenStack Deployment](openstack-deployment.md) tutorial for production deployment
3. **Customize configuration**: Read the [Configuration Reference](../reference/configuration.md) to understand all options
4. **Manage multiple clusters**: Learn about [Multi-Cluster Management](multi-cluster.md)

## Common Questions

**Q: Can I use a different organization name?**

Yes. Use the `--org` flag:
```bash
./bin/openCenter cluster init my-cluster --org production
```

**Q: Where are my credentials stored?**

Credentials go in the `secrets` section of your configuration file. Use SOPS to encrypt them before committing to git. See [Secrets Management](../how-to/secrets-management.md).

**Q: Can I change the default directory?**

Yes. Set the `OPENCENTER_CONFIG_DIR` environment variable:
```bash
export OPENCENTER_CONFIG_DIR=/custom/path
./bin/openCenter cluster init my-cluster
```

**Q: What if I want to start from an existing config file?**

Use the `--config` flag:
```bash
./bin/openCenter cluster init --config my-template.yaml
```

The cluster name is extracted from the config file automatically.

## Troubleshooting

**"cluster configuration directory already exists"**

Use `--force` to overwrite:
```bash
./bin/openCenter cluster init my-cluster --force
```

**"invalid cluster name"**

Cluster names must:
- Start with an alphanumeric character
- Contain only alphanumeric characters, dots, hyphens, and underscores
- Be 255 characters or less

**"validation failed"**

Read the error messages carefully. They indicate which fields are invalid and why. Common issues:
- Empty required fields
- Invalid IP addresses or CIDR ranges
- Conflicting network settings

Check the [Troubleshooting Guide](../how-to/troubleshooting.md) for more help.
