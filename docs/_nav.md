---
id: nav
title: "Documentation Navigation"
sidebar_label: Navigation
description: Navigation guide to find the right openCenter documentation for your needs.
doc_type: reference
audience: "all users"
tags: [navigation, guide, documentation]
---

# openCenter CLI Documentation Navigation

This navigation guide helps you find the right documentation for your needs.

## Quick Links

- [Documentation Home](index.md)
- [Getting Started Tutorial](tutorials/getting-started.md)
- [CLI Commands Reference](reference/cli-commands.md)
- [Contributing Guide](dev/contributing.md)

## By Documentation Type

### Tutorials (Learning-Oriented)

For learning openCenter through hands-on practice:

1. [Getting Started](tutorials/getting-started.md) - Your first cluster in 10 minutes
2. [OpenStack First Cluster](tutorials/openstack-first-cluster.md) - Deploy on OpenStack
3. [Kind Local Development](tutorials/kind-local-development.md) - Local development setup
4. [VMware Deployment](tutorials/vmware-deployment.md) - Deploy on pre-provisioned VMs
5. [Multi-Cluster Setup](tutorials/multi-cluster-setup.md) - Manage multiple clusters

### How-To Guides (Task-Oriented)

For accomplishing specific tasks:

**Configuration & Validation**
- [Validate Configuration](how-to/validate-configuration.md)
- [Customize Services](how-to/customize-services.md)
- [Configure Networking](how-to/configure-networking.md)

**Secrets & Security**
- [Manage Secrets](how-to/manage-secrets.md)

**Operations**
- [Add Worker Pools](how-to/add-worker-pools.md)
- [Backup and Restore](how-to/backup-and-restore.md)
- [Upgrade Kubernetes](how-to/upgrade-kubernetes.md)
- [Migrate Clusters](how-to/migrate-clusters.md)

**Troubleshooting & Integration**
- [Troubleshoot Deployment](how-to/troubleshoot-deployment.md)
- [Integrate CI/CD](how-to/integrate-ci-cd.md)

### Reference (Information-Oriented)

For looking up technical details:

**CLI & Configuration**
- [CLI Commands](reference/cli-commands.md)
- [Configuration Schema](reference/configuration-schema.md)
- [Default Values](reference/default-values.md)
- [Environment Variables](reference/environment-variables.md)
- [Exit Codes](reference/exit-codes.md)
- [File Locations](reference/file-locations.md)

**Platform & Providers**
- [Platform Services](reference/platform-services.md)
- [Providers](reference/providers.md)
- [Validation Rules](reference/validation-rules.md)

**Development**
- [Mise Tasks](reference/mise-tasks.md)

### Explanation (Understanding-Oriented)

For understanding concepts and design:

- [Architecture](explanation/architecture.md) - System design and decisions
- [GitOps Workflow](explanation/gitops-workflow.md) - Repository structure
- [Security Model](explanation/security-model.md) - Security architecture
- [Plugin Internal Services](explanation/plugin-internal-services.md) - Service plugins, cert-manager behavior, and how to add a new service
- [Plugin External CLI](explanation/plugin-external-cli.md) - Command plugin discovery and execution model
- [Configuration Lifecycle](explanation/configuration-lifecycle.md) - Config management
- [Provider Comparison](explanation/provider-comparison.md) - Choosing providers
- [Drift Detection](explanation/drift-detection.md) - Drift and reconciliation

### Developer Documentation

For contributors and developers:

**Getting Started**
- [Contributing Guide](dev/contributing.md)
- [Development Setup](dev/development-setup.md)
- [Code Structure](dev/code-structure.md)

**Testing**
- [Testing Guide](dev/testing-guide.md)

**Extending**
- [Adding Providers](dev/adding-providers.md)
- [Adding Services](dev/adding-services.md)

**Building & Releasing**
- [Build System](dev/build-system.md)
- [Release Process](dev/release-process.md)

## By User Journey

### First-Time User

1. [Getting Started Tutorial](tutorials/getting-started.md)
2. [CLI Commands Reference](reference/cli-commands.md)
3. [Configuration Schema](reference/configuration-schema.md)
4. [Troubleshoot Deployment](how-to/troubleshoot-deployment.md)

### Operator

1. [Validate Configuration](how-to/validate-configuration.md)
2. [Manage Secrets](how-to/manage-secrets.md)
3. [Backup and Restore](how-to/backup-and-restore.md)
4. [Upgrade Kubernetes](how-to/upgrade-kubernetes.md)

### Developer/Contributor

1. [Contributing Guide](dev/contributing.md)
2. [Development Setup](dev/development-setup.md)
3. [Testing Guide](dev/testing-guide.md)
4. [Code Structure](dev/code-structure.md)

### Architect/Decision Maker

1. [Architecture](explanation/architecture.md)
2. [Security Model](explanation/security-model.md)
3. [Provider Comparison](explanation/provider-comparison.md)
4. [GitOps Workflow](explanation/gitops-workflow.md)

## By Task

### Initial Setup
- [Getting Started Tutorial](tutorials/getting-started.md)
- [OpenStack First Cluster](tutorials/openstack-first-cluster.md)
- [Kind Local Development](tutorials/kind-local-development.md)

### Configuration
- [Configuration Schema](reference/configuration-schema.md)
- [Default Values](reference/default-values.md)
- [Validate Configuration](how-to/validate-configuration.md)

### Secrets Management
- [Manage Secrets](how-to/manage-secrets.md)
- [Security Model](explanation/security-model.md)

### Deployment
- [CLI Commands](reference/cli-commands.md)
- [Platform Services](reference/platform-services.md)
- [Troubleshoot Deployment](how-to/troubleshoot-deployment.md)

### Day-2 Operations
- [Add Worker Pools](how-to/add-worker-pools.md)
- [Upgrade Kubernetes](how-to/upgrade-kubernetes.md)
- [Backup and Restore](how-to/backup-and-restore.md)
- [Migrate Clusters](how-to/migrate-clusters.md)

### Troubleshooting
- [Troubleshoot Deployment](how-to/troubleshoot-deployment.md)
- [Validation Rules](reference/validation-rules.md)
- [Exit Codes](reference/exit-codes.md)

### Development
- [Development Setup](dev/development-setup.md)
- [Testing Guide](dev/testing-guide.md)
- [Adding Providers](dev/adding-providers.md)
- [Adding Services](dev/adding-services.md)

## External Resources

- [Diátaxis Framework](https://diataxis.fr/) - Documentation framework we follow
- [GitHub Repository](https://github.com/opencenter-cloud/openCenter-cli)
- [Issue Tracker](https://github.com/opencenter-cloud/openCenter-cli/issues)
- [Discussions](https://github.com/opencenter-cloud/openCenter-cli/discussions)

---

**Using This Navigation:**

- **If you're learning:** Start with Tutorials
- **If you have a specific task:** Check How-To Guides
- **If you need technical details:** Look in Reference
- **If you want to understand why:** Read Explanation
- **If you're contributing:** See Developer Documentation
