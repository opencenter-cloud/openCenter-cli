---
id: "opencenter-secrets-keys-reconcile"
title: "Opencenter_Secrets_Keys_Reconcile"
sidebar_label: "Opencenter_Secrets_Keys_Reconcile"
description: "Reconcile SOPS recipients with the key registry"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets keys reconcile

**Purpose:** For operators, developers, documents `opencenter secrets keys reconcile`: what it does, its flags, and its examples.

Reconcile SOPS recipients with the key registry

```
opencenter secrets keys reconcile [flags]
```

### Options

```
      --apply            Import recipients missing from the active registry
      --cluster string   cluster name or organization/cluster
  -h, --help             help for reconcile
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

* [opencenter secrets keys](opencenter_secrets_keys.md)	 - Manage SOPS encryption keys
