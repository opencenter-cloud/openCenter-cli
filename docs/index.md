---
id: index
title: "openCenter CLI Documentation"
sidebar_label: openCenter CLI Documentation
description: Source index for the openCenter CLI reader-facing documentation.
doc_type: reference
audience: "users, operators, contributors"
tags: [opencenter, cli, documentation, index]
last_updated: 2026-09-25
---

# openCenter CLI Documentation

**Purpose:** For users and contributors, indexes the reader-facing Markdown
source in this repository. This is the canonical documentation index; it does
not claim that a hosted documentation site is published or available.

openCenter CLI manages declarative Kubernetes cluster configuration, provider
workflows, generated GitOps assets, and related secrets operations. The
available provider paths and their boundaries are documented in the [provider
reference](reference/providers.md).

## Start here

- [Getting Started tutorial](getting-started/getting-started.md) — first
  configuration and cluster workflow.
- [OpenStack first cluster](getting-started/openstack-first-cluster.md) —
  OpenStack walkthrough.
- [Kind local development](getting-started/kind-local-development.md) — local
  Kind workflow.
- [CLI command reference](reference/cli-commands.md) — command groups and
  flags.
- [Configuration schema](reference/configuration-schema.md) — configuration
  fields and validation.
- [Glossary](glossary.md) — terms used in these pages.

## Tutorials

Tutorials guide a reader through a working outcome:

- [Getting started](getting-started/getting-started.md)
- [OpenStack first cluster](getting-started/openstack-first-cluster.md)
- [Kind local development](getting-started/kind-local-development.md)
- [VMware deployment](getting-started/vmware-deployment.md)
- [Multi-cluster configuration](getting-started/multi-cluster-setup.md)

## How-to guides

Task-oriented operations are grouped by outcome:

- [Validate configuration](operations/validate-configuration.md)
- [Customize services](operations/customize-services.md)
- [Configure networking](operations/configure-networking.md)
- [Manage secrets](operations/manage-secrets.md)
- [Manage worker pools](operations/manage-worker-pools.md)
- [Production OpenStack deployment](operations/openstack-production-deployment.md)
- [Back up and restore](operations/backup-and-restore.md)
- [Upgrade Kubernetes](operations/upgrade-kubernetes.md)
- [Migrate clusters](operations/migrate-clusters.md)
- [Troubleshoot deployment](operations/troubleshoot-deployment.md)
- [Create and install a CLI plugin](operations/create-install-cli-plugin.md)
- [VMware provider guide](providers/vmware.md)

The complete task set remains in the `docs/operations/` source directory;
individual pages are linked from this index as they become reader entry points.

## Reference

Lookup material for commands, configuration, providers, and services:

- [CLI commands](reference/cli-commands.md)
- [Configuration schema](reference/configuration-schema.md)
- [Configuration precedence](reference/configuration-precedence.md)
- [Default values](reference/default-values.md)
- [Environment variables](reference/environment-variables.md)
- [File locations](reference/file-locations.md)
- [GitOps configuration](reference/gitops-configuration.md)
- [Validation rules](reference/validation-rules.md)
- [Infrastructure providers](reference/providers.md)
- [Platform services](reference/platform-services.md)
- [Per-service reference](reference/services/index.md)
- [Mise tasks](reference/mise-tasks.md)
- [Exit codes](reference/exit-codes.md)

The [generated command pages](reference/cli-commands.md) are refreshed from
the built-in Cobra command tree; use the command reference rather than copying
flags into another page.

## Explanations

Background and rationale:

- [Architecture](concepts/architecture.md)
- [Reference architecture](concepts/reference-architecture.md)
- [GitOps workflow](concepts/gitops-workflow.md)
- [Configuration lifecycle](concepts/configuration-lifecycle.md)
- [Security model](concepts/security-model.md)
- [Services and templates](concepts/services-templates.md)
- [Drift detection](concepts/drift-detection.md)
- [Provider comparison](concepts/provider-comparison.md)
- [External CLI plugins](concepts/plugin-external-cli.md)

## Contributing

Contributor-facing how-to and reference material:

- [Contributing guide](contributing/contributing.md)
- [Development setup](contributing/development-setup.md)
- [Codebase organization](contributing/code-structure.md)
- [Testing guide](contributing/testing-guide.md)
- [Adding providers](contributing/adding-providers.md)
- [Adding services](contributing/adding-services.md)
- [Build system](contributing/build-system.md)
- [Release process](contributing/release-process.md)
- [Documentation maintenance](README.md)

Repository-only architecture maps live in [CODEMAPS/](CODEMAPS/INDEX.md) and
are intentionally not part of this reader-facing index. The repository map
and editing rules are in [docs/README.md](README.md); code-oriented navigation
is in the root `llms.txt` file.

Documentation validation is also repository-only: contributors can run
[`mise run test-docs`](reference/mise-tasks.md), which checks Markdown,
frontmatter, links, and Mermaid fence structure without rendering or
publishing a site. Publication is unconfigured in this repository; no docs
deployment platform is implied. The local full-package Go sweep is
[`mise run test-go-all`](reference/mise-tasks.md), separate from
[`mise run test:all`](reference/mise-tasks.md).

## Framework classification

Pages use [Diátaxis](https://diataxis.fr/) metadata without mirroring its four
types as directory names: tutorials are in `getting-started/`, how-to guides
in `operations/`, reference pages in `reference/`, and explanations in
`concepts/`. Contributor pages may use the type that best matches their scope.

For repository support, use [GitHub Issues](https://github.com/opencenter-cloud/openCenter-cli/issues).
