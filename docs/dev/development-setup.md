---
doc_type: tutorial
title: "Development Environment Setup"
audience: "developers"
---

# Development Environment Setup

**Purpose:** For developers, shows how to set up a complete development environment for openCenter-cli from scratch.

## What You'll Build

By the end of this tutorial, you'll have:
- Complete development environment with all tools installed
- Working openCenter-cli binary built from source
- Tests passing locally
- Editor configured for Go development

**Time:** 15-20 minutes

## Prerequisites

Before starting, ensure you have:
- macOS, Linux, or WSL2 (Windows Subsystem for Linux)
- Git installed and configured
- GitHub account with SSH key configured
- Terminal access
- Text editor or IDE (VS Code, GoLand, vim, etc.)

## Step 1: Install Mise

Mise manages tool versions for the project (Go, kubectl, kind, helm).

**macOS (Homebrew):**
```bash
brew install mise
```

**Linux:**
```bash
curl https://mise.run | sh
```

**Verify installation:**
```bash
mise --version
```

Expected output: `mise 2024.x.x` or similar

## Step 2: Clone Repository

Clone the openCenter-cli repository:

```bash
# Create workspace directory
mkdir -p ~/workspace
cd ~/workspace

# Clone repository
git clone git@github.com:rackerlabs/openCenter-cli.git
cd openCenter-cli
```

If you're contributing, fork first and clone your fork:
```bash
git clone git@github.com:YOUR-USERNAME/openCenter-cli.git
cd openCenter-cli
git remote add upstream git@github.com:rackerlabs/openCenter-cli.git
```

## Step 3: Install Development Tools

Mise reads `.mise.toml` and installs required tools:

```bash
# Install all tools (Go, kubectl, kind, helm)
mise install
```

This installs:
- **Go 1.25.2** - Primary language
- **kubectl** - Kubernetes CLI
- **kind** - Local Kubernetes clusters
- **helm** - Kubernetes package manager

**Verify tools:**
```bash
mise list
```

Expected output shows installed versions:
```
go      1.25.2
kubectl latest
kind    latest
helm    latest
```

## Step 4: Install Go Dependencies

Download all Go module dependencies:

```bash
# Download dependencies
go mod download

# Verify dependencies
go mod verify
```

Expected output: `all modules verified`

## Step 5: Build the CLI

Build the opencenter binary:

```bash
# Build with version information
mise run build
```

Expected output:
```
Built opencenter 0.0.1 (abc1234)
```

**Verify binary:**
```bash
./bin/opencenter version
```

Expected output shows version, commit, branch, and build date.

## Step 6: Run Tests

Verify your environment by running tests:

```bash
# Run unit tests
mise run test
```

Expected output: All tests pass (may take 1-2 minutes)

```bash
# Run BDD tests
mise run godog
```

Expected output: All scenarios pass (may take 2-3 minutes)

If tests fail, check:
- Go version matches `.mise.toml` (1.25.2)
- All dependencies downloaded (`go mod download`)
- No local configuration conflicts (`rm -rf testdata/config`)

## Step 7: Configure Editor

### VS Code

Install recommended extensions:
- **Go** (golang.go) - Go language support
- **YAML** (redhat.vscode-yaml) - YAML validation
- **Cucumber** (alexkrechik.cucumberautocomplete) - Gherkin syntax

**Workspace settings** (`.vscode/settings.json`):
```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "go.formatTool": "gofmt",
  "go.formatOnSave": true,
  "go.testFlags": ["-v"],
  "go.testTimeout": "5m"
}
```

### GoLand / IntelliJ IDEA

1. Open project directory
2. GoLand auto-detects Go module
3. Enable **File Watchers** for gofmt:
   - Settings → Tools → File Watchers
   - Add → go fmt
   - Scope: Project Files

### Vim / Neovim

Install vim-go plugin:
```vim
" Add to .vimrc or init.vim
Plug 'fatih/vim-go', { 'do': ':GoUpdateBinaries' }

" Configure
let g:go_fmt_command = "gofmt"
let g:go_auto_type_info = 1
let g:go_def_mapping_enabled = 1
```

