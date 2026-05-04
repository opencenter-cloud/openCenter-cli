package cluster

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

type fakeTalosRuntime struct {
	calls []string
}

func (f *fakeTalosRuntime) ReadInventory(ctx context.Context) error {
	f.calls = append(f.calls, "talos-read-inventory")
	return nil
}

func (f *fakeTalosRuntime) GenerateSecrets(ctx context.Context) error {
	f.calls = append(f.calls, "talos-generate-secrets")
	return nil
}

func (f *fakeTalosRuntime) ApplyMachineConfigs(ctx context.Context) error {
	f.calls = append(f.calls, "talos-apply-machine-configs")
	return nil
}

func (f *fakeTalosRuntime) BootstrapControlPlane(ctx context.Context) error {
	f.calls = append(f.calls, "talos-bootstrap-controlplane")
	return nil
}

func (f *fakeTalosRuntime) ExportTalosConfig(ctx context.Context) error {
	f.calls = append(f.calls, "talos-export-talosconfig")
	return nil
}

func (f *fakeTalosRuntime) ExportKubeconfig(ctx context.Context) error {
	f.calls = append(f.calls, "talos-export-kubeconfig")
	return nil
}

func (f *fakeTalosRuntime) WaitReady(ctx context.Context) error {
	f.calls = append(f.calls, "talos-wait-ready")
	return nil
}

func TestTalosBootstrapProviderBuildsExactStepSequence(t *testing.T) {
	cfg, clusterPaths := talosBootstrapTestConfig(t, "talos-sequence", "opencenter")
	fakeRunner := &fakeLifecycleRunner{}
	fakeRuntime := &fakeTalosRuntime{}
	restore := replaceTalosRuntimeFactory(t, fakeRuntime)
	defer restore()

	provider := newTalosBootstrapProvider(fakeRunner)
	steps, err := provider.BuildSteps(&cfg, clusterPaths, &BootstrapOptions{KubeconfigPath: clusterPaths.KubeconfigPath})
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}

	wantIDs := []string{
		"talos-preflight",
		"opentofu-init",
		"opentofu-apply",
		"talos-read-inventory",
		"talos-generate-secrets",
		"talos-apply-machine-configs",
		"talos-bootstrap-controlplane",
		"talos-export-talosconfig",
		"talos-export-kubeconfig",
		"talos-wait-ready",
	}
	if got := bootstrapStepIDs(steps); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("BuildSteps() IDs = %v, want %v", got, wantIDs)
	}

	for _, step := range steps {
		if err := step.Run(context.Background()); err != nil {
			t.Fatalf("step %q failed: %v", step.ID, err)
		}
	}

	if len(fakeRunner.calls) != 2 {
		t.Fatalf("expected OpenTofu init/apply commands, got %d", len(fakeRunner.calls))
	}
	if fakeRunner.calls[0].name != "opentofu" || strings.Join(fakeRunner.calls[0].args, " ") != "init" {
		t.Fatalf("first command = %#v, want opentofu init", fakeRunner.calls[0])
	}
	if fakeRunner.calls[1].name != "opentofu" || strings.Join(fakeRunner.calls[1].args, " ") != "apply -auto-approve" {
		t.Fatalf("second command = %#v, want opentofu apply -auto-approve", fakeRunner.calls[1])
	}
	if strings.Join(fakeRuntime.calls, ",") != strings.Join(wantIDs[3:], ",") {
		t.Fatalf("runtime calls = %v, want %v", fakeRuntime.calls, wantIDs[3:])
	}
}

