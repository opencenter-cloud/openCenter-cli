---
id: "opencenter-secrets-get"
title: "Opencenter_Secrets_Get"
sidebar_label: "Opencenter_Secrets_Get"
description: "Download and decrypt a secret"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets get

**Purpose:** For operators, developers, documents `opencenter secrets get`: what it does, its flags, and its examples.

Download and decrypt a secret

```
opencenter secrets get <name> [flags]
```

### Options

```
  -h, --help                 help for get
      --output-file string   Path to save the secret
      --show                 Print secret to stdout (insecure)
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
