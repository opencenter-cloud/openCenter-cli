---
id: "opencenter-secrets-keys-generate"
title: "Opencenter_Secrets_Keys_Generate"
sidebar_label: "Opencenter_Secrets_Keys_Generate"
description: "Generate new Age key pair for SOPS encryption"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets keys generate

**Purpose:** For operators, developers, documents `opencenter secrets keys generate`: what it does, its flags, and its examples.

Generate new Age key pair for SOPS encryption

### Synopsis

Generate a new Age key pair for SOPS encryption.

This command creates a new Age key pair and optionally updates the .sops.yaml
configuration file with the new public key. The private key is saved securely
with appropriate file permissions (600).

The generated public key should be used in .sops.yaml configuration and
cluster specifications for SOPS encryption.

```
opencenter secrets keys generate [flags]
```

### Options

```
      --dry-run              Show what would be done without making changes
  -h, --help                 help for generate
      --key-file string      Path to save the Age key file (default: ~/.config/sops/age/keys.txt)
      --update-sops-config   Update .sops.yaml configuration with new public key (default true)
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter secrets keys](opencenter_secrets_keys.md)	 - Manage SOPS encryption keys
