---
id: doc-map
title: "Documentation Map"
sidebar_label: Doc Map
description: Complete inventory of openCenter CLI documentation organized by the Diataxis framework.
doc_type: reference
audience: "writers, maintainers, all users"
tags: [documentation, diataxis, inventory, navigation]
---

# Documentation Map

This document provides a complete inventory of openCenter CLI documentation, organized by the Diátaxis framework.

## Purpose

This map helps:
- **Writers:** Identify gaps and plan new documentation
- **Maintainers:** Ensure documentation stays organized and complete
- **Users:** Understand what documentation exists and where to find it

## Audience Assumptions

### Beginner User
- Familiar with Kubernetes concepts (pods, services, deployments)
- Basic command-line experience
- May not know GitOps, FluxCD, or Kustomize
- Needs step-by-step guidance

### Operator
- Experienced with Kubernetes operations
- Understands infrastructure provisioning
- Needs task-specific guidance
- Values efficiency and precision

### Developer/Contributor
- Proficient in Go programming
- Understands software architecture
- Needs codebase navigation and extension points
- Values code examples and patterns

### Architect/Decision Maker
- Needs to understand system design
- Evaluates trade-offs and alternatives
- Requires conceptual understanding
- Values rationale and evidence

## Documentation Inventory

### Tutorials (Learning-Oriented)

| Document | Audience | Purpose | Prerequisites | Evidence |
|----------|----------|---------|---------------|----------|
| [Getting Started](tutorials/getting-started.md) | Beginner | Create first cluster end-to-end | Mise, Git, infrastructure access | `tests/features/workflow.feature`, `internal/config/defaults.go` |
| [OpenStack First Cluster](tutorials/openstack-first-cluster.md) | Beginner | Deploy on OpenStack specifically | OpenStack credentials | `internal/cloud/openstack/` |
| [Kind Local Development](tutorials/kind-local-development.md) | Beginner/Developer | Local development setup | Kind installed | `.mise.toml` kind tasks |
| [VMware Deployment](tutorials/vmware-deployment.md) | Beginner | Deploy on pre-provisioned VMs | VMware vSphere access | `internal/cloud/vmware/`, `docs/providers/vmware.md` |
| [Multi-Cluster Setup](tutorials/multi-cluster-setup.md) | Operator | Manage multiple clusters | Completed Getting Started | `cmd/cluster_list.go`, `cmd/cluster_select.go` |

### How-To Guides (Task-Oriented)

| Document | Audience | Purpose | Prerequisites | Evidence |
|----------|----------|---------|---------------|----------|
| [Validate Configuration](how-to/validate-configuration.md) | All | Pre-deployment validation | Configuration file | `cmd/cluster_validate.go`, `internal/config/validator.go` |
| [Manage Secrets](how-to/manage-secrets.md) | Operator | SOPS encryption and key rotation | SOPS installed | `internal/sops/`, `cmd/secrets*.go` |
| [Customize Services](how-to/customize-services.md) | Operator | Configure platform services | Deployed cluster | `internal/config/services/` |
| [Configure Networking](how-to/configure-networking.md) | Operator | CNI and network setup | Network knowledge | `internal/config/types_networking.go` |
| [Add Worker Pools](how-to/add-worker-pools.md) | Operator | Scale cluster | Running cluster | Configuration schema |
| [Backup and Restore](how-to/backup-and-restore.md) | Operator | Disaster recovery | Velero enabled | `cmd/cluster_backup.go` |
| [Upgrade Kubernetes](how-to/upgrade-kubernetes.md) | Operator | Safe version upgrades | Running cluster | Kubespray integration |
| [Troubleshoot Deployment](how-to/troubleshoot-deployment.md) | All | Common issues and solutions | Deployment attempt | Error patterns from tests |
| [Migrate Clusters](how-to/migrate-clusters.md) | Operator | Provider/region migration | Source cluster | Configuration migration |
| [Integrate CI/CD](how-to/integrate-ci-cd.md) | Developer | Pipeline integration | CI/CD system | CLI automation patterns |

### Reference (Information-Oriented)

