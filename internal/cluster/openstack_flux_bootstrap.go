package cluster

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const openStackFluxBootstrapStepID = "openstack-flux-bootstrap"

// buildFluxBootstrapStep returns a bootstrap step that runs `flux bootstrap`
// against the newly provisioned OpenStack cluster. The provider-specific
// command (github, gitea, or gitlab) is chosen based on
// opencenter.gitops.auth.token.provider.
func (p *openstackBootstrapProvider) buildFluxBootstrapStep(
	cfg *v2.Config,
	clusterDir string,
	planEnv []BootstrapPlanEnv,
	opts *BootstrapOptions,
) (bootstrapStep, error) {
	params, err := resolveFluxBootstrapParams(cfg)
	if err != nil {
		return bootstrapStep{}, fmt.Errorf("resolving flux bootstrap parameters: %w", err)
	}

	planCommands := fluxBootstrapPlanCommands(params)

	return bootstrapStep{
		ID:          openStackFluxBootstrapStepID,
		Description: fmt.Sprintf("Bootstrap FluxCD using %s provider", params.Provider),
		Plan: BootstrapPlanStep{
			ID:          openStackFluxBootstrapStepID,
			Action:      fmt.Sprintf("Bootstrap FluxCD using %s provider", params.Provider),
			WorkingDir:  clusterDir,
			Commands:    planCommands,
			Environment: planEnv,
			Reads:       []string{opts.KubeconfigPath},
			Writes:      []string{"FluxCD resources in flux-system namespace"},
			Notes:       []string{"Plan only; flux CLI availability, token validity, and Kubernetes API access were not checked."},
		},
		Run: func(ctx context.Context) error {
			return p.runFluxBootstrap(ctx, cfg, opts.KubeconfigPath)
		},
	}, nil
}

// fluxBootstrapParams holds the resolved parameters for a flux bootstrap command.
type fluxBootstrapParams struct {
	// Provider is the git provider: github, gitea, or gitlab.
	Provider string

	// Owner is the repository owner (organization or username).
	Owner string

	// Repository is the repository name (without owner prefix).
	Repository string

	// Branch is the target branch (defaults to main).
	Branch string

	// Path is the cluster path within the repository.
	Path string

	// Hostname is the git server hostname (including port for self-hosted
	// servers), derived from the repository URL. Empty for hosted providers
	// (flux defaults to the provider's public host).
	Hostname string

	// TokenFile is the path to the file containing the access token.
	TokenFile string
}

// resolveFluxBootstrapParams extracts and validates all parameters needed for
// flux bootstrap from the cluster configuration.
func resolveFluxBootstrapParams(cfg *v2.Config) (*fluxBootstrapParams, error) {
	if cfg.OpenCenter.GitOps.Auth.Token == nil {
		return nil, fmt.Errorf("gitops.auth.token must be configured for flux bootstrap; set opencenter.gitops.auth.token.provider to github, gitea, or gitlab")
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Provider))
	if provider == "" {
		return nil, fmt.Errorf("gitops.auth.token.provider is required; set it to github, gitea, or gitlab")
	}

	switch provider {
	case "github", "gitea", "gitlab":
		// valid
	default:
		return nil, fmt.Errorf("unsupported gitops.auth.token.provider %q; supported values: github, gitea, gitlab", provider)
	}

	repoURL := cfg.ConfiguredGitURL()
	if repoURL == "" {
		return nil, fmt.Errorf("gitops.repository.url must be configured for flux bootstrap")
	}

	owner, repo, err := parseFluxGitURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("parsing repository URL %q: %w", repoURL, err)
	}

	// Derive the git server hostname from the repository URL so self-hosted
	// servers (e.g. a local Gitea at https://10.0.0.5:3000) are used instead of
	// the provider's public default (gitea.com). For SSH "git@host:..." URLs
	// flux takes the host via --ssh-hostname, which is handled separately; we
	// only set the HTTPS hostname here.
	hostname := fluxGitHostname(repoURL)

	// Owner from token config takes precedence over URL-derived owner.
	if configOwner := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Owner); configOwner != "" {
		owner = configOwner
	}
	if owner == "" {
		return nil, fmt.Errorf("repository owner could not be determined from URL %q; set opencenter.gitops.auth.token.owner explicitly", repoURL)
	}

	branch := cfg.GitBranchOrDefault()

	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name must be set for flux bootstrap path")
	}
	bootstrapPath := "clusters/" + clusterName

	// Allow the user to override the path via gitops.repository.path.
	if configPath := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.Path); configPath != "" {
		bootstrapPath = configPath
	}

	tokenFile := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.TokenFile)
	if tokenFile == "" {
		// Token may be inline; we'll resolve it at runtime.
		if strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Token) == "" {
			return nil, fmt.Errorf("gitops.auth.token.token_file or gitops.auth.token.token must be configured for flux bootstrap")
		}
	}

	return &fluxBootstrapParams{
		Provider:   provider,
		Owner:      owner,
		Repository: repo,
		Branch:     branch,
		Path:       bootstrapPath,
		Hostname:   hostname,
		TokenFile:  tokenFile,
	}, nil
}

