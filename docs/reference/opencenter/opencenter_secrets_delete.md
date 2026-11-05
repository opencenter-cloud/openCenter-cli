---
id: "opencenter-secrets-delete"
title: "Opencenter_Secrets_Delete"
sidebar_label: "Opencenter_Secrets_Delete"
description: "Delete a secret"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets delete

**Purpose:** For operators, developers, documents `opencenter secrets delete`: what it does, its flags, and its examples.

Delete a secret

```
opencenter secrets delete <name> [flags]
```

### Options

```
      --force   Force deletion of a secret
  -h, --help    help for delete
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