| Document | Audience | Purpose | Scope | Evidence |
|----------|----------|---------|-------|----------|
| [CLI Commands](reference/cli-commands.md) | All | Complete command reference | All commands, flags, examples | `cmd/*.go` |
| [Configuration Schema](reference/configuration-schema.md) | All | Configuration file structure | All config fields | `internal/config/types*.go`, `schema/` |
| [Default Values](reference/default-values.md) | All | Default configuration values | All defaults | `internal/config/defaults.go` |
| [Platform Services](reference/platform-services.md) | Operator | Available services and versions | All services | `internal/config/services/` |
| [Providers](reference/providers.md) | Operator | Infrastructure provider details | OpenStack, VMware, Baremetal, Kind | `internal/cloud/` |
| [Validation Rules](reference/validation-rules.md) | All | Configuration constraints | All validation rules | `internal/core/validation/` |
| [Environment Variables](reference/environment-variables.md) | All | Environment configuration | All env vars | `cmd/root.go`, `internal/config/` |
| [Exit Codes](reference/exit-codes.md) | Developer | CLI exit codes and meanings | All exit codes | `cmd/*.go` error handling |
| [File Locations](reference/file-locations.md) | All | Configuration file paths | All file paths | `internal/config/paths.go` |
| [Mise Tasks](reference/mise-tasks.md) | Developer | Development and build tasks | All mise tasks | `.mise.toml` |

### Explanation (Understanding-Oriented)

| Document | Audience | Purpose | Scope | Evidence |
|----------|----------|---------|-------|----------|
| [Architecture](explanation/architecture.md) | Architect/Developer | System design and decisions | Core components, patterns, principles | `internal/` structure, design docs |
| [GitOps Workflow](explanation/gitops-workflow.md) | Operator/Architect | Repository structure and reconciliation | GitOps patterns, FluxCD integration | `internal/gitops/`, ecosystem.md |
| [Security Model](explanation/security-model.md) | Architect/Operator | Security architecture and controls | SOPS, encryption, validation | `internal/sops/`, `internal/security/` |
| [Plugin Internal Services](explanation/plugin-internal-services.md) | Developer/Architect | Explain internal service plugins used for platform services | Typed config, cert-manager behavior, validation, rendering, extension points | `internal/services/`, `internal/config/`, `internal/gitops/` |
| [Plugin External CLI](explanation/plugin-external-cli.md) | Developer/Architect | Explain executable command plugins exposed as top-level CLI commands | Discovery order, Cobra wiring, argument forwarding, extension constraints | `cmd/root.go`, `internal/plugins/loader.go` |
| [Configuration Lifecycle](explanation/configuration-lifecycle.md) | Architect/Operator | Configuration management | Init, validate, setup, bootstrap | `cmd/cluster*.go` |
| [Provider Comparison](explanation/provider-comparison.md) | Architect | Choosing infrastructure providers | Provider trade-offs | `internal/cloud/` implementations |
| [Drift Detection](explanation/drift-detection.md) | Architect/Operator | Configuration drift and reconciliation | Drift detection patterns | `internal/operations/drift_detector.go` |

### Developer Documentation

| Document | Audience | Purpose | Scope | Evidence |
|----------|----------|---------|-------|----------|
| [Contributing Guide](dev/contributing.md) | Developer | Contribution guidelines | Workflow, standards, review | `contributing.md` |
| [Development Setup](dev/development-setup.md) | Developer | Local development environment | Tools, build, test | `.mise.toml`, `go.mod` |
| [Testing Guide](dev/testing-guide.md) | Developer | Writing and running tests | Unit, BDD, property tests | `tests/`, `internal/testing/` |
| [Code Structure](dev/code-structure.md) | Developer | Codebase organization | Packages, modules, patterns | `internal/` structure |
| [Adding Providers](dev/adding-providers.md) | Developer | Implement new infrastructure providers | Provider interface, examples | `internal/cloud/factory.go` |
| [Adding Services](dev/adding-services.md) | Developer | Add new platform services | Service interface, examples | `internal/config/services/` |
| [Build System](dev/build-system.md) | Developer | Mise-based build and release | Build tasks, release process | `.mise.toml` |
| [Release Process](dev/release-process.md) | Maintainer | Creating releases | Versioning, changelog, artifacts | `.mise.toml` release tasks |

