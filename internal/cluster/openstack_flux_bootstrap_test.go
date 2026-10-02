package cluster

import (
	"os"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveFluxBootstrapParams_GitHub(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "prod-east"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:my-org/my-repo.git"
	cfg.OpenCenter.GitOps.Repository.Branch = "main"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: "/tmp/token",
		Owner:     "my-org",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	assert.Equal(t, "github", params.Provider)
	assert.Equal(t, "my-org", params.Owner)
	assert.Equal(t, "my-repo", params.Repository)
	assert.Equal(t, "main", params.Branch)
	assert.Equal(t, "clusters/prod-east", params.Path)
	assert.Equal(t, "/tmp/token", params.TokenFile)
}

func TestResolveFluxBootstrapParams_Gitea(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "dev-cluster"
	cfg.OpenCenter.GitOps.Repository.URL = "https://gitea.example.com/team/infra.git"
	cfg.OpenCenter.GitOps.Repository.Branch = "develop"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "gitea",
		TokenFile: "/tmp/gitea-token",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	assert.Equal(t, "gitea", params.Provider)
	assert.Equal(t, "team", params.Owner)
	assert.Equal(t, "infra", params.Repository)
	assert.Equal(t, "develop", params.Branch)
	assert.Equal(t, "clusters/dev-cluster", params.Path)
	assert.Equal(t, "gitea.example.com", params.Hostname)
}

func TestResolveFluxBootstrapParams_SelfHostedGiteaWithPort(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "oc-baremetal"
	cfg.OpenCenter.GitOps.Repository.URL = "https://192.168.123.1:3001/newuser/test-repo.git"
	cfg.OpenCenter.GitOps.Repository.Branch = "main"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "gitea",
		TokenFile: "/tmp/gitea-token",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	assert.Equal(t, "newuser", params.Owner)
	assert.Equal(t, "test-repo", params.Repository)
	// Self-hosted host with port must be derived (host:port) so flux targets
	// the local Gitea instead of the default gitea.com.
	assert.Equal(t, "192.168.123.1:3001", params.Hostname)
}

func TestResolveFluxBootstrapParams_SSHGiteaNoHostname(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@gitea.example.com:team/infra.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "gitea",
		TokenFile: "/tmp/gitea-token",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	// scp-style SSH URLs convey the host via --ssh-hostname, not --hostname.
	assert.Empty(t, params.Hostname)
}

func TestFluxGitHostname(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https", "https://gitea.example.com/team/infra.git", "gitea.example.com"},
		{"https with port", "https://192.168.123.1:3001/newuser/test-repo.git", "192.168.123.1:3001"},
		{"http with port", "http://git.local:8080/org/repo.git", "git.local:8080"},
		{"ssh scheme with port", "ssh://git@gitlab.example.com:2222/group/repo.git", "gitlab.example.com:2222"},
		{"scp-style ssh", "git@github.com:my-org/my-repo.git", ""},
		{"empty", "", ""},
		{"garbage", "not a url", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fluxGitHostname(tt.url))
		})
	}
}

func TestResolveFluxBootstrapParams_GitLab(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "staging"
	cfg.OpenCenter.GitOps.Repository.URL = "https://gitlab.com/group/subgroup/repo.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "gitlab",
		TokenFile: "/tmp/gl-token",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	assert.Equal(t, "gitlab", params.Provider)
	assert.Equal(t, "group/subgroup", params.Owner)
	assert.Equal(t, "repo", params.Repository)
	assert.Equal(t, "main", params.Branch) // default
	assert.Equal(t, "clusters/staging", params.Path)
}

func TestResolveFluxBootstrapParams_OwnerOverride(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:url-owner/repo.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: "/tmp/token",
		Owner:     "config-owner",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	assert.Equal(t, "config-owner", params.Owner, "token.owner should override URL-derived owner")
}

func TestResolveFluxBootstrapParams_CustomPath(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:org/repo.git"
	cfg.OpenCenter.GitOps.Repository.Path = "custom/path/to/cluster"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: "/tmp/token",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	assert.Equal(t, "custom/path/to/cluster", params.Path)
}

