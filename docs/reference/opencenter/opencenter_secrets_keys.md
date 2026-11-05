---
id: "opencenter-secrets-keys"
title: "Opencenter_Secrets_Keys"
sidebar_label: "Opencenter_Secrets_Keys"
description: "Manage SOPS encryption keys"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets keys

**Purpose:** For operators, developers, documents `opencenter secrets keys`: what it does, its flags, and its examples.

Manage SOPS encryption keys

### Synopsis

Manage SOPS encryption keys for secrets management.

The keys subcommand group provides operations for Age key lifecycle management:
• Generate: Create new Age key pairs for SOPS encryption
• Rotate: Rotate Age keys with automatic re-encryption of existing secrets
• Backup: Create secure backups of Age keys and SOPS configuration
• Validate: Validate Age key configuration and SOPS setup

These commands integrate with opencenter workflows to provide seamless key management
for standalone clusters and GitOps deployments.

```
opencenter secrets keys [flags]
```

### Options

```
  -h, --help   help for keys
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
* [opencenter secrets keys backup](opencenter_secrets_keys_backup.md)	 - Create backup of Age keys and SOPS configuration
* [opencenter secrets keys check](opencenter_secrets_keys_check.md)	 - Check encryption key expiration status
* [opencenter secrets keys generate](opencenter_secrets_keys_generate.md)	 - Generate new Age key pair for SOPS encryption
* [opencenter secrets keys reconcile](opencenter_secrets_keys_reconcile.md)	 - Reconcile SOPS recipients with the key registry
* [opencenter secrets keys revoke](opencenter_secrets_keys_revoke.md)	 - Revoke encryption keys for users or compromised keys
* [opencenter secrets keys rotate](opencenter_secrets_keys_rotate.md)	 - Rotate SOPS files or cluster encryption keys
* [opencenter secrets keys set-primary](opencenter_secrets_keys_set-primary.md)	 - Select an existing key as the cluster primary
* [opencenter secrets keys validate](opencenter_secrets_keys_validate.md)	 - Validate Age key configuration and SOPS setup
