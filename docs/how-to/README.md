# How-To Guides


## Table of Contents

- [Who These Are For](#who-these-are-for)
- [Available Guides](#available-guides)
- [How-To Guide Structure](#how-to-guide-structure)
- [Quick Reference](#quick-reference)
- [Getting Help](#getting-help)
- [Related Documentation](#related-documentation)
- [Contributing](#contributing)
**doc_type: how-to**

Practical guides for accomplishing specific tasks with openCenter. Each guide focuses on solving one problem with clear, tested steps.

## Who These Are For

How-to guides are for users who know what they want to do and need instructions. If you're troubleshooting an issue, configuring a feature, or performing a specific task, start here.

## Available Guides

### Essential Operations

**[Troubleshooting](troubleshooting.md)** - Diagnose and fix common issues
Debug problems, interpret error messages, and resolve configuration issues.

**[Secrets Management](secrets-management.md)** - Work with SOPS encryption
Generate keys, encrypt secrets, rotate keys, and manage encrypted files.

**[Upgrading Clusters](upgrading-clusters.md)** - Safely upgrade Kubernetes versions
Plan upgrades, test changes, and roll out new versions without downtime.

**[Backup and Recovery](backup-recovery.md)** - Protect your clusters
Back up configurations, secrets, and state. Restore from backups when needed.

**[Provider Setup](provider-setup.md)** - Configure cloud providers
Set up credentials, networking, and provider-specific requirements.

### Day-to-Day Tasks

**[Deploying Changes](deploying-changes.md)** - Push configuration updates
Validate changes, update GitOps repositories, and deploy to clusters.

**[Monitoring](monitoring.md)** - Set up observability
Configure metrics, logging, and alerting for your clusters.

**[Adding Services](adding-services.md)** - Extend cluster capabilities
Add new services to your cluster configuration and deploy them.

### Advanced Configuration

**[Custom Templates](custom-templates.md)** - Customize generated manifests
Override default templates and create custom resource definitions.

**[Plugin Development](plugin-development.md)** - Extend openCenter
Create custom commands and integrate with external tools.

**[CI/CD Integration](cicd-integration.md)** - Automate workflows
Integrate openCenter into CI/CD pipelines for automated deployments.

### Security and Compliance

**[Audit and Compliance](audit-compliance.md)** - Meet regulatory requirements
Configure audit logging, implement compliance controls, and generate reports.

**[IDE Integration](ide-integration.md)** - Set up development environment
Configure VS Code, JetBrains IDEs, and other editors for YAML validation.

### Migration and Maintenance

**[Migration](migration.md)** - Move clusters between providers
Migrate workloads, configurations, and data between cloud providers.

## How-To Guide Structure

Each guide follows this format:

1. **Task Summary** - What problem this solves
2. **Prerequisites** - What you need
3. **Steps** - Numbered instructions
4. **Expected Outcome** - What success looks like
5. **Troubleshooting** - Common issues
6. **Related Tasks** - What to do next

## Quick Reference

### By Task Type

**Setup and Configuration**
- [Provider Setup](provider-setup.md)
- [IDE Integration](ide-integration.md)
- [Monitoring](monitoring.md)

**Operations**
- [Deploying Changes](deploying-changes.md)
- [Upgrading Clusters](upgrading-clusters.md)
- [Backup and Recovery](backup-recovery.md)

**Security**
- [Secrets Management](secrets-management.md)
- [Audit and Compliance](audit-compliance.md)

**Troubleshooting**
- [Troubleshooting](troubleshooting.md)

**Advanced**
- [Custom Templates](custom-templates.md)
- [Plugin Development](plugin-development.md)
- [CI/CD Integration](cicd-integration.md)
- [Migration](migration.md)

### By User Role

**Cluster Operators**
- [Deploying Changes](deploying-changes.md)
- [Troubleshooting](troubleshooting.md)
- [Backup and Recovery](backup-recovery.md)
- [Upgrading Clusters](upgrading-clusters.md)

**Platform Engineers**
- [Adding Services](adding-services.md)
- [Custom Templates](custom-templates.md)
- [Provider Setup](provider-setup.md)
- [Monitoring](monitoring.md)

**Security Engineers**
- [Secrets Management](secrets-management.md)
- [Audit and Compliance](audit-compliance.md)

**Developers**
- [Plugin Development](plugin-development.md)
- [CI/CD Integration](cicd-integration.md)
- [IDE Integration](ide-integration.md)

## Getting Help

If a guide doesn't solve your problem:
1. Check [Troubleshooting](troubleshooting.md) for diagnostic steps
2. Review [FAQ](../explanation/faq.md) for common questions
3. Search [Reference](../reference/README.md) for technical details
4. Ask in [GitHub Discussions](https://github.com/rackerlabs/openCenter-cli/discussions)

## Related Documentation

- **[Tutorials](../tutorials/README.md)** - Learn through hands-on practice
- **[Reference](../reference/README.md)** - Look up technical specifications
- **[Explanation](../explanation/README.md)** - Understand concepts and architecture

## Contributing

Found an error? Have a better way to solve a problem? See our [Contributing Guide](../../contributing.md) to improve these guides.
