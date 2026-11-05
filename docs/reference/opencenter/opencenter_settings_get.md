---
id: "opencenter-settings-get"
title: "Opencenter_Settings_Get"
sidebar_label: "Opencenter_Settings_Get"
description: "Get a settings value using dot notation"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings get

**Purpose:** For operators, developers, documents `opencenter settings get`: what it does, its flags, and its examples.

Get a settings value using dot notation

### Synopsis

Get a settings value using dot notation.

Examples:
  opencenter settings get logging.level
  opencenter settings get paths.clustersDir
  opencenter settings get paths.pluginsDir
  opencenter settings get paths.stateDir
  opencenter settings get behavior.autoConfirm

Use dot notation to access nested settings values. If the key doesn't exist,
an error will be returned.

```
opencenter settings get <key> [flags]
```

### Options

```
  -h, --help   help for get
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
