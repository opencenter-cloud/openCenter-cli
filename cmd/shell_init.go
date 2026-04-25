// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed shell-integration/*
var shellIntegrationFS embed.FS

func NewShellInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell-init",
		Short: "Output shell integration script for session-scoped active clusters",
		Long: `Outputs shell integration code to be evaluated in your shell.

This enables session-scoped active clusters, where each terminal can have
its own active cluster context without affecting other terminals.

Add to your shell configuration file:

  Bash (~/.bashrc):
    eval "$(opencenter shell-init)"

  Zsh (~/.zshrc):
    eval "$(opencenter shell-init)"

  Fish (~/.config/fish/config.fish):
    opencenter shell-init --shell fish | source

Features:
  • Session-isolated cluster contexts
  • Visual prompt indicator showing active cluster
  • Automatic cleanup on shell exit
  • Compatible with existing persistent selection`,
		Example: `  # Auto-detect shell and output integration script
  opencenter shell-init

  # Specify shell explicitly
  opencenter shell-init --shell zsh

  # Install to shell config
  echo 'eval "$(opencenter shell-init)"' >> ~/.zshrc`,
		RunE: runShellInit,
	}

	cmd.Flags().String("shell", "", "Shell type (bash, zsh, fish) - auto-detected if not specified")

	return cmd
}

func newShellInitCmd() *cobra.Command {
	return NewShellInitCmd()
}

func runShellInit(cmd *cobra.Command, args []string) error {
	shell, _ := cmd.Flags().GetString("shell")

	// Auto-detect shell if not specified
	if shell == "" {
		shell = detectShellForInit()
	}

	// Read embedded shell integration script
	var scriptPath string
	switch shell {
	case "bash":
		scriptPath = "shell-integration/integration.bash"
	case "zsh":
		scriptPath = "shell-integration/integration.zsh"
	case "fish":
		scriptPath = "shell-integration/integration.fish"
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
	}

	content, err := shellIntegrationFS.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("shell integration not available for %s: %w", shell, err)
	}

	fmt.Fprint(cmd.OutOrStdout(), string(content))
	return nil
}

func detectShellForInit() string {
	shell := os.Getenv("SHELL")
	baseName := filepath.Base(shell)

	switch baseName {
	case "zsh":
		return "zsh"
	case "fish":
		return "fish"
	case "bash", "sh":
		return "bash"
	default:
		// Default to bash for unknown shells
		return "bash"
	}
}
