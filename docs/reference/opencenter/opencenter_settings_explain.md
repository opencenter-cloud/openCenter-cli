---
id: "opencenter-settings-explain"
title: "Opencenter_Settings_Explain"
sidebar_label: "Opencenter_Settings_Explain"
description: "Explain how configuration values affect CLI behavior"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings explain

**Purpose:** For operators, developers, documents `opencenter settings explain`: what it does, its flags, and its examples.

Explain how configuration values affect CLI behavior

```
opencenter settings explain [flags]
```

### Options

```
  -h, --help   help for explain
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
* [opencenter settings explain cluster-defaults](opencenter_settings_explain_cluster-defaults.md)	 - Show how cluster_defaults values are applied to new cluster configs
