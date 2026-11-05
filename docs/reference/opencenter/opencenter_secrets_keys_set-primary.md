---
id: "opencenter-secrets-keys-set-primary"
title: "Opencenter_Secrets_Keys_Set Primary"
sidebar_label: "Opencenter_Secrets_Keys_Set Primary"
description: "Select an existing key as the cluster primary"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets keys set-primary

**Purpose:** For operators, developers, documents `opencenter secrets keys set-primary`: what it does, its flags, and its examples.

Select an existing key as the cluster primary

```
opencenter secrets keys set-primary [flags]
```

### Options

```
      --cluster string       cluster name or organization/cluster
      --fingerprint string   exact key fingerprint
  -h, --help                 help for set-primary
      --type string          key type: age or ssh
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
