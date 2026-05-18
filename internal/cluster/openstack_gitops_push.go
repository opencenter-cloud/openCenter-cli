package cluster

import (
	"context"
	"fmt"
	"net/url"
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

	// Build a masked version of the authenticated URL for the plan output
	// (shows the format without exposing the actual token)
	gitOrg := ""
	if cfg.OpenCenter.GitOps.Auth.Token != nil {
		gitOrg = strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Organization)
	}
	planURL := fmt.Sprintf("https://%s:<token>@<host>/<path>.git", gitOrg)

	return bootstrapStep{
		ID:          openStackGitOpsPushStepID,
		Description: "Push GitOps repository to remote",
		Plan: BootstrapPlanStep{
			ID:         openStackGitOpsPushStepID,
			Action:     "Push GitOps repository to remote origin",
			WorkingDir: gitDir,
			Commands: []BootstrapPlanCommand{
				commandPlan("git", "remote", "add", "origin", planURL),
				commandPlan("git", "stash", "--include-untracked"),
				commandPlan("git", "pull", "--rebase", "origin", "main"),
				commandPlan("git", "stash", "pop"),
				commandPlan("git", "push", "-u", "origin", "main"),
			},
			Environment: planEnv,
			Reads:       []string{gitDir},
			Writes:      []string{"Remote git repository"},
			Notes:       []string{"Plan only; git remote access and authentication were not checked. Uses gitops token for push."},
		},
		Run: func(ctx context.Context) error {
			return p.runGitOpsPush(ctx, cfg, gitDir, gitURL)
		},
	}
}

// runGitOpsPush ensures the origin remote is correctly configured, pulls with
// rebase, and pushes the local GitOps repository to the remote. Authentication
// uses the git token from the gitops configuration section.
func (p *openstackBootstrapProvider) runGitOpsPush(ctx context.Context, cfg *v2.Config, gitDir, gitURL string) error {
	if strings.TrimSpace(gitDir) == "" {
		return fmt.Errorf("gitops.git_dir must be configured for gitops push")
	}
	if strings.TrimSpace(gitURL) == "" {
		return fmt.Errorf("gitops.repository.url must be configured for gitops push")
	}

	// Resolve the git token for authenticated push
	token, err := resolveFluxToken(cfg)
	if err != nil {
		return fmt.Errorf("resolving git token for push: %w", err)
	}

	// Resolve the git organization for the URL username
	gitOrg := ""
	if cfg.OpenCenter.GitOps.Auth.Token != nil {
		gitOrg = strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Organization)
	}
	if gitOrg == "" {
		return fmt.Errorf("gitops.auth.token.organization must be configured for gitops push")
	}

	env := buildGitOpsPushEnvironment()

	// Build the authenticated URL: https://<organization>:<token>@<host>/<path>
	authURL, err := buildAuthenticatedGitURL(gitURL, gitOrg, token)
	if err != nil {
		return fmt.Errorf("building authenticated git URL: %w", err)
	}

	// Check if origin remote already exists; if not, add it with the
	// authenticated URL. If it exists, verify it matches.
	if err := p.ensureOriginRemote(ctx, gitDir, env, authURL); err != nil {
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

	// Push to remote using the authenticated origin URL
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

	// Origin exists — verify the host/path matches the configured URL.
	// Strip credentials before comparing to avoid false mismatches from
	// different tokens and to avoid leaking tokens in error messages.
	currentURL := strings.TrimSpace(string(output))
	if stripCredentialsFromURL(currentURL) != stripCredentialsFromURL(strings.TrimSpace(expectedURL)) {
		return fmt.Errorf("git remote origin host/path %q does not match configuration %q; update the remote or fix gitops.repository.url", stripCredentialsFromURL(currentURL), stripCredentialsFromURL(expectedURL))
	}

	// Update the remote URL to ensure the current token is used
	if _, err := p.runner.Run(ctx, gitDir, env, "git", "remote", "set-url", "origin", expectedURL); err != nil {
		return fmt.Errorf("update git remote origin URL: %w", err)
	}

	return nil
}

// buildAuthenticatedGitURL constructs an HTTPS URL with credentials in the
// format https://<organization>:<token>@<host>/<path>.
// For SSH URLs, it converts to HTTPS with the embedded credentials.
func buildAuthenticatedGitURL(gitURL, organization, token string) (string, error) {
	gitURL = strings.TrimSpace(gitURL)

	// Handle SSH format: git@host:owner/repo.git
	if strings.HasPrefix(gitURL, "git@") {
		parts := strings.SplitN(gitURL, ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid SSH URL format: %s", gitURL)
		}
		host := strings.TrimPrefix(parts[0], "git@")
		path := parts[1]
		return fmt.Sprintf("https://%s:%s@%s/%s", organization, token, host, path), nil
	}

	if strings.HasPrefix(gitURL, "ssh://") {
		// Convert ssh://git@host/owner/repo.git → https://org:token@host/owner/repo.git
		parsed, err := url.Parse(gitURL)
		if err != nil {
			return "", fmt.Errorf("parsing SSH URL %s: %w", gitURL, err)
		}
		return fmt.Sprintf("https://%s:%s@%s%s", organization, token, parsed.Host, parsed.Path), nil
	}

	// HTTPS URL — set userinfo to organization:token
	parsed, err := url.Parse(gitURL)
	if err != nil {
		return "", fmt.Errorf("parsing URL %s: %w", gitURL, err)
	}
	parsed.User = url.UserPassword(organization, token)
	return parsed.String(), nil
}

// stripCredentialsFromURL removes userinfo (username/password/token) from a URL
// for safe comparison.
func stripCredentialsFromURL(rawURL string) string {
	// Handle SSH scp-style URLs (git@host:path)
	if strings.HasPrefix(rawURL, "git@") {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = nil
	return parsed.String()
}

// buildGitOpsPushEnvironment constructs the environment variables for git
// operations. Token auth is handled via the push URL, not env vars.
func buildGitOpsPushEnvironment() map[string]string {
	return map[string]string{
		// Prevent git from prompting for credentials interactively
		"GIT_TERMINAL_PROMPT": "0",
	}
}
