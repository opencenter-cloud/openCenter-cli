---
id: "opencenter-settings-path"
title: "Opencenter_Settings_Path"
sidebar_label: "Opencenter_Settings_Path"
description: "Show the path to the settings file"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings path

**Purpose:** For operators, developers, documents `opencenter settings path`: what it does, its flags, and its examples.

Show the path to the settings file

### Synopsis

Show the absolute path to the CLI settings file.

The settings file location is determined by:
1. OPENCENTER_CONFIG_DIR environment variable (if set)
2. Default OS-specific config directory (~/.config/opencenter on Linux/macOS)

The settings file is named 'settings.yaml' within the configuration directory.

```
opencenter settings path [flags]
```

### Options

```
  -h, --help   help for path
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
