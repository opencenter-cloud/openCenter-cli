---
id: "opencenter-secrets"
title: "Opencenter_Secrets"
sidebar_label: "Opencenter_Secrets"
description: "Manage secrets across backends"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets

**Purpose:** For operators, developers, documents `opencenter secrets`: what it does, its flags, and its examples.

Manage secrets across backends

### Synopsis

Manage secrets across different backends (Barbican, SOPS, file) based on cluster configuration.

```
opencenter secrets [flags]
```

### Options

```
  -h, --help   help for secrets
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

* [opencenter](opencenter.md)	 - opencenter CLI manages cluster configurations and GitOps scaffolding
* [opencenter secrets decrypt](opencenter_secrets_decrypt.md)	 - Decrypt secrets in YAML files
* [opencenter secrets delete](opencenter_secrets_delete.md)	 - Delete a secret
* [opencenter secrets describe](opencenter_secrets_describe.md)	 - Show metadata for a single secret
* [opencenter secrets encrypt](opencenter_secrets_encrypt.md)	 - Encrypt secrets in YAML files
* [opencenter secrets get](opencenter_secrets_get.md)	 - Download and decrypt a secret
* [opencenter secrets keys](opencenter_secrets_keys.md)	 - Manage SOPS encryption keys
* [opencenter secrets list](opencenter_secrets_list.md)	 - List secrets associated with the current cluster
* [opencenter secrets login](opencenter_secrets_login.md)	 - Create or refresh a Keystone token
* [opencenter secrets set](opencenter_secrets_set.md)	 - Create or update a secret
* [opencenter secrets status](opencenter_secrets_status.md)	 - Show encryption status of YAML files
* [opencenter secrets sync](opencenter_secrets_sync.md)	 - Synchronize secrets from config to encrypted manifests
* [opencenter secrets validate](opencenter_secrets_validate.md)	 - Validate secrets for configuration drift
