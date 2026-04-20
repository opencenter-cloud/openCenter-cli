package flux

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitea"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitops"
)

// BootstrapResult reports the inputs used for Flux bootstrap.
type BootstrapResult struct {
	GitDir         string
	RepoURL        string
	Branch         string
	KubeconfigPath string
}

// Service manages Flux bootstrap orchestration for local Kind clusters.
type Service struct {
	executor localdev.Executor
	resolver *localdev.ClusterResolver
	gitops   *gitops.Service
	stateDir string
}

// NewService returns a Flux orchestration helper.
func NewService(executor localdev.Executor, stateDir string) (*Service, error) {
	if executor == nil {
		executor = localdev.NewExecutor()
	}
	resolver, err := localdev.NewClusterResolver()
	if err != nil {
		return nil, err
	}
	gitopsService, err := gitops.NewService(executor, stateDir)
	if err != nil {
		return nil, err
	}
	return &Service{
		executor: executor,
		resolver: resolver,
		gitops:   gitopsService,
		stateDir: stateDir,
	}, nil
}

// Bootstrap runs the appropriate `flux bootstrap` command based on the
// configured git_token_provider.
//
// Provider-specific commands:
//   - gitea: Uses `flux bootstrap git` (generic) because the Gitea provider
//     in go-git-providers panics on non-standard ports (e.g. 172.16.0.146:3001).
//   - github: Uses `flux bootstrap github --owner --repository --token-auth --personal`
//   - gitlab: Uses `flux bootstrap gitlab --owner --repository --token-auth`
//
// The config field git_token_provider signals which bootstrap method to use.
// When git_token is configured, it is treated as the authoritative token file
// path; otherwise the local Gitea state token is used as a fallback (for gitea
// provider only).
//
// For local Kind clusters with Gitea, the bootstrap URL uses the host's
// routable IP (e.g. 172.16.0.146:3001) rather than localhost. Podman binds
// the Gitea port on 0.0.0.0, so this IP is reachable from both the macOS
// host (where the flux CLI clones) and from inside the Kind cluster (where
// the source-controller reconciles).
func (s *Service) Bootstrap(ctx context.Context, clusterIdentifier string) (*BootstrapResult, error) {
	cluster, err := s.resolver.Resolve(ctx, clusterIdentifier)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(cluster.Config.OpenCenter.Infrastructure.Provider, "kind") {
		return nil, fmt.Errorf("cluster %q is not a kind cluster", clusterIdentifier)
	}

	giteaService, err := gitea.NewService(s.executor, s.stateDir, gitea.DefaultSettings(""))
	if err != nil {
		return nil, err
	}
	status, err := giteaService.Status(ctx)
	if err != nil {
		return nil, err
	}
	if !status.Running {
		return nil, fmt.Errorf("local gitea is not running")
	}
	if status.KindIP == "" {
		return nil, fmt.Errorf("local gitea is not attached to the kind network; run `opencenter local gitea attach-kind --cluster %s` first", cluster.ClusterName)
	}

	gitDir := strings.TrimSpace(cluster.Config.GitDir())
	if gitDir == "" {
		gitDir = cluster.Paths.GitOpsDir
	}
	if gitDir == "" {
		return nil, fmt.Errorf("cluster %q does not define a git_dir", clusterIdentifier)
	}
	if _, err := os.Stat(cluster.Paths.KubeconfigPath); err != nil {
		return nil, fmt.Errorf("cluster kubeconfig %s: %w", cluster.Paths.KubeconfigPath, err)
	}

	repoURL := cluster.Config.ConfiguredGitURL()
	if repoURL == "" {
		repoURL = status.HostRepoURL
	}
	if repoURL == "" {
		return nil, fmt.Errorf("cluster %q does not define git_url and no routable host IP was found for local Gitea", clusterIdentifier)
	}

	tokenProvider := resolveGitTokenProvider(cluster.Config)
	if tokenProvider == "" {
		tokenProvider = "gitea"
	}

	tokenPath, err := resolveTokenPath(resolveGitTokenFile(cluster.Config), status.UserTokenPath, status.UserTokenExists)
	if err != nil {
		return nil, err
	}

	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("read git token %s: %w", tokenPath, err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	branch, err := s.gitops.CurrentBranch(ctx, gitDir)
	if err != nil {
		return nil, err
	}

	bootstrapPath := filepathForFlux(cluster.ClusterName)
	kubeconfigEnv := map[string]string{"KUBECONFIG": cluster.Paths.KubeconfigPath}

	// Build provider-specific flux bootstrap arguments
	var fluxArgs []string
	switch tokenProvider {
	case "github":
		owner, repo, err := parseGitHubURL(repoURL, resolveGitOwner(cluster.Config))
		if err != nil {
			return nil, fmt.Errorf("parse github url: %w", err)
		}
		fluxArgs = []string{
			"bootstrap", "github",
			"--token-auth",
			"--owner=" + owner,
			"--repository=" + repo,
			"--branch=" + branch,
			"--path=" + bootstrapPath,
			"--personal",
		}
		kubeconfigEnv["GITHUB_TOKEN"] = token

	case "gitlab":
		owner, repo, err := parseGitLabURL(repoURL, resolveGitOwner(cluster.Config))
		if err != nil {
			return nil, fmt.Errorf("parse gitlab url: %w", err)
		}
		fluxArgs = []string{
			"bootstrap", "gitlab",
			"--token-auth",
			"--owner=" + owner,
			"--repository=" + repo,
			"--branch=" + branch,
			"--path=" + bootstrapPath,
		}
		kubeconfigEnv["GITLAB_TOKEN"] = token

	case "gitea":
		// Use generic git bootstrap for Gitea because the Gitea provider
		// in go-git-providers panics on non-standard ports.
		fluxArgs = []string{
			"bootstrap", "git",
			"--url=" + repoURL,
			"--branch=" + branch,
			"--path=" + bootstrapPath,
			"--token-auth",
			"--username=" + status.Metadata.RepoOwner,
			"--password=" + token,
			"--ca-file=" + status.CAPath,
		}

	default:
		return nil, fmt.Errorf("unsupported git_token_provider: %q (supported: github, gitlab, gitea)", tokenProvider)
	}

	if _, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: "flux",
		Dir:  gitDir,
		Env:  kubeconfigEnv,
		Args: fluxArgs,
	}); err != nil {
		return nil, fmt.Errorf("flux bootstrap: %w", err)
	}

	return &BootstrapResult{
		GitDir:         gitDir,
		RepoURL:        repoURL,
		Branch:         branch,
		KubeconfigPath: cluster.Paths.KubeconfigPath,
	}, nil
}

