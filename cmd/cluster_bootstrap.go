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
	"github.com/opencenter-cloud/opencenter-cli/internal/resilience"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newClusterBootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap [name]",
		Short: "Run provider-specific bootstrap actions for a cluster",
		Long: `Run provider-specific bootstrap actions for a cluster.

This command provisions infrastructure and deploys the Kubernetes cluster
based on the cluster configuration. The bootstrap process varies by provider:

- OpenStack/VMware: Runs provider-specific infrastructure bootstrap
- Kind: Creates a local Kubernetes cluster using kind

Only v2 configurations (schema_version: "2.0") are supported.
Configurations with any other schema version are invalid.

The bootstrap process is resumable - if a step fails, you can fix the issue
and re-run bootstrap to continue from where it left off.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterBootstrap,
	}

	cmd.Flags().Bool("dry-run", false, "show planned actions without executing")
	cmd.Flags().String("kubeconfig", "", "path to kubeconfig used by bootstrap actions (defaults to the cluster-owned kubeconfig path)")
	cmd.Flags().String("log", "", "log file path (defaults to <state_dir>/logs/bootstrap/<org>/<name>/bootstrap-YYYYMMDDTHHMMSSZ.log)")
	cmd.Flags().String("container-runtime", "", "container runtime for kind clusters (docker or podman)")
	cmd.Flags().Bool("restart", false, "rerun all bootstrap steps and ignore saved state")
	cmd.Flags().String("step", "", "run a single bootstrap step by ID")
	cmd.Flags().String("from-step", "", "restart bootstrap from the specified step ID")

	return cmd
}

func runClusterBootstrap(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Resolve cluster name from args or active cluster
	name, err := resolveClusterName(args, true)
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

	// Acquire lock for bootstrap operation
	lockMgr, err := resilience.NewLockManager(resilience.DefaultLockConfig)
	if err != nil {
		return fmt.Errorf("failed to create lock manager: %w", err)
	}

	lock, err := lockMgr.AcquireWithMetadata(ctx, name, 1*time.Hour, map[string]string{
		"operation": "bootstrap",
		"command":   "cluster bootstrap",
	})
	if err != nil {
		return fmt.Errorf("failed to acquire lock for cluster %q: %w\nAnother operation may be in progress. Wait for it to complete or use 'opencenter cluster info %s' to check lock status", name, err, name)
	}
	defer lockMgr.Release(lock)

	app, err := di.NewApp(config.ResolveClustersDir())
	if err != nil {
		return fmt.Errorf("initialize application graph: %w", err)
	}
	bootstrapService := app.BootstrapService

	// Parse command-line options
	opts, err := parseBootstrapOptions(cmd, args, actualClusterName)
	if err != nil {
		return err
	}
	opts.Organization = organization

	// Pre-check: ensure the GitOps working tree is clean before bootstrap.
	// A dirty tree causes git pull --rebase to fail during the gitea-rebase step.
	if !opts.DryRun {
		if gitDir := strings.TrimSpace(cfg.OpenCenter.GitOps.GitDir); gitDir != "" {
			if err := ensureCleanWorkingTree(ctx, cmd, gitDir); err != nil {
				return err
			}
			// Verify the local repo's origin remote points to git_url so the
			// gitea-rebase and gitops-push steps operate against the expected remote.
			if gitURL := strings.TrimSpace(cfg.OpenCenter.GitOps.GitURL); gitURL != "" {
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

	// Execute bootstrap
	result, err := bootstrapService.Bootstrap(ctx, opts)
	if err != nil {
		if !opts.DryRun {
			if statusErr := config.UpdateStatus(name, config.StageBootstrap, config.StatusFailed); statusErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", statusErr)
			}
		}
		if result != nil {
			if strings.TrimSpace(result.LogPath) != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Bootstrap log: %s\n", result.LogPath)
			}
			if strings.TrimSpace(result.ResumeStatePath) != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Resume state: %s\n", result.ResumeStatePath)
			}
		}
		return err
	}

	// Display results
	fmt.Fprintf(cmd.OutOrStdout(), "Bootstrap complete in %v\n", result.Duration.Round(time.Second))
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
	opts.DryRun, _ = cmd.Flags().GetBool("dry-run")
	opts.KubeconfigPath, _ = cmd.Flags().GetString("kubeconfig")
	opts.LogPath, _ = cmd.Flags().GetString("log")
	opts.ContainerRuntime, _ = cmd.Flags().GetString("container-runtime")
	opts.Restart, _ = cmd.Flags().GetBool("restart")
	opts.OnlyStep, _ = cmd.Flags().GetString("step")
	opts.FromStep, _ = cmd.Flags().GetString("from-step")

	// Validate mutually exclusive flags
	if strings.TrimSpace(opts.OnlyStep) != "" && strings.TrimSpace(opts.FromStep) != "" {
		return opts, fmt.Errorf("--step and --from-step cannot be used together")
	}

	return opts, nil
}

// ensureCleanWorkingTree checks whether the GitOps directory has uncommitted
// changes. If it does, the user is prompted to commit them before proceeding.
// Returning an error aborts the bootstrap.
func ensureCleanWorkingTree(ctx context.Context, cmd *cobra.Command, gitDir string) error {
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

	testMode := os.Getenv("OPENCENTER_TEST_MODE") != ""
	prompter := ui.GetPrompter(os.Stdin, cmd.OutOrStdout(), testMode)

	confirmed, err := prompter.Confirm(ctx, "Commit all changes before continuing?")
	if err != nil {
		return fmt.Errorf("confirmation prompt failed: %w", err)
	}
	if !confirmed {
		return fmt.Errorf("bootstrap aborted: uncommitted changes in %s\nPlease commit or stash your changes and retry", gitDir)
	}

	addCmd := exec.CommandContext(ctx, "git", "-C", gitDir, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	commitCmd := exec.CommandContext(ctx, "git", "-C", gitDir, "commit", "-m", "committing staged changes")
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
