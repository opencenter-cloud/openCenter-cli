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
	"os/exec"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newClusterDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [name]",
		Short: "Deploy a cluster from its openCenter configuration",
		Long: `Deploy a cluster from its openCenter configuration.

This command provisions infrastructure and deploys Kubernetes based on the
cluster configuration. The process is resumable; if a step fails, fix the issue
and re-run deploy to continue from the saved state.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterDeploy,
	}

	cmd.Flags().String("kubeconfig", "", "path to kubeconfig used by deploy actions (defaults to the cluster-owned kubeconfig path)")
	cmd.Flags().String("log", "", "log file path (defaults to <state_dir>/logs/bootstrap/<org>/<name>/bootstrap-YYYYMMDDTHHMMSSZ.log)")
	cmd.Flags().String("container-runtime", "", "container runtime for kind clusters (docker or podman)")
	cmd.Flags().Bool("restart", false, "rerun all deploy steps and ignore saved state")
	cmd.Flags().Bool("debug", false, "print deploy step debug details before each step runs")
	cmd.Flags().String("step", "", "run a single deploy step by ID")
	cmd.Flags().String("from-step", "", "restart deploy from the specified step ID")
	cmd.Flags().Bool("confirm-commit", false, "prompt for confirmation before auto-committing uncommitted changes")
	cmd.Flags().Bool("break-lock", false, "force removal of an existing operation lock before deploying")

	return cmd
}

func runClusterDeploy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Apply log-level flag (already parsed by PersistentPreRunE, but ensure
	// it is honoured when the caller passes --log-level explicitly).
	if logLevel, _ := cmd.Flags().GetString("log-level"); logLevel != "" {
		_ = config.SetLogLevel(logLevel)
	}

	// Resolve cluster name from args or active cluster
	name, err := resolveClusterNameForCommand(cmd, args, true)
	if err != nil {
		return err
	}

	// Reject planned providers that are not yet available
	cfg, err := loadCanonicalConfig(name)
	if err == nil {
		if err := checkProviderAvailability(cfg.OpenCenter.Infrastructure.Provider); err != nil {
			return err
		}
	}

	// Extract just the cluster name (without organization prefix) for path resolution
	actualClusterName := extractClusterName(name)
	organization := ""
	if err == nil {
		organization = cfg.OpenCenter.Meta.Organization
	}

	// Pre-check: ensure all secret manifests are encrypted before deploying.
	// This catches unencrypted secrets early, before any infrastructure changes.
	if err == nil {
		gitDir := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.LocalDir)
		if gitDir != "" {
			if unencryptedErr := checkUnencryptedSecrets(gitDir); unencryptedErr != nil {
				return unencryptedErr
			}
		}
	}

	// Parse command-line options
	opts, err := parseBootstrapOptions(cmd, args, actualClusterName)
	if err != nil {
		return err
	}
	opts.Organization = organization

	if !opts.DryRun {
		// Acquire lock for deploy operation (with prompt if lock exists)
		lockResult, err := AcquireLockWithPrompt(ctx, cmd, name, "deploy", 1*time.Hour, map[string]string{
			"operation": "deploy",
			"command":   "cluster deploy",
		})
		if err != nil {
			return err
		}
		defer func() {
			if lockResult.Lock != nil {
				lockResult.LockManager.Release(lockResult.Lock)
			}
		}()
	}

	app, err := di.NewApp(config.ResolveClustersDir())
	if err != nil {
		return fmt.Errorf("initialize application graph: %w", err)
	}
	bootstrapService := app.BootstrapService
	bootstrapService.SetOutput(cmd.OutOrStdout())

	// Pre-check: ensure the GitOps working tree is clean before bootstrap.
	// A dirty tree causes git pull --rebase to fail during the gitea-rebase step.
	if !opts.DryRun {
		if gitDir := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.LocalDir); gitDir != "" {
			confirmCommit, _ := cmd.Flags().GetBool("confirm-commit")
			if err := ensureCleanWorkingTree(ctx, cmd, gitDir, confirmCommit); err != nil {
				return err
			}
			// Verify the local repo's origin remote points to git_url so the
			// gitea-rebase and gitops-push steps operate against the expected remote.
			if gitURL := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.URL); gitURL != "" {
				if err := verifyOriginMatchesGitURL(ctx, gitDir, gitURL); err != nil {
					return err
				}
			}
		}
	}

	if !opts.DryRun {
		if err := config.UpdateStatus(name, config.StageBootstrap, config.StatusRunning); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
	}

	// Execute deploy
	result, err := bootstrapService.Bootstrap(ctx, opts)
	if err != nil {
		if !opts.DryRun {
			if statusErr := config.UpdateStatus(name, config.StageBootstrap, config.StatusFailed); statusErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", statusErr)
			}
		}
		if !opts.DryRun && result != nil {
			if strings.TrimSpace(result.LogPath) != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Bootstrap log: %s\n", result.LogPath)
			}
			if strings.TrimSpace(result.ResumeStatePath) != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Resume state: %s\n", result.ResumeStatePath)
			}
		}
		return err
	}

	if opts.DryRun {
		printClusterDeployPlan(cmd.OutOrStdout(), result.Plan)
		return nil
	}

	// Display results
	fmt.Fprintf(cmd.OutOrStdout(), "Deploy complete in %v\n", result.Duration.Round(time.Second))
	if result.Endpoint != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Cluster endpoint: %s\n", result.Endpoint)
	}
	if strings.TrimSpace(result.LogPath) != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Bootstrap log: %s\n", result.LogPath)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Steps completed: %d\n", len(result.StepsCompleted))
	if len(result.StepsFailed) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Steps failed: %d\n", len(result.StepsFailed))
	}

	if !opts.DryRun {
		if err := config.UpdateStatus(name, config.StageBootstrap, config.StatusSuccess); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Next: opencenter cluster status %s\n", name)
	}

	return nil
}

// setupBootstrapContainer initializes the DI container with all required services
func setupBootstrapContainer(container di.Container) error {
	pathResolver, err := di.ProvidePathResolver(config.ResolveClustersDir())
	if err != nil {
		return err
	}
	if err := container.Singleton("path-resolver", func() (*paths.PathResolver, error) {
		return pathResolver, nil
	}); err != nil {
		return err
	}
	if err := container.Singleton("validation-engine", di.ProvideValidationEngine); err != nil {
		return err
	}
	if err := container.Singleton("bootstrap-service", di.ProvideBootstrapService); err != nil {
		return err
	}
	return container.Initialize()
}

// parseBootstrapOptions parses command-line flags into BootstrapOptions
func parseBootstrapOptions(cmd *cobra.Command, args []string, clusterName string) (cluster.BootstrapOptions, error) {
	opts := cluster.BootstrapOptions{
		ClusterName: clusterName,
	}

	// Parse flags
	opts.DryRun = getGlobalOptions(cmd).DryRun
	opts.KubeconfigPath, _ = cmd.Flags().GetString("kubeconfig")
	opts.LogPath, _ = cmd.Flags().GetString("log")
	opts.ContainerRuntime, _ = cmd.Flags().GetString("container-runtime")
	opts.Restart, _ = cmd.Flags().GetBool("restart")
	opts.Debug, _ = cmd.Flags().GetBool("debug")
	opts.OnlyStep, _ = cmd.Flags().GetString("step")
	opts.FromStep, _ = cmd.Flags().GetString("from-step")

	// Validate mutually exclusive flags
	if strings.TrimSpace(opts.OnlyStep) != "" && strings.TrimSpace(opts.FromStep) != "" {
		return opts, fmt.Errorf("--step and --from-step cannot be used together")
	}

	return opts, nil
}

// ensureCleanWorkingTree checks whether the GitOps directory has uncommitted
// changes. If confirmCommit is true, the user is prompted before committing.
// Otherwise, changes are auto-committed without prompting.
// Returning an error aborts the bootstrap.
func ensureCleanWorkingTree(ctx context.Context, cmd *cobra.Command, gitDir string, confirmCommit bool) error {
	statusCmd := exec.CommandContext(ctx, "git", "-C", gitDir, "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil {
		// Not a git repo or git not available — skip the check silently.
		return nil
	}
	if len(strings.TrimSpace(string(output))) == 0 {
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "The GitOps directory has uncommitted changes:\n%s\n", strings.TrimRight(string(output), "\n"))

	// If --confirm-commit is set, prompt for confirmation
	if confirmCommit {
		testMode := os.Getenv("OPENCENTER_TEST_MODE") != ""
		prompter := ui.GetPrompter(os.Stdin, cmd.OutOrStdout(), testMode)

		confirmed, err := prompter.Confirm(ctx, "Commit all changes before continuing?")
		if err != nil {
			return fmt.Errorf("confirmation prompt failed: %w", err)
		}
		if !confirmed {
			return fmt.Errorf("bootstrap aborted: uncommitted changes in %s\nPlease commit or stash your changes and retry", gitDir)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Auto-committing changes...\n")
	}

	addCmd := exec.CommandContext(ctx, "git", "-C", gitDir, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	commitCmd := exec.CommandContext(ctx, "git", "-C", gitDir, "commit", "-m", "chore: auto-commit before bootstrap")
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Changes committed successfully.\n")
	return nil
}

// verifyOriginMatchesGitURL checks that the "origin" remote in gitDir points to
// the expected git_url from the cluster configuration. A mismatch means the
// rebase and push steps would operate against the wrong repository.
func verifyOriginMatchesGitURL(ctx context.Context, gitDir, expectedURL string) error {
	remoteCmd := exec.CommandContext(ctx, "git", "-C", gitDir, "remote", "get-url", "origin")
	output, err := remoteCmd.Output()
	if err != nil {
		// No origin remote — the bootstrap steps will add it, so skip the check.
		return nil
	}
	actual := strings.TrimSpace(string(output))
	if actual != expectedURL {
		return fmt.Errorf("git remote origin in %s points to %q, but git_url is %q\nUpdate the remote with: git -C %s remote set-url origin %s",
			gitDir, actual, expectedURL, gitDir, expectedURL)
	}
	return nil
}

// checkUnencryptedSecrets scans the GitOps directory for unencrypted secret
// manifests. If any are found, it encrypts them using SOPS before proceeding.
func checkUnencryptedSecrets(gitOpsDir string) error {
	findings, err := gitops.ScanGitOpsSecrets(gitOpsDir)
	if err != nil {
		return fmt.Errorf("scanning GitOps directory for unencrypted secrets: %w", err)
	}

	var unencrypted []string
	for _, finding := range findings {
		switch finding.Rule {
		case "unencrypted-kubernetes-secret", "plaintext-secret-field", "invalid-sops-metadata":
			unencrypted = append(unencrypted, finding.Path)
		case "age-private-key", "private-key", "git-token":
			return fmt.Errorf("GitOps directory contains %s in %s: %s\nThis must be removed before deploying", finding.Rule, finding.Path, finding.Message)
		}
	}

	if len(unencrypted) == 0 {
		return nil
	}

	// In test mode, skip real encryption
	if os.Getenv("OPENCENTER_TEST_MODE") == "1" {
		return nil
	}

	// Encrypt unencrypted secrets before deploying
	fmt.Fprintf(os.Stderr, "Encrypting %d unencrypted secret manifest(s)...\n", len(unencrypted))
	if err := executeSOPSSecretsEncrypt(context.Background(), "", gitOpsDir, false, false); err != nil {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("deploy blocked: failed to encrypt %d secret manifest(s):\n", len(unencrypted)))
		for _, path := range unencrypted {
			sb.WriteString(fmt.Sprintf("  • %s\n", path))
		}
		sb.WriteString(fmt.Sprintf("\nEncrypt them manually before deploying:\n"))
		sb.WriteString(fmt.Sprintf("  opencenter secrets encrypt --path %s\n", gitOpsDir))
		sb.WriteString(fmt.Sprintf("\nUnderlying error: %v\n", err))
		return fmt.Errorf("%s", sb.String())
	}

	fmt.Fprintf(os.Stderr, "Encryption complete.\n")
	return nil
}
