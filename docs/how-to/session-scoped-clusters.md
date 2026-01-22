# Session-Scoped Cluster Selection


## Table of Contents

- [Overview](#overview)
- [Problem](#problem)
- [Solution](#solution)
- [Quick Start](#quick-start)
- [Use Cases](#use-cases)
- [How It Works](#how-it-works)
- [Advanced Usage](#advanced-usage)
- [Optional: Prompt Integration](#optional-prompt-integration)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Command Reference](#command-reference)
- [Environment Variables](#environment-variables)
- [See Also](#see-also)
This guide shows you how to work with multiple clusters simultaneously using session-scoped cluster selection.

## Overview

By default, `opencenter cluster select` sets a **persistent** cluster selection that affects all terminal sessions. With shell integration enabled, cluster selection becomes **session-scoped**, allowing each terminal to maintain its own independent cluster context.

## Problem

When working with multiple clusters, persistent cluster selection can be problematic when:

- Multiple engineers work on different clusters on the same server
- You need to run commands on multiple clusters simultaneously
- You want to test changes on one cluster while monitoring another
- Switching clusters in one terminal affects all other terminals

## Solution

Enable shell integration to use session-scoped cluster selection. Each terminal maintains its own cluster context without affecting other terminals.

## Quick Start

### 1. Install Shell Integration

**Automatic installation:**
```bash
opencenter config ide --shell-integration
```

This will:
- Detect your shell (bash, zsh, or fish)
- Add the integration line to your shell RC file
- Provide instructions for optional prompt integration

**Manual installation:**

Add to your shell configuration file:

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

### 2. Reload Your Shell

```bash
source ~/.bashrc  # or ~/.zshrc for zsh
# For fish: source ~/.config/fish/config.fish
```

### 3. Verify Installation

```bash
# Check if shell integration is active
echo $OPENCENTER_SESSION_FILE
# Should output: /home/user/.config/openCenter/.session-XXXXX

# Check helper functions are available
type opencenter_current_cluster_short
# Should output: opencenter_current_cluster_short is a shell function
```

### 4. Use Session-Scoped Selection

```bash
# Terminal 1
opencenter cluster select prod-cluster
opencenter cluster status

# Terminal 2 (independent)
opencenter cluster select dev-cluster
opencenter cluster validate
```

## Use Cases

### Multiple Engineers on Same Server

**Engineer 1** (working on production):
```bash
opencenter cluster select prod-cluster
opencenter cluster status
opencenter cluster drift
```

**Engineer 2** (working on staging):
```bash
opencenter cluster select staging-cluster
opencenter cluster validate
opencenter cluster bootstrap
```

Both engineers can work independently without interfering with each other's cluster context.

### Parallel Operations

Monitor multiple clusters simultaneously:

```bash
# Terminal 1: Monitor production
opencenter cluster select prod-cluster
watch opencenter cluster status

# Terminal 2: Monitor staging
opencenter cluster select staging-cluster
watch opencenter cluster status

# Terminal 3: Work on development
opencenter cluster select dev-cluster
opencenter cluster update
```

### Testing and Validation

Test changes on one cluster while keeping another as reference:

```bash
# Terminal 1: Test cluster
opencenter cluster select test-cluster
opencenter cluster validate
opencenter cluster bootstrap

# Terminal 2: Reference cluster
opencenter cluster select prod-cluster
opencenter cluster info  # Compare configuration
```

## How It Works

### Session Isolation

When shell integration is enabled:

1. Each terminal gets a unique **session ID** (`OPENCENTER_SESSION_ID`)
2. A **session file** is created at `~/.config/openCenter/.session-<ID>`
3. `opencenter cluster select` writes the cluster name to this session file
4. The session file is automatically cleaned up when the shell exits

### Cluster Selection Precedence

The active cluster is determined by this order (highest to lowest priority):

1. **OPENCENTER_CLUSTER** environment variable (temporary override)
2. **Session file** (if shell integration is active)
3. **Persistent selection** (fallback when no session is active)

### Helper Functions

Shell integration provides these functions:

- `opencenter_current_cluster()`: Returns full cluster name (e.g., `myorg/prod-cluster`)
- `opencenter_current_cluster_short()`: Returns short name (e.g., `prod-cluster`)

## Advanced Usage

### Temporary Environment Variable Override

Override cluster selection for specific commands without changing the session:

```bash
# One-off command on different cluster
OPENCENTER_CLUSTER=prod-cluster opencenter cluster status

# Multiple commands with temporary override
export OPENCENTER_CLUSTER=prod-cluster
opencenter cluster status
opencenter cluster info
unset OPENCENTER_CLUSTER
```

### Persistent Selection

Set a cluster that affects all terminals (bypasses session isolation):

```bash
# Set persistent cluster (all terminals)
opencenter cluster select prod-cluster --persistent

# Clear persistent cluster
opencenter cluster select --clear-persistent
```

### Scripting with Session Selection

```bash
#!/bin/bash
# Script that works with multiple clusters

# Enable shell integration in script
eval "$(opencenter shell-init)"

# Work with cluster 1
opencenter cluster select cluster1
echo "Validating cluster1..."
opencenter cluster validate

# Work with cluster 2
opencenter cluster select cluster2
echo "Validating cluster2..."
opencenter cluster validate
```

### Check Current Cluster and Source

```bash
# Show cluster with source information
opencenter cluster current
# Output: prod-cluster (session)
# or: prod-cluster (environment)
# or: prod-cluster (persistent)

# Just the cluster name (for scripting)
CLUSTER=$(opencenter cluster current --quiet)
echo "Working on: $CLUSTER"
```

### Activate Cluster Environment

Automatically set environment variables (KUBECONFIG, credentials, PATH):

```bash
# Activate cluster environment with credentials
eval "$(opencenter cluster select prod-cluster --activate --export-only)"

# Deactivate cluster environment
eval "$(opencenter cluster select --clear --export-only)"
```

## Optional: Prompt Integration

Shell integration provides helper functions but doesn't automatically modify your prompt. To show the active cluster in your prompt, add one of these to your shell RC file **after** the shell-init line:

### Bash

Add to `~/.bashrc` after `eval "$(opencenter shell-init)"`:

```bash
# Simple prefix
PS1="\[\033[36m\]\$(opencenter_current_cluster_short)\[\033[0m\] $PS1"

# Or bracketed format
PS1="\[\033[36m\][\$(opencenter_current_cluster_short)]\[\033[0m\] $PS1"
```

### Zsh

Add to `~/.zshrc` after `eval "$(opencenter shell-init)"`:

```zsh
# Enable prompt substitution (required)
setopt PROMPT_SUBST

# Simple prefix
PROMPT='%F{cyan}$(opencenter_current_cluster_short)%f ${PROMPT}'

# Or right prompt
RPROMPT='%F{cyan}[$(opencenter_current_cluster_short)]%f'
```

### Fish

Add to `~/.config/fish/config.fish`:

```fish
# Left prompt
function fish_prompt
    set -l cluster (opencenter_current_cluster_short)
    if test -n "$cluster"
        set_color cyan
        echo -n "[$cluster] "
        set_color normal
    end
    echo -n "> "
end

# Or right prompt
function fish_right_prompt
    set -l cluster (opencenter_current_cluster_short)
    if test -n "$cluster"
        set_color cyan
        echo -n "[$cluster]"
        set_color normal
    end
end
```

## Best Practices

1. **Use session-scoped selection for temporary work**: Switch clusters in specific terminals without affecting others
2. **Use `--persistent` for default cluster**: Set your most-used cluster with `opencenter cluster select <cluster> --persistent`
3. **Enable prompt integration**: Always know which cluster you're working on at a glance
4. **Use `--activate` for full environment**: Set credentials and paths with `eval "$(opencenter cluster select <cluster> --activate --export-only)"`
5. **Document your workflow**: Add comments in scripts that use session-scoped selection

## Troubleshooting

### Shell integration not detected

**Symptom:** Warning message when running `opencenter cluster select`:
```
⚠️  Shell integration not detected. Setting persistent cluster selection.
💡 To enable session-scoped selection, run: eval "$(opencenter shell-init)"
```

**Solution:**
1. Add shell integration to your RC file:
   ```bash
   echo 'eval "$(opencenter shell-init)"' >> ~/.zshrc  # or ~/.bashrc
   ```
2. Reload your shell:
   ```bash
   source ~/.zshrc  # or source ~/.bashrc
   ```
3. Verify it's active:
   ```bash
   echo $OPENCENTER_SESSION_FILE
   # Should output: /home/user/.config/openCenter/.session-XXXXX
   ```

### Commands still use wrong cluster

**Symptom:** Cluster selection doesn't seem to work or uses unexpected cluster.

**Solution:**
1. Check cluster selection precedence:
   ```bash
   # Check environment variable (highest priority)
   echo $OPENCENTER_CLUSTER
   
   # Check session file
   echo $OPENCENTER_SESSION_FILE
   cat $OPENCENTER_SESSION_FILE
   
   # Check persistent selection (fallback)
   opencenter cluster current
   ```

2. Clear any overrides:
   ```bash
   unset OPENCENTER_CLUSTER
   opencenter cluster select <desired-cluster>
   ```

### Session not isolated

**Symptom:** Changing cluster in one terminal affects other terminals.

**Solution:**
Verify that each terminal has a unique session ID:
```bash
echo $OPENCENTER_SESSION_ID
# Should be unique per terminal (e.g., 12345-1234567890)
```

If session IDs are the same, shell integration may not be properly loaded. Restart your terminals after adding shell integration.

### Prompt not showing cluster name

**Symptom:** Shell integration works but prompt doesn't show cluster.

**Cause:** Prompt integration is optional and must be configured separately.

**Solution:**
1. Verify helper function exists:
   ```bash
   type opencenter_current_cluster_short
   # Should output: opencenter_current_cluster_short is a shell function
   ```

2. Add prompt integration to your RC file (see [Optional: Prompt Integration](#optional-prompt-integration) section above)

3. Reload your shell:
   ```bash
   source ~/.zshrc  # or ~/.bashrc
   ```

### Session files not cleaned up

**Symptom:** Stale session files in `~/.config/openCenter/.session-*`

**Cause:** Shell exited abnormally without running cleanup.

**Solution:**
Session files are automatically cleaned up on normal shell exit. Stale files can be safely deleted:
```bash
rm -f ~/.config/openCenter/.session-*
```

## Command Reference

| Command | Description | Scope |
|---------|-------------|-------|
| `opencenter cluster select <cluster>` | Select cluster (session-scoped with shell integration) | Current terminal |
| `opencenter cluster select <cluster> --persistent` | Select cluster for all terminals | All terminals |
| `opencenter cluster select --clear` | Clear session cluster | Current terminal |
| `opencenter cluster select --clear-persistent` | Clear persistent cluster | All terminals |
| `opencenter cluster current` | Show current cluster and source | - |
| `opencenter cluster current --quiet` | Show only cluster name | - |
| `eval "$(opencenter cluster select <cluster> --activate --export-only)"` | Activate cluster environment | Current terminal |
| `eval "$(opencenter cluster select --clear --export-only)"` | Deactivate cluster environment | Current terminal |

## Environment Variables

| Variable | Description | Set By |
|----------|-------------|--------|
| `OPENCENTER_SESSION_ID` | Unique session identifier | Shell integration |
| `OPENCENTER_SESSION_FILE` | Path to session file | Shell integration |
| `OPENCENTER_CLUSTER` | Current cluster name (temporary override) | User or `cluster select` |

## See Also

- [Shell Integration Reference](../reference/shell-integration.md) - Complete shell integration documentation
- [CLI Commands](../reference/cli-commands.md) - All CLI commands
- [Multi-Cluster Management](multi-cluster.md) - Managing multiple clusters
