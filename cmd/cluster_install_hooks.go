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
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rackerlabs/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// newClusterInstallHooksCmd creates the command for installing Git pre-commit hooks.
func newClusterInstallHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-hooks [cluster]",
		Short: "Install Git pre-commit hooks for secrets validation",
		Long: `Install Git pre-commit hooks that validate secrets before commits.

This command installs a pre-commit hook in the GitOps repository that
automatically validates staged files before allowing commits. The hook prevents:

  • Plaintext Age keys from being committed
  • Plaintext SSH keys from being committed
  • Unencrypted secret manifests from being committed
  • Configuration drift between config and manifests

The pre-commit hook runs automatically on every commit and blocks the commit
if any validation failures are detected. This provides an additional safety
layer to prevent accidental exposure of secrets in Git.

Hook validation checks:
  1. Scans staged files for plaintext Age keys (*.txt in secrets/age/)
  2. Scans staged files for plaintext SSH keys (in secrets/ssh/)
  3. Checks that secret manifests are SOPS-encrypted
  4. Validates that staged manifests match the cluster config (no drift)

The hook can be bypassed in emergencies using:
  OPENCENTER_SKIP_HOOKS=1 git commit

However, bypassing the hook is NOT RECOMMENDED and should only be done
with explicit approval and understanding of the security implications.

If no cluster name is provided, uses the currently active cluster.`,
		Example: `  # Install hooks in current directory
  opencenter cluster install-hooks my-cluster

  # Install hooks in specific repository path
  opencenter cluster install-hooks my-cluster --repo-path /path/to/repo

  # Force overwrite existing hooks
  opencenter cluster install-hooks my-cluster --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterInstallHooks,
	}

	cmd.Flags().String("repo-path", ".", "Path to GitOps repository (default: current directory)")
	cmd.Flags().Bool("force", false, "Overwrite existing hooks")

	return cmd
}

func runClusterInstallHooks(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get flags
	repoPath, _ := cmd.Flags().GetString("repo-path")
	force, _ := cmd.Flags().GetBool("force")

	// Resolve cluster name
	clusterName, err := resolveClusterName(args, true)
	if err != nil {
		return err
	}

	// Resolve absolute repository path
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve repository path: %w", err)
	}

	// Check if it's a Git repository
	gitDir := filepath.Join(absRepoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", absRepoPath)
	}

	// Check if hooks already exist
	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); err == nil && !force {
		return fmt.Errorf("pre-commit hook already exists at %s\nUse --force to overwrite", hookPath)
	}

	// Initialize hook manager
	hookManager, err := initializeHookManager()
	if err != nil {
		return fmt.Errorf("failed to initialize hook manager: %w", err)
	}

	// Install hooks
	if err := hookManager.InstallHooks(ctx, absRepoPath, clusterName); err != nil {
		return fmt.Errorf("failed to install hooks: %w", err)
	}

	// Display success message and instructions
	displayHookInstallationSuccess(cmd, clusterName, absRepoPath, hookPath)

	return nil
}

// displayHookInstallationSuccess displays installation success message and usage instructions
func displayHookInstallationSuccess(cmd *cobra.Command, clusterName, repoPath, hookPath string) {
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Pre-commit hooks installed successfully\n\n")

	fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", clusterName)
	fmt.Fprintf(cmd.OutOrStdout(), "Repository: %s\n", repoPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Hook path: %s\n\n", hookPath)

	fmt.Fprintln(cmd.OutOrStdout(), "The pre-commit hook will now automatically validate:")
	fmt.Fprintln(cmd.OutOrStdout(), "  • Plaintext Age keys are not committed")
	fmt.Fprintln(cmd.OutOrStdout(), "  • Plaintext SSH keys are not committed")
	fmt.Fprintln(cmd.OutOrStdout(), "  • Secret manifests are SOPS-encrypted")
	fmt.Fprintln(cmd.OutOrStdout(), "  • Staged manifests match cluster configuration")
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintln(cmd.OutOrStdout(), "Usage:")
	fmt.Fprintln(cmd.OutOrStdout(), "  The hook runs automatically on every commit.")
	fmt.Fprintln(cmd.OutOrStdout(), "  If validation fails, the commit will be blocked.")
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintln(cmd.OutOrStdout(), "Bypassing the hook (NOT RECOMMENDED):")
	fmt.Fprintln(cmd.OutOrStdout(), "  OPENCENTER_SKIP_HOOKS=1 git commit")
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintln(cmd.OutOrStdout(), "⚠️  Security Warning:")
	fmt.Fprintln(cmd.OutOrStdout(), "  Bypassing the hook can lead to accidental exposure of secrets.")
	fmt.Fprintln(cmd.OutOrStdout(), "  Only bypass with explicit approval and understanding of risks.")
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintln(cmd.OutOrStdout(), "Testing the hook:")
	fmt.Fprintln(cmd.OutOrStdout(), "  1. Make a change to a secret manifest")
	fmt.Fprintln(cmd.OutOrStdout(), "  2. Stage the change: git add <file>")
	fmt.Fprintln(cmd.OutOrStdout(), "  3. Try to commit: git commit -m \"test\"")
	fmt.Fprintln(cmd.OutOrStdout(), "  4. The hook will validate before allowing the commit")
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintln(cmd.OutOrStdout(), "Uninstalling hooks:")
	fmt.Fprintf(cmd.OutOrStdout(), "  rm %s\n", hookPath)
	fmt.Fprintln(cmd.OutOrStdout(), "  Or use: opencenter cluster uninstall-hooks")
}

// initializeHookManager creates and configures a hook manager instance
func initializeHookManager() (secrets.HookManager, error) {
	logger := createSecretsLogger()
	configLoader := createConfigLoader()
	sopsManager := createSOPSManager(logger)
	auditLogger := &noOpAuditLogger{}

	// Create secrets manager (needed by hook manager)
	secretsManager := secrets.NewDefaultSecretsManager(configLoader, sopsManager, auditLogger, logger)

	// Create hook manager
	hookManager := secrets.NewDefaultHookManager(secretsManager, logger)

	return hookManager, nil
}
