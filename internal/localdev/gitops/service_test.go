package gitops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	calls    []localdev.RunOptions
}

func (f *fakeExecutor) Run(ctx context.Context, opts localdev.RunOptions) ([]byte, error) {
	f.calls = append(f.calls, opts)
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

func TestPushUsesCleanRemoteAndExplicitCA(t *testing.T) {
	configDir := t.TempDir()
	stateDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "gitops")
	clusterName := "dev-cluster"
	org := "local"
	t.Setenv("OPENCENTER_CONFIG_DIR", configDir)

	writeClusterFixture(t, configDir, clusterName, org, gitDir)
	status := writeGiteaState(t, stateDir)
	authHeader := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(status.Metadata.RepoOwner+":user-token"))

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
				return []byte("10.89.0.11"), nil
			},
			"git remote get-url origin": func(opts localdev.RunOptions) ([]byte, error) {
				return nil, fmt.Errorf("no such remote 'origin'")
			},
			"git remote add origin " + status.LocalRepoURL: func(opts localdev.RunOptions) ([]byte, error) {
				return nil, nil
			},
			"git branch --show-current": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("main"), nil
			},
			fmt.Sprintf("git -c http.sslCAInfo=%s -c http.extraHeader=%s push -u origin main", status.CAPath, authHeader): func(opts localdev.RunOptions) ([]byte, error) {
				return nil, nil
			},
		},
	}

	service, err := NewService(executor, stateDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result, err := service.Push(context.Background(), org+"/"+clusterName)
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if result.RemoteURL != status.LocalRepoURL {
		t.Fatalf("RemoteURL = %q, want %q", result.RemoteURL, status.LocalRepoURL)
	}
	if result.Branch != "main" {
		t.Fatalf("Branch = %q, want main", result.Branch)
	}
}

func TestPushPrefersConfiguredGitURLAndTokenPath(t *testing.T) {
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

	writeClusterFixtureWithGitOps(t, configDir, clusterName, org, gitDir, configuredRepoURL, configuredTokenPath)
	status := writeGiteaState(t, stateDir)
	authHeader := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(status.Metadata.RepoOwner+":configured-user-token"))

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
				return []byte("10.89.0.11"), nil
			},
			"git remote get-url origin": func(opts localdev.RunOptions) ([]byte, error) {
				return nil, fmt.Errorf("no such remote 'origin'")
			},
			"git remote add origin " + configuredRepoURL: func(opts localdev.RunOptions) ([]byte, error) {
				return nil, nil
			},
			"git branch --show-current": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("main"), nil
			},
			fmt.Sprintf("git -c http.sslCAInfo=%s -c http.extraHeader=%s push -u origin main", status.CAPath, authHeader): func(opts localdev.RunOptions) ([]byte, error) {
				return nil, nil
			},
		},
	}

	service, err := NewService(executor, stateDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result, err := service.Push(context.Background(), org+"/"+clusterName)
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if result.RemoteURL != configuredRepoURL {
		t.Fatalf("RemoteURL = %q, want %q", result.RemoteURL, configuredRepoURL)
	}
}

