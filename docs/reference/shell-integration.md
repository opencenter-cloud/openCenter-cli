# openCenter Shell Integration

This directory contains shell integration scripts to display the current active cluster in your shell prompt and provide convenient aliases.

## Quick Setup

```bash
# Install shell integration
mise run install-shell-integration

# Follow the instructions for your shell
```

## Manual Setup

### Bash/Zsh

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
source ~/.config/openCenter/shell/shell-integration.sh

# Add to prompt (optional)
PS1="\$(opencenter_prompt)$PS1"        # Bash
PROMPT="\$(opencenter_prompt)$PROMPT"  # Zsh
```

### Fish

The installer automatically sets up Fish integration. For manual setup:

```fish
# Copy to Fish config directory
cp ~/.config/openCenter/shell/shell-integration.fish ~/.config/fish/conf.d/opencenter.fish

# Add to prompt function (in ~/.config/fish/functions/fish_prompt.fish)
echo -n (opencenter_prompt)
```

### Starship

Add to your `~/.config/starship.toml`:

```toml
[custom.opencenter]
command = "cat ~/.config/openCenter/.active 2>/dev/null || echo ''"
when = "test -f ~/.config/openCenter/.active"
format = "[$symbol$output]($style) "
symbol = "🚀 "
style = "bold blue"
```

## Available Functions

### Shell Functions

- `opencenter_active` - Get active cluster name
- `opencenter_prompt` - Get formatted prompt string `[cluster]`
- `opencenter_active_short` - Get short cluster name (without organization)
- `opencenter_update_env` - Update `$OPENCENTER_ACTIVE_CLUSTER` environment variable

### Aliases

- `oc-active` - Same as `opencenter_active`
- `oc-status` - Run `openCenter cluster status`
- `oc-select` - Run `openCenter cluster select`
- `oc-list` - Run `openCenter cluster list`

### Environment Variables

- `$OPENCENTER_ACTIVE_CLUSTER` - Automatically updated with current active cluster

## CLI Commands (Performance Optimized)

For maximum performance in shell prompts:

```bash
# Fast commands (bypass full config loading)
openCenter cluster active-fast                    # Get active cluster
openCenter cluster active-fast --short           # Get short name
openCenter cluster active-fast --prompt          # Get formatted for prompt

# Mise tasks
mise run active-fast        # Fast active cluster lookup
mise run active-prompt      # Fast prompt format
mise run active-short       # Fast short name
```

## Performance Comparison

| Method | Speed | Use Case |
|--------|-------|----------|
| `active-fast` | ~1ms | Shell prompts, scripts |
| `cluster current` | ~50ms | Interactive use |
| `cluster status` | ~100ms | Detailed information |

## Example Prompt Configurations

### Bash

```bash
# Simple
PS1="\$(opencenter_prompt)$PS1"

# With colors
PS1="\[\033[36m\]\$(opencenter_prompt)\[\033[0m\]$PS1"

# Using fast command
PS1="\$(openCenter cluster active-fast --prompt 2>/dev/null)$PS1"
```

### Zsh

```zsh
# Simple
PROMPT="\$(opencenter_prompt)$PROMPT"

# With colors
PROMPT="%F{cyan}\$(opencenter_prompt)%f$PROMPT"

# Using fast command
PROMPT="\$(openCenter cluster active-fast --prompt 2>/dev/null)$PROMPT"
```

### Fish

```fish
function fish_prompt
    # Your existing prompt
    echo -n (opencenter_prompt)
    # Rest of your prompt...
end
```

### Oh My Zsh Theme

Add to your custom theme or modify existing one:

```zsh
# Add this to your theme file
opencenter_prompt_info() {
    local cluster=$(openCenter cluster active-fast 2>/dev/null)
    if [[ -n "$cluster" ]]; then
        echo "%{$fg[cyan]%}[$cluster]%{$reset_color%} "
    fi
}

# Use in your PROMPT
PROMPT='$(opencenter_prompt_info)'$PROMPT
```

## Caching

The shell integration uses intelligent caching:

- Cache file: `~/.cache/openCenter/active_cluster`
- Updates only when `~/.config/openCenter/.active` changes
- Automatic cache invalidation
- No performance impact on repeated calls

## Troubleshooting

### Prompt Not Showing

1. Check if active cluster is set:
   ```bash
   openCenter cluster current
   ```

2. Test the function directly:
   ```bash
   opencenter_prompt
   ```

3. Verify integration is loaded:
   ```bash
   type opencenter_active
   ```

### Performance Issues

Use the fast commands for shell prompts:

```bash
# Instead of this (slow)
PS1="\$(openCenter cluster current 2>/dev/null | sed 's/^/[/' | sed 's/$/] /')$PS1"

# Use this (fast)
PS1="\$(openCenter cluster active-fast --prompt 2>/dev/null)$PS1"
```

### Fish Shell Issues

Ensure the integration file is in the right location:

```bash
ls ~/.config/fish/conf.d/opencenter.fish
```

## Advanced Usage

### Conditional Prompt

Only show cluster in specific directories:

```bash
opencenter_conditional_prompt() {
    if [[ "$PWD" == *"/k8s/"* ]] || [[ "$PWD" == *"/clusters/"* ]]; then
        opencenter_prompt
    fi
}

PS1="\$(opencenter_conditional_prompt)$PS1"
```

### Multiple Cluster Indicators

Show both active cluster and kubectl context:

```bash
k8s_prompt() {
    local oc_cluster=$(opencenter_active 2>/dev/null)
    local k8s_context=$(kubectl config current-context 2>/dev/null)
    
    if [[ -n "$oc_cluster" ]]; then
        echo -n "[oc:$oc_cluster]"
    fi
    if [[ -n "$k8s_context" ]]; then
        echo -n "[k8s:$k8s_context]"
    fi
}

PS1="\$(k8s_prompt)$PS1"
```

## Files

- `shell-integration.sh` - Bash/Zsh integration
- `shell-integration.fish` - Fish shell integration  
- `starship-opencenter.toml` - Starship configuration
- `README.md` - This documentation

## Installation

- `../install-shell-integration.sh` - Installation script (run with `mise run install-shell-integration`)