func TestResolveFluxBootstrapParams_DefaultBranch(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:org/repo.git"
	// Branch intentionally left empty
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: "/tmp/token",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	assert.Equal(t, "main", params.Branch, "should default to main when branch is empty")
}

func TestResolveFluxBootstrapParams_MissingTokenConfig(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:org/repo.git"
	// No token config

	_, err := resolveFluxBootstrapParams(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gitops.auth.token must be configured")
}

func TestResolveFluxBootstrapParams_MissingProvider(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:org/repo.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		TokenFile: "/tmp/token",
		// Provider intentionally empty
	}

	_, err := resolveFluxBootstrapParams(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gitops.auth.token.provider is required")
}

func TestResolveFluxBootstrapParams_UnsupportedProvider(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:org/repo.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "bitbucket",
		TokenFile: "/tmp/token",
	}

	_, err := resolveFluxBootstrapParams(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported gitops.auth.token.provider")
}

func TestResolveFluxBootstrapParams_MissingRepoURL(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	// No repository URL
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: "/tmp/token",
	}

	_, err := resolveFluxBootstrapParams(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gitops.repository.url must be configured")
}

func TestResolveFluxBootstrapParams_MissingToken(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:org/repo.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider: "github",
		// No token or token_file
	}

	_, err := resolveFluxBootstrapParams(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token_file or gitops.auth.token.token must be configured")
}

func TestResolveFluxBootstrapParams_InlineToken(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Cluster.ClusterName = "test"
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:org/repo.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider: "github",
		Token:    "ghp_inline_token_value",
	}

	params, err := resolveFluxBootstrapParams(cfg)
	require.NoError(t, err)
	assert.Equal(t, "", params.TokenFile, "token_file should be empty when using inline token")
}

func TestResolveFluxBootstrapParams_MissingClusterName(t *testing.T) {
	cfg := &v2.Config{}
	// No cluster name
	cfg.OpenCenter.GitOps.Repository.URL = "git@github.com:org/repo.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: "/tmp/token",
	}

	_, err := resolveFluxBootstrapParams(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster name must be set")
}

func TestParseFluxGitURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "SSH format",
			url:       "git@github.com:my-org/my-repo.git",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "HTTPS format",
			url:       "https://github.com/my-org/my-repo.git",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "HTTPS without .git",
			url:       "https://github.com/my-org/my-repo",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "SSH protocol format",
			url:       "ssh://git@github.com/my-org/my-repo.git",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "nested groups (GitLab)",
			url:       "https://gitlab.com/group/subgroup/repo.git",
			wantOwner: "group/subgroup",
			wantRepo:  "repo",
		},
		{
			name:      "self-hosted Gitea",
			url:       "https://gitea.example.com:3000/team/infra.git",
			wantOwner: "team",
			wantRepo:  "infra",
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "URL without owner/repo",
			url:     "https://github.com/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseFluxGitURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestFluxBootstrapPlanCommands_GitHub(t *testing.T) {
	params := &fluxBootstrapParams{
		Provider:   "github",
		Owner:      "my-org",
		Repository: "my-repo",
		Branch:     "main",
		Path:       "clusters/prod",
	}

	commands := fluxBootstrapPlanCommands(params)
	require.Len(t, commands, 1)
	assert.Equal(t, "flux", commands[0].Name)
	assert.Contains(t, commands[0].Args, "bootstrap")
	assert.Contains(t, commands[0].Args, "github")
	assert.Contains(t, commands[0].Args, "--token-auth")
	assert.Contains(t, commands[0].Args, "--owner=my-org")
	assert.Contains(t, commands[0].Args, "--repository=my-repo")
	assert.Contains(t, commands[0].Args, "--branch=main")
	assert.Contains(t, commands[0].Args, "--path=clusters/prod")
}

