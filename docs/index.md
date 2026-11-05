---
id: index
title: "openCenter CLI Documentation"
sidebar_label: openCenter CLI Documentation
description: Source index for the openCenter CLI reader-facing documentation.
doc_type: reference
audience: "users, operators, contributors"
tags: [opencenter, cli, documentation, index]
last_updated: 2026-11-05
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
- [Install the CLI](reference/installation.md) — installer options, checksum
verification, and uninstall.
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

- [Install the CLI](reference/installation.md)
- [Create a Kind cluster](operations/create-kind-cluster.md)
- [Create an OpenStack cluster](operations/create-openstack-cluster.md)
- [Deploy from a cluster file](operations/deploy-openstack-cluster.md)
- [Destroy an OpenStack cluster](operations/destroy-openstack-cluster.md)
- [Validate configuration](operations/validate-configuration.md)
- [Customize services](operations/customize-services.md)
- [Service enablement profiles](operations/deployment-profiles.md)
- [Normalize a legacy configuration](operations/normalize-legacy-renderer-metadata.md)
- [Configure networking](operations/configure-networking.md)
- [Flux and GitOps configuration](operations/flux-bootstrap-methods.md)
- [Manage secrets](operations/manage-secrets.md)
- [Manage worker pools](operations/manage-worker-pools.md)
- [Add a worker pool](operations/add-worker-pools.md)
- [Production OpenStack deployment](operations/openstack-production-deployment.md)
- [Back up and restore](operations/backup-and-restore.md)
- [Upgrade Kubernetes](operations/upgrade-kubernetes.md)
- [Migrate clusters](operations/migrate-clusters.md)
- [Troubleshoot deployment](operations/troubleshoot-deployment.md)
- [Use openCenter in CI](operations/integrate-ci-cd.md)
- [Reconcile GitOps changes](operations/reconcile-gitops-changes.md)
- [Create and install a CLI plugin](operations/create-install-cli-plugin.md)
- [VMware provider guide](providers/vmware.md)
- [Infrastructure providers](providers/README.md)
 
Every page under `docs/operations/` is listed above. The one-off notice about the
`k8s_hardening` and `os_hardening` default change lives with the release
material as [Hardening defaults changed](release/hardening-defaults-change.md).

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
- [Audit signing key](reference/audit-key.md)
- [GitHub Actions workflows](reference/github-actions-workflows.md)
- [Infrastructure providers](reference/providers.md)
- [Platform services](reference/platform-services.md)
- [Per-service reference](reference/services/index.md)
- [Mise tasks](reference/mise-tasks.md)
- [Local RustFS development service](reference/localdev-rustfs.md)
- [Exit codes](reference/exit-codes.md)
- [Error codes](reference/error-codes.md)
- [Release notes](release/index.md)

The [generated command pages](reference/cli-commands.md) are refreshed from
the built-in Cobra command tree; use the command reference rather than copying
flags into another page.

## Explanations

Background and rationale:

- [Architecture: rationale and design](concepts/architecture.md)
- [CLI architecture (contributor reference)](architecture.md)
- [Reference architecture](concepts/reference-architecture.md)
- [GitOps workflow](concepts/gitops-workflow.md)
- [Configuration lifecycle](concepts/configuration-lifecycle.md)
- [Security model](concepts/security-model.md)
- [Secret and config separation](concepts/security-update-design.md)
- [Services and templates](concepts/services-templates.md)
- [Drift detection](concepts/drift-detection.md)
- [Provider comparison](concepts/provider-comparison.md)
- [External CLI plugins](concepts/plugin-external-cli.md)
- [Internal service plugins](concepts/plugin-internal-services.md)
- [Deployment recommendations](recommendations.md)

## Contributing

Contributor-facing how-to and reference material:

- [Contributing guide](contributing/contributing.md)
- [Development setup](contributing/development-setup.md)
- [Pre-commit hooks](contributing/pre-commit-hooks.md)
- [Codebase organization](contributing/code-structure.md)
- [Testing guide](contributing/testing-guide.md)
- [Kind cluster verification](contributing/kind-cluster-verification.md)
- [Adding providers](contributing/adding-providers.md)
- [Adding services](contributing/adding-services.md)
- [Adding a built-in service](contributing/adding-a-built-in-service.md)
- [Build system](contributing/build-system.md)
- [Release process](contributing/release-process.md)
- [Documentation maintenance](README.md)
 
Execution-flow and contract references for contributors changing the core
pipelines:

- [cluster deploy — OpenStack](contributing/cluster-deploy-openstack.md)
- [cluster deploy — Step Matrix per Provider](contributing/cluster-deploy-steps.md)
- [Cluster init details](contributing/cluster-init-details.md)
- [Cluster validate execution flow](contributing/validation.md)
- [Service enable/disable lifecycle](contributing/services.md)
- [Renderer contract](contributing/rendering-contract.md)
- [Descriptor condition schema](contributing/descriptor-condition-schema.md)
- [Overlay rendering security policy](contributing/overlay-security-policy.md)

Repository-only architecture maps live in [CODEMAPS/](CODEMAPS/INDEX.md) and
are intentionally not part of this reader-facing index. The repository map
and editing rules are in [docs/README.md](README.md); code-oriented navigation
is in the root `llms.txt` file.

Documentation validation is repository-only:
[`mise run test-docs`](reference/mise-tasks.md) checks Markdown, frontmatter,
links, page reachability, and Mermaid fence structure without rendering or
publishing a site. Publication is unconfigured in this repository; no docs
deployment platform is implied. `test-docs` is chained by
[`mise run test:all`](reference/mise-tasks.md), so a local full validation
cannot pass with the documentation gates unrun. The local full-package Go sweep
is [`mise run test-go-all`](reference/mise-tasks.md), separate from both.

## Framework classification

Pages use [Diátaxis](https://diataxis.fr/) metadata without mirroring its four
types as directory names: tutorials are in `getting-started/`, how-to guides
in `operations/`, reference pages in `reference/`, and explanations in
`concepts/`. Contributor pages may use the type that best matches their scope.

For repository support, use [GitHub Issues](https://github.com/opencenter-cloud/openCenter-cli/issues).

To report a vulnerability, use the process in `SECURITY.md` at the repository
root. It is deliberately not reproduced here: the reporting channel, disclosure
window, and supported version list must have one owner, and that file is it. The
threat model the process assumes is documented in
[Security Model](concepts/security-model.md).
