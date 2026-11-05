---
id: "opencenter-settings-ide"
title: "Opencenter_Settings_Ide"
sidebar_label: "Opencenter_Settings_Ide"
description: "Generate v2 schema and editor setup for cluster configuration files"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings ide

**Purpose:** For operators, developers, documents `opencenter settings ide`: what it does, its flags, and its examples.

Generate v2 schema and editor setup for cluster configuration files

### Synopsis

Generate the current v2 JSON Schema and print setup instructions for IDEs
and YAML Language Server clients.

The generated schema is an editor aid for autocomplete, hover, and early shape
validation. Runtime validation remains owned by "opencenter cluster validate".

```
opencenter settings ide [flags]
```

### Examples

```
  # Generate schema and print generic YAML Language Server instructions
  opencenter settings ide

  # Generate schema and merge VS Code workspace settings
  opencenter settings ide --ide vscode --write

  # Print schema to stdout for external tooling
  opencenter settings ide --print

  # CI check for checked-in schema drift
  opencenter settings ide --check
```

### Options

```
      --check                fail if the schema file is missing or stale
  -h, --help                 help for ide
      --ide string           target IDE (auto, vscode, jetbrains, yaml-language-server, none) (default "auto")
      --print                print schema to stdout and write no files
      --schema-only          write schema and skip editor instructions
      --schema-path string   path to write or check the generated v2 JSON Schema (default "schema/opencenter-v2.schema.json")
      --write                write supported editor configuration files
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
