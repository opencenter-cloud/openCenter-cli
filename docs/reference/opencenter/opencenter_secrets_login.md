---
id: "opencenter-secrets-login"
title: "Opencenter_Secrets_Login"
sidebar_label: "Opencenter_Secrets_Login"
description: "Create or refresh a Keystone token"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets login

**Purpose:** For operators, developers, documents `opencenter secrets login`: what it does, its flags, and its examples.

Create or refresh a Keystone token

```
opencenter secrets login [flags]
```

### Options

```
  -h, --help                help for login
      --password-stdin      Read password from stdin (required for non-interactive use)
      --project-id string   OpenStack project ID
      --username string     OpenStack username
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
