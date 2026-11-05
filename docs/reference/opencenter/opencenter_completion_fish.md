---
id: "opencenter-completion-fish"
title: "Opencenter_Completion_Fish"
sidebar_label: "Opencenter_Completion_Fish"
description: "Generate the autocompletion script for fish"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter completion fish

**Purpose:** For operators, developers, documents `opencenter completion fish`: what it does, its flags, and its examples.

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	opencenter completion fish | source

To load completions for every new session, execute once:

	opencenter completion fish > ~/.config/fish/completions/opencenter.fish

You will need to start a new shell for this setup to take effect.


```
opencenter completion fish [flags]
```

### Options

```
  -h, --help              help for fish
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
