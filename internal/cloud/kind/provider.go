package kind

import (
	"context"
	"fmt"
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

	// Under rootless Podman the API server's published port is bound inside the
	// Podman network namespace and is NOT reachable on the host loopback, so the
	// kind-exported kubeconfig (server: https://127.0.0.1:<port>) yields
	// "dial tcp 127.0.0.1:6443: connection refused" from the host. The control-
	// plane container is, however, directly reachable at its container IP on
	// :6443 (the same pattern used for the local Gitea). kind already includes
	// the control-plane container IP in the API server certificate SANs, so
	// rewriting the kubeconfig server to that IP verifies cleanly. Docker
	// publishes to the host loopback normally, so this rewrite only applies to
	// Podman.
	if ResolveRuntime(env["KIND_EXPERIMENTAL_PROVIDER"]) == "podman" {
		if err := p.repointKubeconfigToContainerIP(ctx, clusterName, kubeconfigPath, env); err != nil {
			return fmt.Errorf("repoint kubeconfig to container IP for podman: %w", err)
		}
	}
	return nil
}

// repointKubeconfigToContainerIP rewrites the kubeconfig server URL to the
// kind control-plane container's IP on port 6443, for runtimes (rootless
// Podman) where the host-loopback-published API port is unreachable.
func (p *Provider) repointKubeconfigToContainerIP(ctx context.Context, clusterName, kubeconfigPath string, env map[string]string) error {
	container := clusterName + "-control-plane"
	out, err := p.runner.Run(ctx, env, "podman", "inspect", "-f",
		"{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}", container)
	if err != nil {
		return fmt.Errorf("inspect kind control-plane container %s: %w", container, err)
	}
	var ip string
	for _, field := range strings.Fields(string(out)) {
		if candidate := strings.TrimSpace(field); candidate != "" {
			ip = candidate
			break
		}
	}
	if ip == "" {
		return fmt.Errorf("no container IP found for %s", container)
	}

	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("read kubeconfig: %w", err)
	}
	// Replace the server line's host:port with the container IP on 6443. The
	// kind-exported kubeconfig always uses a 127.0.0.1 (or 0.0.0.0) server URL.
	re := regexp.MustCompile(`(?m)^(\s*server:\s*https://)[^\s]+`)
	newData := re.ReplaceAll(data, []byte(fmt.Sprintf("${1}%s:6443", ip)))
	if err := os.WriteFile(kubeconfigPath, newData, 0o600); err != nil {
		return fmt.Errorf("write kubeconfig: %w", err)
	}
	return nil
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
