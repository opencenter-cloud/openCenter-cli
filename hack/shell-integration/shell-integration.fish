# openCenter Fish Shell Integration
# Save this as ~/.config/fish/conf.d/opencenter.fish or source it in your config.fish

set -g OPENCENTER_CACHE_FILE "$HOME/.cache/openCenter/active_cluster"
set -g OPENCENTER_ACTIVE_FILE "$HOME/.config/openCenter/.active"

# Ensure cache directory exists
mkdir -p (dirname $OPENCENTER_CACHE_FILE)

# Function to get active cluster (cached)
function opencenter_active
    # Check if active file exists and is newer than cache
    if test -f $OPENCENTER_ACTIVE_FILE
        if not test -f $OPENCENTER_CACHE_FILE; or test $OPENCENTER_ACTIVE_FILE -nt $OPENCENTER_CACHE_FILE
            # Update cache
            cat $OPENCENTER_ACTIVE_FILE 2>/dev/null > $OPENCENTER_CACHE_FILE
        end
        cat $OPENCENTER_CACHE_FILE 2>/dev/null | tr -d '\n'
    else
        # No active cluster, clear cache
        rm -f $OPENCENTER_CACHE_FILE 2>/dev/null
        return 1
    end
end

# Function to get active cluster for prompt (with formatting)
function opencenter_prompt
    set cluster (opencenter_active 2>/dev/null)
    if test -n "$cluster"
        echo "[$cluster]"
    end
end

# Function to get active cluster short name
function opencenter_active_short
    set cluster (opencenter_active 2>/dev/null)
    if test -n "$cluster"
        echo (string split -r -m1 / $cluster)[-1]
    end
end

# Convenient aliases
alias oc-active='opencenter_active'
alias oc-status='openCenter cluster status'
alias oc-select='openCenter cluster select'
alias oc-list='openCenter cluster list'

# Environment variable for current active cluster
set -gx OPENCENTER_ACTIVE_CLUSTER ""

# Function to update environment variable
function opencenter_update_env
    set -gx OPENCENTER_ACTIVE_CLUSTER (opencenter_active 2>/dev/null; or echo "")
end

# Auto-update on each prompt
function __opencenter_prompt_update --on-event fish_prompt
    opencenter_update_env
end

# Enable completion if available
if command -v openCenter >/dev/null 2>&1
    if openCenter completion fish >/dev/null 2>&1
        openCenter completion fish | source
    end
end

echo "openCenter Fish shell integration loaded!"
echo "Available functions:"
echo "  opencenter_active      - Get active cluster name"
echo "  opencenter_prompt      - Get formatted prompt string" 
echo "  opencenter_active_short - Get short cluster name"
echo "  oc-active, oc-status, oc-select, oc-list - Convenient aliases"
echo ""
echo "Environment variable: \$OPENCENTER_ACTIVE_CLUSTER"
echo ""
echo "To add to your prompt, add this to your fish_prompt function:"
echo "  echo -n (opencenter_prompt)"