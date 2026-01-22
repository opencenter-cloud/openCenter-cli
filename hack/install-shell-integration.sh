#!/usr/bin/env bash
# openCenter Shell Integration Installer

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHELL_INTEGRATION_DIR="$SCRIPT_DIR/shell-integration"
OPENCENTER_CONFIG_DIR="${HOME}/.config/openCenter"
INTEGRATION_DIR="${OPENCENTER_CONFIG_DIR}/shell"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_header "openCenter Shell Integration Installer"
echo ""

# Verify source files exist
if [[ ! -d "$SHELL_INTEGRATION_DIR" ]]; then
    print_error "Shell integration directory not found: $SHELL_INTEGRATION_DIR"
    exit 1
fi

for file in shell-integration.sh shell-integration.fish starship-opencenter.toml; do
    if [[ ! -f "$SHELL_INTEGRATION_DIR/$file" ]]; then
        print_error "Required file not found: $SHELL_INTEGRATION_DIR/$file"
        exit 1
    fi
done

# Create directories
mkdir -p "$INTEGRATION_DIR"
mkdir -p "${HOME}/.cache/openCenter"

print_success "Created directories"

# Copy integration files
cp "$SHELL_INTEGRATION_DIR/shell-integration.sh" "$INTEGRATION_DIR/"
cp "$SHELL_INTEGRATION_DIR/shell-integration.fish" "$INTEGRATION_DIR/"
cp "$SHELL_INTEGRATION_DIR/starship-opencenter.toml" "$INTEGRATION_DIR/"

# Make shell script executable
chmod +x "$INTEGRATION_DIR/shell-integration.sh"

print_success "Installed files to: $INTEGRATION_DIR"
echo ""

# Detect shell and provide instructions
SHELL_NAME=$(basename "$SHELL")

print_header "Shell Integration Instructions"
echo ""

case "$SHELL_NAME" in
    bash)
        echo "Detected: Bash"
        echo ""
        echo "Add this line to your ~/.bashrc:"
        echo ""
        echo -e "${BLUE}source $INTEGRATION_DIR/shell-integration.sh${NC}"
        echo ""
        echo "To add openCenter cluster to your prompt:"
        echo ""
        echo -e "${BLUE}PS1=\"\\\$(opencenter_prompt)\$PS1\"${NC}"
        echo ""
        
        # Offer to auto-install
        if [[ -f "${HOME}/.bashrc" ]]; then
            read -p "Add to ~/.bashrc automatically? (y/N) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                if ! grep -q "shell-integration.sh" "${HOME}/.bashrc"; then
                    echo "" >> "${HOME}/.bashrc"
                    echo "# openCenter shell integration" >> "${HOME}/.bashrc"
                    echo "source $INTEGRATION_DIR/shell-integration.sh" >> "${HOME}/.bashrc"
                    print_success "Added to ~/.bashrc"
                else
                    print_warning "Already present in ~/.bashrc"
                fi
            fi
        fi
        ;;
    zsh)
        echo "Detected: Zsh"
        echo ""
        echo "Add this line to your ~/.zshrc:"
        echo ""
        echo -e "${BLUE}source $INTEGRATION_DIR/shell-integration.sh${NC}"
        echo ""
        echo "To add openCenter cluster to your prompt:"
        echo ""
        echo -e "${BLUE}PROMPT=\"\\\$(opencenter_prompt)\$PROMPT\"${NC}"
        echo ""
        
        # Offer to auto-install
        if [[ -f "${HOME}/.zshrc" ]]; then
            read -p "Add to ~/.zshrc automatically? (y/N) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                if ! grep -q "shell-integration.sh" "${HOME}/.zshrc"; then
                    echo "" >> "${HOME}/.zshrc"
                    echo "# openCenter shell integration" >> "${HOME}/.zshrc"
                    echo "source $INTEGRATION_DIR/shell-integration.sh" >> "${HOME}/.zshrc"
                    print_success "Added to ~/.zshrc"
                else
                    print_warning "Already present in ~/.zshrc"
                fi
            fi
        fi
        ;;
    fish)
        echo "Detected: Fish"
        echo ""
        FISH_CONFIG_DIR="${HOME}/.config/fish/conf.d"
        mkdir -p "$FISH_CONFIG_DIR"
        
        if [[ ! -f "$FISH_CONFIG_DIR/opencenter.fish" ]]; then
            cp "$INTEGRATION_DIR/shell-integration.fish" "$FISH_CONFIG_DIR/opencenter.fish"
            print_success "Installed to: $FISH_CONFIG_DIR/opencenter.fish"
        else
            print_warning "Already installed: $FISH_CONFIG_DIR/opencenter.fish"
        fi
        
        echo ""
        echo "To add openCenter cluster to your prompt, modify your fish_prompt function:"
        echo ""
        echo -e "${BLUE}echo -n (opencenter_prompt)${NC}"
        ;;
    *)
        print_warning "Shell '$SHELL_NAME' detected - manual integration required"
        echo ""
        echo "Source the appropriate file from: $INTEGRATION_DIR"
        ;;
esac

echo ""
print_header "Starship Integration (Optional)"
echo ""

if command -v starship >/dev/null 2>&1; then
    print_success "Starship detected"
    echo ""
    echo "Add the contents of:"
    echo -e "${BLUE}$INTEGRATION_DIR/starship-opencenter.toml${NC}"
    echo ""
    echo "To your starship config:"
    echo -e "${BLUE}~/.config/starship.toml${NC}"
    echo ""
    
    if [[ -f "${HOME}/.config/starship.toml" ]]; then
        read -p "View starship integration config? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            cat "$INTEGRATION_DIR/starship-opencenter.toml"
        fi
    fi
else
    echo "Starship not detected (optional)"
    echo "If you use Starship, see: $INTEGRATION_DIR/starship-opencenter.toml"
fi

echo ""
print_header "Verification"
echo ""

# Check if openCenter binary exists
if command -v openCenter >/dev/null 2>&1; then
    OPENCENTER_VERSION=$(openCenter version 2>/dev/null | head -1 || echo "unknown")
    print_success "openCenter binary found: $OPENCENTER_VERSION"
else
    print_warning "openCenter binary not found in PATH"
    echo "  Build and install: mise run build"
    echo "  Then add bin/openCenter to your PATH or copy to /usr/local/bin"
fi

# Check for active cluster
if [[ -f "${HOME}/.config/openCenter/.active" ]]; then
    ACTIVE_CLUSTER=$(cat "${HOME}/.config/openCenter/.active" 2>/dev/null || echo "")
    if [[ -n "$ACTIVE_CLUSTER" ]]; then
        print_success "Active cluster: $ACTIVE_CLUSTER"
    fi
else
    echo "  No active cluster set (use: openCenter cluster select)"
fi

echo ""
print_header "Next Steps"
echo ""
echo "1. Restart your shell or run:"
echo -e "   ${BLUE}source ~/.${SHELL_NAME}rc${NC}"
echo ""
echo "2. Test the integration:"
echo -e "   ${BLUE}opencenter_active${NC}"
echo -e "   ${BLUE}oc-status${NC}"
echo ""
echo "3. Available functions:"
echo "   - opencenter_active       Get active cluster name"
echo "   - opencenter_prompt       Get formatted prompt string"
echo "   - opencenter_active_short Get short cluster name"
echo "   - oc-active, oc-status, oc-select, oc-list (aliases)"
echo ""
echo "4. Environment variable:"
echo "   - \$OPENCENTER_ACTIVE_CLUSTER"
echo ""

print_success "Installation complete!"