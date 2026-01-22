# openCenter Documentation


## Table of Contents

- [What is openCenter?](#what-is-opencenter)
- [Quick Navigation](#quick-navigation)
- [Documentation Structure](#documentation-structure)
- [By User Role](#by-user-role)
- [By Task](#by-task)
- [Provider-Specific Documentation](#provider-specific-documentation)
- [Operations Documentation](#operations-documentation)
- [Development Documentation](#development-documentation)
- [Additional Resources](#additional-resources)
- [Contributing to Documentation](#contributing-to-documentation)
- [Version Information](#version-information)
- [License](#license)
**doc_type: reference**

Welcome to openCenter documentation. This guide helps you find what you need quickly.

## What is openCenter?

openCenter transforms a single YAML configuration into a production-ready Kubernetes cluster with GitOps workflows, secrets management, and multi-cloud support.

## Quick Navigation

**New to openCenter?** Start with [Getting Started](tutorials/getting-started.md) for a 15-minute walkthrough.

**Need to solve a problem?** Check [How-To Guides](how-to/README.md) for task-focused instructions.

**Looking up details?** Browse [Reference](reference/README.md) for complete technical specifications.

**Want to understand concepts?** Read [Explanations](explanation/README.md) for architecture and design decisions.

## Documentation Structure

This documentation follows the [Diátaxis framework](https://diataxis.fr/), organizing content by purpose:

### 📚 [Tutorials](tutorials/README.md)
*Learning by doing*

Step-by-step guides that build confidence through hands-on practice:
- [Getting Started](tutorials/getting-started.md) - Deploy your first cluster in 15 minutes
- [OpenStack Deployment](tutorials/openstack-deployment.md) - Production OpenStack cluster
- [Local Development with Kind](tutorials/kind-local-dev.md) - Test locally before deploying
- [Multi-Cluster Management](tutorials/multi-cluster.md) - Manage multiple clusters

### 🔧 [How-To Guides](how-to/README.md)
*Solving specific problems*

Practical instructions for common tasks:
- [Troubleshooting](how-to/troubleshooting.md) - Diagnose and fix issues
- [Managing Secrets](how-to/secrets-management.md) - SOPS encryption workflows
- [Adding Services](how-to/adding-services.md) - Extend cluster capabilities
- [Upgrading Clusters](how-to/upgrading-clusters.md) - Safe upgrade procedures
- [Backup and Recovery](how-to/backup-recovery.md) - Protect your clusters

### 📖 [Reference](reference/README.md)
*Looking up facts*

Complete technical specifications:
- [CLI Commands](reference/cli-commands.md) - All commands and flags
- [Configuration Schema](reference/configuration.md) - Every configuration option
- [API Reference](reference/api.md) - Go package documentation
- [Error Codes](reference/error-codes.md) - Error messages and solutions

### 💡 [Explanation](explanation/README.md)
*Understanding concepts*

Architecture, design decisions, and background:
- [Architecture Overview](explanation/architecture.md) - System design and components
- [GitOps Workflow](explanation/gitops-workflow.md) - How GitOps integration works
- [Security Model](explanation/security-model.md) - Security architecture
- [Provider Comparison](explanation/provider-comparison.md) - Choosing the right provider

## By User Role

### Cluster Operators
You deploy and maintain Kubernetes clusters.

1. [Getting Started](tutorials/getting-started.md)
2. [CLI Commands Reference](reference/cli-commands.md)
3. [Troubleshooting Guide](how-to/troubleshooting.md)
4. [Backup and Recovery](how-to/backup-recovery.md)

### Platform Engineers
You design and standardize cluster configurations.

1. [Architecture Overview](explanation/architecture.md)
2. [Configuration Schema](reference/configuration.md)
3. [Adding Services](how-to/adding-services.md)
4. [Multi-Cluster Management](tutorials/multi-cluster.md)

### Security Engineers
You manage secrets and enforce security policies.

1. [Security Model](explanation/security-model.md)
2. [Managing Secrets](how-to/secrets-management.md)
3. [Audit and Compliance](how-to/audit-compliance.md)
4. [Secrets Reference](reference/secrets.md)

### Developers
You extend openCenter or integrate it into workflows.

1. [Developer Guide](dev/README.md)
2. [API Reference](reference/api.md)
3. [Plugin Development](how-to/plugin-development.md)
4. [Contributing Guide](../contributing.md)

## By Task

### Initial Setup
- [Install openCenter](tutorials/getting-started.md#installation)
- [Initialize first cluster](tutorials/getting-started.md#initialize-cluster)
- [Configure cloud provider](how-to/provider-setup.md)
- [Generate SOPS keys](how-to/secrets-management.md#generating-keys)

### Daily Operations
- [List clusters](reference/cli-commands.md#cluster-list)
- [Validate configuration](reference/cli-commands.md#cluster-validate)
- [Deploy changes](how-to/deploying-changes.md)
- [Monitor cluster status](how-to/monitoring.md)

### Troubleshooting
- [Common errors](how-to/troubleshooting.md#common-errors)
- [Debug mode](how-to/troubleshooting.md#debug-mode)
- [Log analysis](how-to/troubleshooting.md#log-analysis)
- [Getting help](how-to/troubleshooting.md#getting-help)

### Advanced Topics
- [Custom templates](how-to/custom-templates.md)
- [Plugin development](how-to/plugin-development.md)
- [CI/CD integration](how-to/cicd-integration.md)
- [Disaster recovery](operations/disaster-recovery.md)

## Provider-Specific Documentation

### [OpenStack](providers/openstack/README.md)
- [Setup Guide](providers/openstack/setup.md)
- [Network Configuration](providers/openstack/networking.md)
- [Troubleshooting](providers/openstack/troubleshooting.md)

### [AWS](providers/aws/README.md)
- [Setup Guide](providers/aws/setup.md)
- [IAM Configuration](providers/aws/iam.md)
- [VPC Design](providers/aws/vpc.md)

### [Kind](providers/kind/README.md)
- [Local Development](providers/kind/local-dev.md)
- [Testing Workflows](providers/kind/testing.md)

### [Talos Linux](providers/talos/README.md)
- [Architecture](providers/talos/architecture.md)
- [Implementation](providers/talos/implementation.md)

## Operations Documentation

### [Disaster Recovery](operations/disaster-recovery.md)
Backup strategies, recovery procedures, and business continuity planning.

### [Monitoring and Observability](operations/monitoring.md)
Metrics, logging, alerting, and performance tuning.

### [Security Operations](operations/security.md)
Security hardening, vulnerability management, and incident response.

### [Capacity Planning](operations/capacity-planning.md)
Resource sizing, scaling strategies, and cost optimization.

## Development Documentation

Internal documentation for contributors and maintainers:

- [Developer Guide](dev/README.md) - Architecture and development workflow
- [Testing Guide](dev/testing/README.md) - Test infrastructure and practices
- [Internal Packages](dev/internal/README.md) - Package documentation
- [Release Process](dev/release-process.md) - How releases are created

## Additional Resources

### External Documentation
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [FluxCD Documentation](https://fluxcd.io/docs/)
- [SOPS Documentation](https://github.com/mozilla/sops)
- [Age Encryption](https://age-encryption.org/)

### Community
- [GitHub Repository](https://github.com/rackerlabs/openCenter-cli)
- [Issue Tracker](https://github.com/rackerlabs/openCenter-cli/issues)
- [Discussions](https://github.com/rackerlabs/openCenter-cli/discussions)

### Support
- [Troubleshooting Guide](how-to/troubleshooting.md)
- [FAQ](explanation/faq.md)
- [Known Issues](explanation/known-issues.md)

## Contributing to Documentation

We welcome documentation improvements. See [Contributing Guide](../contributing.md) for:
- Documentation standards
- Diátaxis framework guidelines
- Review process
- Style guide

## Version Information

- **Documentation Version:** 1.0.0
- **openCenter Version:** 1.0.0
- **Last Updated:** January 19, 2026

## License

Documentation licensed under Apache 2.0. See [LICENSE](../LICENSE).