func filepathForFlux(clusterName string) string {
	return "applications/overlays/" + clusterName
}

// parseGitHubURL extracts owner and repository from a GitHub URL.
// If ownerOverride is provided, it takes precedence over the URL-derived owner.
// Supports formats:
//   - https://github.com/owner/repo.git
//   - https://github.com/owner/repo
//   - git@github.com:owner/repo.git
//   - ssh://git@github.com/owner/repo.git
func parseGitHubURL(repoURL, ownerOverride string) (owner, repo string, err error) {
	owner, repo, err = parseGitURL(repoURL)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(ownerOverride) != "" {
		owner = strings.TrimSpace(ownerOverride)
	}
	if owner == "" {
		return "", "", fmt.Errorf("github owner not found in URL %q and git_owner not configured", repoURL)
	}
	return owner, repo, nil
}

// parseGitLabURL extracts owner and repository from a GitLab URL.
// If ownerOverride is provided, it takes precedence over the URL-derived owner.
// Supports formats:
//   - https://gitlab.com/owner/repo.git
//   - https://gitlab.com/owner/repo
//   - git@gitlab.com:owner/repo.git
//   - ssh://git@gitlab.com/owner/repo.git
//   - https://gitlab.example.com/owner/repo.git (self-hosted)
func parseGitLabURL(repoURL, ownerOverride string) (owner, repo string, err error) {
	owner, repo, err = parseGitURL(repoURL)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(ownerOverride) != "" {
		owner = strings.TrimSpace(ownerOverride)
	}
	if owner == "" {
		return "", "", fmt.Errorf("gitlab owner not found in URL %q and git_owner not configured", repoURL)
	}
	return owner, repo, nil
}

// parseGitURL extracts owner and repository from various Git URL formats.
// Returns owner (username or organization) and repository name.
func parseGitURL(repoURL string) (owner, repo string, err error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return "", "", fmt.Errorf("empty repository URL")
	}

	// Handle SSH format: git@host:owner/repo.git
	if strings.HasPrefix(repoURL, "git@") {
		// git@github.com:owner/repo.git -> owner/repo.git
		parts := strings.SplitN(repoURL, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH URL format: %s", repoURL)
		}
		pathPart := strings.TrimSuffix(parts[1], ".git")
		return splitOwnerRepo(pathPart)
	}

	// Handle URL formats: https://... or ssh://...
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("parse URL %s: %w", repoURL, err)
	}

	// Clean the path: remove leading slash and .git suffix
	urlPath := strings.TrimPrefix(parsed.Path, "/")
	urlPath = strings.TrimSuffix(urlPath, ".git")

	return splitOwnerRepo(urlPath)
}

// splitOwnerRepo splits "owner/repo" or "owner/group/repo" into owner and repo.
// For nested groups (GitLab), the full path before the last segment is the owner.
func splitOwnerRepo(pathPart string) (owner, repo string, err error) {
	pathPart = strings.Trim(pathPart, "/")
	if pathPart == "" {
		return "", "", fmt.Errorf("empty path in URL")
	}

	parts := strings.Split(pathPart, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("URL path %q does not contain owner/repo", pathPart)
	}

	// Last part is the repo, everything before is the owner (supports nested groups)
	repo = parts[len(parts)-1]
	owner = path.Join(parts[:len(parts)-1]...)

	return owner, repo, nil
}

func resolveTokenPath(configuredPath, fallbackPath string, fallbackExists bool) (string, error) {
	if configuredPath != "" {
		if _, err := os.Stat(configuredPath); err != nil {
			return "", fmt.Errorf("configured git token %s: %w", configuredPath, err)
		}
		return configuredPath, nil
	}
	if !fallbackExists {
		return "", fmt.Errorf("missing Gitea user token at %s", fallbackPath)
	}
	if _, err := os.Stat(fallbackPath); err != nil {
		return "", fmt.Errorf("gitea user token %s: %w", fallbackPath, err)
	}
	return fallbackPath, nil
}

// resolveGitTokenProvider extracts the token provider from the GitOps config.
func resolveGitTokenProvider(cfg *v2.Config) string {
	if cfg.OpenCenter.GitOps.Auth.Token != nil {
		return strings.ToLower(strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Provider))
	}
	return ""
}

// resolveGitTokenFile extracts the token file path from the GitOps config.
func resolveGitTokenFile(cfg *v2.Config) string {
	if cfg.OpenCenter.GitOps.Auth.Token != nil {
		return strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.TokenFile)
	}
	return ""
}

// resolveGitOwner extracts the owner from the GitOps config.
func resolveGitOwner(cfg *v2.Config) string {
	if cfg.OpenCenter.GitOps.Auth.Token != nil {
		return strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Owner)
	}
	return ""
}
