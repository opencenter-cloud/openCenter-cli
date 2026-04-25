---
id: cli-commands
title: "CLI Commands Reference"
sidebar_label: CLI Commands
description: Complete reference of all openCenter CLI commands, flags, and options.
doc_type: reference
audience: "all users"
tags: [cli, commands, flags, reference]
---

# CLI Commands Reference

**Purpose:** Complete reference of the shipped `opencenter` command tree, generated from the live Cobra command graph.

Use `go run -tags tools ./cmd/docs` to refresh the generated inventory in [`docs/reference/opencenter`](/Users/victor.palma/projects/openCenter-cloud/openCenter-cli/docs/reference/opencenter).

## Global Flags

| Flag | Description |
|------|-------------|
| `--config-dir` | Configuration directory override |
| `--dry-run` | Print planned actions without executing them |
| `--log-level` | Set log level: `debug`, `info`, `warn`, `error` |
| `--output` | Select output format: `text`, `json`, `yaml` |
| `--quiet` | Suppress nonessential human output |
| `--yes` | Answer yes to confirmation prompts |
| `-h, --help` | Show command help |
| `-v, --version` | Show version information |

## Cluster Set Dot-Notation Examples

```bash
# Update provider and VMware metadata
opencenter cluster set prod-cluster \
  opencenter.infrastructure.provider=vmware \
  opencenter.infrastructure.cloud.vmware.datacenter=DC1 \
  opencenter.infrastructure.cloud.vmware.network=dvpg-prod

# Update a service value before deployment
opencenter cluster set prod-cluster \
  opencenter.services.cert-manager.email=platform@example.com
```

## Root Commands

| Command | Purpose |
|---------|---------|
| `opencenter cluster` | Cluster lifecycle, validation, rendering, drift, services, backup, and import management |
| `opencenter completion` | Shell completion scripts for `bash`, `fish`, `powershell`, and `zsh` |
| `opencenter config` | CLI defaults and local IDE configuration |
| `opencenter plugins` | External plugin discovery |
| `opencenter secrets` | Secret encryption, sync, validation, and key operations |
| `opencenter shell-init` | Session-scoped shell integration for active-cluster context |
| `opencenter version` | Version and build metadata |
| `opencenter <external-plugin>` | Dynamically discovered plugin entrypoints such as `rmpk` |

## Cluster Commands

### Lifecycle and Validation

| Command |
|---------|
| `opencenter cluster active` |
| `opencenter cluster configure` |
| `opencenter cluster deploy` |
| `opencenter cluster describe` |
| `opencenter cluster destroy` |
| `opencenter cluster edit` |
| `opencenter cluster env` |
| `opencenter cluster export` |
| `opencenter cluster generate` |
| `opencenter cluster generate --render-only` |
| `opencenter cluster import` |
| `opencenter cluster import apply` |
| `opencenter cluster import report` |
| `opencenter cluster import scan` |
| `opencenter cluster init` |
| `opencenter cluster list` |
| `opencenter cluster lock` |
| `opencenter cluster normalize` |
| `opencenter cluster doctor` |
| `opencenter cluster set` |
| `opencenter cluster use` |
| `opencenter cluster status` |
| `opencenter cluster unlock` |
| `opencenter cluster validate` |

### Backup

| Command |
|---------|
| `opencenter cluster backup create` |
| `opencenter cluster backup delete` |
| `opencenter cluster backup list` |
| `opencenter cluster backup restore` |
| `opencenter cluster backup schedule` |

### Configuration

| Command |
|---------|
| `opencenter cluster describe` |
| `opencenter cluster edit` |
| `opencenter cluster export` |
| `opencenter cluster normalize` |
| `opencenter cluster set` |

### Drift Detection

| Command |
|---------|
| `opencenter cluster drift detect` |
| `opencenter cluster drift reconcile` |
| `opencenter cluster drift schedule` |

### Services

| Command |
|---------|
| `opencenter cluster service disable` |
| `opencenter cluster service enable` |
| `opencenter cluster service options` |
| `opencenter cluster service status` |

## Config Commands

| Command |
|---------|
| `opencenter config edit` |
| `opencenter config explain` |
| `opencenter config explain cluster-defaults` |
| `opencenter config get` |
| `opencenter config ide` |
| `opencenter config path` |
| `opencenter config reset` |
| `opencenter config set` |
| `opencenter config view` |

## Secrets Commands

### Core Operations

| Command |
|---------|
| `opencenter secrets decrypt` |
| `opencenter secrets delete` |
| `opencenter secrets describe` |
| `opencenter secrets encrypt` |
| `opencenter secrets get` |
| `opencenter secrets list` |
| `opencenter secrets login` |
| `opencenter secrets set` |
| `opencenter secrets status` |
| `opencenter secrets sync` |
| `opencenter secrets validate` |

### Secret Keys

| Command |
|---------|
| `opencenter secrets keys backup` |
| `opencenter secrets keys check` |
| `opencenter secrets keys generate` |
| `opencenter secrets keys revoke` |
| `opencenter secrets keys rotate` |
| `opencenter secrets keys validate` |

## Completion Commands

| Command |
|---------|
| `opencenter completion bash` |
| `opencenter completion fish` |
| `opencenter completion powershell` |
| `opencenter completion zsh` |

## GA Notes

- Canonical infrastructure provider names are `openstack`, `vmware`, `kind`, and `baremetal`.
- `vsphere` remains accepted as a compatibility alias for existing configuration files, but documentation now uses `vmware`.
- AWS-backed integrations such as Route53 and S3 credential flows remain supported where services use them, but AWS is not a GA infrastructure provider.
