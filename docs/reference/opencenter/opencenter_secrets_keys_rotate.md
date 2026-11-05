---
id: "opencenter-secrets-keys-rotate"
title: "Opencenter_Secrets_Keys_Rotate"
sidebar_label: "Opencenter_Secrets_Keys_Rotate"
description: "Rotate SOPS files or cluster encryption keys"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets keys rotate

**Purpose:** For operators, developers, documents `opencenter secrets keys rotate`: what it does, its flags, and its examples.

Rotate SOPS files or cluster encryption keys

### Synopsis

Rotate SOPS files or cluster encryption keys.

Without --cluster, --type, or --complete, this command rotates the local Age
key and re-encrypts SOPS files under --path.

With --cluster, it rotates a cluster encryption key. Use --type age or --type
ssh to choose the key type, and add --complete to finish a dual-key rotation
by removing the old key.

If any step fails, the old key is restored automatically.

```
opencenter secrets keys rotate [flags]
```

### Options

```
      --cluster string    cluster name or organization/cluster for cluster key rotation
      --complete          complete dual-key cluster rotation by removing the old key
      --dry-run           Show what would be done without making changes
  -h, --help              help for rotate
      --key-file string   Path to Age key file (default: ~/.config/sops/age/keys.txt)
      --path string       Path to search for SOPS files to re-encrypt (default ".")
      --type string       cluster key type to rotate: age or ssh
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
