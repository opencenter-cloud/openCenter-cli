---
id: "opencenter-settings-explain-cluster-defaults"
title: "Opencenter_Settings_Explain_Cluster Defaults"
sidebar_label: "Opencenter_Settings_Explain_Cluster Defaults"
description: "Show how cluster_defaults values are applied to new cluster configs"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter settings explain cluster-defaults

**Purpose:** For operators, developers, documents `opencenter settings explain cluster-defaults`: what it does, its flags, and its examples.

Show how cluster_defaults values are applied to new cluster configs

### Synopsis

Display each cluster_defaults field, its current value, and exactly where
it is injected into cluster configurations during "opencenter cluster init".

```
opencenter settings explain cluster-defaults [flags]
```

### Options

```
  -h, --help   help for cluster-defaults
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

* [opencenter settings explain](opencenter_settings_explain.md)	 - Explain how configuration values affect CLI behavior
