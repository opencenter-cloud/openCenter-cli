---
id: "opencenter-secrets-status"
title: "Opencenter_Secrets_Status"
sidebar_label: "Opencenter_Secrets_Status"
description: "Show encryption status of YAML files"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets status

**Purpose:** For operators, developers, documents `opencenter secrets status`: what it does, its flags, and its examples.

Show encryption status of YAML files

### Synopsis

Show the encryption status of YAML files in your project.

This command searches for YAML files in the specified path and displays
their SOPS encryption status. It identifies:
• Encrypted files (already protected with SOPS)
• Unencrypted files that should be encrypted (based on SOPS rules)

Use this command to get an overview of all secrets in your project and
verify which files are encrypted and which need encryption.

```
opencenter secrets status [flags]
```

### Options

```
  -h, --help          help for status
      --path string   Path to search for YAML files (default ".")
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
