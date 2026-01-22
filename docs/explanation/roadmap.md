# Roadmap and Future Plans


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Current status](#current-status)
- [Development philosophy](#development-philosophy)
- [Near-term priorities (next 3-6 months)](#near-term-priorities-next-3-6-months)
- [Medium-term goals (6-12 months)](#medium-term-goals-6-12-months)
- [Long-term vision (12+ months)](#long-term-vision-12-months)
- [Features we're not planning](#features-were-not-planning)
- [How features are prioritized](#how-features-are-prioritized)
- [Contributing to the roadmap](#contributing-to-the-roadmap)
- [Migration and deprecation policy](#migration-and-deprecation-policy)
- [Version numbering](#version-numbering)
- [Release cadence](#release-cadence)
- [Technology evolution](#technology-evolution)
- [Community and governance](#community-and-governance)
- [How to stay informed](#how-to-stay-informed)
- [Conclusion](#conclusion)
- [See also](#see-also)
**doc_type**: explanation

## Who this is for

Users and contributors who want to understand where openCenter is headed, what features are planned, and how the project is evolving.

## Current status

openCenter is production-ready for OpenStack-based Kubernetes clusters. The core functionality is stable:

- ✅ Configuration management with validation
- ✅ GitOps repository generation
- ✅ SOPS-based secrets management
- ✅ OpenStack provider support
- ✅ Kubespray-based cluster provisioning
- ✅ Service plugin system
- ✅ Comprehensive testing (unit, property-based, BDD)

The tool is actively used in production environments and receives regular updates for bug fixes, security patches, and incremental improvements.

## Development philosophy

openCenter follows a **stability-first** approach:

**Production reliability over feature velocity**: New features are added carefully, with comprehensive testing and documentation. Breaking changes are avoided when possible.

**Incremental improvement over big rewrites**: The codebase evolves through small, well-tested changes rather than large refactorings.

**User feedback drives priorities**: Features are prioritized based on real user needs, not theoretical requirements.

**Backward compatibility matters**: Existing configurations and workflows continue to work across versions.

This philosophy means openCenter evolves deliberately. Features are added when they're needed and can be implemented reliably, not because they're trendy or interesting.

## Near-term priorities (next 3-6 months)

### Pipeline-based GitOps generation

**Status**: In development

**What**: Replace the current generation system with a staged pipeline that supports:
- Atomic operations with automatic rollback
- Progress reporting with detailed status
- Dry-run mode showing planned changes
- Checkpoint-based recovery

**Why**: The current generation system works but lacks visibility and recovery options. The pipeline system provides better error handling and user feedback.

**Impact**: Transparent to users—existing commands work the same. Internally, generation becomes more robust and maintainable.

**Timeline**: Q2 2025

### Enhanced TUI for cluster initialization

**Status**: Planned

**What**: Interactive terminal UI for cluster initialization with:
- Step-by-step prompts for configuration
- Input validation at each step
- Preview of generated configuration
- Saved preferences for future sessions

**Why**: The current `cluster init` generates a default configuration that users must edit manually. An interactive mode guides users through the process, reducing errors and improving the first-time experience.

**Impact**: Optional feature—existing non-interactive mode remains available. Particularly helpful for new users and CI/CD environments.

**Timeline**: Q2 2025

### AWS provider support

**Status**: Partial implementation

**What**: Full support for AWS-based clusters:
- AWS-specific validation (VPC, subnets, IAM)
- Terraform templates for AWS resources
- EKS integration (optional)
- AWS-specific services (ALB ingress, EBS CSI)

**Why**: Many users deploy on AWS. Currently, openCenter focuses on OpenStack. AWS support expands the tool's applicability.

**Impact**: New provider option. Existing OpenStack users unaffected.

**Timeline**: Q3 2025

### Observability improvements

**Status**: In development

**What**: Built-in observability for openCenter operations:
- Structured logging with levels
- Metrics collection (Prometheus format)
- Distributed tracing (OpenTelemetry)
- Performance profiling

**Why**: Understanding what openCenter is doing (especially during long operations like bootstrap) helps with debugging and optimization.

**Impact**: Optional feature—can be enabled for troubleshooting. No impact on normal operation.

**Timeline**: Q2 2025

## Medium-term goals (6-12 months)

### Drift detection and reconciliation

**Status**: Designed, not implemented

**What**: Detect when cluster state diverges from configuration:
- Compare running cluster with GitOps repository
- Identify configuration drift
- Classify drift severity (critical, warning, info)
- Provide reconciliation commands to fix drift

**Why**: Clusters drift over time due to manual changes, failed deployments, or external factors. Drift detection helps maintain consistency.

**Impact**: New command (`cluster drift detect`). Existing workflows unaffected.

**Timeline**: Q3 2025

### Backup and disaster recovery

**Status**: Designed, not implemented

**What**: Automated backup and recovery for clusters:
- Backup cluster configuration and state
- Backup etcd data
- Backup persistent volumes
- Restore from backup with validation

**Why**: Disaster recovery is critical for production clusters. Automated backup reduces risk and recovery time.

**Impact**: New commands (`cluster backup`, `cluster restore`). Integrates with existing backup tools (Velero).

**Timeline**: Q4 2025

### Multi-cluster management

**Status**: Conceptual

**What**: Manage multiple clusters as a fleet:
- List and filter clusters
- Bulk operations (update, validate)
- Cross-cluster configuration sharing
- Cluster groups and tags

**Why**: Organizations run multiple clusters (dev, staging, prod, per-region). Managing them individually is tedious.

**Impact**: New commands for fleet management. Single-cluster workflows unchanged.

**Timeline**: Q4 2025

### Template marketplace

**Status**: Conceptual

**What**: Community-contributed templates and plugins:
- Registry of templates and plugins
- Installation from registry
- Version management
- Signature verification

**Why**: Community contributions extend openCenter's capabilities. A marketplace makes discovery and installation easy.

**Impact**: Optional feature. Embedded templates remain the default.

**Timeline**: 2026

## Long-term vision (12+ months)

### Declarative cluster lifecycle

**Status**: Conceptual

**What**: Manage entire cluster lifecycle declaratively:
- Define desired state in configuration
- openCenter reconciles actual state to desired state
- Automatic updates, scaling, and healing
- GitOps-driven cluster management

**Why**: Current workflow is imperative (run commands to change state). Declarative management is more reliable and automatable.

**Impact**: Major workflow change. Existing imperative commands remain for backward compatibility.

**Timeline**: 2026

### Multi-cloud abstraction

**Status**: Conceptual

**What**: Abstract cloud provider differences:
- Common configuration for all providers
- Provider-specific overrides when needed
- Automatic provider selection based on configuration
- Seamless migration between providers

**Why**: Organizations use multiple cloud providers. Managing provider-specific configuration is complex.

**Impact**: Simplifies multi-cloud deployments. Single-cloud users unaffected.

**Timeline**: 2026+

### Policy-driven configuration

**Status**: Conceptual

**What**: Define organizational policies that constrain configuration:
- Security policies (encryption required, hardening enabled)
- Compliance policies (audit logging, data residency)
- Operational policies (minimum HA, backup frequency)
- Automatic policy enforcement during validation

**Why**: Organizations have standards that all clusters must meet. Policy enforcement prevents non-compliant configurations.

**Impact**: Optional feature. Useful for enterprises with compliance requirements.

**Timeline**: 2026+

### AI-assisted configuration

**Status**: Conceptual

**What**: Use AI to help with configuration:
- Suggest configuration based on requirements
- Detect configuration anti-patterns
- Recommend optimizations
- Explain configuration options

**Why**: Configuration is complex. AI assistance can reduce errors and improve best practices adoption.

**Impact**: Optional feature. Traditional configuration workflow remains.

**Timeline**: 2026+

## Features we're not planning

Some features are explicitly not planned because they conflict with openCenter's philosophy:

### Graphical user interface

**Why not**: openCenter is a CLI tool designed for automation and GitOps workflows. A GUI would require maintaining two interfaces and would be less automatable.

**Alternative**: The planned TUI (terminal UI) provides interactivity while remaining scriptable.

### Built-in cluster runtime management

**Why not**: openCenter generates GitOps repositories. Flux/ArgoCD handle runtime management. Duplicating their functionality would be redundant.

**Alternative**: Use Flux/ArgoCD for runtime management. openCenter focuses on cluster bootstrapping and configuration.

### Application deployment

**Why not**: openCenter deploys infrastructure and platform services. Application deployment is handled by GitOps tools (Flux/ArgoCD) or CI/CD pipelines.

**Alternative**: Use Flux/ArgoCD for application deployment. openCenter provides the platform.

### Custom Kubernetes distributions

**Why not**: openCenter uses standard Kubernetes (via Kubespray) or Talos. Supporting custom distributions (Rancher, OpenShift) would require significant provider-specific code.

**Alternative**: Use the distribution's native tools. openCenter focuses on standard Kubernetes.

## How features are prioritized

Feature prioritization considers:

1. **User demand**: How many users need this feature?
2. **Production impact**: Does this improve reliability or reduce operational risk?
3. **Maintenance burden**: How much ongoing maintenance will this require?
4. **Alignment with philosophy**: Does this fit openCenter's stability-first approach?
5. **Implementation complexity**: Can this be implemented reliably?

Features that improve production reliability for many users are prioritized. Features that add complexity without clear benefit are deferred or rejected.

## Contributing to the roadmap

The roadmap is shaped by user feedback. If you have feature requests:

1. **Open an issue**: Describe the feature and your use case
2. **Explain the problem**: What problem does this solve?
3. **Provide context**: How many users would benefit?
4. **Suggest implementation**: If you have ideas, share them

Features with clear use cases and community support are more likely to be prioritized.

## Migration and deprecation policy

When features change or are deprecated:

**Advance notice**: Deprecations are announced at least one major version in advance.

**Migration guides**: Detailed guides explain how to migrate to new approaches.

**Backward compatibility**: Deprecated features continue to work during the deprecation period.

**Feature flags**: New features can be tested with feature flags before becoming default.

This policy ensures users have time to adapt to changes without disruption.

## Version numbering

openCenter follows semantic versioning:

- **Major version** (1.x.x → 2.x.x): Breaking changes, major features
- **Minor version** (x.1.x → x.2.x): New features, backward compatible
- **Patch version** (x.x.1 → x.x.2): Bug fixes, security patches

Current version: 1.x.x (stable, production-ready)

Next major version (2.0.0) is not planned until there's a compelling reason for breaking changes.

## Release cadence

**Minor releases**: Every 2-3 months with new features and improvements

**Patch releases**: As needed for bug fixes and security issues

**Major releases**: When necessary for breaking changes (no fixed schedule)

This cadence balances feature delivery with stability. Users can upgrade minor versions confidently, knowing backward compatibility is maintained.

## Technology evolution

openCenter's technology stack evolves conservatively:

**Go version**: Stay current with Go releases (currently 1.25.2)

**Dependencies**: Update regularly for security, cautiously for features

**Kubernetes versions**: Support recent versions (currently 1.33.5)

**Cloud provider APIs**: Track provider changes, maintain compatibility

The goal is to stay current without chasing every new technology. Stability matters more than being on the bleeding edge.

## Community and governance

openCenter is developed by Rackspace Technology with community contributions welcome.

**Decision making**: Core team makes final decisions, informed by community feedback

**Contributions**: Pull requests welcome for bug fixes, features, and documentation

**Communication**: GitHub issues and discussions for feature requests and support

**Transparency**: Roadmap and decisions are public and documented

This model balances the need for direction with community input.

## How to stay informed

Track openCenter's progress:

- **GitHub releases**: Announcements of new versions
- **Changelog**: Detailed list of changes in each release
- **GitHub issues**: Feature requests and bug reports
- **Documentation**: Updated with each release

Subscribe to GitHub notifications to stay informed about releases and major changes.

## Conclusion

openCenter's roadmap reflects its core philosophy: **production reliability through opinionated simplicity**. Features are added when they improve reliability, reduce operational risk, or address clear user needs.

The roadmap is a living document. Priorities shift based on user feedback, production experience, and technology evolution. But the philosophy remains constant: build a tool that makes Kubernetes cluster deployment reliable and maintainable.

If you have feedback on the roadmap or feature requests, open an issue on GitHub. Your input helps shape openCenter's future.

## See also

- **[Architecture](./architecture.md)**: Technical architecture and design
- **[Design Decisions](./design-decisions.md)**: Why openCenter is built this way
- **[Contributing Guide](../contributing.md)**: How to contribute to openCenter
- **[Changelog](../../CHANGELOG.md)**: Detailed history of changes
