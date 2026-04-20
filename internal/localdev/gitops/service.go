package gitops

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitea"
)

const defaultRemoteName = "origin"

// PushResult reports the repository state used for the push.
type PushResult struct {
	GitDir     string
	RemoteName string
	RemoteURL  string
	Branch     string
}

// Service manages GitOps repository operations for the local-dev plugin.
type Service struct {
	executor localdev.Executor
	resolver *localdev.ClusterResolver
	stateDir string
}

// NewService returns a GitOps helper service.
func NewService(executor localdev.Executor, stateDir string) (*Service, error) {
	if executor == nil {
		executor = localdev.NewExecutor()
	}
	resolver, err := localdev.NewClusterResolver()
	if err != nil {
		return nil, err
	}
	return &Service{
		executor: executor,
		resolver: resolver,
		stateDir: stateDir,
	}, nil
}

// Push pushes the cluster GitOps repo to the local Gitea remote.
func (s *Service) Push(ctx context.Context, clusterIdentifier string) (*PushResult, error) {
	cluster, err := s.resolver.Resolve(ctx, clusterIdentifier)
	if err != nil {
		return nil, err
	}

	gitDir := strings.TrimSpace(cluster.Config.GitDir())
	if gitDir == "" {
		gitDir = cluster.Paths.GitOpsDir
	}
	if gitDir == "" {
		return nil, fmt.Errorf("cluster %q does not define a git_dir", clusterIdentifier)
	}
	if _, err := os.Stat(gitDir); err != nil {
		return nil, fmt.Errorf("git_dir %s: %w", gitDir, err)
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

	remoteURL := cluster.Config.ConfiguredGitURL()
	if remoteURL == "" {
		remoteURL = status.LocalRepoURL
	}
	if remoteURL == "" {
		return nil, fmt.Errorf("cluster %q does not define git_url and local gitea repository URL is unavailable", clusterIdentifier)
	}
	if err := s.ensureRemote(ctx, gitDir, defaultRemoteName, remoteURL); err != nil {
		return nil, err
	}

	branch, err := s.currentBranch(ctx, gitDir)
	if err != nil {
		return nil, err
	}
	tokenPath, err := resolveTokenPath(resolveGitTokenPathFromConfig(cluster.Config), status.UserTokenPath, status.UserTokenExists)
	if err != nil {
		return nil, err
	}
	if err := s.gitAuth(ctx, gitDir, status.CAPath, status.Metadata.RepoOwner, tokenPath, "push", "-u", defaultRemoteName, branch); err != nil {
		return nil, err
	}

	return &PushResult{
		GitDir:     gitDir,
		RemoteName: defaultRemoteName,
		RemoteURL:  remoteURL,
		Branch:     branch,
	}, nil
}

// PullRebase synchronizes the local checkout after a Flux bootstrap commit.
//
// Flux bootstrap writes manifests into the local working tree (e.g. the
// flux-system/ directory) without staging them. Git refuses to rebase with
// unstaged changes, so we commit any dirty state before pulling.
func (s *Service) PullRebase(ctx context.Context, clusterIdentifier, gitDir string) (string, error) {
	cluster, err := s.resolver.Resolve(ctx, clusterIdentifier)
	if err != nil {
		return "", err
	}
	giteaService, err := gitea.NewService(s.executor, s.stateDir, gitea.DefaultSettings(""))
	if err != nil {
		return "", err
	}
	status, err := giteaService.Status(ctx)
	if err != nil {
		return "", err
	}

	// Ensure the origin remote exists before attempting to pull.
	// The GitOps directory may not have a remote configured after cluster setup.
	remoteURL := cluster.Config.ConfiguredGitURL()
	if remoteURL == "" {
		remoteURL = status.HostRepoURL // Prefer host-routable URL for Kind clusters
	}
	if remoteURL == "" {
		remoteURL = status.LocalRepoURL
	}
	if remoteURL == "" {
		return "", fmt.Errorf("cluster %q does not define git_url and local gitea repository URL is unavailable", clusterIdentifier)
	}
	if err := s.ensureRemote(ctx, gitDir, defaultRemoteName, remoteURL); err != nil {
		return "", err
	}

	branch, err := s.currentBranch(ctx, gitDir)
	if err != nil {
		return "", err
	}
	tokenPath, err := resolveTokenPath(resolveGitTokenPathFromConfig(cluster.Config), status.UserTokenPath, status.UserTokenExists)
	if err != nil {
		return "", err
	}

	// Stage and commit any local changes left by the previous step (e.g.
	// flux bootstrap) so that git pull --rebase does not fail with
	// "You have unstaged changes".
	if err := s.commitIfDirty(ctx, gitDir); err != nil {
		return "", fmt.Errorf("commit local changes before rebase: %w", err)
	}

	if err := s.gitAuth(ctx, gitDir, status.CAPath, status.Metadata.RepoOwner, tokenPath, "pull", "--rebase", defaultRemoteName, branch); err != nil {
		return "", err
	}
	return branch, nil
}

// commitIfDirty stages all changes in gitDir and commits them when the
// working tree is dirty. It is a no-op when the tree is clean.
func (s *Service) commitIfDirty(ctx context.Context, gitDir string) error {
	// Check for uncommitted changes (staged + unstaged + untracked).
	output, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: "git",
		Dir:  gitDir,
		Args: []string{"status", "--porcelain"},
	})
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(string(output)) == "" {
		return nil // nothing to commit
	}

	if _, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: "git",
		Dir:  gitDir,
		Args: []string{"add", "-A"},
	}); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	if _, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: "git",
		Dir:  gitDir,
		Args: []string{"commit", "-m", "stage local changes before rebase"},
	}); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// CurrentBranch returns the currently checked-out branch or main when detached.
