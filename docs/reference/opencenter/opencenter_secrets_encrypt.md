---
id: "opencenter-secrets-encrypt"
title: "Opencenter_Secrets_Encrypt"
sidebar_label: "Opencenter_Secrets_Encrypt"
description: "Encrypt secrets in YAML files"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets encrypt

**Purpose:** For operators, developers, documents `opencenter secrets encrypt`: what it does, its flags, and its examples.

Encrypt secrets in YAML files

### Synopsis

Encrypt secrets in YAML files using SOPS.

This command finds all unencrypted YAML files that match SOPS configuration
rules and encrypts them using the configured Age keys. By default, it creates
backups of the original files for safety.

The encryption process:
1. Creates timestamped backups of original files (unless --no-backup is set)
2. Encrypts files using SOPS with configured Age keys
3. Validates successful encryption
4. Reports results and backup locations

Use this command when you want to encrypt secrets in your project.

```
opencenter secrets encrypt [flags]
```

### Options

```
  -h, --help          help for encrypt
      --no-backup     Skip backup creation before encryption
      --path string   Path to search for files to encrypt (default ".")
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
