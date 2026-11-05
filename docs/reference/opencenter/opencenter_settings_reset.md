---
id: "opencenter-settings-reset"
title: "Opencenter_Settings_Reset"
sidebar_label: "Opencenter_Settings_Reset"
description: "Reset settings to default values"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings reset

**Purpose:** For operators, developers, documents `opencenter settings reset`: what it does, its flags, and its examples.

Reset settings to default values

### Synopsis

Reset the CLI settings to default values.

This will overwrite the current settings file with default values.
All custom settings will be lost.

Default values:
  - logging.level: warn
  - logging.format: text
  - logging.output: stderr
  - paths.settingsDir: ~/.config/opencenter
  - paths.clustersDir: ~/.config/opencenter/clusters
  - paths.pluginsDir: ~/.config/opencenter/plugins
  - paths.stateDir: ~/.local/state/opencenter
  - behavior.autoConfirm: false
  - behavior.dryRun: false
  - cluster_defaults.provider: openstack
  - cluster_defaults.region: dfw3
  - cluster_defaults.environment: dev
  - cluster_defaults.gitops_auth_method: token

```
opencenter settings reset [flags]
```

### Options

```
  -h, --help   help for reset
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