## Step 8: Set Up Shell Integration (Optional)

Enable shell completion and prompt integration:

```bash
# Install shell integration
mise run install-shell-integration
```

This adds:
- Command completion (bash/zsh/fish)
- Active cluster in prompt
- Cluster switching shortcuts

**Reload shell:**
```bash
# Bash
source ~/.bashrc

# Zsh
source ~/.zshrc

# Fish
source ~/.config/fish/config.fish
```

**Test completion:**
```bash
opencenter cluster <TAB>
```

Expected: Shows available subcommands

## Step 9: Create Test Cluster (Optional)

Verify end-to-end functionality with a local Kind cluster:

```bash
# Initialize test cluster configuration
./bin/opencenter cluster init test-dev --org my-org

# Validate configuration
./bin/opencenter cluster validate test-dev

# Create local Kind cluster (requires Docker or Podman)
mise run kind-cluster-no-cni
```

Expected: Kind cluster created with 1 control plane + 3 workers

**Clean up:**
```bash
kind delete cluster --name opencenter-dev
```

## Check Your Work

Verify your development environment:

```bash
# 1. Mise installed and tools available
mise list

# 2. Binary builds successfully
mise run build
./bin/opencenter version

# 3. Tests pass
mise run test
mise run godog

# 4. Code formatting works
mise run fmt

# 5. Dependencies are tidy
mise run tidy
```

All commands should complete without errors.

## Troubleshooting

### Mise not found

**Problem:** `mise: command not found`

**Solution:**
```bash
# Add to shell profile (~/.bashrc, ~/.zshrc, etc.)
export PATH="$HOME/.local/bin:$PATH"

# Reload shell
source ~/.bashrc  # or ~/.zshrc
```

### Go version mismatch

**Problem:** `go: version "1.25.2" does not match go.mod`

**Solution:**
```bash
# Let mise manage Go version
mise install go

# Verify
mise which go
```

### Tests fail with "config directory not found"

**Problem:** Tests fail with configuration errors

**Solution:**
```bash
# Clean test artifacts
rm -rf testdata/config

# Re-run tests
mise run test
```

### Build fails with "package not found"

**Problem:** `package github.com/... not found`

**Solution:**
```bash
# Download dependencies
go mod download

# Verify
go mod verify

# Rebuild
mise run build
```

### Kind cluster creation fails

**Problem:** `kind create cluster` fails

**Solution:**
```bash
# Check Docker/Podman is running
docker ps  # or: podman ps

# If using Podman, set environment variable
export KIND_EXPERIMENTAL_PROVIDER=podman

# Retry
mise run kind-cluster-no-cni
```

## Next Steps

Now that your environment is set up:

1. **Read the code structure** - [Code Structure](code-structure.md)
2. **Learn the build system** - [Build System](build-system.md)
3. **Write your first test** - [Testing Guide](testing-guide.md)
4. **Make your first contribution** - [Contributing](contributing.md)

## Common Development Tasks

**Build and test:**
```bash
mise run build && mise run test && mise run godog
```

**Format and tidy:**
```bash
mise run fmt && mise run tidy
```

**Schema changes:**
```bash
mise run schema-verify
```

**Clean build artifacts:**
```bash
mise run clean
```

**See all available tasks:**
```bash
mise tasks
```

---

## Evidence

This documentation is based on the following repository files:

- Tool versions: `.mise.toml:1-5` (tools section)
- Build process: `.mise.toml:23-47` (build task)
- Test execution: `.mise.toml:64-67` (test tasks)
- Development guide: `.kiro/steering/tech.md:1-149`
- Project structure: `.kiro/steering/structure.md:1-128`
- Go dependencies: `go.mod:1-77`
- Shell integration: `.mise.toml:945-948` (install-shell-integration task)
- Kind cluster setup: `.mise.toml:920-937` (kind-cluster-no-cni task)
