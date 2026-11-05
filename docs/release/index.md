---
last_updated: 2026-11-05
id: release-index
title: "Release Notes Index"
sidebar_label: Release Notes
description: Index of openCenter CLI release notes, newest first, with the upgrade action each release requires.
doc_type: reference
audience: "operators, platform engineers, contributors"
tags: [release-notes, changelog, releases, index]
---
# Release Notes Index

**Purpose:** For operators and platform engineers, indexes every published
openCenter CLI release note newest-first, and names the upgrade action each one
requires so you can tell whether a release is a re-deploy or needs manual work.

Release notes describe what changed between release candidates. They are not the
installation guide: to install or pin a specific version, use
[Installation](../reference/installation.md).

## Current release

The repository's `VERSION` file reads `1.0.0`. The highest published tag is
`v1.0.0-rc07`; the newest release note in this tree is
[v1.0.0-rc08](1.0.0-rc08.md), pending its tag. Treat rc08 as the current
pre-release of 1.0.0.

## Release notes

Newest first. Each page lists features, fixes, migrations, and improvements.

| Release | Date | Headline | Upgrade action |
| --- | --- | --- | --- |
| [v1.0.0-rc08](1.0.0-rc08.md) | 2026-11-05 | `cluster reconcile` Day-2 convergence workflow, verified `scripts/install.sh` installer, Headlamp branding plugin, CCM LB naming/subnet/upgrade-stall fixes, Go 1.27.3 | Re-deploy; building from source now requires Go 1.27.3, and re-run `cluster generate` so CCM override values pick up cluster name and `subnet-id` |
| [v1.0.0-rc07](1.0.0-rc07.md) | 2026-11-02 | Lifecycle contract v3, automatic OpenTofu network output binding, OpenStack Magnum managed provider, cert-manager Cloudflare DNS automation | Re-deploy; review the Magnum provider and network-binding sections before upgrading a production cluster |
| [v1.0.0-rc06](1.0.0-rc06.md) | 2026-11-02 | Transactional ownership-aware GitOps promotion, hardened SOPS Age key lifecycle, explicit provider and per-service storage plan/apply | Re-deploy; storage operations moved from one sync command to explicit plan/apply, so update any scripted calls |
| [v1.0.0-rc05](1.0.0-rc05.md) | 2026-11-02 | Provider-agnostic configuration validation with hints, `vip_interface` networking field, SOPS Age secret fix during deploy | Re-deploy |
| [v1.0.0-rc04](1.0.0-rc04.md) | 2026-11-02 | OpenStack cluster sync command, GitOps auth selection with base secret reconciliation, GitOps manifest fixes | Re-deploy |
| [v1.0.0-rc03](1.0.0-rc03.md) | 2026-11-02 | Release-tag pinning for the base services GitRepository, legacy layout false-positive fix, AsciiDoc-to-Markdown migration | Re-deploy |
| [v1.0.0-rc02](1.0.0-rc02.md) | 2026-11-02 | Worker pool management, VMware and Baremetal deployment, v2 configuration system rewrite, cosign artifact signing | **Migration required** -- the configuration system was rewritten to v2; validate existing configurations before deploying |
| [v1.0.0-rc01](1.0.0-rc01.md) | 2026-11-02 | First 1.0.0 release candidate | Initial adoption |

The dates above are the release-note page dates. For the authoritative tag date,
use `git log -1 --format=%ci v1.0.0-rc07` or the GitHub release entry at
<https://github.com/opencenter-cloud/opencenter-cli/releases>.

## Behaviour changes that outlive one release

- [Hardening Defaults Changed](hardening-defaults-change.md) -- defaults that
  became stricter, and the configuration you must add to keep a previous
  behaviour. Read this before upgrading an existing cluster, regardless of which
  release introduced the change.

## Related

- [Installation](../reference/installation.md) - install or pin a version.
- [Mise tasks](../reference/mise-tasks.md) - the build and release tasks.
- [Documentation index](../index.md) - all reader-facing documentation.
