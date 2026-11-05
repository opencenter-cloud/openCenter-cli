---
id: "opencenter-secrets-keys-check"
title: "Opencenter_Secrets_Keys_Check"
sidebar_label: "Opencenter_Secrets_Keys_Check"
description: "Check encryption key expiration status"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets keys check

**Purpose:** For operators, developers, documents `opencenter secrets keys check`: what it does, its flags, and its examples.

Check encryption key expiration status

### Synopsis

Check the expiration status of encryption keys across clusters.

This command monitors Age and SSH key expiration to help maintain security
through proactive key rotation. It displays:

  • Days until expiration for each key
  • Warning indicators for keys expiring within threshold (default: 14 days)
  • Error indicators for expired keys
  • Key metadata (cluster, type, fingerprint, creation date)

Keys are tracked in the key registry with default expiration policies:
  • Age keys: 90 days
  • SSH keys: 180 days

Use --all to check all clusters, or --cluster to check a specific cluster.
The --warn-days flag controls the warning threshold.

```
opencenter secrets keys check [flags]
```

### Examples

```
  # Check keys for all clusters
  opencenter secrets keys check --all

  # Check keys for specific cluster
  opencenter secrets keys check --cluster my-cluster

  # Check with custom warning threshold (30 days)
  opencenter secrets keys check --all --warn-days 30

  # Output in JSON format for automation
  opencenter secrets keys check --all --output json
```

### Options

```
      --all              Check keys for all clusters
      --cluster string   Check keys for specific cluster
  -h, --help             help for check
      --output string    Output format: text or json (default "text")
      --warn-days int    Warning threshold in days (default 14)
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter secrets keys](opencenter_secrets_keys.md)	 - Manage SOPS encryption keys
