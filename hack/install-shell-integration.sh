#!/usr/bin/env bash
# openCenter Shell Integration Installer

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHELL_INTEGRATION_DIR="$SCRIPT_DIR/shell-integration"
OPENCENTER_CONFIG_DIR="${HOME}/.config/openCenter"
INTEGRATION_DIR="${OPENCENTER_CONFIG_DIR}/shell"

echo "Installing openCenter shell integration..."

# Create directories
mkdir -p "$INTEGRATION_DIR"
mkdir -p "${HOME}/.cache/openCenter"

# Copy integration files
cp "$SHELL_INTEGRATION_DIR/shell-integration.sh" "$INTEGRATION_DIR/"
cp "$SHELL_INTEGRATION_DIR/shell-integration.fish" "$INTEGRATION_DIR/"
cp "$SHELL_INTEGRATION_DIR/starship-opencenter.toml" "$INTEGRATION_DIR/"

# Make shell script executable
chmod +x "$INTEGRATION_DIR/shell-integration.sh"

echo "Files installed to: $INTEGRATION_DIR"
echo ""

# Detect shell and provide instructions
SHELL_NAME=$(basename "$SHELL")

case "$SHELL_NAME" in
    bash)
        echo "For Bash integration, add this to your ~/.bashrc:"
        echo "source $INTEGRATION_DIR/shell-integration.sh"
        echo ""
        echo "To add openCenter cluster to your prompt, also add:"
        echo "PS1=\"\\\$(opencenter_prompt)\$PS1\""
        ;;
    zsh)
        echo "For Zsh integration, add this to your ~/.zshrc:"
        echo "source $INTEGRATION_DIR/shell-integration.sh"
        echo ""
        echo "To add openCenter cluster to your prompt, also add:"
        echo "PROMPT=\"\\\$(opencenter_prompt)\$PROMPT\""
        ;;
    fish)
        FISH_CONFIG_DIR="${HOME}/.config/fish/conf.d"
        mkdir -p "$FISH_CONFIG_DIR"
        cp "$INTEGRATION_DIR/shell-integration.fish" "$FISH_CONFIG_DIR/opencenter.fish"
        echo "Fish integration installed to: $FISH_CONFIG_DIR/opencenter.fish"
        echo ""
        echo "To add openCenter cluster to your prompt, modify your fish_prompt function:"
        echo "echo -n (opencenter_prompt)"
        ;;
    *)
        echo "Shell '$SHELL_NAME' detected. Manual integration required."
        echo "Source the appropriate file from: $INTEGRATION_DIR"
        ;;
esac

echo ""
echo "Starship users:"
echo "Add the contents of $INTEGRATION_DIR/starship-opencenter.toml"
echo "to your ~/.config/starship.toml file"
echo ""

# Check if openCenter binary exists and suggest building
if ! command -v openCenter >/dev/null 2>&1; then
    echo "Note: openCenter binary not found in PATH."
    echo "Make sure to build and install it: mise run build"
fi

echo ""
echo "Installation complete! Restart your shell or source your config file to activate."
echo ""
echo "Documentation: $SHELL_INTEGRATION_DIR/README.md"