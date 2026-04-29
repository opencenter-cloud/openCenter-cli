# openCenter CLI

**openCenter** is a command-line tool that transforms a single declarative YAML configuration into a production-ready Kubernetes cluster with GitOps management.

It standardizes cluster deployment across OpenStack, VMware, Baremetal, and Kind, providing configuration validation, secrets management, and automated GitOps repository generation.

## What openCenter Does

- **Configuration-First Workflow:** Single YAML file defines your entire cluster (infrastructure, Kubernetes, services, secrets)
- **Multi-Provider Support:** Deploy to OpenStack, VMware, Baremetal, or Kind with the same configuration structure
- **Built-in Validation:** Schema validation, business rules, and provider-specific checks catch errors before deployment
- **GitOps Native:** Generates complete FluxCD-ready repository with Kustomize overlays for cluster-specific customization
- **Secrets Management:** SOPS Age encryption for safe version control of sensitive data
- **Platform Services:** 20+ pre-configured services (monitoring, logging, ingress, auth, storage, backup)

## Quick Start

```bash
# Install tools
mise install

# Build CLI
mise run build

# Initialize cluster
./bin/opencenter cluster init my-cluster --org my-org

# Edit configuration
$EDITOR ~/.config/opencenter/clusters/my-org/.my-cluster-config.yaml

# Validate
./bin/opencenter cluster validate my-cluster

# Generate GitOps repository
./bin/opencenter cluster generate my-cluster

# Deploy
./bin/opencenter cluster deploy my-cluster
```

**Time to first cluster:** 10 minutes configuration + 30-50 minutes deployment

See [Getting Started Tutorial](docs/tutorials/getting-started.md) for complete walkthrough.

## Key Capabilities

- **Cluster Lifecycle:** Initialize, validate, setup, bootstrap, update, destroy
- **Configuration Management:** Schema-driven with defaults, validation, and override capabilities
- **Secrets Operations:** Generate keys, encrypt/decrypt, rotate, check expiration
- **GitOps Repository:** Automated generation with infrastructure (Terraform/Kubespray) and applications (FluxCD/Kustomize)
- **Provider Abstraction:** Unified interface across OpenStack, VMware, Baremetal, and Kind
- **Service Management:** Enable/disable platform services, customize configurations
- **Operational Tools:** Drift detection, backup/restore, audit logging, preflight checks

## Configuration Example

```yaml
opencenter:
  cluster:
    cluster_name: production
    organization: acme-corp
  
  infrastructure:
    provider: openstack
    cloud:
      openstack:
        auth_url: https://identity.api.rackspacecloud.com/v3
        region: sjc3
        application_credential_id: ${OPENSTACK_APP_CRED_ID}
        application_credential_secret: ${OPENSTACK_APP_CRED_SECRET}
  
  kubernetes:
    version: 1.33.5
    control_plane_count: 3
    worker_count: 2
    cni: calico
  
  services:
    keycloak:
      enabled: true
    kube-prometheus-stack:
      enabled: true
    loki:
      enabled: true
    velero:
      enabled: true

secrets:
  sops:
    age_keys:
      - age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
```

See [Configuration Schema Reference](docs/reference/configuration-schema.md) for complete structure.

## Documentation