## Documentation Gaps

### Identified Gaps

1. **Tutorial:** AWS deployment tutorial (AWS provider is experimental)
2. **How-To:** Deploy custom applications (beyond platform services)
3. **How-To:** Configure observability stack (Prometheus, Grafana, Loki)
4. **How-To:** Set up multi-tenancy with Keycloak
5. **Reference:** JSON Schema IDE integration guide
6. **Reference:** Shell completion setup
7. **Explanation:** Template engine and customization
8. **Explanation:** Dependency injection container
9. **Dev:** Plugin development guide
10. **Dev:** Performance optimization guide

### Planned Documentation

Priority 1 (Next Release):
- JSON Schema IDE integration (Reference)
- Deploy custom applications (How-To)
- Shell completion setup (Reference)

Priority 2 (Future):
- AWS deployment tutorial (Tutorial)
- Configure observability stack (How-To)
- Template engine explanation (Explanation)

Priority 3 (As Needed):
- Multi-tenancy setup (How-To)
- Plugin development (Dev)
- Performance optimization (Dev)

## Cross-References

### Common User Paths

**Path 1: First-Time User**
1. Getting Started Tutorial → CLI Commands Reference → Troubleshoot Deployment

**Path 2: Production Deployment**
1. OpenStack First Cluster → Validate Configuration → Manage Secrets → Backup and Restore

**Path 3: Understanding System**
1. Architecture → GitOps Workflow → Security Model → Configuration Lifecycle

**Path 4: Contributing**
1. Contributing Guide → Development Setup → Testing Guide → Code Structure

### Topic Clusters

**Cluster 1: Configuration**
- Tutorial: Getting Started
- How-To: Validate Configuration, Customize Services
- Reference: Configuration Schema, Default Values, Validation Rules
- Explanation: Configuration Lifecycle

**Cluster 2: Secrets**
- How-To: Manage Secrets
- Reference: File Locations (Age keys)
- Explanation: Security Model

**Cluster 3: Providers**
- Tutorial: OpenStack First Cluster, VMware Deployment, Kind Local Development
- Reference: Providers
- Explanation: Provider Comparison
- Dev: Adding Providers

**Cluster 4: Operations**
- How-To: Add Worker Pools, Backup and Restore, Upgrade Kubernetes, Migrate Clusters
- Reference: CLI Commands, Platform Services
- Explanation: Drift Detection

## Maintenance Guidelines

### When to Update

- **Code changes:** Update Reference docs immediately
- **New features:** Add How-To guide and update Reference
- **Architecture changes:** Update Explanation docs
- **Bug fixes:** Update Troubleshooting guide if user-facing

### Review Schedule

- **Monthly:** Check for broken links, outdated examples
- **Quarterly:** Review gaps, plan new documentation
- **Per release:** Update version numbers, new features, deprecations

### Quality Checklist

For each document:
- [ ] Clear doc_type (tutorial/how-to/reference/explanation)
- [ ] Purpose statement at top
- [ ] Prerequisites listed (if applicable)
- [ ] Evidence citations (file paths, line numbers)
- [ ] Examples are copy-paste ready
- [ ] Cross-links to related docs
- [ ] No mixing of doc types

## Evidence Sources

This documentation is based on:

- **Codebase:** `openCenter-cli/` repository structure
- **Configuration:** `internal/config/` types and defaults
- **Commands:** `cmd/` Cobra command definitions
- **Tests:** `tests/features/` BDD scenarios
- **Build system:** `.mise.toml` task definitions
- **Existing docs:** `docs/` current documentation
- **Ecosystem:** `../docs/ecosystem.md` architecture
- **Steering:** `.kiro/steering/` product and technical guidance

---

**Last Updated:** 2026-02-20
**Maintainer:** openCenter CLI Team
**Framework:** [Diátaxis](https://diataxis.fr/)
