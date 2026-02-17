# openCenter Documentation

Welcome to the openCenter documentation. openCenter is a CLI tool that streamlines Kubernetes cluster bootstrapping by transforming a single declarative YAML configuration into a production-ready GitOps repository.

## Quick Start

New to openCenter? Start here:

- **[Getting Started Tutorial](tutorials/getting-started.md)** - Create your first cluster in 10 minutes
- **[CLI Commands Reference](reference/cli-commands.md)** - Quick command reference
- **[Configuration Schema](reference/configuration-schema.md)** - Configuration file structure

## Documentation by Type

### Tutorials (Learning-Oriented)

Step-by-step guides to help you learn openCenter:

- **[Getting Started](tutorials/getting-started.md)** - Create your first cluster
- **[OpenStack First Cluster](tutorials/openstack-first-cluster.md)** - Deploy on OpenStack
- **[Kind Local Development](tutorials/kind-local-development.md)** - Local development setup
- **[VMware Deployment](tutorials/vmware-deployment.md)** - Deploy on pre-provisioned VMs
- **[Multi-Cluster Setup](tutorials/multi-cluster-setup.md)** - Manage multiple clusters

### How-To Guides (Task-Oriented)

Practical guides for specific tasks:

- **[Validate Configuration](how-to/validate-configuration.md)** - Validate before deployment
- **[Manage Secrets](how-to/manage-secrets.md)** - SOPS and secrets management
- **[Customize Services](how-to/customize-services.md)** - Configure platform services
- **[Configure Networking](how-to/configure-networking.md)** - CNI and network setup
- **[Add Worker Pools](how-to/add-worker-pools.md)** - Additional worker node pools
- **[Backup and Restore](how-to/backup-and-restore.md)** - Disaster recovery
- **[Upgrade Kubernetes](how-to/upgrade-kubernetes.md)** - Safe version upgrades
- **[Troubleshoot Deployment](how-to/troubleshoot-deployment.md)** - Common issues
- **[Migrate Clusters](how-to/migrate-clusters.md)** - Provider/region migration
- **[Integrate CI/CD](how-to/integrate-ci-cd.md)** - Pipeline integration

### Reference (Information-Oriented)

Technical specifications and detailed information:

- **[CLI Commands](reference/cli-commands.md)** - Complete command reference
- **[Configuration Schema](reference/configuration-schema.md)** - Configuration structure
- **[Default Values](reference/default-values.md)** - Default configuration values
- **[Platform Services](reference/platform-services.md)** - Available services
- **[Providers](reference/providers.md)** - Infrastructure providers
- **[Mise Tasks](reference/mise-tasks.md)** - Development tasks
- **[Validation Rules](reference/validation-rules.md)** - Configuration constraints
- **[Environment Variables](reference/environment-variables.md)** - Environment configuration
- **[Exit Codes](reference/exit-codes.md)** - CLI exit codes
- **[File Locations](reference/file-locations.md)** - Configuration paths

### Explanation (Understanding-Oriented)

Conceptual explanations and background:

- **[Architecture](explanation/architecture.md)** - System design and decisions
- **[GitOps Workflow](explanation/gitops-workflow.md)** - Repository structure
- **[Security Model](explanation/security-model.md)** - Security architecture
- **[Provider Comparison](explanation/provider-comparison.md)** - Choosing providers
- **[Configuration Lifecycle](explanation/configuration-lifecycle.md)** - Config management

## For Developers

- **[Contributing](dev/contributing.md)** - Contribution guidelines
- **[Development Setup](dev/development-setup.md)** - Dev environment
- **[Testing Guide](dev/testing-guide.md)** - Writing tests
- **[Adding Providers](dev/adding-providers.md)** - New providers
- **[Adding Services](dev/adding-services.md)** - New services
- **[Code Structure](dev/code-structure.md)** - Codebase organization
- **[Build System](dev/build-system.md)** - Mise-based builds
- **[Release Process](dev/release-process.md)** - Creating releases

## Common Tasks

### First-Time Setup
1. [Install openCenter](tutorials/getting-started.md#installation)
2. [Initialize your first cluster](tutorials/getting-started.md#initialize-cluster)
3. [Validate configuration](how-to/validate-configuration.md)
4. [Deploy cluster](tutorials/getting-started.md#deploy-cluster)

### Day-2 Operations
- [Add worker nodes](how-to/add-worker-pools.md)
- [Upgrade Kubernetes version](how-to/upgrade-kubernetes.md)
- [Rotate secrets](how-to/manage-secrets.md#rotate-keys)
- [Backup etcd](how-to/backup-and-restore.md)

### Troubleshooting
- [Validation errors](how-to/troubleshoot-deployment.md#validation-errors)
- [Deployment failures](how-to/troubleshoot-deployment.md#deployment-failures)
- [Network issues](how-to/troubleshoot-deployment.md#network-issues)
- [Service failures](how-to/troubleshoot-deployment.md#service-failures)

## Getting Help

- **GitHub Issues:** [Report bugs or request features](https://github.com/rackerlabs/openCenter-cli/issues)
- **Documentation Issues:** Found a problem in the docs? [Open an issue](https://github.com/rackerlabs/openCenter-cli/issues/new)

## Documentation Framework

This documentation follows the [Diátaxis framework](https://diataxis.fr/), organizing content into four types:

- **Tutorials:** Learning-oriented, step-by-step guides for beginners
- **How-To Guides:** Task-oriented, practical guides for specific goals
- **Reference:** Information-oriented, technical specifications for lookup
- **Explanation:** Understanding-oriented, conceptual background and rationale

Each document has a single, clear purpose. If you need to learn, start with tutorials. If you need to accomplish a task, use how-to guides. If you need to look up details, check reference. If you need to understand concepts, read explanations.

---

## Evidence

This documentation hub is based on:

- Existing README structure: `README.md:30-82`
- Diátaxis documentation map: Session 2 B1
- Product overview: `.kiro/steering/product.md:1-35`
- Documentation framework: Diátaxis (https://diataxis.fr/)
