# Frequently Asked Questions


## Table of Contents

- [Installation and Setup](#installation-and-setup)
- [Configuration](#configuration)
- [Validation](#validation)
- [GitOps and Deployment](#gitops-and-deployment)
- [Secrets Management](#secrets-management)
- [Providers](#providers)
- [Troubleshooting](#troubleshooting)
- [Performance and Scaling](#performance-and-scaling)
- [Advanced Usage](#advanced-usage)
- [Comparison with Other Tools](#comparison-with-other-tools)
- [Getting Help](#getting-help)
- [Related Documentation](#related-documentation)
**doc_type**: explanation

This document answers common questions about openCenter, organized by topic. If you don't find your answer here, check the [Troubleshooting Guide](../how-to/troubleshooting.md) or open an issue on GitHub.

## Installation and Setup

### How do I install openCenter?

openCenter requires Mise for tool version management. Install Mise first, then:

```bash
git clone <repository-url>
cd openCenter-cli
mise install
mise run build
```

The binary will be in `bin/openCenter`. Add it to your PATH or run it directly.

### What are the system requirements?

- Go 1.25.2 or later (managed by Mise)
- Git for version control
- 2GB RAM minimum for building
- Network access to cloud providers during validation and bootstrap

For deployment, you'll also need credentials for your target cloud provider (OpenStack, AWS, vSphere, or Kind for local development).

### Do I need to install Terraform or Ansible separately?

No. openCenter generates Terraform/OpenTofu and Ansible configurations, but you run them through Mise tasks. The required tools are specified in `.mise.toml` and installed with `mise install`.

For OpenTofu: `mise run tofu-apply`  
For Ansible: `mise run ansible-deploy`

### Can I use openCenter without Mise?

Technically yes, but it's not recommended. Mise ensures consistent tool versions across your team and provides task automation. Without it, you'd need to manually install Go, OpenTofu, Ansible, and other dependencies at the correct versions.

## Configuration

### Where is my cluster configuration stored?

Configurations are stored in `~/.config/openCenter/clusters/<organization>/<cluster>/.<cluster>-config.yaml`.

The organization-based structure prevents cluster name collisions when multiple teams use openCenter. If you don't specify an organization, openCenter uses a default organization name.

### Can I have multiple clusters?

Yes. Each cluster gets its own configuration file and directory. Use `openCenter cluster list` to see all clusters and `openCenter cluster select <name>` to switch between them.

### How do I share configuration across clusters?

openCenter doesn't support configuration inheritance or includes. Each cluster has a complete, self-contained configuration file.

For shared settings, use a template or script to generate multiple configurations. This keeps each cluster's configuration explicit and auditable.

### What's the difference between the config file and the GitOps repository?

The config file (`.opencenter-config.yaml`) is the source of truth. It describes what you want.

The GitOps repository is generated from the config file. It contains the actual Kubernetes manifests, Terraform code, and Ansible playbooks that implement what you want.

When you change the config file, run `openCenter cluster setup` to regenerate the GitOps repository.

## Validation

### Why does validation fail even though my YAML is correct?

YAML syntax correctness doesn't guarantee semantic correctness. Validation checks:

- Required fields are present
- Values are in valid ranges (e.g., master_count >= 1)
- Cloud provider configuration is complete
- Network subnets don't overlap
- Only one network plugin is enabled

Run `openCenter cluster validate <name>` to see specific errors and suggestions.

### Can I skip validation?

No. Validation runs automatically before operations that modify infrastructure. This prevents deploying broken configurations that would require manual cleanup.

You can skip connectivity checks with `--skip-connectivity` if you're in an environment without cloud provider access (like CI/CD).

### What's the difference between errors and warnings?

Errors block deployment. You must fix them before proceeding.

Warnings indicate potential issues but don't block deployment. For example, connectivity check failures are warnings because they might be environmental (no VPN connection) rather than configuration errors.

### How do I fix validation errors?

Each validation error includes suggestions. For example:

```
Error: invalid cluster name format
Field: opencenter.cluster.cluster_name
Suggestions:
  1. Use alphanumeric characters, hyphens, and underscores only
  2. Start with an alphanumeric character
  3. Keep length under 255 characters
```

Follow the suggestions to fix the error, then run validation again.

## GitOps and Deployment

### What is GitOps and why does openCenter use it?

GitOps treats Git as the source of truth for infrastructure. Every change flows through a Git commit. A tool like FluxCD watches the repository and automatically applies changes to the cluster.

openCenter uses GitOps because it provides:

- Version control for all cluster changes
- Audit trail of who changed what and when
- Automated rollback (revert the commit)
- Drift detection (cluster self-heals from manual changes)

See [GitOps Workflow](gitops-workflow.md) for details.

### Do I have to use GitOps?

For production clusters, yes. GitOps is how openCenter manages cluster state.

For local development with Kind, you can skip the GitOps repository and apply manifests directly. But you lose the benefits of version control and automated reconciliation.

### Can I use ArgoCD instead of FluxCD?

Yes. openCenter generates FluxCD-compatible manifests by default, but the repository structure works with ArgoCD. You'll need to create ArgoCD Application resources that point to the `applications/overlays/<cluster-name>/` directory.

### How do I update my cluster after changing the configuration?

1. Edit `.opencenter-config.yaml`
2. Run `openCenter cluster setup <name>` to regenerate the GitOps repository
3. Review changes with `git diff`
4. Commit and push: `git commit -am "Update configuration" && git push`
5. FluxCD detects the commit and applies changes automatically

### What if I need to make an emergency change?

You can run `kubectl` directly, but FluxCD will undo your change on the next reconciliation (typically 5-10 minutes).

To make the change stick:
- Suspend reconciliation: `flux suspend kustomization <name>`
- Make your change with `kubectl`
- Update the GitOps repository to match
- Resume reconciliation: `flux resume kustomization <name>`

Or commit the change to Git before the next reconciliation cycle.

## Secrets Management

### How does SOPS encryption work?

SOPS encrypts values in YAML files while preserving structure. You can see field names but not values:

```yaml
password: ENC[AES256_GCM,data:xK8...,iv:...,tag:...,type:str]
```

openCenter uses Age encryption (modern, simple cryptography). Generate a key with `openCenter sops generate-key`, then SOPS uses it automatically.

### Where are encryption keys stored?

Age keys are stored in `~/.config/openCenter/clusters/<organization>/secrets/age/<cluster>-key.txt`.

This is separate from the GitOps repository. The repository contains encrypted secrets, but not the keys to decrypt them. This separation means you can make the GitOps repository public—encrypted secrets are safe without the key.

### Do I have to encrypt all secrets?

For production, yes. openCenter validates that sensitive fields (passwords, API keys, tokens) are encrypted before deployment.

For local development, you can use plaintext secrets, but you'll get warnings.

### How do I rotate encryption keys?

1. Generate a new Age key: `openCenter sops generate-key --output new-key.txt`
2. Re-encrypt secrets with the new key: `sops updatekeys --input-type yaml --output-type yaml <file>`
3. Update `.sops.yaml` to reference the new key
4. Delete the old key after verifying decryption works

Key rotation doesn't require redeploying the cluster—just re-encrypting the secrets files.

### Can I use Vault or other secret stores?

SOPS handles secrets in Git. Vault handles secrets at runtime. You can use both:

- SOPS for GitOps repository secrets (credentials needed during bootstrap)
- Vault for application runtime secrets (database passwords, API keys)

openCenter doesn't integrate with Vault directly, but you can configure applications to use Vault after deployment.

## Providers

### Which cloud providers are supported?

- **OpenStack**: Full support with OpenTofu provisioning
- **AWS**: Full support with OpenTofu provisioning
- **vSphere**: Full support with OpenTofu provisioning
- **Kind**: Local development clusters (no cloud provisioning)

Each provider has specific configuration requirements. See the provider-specific documentation in `docs/providers/`.

### Can I use openCenter with bare metal?

Not directly. openCenter assumes cloud provider APIs for provisioning.

For bare metal, you'd need to:
1. Provision servers manually or with a separate tool
2. Use openCenter to generate Kubernetes manifests
3. Apply manifests to your pre-provisioned infrastructure

This workflow isn't officially supported but is technically possible.

### How do I switch providers?

Each cluster is tied to one provider. To switch providers, create a new cluster configuration with the new provider.

You can't change a cluster's provider after creation—the infrastructure code is provider-specific.

### What if my provider isn't supported?

openCenter uses a provider adapter pattern. Adding a new provider means:

1. Implementing the `CloudProviderValidator` interface
2. Creating provider-specific templates
3. Registering the provider in the validator

See `docs/dev/adding-providers.md` for details (if you're interested in contributing).

## Troubleshooting

### Validation passes but bootstrap fails. Why?

Validation checks configuration correctness, not runtime conditions. Bootstrap can fail due to:

- Insufficient cloud provider quotas
- Unavailable VM flavors or images
- Network connectivity issues
- Credential expiration

Run preflight checks before bootstrap: `openCenter cluster preflight <name>`. This performs deeper checks including API connectivity and resource availability.

### How do I debug template rendering errors?

Template errors include line numbers and context. Look for the error message, then check the template file at that line.

Common template errors:
- Undefined variables: Check that the field exists in your configuration
- Type mismatches: Ensure you're using the right data type (string vs. int)
- Syntax errors: Verify Go template syntax (especially closing brackets)

### My cluster is stuck in a bad state. How do I recover?

For GitOps issues:
1. Check FluxCD status: `flux get kustomizations`
2. Look for reconciliation errors: `flux logs`
3. Suspend problematic kustomizations: `flux suspend kustomization <name>`
4. Fix the issue in Git, commit, and push
5. Resume: `flux resume kustomization <name>`

For infrastructure issues:
1. Check Terraform state: `mise run tofu-state-list`
2. Manually fix resources in cloud provider console
3. Import fixed resources: `mise run tofu-import <resource> <id>`
4. Re-run provisioning: `mise run tofu-apply`

### Where are logs stored?

openCenter logs to stdout/stderr. Redirect to a file if needed:

```bash
openCenter cluster bootstrap my-cluster 2>&1 | tee bootstrap.log
```

For component logs:
- FluxCD: `flux logs`
- Kubernetes: `kubectl logs -n <namespace> <pod>`
- OpenTofu: Check `.terraform/` directory in infrastructure path

### How do I report a bug?

1. Check if it's a known issue: `docs/explanation/known-issues.md`
2. Search existing GitHub issues
3. If not found, open a new issue with:
   - openCenter version: `openCenter version`
   - Command that failed
   - Full error output
   - Configuration file (redact secrets)
   - Provider and environment details

## Performance and Scaling

### How long does cluster bootstrap take?

Typical bootstrap times:

- Kind (local): 2-5 minutes
- OpenStack (3 masters, 3 workers): 15-30 minutes
- AWS (3 masters, 3 workers): 20-40 minutes

Time varies based on cloud provider performance, VM sizes, and network speed.

### Can I bootstrap multiple clusters in parallel?

Yes, but be aware of:

- Cloud provider API rate limits
- Quota limits (you might exhaust quotas)
- Local resource usage (each bootstrap uses CPU/memory)

Run bootstraps sequentially if you hit rate limits or quota issues.

### How many clusters can openCenter manage?

There's no hard limit. The organization-based directory structure scales to hundreds of clusters.

Practical limits depend on:
- Filesystem performance (hundreds of config files)
- Git repository size (if you store all GitOps repos in one place)
- Your ability to manage many clusters (operational complexity)

### Does openCenter support cluster upgrades?

Not directly. openCenter creates clusters but doesn't manage upgrades.

To upgrade Kubernetes:
1. Update `kubernetes.version` in your config
2. Regenerate the GitOps repository
3. Follow your provider's upgrade procedure (usually involves draining nodes and updating one at a time)

Automated upgrades are planned for a future release.

## Advanced Usage

### Can I customize the generated templates?

Templates are embedded in the binary. You can't modify them without rebuilding openCenter.

For customization:
- Use configuration overrides (most common needs)
- Apply Kustomize overlays after generation
- Fork openCenter and modify templates

The embedded approach ensures version consistency—template version always matches CLI version.

### How do I add custom services?

After generating the GitOps repository:

1. Add service manifests to `applications/overlays/<cluster>/services/`
2. Create a FluxCD Kustomization in `applications/overlays/<cluster>/fluxcd/`
3. Commit and push

FluxCD will deploy your custom service alongside openCenter-managed services.

### Can I use openCenter in CI/CD?

Yes. openCenter is designed for automation:

- Non-interactive mode (all flags, no prompts)
- Exit codes indicate success/failure
- Structured output (JSON with `--output json`)
- Skip connectivity checks in restricted environments

Example CI/CD workflow:
```bash
openCenter cluster validate my-cluster --skip-connectivity
openCenter cluster setup my-cluster
git -C <gitops-dir> commit -am "Update from CI"
git -C <gitops-dir> push
```

### How do I integrate with existing infrastructure?

openCenter assumes it controls the infrastructure. For existing infrastructure:

- Import existing resources into Terraform state
- Use openCenter for new clusters, not existing ones
- Or use openCenter to generate manifests only (skip infrastructure provisioning)

Full integration with existing infrastructure isn't a primary use case.

## Comparison with Other Tools

### How is openCenter different from Terraform?

Terraform provisions infrastructure. openCenter generates Terraform code from high-level configuration.

You could write Terraform directly, but openCenter provides:
- Validation before provisioning
- Opinionated best practices
- Integrated secrets management
- GitOps repository generation

Think of openCenter as a layer above Terraform that handles the full cluster lifecycle.

### How is openCenter different from Kubespray?

Kubespray deploys Kubernetes using Ansible. openCenter generates Kubespray configurations and orchestrates the deployment.

openCenter adds:
- Configuration validation
- Multi-provider support
- GitOps integration
- Secrets management

Kubespray is one of the provisioning engines openCenter uses.

### How is openCenter different from Cluster API?

Cluster API is a Kubernetes-native way to manage cluster lifecycle. It runs inside Kubernetes and uses custom resources.

openCenter is a CLI tool that generates infrastructure code. It doesn't require an existing Kubernetes cluster to create new clusters.

The approaches are complementary—you could use openCenter to bootstrap a management cluster, then use Cluster API for workload clusters.

### Should I use openCenter or Helm?

Different purposes. Helm deploys applications to existing clusters. openCenter creates clusters and generates the GitOps repository that includes Helm charts.

You use both: openCenter creates the cluster, Helm (via FluxCD) deploys applications.

## Getting Help

### Where can I find more documentation?

- **Tutorials**: Step-by-step learning guides in `docs/tutorials/`
- **How-To Guides**: Task-specific instructions in `docs/how-to/`
- **Reference**: Complete CLI and configuration reference in `docs/reference/`
- **Explanations**: Conceptual documentation in `docs/explanation/`

### How do I get support?

1. Check this FAQ
2. Read the [Troubleshooting Guide](../how-to/troubleshooting.md)
3. Search GitHub issues
4. Open a new issue with details

For security issues, see `SECURITY.md` for responsible disclosure.

### How can I contribute?

See [Contributing Guide](../contributing.md) for:
- Code contribution process
- Documentation improvements
- Bug reports and feature requests
- Development setup

### Is there a community?

Check the GitHub repository for:
- Discussions (Q&A, ideas, show-and-tell)
- Issues (bugs, feature requests)
- Pull requests (contributions)

## Related Documentation

- [Troubleshooting Guide](../how-to/troubleshooting.md) - Detailed problem-solving steps
- [Known Issues](known-issues.md) - Current limitations and workarounds
- [Architecture](architecture.md) - How openCenter works internally
- [GitOps Workflow](gitops-workflow.md) - Understanding the GitOps approach
- [CLI Commands Reference](../reference/cli-commands.md) - Complete command documentation
