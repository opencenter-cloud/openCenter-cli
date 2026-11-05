---
id: "opencenter-settings"
title: "Opencenter_Settings"
sidebar_label: "Opencenter_Settings"
description: "Manage CLI settings"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings

**Purpose:** For operators, developers, documents `opencenter settings`: what it does, its flags, and its examples.

Manage CLI settings

### Synopsis

Manage CLI settings including logging, paths, behavior, and cluster defaults.

The settings file is stored at ~/.config/opencenter/settings.yaml by default,
or at the location specified by the OPENCENTER_CONFIG_DIR environment variable.

Settings values can be accessed and modified using dot notation (e.g., logging.level).
The cluster_defaults section controls values injected into new cluster configs during
"opencenter cluster init".

```
opencenter settings [flags]
```

### Options

```
  -h, --help   help for settings
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
* [opencenter settings edit](opencenter_settings_edit.md)	 - Edit the CLI settings file in your default editor
* [opencenter settings explain](opencenter_settings_explain.md)	 - Explain how configuration values affect CLI behavior
* [opencenter settings get](opencenter_settings_get.md)	 - Get a settings value using dot notation
* [opencenter settings ide](opencenter_settings_ide.md)	 - Generate v2 schema and editor setup for cluster configuration files
* [opencenter settings path](opencenter_settings_path.md)	 - Show the path to the settings file
* [opencenter settings reset](opencenter_settings_reset.md)	 - Reset settings to default values
* [opencenter settings set](opencenter_settings_set.md)	 - Set a settings value using dot notation
* [opencenter settings view](opencenter_settings_view.md)	 - Display the current CLI settings
