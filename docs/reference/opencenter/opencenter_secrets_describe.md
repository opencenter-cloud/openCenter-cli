---
id: "opencenter-secrets-describe"
title: "Opencenter_Secrets_Describe"
sidebar_label: "Opencenter_Secrets_Describe"
description: "Show metadata for a single secret"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets describe

**Purpose:** For operators, developers, documents `opencenter secrets describe`: what it does, its flags, and its examples.

Show metadata for a single secret

```
opencenter secrets describe <name> [flags]
```

### Options

```
  -h, --help   help for describe
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