func TestPullRebasePrefersConfiguredGitTokenPath(t *testing.T) {
	configDir := t.TempDir()
	stateDir := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "gitops")
	clusterName := "dev-cluster"
	org := "local"
	configuredTokenPath := filepath.Join(t.TempDir(), "cluster-user.token")
	t.Setenv("OPENCENTER_CONFIG_DIR", configDir)

	if err := os.WriteFile(configuredTokenPath, []byte("configured-user-token"), 0o600); err != nil {
		t.Fatalf("write configured token: %v", err)
	}

	writeClusterFixtureWithGitOps(t, configDir, clusterName, org, gitDir, "", configuredTokenPath)
	status := writeGiteaState(t, stateDir)
	authHeader := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(status.Metadata.RepoOwner+":configured-user-token"))

	// Track the remote URL that ensureRemote adds so we can verify it was called
	var addedRemoteURL string

	executor := &dynamicFakeExecutor{
		t: t,
		handlers: map[string]func(opts localdev.RunOptions) ([]byte, error){
			"podman inspect --format {{.State.Running}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("true"), nil
			},
			"podman inspect --format {{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{\"\\n\"}}{{end}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("podman\nkind\n"), nil
			},
			"podman inspect --format {{with index .NetworkSettings.Networks \"kind\"}}{{.IPAddress}}{{end}} gitea": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("10.89.0.11"), nil
			},
			// ensureRemote checks if origin exists; return error to simulate missing remote
			"git remote get-url origin": func(opts localdev.RunOptions) ([]byte, error) {
				return nil, fmt.Errorf("error: No such remote 'origin'")
			},
			"git branch --show-current": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte("main"), nil
			},
			"git status --porcelain": func(opts localdev.RunOptions) ([]byte, error) {
				return []byte(""), nil
			},
			fmt.Sprintf("git -c http.sslCAInfo=%s -c http.extraHeader=%s pull --rebase origin main", status.CAPath, authHeader): func(opts localdev.RunOptions) ([]byte, error) {
				return nil, nil
			},
		},
		prefixHandlers: map[string]func(opts localdev.RunOptions) ([]byte, error){
			// Handle git remote add with dynamic URL (depends on hostRoutableIP())
			"git remote add origin ": func(opts localdev.RunOptions) ([]byte, error) {
				addedRemoteURL = opts.Args[3]
				return nil, nil
			},
		},
	}

	service, err := NewService(executor, stateDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	branch, err := service.PullRebase(context.Background(), org+"/"+clusterName, gitDir)
	if err != nil {
		t.Fatalf("PullRebase() error = %v", err)
	}

	if branch != "main" {
		t.Fatalf("Branch = %q, want main", branch)
	}

	// Verify ensureRemote was called to add the origin
	if addedRemoteURL == "" {
		t.Fatal("expected ensureRemote to add origin remote, but it was not called")
	}
	// The URL should be the host-routable URL (computed from hostRoutableIP())
	if !strings.Contains(addedRemoteURL, ":3001/newuser/test-repo.git") {
		t.Fatalf("added remote URL %q does not look like expected Gitea URL", addedRemoteURL)
	}
}

// dynamicFakeExecutor extends fakeExecutor with prefix-based matching for dynamic commands
type dynamicFakeExecutor struct {
	t              *testing.T
	handlers       map[string]func(opts localdev.RunOptions) ([]byte, error)
	prefixHandlers map[string]func(opts localdev.RunOptions) ([]byte, error)
	calls          []localdev.RunOptions
}

func (f *dynamicFakeExecutor) Run(ctx context.Context, opts localdev.RunOptions) ([]byte, error) {
	f.calls = append(f.calls, opts)
	key := opts.Name + " " + strings.Join(opts.Args, " ")

	// Check prefix handlers first
	for prefix, handler := range f.prefixHandlers {
		if strings.HasPrefix(key, prefix) {
			return handler(opts)
		}
	}

	// Fall back to exact match handlers
	handler, ok := f.handlers[key]
	if !ok {
		f.t.Fatalf("unexpected command: %s", key)
	}
	return handler(opts)
}

func (f *dynamicFakeExecutor) RunStreaming(ctx context.Context, opts localdev.RunOptions) error {
	_, err := f.Run(ctx, opts)
	return err
}

func writeClusterFixture(t *testing.T, configDir, clusterName, org, gitDir string) {
	writeClusterFixtureWithGitOps(t, configDir, clusterName, org, gitDir, "", "")
}

func writeClusterFixtureWithGitOps(t *testing.T, configDir, clusterName, org, gitDir, gitURL, gitToken string) {
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

	return &gitea.Status{
		Metadata:     metadata,
		BaseURL:      "https://localhost:3001",
		LocalRepoURL: "https://localhost:3001/newuser/test-repo.git",
		HostRepoURL:  "https://192.168.1.100:3001/newuser/test-repo.git",
		KindIP:       "10.89.0.11",
		HostIP:       "192.168.1.100",
		Running:      true,
		CAPath:       layout.CACertPath,
	}
}