func TestBootstrapServiceTalosResumeSkipsCompletedSteps(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "talos-resume"
	organization := "opencenter"
	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)
	fakeRunner := &fakeLifecycleRunner{}
	bootstrapService.runner = fakeRunner
	fakeRuntime := &fakeTalosRuntime{}
	restore := replaceTalosRuntimeFactory(t, fakeRuntime)
	defer restore()

	if err := pathResolver.CreateClusterDirectories(context.Background(), clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	clusterPaths, err := pathResolver.Resolve(context.Background(), clusterName, organization)
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}
	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	v2.ApplyTalosDeploymentDefaults(&cfg)
	cfg.Deployment.Talos.Network.ManagementCIDRs = []string{"203.0.113.10/32"}
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	stateRoot := t.TempDir()
	t.Setenv("OPENCENTER_STATE_DIR", stateRoot)
	runtimePaths, err := resolveBootstrapRuntimePaths(&cfg, "", time.Now())
	if err != nil {
		t.Fatalf("resolve runtime paths: %v", err)
	}
	state := bootstrapService.newBootstrapState()
	bootstrapService.setStepStatus(state, "talos-preflight", bootstrapStatusSuccess, "")
	bootstrapService.setStepStatus(state, "opentofu-init", bootstrapStatusSuccess, "")
	if err := bootstrapService.saveBootstrapState(runtimePaths.StatePath, state); err != nil {
		t.Fatalf("save bootstrap state: %v", err)
	}

	result := &BootstrapResult{}
	if err := bootstrapService.provisionInfrastructure(context.Background(), &cfg, clusterPaths, &BootstrapOptions{
		KubeconfigPath: clusterPaths.KubeconfigPath,
	}, runtimePaths, result); err != nil {
		t.Fatalf("provisionInfrastructure() error = %v", err)
	}

	if len(fakeRunner.calls) != 1 || strings.Join(fakeRunner.calls[0].args, " ") != "apply -auto-approve" {
		t.Fatalf("expected resume to run only opentofu apply, got %#v", fakeRunner.calls)
	}
	if len(result.StepsCompleted) == 0 || result.StepsCompleted[0] != "opentofu-apply" {
		t.Fatalf("StepsCompleted = %v, want first resumed step opentofu-apply", result.StepsCompleted)
	}
}

func talosBootstrapTestConfig(t *testing.T, clusterName, organization string) (v2.Config, *paths.ClusterPaths) {
	t.Helper()

	tmpDir := t.TempDir()
	resolver := paths.NewPathResolver(tmpDir)
	if err := resolver.CreateClusterDirectories(context.Background(), clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	clusterPaths, err := resolver.Resolve(context.Background(), clusterName, organization)
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	v2.ApplyTalosDeploymentDefaults(&cfg)
	cfg.Deployment.Talos.Network.ManagementCIDRs = []string{"203.0.113.10/32"}
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}
	return cfg, clusterPaths
}

func TestTalosBootstrapProviderPreflightRequiresManagementCIDRs(t *testing.T) {
	cfg, clusterPaths := talosBootstrapTestConfig(t, "talos-no-management", "opencenter")
	cfg.Deployment.Talos.Network.ManagementCIDRs = nil
	provider := newTalosBootstrapProvider(&fakeLifecycleRunner{})

	steps, err := provider.BuildSteps(&cfg, clusterPaths, &BootstrapOptions{KubeconfigPath: clusterPaths.KubeconfigPath})
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}
	if len(steps) == 0 || steps[0].ID != "talos-preflight" {
		t.Fatalf("first step = %#v, want talos-preflight", bootstrapStepIDs(steps))
	}
	err = steps[0].Run(context.Background())
	if err == nil {
		t.Fatal("expected talos-preflight to reject missing management CIDRs")
	}
	if !strings.Contains(err.Error(), "deployment.talos.network.management_cidrs") {
		t.Fatalf("error = %q, want management_cidrs guidance", err.Error())
	}
}

func replaceTalosRuntimeFactory(t *testing.T, runtime talosBootstrapRuntime) func() {
	t.Helper()

	original := newTalosBootstrapRuntime
	newTalosBootstrapRuntime = func(*v2.Config, *paths.ClusterPaths) (talosBootstrapRuntime, error) {
		return runtime, nil
	}
	return func() {
		newTalosBootstrapRuntime = original
	}
}
