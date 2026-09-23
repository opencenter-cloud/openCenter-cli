package kind

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

const defaultReadyPollInterval = 5 * time.Second

// ResolveRuntime normalizes the Kind runtime selection.
// Precedence: explicit value, CONTAINER_RUNTIME, KIND_EXPERIMENTAL_PROVIDER, docker.
func ResolveRuntime(value string) string {
	if v := strings.TrimSpace(value); v != "" {
		return strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv("CONTAINER_RUNTIME")); v != "" {
		return strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv("KIND_EXPERIMENTAL_PROVIDER")); v != "" {
		return strings.ToLower(v)
	}
	return "docker"
}

// BuildEnvironment constructs the environment required for Kind CLI invocations.
func BuildEnvironment(runtime string) map[string]string {
	env := make(map[string]string)

	if ResolveRuntime(runtime) == "podman" {
		env["KIND_EXPERIMENTAL_PROVIDER"] = "podman"
	}
	if path := os.Getenv("PATH"); path != "" {
		env["PATH"] = path
	}

	return env
}

type commandRunner interface {
	Run(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, error)
}

type execRunner struct {
	commandRunner security.CommandRunner
}

func newExecRunner() execRunner {
	return execRunner{commandRunner: security.GetDefaultCommandRunner()}
}

func (r execRunner) Run(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, error) {
	cmd, err := r.commandRunner.PrepareCommandContext(ctx, name, args...)
	if err != nil {
		return nil, fmt.Errorf("preparing command %s: %w", name, err)
	}

	envList := os.Environ()
	for key, value := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = envList

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("command failed: %s %v: %w\nOutput: %s", name, args, err, string(output))
	}

	return output, nil
}

// Provider manages Kind cluster lifecycle operations through the kind and kubectl CLIs.
type Provider struct {
	runner            commandRunner
	readyPollInterval time.Duration
}

func NewProvider() *Provider {
	return &Provider{
		runner:            newExecRunner(),
		readyPollInterval: defaultReadyPollInterval,
	}
}

func NewProviderWithRunner(runner commandRunner) *Provider {
	return &Provider{
		runner:            runner,
		readyPollInterval: defaultReadyPollInterval,
	}
}

func (p *Provider) ClusterExists(ctx context.Context, clusterName string, env map[string]string) (bool, error) {
	output, err := p.runner.Run(ctx, env, "kind", "get", "clusters")
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == clusterName {
			return true, nil
		}
	}

	return false, nil
}

func (p *Provider) CreateCluster(ctx context.Context, clusterName, configPath string, env map[string]string) error {
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("kind config path must be set")
	}

	exists, err := p.ClusterExists(ctx, clusterName, env)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = p.runner.Run(ctx, env, "kind", "create", "cluster", "--name", clusterName, "--config", configPath)
	return err
}

func (p *Provider) ExportKubeconfig(ctx context.Context, clusterName, kubeconfigPath string, env map[string]string) error {
	if strings.TrimSpace(kubeconfigPath) == "" {
		return fmt.Errorf("kubeconfig path must be set")
	}
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0o755); err != nil {
		return fmt.Errorf("create kubeconfig directory: %w", err)
	}

	if _, err := p.runner.Run(ctx, env, "kind", "export", "kubeconfig", "--name", clusterName, "--kubeconfig", kubeconfigPath); err != nil {
		return err
	}

	// Under rootless Podman (including a docker->podman shim) the API server's
	// published port is bound inside the Podman network namespace and is NOT
	// reachable on the host loopback, so the kind-exported kubeconfig
	// (server: https://127.0.0.1:<port>) yields
	// "dial tcp 127.0.0.1:6443: connection refused" for kubectl and flux.
	//
	// We cannot key this off the declared runtime: the runner's `docker` is a
	// Podman shim, so KIND_EXPERIMENTAL_PROVIDER reports "docker" while the real
	// engine is Podman. Instead, probe whether the exported loopback endpoint is
	// actually reachable; if not, repoint the kubeconfig server to the control-
	// plane container's IP on :6443 (kind includes that IP in the API server
	// cert SANs, so it verifies). On real Docker the loopback endpoint IS
	// reachable, so this is a no-op there.
	//
	// Best-effort: repoint failures (unreadable kubeconfig, unresolvable IP) must
	// not fail the export itself, since the exported kubeconfig is already valid
	// on a normal Docker host.
	p.repointKubeconfigIfLoopbackUnreachable(ctx, clusterName, kubeconfigPath, env)
	return nil
}