func (s *Service) CurrentBranch(ctx context.Context, gitDir string) (string, error) {
	return s.currentBranch(ctx, gitDir)
}

func (s *Service) currentBranch(ctx context.Context, gitDir string) (string, error) {
	output, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: "git",
		Dir:  gitDir,
		Args: []string{"branch", "--show-current"},
	})
	if err != nil {
		return "", fmt.Errorf("determine current git branch: %w", err)
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		branch = "main"
	}
	return branch, nil
}

func (s *Service) ensureRemote(ctx context.Context, gitDir, remoteName, remoteURL string) error {
	output, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: "git",
		Dir:  gitDir,
		Args: []string{"remote", "get-url", remoteName},
	})
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "no such remote") || strings.Contains(lower, "not a git repository") {
			if strings.Contains(lower, "not a git repository") {
				return fmt.Errorf("git_dir %s is not a git repository", gitDir)
			}
			if _, err := s.executor.Run(ctx, localdev.RunOptions{
				Name: "git",
				Dir:  gitDir,
				Args: []string{"remote", "add", remoteName, remoteURL},
			}); err != nil {
				return fmt.Errorf("add git remote %s: %w", remoteName, err)
			}
			return nil
		}
		return fmt.Errorf("inspect git remote %s: %w", remoteName, err)
	}

	currentURL := strings.TrimSpace(string(output))
	if currentURL == remoteURL {
		return nil
	}

	if _, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: "git",
		Dir:  gitDir,
		Args: []string{"remote", "set-url", remoteName, remoteURL},
	}); err != nil {
		return fmt.Errorf("set git remote %s: %w", remoteName, err)
	}
	return nil
}

func (s *Service) gitAuth(ctx context.Context, gitDir, caPath, username, tokenPath string, gitArgs ...string) error {
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("read Gitea token %s: %w", tokenPath, err)
	}

	authHeader := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+strings.TrimSpace(string(tokenBytes))))
	args := []string{
		"-c", "http.sslCAInfo=" + caPath,
		"-c", "http.extraHeader=" + authHeader,
	}
	args = append(args, gitArgs...)

	if _, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: "git",
		Dir:  gitDir,
		Args: args,
	}); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(gitArgs, " "), err)
	}
	return nil
}

func resolveTokenPath(configuredPath, fallbackPath string, fallbackExists bool) (string, error) {
	if configuredPath != "" {
		if _, err := os.Stat(configuredPath); err != nil {
			return "", fmt.Errorf("configured git token %s: %w", configuredPath, err)
		}
		return configuredPath, nil
	}
	if !fallbackExists {
		return "", fmt.Errorf("missing Gitea user token at %s; run `opencenter local gitea up` first", fallbackPath)
	}
	if _, err := os.Stat(fallbackPath); err != nil {
		return "", fmt.Errorf("gitea user token %s: %w", fallbackPath, err)
	}
	return fallbackPath, nil
}

// resolveGitTokenPathFromConfig extracts the token file path from the GitOps config.
func resolveGitTokenPathFromConfig(cfg *v2.Config) string {
	if cfg.OpenCenter.GitOps.Auth.Token != nil {
		return strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.TokenFile)
	}
	return ""
}
