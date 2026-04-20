package flux

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitea"
	"gopkg.in/yaml.v3"
)

type fakeExecutor struct {
	t        *testing.T
	handlers map[string]func(opts localdev.RunOptions) ([]byte, error)
}

func (f *fakeExecutor) Run(ctx context.Context, opts localdev.RunOptions) ([]byte, error) {
	key := opts.Name + " " + strings.Join(opts.Args, " ")
	handler, ok := f.handlers[key]
	if !ok {
		f.t.Fatalf("unexpected command: %s", key)
	}
	return handler(opts)
}

func (f *fakeExecutor) RunStreaming(ctx context.Context, opts localdev.RunOptions) error {
	_, err := f.Run(ctx, opts)
	return err
}

func TestBootstrapUsesHostIPURLAndReconciles(t *testing.T) {
	configDir := t.TempDir()
	stateDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "gitops")
	clusterName := "dev-cluster"
	org := "local"
	t.Setenv("OPENCENTER_CONFIG_DIR", configDir)

	clusterCtx := writeClusterFixture(t, configDir, clusterName, org, gitDir)
	status := writeGiteaState(t, stateDir)

	executor := &fakeExecutor{
		t: t,
		handlers: map[string]func(opts localdev.RunOptions) ([]byte, error){
			"podman inspect --format {{.State.Running}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("true"), nil
			},
			"podman inspect --format {{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{\"\\n\"}}{{end}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("podman\nkind\n"), nil
			},
			"podman inspect --format {{with index .NetworkSettings.Networks \"kind\"}}{{.IPAddress}}{{end}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte(status.KindIP), nil
			},
			"git branch --show-current": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("main"), nil
			},
			// Bootstrap uses the host's routable IP — reachable from both
			// the host (flux CLI clone) and inside the Kind cluster
			// (source-controller reconciliation).
			fmt.Sprintf("flux bootstrap git --url=%s --branch=main --path=applications/overlays/%s --token-auth --username=newuser --password=user-token --ca-file=%s", status.HostRepoURL, clusterName, status.CAPath): func(opts localdev.RunOptions) ([]byte, error) {
				if opts.Env["KUBECONFIG"] != clusterCtx.Paths.KubeconfigPath {
					t.Fatalf("bootstrap KUBECONFIG = %q, want %q", opts.Env["KUBECONFIG"], clusterCtx.Paths.KubeconfigPath)
				}
				return nil, nil
			},
			fmt.Sprintf("git -c http.sslCAInfo=%s -c http.extraHeader=Authorization: Basic bmV3dXNlcjp1c2VyLXRva2Vu pull --rebase origin main", status.CAPath): func(opts localdev.RunOptions) ([]byte, error) {
				return nil, nil
			},
			"flux reconcile source git flux-system -n flux-system": func(opts localdev.RunOptions) ([]byte, error) {
				if opts.Env["KUBECONFIG"] != clusterCtx.Paths.KubeconfigPath {
					t.Fatalf("reconcile KUBECONFIG = %q, want %q", opts.Env["KUBECONFIG"], clusterCtx.Paths.KubeconfigPath)
				}
				return nil, nil
			},
		},
	}

	service, err := NewService(executor, stateDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result, err := service.Bootstrap(context.Background(), org+"/"+clusterName)
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	if result.RepoURL != status.HostRepoURL {
		t.Fatalf("RepoURL = %q, want %q", result.RepoURL, status.HostRepoURL)
	}
	if result.KubeconfigPath != clusterCtx.Paths.KubeconfigPath {
		t.Fatalf("KubeconfigPath = %q, want %q", result.KubeconfigPath, clusterCtx.Paths.KubeconfigPath)
	}
}