// repointKubeconfigIfLoopbackUnreachable rewrites the kubeconfig server URL to
// the kind control-plane container's IP on :6443 when the exported loopback
// endpoint is not reachable (rootless Podman / docker-podman shim). No-op when
// the loopback endpoint works (real Docker) or when the container IP cannot be
// determined.
func (p *Provider) repointKubeconfigIfLoopbackUnreachable(ctx context.Context, clusterName, kubeconfigPath string, env map[string]string) {
	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return // kubeconfig not present/readable; nothing to repoint
	}

	serverRe := regexp.MustCompile(`(?m)^(\s*server:\s*)(https://\S+)`)
	m := serverRe.FindSubmatch(data)
	if m == nil {
		return // no server line; nothing to do
	}
	currentServer := string(m[2])

	// If the current endpoint is already reachable, leave it (real Docker case).
	if p.endpointReachable(currentServer) {
		return
	}

	// Determine the control-plane container IP via the active container engine.
	// Try docker first (the shim), then podman, so this works regardless of
	// which binary fronts the engine.
	container := clusterName + "-control-plane"
	ip := ""
	for _, engine := range []string{"docker", "podman"} {
		out, ierr := p.runner.Run(ctx, env, engine, "inspect", "-f",
			"{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}", container)
		if ierr != nil {
			continue
		}
		for _, field := range strings.Fields(string(out)) {
			if candidate := strings.TrimSpace(field); candidate != "" {
				ip = candidate
				break
			}
		}
		if ip != "" {
			break
		}
	}
	if ip == "" {
		// Could not resolve a container IP; leave the kubeconfig unchanged rather
		// than break the working (Docker) case.
		return
	}

	newServer := fmt.Sprintf("https://%s:6443", ip)
	newData := serverRe.ReplaceAll(data, []byte("${1}"+newServer))
	_ = os.WriteFile(kubeconfigPath, newData, 0o600)
}

// endpointReachable reports whether a TLS TCP connection to the host:port of
// the given https URL can be established quickly. It does not validate the
// certificate — it only checks that something is listening (i.e. the loopback
// publish works), which is enough to decide whether to repoint the kubeconfig.
func (p *Provider) endpointReachable(serverURL string) bool {
	u, err := url.Parse(serverURL)
	if err != nil {
		return false
	}
	host := u.Host
	if host == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (p *Provider) WaitReady(ctx context.Context, kubeconfigPath string) (string, error) {
	ticker := time.NewTicker(p.readyPollInterval)
	defer ticker.Stop()

	for {
		if endpoint, ready, err := p.clusterEndpoint(ctx, kubeconfigPath); err != nil {
			return "", err
		} else if ready {
			return endpoint, nil
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for kind cluster to be ready: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (p *Provider) DeleteCluster(ctx context.Context, clusterName string, env map[string]string) error {
	exists, err := p.ClusterExists(ctx, clusterName, env)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	_, err = p.runner.Run(ctx, env, "kind", "delete", "cluster", "--name", clusterName)
	return err
}

func (p *Provider) APIReady(ctx context.Context, kubeconfigPath string) (bool, string, error) {
	endpoint, ready, err := p.clusterEndpoint(ctx, kubeconfigPath)
	return ready, endpoint, err
}

func (p *Provider) clusterEndpoint(ctx context.Context, kubeconfigPath string) (string, bool, error) {
	if strings.TrimSpace(kubeconfigPath) == "" {
		return "", false, fmt.Errorf("kubeconfig path must be set")
	}

	if _, err := p.runner.Run(ctx, nil, "kubectl", "--kubeconfig", kubeconfigPath, "cluster-info"); err != nil {
		return "", false, nil
	}

	output, err := p.runner.Run(ctx, nil, "kubectl", "--kubeconfig", kubeconfigPath, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	if err != nil {
		return "", false, err
	}

	return strings.TrimSpace(string(output)), true, nil
}
