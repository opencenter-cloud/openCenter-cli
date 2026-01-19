# openCenter Documentation Index

Welcome to the openCenter documentation. This index will help you find the information you need.

## Quick Start

- **[Getting Started](getting-started.md)** - Your first cluster in 10 minutes
- **[Overview](explanation/overview.md)** - What is openCenter and what can it do?
- **[Current Status](explanation/current-status.md)** - Implementation status and roadmap
- **[Architecture](explanation/architecture.md)** - Technical architecture and design

## Documentation Structure

openCenter documentation follows the [Diátaxis](https://diataxis.fr/) framework, organizing content into four categories:

### 📚 Tutorials

**Learning-oriented:** Step-by-step guides to help you learn openCenter.

- [Getting Started](getting-started.md) - Your first cluster in 10 minutes
- [OpenStack Quickstart](tutorials/quickstart-openstack.md) - Deploy on OpenStack
- [AWS Quickstart](tutorials/quickstart-aws.md) - Deploy on AWS
- [Kind Quickstart](tutorials/quickstart-kind.md) - Local development with Kind

**When to use:** You're new to openCenter and want to learn by doing.

### 🔧 How-To Guides

**Task-oriented:** Practical guides for accomplishing specific goals.

- [Troubleshooting](how-to/troubleshooting.md) - Common issues and solutions
- [Adding Services](how-to/adding-services.md) - Add services to your cluster
- [Managing Secrets](how-to/secrets.md) - SOPS and secrets management
- [IDE Integration](how-to/ide-integration.md) - Setup your development environment

**When to use:** You know what you want to do and need instructions.

### 📖 Reference

**Information-oriented:** Technical specifications and detailed information.

- [CLI Commands](reference/cli-commands.md) - Complete CLI reference
- [Configuration](reference/configuration.md) - Configuration file reference
- [Cluster Commands](reference/cluster/readme.md) - Cluster lifecycle commands
- [Shell Integration](reference/shell-integration.md) - Shell completion and integration

**When to use:** You need to look up specific details or specifications.

### 💡 Explanation

**Understanding-oriented:** Conceptual explanations and background information.

- [Overview](explanation/overview.md) - High-level overview of openCenter
- [Architecture](explanation/architecture.md) - Technical architecture and design
- [Current Status](explanation/current-status.md) - Implementation status and roadmap

**When to use:** You want to understand concepts and design decisions.

## By Topic

### Getting Started
1. [Getting Started](getting-started.md) - Your first cluster in 10 minutes
2. [Overview](explanation/overview.md) - Understand what openCenter is
3. [CLI Commands](reference/cli-commands.md) - Learn available commands
4. [Configuration](reference/configuration.md) - Understand configuration structure

### Configuration Management
- [Configuration Reference](reference/configuration.md) - Complete configuration guide
- [Cluster Configuration](cluster-config.md) - Cluster config details

### Secrets Management
- [Managing Secrets](how-to/secrets.md) - SOPS integration guide
- [CLI Commands - SOPS](reference/cli-commands.md#sops-commands) - SOPS command reference

### Provider Support
- [OpenStack Quickstart](tutorials/quickstart-openstack.md) - Deploy on OpenStack
- [AWS Quickstart](tutorials/quickstart-aws.md) - Deploy on AWS
- [Kind Quickstart](tutorials/quickstart-kind.md) - Local development
- [Configuration - Providers](reference/configuration.md#opencenterinfrastructure) - Provider configuration

### Development
- [Architecture](explanation/architecture.md) - Technical architecture
- [Current Status](explanation/current-status.md) - Development status
- [Developer Guide](dev/readme.md) - CLI architecture and implementation
- [Contributing](contributing.md) - Contribution guidelines

### Internal Documentation
- [Internal Packages](dev/internal/README.md) - Implementation details for internal packages
- [Completed Tasks](dev/completed-tasks/README.md) - Historical task completion records
- [Testing Documentation](dev/testing/README.md) - Testing infrastructure and practices

## By Role

### For Cluster Operators
1. [Getting Started](getting-started.md)
2. [Troubleshooting](how-to/troubleshooting.md)
3. [Managing Secrets](how-to/secrets.md)
4. [CLI Commands](reference/cli-commands.md)

### For Platform Engineers
1. [Overview](explanation/overview.md)
2. [Architecture](explanation/architecture.md)
3. [Configuration Reference](reference/configuration.md)
4. [Adding Services](how-to/adding-services.md)

### For Developers
1. [Architecture](explanation/architecture.md)
2. [Current Status](explanation/current-status.md)
3. [Developer Guide](dev/readme.md)
4. [Internal Packages](dev/internal/README.md)
5. [Testing Documentation](dev/testing/README.md)
6. [Contributing](contributing.md)

### For Decision Makers
1. [Overview](explanation/overview.md)
2. [Current Status](explanation/current-status.md)
3. [Architecture](explanation/architecture.md)

## Common Tasks

### Initial Setup
- [Install openCenter](tutorials/quickstart.md#prerequisites)
- [Initialize a Cluster](reference/cli-commands.md#cluster-init)
- [Validate Configuration](reference/cli-commands.md#cluster-validate)
- [Setup GitOps](reference/cli-commands.md#cluster-setup)

### Daily Operations
- [List Clusters](reference/cli-commands.md#cluster-list)
- [Select Active Cluster](reference/cli-commands.md#cluster-select)
- [Update Configuration](reference/cli-commands.md#cluster-update)
- [Validate Changes](reference/cli-commands.md#cluster-validate)

### Secrets Management
- [Generate SOPS Keys](reference/cli-commands.md#sops-generate-key)
- [Encrypt Secrets](reference/cli-commands.md#sops-secrets-encrypt)
- [Rotate Keys](reference/cli-commands.md#sops-rotate-key)
- [Backup Keys](reference/cli-commands.md#sops-backup-key)

### Cluster Lifecycle
- [Initialize Cluster](reference/cli-commands.md#cluster-init)
- [Setup Infrastructure](reference/cli-commands.md#cluster-setup)
- [Bootstrap Cluster](reference/cli-commands.md#cluster-bootstrap)
- [Destroy Cluster](reference/cli-commands.md#cluster-destroy)

## Troubleshooting

### Common Issues
- [Troubleshooting Guide](how-to/troubleshooting.md) - Complete troubleshooting reference
- Check [Current Status](explanation/current-status.md#known-issues) for known issues
- Review [CLI Commands](reference/cli-commands.md) for correct usage
- Enable verbose logging with `--verbose` flag
- Generate debug config with `--generate-debug-config`

## Additional Resources

### External Documentation
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [FluxCD Documentation](https://fluxcd.io/docs/)
- [SOPS Documentation](https://github.com/mozilla/sops)
- [Age Encryption](https://age-encryption.org/)
- [OpenTofu Documentation](https://opentofu.org/docs/)

### Community
- GitHub Repository: https://github.com/rackerlabs/openCenter-cli
- Issue Tracker: https://github.com/rackerlabs/openCenter-cli/issues
- Discussions: https://github.com/rackerlabs/openCenter-cli/discussions (Coming Soon)

## Contributing to Documentation

We welcome documentation contributions! See our [Contributing Guide](contributing.md) for details.

### Documentation Standards
- Follow the Diátaxis framework
- Use clear, concise language
- Include code examples
- Test all commands and examples
- Keep documentation up-to-date with code changes

### Documentation Structure
```
docs/
├── readme.md             # This file
├── getting-started.md    # Main entry point for new users
├── tutorials/            # Learning-oriented guides
├── how-to/              # Task-oriented guides
├── reference/           # Information-oriented docs
├── explanation/         # Understanding-oriented docs
├── dev/                 # Developer documentation
├── operations/          # Operational guides
└── providers/           # Provider-specific docs
```

## Version Information

- **Documentation Version:** 1.0.0
- **openCenter Version:** 0.0.1
- **Last Updated:** November 7, 2025

## License

This documentation is licensed under the Apache 2.0 License. See [LICENSE](../LICENSE) for details.