func TestBootstrapPrefersConfiguredGitURLAndTokenPath(t *testing.T) {
	configDir := t.TempDir()
	stateDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "gitops")
	clusterName := "dev-cluster"
	org := "local"
	configuredTokenPath := filepath.Join(t.TempDir(), "cluster-user.token")
	configuredRepoURL := "https://172.16.0.200:3001/newuser/test-repo.git"
	t.Setenv("OPENCENTER_CONFIG_DIR", configDir)

	if err := os.WriteFile(configuredTokenPath, []byte("configured-user-token"), 0o600); err != nil {
		t.Fatalf("write configured token: %v", err)
	}

	clusterCtx := writeClusterFixtureWithGitOps(t, configDir, clusterName, org, gitDir, configuredRepoURL, configuredTokenPath)
	status := writeGiteaState(t, stateDir)

	executor := &fakeExecutor{
		t: t,
		handlers: map[string]func(opts localdev.RunOptions) ([]byte, error){
			"podman inspect --format {{.State.Running}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("true"), nil
			},
			"podman inspect --format {{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{\"\\n\"}}{{end}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("podman\nkind\n"), nil
			},
			"podman inspect --format {{with index .NetworkSettings.Networks \"kind\"}}{{.IPAddress}}{{end}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte(status.KindIP), nil
			},
			"git branch --show-current": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("main"), nil
			},
			fmt.Sprintf("flux bootstrap git --url=%s --branch=main --path=applications/overlays/%s --token-auth --username=newuser --password=configured-user-token --ca-file=%s", configuredRepoURL, clusterName, status.CAPath): func(opts localdev.RunOptions) ([]byte, error) {
				if opts.Env["KUBECONFIG"] != clusterCtx.Paths.KubeconfigPath {
					t.Fatalf("bootstrap KUBECONFIG = %q, want %q", opts.Env["KUBECONFIG"], clusterCtx.Paths.KubeconfigPath)
				}
				return nil, nil
			},
		},
	}

	service, err := NewService(executor, stateDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result, err := service.Bootstrap(context.Background(), org+"/"+clusterName)
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	if result.RepoURL != configuredRepoURL {
		t.Fatalf("RepoURL = %q, want %q", result.RepoURL, configuredRepoURL)
	}
}

func writeClusterFixture(t *testing.T, configDir, clusterName, org, gitDir string) *localdev.ClusterContext {
	return writeClusterFixtureWithGitOps(t, configDir, clusterName, org, gitDir, "", "")
}

func writeClusterFixtureWithGitOps(t *testing.T, configDir, clusterName, org, gitDir, gitURL, gitToken string) *localdev.ClusterContext {
	t.Helper()

	cfg, err := v2.NewV2Default(clusterName, "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.Meta.Organization = org
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	if gitURL != "" {
		cfg.OpenCenter.GitOps.Repository.URL = gitURL
	}
	if gitToken != "" {
		if cfg.OpenCenter.GitOps.Auth.Token == nil {
			cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{}
		}
		cfg.OpenCenter.GitOps.Auth.Token.TokenFile = gitToken
	}
	cfg.OpenCenter.Infrastructure.Kind.Runtime = "podman"

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	orgDir := filepath.Join(configDir, "clusters", org)
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clusterDir, "kubeconfig.yaml"), []byte("apiVersion: v1\n"), 0o644); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orgDir, "."+clusterName+"-config.yaml"), data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolver, err := localdev.NewClusterResolver()
	if err != nil {
		t.Fatalf("NewClusterResolver() error = %v", err)
	}
	clusterCtx, err := resolver.Resolve(context.Background(), org+"/"+clusterName)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	return clusterCtx
}

