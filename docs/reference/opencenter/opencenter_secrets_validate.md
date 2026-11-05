---
id: "opencenter-secrets-validate"
title: "Opencenter_Secrets_Validate"
sidebar_label: "Opencenter_Secrets_Validate"
description: "Validate secrets for configuration drift"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets validate

**Purpose:** For operators, developers, documents `opencenter secrets validate`: what it does, its flags, and its examples.

Validate secrets for configuration drift

### Synopsis

Validate secrets by comparing config file against encrypted manifests.

This command detects configuration drift between the cluster's config file
(.k8s-<cluster>-config.yaml) and the deployed encrypted manifests. It identifies:

  • Secrets that differ between config and manifests (drift)
  • Secrets in config but missing from manifests
  • Secrets in manifests but not in config (orphaned)
  • Unencrypted secrets in manifests (security violations)

The validation returns exit code 0 if no drift is detected, or exit code 1 if
drift exists. This makes it suitable for CI/CD pipelines.

If no cluster name is provided, uses the currently active cluster.

```
opencenter secrets validate [cluster] [flags]
```

### Examples

```
  # Validate secrets for active cluster
  opencenter secrets validate

  # Validate secrets for specific cluster
  opencenter secrets validate my-cluster

  # Auto-fix detected drift
  opencenter secrets validate my-cluster --fix

  # Output in JSON format for CI/CD
  opencenter secrets validate my-cluster --output json
```

### Options

```
      --fix    Automatically fix drift by running opencenter secrets sync
  -h, --help   help for validate
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter secrets](opencenter_secrets.md)	 - Manage secrets across backends
