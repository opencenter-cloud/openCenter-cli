---
id: "opencenter-completion-bash"
title: "Opencenter_Completion_Bash"
sidebar_label: "Opencenter_Completion_Bash"
description: "Generate the autocompletion script for bash"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter completion bash

**Purpose:** For operators, developers, documents `opencenter completion bash`: what it does, its flags, and its examples.

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(opencenter completion bash)

To load completions for every new session, execute once:

#### Linux:

	opencenter completion bash > /etc/bash_completion.d/opencenter

#### macOS:

	opencenter completion bash > $(brew --prefix)/etc/bash_completion.d/opencenter

You will need to start a new shell for this setup to take effect.


```
opencenter completion bash
```

### Options

```
  -h, --help              help for bash
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
