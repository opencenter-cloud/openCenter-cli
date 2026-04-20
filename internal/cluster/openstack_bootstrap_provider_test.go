package cluster

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

type recordedLifecycleCommand struct {
	dir  string
	env  map[string]string
	name string
	args []string
}

type fakeLifecycleRunner struct {
	calls []recordedLifecycleCommand
	onRun func(dir string, env map[string]string, name string, args ...string) ([]byte, error)
}

func (f *fakeLifecycleRunner) Run(ctx context.Context, dir string, env map[string]string, name string, args ...string) ([]byte, error) {
	call := recordedLifecycleCommand{
		dir:  dir,
		env:  copyStringMap(env),
		name: name,
		args: append([]string(nil), args...),
	}
	f.calls = append(f.calls, call)

	if f.onRun != nil {
		return f.onRun(dir, env, name, args...)
	}

	return nil, nil
}

func copyStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func TestOpenStackBootstrapProviderUsesOpenTofuAndNormalizesKubeconfig(t *testing.T) {
	clusterName := "demo"
	cfg := mustNewClusterTestConfig(clusterName, "openstack")

	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(t.TempDir(), "repo")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	targetKubeconfig := filepath.Join(t.TempDir(), "owned", "kubeconfig.yaml")
	fakeRunner := &fakeLifecycleRunner{
		onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "apply" {
				sourceKubeconfig := filepath.Join(clusterDir, "kubeconfig.yaml")
				if err := os.WriteFile(sourceKubeconfig, []byte("apiVersion: v1\n"), 0o600); err != nil {
					t.Fatalf("write source kubeconfig: %v", err)
				}
			}
			return nil, nil
		},
	}

	provider := &openstackBootstrapProvider{runner: fakeRunner}
	steps, err := provider.BuildSteps(&cfg, nil, &BootstrapOptions{KubeconfigPath: targetKubeconfig})
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}

	for _, step := range steps {
		if err := step.Run(context.Background()); err != nil {
			t.Fatalf("step %q failed: %v", step.ID, err)
		}
	}

	if len(fakeRunner.calls) != 2 {
		t.Fatalf("expected 2 lifecycle commands, got %d", len(fakeRunner.calls))
	}
	if fakeRunner.calls[0].name != "opentofu" || len(fakeRunner.calls[0].args) == 0 || fakeRunner.calls[0].args[0] != "init" {
		t.Fatalf("expected first command to be opentofu init, got %#v", fakeRunner.calls[0])
	}
	if fakeRunner.calls[1].name != "opentofu" || len(fakeRunner.calls[1].args) < 2 || fakeRunner.calls[1].args[0] != "apply" || fakeRunner.calls[1].args[1] != "-auto-approve" {
		t.Fatalf("expected second command to be opentofu apply -auto-approve, got %#v", fakeRunner.calls[1])
	}
	if fakeRunner.calls[0].env["OS_AUTH_URL"] != "https://keystone.example.com/v3" {
		t.Fatalf("expected OS_AUTH_URL in env, got %#v", fakeRunner.calls[0].env)
	}
	if fakeRunner.calls[0].env["OS_APPLICATION_CREDENTIAL_ID"] != "app-cred-id" {
		t.Fatalf("expected OS_APPLICATION_CREDENTIAL_ID in env, got %#v", fakeRunner.calls[0].env)
	}
	if fakeRunner.calls[0].env["KUBECONFIG"] != targetKubeconfig {
		t.Fatalf("expected KUBECONFIG %q, got %#v", targetKubeconfig, fakeRunner.calls[0].env)
	}
	if _, err := os.Stat(targetKubeconfig); err != nil {
		t.Fatalf("expected normalized kubeconfig at %s: %v", targetKubeconfig, err)
	}
}

func TestBootstrapServiceOpenStackProvisionInfrastructureHonorsSavedState(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "resume-demo"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	fakeRunner := &fakeLifecycleRunner{
		onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "apply" {
				sourceKubeconfig := filepath.Join(dir, "kubeconfig.yaml")
				if err := os.WriteFile(sourceKubeconfig, []byte("apiVersion: v1\n"), 0o600); err != nil {
					t.Fatalf("write source kubeconfig: %v", err)
				}
			}
			return nil, nil
		},
	}
	bootstrapService.runner = fakeRunner

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	clusterPaths, err := pathResolver.Resolve(ctx, clusterName, organization)
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "repo")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	stateRoot := t.TempDir()
	t.Setenv("OPENCENTER_STATE_DIR", stateRoot)

	statePath := filepath.Join(clusterDir, "logs", "bootstrap-state.json")
	state := bootstrapService.newBootstrapState()
	bootstrapService.setStepStatus(state, "openstack-preflight", bootstrapStatusSuccess, "")
	bootstrapService.setStepStatus(state, "opentofu-init", bootstrapStatusSuccess, "")
	if err := bootstrapService.saveBootstrapState(statePath, state); err != nil {
		t.Fatalf("save bootstrap state: %v", err)
	}

	runtimePaths, err := resolveBootstrapRuntimePaths(&cfg, "", time.Now())
	if err != nil {
		t.Fatalf("resolve bootstrap runtime paths: %v", err)
	}

	result := &BootstrapResult{}
	if err := bootstrapService.provisionInfrastructure(ctx, &cfg, clusterPaths, &BootstrapOptions{
		KubeconfigPath: clusterPaths.KubeconfigPath,
	}, runtimePaths, result); err != nil {
		t.Fatalf("provisionInfrastructure() error = %v", err)
	}

	if len(fakeRunner.calls) != 1 {
		t.Fatalf("expected only the apply command to run after resuming, got %#v", fakeRunner.calls)
	}
	if len(fakeRunner.calls[0].args) == 0 || fakeRunner.calls[0].args[0] != "apply" {
		t.Fatalf("expected resumed command to be opentofu apply, got %#v", fakeRunner.calls[0])
	}
	if _, err := os.Stat(clusterPaths.KubeconfigPath); err != nil {
		t.Fatalf("expected cluster-owned kubeconfig at %s: %v", clusterPaths.KubeconfigPath, err)
	}
	if _, err := os.Stat(runtimePaths.StatePath); err != nil {
		t.Fatalf("expected migrated bootstrap state at %s: %v", runtimePaths.StatePath, err)
	}
}
