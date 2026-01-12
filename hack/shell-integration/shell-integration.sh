#!/usr/bin/env bash
# openCenter Shell Integration
# Source this file in your .bashrc, .zshrc, or .profile for enhanced shell integration

# Cache file for active cluster to avoid repeated file reads
OPENCENTER_CACHE_FILE="${HOME}/.cache/openCenter/active_cluster"
OPENCENTER_ACTIVE_FILE="${HOME}/.config/openCenter/.active"

# Ensure cache directory exists
mkdir -p "$(dirname "$OPENCENTER_CACHE_FILE")"

# Function to get active cluster (cached)
opencenter_active() {
    # Check if active file exists and is newer than cache
    if [[ -f "$OPENCENTER_ACTIVE_FILE" ]]; then
        if [[ ! -f "$OPENCENTER_CACHE_FILE" ]] || [[ "$OPENCENTER_ACTIVE_FILE" -nt "$OPENCENTER_CACHE_FILE" ]]; then
            # Update cache
            cat "$OPENCENTER_ACTIVE_FILE" 2>/dev/null > "$OPENCENTER_CACHE_FILE"
        fi
        cat "$OPENCENTER_CACHE_FILE" 2>/dev/null | tr -d '\n'
    else
        # No active cluster, clear cache
        rm -f "$OPENCENTER_CACHE_FILE" 2>/dev/null
        return 1
    fi
}

# Function to get active cluster for prompt (with formatting)
opencenter_prompt() {
    local cluster
    cluster=$(opencenter_active 2>/dev/null)
    if [[ -n "$cluster" ]]; then
        echo "[$cluster]"
    fi
}

# Function to get active cluster short name (just cluster, not org/cluster)
opencenter_active_short() {
    local cluster
    cluster=$(opencenter_active 2>/dev/null)
    if [[ -n "$cluster" ]]; then
        echo "${cluster##*/}"  # Remove everything before last /
    fi
}

# Convenient aliases
alias oc-active='opencenter_active'
alias oc-status='openCenter cluster status'
alias oc-select='openCenter cluster select'
alias oc-list='openCenter cluster list'

# Environment variable for current active cluster (updated on each prompt)
export OPENCENTER_ACTIVE_CLUSTER=""

# Function to update environment variable (call this in PROMPT_COMMAND or precmd)
opencenter_update_env() {
    OPENCENTER_ACTIVE_CLUSTER=$(opencenter_active 2>/dev/null || echo "")
}

# Auto-completion for cluster names (if openCenter supports it)
if command -v openCenter >/dev/null 2>&1; then
    # Enable completion if available
    if openCenter completion bash >/dev/null 2>&1; then
        source <(openCenter completion bash)
    elif openCenter completion zsh >/dev/null 2>&1; then
        source <(openCenter completion zsh)
    fi
fi

# Shell-specific integration
if [[ -n "$BASH_VERSION" ]]; then
    # Bash integration
    
    # Add to PROMPT_COMMAND for automatic updates
    if [[ "$PROMPT_COMMAND" != *"opencenter_update_env"* ]]; then
        PROMPT_COMMAND="opencenter_update_env; $PROMPT_COMMAND"
    fi
    
    # Example PS1 integration (uncomment to use)
    # PS1="\$(opencenter_prompt)$PS1"
    
elif [[ -n "$ZSH_VERSION" ]]; then
    # Zsh integration
    
    # Add to precmd for automatic updates
    autoload -U add-zsh-hook
    add-zsh-hook precmd opencenter_update_env
    
    # Example PROMPT integration (uncomment to use)
    # PROMPT="\$(opencenter_prompt)$PROMPT"
    
fi

# Fish shell integration (separate file needed)
if [[ "$SHELL" == *"fish"* ]]; then
    echo "For Fish shell integration, source the fish integration file:"
    echo "source ~/.config/openCenter/shell-integration.fish"
fi

echo "openCenter shell integration loaded!"
echo "Available functions:"
echo "  opencenter_active      - Get active cluster name"
echo "  opencenter_prompt      - Get formatted prompt string"
echo "  opencenter_active_short - Get short cluster name"
echo "  oc-active, oc-status, oc-select, oc-list - Convenient aliases"
echo ""
echo "Environment variable: \$OPENCENTER_ACTIVE_CLUSTER"
echo ""
echo "To add to your prompt, add one of these to your shell config:"
echo "  Bash: PS1=\"\\\$(opencenter_prompt)\$PS1\""
echo "  Zsh:  PROMPT=\"\\\$(opencenter_prompt)\$PROMPT\""