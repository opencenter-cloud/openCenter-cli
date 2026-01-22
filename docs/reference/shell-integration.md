# Shell Integration


## Table of Contents

- [Quick Start](#quick-start)
- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
- [Prompt Integration](#prompt-integration)
- [Environment Variables](#environment-variables)
- [Helper Functions](#helper-functions)
- [Troubleshooting](#troubleshooting)
- [Cluster Selection Modes](#cluster-selection-modes)
- [See Also](#see-also)
openCenter provides shell integration for session-scoped cluster selection, allowing multiple terminal sessions to work with different clusters independently.

## Quick Start

**1. Install shell integration:**
```bash
# Add to ~/.zshrc (or ~/.bashrc for bash)
eval "$(opencenter shell-init)"
```

**2. Enable prompt display (optional but recommended):**
```zsh
# For zsh - add to ~/.zshrc after the shell-init line
setopt PROMPT_SUBST
PROMPT='%F{cyan}$(opencenter_current_cluster_short)%f ${PROMPT}'
```

```bash
# For bash - add to ~/.bashrc after the shell-init line
PS1="\[\033[36m\]\$(opencenter_current_cluster_short)\[\033[0m\] $PS1"
```

**3. Reload your shell:**
```bash
source ~/.zshrc  # or source ~/.bashrc
```

**4. Use it:**
```bash
# Switch cluster in current terminal only
opencenter cluster select my-cluster

# Your prompt now shows the cluster name:
# my-cluster ~ $
```

## Overview

With shell integration enabled, `opencenter cluster select` switches clusters only in the current terminal session. Use `--persistent` to set a cluster selection that affects all terminals.

## Features

- **Session Isolation**: Each terminal has its own active cluster context
- **Visual Feedback**: Optional prompt integration shows the active cluster
- **Automatic Cleanup**: Session files are cleaned up when the shell exits
- **Backward Compatible**: Falls back to persistent selection if shell integration is not active

## Installation

The shell integration has two parts:

1. **Core integration** (required): Enables session-scoped cluster switching
2. **Prompt display** (optional): Shows active cluster in your prompt

Both must be configured separately in your shell RC file.

### Quick Install

```bash
# Install shell integration automatically
opencenter config ide --shell-integration
```

This will:
1. Detect your shell (bash, zsh, or fish)
2. Add the integration line to your shell RC file
3. Provide instructions for enabling prompt display (optional)

**Note:** This only installs the core integration. To show the cluster in your prompt, follow the [Prompt Integration](#prompt-integration) section below.

### Manual Install

Add this line to your shell configuration file:

**Bash** (`~/.bashrc`):
```bash
eval "$(opencenter shell-init)"
```

**Zsh** (`~/.zshrc`):
```bash
eval "$(opencenter shell-init)"
```

**Fish** (`~/.config/fish/config.fish`):
```fish
opencenter shell-init --shell fish | source
```

Then reload your shell:
```bash
source ~/.bashrc  # or ~/.zshrc
```

## Usage

### Basic Commands

```bash
# Switch to a cluster in the current session
opencenter cluster select prod-cluster

# Switch to a cluster with organization
opencenter cluster select myorg/prod-cluster

# Set persistent cluster (affects all terminals)
opencenter cluster select prod-cluster --persistent

# Export environment variables for current cluster
eval "$(opencenter cluster env)"

# Export environment for specific cluster
eval "$(opencenter cluster env prod-cluster)"

# Show current cluster and its source
opencenter cluster current
# Output: prod-cluster (session)

# Show only cluster name (for scripting)
opencenter cluster current --quiet
# Output: prod-cluster

# Clear persistent cluster selection
opencenter cluster select --clear-persistent
```

### Cluster Selection Precedence

The active cluster is determined by this precedence order:

1. **OPENCENTER_CLUSTER** environment variable (highest priority)
2. **Session file** (if shell integration is active)
3. **Persistent selection** (fallback)

### Example Workflow

```bash
# Terminal 1: Work on production cluster
opencenter cluster select prod-cluster
opencenter cluster status
# All commands operate on prod-cluster

# Terminal 2: Work on development cluster (independent)
opencenter cluster select dev-cluster
opencenter cluster validate
# All commands operate on dev-cluster

# Both terminals maintain their own cluster context
```

## Prompt Integration

The shell integration provides helper functions to display the active cluster in your prompt, but you need to enable this manually.

### Bash

The integration script provides helper functions. Add one of these to your `~/.bashrc` **after** the `eval "$(opencenter shell-init)"` line:

**Option 1: Simple prefix**
```bash
PS1="\[\033[36m\]\$(opencenter_current_cluster_short)\[\033[0m\] $PS1"
```

**Option 2: Bracketed format (only when cluster is set)**
```bash
opencenter_prompt() {
    local cluster=$(opencenter_current_cluster_short)
    if [[ -n "$cluster" ]]; then
        echo -e "\033[36m[$cluster]\033[0m "
    fi
}
PS1="\$(opencenter_prompt)$PS1"
```

**Complete example for ~/.bashrc:**
```bash
# openCenter shell integration
eval "$(opencenter shell-init)"

# Add cluster to prompt
PS1="\[\033[36m\]\$(opencenter_current_cluster_short)\[\033[0m\] $PS1"
```

After adding this, reload your shell:
```bash
source ~/.bashrc
```

### Zsh

The integration script includes helper functions but leaves prompt customization to you. Add one of these to your `~/.zshrc` **after** the `eval "$(opencenter shell-init)"` line:

**Option 1: Left prompt with simple prefix**
```zsh
# Enable prompt substitution (required for dynamic prompts)
setopt PROMPT_SUBST

# Add cluster name before existing prompt
PROMPT='%F{cyan}$(opencenter_current_cluster_short)%f ${PROMPT}'
```

**Option 2: Left prompt with brackets (only when cluster is set)**
```zsh
setopt PROMPT_SUBST

opencenter_prompt() {
    local cluster=$(opencenter_current_cluster_short)
    if [[ -n "$cluster" ]]; then
        echo "%F{cyan}[$cluster]%f "
    fi
}
PROMPT='$(opencenter_prompt)'"$PROMPT"
```

**Option 3: Right prompt**
```zsh
setopt PROMPT_SUBST

RPROMPT='%F{cyan}$(opencenter_current_cluster_short)%f'
```

**Complete example for ~/.zshrc:**
```zsh
# openCenter shell integration
eval "$(opencenter shell-init)"

# Enable prompt substitution
setopt PROMPT_SUBST

# Add cluster to prompt (choose one)
PROMPT='%F{cyan}$(opencenter_current_cluster_short)%f ${PROMPT}'
```

After adding this, reload your shell:
```bash
source ~/.zshrc
```

### Fish

Add to your `~/.config/fish/config.fish`:

```fish
# Option 1: Left prompt
function fish_prompt
    set -l cluster (opencenter_current_cluster_short)
    if test -n "$cluster"
        set_color cyan
        echo -n "[$cluster] "
        set_color normal
    end
    # Add your existing prompt here
    echo -n "> "
end

# Option 2: Right prompt
function fish_right_prompt
    set -l cluster (opencenter_current_cluster_short)
    if test -n "$cluster"
        set_color cyan
        echo -n "[$cluster]"
        set_color normal
    end
end
```

### Starship Integration

If you use [Starship](https://starship.rs/), add this to your `~/.config/starship.toml`:

```toml
[env_var.OPENCENTER_CLUSTER]
variable = "OPENCENTER_CLUSTER"
format = "[$env_value]($style) "
style = "cyan bold"
disabled = false
```

## Environment Variables

Use `opencenter cluster env` to export environment variables for a cluster:

```bash
# Export current cluster environment
eval "$(opencenter cluster env)"

# Export specific cluster environment
eval "$(opencenter cluster env my-cluster)"
```

This sets:
- **OPENCENTER_CLUSTER**: Current cluster name
- **KUBECONFIG**: Path to kubeconfig file
- **ANSIBLE_INVENTORY**: Path to Ansible inventory
- **PATH**: Includes cluster-specific binaries
- Cloud provider credentials (AWS, OpenStack)

Additional environment variables managed by shell integration:
- **OPENCENTER_SESSION_ID**: Unique session identifier
- **OPENCENTER_SESSION_FILE**: Path to session file storing cluster selection

## Helper Functions

The shell integration provides these helper functions:

- `opencenter_current_cluster()`: Get the current cluster name (full path with organization)
- `opencenter_current_cluster_short()`: Get the short cluster name (without organization prefix)

## Troubleshooting

### Prompt not showing cluster name

**Symptom:** The `opencenter cluster select` command works, but your prompt doesn't show the cluster name.

**Cause:** Prompt display is not enabled by default. The shell integration only provides helper functions.

**Solution for zsh:**

1. Verify shell integration is loaded:
```bash
type opencenter_current_cluster_short
```

Expected output: `opencenter_current_cluster_short is a shell function`

If you get "not found", add this to `~/.zshrc`:
```zsh
eval "$(opencenter shell-init)"
```

2. Add prompt customization to `~/.zshrc` **after** the shell-init line:
```zsh
setopt PROMPT_SUBST
PROMPT='%F{cyan}$(opencenter_current_cluster_short)%f ${PROMPT}'
```

3. Reload your shell:
```bash
source ~/.zshrc
```

4. Test it:
```bash
# Switch to a cluster
opencenter cluster select test-cluster

# Manually check the function
opencenter_current_cluster_short
# Should output: test-cluster

# Your prompt should now show: test-cluster
```

**Solution for bash:**

Add to `~/.bashrc` after the shell-init line:
```bash
PS1="\[\033[36m\]\$(opencenter_current_cluster_short)\[\033[0m\] $PS1"
```

Then reload:
```bash
source ~/.bashrc
```

### Function returns empty string

**Symptom:** `opencenter_current_cluster_short` returns nothing.

**Cause:** No cluster is currently selected.

**Solution:**
```bash
# Check current cluster
opencenter cluster current

# If no cluster is active, select one
opencenter cluster select my-cluster

# Now the function should return the cluster name
opencenter_current_cluster_short
```

### Shell integration not detected

If you see this warning:
```
⚠️  Shell integration not detected. Setting persistent cluster selection.
💡 To enable session-scoped selection, run: eval "$(opencenter shell-init)"
```

Make sure you've:
1. Added the integration line to your shell RC file
2. Reloaded your shell or started a new terminal session

### Session file not cleaned up

Session files are automatically cleaned up when the shell exits. If you find stale session files in `~/.config/openCenter/.session-*`, you can safely delete them.

## Cluster Selection Modes

| Mode | Command | Scope | Persistence |
|------|---------|-------|-------------|
| Session (default) | `cluster select <name>` | Current terminal only | Lost on shell exit |
| Persistent | `cluster select <name> --persistent` | All terminals | Survives shell restart |

## See Also

- [CLI Commands Reference](cli-commands.md)
- [Configuration](configuration.md)
- [IDE Integration](../how-to/ide-integration.md)
