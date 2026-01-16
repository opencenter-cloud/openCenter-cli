# openCenter Documentation Index

Welcome to the openCenter documentation. This index will help you find the information you need.

## Quick Links

- **[Overview](overview.md)** - What is openCenter and what can it do?
- **[Current Status](current-status.md)** - Implementation status and roadmap
- **[Architecture](architecture.md)** - Technical architecture and design
- **[Getting Started](tutorials/quickstart.md)** - Quick start tutorial

## Documentation Structure

openCenter documentation follows the [Diátaxis](https://diataxis.fr/) framework, organizing content into four categories:

### 📚 Tutorials

**Learning-oriented:** Step-by-step guides to help you learn openCenter.

- [Quickstart Tutorial](tutorials/quickstart.md) - Get started with your first cluster
- Multi-Cluster Setup (Coming Soon)
- Production Deployment (Coming Soon)
- Disaster Recovery (Coming Soon)

**When to use:** You're new to openCenter and want to learn by doing.

### 🔧 How-To Guides

**Task-oriented:** Practical guides for accomplishing specific goals.

- [Configure a Cluster](how-to/configure-cluster.md) - Cluster configuration guide
- [Manage Secrets with SOPS](how-to/manage-secrets.md) - Secrets management
- [Setup GitOps Repository](how-to/setup-gitops.md) - GitOps integration
- [Use Plugins](how-to/plugins.md) - Plugin system usage
- Advanced Configurations (Coming Soon)
- Troubleshooting (Coming Soon)
- Migration Guides (Coming Soon)

**When to use:** You know what you want to do and need instructions.

### 📖 Reference

**Information-oriented:** Technical specifications and detailed information.

- [CLI Commands](reference/cli-commands.md) - Complete CLI reference
- [Configuration](reference/configuration.md) - Configuration file reference
- [Schema](reference/schema.md) - JSON schema reference (Coming Soon)
- [Environment Variables](reference/environment.md) - Environment configuration
- [Plugins](reference/plugins.md) - Plugin development reference
- API Documentation (Coming Soon)

**When to use:** You need to look up specific details or specifications.

### 💡 Explanation

**Understanding-oriented:** Conceptual explanations and background information.

- [Overview](overview.md) - High-level overview of openCenter
- [Architecture](architecture.md) - Technical architecture and design
- [Current Status](current-status.md) - Implementation status and roadmap
- Design Decisions (Coming Soon)
- Best Practices (Coming Soon)

**When to use:** You want to understand concepts and design decisions.

## By Topic

### Getting Started
1. [Overview](overview.md) - Understand what openCenter is
2. [Quickstart Tutorial](tutorials/quickstart.md) - Create your first cluster
3. [CLI Commands](reference/cli-commands.md) - Learn available commands
4. [Configuration](reference/configuration.md) - Understand configuration structure

### Configuration Management
- [Configuration Reference](reference/configuration.md) - Complete configuration guide
- [Configure a Cluster](how-to/configure-cluster.md) - Step-by-step configuration
- [Schema Reference](reference/schema.md) - JSON schema details (Coming Soon)

### Secrets Management
- [Manage Secrets with SOPS](how-to/manage-secrets.md) - SOPS integration guide
- [CLI Commands - SOPS](reference/cli-commands.md#sops-commands) - SOPS command reference

### GitOps Integration
- [Setup GitOps Repository](how-to/setup-gitops.md) - GitOps setup guide
- [Architecture - GitOps](architecture.md#3-gitops-scaffolding-internalgitops) - GitOps architecture

### Provider Support
- [Configuration - Providers](reference/configuration.md#opencenterinfrastructure) - Provider configuration
- [Architecture - Providers](architecture.md#5-provider-adapters-internalcloud-internalprovision) - Provider architecture
- OpenStack Guide (Coming Soon)
- AWS Guide (Coming Soon)
- Kind Guide (Coming Soon)

### Plugin Development
- [Use Plugins](how-to/plugins.md) - Using plugins
- [Plugin Reference](reference/plugins.md) - Plugin development
- [Architecture - Plugins](architecture.md#7-plugin-system-internalplugins) - Plugin architecture

### Development
- [Architecture](architecture.md) - Technical architecture
- [Current Status](current-status.md) - Development status
- [Developer Guide](dev/readme.md) - CLI architecture and implementation
- [Contributing](../CONTRIBUTING.md) - Contribution guidelines (Coming Soon)
- Development Setup (Coming Soon)

### Internal Documentation
- [Internal Packages](dev/internal/README.md) - Implementation details for internal packages
- [Completed Tasks](dev/completed-tasks/README.md) - Historical task completion records
- [Testing Documentation](dev/testing/README.md) - Testing infrastructure and practices

## By Role

### For Cluster Operators
1. [Quickstart Tutorial](tutorials/quickstart.md)
2. [Configure a Cluster](how-to/configure-cluster.md)
3. [Manage Secrets](how-to/manage-secrets.md)
4. [CLI Commands](reference/cli-commands.md)
5. Troubleshooting Guide (Coming Soon)

### For Platform Engineers
1. [Overview](overview.md)
2. [Architecture](architecture.md)
3. [Configuration Reference](reference/configuration.md)
4. [Setup GitOps](how-to/setup-gitops.md)
5. [Plugin Development](reference/plugins.md)

### For Developers
1. [Architecture](architecture.md)
2. [Current Status](current-status.md)
3. [Developer Guide](dev/readme.md)
4. [Internal Packages](dev/internal/README.md)
5. [Testing Documentation](dev/testing/README.md)
6. Contributing Guide (Coming Soon)
7. Development Setup (Coming Soon)

### For Decision Makers
1. [Overview](overview.md)
2. [Current Status](current-status.md)
3. [Architecture](architecture.md)
4. Best Practices (Coming Soon)

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
- Configuration Validation Errors (Coming Soon)
- SOPS Key Issues (Coming Soon)
- Provider Connection Problems (Coming Soon)
- GitOps Setup Issues (Coming Soon)

### Getting Help
- Check [Current Status](current-status.md#known-issues) for known issues
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

We welcome documentation contributions! See our [Contributing Guide](../CONTRIBUTING.md) for details.

### Documentation Standards
- Follow the Diátaxis framework
- Use clear, concise language
- Include code examples
- Test all commands and examples
- Keep documentation up-to-date with code changes

### Documentation Structure
```
docs/
├── INDEX.md              # This file
├── overview.md           # High-level overview
├── architecture.md       # Technical architecture
├── current-status.md     # Implementation status
├── tutorials/            # Learning-oriented guides
├── how-to/              # Task-oriented guides
├── reference/           # Information-oriented docs
└── explanation/         # Understanding-oriented docs
```

## Version Information

- **Documentation Version:** 1.0.0
- **openCenter Version:** 0.0.1
- **Last Updated:** November 7, 2025

## License

This documentation is licensed under the Apache 2.0 License. See [LICENSE](../LICENSE) for details.
