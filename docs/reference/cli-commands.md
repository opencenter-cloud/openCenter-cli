---
last_updated: 2026-09-25
id: cli-commands
title: "CLI Commands Reference"
sidebar_label: CLI Commands Reference
description: Entry points and navigation for the generated openCenter CLI command reference.
doc_type: reference
audience: "all users"
tags: [cli, commands, flags, reference]
---
# CLI Commands Reference

**Purpose:** For all users, documents entry points and navigation for the generated openCenter CLI command reference.

The per-command pages under [`opencenter/`](opencenter/opencenter.md) are generated from the
live Cobra command tree. They are the canonical reference for command usage,
arguments, flags, examples, and parent/child links. Regenerate them with:

```bash
mise run docs-gen
```

## Command groups

* [`opencenter`](opencenter/opencenter.md) — root command and global flags.
* [`cluster`](opencenter/opencenter_cluster.md) — cluster configuration,
  lifecycle, deployment, services, providers, backups, and drift.
* [`secrets`](opencenter/opencenter_secrets.md) — secret storage, validation,
  synchronization, and key operations.
* [`settings`](opencenter/opencenter_settings.md) — CLI settings and editor
  integration.
* [`plugins`](opencenter/opencenter_plugins.md) — external plugin discovery.
* [`version`](opencenter/opencenter_version.md) and
  [`shell-init`](opencenter/opencenter_shell-init.md) — build information and
  shell integration.

The generated pages also include completion commands and any command-specific
flags. Do not duplicate that tree here: if a command's spelling or flags differ
from this overview, the generated page and `opencenter <command> --help` are
authoritative.

## Common global flags

These persistent flags are defined on the root command and inherited by
subcommands:

| Flag | Description |
| --- | --- |
| `--config-dir` | Override the configuration directory. |
| `--dry-run` | Preview supported mutating operations without writing or acting. |
| `--log-level` | Set `debug`, `info`, `warn`, or `error`. |
| `--output` | Select `text`, `json`, or `yaml` where supported. |
| `--quiet` | Suppress nonessential human output. |
| `--yes` | Answer confirmation prompts. |

## Configuration examples

Use the generated [`cluster set`](opencenter/opencenter_cluster_set.md) page
for the current dotted-key syntax. For example:

```bash
opencenter cluster set prod-cluster \
  opencenter.infrastructure.provider=vmware \
  opencenter.infrastructure.cloud.vmware.datacenter=DC1
```

For configuration structure and validation, see [Configuration Schema](configuration-schema.md),
[Validation Rules](validation-rules.md), and [Providers](providers.md).