Comprehensive documentation organized by the [Diátaxis framework](https://diataxis.fr/):

### 📚 [Tutorials](docs/tutorials/) (Learning-Oriented)
Step-by-step guides for learning openCenter:
- [Getting Started](docs/tutorials/getting-started.md) - Your first cluster in 10 minutes
- [OpenStack First Cluster](docs/tutorials/openstack-first-cluster.md) - Deploy on OpenStack
- [Kind Local Development](docs/tutorials/kind-local-development.md) - Local development setup
- [VMware Deployment](docs/tutorials/vmware-deployment.md) - Deploy on pre-provisioned VMs
- [Multi-Cluster Management](docs/tutorials/multi-cluster-setup.md) - Manage multiple clusters

### 🔧 [How-To Guides](docs/how-to/) (Task-Oriented)
Practical guides for specific tasks:
- [Validate Configuration](docs/how-to/validate-configuration.md) - Pre-deployment validation
- [Manage Secrets](docs/how-to/manage-secrets.md) - SOPS encryption and key rotation
- [Customize Services](docs/how-to/customize-services.md) - Configure platform services
- [Add Worker Pools](docs/how-to/add-worker-pools.md) - Scale your cluster
- [Backup and Restore](docs/how-to/backup-and-restore.md) - Disaster recovery
- [Upgrade Kubernetes](docs/how-to/upgrade-kubernetes.md) - Safe version upgrades
- [Troubleshoot Deployment](docs/how-to/troubleshoot-deployment.md) - Common issues and solutions
- [Migrate Clusters](docs/how-to/migrate-clusters.md) - Provider/region migration
- [Integrate CI/CD](docs/how-to/integrate-ci-cd.md) - Pipeline integration

### 📖 [Reference](docs/reference/) (Information-Oriented)
Technical specifications and detailed information:
- [CLI Commands](docs/reference/cli-commands.md) - Complete command reference
- [Configuration Schema](docs/reference/configuration-schema.md) - Configuration file structure
- [Default Values](docs/reference/default-values.md) - Default configuration values
- [Platform Services](docs/reference/platform-services.md) - Available services and versions
- [Providers](docs/reference/providers.md) - Infrastructure provider details
- [Validation Rules](docs/reference/validation-rules.md) - Configuration constraints
- [Environment Variables](docs/reference/environment-variables.md) - Environment configuration
- [Exit Codes](docs/reference/exit-codes.md) - CLI exit codes and meanings
- [File Locations](docs/reference/file-locations.md) - Configuration file paths
- [Mise Tasks](docs/reference/mise-tasks.md) - Development and build tasks

### 💡 [Explanation](docs/explanation/) (Understanding-Oriented)
Conceptual explanations and background:
- [Architecture](docs/explanation/architecture.md) - System design and decisions
- [GitOps Workflow](docs/explanation/gitops-workflow.md) - Repository structure and reconciliation
- [Security Model](docs/explanation/security-model.md) - Security architecture and controls
- [Configuration Lifecycle](docs/explanation/configuration-lifecycle.md) - Configuration management
- [Provider Comparison](docs/explanation/provider-comparison.md) - Choosing infrastructure providers
- [Drift Detection](docs/explanation/drift-detection.md) - Configuration drift and reconciliation

### 🛠️ [Developer Documentation](docs/dev/)
For contributors and developers:
- [Contributing Guide](docs/dev/contributing.md) - Contribution guidelines
- [Development Setup](docs/dev/development-setup.md) - Local development environment
- [Testing Guide](docs/dev/testing-guide.md) - Writing and running tests
- [Code Structure](docs/dev/code-structure.md) - Codebase organization
- [Adding Providers](docs/dev/adding-providers.md) - Implement new infrastructure providers
- [Adding Services](docs/dev/adding-services.md) - Add new platform services
- [Build System](docs/dev/build-system.md) - Mise-based build and release
- [Release Process](docs/dev/release-process.md) - Creating releases

**Start here:** [Documentation Home](docs/index.md) | [Navigation Guide](docs/_nav.md)

## CLI Commands Quick Reference

```bash
# Cluster Management
opencenter cluster init <name>              # Initialize new cluster
opencenter cluster list                     # List all clusters
opencenter cluster use <name>            # Set active cluster
opencenter cluster validate <name>          # Validate configuration
opencenter cluster generate <name>    # Generate GitOps repository
opencenter cluster deploy <name>         # Deploy cluster

# Configuration
opencenter cluster describe <name>          # View cluster configuration
opencenter cluster set <name> <path=value>  # Update configuration value
opencenter cluster edit <name>              # Edit in $EDITOR

# Secrets Management
opencenter secrets keys generate                # Generate Age key pair
opencenter secrets encrypt             # Encrypt secrets
opencenter secrets keys rotate --cluster <cluster> --type age   # Rotate encryption keys
opencenter secrets keys check               # Check key expiration

# Operations
opencenter cluster status                   # Show cluster status
opencenter cluster describe <name>              # Detailed cluster description
opencenter cluster drift                    # Detect configuration drift
opencenter cluster backup                   # Backup configuration
opencenter cluster destroy <name>           # Destroy cluster

# Utilities
opencenter version                          # Show version information
opencenter --help                           # Show help
```

See [CLI Commands Reference](docs/reference/cli-commands.md) for complete documentation.

## Development Workflow

### Prerequisites

- [Mise](https://mise.jdx.dev/) - Tool version manager
- [Git](https://git-scm.com/) - Version control
- Go, kubectl, kind, helm (managed by Mise)

### Build and Test

```bash
# Install tools
mise install

# Build binary
mise run build

# Run unit tests
mise run test

# Run BDD tests
mise run godog

# Run property-based tests
mise run test-properties

# Lint code
mise run lint

# Format code
mise run fmt
```

### Development Tasks

```bash
# Build for multiple platforms
mise run build-all

# Create release
mise run release v1.0.0

# Publish official release
git tag -a v1.0.0 -m "Release 1.0.0"
git push origin v1.0.0

# Generate JSON schema
mise run schema

# Validate templates
mise run validate-templates

# Run a named Kind cluster with openCenter-managed CNI
opencenter cluster init dev-cluster --type kind --kind-disable-default-cni
opencenter cluster validate dev-cluster
opencenter cluster generate dev-cluster
opencenter cluster deploy dev-cluster

# Setup local Gitea for testing
mise run gitea-up
```

See [Mise Tasks Reference](docs/reference/mise-tasks.md) for complete list.

Tagged releases are published by GitHub Actions. Use `mise run release` for local preflight builds, then push a `v*` tag to create the signed release artifacts.

## Project Structure

```
openCenter-cli/
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go            # Root command and global flags
│   ├── cluster*.go        # Cluster lifecycle commands
│   ├── secrets*.go        # Secrets management commands
│   └── config*.go         # Configuration commands
├── internal/              # Internal packages
│   ├── config/           # Configuration management
│   ├── gitops/           # GitOps repository generation
│   ├── sops/             # SOPS secrets management
│   ├── cloud/            # Provider adapters (OpenStack, VMware, Kind)
│   ├── provision/        # Provisioning logic (Terraform, Kubespray)
│   ├── template/         # Template engine
│   ├── di/               # Dependency injection
│   └── core/             # Core validation and utilities
├── docs/                  # Documentation (Diátaxis)
│   ├── tutorials/        # Learning-oriented guides
│   ├── how-to/           # Task-oriented guides
│   ├── reference/        # Information-oriented specs
│   ├── explanation/      # Understanding-oriented concepts
│   └── dev/              # Developer documentation
├── tests/                 # BDD tests (Godog)
│   └── features/         # Gherkin feature files
├── schema/                # JSON schema definitions
├── .mise.toml            # Mise configuration and tasks
├── go.mod                # Go module definition
└── main.go               # CLI entrypoint
```

See [Code Structure](docs/dev/code-structure.md) for detailed explanation.

## Configuration File Locations

- **Cluster configurations:** `~/.config/opencenter/clusters/<org>/.<cluster>-config.yaml`
- **CLI configuration:** `~/.config/opencenter/config.yaml`
- **Active cluster:** `~/.config/opencenter/active`
- **SOPS Age keys:** `~/.config/opencenter/clusters/<org>/secrets/age/`
- **SSH keys:** `~/.config/opencenter/clusters/<org>/secrets/ssh/`

Override CLI configuration storage with `OPENCENTER_CONFIG_DIR` and cluster storage with `OPENCENTER_CLUSTERS_DIR`.

See [File Locations Reference](docs/reference/file-locations.md) for complete paths.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENCENTER_CONFIG_DIR` | Configuration directory | `~/.config/opencenter` |
| `OPENCENTER_CLUSTERS_DIR` | Cluster storage directory | `${OPENCENTER_CONFIG_DIR}/clusters` |
| `OPENCENTER_PLUGINS_DIR` | Plugins directory | `${OPENCENTER_CONFIG_DIR}/plugins` |
| `OPENCENTER_DEBUG` | Enable debug logging | `false` |
| `SOPS_AGE_KEY_FILE` | Path to Age key file | |
| `SOPS_AGE_RECIPIENTS` | Age public keys for encryption | |
| `KUBECONFIG` | Kubernetes config file | `~/.kube/config` |

See [Environment Variables Reference](docs/reference/environment-variables.md) for complete list.

## Contributing

We welcome contributions! Please see our [Contributing Guide](docs/dev/contributing.md) to get started.

### Quick Contribution Workflow

1. Fork and clone the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `mise run test && mise run godog`
5. Submit a pull request

### Extension Points

- **Custom Providers:** Add new infrastructure providers in `internal/cloud/<provider>/`
- **Custom Services:** Add platform services in `internal/config/services/<service>.go`
- **Custom Validators:** Add validation rules in `internal/core/validation/validators/`
- **Plugins:** Create external plugins as `opencenter-<plugin>` executables

See [Developer Documentation](docs/dev/) for detailed guides.

## License

This project is licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

## Support

- **Documentation:** [docs/](docs/)
- **Security Policy:** [SECURITY.md](SECURITY.md)
- **Issues:** [GitHub Issues](https://github.com/opencenter-cloud/openCenter-cli/issues)
- **Discussions:** [GitHub Discussions](https://github.com/opencenter-cloud/openCenter-cli/discussions)

## Related Projects

openCenter CLI is part of the openCenter ecosystem:

- **[openCenter-gitops-base](https://github.com/opencenter-cloud/openCenter-gitops-base)** - Platform services library with security-hardened Helm values
- **[openCenter-customer-app-example](https://github.com/opencenter-cloud/openCenter-customer-app-example)** - Reference application deployment patterns
- **[openCenter-AirGap](https://github.com/opencenter-cloud/openCenter-AirGap)** - Air-gapped deployment packaging
- **[opencenter-windows](https://github.com/opencenter-cloud/opencenter-windows)** - Windows worker node support

See [Ecosystem Architecture](../docs/ecosystem.md) for how these repositories work together.

---

**Evidence:**
- Project structure: `openCenter-cli/` directory listing
- Build system: `.mise.toml:1-968`
- CLI entrypoint: `main.go:1-60`
- Go module: `go.mod:1-80`
- Version: `VERSION:1`
- Documentation structure: `docs/` directory listing
- Commands: `cmd/*.go` files
- Configuration: `internal/config/` types and defaults