func writeGiteaState(t *testing.T, stateDir string) *gitea.Status {
	t.Helper()

	layout, err := localdev.ResolveLayout(stateDir)
	if err != nil {
		t.Fatalf("ResolveLayout() error = %v", err)
	}
	if err := layout.Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	metadata := gitea.Metadata{
		Runtime:       "podman",
		Image:         "docker.gitea.com/gitea:1.24.5",
		ContainerName: "gitea",
		HTTPPort:      3000,
		HTTPSPort:     3001,
		SSHPort:       2222,
		AdminUser:     "admin",
		RepoOwner:     "newuser",
		RepoName:      "test-repo",
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(layout.MetadataPath, data, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if err := os.WriteFile(layout.UserTokenPath, []byte("user-token"), 0o600); err != nil {
		t.Fatalf("write user token: %v", err)
	}
	if err := os.WriteFile(layout.CACertPath, []byte("ca"), 0o644); err != nil {
		t.Fatalf("write ca: %v", err)
	}

	// Use the real host IP so the test matches what Status() produces
	// at runtime (hostRoutableIP reads real network interfaces).
	hostIP := resolveTestHostIP(t)
	hostRepoURL := fmt.Sprintf("https://%s:3001/newuser/test-repo.git", hostIP)

	return &gitea.Status{
		Metadata:     metadata,
		BaseURL:      "https://localhost:3001",
		LocalRepoURL: "https://localhost:3001/newuser/test-repo.git",
		HostRepoURL:  hostRepoURL,
		KindIP:       "10.89.0.11",
		HostIP:       hostIP,
		Running:      true,
		CAPath:       layout.CACertPath,
	}
}

// resolveTestHostIP returns the first non-loopback IPv4 address, matching
// the logic in gitea.hostRoutableIP(). Tests use this to build expected
// URLs that agree with the real Status() output.
func resolveTestHostIP(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatalf("net.InterfaceAddrs() error = %v", err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		if ip.IsLoopback() || ip.To4() == nil {
			continue
		}
		return ip.String()
	}
	t.Fatal("no routable IPv4 address found for test")
	return ""
}


func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "https github url with .git",
			url:       "https://github.com/my-org/my-repo.git",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "https github url without .git",
			url:       "https://github.com/my-org/my-repo",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "ssh github url",
			url:       "git@github.com:my-org/my-repo.git",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "ssh protocol github url",
			url:       "ssh://git@github.com/my-org/my-repo.git",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "gitlab nested group",
			url:       "https://gitlab.com/group/subgroup/my-repo.git",
			wantOwner: "group/subgroup",
			wantRepo:  "my-repo",
		},
		{
			name:      "self-hosted gitlab",
			url:       "https://gitlab.example.com/team/project.git",
			wantOwner: "team",
			wantRepo:  "project",
		},
		{
			name:    "empty url",
			url:     "",
			wantErr: true,
		},
		{
			name:    "url without repo path",
			url:     "https://github.com/",
			wantErr: true,
		},
		{
			name:    "url with only owner",
			url:     "https://github.com/owner",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseGitURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("parseGitURL() owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseGitURL() repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		ownerOverride string
		wantOwner     string
		wantRepo      string
		wantErr       bool
	}{
		{
			name:      "github url extracts owner",
			url:       "https://github.com/my-org/my-repo.git",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:          "owner override takes precedence",
			url:           "https://github.com/url-org/my-repo.git",
			ownerOverride: "override-org",
			wantOwner:     "override-org",
			wantRepo:      "my-repo",
		},
		{
			name:          "owner override with whitespace is trimmed",
			url:           "https://github.com/url-org/my-repo.git",
			ownerOverride: "  trimmed-org  ",
			wantOwner:     "trimmed-org",
			wantRepo:      "my-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseGitHubURL(tt.url, tt.ownerOverride)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitHubURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("parseGitHubURL() owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseGitHubURL() repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

func TestParseGitLabURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		ownerOverride string
		wantOwner     string
		wantRepo      string
		wantErr       bool
	}{
		{
			name:      "gitlab url extracts owner",
			url:       "https://gitlab.com/my-group/my-project.git",
			wantOwner: "my-group",
			wantRepo:  "my-project",
		},
		{
			name:      "gitlab nested groups",
			url:       "https://gitlab.com/parent/child/grandchild/my-project.git",
			wantOwner: "parent/child/grandchild",
			wantRepo:  "my-project",
		},
		{
			name:          "owner override takes precedence over nested groups",
			url:           "https://gitlab.com/parent/child/my-project.git",
			ownerOverride: "flat-owner",
			wantOwner:     "flat-owner",
			wantRepo:      "my-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseGitLabURL(tt.url, tt.ownerOverride)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitLabURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("parseGitLabURL() owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseGitLabURL() repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}
