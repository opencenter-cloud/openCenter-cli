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
				commandPlan("git", "stash"),
				commandPlan("git", "pull", "--rebase", "origin", "main"),
				commandPlan("git", "stash", "pop"),
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

// runGitOpsPush adds the remote origin (if not already configured), pulls with
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

	// Add origin remote — ignore errors if it already exists
	_, _ = p.runner.Run(ctx, gitDir, env, "git", "remote", "add", "origin", gitURL)

	// Pull with rebase to incorporate any remote changes
	if _, err := p.runner.Run(ctx, gitDir, env, "git", "pull", "--rebase", "origin", "main"); err != nil {
		return fmt.Errorf("git pull --rebase from origin: %w", err)
	}

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
