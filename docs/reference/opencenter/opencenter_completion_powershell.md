---
id: "opencenter-completion-powershell"
title: "Opencenter_Completion_Powershell"
sidebar_label: "Opencenter_Completion_Powershell"
description: "Generate the autocompletion script for powershell"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter completion powershell

**Purpose:** For operators, developers, documents `opencenter completion powershell`: what it does, its flags, and its examples.

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	opencenter completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
opencenter completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
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

* [opencenter completion](opencenter_completion.md)	 - Generate the autocompletion script for the specified shell
