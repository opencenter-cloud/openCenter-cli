---
id: "opencenter-secrets-list"
title: "Opencenter_Secrets_List"
sidebar_label: "Opencenter_Secrets_List"
description: "List secrets associated with the current cluster"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets list

**Purpose:** For operators, developers, documents `opencenter secrets list`: what it does, its flags, and its examples.

List secrets associated with the current cluster

```
opencenter secrets list [flags]
```

### Options

```
  -h, --help                help for list
      --label stringArray   Filter secrets by labels in key=value form
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
