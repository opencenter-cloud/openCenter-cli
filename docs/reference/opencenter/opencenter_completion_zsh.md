---
id: "opencenter-completion-zsh"
title: "Opencenter_Completion_Zsh"
sidebar_label: "Opencenter_Completion_Zsh"
description: "Generate the autocompletion script for zsh"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter completion zsh

**Purpose:** For operators, developers, documents `opencenter completion zsh`: what it does, its flags, and its examples.

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(opencenter completion zsh)

To load completions for every new session, execute once:

#### Linux:

	opencenter completion zsh > "${fpath[1]}/_opencenter"

#### macOS:

	opencenter completion zsh > $(brew --prefix)/share/zsh/site-functions/_opencenter

You will need to start a new shell for this setup to take effect.


```
opencenter completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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
