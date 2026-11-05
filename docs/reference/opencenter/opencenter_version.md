---
id: "opencenter-version"
title: "Opencenter_Version"
sidebar_label: "Opencenter_Version"
description: "Display version and build information"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter version

**Purpose:** For operators, developers, documents `opencenter version`: what it does, its flags, and its examples.

Display version and build information

### Synopsis

Display version and build information for opencenter.

Shows the version, git commit, branch, tag (if applicable), build date,
Go version, and platform information.

```
opencenter version [flags]
```

### Examples

```
  # Show full version information
  opencenter version

  # Show short version only
  opencenter version --short
```

### Options

```
  -h, --help    help for version
      --short   Display short version only
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
