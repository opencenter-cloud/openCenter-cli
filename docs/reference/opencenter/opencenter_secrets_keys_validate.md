---
id: "opencenter-secrets-keys-validate"
title: "Opencenter_Secrets_Keys_Validate"
sidebar_label: "Opencenter_Secrets_Keys_Validate"
description: "Validate Age key configuration and SOPS setup"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets keys validate

**Purpose:** For operators, developers, documents `opencenter secrets keys validate`: what it does, its flags, and its examples.

Validate Age key configuration and SOPS setup

### Synopsis

Validate Age key configuration and SOPS setup.

This command performs comprehensive validation of the SOPS configuration:
• Checks Age key file existence and permissions
• Validates Age key format and functionality
• Tests SOPS encryption/decryption functionality
• Verifies .sops.yaml configuration
• Ensures all required tools are installed

Use this command to troubleshoot SOPS issues or verify configuration
after key rotation or setup changes.

```
opencenter secrets keys validate [flags]
```

### Options

```
      --config-file string   Path to SOPS configuration file (default ".sops.yaml")
      --dry-run              Show what would be done without making changes
  -h, --help                 help for validate
      --key-file string      Path to Age key file (default: ~/.config/sops/age/keys.txt)
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
