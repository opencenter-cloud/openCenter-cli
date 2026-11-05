---
id: "opencenter-settings-view"
title: "Opencenter_Settings_View"
sidebar_label: "Opencenter_Settings_View"
description: "Display the current CLI settings"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings view

**Purpose:** For operators, developers, documents `opencenter settings view`: what it does, its flags, and its examples.

Display the current CLI settings

### Synopsis

Display the current CLI settings in YAML format.

This shows the settings file content exactly as it would appear in an editor.
Use 'opencenter settings edit' to modify the settings.

```
opencenter settings view [flags]
```

### Options

```
  -h, --help   help for view
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