// fluxGitHostname extracts the host (including port, when present) from a git
// URL that uses an explicit scheme (https://, ssh://, http://). It returns an
// empty string for SSH scp-style URLs ("git@host:owner/repo.git"), whose host
// is conveyed to flux through --ssh-hostname instead of --hostname.
//
// It uses parsed.Host (not Hostname) so a self-hosted server on a non-default
// port (e.g. https://10.0.0.5:3000) keeps the port, which flux's --hostname
// flag accepts as "host:port".
func fluxGitHostname(repoURL string) string {
	repoURL = strings.TrimSpace(repoURL)
	if strings.HasPrefix(repoURL, "git@") {
		return ""
	}
	parsed, err := url.Parse(repoURL)
	if err != nil || parsed.Scheme == "" {
		return ""
	}
	return parsed.Host
}

// runFluxBootstrap executes the flux bootstrap command for the configured
// git provider.
func (p *openstackBootstrapProvider) runFluxBootstrap(ctx context.Context, cfg *v2.Config, kubeconfigPath string) error {
	params, err := resolveFluxBootstrapParams(cfg)
	if err != nil {
		return err
	}

	token, err := resolveFluxToken(cfg)
	if err != nil {
		return err
	}

	env := buildBootstrapEnvironment(kubeconfigPath)

	fluxArgs, ok := fluxBootstrapArgs(params)
	if !ok {
		return fmt.Errorf("unsupported git provider: %q", params.Provider)
	}

	switch params.Provider {
	case "github":
		env["GITHUB_TOKEN"] = token
	case "gitea":
		env["GITEA_TOKEN"] = token
	case "gitlab":
		env["GITLAB_TOKEN"] = token
	}

	gitDir := cfg.GitDir()
	if gitDir == "" {
		gitDir = "."
	}

	_, runErr := p.runner.Run(ctx, gitDir, env, "flux", fluxArgs...)
	if runErr != nil {
		return fmt.Errorf("flux bootstrap %s: %w", params.Provider, runErr)
	}

	return nil
}

// resolveFluxToken reads the access token from the configured token file or
// returns the inline token value.
func resolveFluxToken(cfg *v2.Config) (string, error) {
	if cfg.OpenCenter.GitOps.Auth.Token == nil {
		return "", fmt.Errorf("gitops.auth.token is not configured")
	}

	tokenFile := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.TokenFile)
	if tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("reading token file %s: %w", tokenFile, err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("token file %s is empty", tokenFile)
		}
		return token, nil
	}

	inlineToken := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Token)
	if inlineToken != "" {
		if inlineToken == "CHANGEME" {
			return "", fmt.Errorf("gitops.auth.token.token is still set to the placeholder value; replace it with a real access token")
		}
		return inlineToken, nil
	}

	return "", fmt.Errorf("no token available: set gitops.auth.token.token_file or gitops.auth.token.token")
}

// parseFluxGitURL extracts owner and repository from various Git URL formats.
// Supports SSH (git@host:owner/repo.git), HTTPS, and ssh:// formats.
func parseFluxGitURL(repoURL string) (owner, repo string, err error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return "", "", fmt.Errorf("empty repository URL")
	}

	// Handle SSH format: git@host:owner/repo.git
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.SplitN(repoURL, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH URL format: %s", repoURL)
		}
		pathPart := strings.TrimSuffix(parts[1], ".git")
		return splitFluxOwnerRepo(pathPart)
	}

	// Handle URL formats: https://... or ssh://...
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing URL %s: %w", repoURL, err)
	}

	urlPath := strings.TrimPrefix(parsed.Path, "/")
	urlPath = strings.TrimSuffix(urlPath, ".git")

	return splitFluxOwnerRepo(urlPath)
}

// splitFluxOwnerRepo splits "owner/repo" or "owner/group/repo" into owner and
// repo. For nested groups (GitLab), the full path before the last segment is
// the owner.
func splitFluxOwnerRepo(pathPart string) (owner, repo string, err error) {
	pathPart = strings.Trim(pathPart, "/")
	if pathPart == "" {
		return "", "", fmt.Errorf("empty path in URL")
	}

	parts := strings.Split(pathPart, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("URL path %q does not contain owner/repo", pathPart)
	}

	repo = parts[len(parts)-1]
	owner = path.Join(parts[:len(parts)-1]...)

	return owner, repo, nil
}

// fluxBootstrapPlanCommands returns the planned commands for the dry-run output.
func fluxBootstrapPlanCommands(params *fluxBootstrapParams) []BootstrapPlanCommand {
	args, ok := fluxBootstrapArgs(params)
	if !ok {
		return nil
	}
	return []BootstrapPlanCommand{commandPlan("flux", args...)}
}

// fluxBootstrapArgs returns the shared flux bootstrap argument list for a set
// of params, kept in sync between the plan display and the actual run.
func fluxBootstrapArgs(params *fluxBootstrapParams) ([]string, bool) {
	var args []string
	switch params.Provider {
	case "github":
		args = []string{"bootstrap", "github", "--token-auth"}
	case "gitea", "gitlab":
		args = []string{"bootstrap", params.Provider, "--token-auth"}
	default:
		return nil, false
	}
	args = append(args,
		"--owner="+params.Owner,
		"--repository="+params.Repository,
		"--branch="+params.Branch,
		"--path="+params.Path,
	)
	// Self-hosted servers need the hostname; hosted providers (and scp-style
	// SSH URLs) leave it empty and flux falls back to its default.
	if params.Hostname != "" {
		args = append(args, "--hostname="+params.Hostname)
	}
	return args, true
}
