---
id: "opencenter-settings-edit"
title: "Opencenter_Settings_Edit"
sidebar_label: "Opencenter_Settings_Edit"
description: "Edit the CLI settings file in your default editor"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings edit

**Purpose:** For operators, developers, documents `opencenter settings edit`: what it does, its flags, and its examples.

Edit the CLI settings file in your default editor

### Synopsis

Edit the CLI settings file in your default editor.

This command opens the settings file in the editor specified by the EDITOR
environment variable. If EDITOR is not set, it falls back to common editors
in the following order: vim, vi, nano.

After editing, the settings will be validated. If validation fails, you'll
be notified of the errors but the file will remain saved.

Examples:
  opencenter settings edit
  EDITOR=nano opencenter settings edit

```
opencenter settings edit [flags]
```

### Options

```
  -h, --help   help for edit
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
