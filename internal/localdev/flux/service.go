package flux

import (
	"context"
	"fmt"
	"os"
	"strings"

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

// Bootstrap runs `flux bootstrap git --token-auth` against the attached local
// Gitea repo.
//
// We use the generic `flux bootstrap git` subcommand rather than
// `flux bootstrap gitea` because the Gitea provider in go-git-providers
// panics on non-standard ports (e.g. 172.16.0.146:3001). The generic git
// subcommand handles arbitrary HTTPS URLs correctly.
//
// The config field git_token_provider: gitea signals that token-based HTTPS
// auth should be used (not SSH). When git_token is configured, it is treated
// as the authoritative token file path; otherwise the local Gitea state token
// is used as a fallback.
//
// The bootstrap URL uses the host's routable IP (e.g. 172.16.0.146:3001)
// rather than localhost. Podman binds the Gitea port on 0.0.0.0, so this
// IP is reachable from both the macOS host (where the flux CLI clones) and
// from inside the Kind cluster (where the source-controller reconciles).
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

	gitDir := strings.TrimSpace(cluster.Config.GitOps().GitDir)
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

	tokenPath, err := resolveTokenPath(strings.TrimSpace(cluster.Config.OpenCenter.GitOps.GitToken), status.UserTokenPath, status.UserTokenExists)
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

	fluxArgs := []string{
		"bootstrap", "git",
		"--url=" + repoURL,
		"--branch=" + branch,
		"--path=" + bootstrapPath,
		"--token-auth",
		"--username=" + status.Metadata.RepoOwner,
		"--password=" + token,
		"--ca-file=" + status.CAPath,
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
