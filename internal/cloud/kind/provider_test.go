package kind

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls    []runnerCall
	handlers map[string]func(args []string) ([]byte, error)
}

func (f *fakeRunner) Run(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, runnerCall{name: name, args: args})
	key := name + " " + strings.Join(args, " ")
	if handler, ok := f.handlers[key]; ok {
		return handler(args)
	}
	return nil, fmt.Errorf("unexpected command: %s", key)
}

func TestProviderClusterExists(t *testing.T) {
	runner := &fakeRunner{
		handlers: map[string]func(args []string) ([]byte, error){
			"kind get clusters": func(args []string) ([]byte, error) {
				return []byte("dev\nworkload\n"), nil
			},
		},
	}

	provider := NewProviderWithRunner(runner)
	exists, err := provider.ClusterExists(context.Background(), "workload", nil)
	if err != nil {
		t.Fatalf("ClusterExists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected cluster to exist")
	}
}

func TestProviderCreateClusterSkipsExistingCluster(t *testing.T) {
	runner := &fakeRunner{
		handlers: map[string]func(args []string) ([]byte, error){
			"kind get clusters": func(args []string) ([]byte, error) {
				return []byte("dev\n"), nil
			},
		},
	}

	provider := NewProviderWithRunner(runner)
	if err := provider.CreateCluster(context.Background(), "dev", "/tmp/kind-config.yaml", nil); err != nil {
		t.Fatalf("CreateCluster returned error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected one command call, got %d", len(runner.calls))
	}
}

func TestProviderExportKubeconfig(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := dir + "/kubeconfig.yaml"
	runner := &fakeRunner{
		handlers: map[string]func(args []string) ([]byte, error){
			"kind export kubeconfig --name dev --kubeconfig " + kubeconfigPath: func(args []string) ([]byte, error) {
				return []byte("ok"), nil
			},
		},
	}

	provider := NewProviderWithRunner(runner)
	if err := provider.ExportKubeconfig(context.Background(), "dev", kubeconfigPath, nil); err != nil {
		t.Fatalf("ExportKubeconfig returned error: %v", err)
	}
}

func TestProviderWaitReady(t *testing.T) {
	runner := &fakeRunner{
		handlers: map[string]func(args []string) ([]byte, error){
			"kubectl --kubeconfig /tmp/kubeconfig.yaml cluster-info": func(args []string) ([]byte, error) {
				return []byte("ready"), nil
			},
			"kubectl --kubeconfig /tmp/kubeconfig.yaml config view --minify -o jsonpath={.clusters[0].cluster.server}": func(args []string) ([]byte, error) {
				return []byte("https://127.0.0.1:6443"), nil
			},
		},
	}

	provider := NewProviderWithRunner(runner)
	endpoint, err := provider.WaitReady(context.Background(), "/tmp/kubeconfig.yaml")
	if err != nil {
		t.Fatalf("WaitReady returned error: %v", err)
	}
	if endpoint != "https://127.0.0.1:6443" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
}

func TestResolveRuntime(t *testing.T) {
	t.Setenv("CONTAINER_RUNTIME", "")
	t.Setenv("KIND_EXPERIMENTAL_PROVIDER", "")
	if got := ResolveRuntime(""); got != "docker" {
		t.Fatalf("expected docker default, got %q", got)
	}

	t.Setenv("CONTAINER_RUNTIME", "podman")
	if got := ResolveRuntime(""); got != "podman" {
		t.Fatalf("expected podman from env, got %q", got)
	}

	if got := ResolveRuntime("docker"); got != "docker" {
		t.Fatalf("expected explicit runtime to win, got %q", got)
	}
}

func TestBuildEnvironment(t *testing.T) {
	t.Setenv("PATH", os.Getenv("PATH"))
	env := BuildEnvironment("podman")
	if env["KIND_EXPERIMENTAL_PROVIDER"] != "podman" {
		t.Fatalf("expected podman provider env, got %#v", env)
	}
	if env["PATH"] == "" {
		t.Fatal("expected PATH to be preserved")
	}
}

func TestDeleteClusterRemovesExistingCluster(t *testing.T) {
	runner := &fakeRunner{
		handlers: map[string]func(args []string) ([]byte, error){
			"kind get clusters": func(args []string) ([]byte, error) {
				return []byte("dev\nworkload\n"), nil
			},
			"kind delete cluster --name workload": func(args []string) ([]byte, error) {
				return []byte("Deleting cluster \"workload\" ...\n"), nil
			},
		},
	}

	provider := NewProviderWithRunner(runner)
	if err := provider.DeleteCluster(context.Background(), "workload", nil); err != nil {
		t.Fatalf("DeleteCluster returned error: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 command calls (get clusters + delete), got %d", len(runner.calls))
	}
	if runner.calls[1].name != "kind" || runner.calls[1].args[0] != "delete" {
		t.Fatalf("expected delete cluster call, got: %s %v", runner.calls[1].name, runner.calls[1].args)
	}
}

func TestDeleteClusterSkipsWhenAbsent(t *testing.T) {
	runner := &fakeRunner{
		handlers: map[string]func(args []string) ([]byte, error){
			"kind get clusters": func(args []string) ([]byte, error) {
				return []byte("other-cluster\n"), nil
			},
		},
	}

	provider := NewProviderWithRunner(runner)
	if err := provider.DeleteCluster(context.Background(), "workload", nil); err != nil {
		t.Fatalf("DeleteCluster returned error: %v", err)
	}

	// Only the existence check should have been called, no delete.
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command call (get clusters only), got %d", len(runner.calls))
	}
}

func TestDeleteClusterPropagatesDeleteError(t *testing.T) {
	runner := &fakeRunner{
		handlers: map[string]func(args []string) ([]byte, error){
			"kind get clusters": func(args []string) ([]byte, error) {
				return []byte("workload\n"), nil
			},
			"kind delete cluster --name workload": func(args []string) ([]byte, error) {
				return nil, fmt.Errorf("command failed: kind delete cluster: exit status 1")
			},
		},
	}

	provider := NewProviderWithRunner(runner)
	err := provider.DeleteCluster(context.Background(), "workload", nil)
	if err == nil {
		t.Fatal("expected DeleteCluster to return an error")
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
