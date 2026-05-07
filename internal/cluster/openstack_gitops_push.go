package cluster

import (
	"context"
	"fmt"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const openStackGitOpsPushStepID = "openstack-gitops-push"

// buildGitOpsPushStep returns a bootstrap step that pushes the local GitOps
// repository to the configured remote. This ensures FluxCD can reconcile the
// cluster state from the remote repository.
func (p *openstackBootstrapProvider) buildGitOpsPushStep(
	cfg *v2.Config,
	planEnv []BootstrapPlanEnv,
	opts *BootstrapOptions,
) bootstrapStep {
	gitDir := cfg.GitDir()
	gitURL := cfg.ConfiguredGitURL()

	return bootstrapStep{
		ID:          openStackGitOpsPushStepID,
		Description: "Push GitOps repository to remote",
		Plan: BootstrapPlanStep{
			ID:         openStackGitOpsPushStepID,
			Action:     "Push GitOps repository to remote origin",
			WorkingDir: gitDir,
			Commands: []BootstrapPlanCommand{
				commandPlan("git", "remote", "add", "origin", gitURL),
				commandPlan("git", "stash", "--include-untracked"),
				commandPlan("git", "pull", "--rebase", "origin", "main"),
				commandPlan("git", "stash", "pop"),
				commandPlan("git", "add", "-A"),
				commandPlan("git", "commit", "-m", "chore: bootstrap cluster gitops state"),
				commandPlan("git", "push", "-u", "origin", "main"),
			},
			Environment: planEnv,
			Reads:       []string{gitDir},
			Writes:      []string{"Remote git repository"},
			Notes:       []string{"Plan only; git remote access and authentication were not checked."},
		},
		Run: func(ctx context.Context) error {
			return p.runGitOpsPush(ctx, cfg, gitDir, gitURL)
		},
	}
}

// runGitOpsPush ensures the origin remote is correctly configured, pulls with
// rebase, and pushes the local GitOps repository to the remote.
func (p *openstackBootstrapProvider) runGitOpsPush(ctx context.Context, cfg *v2.Config, gitDir, gitURL string) error {
	if strings.TrimSpace(gitDir) == "" {
		return fmt.Errorf("gitops.git_dir must be configured for gitops push")
	}
	if strings.TrimSpace(gitURL) == "" {
		return fmt.Errorf("gitops.repository.url must be configured for gitops push")
	}

	env, err := buildGitOpsPushEnvironment(cfg)
	if err != nil {
		return err
	}

	// Check if origin remote already exists
	if err := p.ensureOriginRemote(ctx, gitDir, env, gitURL); err != nil {
		return err
	}

	// Stash any unstaged changes so pull --rebase can proceed
	if _, err := p.runner.Run(ctx, gitDir, env, "git", "stash", "--include-untracked"); err != nil {
		return fmt.Errorf("git stash: %w", err)
	}

	// Pull with rebase to incorporate any remote changes
	if _, err := p.runner.Run(ctx, gitDir, env, "git", "pull", "--rebase", "origin", "main"); err != nil {
		// Restore stashed changes before returning the error
		_, _ = p.runner.Run(ctx, gitDir, env, "git", "stash", "pop")
		return fmt.Errorf("git pull --rebase from origin: %w", err)
	}

	// Restore stashed changes on top of the rebased history
	if _, err := p.runner.Run(ctx, gitDir, env, "git", "stash", "pop"); err != nil {
		return fmt.Errorf("git stash pop: %w", err)
	}

	// Stage all changes and commit before pushing
	if _, err := p.runner.Run(ctx, gitDir, env, "git", "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	// Commit only if there are staged changes (--allow-empty is not used)
	_, _ = p.runner.Run(ctx, gitDir, env, "git", "commit", "-m", "chore: bootstrap cluster gitops state")

	// Push to remote
	if _, err := p.runner.Run(ctx, gitDir, env, "git", "push", "-u", "origin", "main"); err != nil {
		return fmt.Errorf("git push to origin: %w", err)
	}

	fmt.Println("\n✓ GitOps repository pushed to remote")
	fmt.Println("\nTo check FluxCD reconciliation status, run:")
	fmt.Printf("  kubectl get kustomizations -n flux-system\n")
	fmt.Printf("  kubectl get gitrepositories -n flux-system\n")
	fmt.Printf("  flux get all -A\n")

	return nil
}

// ensureOriginRemote verifies the origin remote is set to the expected URL.
// If origin does not exist, it adds it. If it exists but points to a different
// URL, it returns an error to prevent pushing to the wrong repository.
func (p *openstackBootstrapProvider) ensureOriginRemote(ctx context.Context, gitDir string, env map[string]string, expectedURL string) error {
	output, err := p.runner.Run(ctx, gitDir, env, "git", "remote", "get-url", "origin")
	if err != nil {
		// Origin doesn't exist — add it
		if _, addErr := p.runner.Run(ctx, gitDir, env, "git", "remote", "add", "origin", expectedURL); addErr != nil {
			return fmt.Errorf("add git remote origin: %w", addErr)
		}
		return nil
	}

	// Origin exists — verify it matches the configured URL
	currentURL := strings.TrimSpace(string(output))
	if currentURL != strings.TrimSpace(expectedURL) {
		return fmt.Errorf("git remote origin is %q but configuration expects %q; update the remote or fix gitops.repository.url", currentURL, expectedURL)
	}

	return nil
}

// buildGitOpsPushEnvironment constructs the environment variables needed for
// git operations. It includes token-based auth when configured.
func buildGitOpsPushEnvironment(cfg *v2.Config) (map[string]string, error) {
	env := make(map[string]string)

	// Pass through any configured git credentials
	if cfg.OpenCenter.GitOps.Auth.Token != nil {
		tokenFile := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.TokenFile)
		if tokenFile != "" {
			env["GIT_TOKEN_FILE"] = tokenFile
		}
	}

	return env, nil
}
