---
id: index
title: "openCenter CLI Documentation"
sidebar_label: openCenter CLI Documentation
description: Landing page for the openCenter CLI documentation, organised by lifecycle category.
doc_type: reference
audience: "all users"
tags: [opencenter, cli, documentation, home]
---
# openCenter CLI Documentation

**Purpose:** For all users, points to the openCenter CLI documentation organised by lifecycle category (getting started, operations, reference, concepts, contributing).

openCenter is a command-line tool that turns a single declarative YAML file into a production-ready Kubernetes cluster with GitOps management. It standardises cluster deployment across OpenStack, VMware, Baremetal, and Kind, and ships configuration validation, secrets management, and FluxCD-ready repository generation.

## What openCenter does

* **Configuration-first workflow.** One YAML file declares infrastructure, Kubernetes, services, and secrets.
* **Built-in validation.** Schema, business-rule, and provider-specific checks run before any infrastructure is touched.
* **GitOps native.** Generates a complete FluxCD repository with Kustomize overlays for cluster-specific overrides.
* **Secrets management.** SOPS Age encryption keeps secrets safe in Git.
* **Platform services.** 20+ pre-configured services (monitoring, logging, ingress, auth, storage, backup).

## Quick start

* [Getting Started](getting-started/getting-started.md) -- create your first cluster in 10 minutes.
* [CLI Commands Reference](reference/cli-commands.md) -- complete command tree.
* [Configuration Schema](reference/configuration-schema.md) -- file structure and field reference.
* [Glossary](glossary.md) -- terminology used throughout these docs.

## Getting started

Tutorial-style walkthroughs for first-time setup.

* [Getting Started](getting-started/getting-started.md) -- end-to-end first cluster.
* [OpenStack First Cluster](getting-started/openstack-first-cluster.md) -- deploy on OpenStack.
* [Kind Local Development](getting-started/kind-local-development.md) -- local development cluster.
* [VMware Deployment](getting-started/vmware-deployment.md) -- deploy on pre-provisioned vSphere VMs.
* [Multi-Cluster Management](getting-started/multi-cluster-setup.md) -- manage several clusters.

## Operations

Task-oriented how-to guides for day-2 work.

|     |     |
| --- | --- |
| Cluster creation | [Create a Kind cluster](operations/create-kind-cluster.md)<br> [Create an OpenStack cluster](operations/create-openstack-cluster.md)<br> [Deploy an OpenStack cluster](operations/deploy-openstack-cluster.md) |
| Configuration | [Validate configuration](operations/validate-configuration.md)<br> [Customize services](operations/customize-services.md)<br> [Configure networking](operations/configure-networking.md) |
| Secrets and bootstrap | [Manage secrets](operations/manage-secrets.md)<br> [Configure Flux bootstrap auth](operations/flux-bootstrap-methods.md) |
| Day-2 operations | [Add worker pools](operations/add-worker-pools.md)<br> [Backup and restore](operations/backup-and-restore.md)<br> [Upgrade Kubernetes](operations/upgrade-kubernetes.md)<br> [Migrate clusters](operations/migrate-clusters.md) |
| Integration and plugins | [Integrate CI/CD](operations/integrate-ci-cd.md)<br> [Create and install a CLI plugin](operations/create-install-cli-plugin.md) |
| Troubleshooting | [Troubleshoot deployment](operations/troubleshoot-deployment.md) |

## Providers

Provider-specific guides.

* [Infrastructure Providers Overview](providers/README.md)
* [VMware Provider Guide](providers/vmware.md)
* [VMware Quick Start](providers/vmware-quick-start.md)
* [VMware Terraform Template](providers/vmware-terraform-template.md)

## Reference

Lookup material -- structured, complete, scan-friendly.

|     |     |
| --- | --- |
| CLI and configuration | [CLI Commands](reference/cli-commands.md)<br> [Configuration Schema](reference/configuration-schema.md)<br> [Configuration Precedence](reference/configuration-precedence.md)<br> [Default Values](reference/default-values.md)<br> [Environment Variables](reference/environment-variables.md)<br> [Exit Codes](reference/exit-codes.md)<br> [File Locations](reference/file-locations.md) |
| Platform and providers | [Platform Services](reference/platform-services.md)<br> [Providers](reference/providers.md)<br> [Validation Rules](reference/validation-rules.md) |
| Security and tooling | [Audit Signing Key](reference/audit-key.md)<br> [Mise Tasks](reference/mise-tasks.md) |

## Concepts

Background and rationale -- the "why" behind the design.

* [Architecture](concepts/architecture.md)
* [Reference Architecture](concepts/reference-architecture.md)
* [GitOps Workflow](concepts/gitops-workflow.md)
* [Configuration Lifecycle](concepts/configuration-lifecycle.md)
* [Security Model](concepts/security-model.md)
* [Security Update Design](concepts/security-update-design.md)
* [Services and Templates](concepts/services-templates.md)
* [Drift Detection](concepts/drift-detection.md)
* [Plugin Internal Services](concepts/plugin-internal-services.md)
* [Plugin External CLI](concepts/plugin-external-cli.md)
* [Provider Comparison](concepts/provider-comparison.md)

## Contributing

Developer documentation for contributors and maintainers.

* [Contributing Guide](contributing/contributing.md)
* [Development Setup](contributing/development-setup.md)
* [Code Structure](contributing/code-structure.md)
* [Testing Guide](contributing/testing-guide.md)
* [Adding Providers](contributing/adding-providers.md)
* [Adding Services](contributing/adding-services.md)
* [Build System](contributing/build-system.md)
* [Release Process](contributing/release-process.md)

## Release notes

* [1.0.0-rc01](release/1.0.0-rc01.md)

## Documentation framework

These docs follow the [Diátaxis framework](https://diataxis.fr/) but are organised by lifecycle category (getting-started, operations, reference, concepts, contributing) rather than by Diátaxis type. The Diátaxis type is recorded as `:page-type:` in each page’s attribute block.

## Getting help

* [GitHub Issues](https://github.com/opencenter-cloud/openCenter-cli/issues) -- report bugs or request features.
* [Open a docs issue](https://github.com/opencenter-cloud/openCenter-cli/issues/new) for documentation problems.