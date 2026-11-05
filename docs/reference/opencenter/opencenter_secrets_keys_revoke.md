---
id: "opencenter-secrets-keys-revoke"
title: "Opencenter_Secrets_Keys_Revoke"
sidebar_label: "Opencenter_Secrets_Keys_Revoke"
description: "Revoke encryption keys for users or compromised keys"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets keys revoke

**Purpose:** For operators, developers, documents `opencenter secrets keys revoke`: what it does, its flags, and its examples.

Revoke encryption keys for users or compromised keys

### Synopsis

Revoke encryption keys to remove access for departed team members or compromised keys.

This command provides three revocation modes:

1. Revoke by user (--user):
   • Removes all keys associated with a user email
   • Re-encrypts all secrets without the user's keys
   • Logs revocation event to audit log

2. Revoke by fingerprint (--key):
   • Removes a specific key by fingerprint
   • Re-encrypts all secrets without the revoked key
   • Useful for revoking individual compromised keys

3. Emergency revocation (--emergency):
   • Immediately revokes the specified key
   • Generates a new primary key
   • Re-encrypts all secrets with the new key
   • Use when a key is compromised and immediate action is needed

The revocation process:
  • Validates that at least one key will remain active
  • Removes the revoked key from .sops.yaml
  • Re-encrypts all manifests without the revoked key
  • Updates key registry with revocation details
  • Logs the revocation event for audit trail

If no cluster name is provided, uses the currently active cluster.

```
opencenter secrets keys revoke [flags]
```

### Examples

```
  # Revoke all keys for a user
  opencenter secrets keys revoke --cluster my-cluster --user user@example.com

  # Revoke a specific key by fingerprint
  opencenter secrets keys revoke --cluster my-cluster --key age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn

  # Emergency revocation (generates new key immediately)
  opencenter secrets keys revoke --cluster my-cluster --key <fingerprint> --emergency

  # Preview revocation without making changes
  opencenter secrets keys revoke --cluster my-cluster --user user@example.com --dry-run
```

### Options

```
      --cluster string   cluster name or organization/cluster
      --dry-run          Preview revocation without making changes
      --emergency        Perform emergency revocation with new key generation
  -h, --help             help for revoke
      --key string       Revoke specific key by fingerprint
      --user string      Revoke all keys for user email
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
