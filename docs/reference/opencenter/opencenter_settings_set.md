---
id: "opencenter-settings-set"
title: "Opencenter_Settings_Set"
sidebar_label: "Opencenter_Settings_Set"
description: "Set a settings value using dot notation"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings set

**Purpose:** For operators, developers, documents `opencenter settings set`: what it does, its flags, and its examples.

Set a settings value using dot notation

### Synopsis

Set a settings value using dot notation.

Examples:
  opencenter settings set logging.level debug
  opencenter settings set paths.clustersDir ~/my-clusters
  opencenter settings set paths.pluginsDir ~/my-plugins
  opencenter settings set paths.stateDir ~/.local/state/opencenter
  opencenter settings set behavior.autoConfirm true
  opencenter settings set behavior.validation online
  opencenter settings set cluster_defaults.provider openstack
  opencenter settings set cluster_defaults.gitops_auth_method token
  opencenter settings set cluster_defaults.base_domain k8s.example.com
  opencenter settings set cluster_defaults.kubernetes_version 1.34.3

Supported settings sections:
  - logging.level (debug, info, warn, error)
  - logging.format (text, json, yaml)
  - logging.output (stdout, stderr, or file path)
  - logging.file.maxSize (integer, MB)
  - logging.file.maxBackups (integer)
  - logging.file.maxAge (integer, days)
  - logging.file.compress (boolean)
  - paths.settingsDir (string)
  - paths.clustersDir (string)
  - paths.pluginsDir (string)
  - paths.stateDir (string)
  - behavior.autoConfirm (boolean)
  - behavior.dryRun (boolean)
  - behavior.validation (string: offline, online)
  - cluster_defaults.provider (string)
  - cluster_defaults.region (string)
  - cluster_defaults.environment (string)
  - cluster_defaults.gitops_auth_method (string: ssh, token)
  - cluster_defaults.base_domain (string)
  - cluster_defaults.admin_email (string)
  - cluster_defaults.kubernetes_version (string)
  - cluster_defaults.cni (string: calico, cilium, kube-ovn)
  - cluster_defaults.ssh_user (string)

```
opencenter settings set <key> <value> [flags]
```

### Options

```
  -h, --help   help for set
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

* [opencenter settings](opencenter_settings.md)	 - Manage CLI settings