func TestFluxBootstrapPlanCommands_Gitea(t *testing.T) {
	params := &fluxBootstrapParams{
		Provider:   "gitea",
		Owner:      "team",
		Repository: "infra",
		Branch:     "develop",
		Path:       "clusters/dev",
		Hostname:   "192.168.123.1",
	}

	commands := fluxBootstrapPlanCommands(params)
	require.Len(t, commands, 1)
	assert.Equal(t, "flux", commands[0].Name)
	assert.Contains(t, commands[0].Args, "bootstrap")
	assert.Contains(t, commands[0].Args, "gitea")
	assert.Contains(t, commands[0].Args, "--token-auth")
	assert.Contains(t, commands[0].Args, "--owner=team")
	// Self-hosted Gitea must pass --hostname so flux targets the local server
	// instead of the default gitea.com.
	assert.Contains(t, commands[0].Args, "--hostname=192.168.123.1")
}

func TestFluxBootstrapPlanCommands_GiteaNoHostname(t *testing.T) {
	params := &fluxBootstrapParams{
		Provider:   "gitea",
		Owner:      "team",
		Repository: "infra",
		Branch:     "develop",
		Path:       "clusters/dev",
		// Hostname intentionally empty (hosted gitea.com)
	}

	commands := fluxBootstrapPlanCommands(params)
	require.Len(t, commands, 1)
	assert.NotContains(t, commands[0].Args, "--hostname")
}

func TestFluxBootstrapPlanCommands_GitLab(t *testing.T) {
	params := &fluxBootstrapParams{
		Provider:   "gitlab",
		Owner:      "group/subgroup",
		Repository: "repo",
		Branch:     "main",
		Path:       "clusters/staging",
	}

	commands := fluxBootstrapPlanCommands(params)
	require.Len(t, commands, 1)
	assert.Equal(t, "flux", commands[0].Name)
	assert.Contains(t, commands[0].Args, "bootstrap")
	assert.Contains(t, commands[0].Args, "gitlab")
	assert.Contains(t, commands[0].Args, "--owner=group/subgroup")
}

func TestFluxBootstrapPlanCommands_UnsupportedProvider(t *testing.T) {
	params := &fluxBootstrapParams{
		Provider: "bitbucket",
	}

	commands := fluxBootstrapPlanCommands(params)
	assert.Nil(t, commands)
}

func TestResolveFluxToken_FromFile(t *testing.T) {
	tokenFile := t.TempDir() + "/token"
	err := os.WriteFile(tokenFile, []byte("ghp_test_token_123\n"), 0o600)
	require.NoError(t, err)

	cfg := &v2.Config{}
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: tokenFile,
	}

	token, err := resolveFluxToken(cfg)
	require.NoError(t, err)
	assert.Equal(t, "ghp_test_token_123", token)
}

func TestResolveFluxToken_InlineToken(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider: "github",
		Token:    "ghp_inline_token",
	}

	token, err := resolveFluxToken(cfg)
	require.NoError(t, err)
	assert.Equal(t, "ghp_inline_token", token)
}

func TestResolveFluxToken_EmptyFile(t *testing.T) {
	tokenFile := t.TempDir() + "/token"
	err := os.WriteFile(tokenFile, []byte("  \n"), 0o600)
	require.NoError(t, err)

	cfg := &v2.Config{}
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: tokenFile,
	}

	_, err = resolveFluxToken(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty")
}

func TestResolveFluxToken_MissingFile(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: "/nonexistent/path/token",
	}

	_, err := resolveFluxToken(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading token file")
}

func TestResolveFluxToken_NoTokenConfig(t *testing.T) {
	cfg := &v2.Config{}

	_, err := resolveFluxToken(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestResolveFluxToken_NoTokenOrFile(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider: "github",
	}

	_, err := resolveFluxToken(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no token available")
}

func TestResolveFluxToken_PlaceholderToken(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider: "github",
		Token:    "CHANGEME",
	}

	_, err := resolveFluxToken(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "placeholder value")
}
