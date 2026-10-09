package cluster

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
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

func TestBuildProviderBootstrapEnvironmentBaremetalExcludesOpenStackCredentials(t *testing.T) {
	cfg := mustNewClusterTestConfig("baremetal-env", "baremetal")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		AuthURL:                     "https://keystone.example.com/v3",
		ApplicationCredentialID:     "app-cred-id",
		ApplicationCredentialSecret: "app-cred-secret",
	}

	env, err := buildProviderBootstrapEnvironment(&cfg, filepath.Join(t.TempDir(), "kubeconfig.yaml"))
	if err != nil {
		t.Fatalf("buildProviderBootstrapEnvironment() error = %v", err)
	}
	for name := range env {
		if strings.HasPrefix(name, "OS_") {
			t.Fatalf("baremetal environment should not contain OpenStack variable %q: %#v", name, env)
		}
	}
}

func TestOpenStackBootstrapProviderUsesOpenTofuAndNormalizesKubeconfig(t *testing.T) {
	clusterName := "demo"
	cfg := mustNewClusterTestConfig(clusterName, "openstack")

	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(t.TempDir(), "repo")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"
	cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = true
	cfg.OpenCenter.Infrastructure.Networking.VRRPIP = "10.2.128.5"
	cfg.OpenTofu.Path = "tofu"

	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}
	inventoryDir := filepath.Join(clusterDir, "inventory")
	if err := os.MkdirAll(inventoryDir, 0o755); err != nil {
		t.Fatalf("mkdir inventory dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inventoryDir, "inventory.yaml"), []byte("all:\n"), 0o600); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	calicoValuesDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", clusterName, "services", "calico", "helm-values")
	if err := os.MkdirAll(calicoValuesDir, 0o755); err != nil {
		t.Fatalf("mkdir Calico values dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(calicoValuesDir, "override_values.yaml"), []byte("installation: {}\n"), 0o600); err != nil {
		t.Fatalf("write Calico values: %v", err)
	}

	localhostKubeconfig := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: demo
contexts:
- context:
    cluster: demo
    user: demo
  name: demo
current-context: demo
kind: Config
users:
- name: demo
  user:
    token: fake
`

	targetKubeconfig := filepath.Join(t.TempDir(), "owned", "kubeconfig.yaml")
	fakeRunner := &fakeLifecycleRunner{
		onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
			if name == "tofu" && len(args) > 0 && args[0] == "output" {
				return []byte(`{"opencenter_kubespray_inventory_path":{"value":"` + filepath.Join(dir, "inventory", "inventory.yaml") + `"},"opencenter_kubespray_lifecycle_contract_version":{"value":1},"opencenter_kubespray_api_address":{"value":"10.2.128.5"},"opencenter_kubespray_api_port":{"value":6443}}`), nil
			}
			if len(args) > 0 && args[0] == "apply" {
				sourceKubeconfig := filepath.Join(clusterDir, "kubeconfig.yaml")
				if err := os.WriteFile(sourceKubeconfig, []byte(localhostKubeconfig), 0o600); err != nil {
					t.Fatalf("write source kubeconfig: %v", err)
				}
			}
			if strings.HasSuffix(name, string(filepath.Separator)+"ansible") && strings.Contains(strings.Join(args, " "), " -m fetch ") {
				for i, arg := range args {
					if arg != "-a" || i+1 >= len(args) {
						continue
					}
					for _, part := range strings.Fields(args[i+1]) {
						if strings.HasPrefix(part, "dest=") {
							if err := os.WriteFile(strings.TrimPrefix(part, "dest="), []byte(localhostKubeconfig), 0o600); err != nil {
								t.Fatalf("write fetched kubeconfig: %v", err)
							}
						}
					}
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

	wantIDs := []string{"preflight", "opentofu-init", "opentofu-apply", "kubespray-prepare", "kubespray-wait-cloudinit", "kubespray-deploy", "kubespray-export-kubeconfig", "openstack-normalize-kubeconfig", "openstack-install-network-plugin", "patch-coredns-toleration"}
	if got := bootstrapStepIDs(steps); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("BuildSteps() IDs = %v, want %v", got, wantIDs)
	}

	for _, step := range steps {
		if err := step.Run(context.Background()); err != nil {
			t.Fatalf("step %q failed: %v", step.ID, err)
		}
	}

	if len(fakeRunner.calls) < 7 {
		t.Fatalf("expected tofu and Kubespray lifecycle commands, got %d", len(fakeRunner.calls))
	}
	if fakeRunner.calls[0].name != "tofu" || len(fakeRunner.calls[0].args) == 0 || fakeRunner.calls[0].args[0] != "init" {
		t.Fatalf("expected first command to be tofu init, got %#v", fakeRunner.calls[0])
	}
	if fakeRunner.calls[1].name != "tofu" || len(fakeRunner.calls[1].args) != 2 || fakeRunner.calls[1].args[0] != "state" || fakeRunner.calls[1].args[1] != "list" {
		t.Fatalf("expected second command to be tofu state list, got %#v", fakeRunner.calls[1])
	}
	if fakeRunner.calls[2].name != "tofu" || len(fakeRunner.calls[2].args) < 3 || fakeRunner.calls[2].args[0] != "apply" || fakeRunner.calls[2].args[1] != "-auto-approve" || fakeRunner.calls[2].args[2] != "-var=opencenter_lifecycle_mode=cli" {
		t.Fatalf("expected third command to be tofu apply -auto-approve, got %#v", fakeRunner.calls[2])
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

	// Verify the kubeconfig server URL was rewritten from localhost to the VIP.
	kubeconfigData, err := os.ReadFile(targetKubeconfig)
	if err != nil {
		t.Fatalf("read normalized kubeconfig: %v", err)
	}
	kubeconfigContent := string(kubeconfigData)
	if strings.Contains(kubeconfigContent, "127.0.0.1") {
		t.Fatalf("kubeconfig still contains 127.0.0.1; expected VIP replacement:\n%s", kubeconfigContent)
	}
	if !strings.Contains(kubeconfigContent, "https://10.2.128.5:6443") {
		t.Fatalf("kubeconfig does not contain expected VIP endpoint https://10.2.128.5:6443:\n%s", kubeconfigContent)
	}
}

func TestOpenStackBootstrapCreatesFluxBeforeSOPSSecret(t *testing.T) {
	ctx := context.Background()
	resolver := paths.NewPathResolver(t.TempDir())
	if err := resolver.CreateClusterDirectories(ctx, "sops-order", "test-org"); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}
	clusterPaths, err := resolver.Resolve(ctx, "sops-order", "test-org")
	if err != nil {
		t.Fatalf("resolve cluster paths: %v", err)
	}

	cfg := mustNewClusterTestConfig("sops-order", "openstack")
	cfg.OpenCenter.Meta.Organization = "test-org"
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(t.TempDir(), "repo")
	cfg.OpenCenter.GitOps.Repository.URL = "https://github.com/example-org/sops-order-gitops.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{Provider: "github", Owner: "example-org", Token: "test-token"}
	cfg.OpenTofu.Path = "tofu"

	provider := &openstackBootstrapProvider{runner: &fakeLifecycleRunner{}}
	steps, err := provider.BuildSteps(&cfg, clusterPaths, &BootstrapOptions{KubeconfigPath: filepath.Join(t.TempDir(), "kubeconfig.yaml")})
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}
	ids := bootstrapStepIDs(steps)
	index := func(want string) int {
		for i, id := range ids {
			if id == want {
				return i
			}
		}
		return -1
	}
	if index(sopsAgeSecretStepID) < 0 || index("openstack-flux-bootstrap") < 0 {
		t.Fatalf("expected SOPS and Flux steps, got %v", ids)
	}
	if index("openstack-flux-bootstrap") >= index(sopsAgeSecretStepID) {
		t.Fatalf("Flux bootstrap must run before SOPS secret reconciliation, got %v", ids)
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
			if name == "tofu" && len(args) > 0 && args[0] == "output" {
				return []byte(`{"opencenter_kubespray_inventory_path":{"value":"` + filepath.Join(dir, "inventory", "inventory.yaml") + `"},"opencenter_kubespray_lifecycle_contract_version":{"value":1},"opencenter_kubespray_api_address":{"value":"10.2.128.5"},"opencenter_kubespray_api_port":{"value":6443}}`), nil
			}
			if len(args) > 0 && args[0] == "apply" {
				sourceKubeconfig := filepath.Join(dir, "kubeconfig.yaml")
				if err := os.WriteFile(sourceKubeconfig, []byte("apiVersion: v1\n"), 0o600); err != nil {
					t.Fatalf("write source kubeconfig: %v", err)
				}
			}
			if strings.HasSuffix(name, string(filepath.Separator)+"ansible") && strings.Contains(strings.Join(args, " "), " -m fetch ") {
				for i, arg := range args {
					if arg == "-a" && i+1 < len(args) {
						for _, part := range strings.Fields(args[i+1]) {
							if strings.HasPrefix(part, "dest=") {
								if err := os.WriteFile(strings.TrimPrefix(part, "dest="), []byte("apiVersion: v1\nserver: https://127.0.0.1:6443\n"), 0o600); err != nil {
									t.Fatalf("write fetched kubeconfig: %v", err)
								}
							}
						}
					}
				}
			}
			if len(args) > 0 && args[0] == "output" {
				return nil, errors.New("unexpected OpenTofu output command: test fixture requires cfg.OpenTofu.Path = tofu")
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
	cfg.OpenTofu.Path = "tofu"
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "repo")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}
	inventoryDir := filepath.Join(clusterDir, "inventory")
	if err := os.MkdirAll(inventoryDir, 0o755); err != nil {
		t.Fatalf("mkdir inventory dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inventoryDir, "inventory.yaml"), []byte("all:\n"), 0o600); err != nil {
		t.Fatalf("write inventory: %v", err)
	}

	// Create the Calico Helm override values file expected by the Helm install step
	calicoValuesDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", clusterName, "services", "calico", "helm-values")
	if err := os.MkdirAll(calicoValuesDir, 0o755); err != nil {
		t.Fatalf("mkdir calico values dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(calicoValuesDir, "override_values.yaml"), []byte("installation: {}\n"), 0o600); err != nil {
		t.Fatalf("write calico override values: %v", err)
	}

	stateRoot := t.TempDir()
	t.Setenv("OPENCENTER_STATE_DIR", stateRoot)

	statePath := filepath.Join(clusterDir, "logs", "bootstrap-state.json")
	state := bootstrapService.newBootstrapState()
	bootstrapService.setStepStatus(state, "preflight", bootstrapStatusSuccess, "")
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

	if len(fakeRunner.calls) < 4 {
		t.Fatalf("expected opentofu init, state list, opentofu apply, and network plugin commands to run after resuming, got %d calls", len(fakeRunner.calls))
	}
	// opentofu-init always runs (NeverSkip) even when state marks it as completed
	if len(fakeRunner.calls[0].args) == 0 || fakeRunner.calls[0].args[0] != "init" {
		t.Fatalf("expected first resumed command to be opentofu init (NeverSkip), got %#v", fakeRunner.calls[0])
	}
	if len(fakeRunner.calls[1].args) != 2 || fakeRunner.calls[1].args[0] != "state" || fakeRunner.calls[1].args[1] != "list" {
		t.Fatalf("expected second resumed command to be opentofu state list, got %#v", fakeRunner.calls[1])
	}
	if len(fakeRunner.calls[2].args) == 0 || fakeRunner.calls[2].args[0] != "apply" {
		t.Fatalf("expected third resumed command to be opentofu apply, got %#v", fakeRunner.calls[2])
	}
	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "repo add projectcalico")
	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "upgrade --install calico projectcalico/tigera-operator")
	if _, err := os.Stat(clusterPaths.KubeconfigPath); err != nil {
		t.Fatalf("expected cluster-owned kubeconfig at %s: %v", clusterPaths.KubeconfigPath, err)
	}
	if _, err := os.Stat(runtimePaths.StatePath); err != nil {
		t.Fatalf("expected migrated bootstrap state at %s: %v", runtimePaths.StatePath, err)
	}
}

func TestOpenTofuCalicoSafetyGuards(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		state     string
		stateErr  error
		mutate    func(string) error
		wantErr   string
		wantCalls []string
	}{
		{
			name:      "state detection fails closed",
			method:    "helm",
			state:     "module.calico.local_file.calico_values",
			wantErr:   "tofu state rm 'module.calico.local_file.calico_values'",
			wantCalls: []string{"tofu state list"},
		},
		{
			name:      "missing state file is treated as empty",
			method:    "helm",
			state:     "No state file was found!\n",
			stateErr:  errors.New("command failed: tofu state list: exit status 1\nOutput: No state file was found!\n"),
			wantCalls: []string{"tofu state list", "tofu apply -auto-approve -var=opencenter_lifecycle_mode=cli"},
		},
		{
			name:      "unchanged tree succeeds",
			method:    "",
			wantCalls: []string{"tofu state list", "tofu apply -auto-approve -var=opencenter_lifecycle_mode=cli"},
		},
		{
			name:   "mutation fails",
			method: "kustomize-helm",
			mutate: func(root string) error {
				return os.WriteFile(filepath.Join(root, "helm-values", "override_values.yaml"), []byte("changed\n"), 0o600)
			},
			wantErr:   "changed helm-values/override_values.yaml",
			wantCalls: []string{"tofu state list", "tofu apply -auto-approve -var=opencenter_lifecycle_mode=cli"},
		},
		{
			name:      "kubespray bypasses guard",
			method:    "kubespray",
			state:     "module.calico.local_file.calico_values",
			wantCalls: []string{"tofu apply -auto-approve -var=opencenter_lifecycle_mode=cli"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustNewClusterTestConfig("calico-safety", "openstack")
			cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
			cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod = tt.method
			root, err := calicoGitOpsFilesRoot(&cfg)
			if err != nil {
				t.Fatalf("calicoGitOpsFilesRoot() error = %v", err)
			}
			valuesPath := filepath.Join(root, "helm-values", "override_values.yaml")
			if err := os.MkdirAll(filepath.Dir(valuesPath), 0o755); err != nil {
				t.Fatalf("mkdir Calico GitOps files: %v", err)
			}
			if err := os.WriteFile(valuesPath, []byte("unchanged\n"), 0o600); err != nil {
				t.Fatalf("write Calico GitOps file: %v", err)
			}

			runner := &fakeLifecycleRunner{
				onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
					if len(args) > 0 && args[0] == "state" {
						return []byte(tt.state), tt.stateErr
					}
					if len(args) > 0 && args[0] == "apply" && tt.mutate != nil {
						if err := tt.mutate(root); err != nil {
							return nil, err
						}
					}
					return nil, nil
				},
			}
			provider := &openstackBootstrapProvider{runner: runner}
			err = provider.runOpenTofuApply(context.Background(), &cfg, filepath.Join(t.TempDir(), "infrastructure"), nil, "tofu")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("runOpenTofuApply() error = %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("runOpenTofuApply() error = %v, want substring %q", err, tt.wantErr)
			}

			var gotCalls []string
			for _, call := range runner.calls {
				gotCalls = append(gotCalls, call.name+" "+strings.Join(call.args, " "))
			}
			if strings.Join(gotCalls, "\x00") != strings.Join(tt.wantCalls, "\x00") {
				t.Fatalf("commands = %v, want %v", gotCalls, tt.wantCalls)
			}
		})
	}
}

func TestOpenStackNetworkPluginInstallCalicoUsesHelmChart(t *testing.T) {
	cfg, _, kubeconfigPath := openStackNetworkPluginTestConfig(t, "calico-demo")

	// Pin an explicit version so the assertion below proves the value flows
	// from the config through selection.Version to the helm --version flag,
	// rather than passing because of defaultCalicoChartVersion.
	const configuredCalicoVersion = "3.29.4"
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Version = configuredCalicoVersion

	// Create the override values file at the expected path
	clusterName := cfg.ClusterName()
	valuesDir := filepath.Join(cfg.GitDir(), "applications", "overlays", clusterName, "services", "calico", "helm-values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatalf("mkdir values dir: %v", err)
	}
	valuesPath := filepath.Join(valuesDir, "override_values.yaml")
	if err := os.WriteFile(valuesPath, []byte("installation:\n  calicoNetwork:\n    linuxDataplane: BPF\n"), 0o600); err != nil {
		t.Fatalf("write override values: %v", err)
	}

	fakeRunner := &fakeLifecycleRunner{}
	provider := &openstackBootstrapProvider{runner: fakeRunner}

	step := findBootstrapStep(t, provider, cfg, cfg.GitDir(), kubeconfigPath, "openstack-install-network-plugin")
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("install step failed: %v", err)
	}

	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "apply --server-side -f https://raw.githubusercontent.com/projectcalico/calico/v"+configuredCalicoVersion+"/manifests/operator-crds.yaml")
	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "repo add projectcalico https://docs.tigera.io/calico/charts")
	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "repo update projectcalico")
	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "upgrade --install calico projectcalico/tigera-operator --version v"+configuredCalicoVersion+" --namespace tigera-operator --create-namespace --skip-crds --force-conflicts -f "+valuesPath)
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" -n tigera-operator rollout status deployment/tigera-operator --timeout=5m")

	crdApplyIndex, helmInstallIndex := -1, -1
	for i, call := range fakeRunner.calls {
		command := strings.Join(call.args, " ")
		if call.name == "kubectl" && strings.Contains(command, "apply --server-side -f https://raw.githubusercontent.com/projectcalico/calico/v"+configuredCalicoVersion+"/manifests/operator-crds.yaml") {
			crdApplyIndex = i
		}
		if call.name == "helm" && strings.Contains(command, "upgrade --install calico projectcalico/tigera-operator") {
			helmInstallIndex = i
		}
	}
	if crdApplyIndex == -1 || helmInstallIndex == -1 || crdApplyIndex >= helmInstallIndex {
		t.Fatalf("expected Calico CRD apply before Helm install, got CRD apply index %d and Helm install index %d in:\n%s", crdApplyIndex, helmInstallIndex, renderRecordedCommands(fakeRunner.calls))
	}
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" wait --for=create tigerastatus/calico --timeout=5m")
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" wait --for=condition=Available tigerastatus/calico --timeout=10m")
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" -n calico-system wait --for=condition=Ready pods --all --timeout=10m")
}

func TestOpenStackCalicoSelectionAcceptsAnyVersion(t *testing.T) {
	cfg, _, _ := openStackNetworkPluginTestConfig(t, "calico-version")

	tests := []struct {
		input string
		want  string
	}{
		{"", "v3.31.6"},
		{"3.31.6", "v3.31.6"},
		{"v3.31.6", "v3.31.6"},
		{"3.31.0", "v3.31.0"},
		{"v3.33.1", "v3.33.1"},
	}

	for _, tt := range tests {
		cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Version = tt.input
		selection, err := selectOpenStackNetworkPlugin(cfg)
		if err != nil {
			t.Fatalf("selectOpenStackNetworkPlugin(%q) error = %v", tt.input, err)
		}
		if selection.Version != tt.want {
			t.Fatalf("selectOpenStackNetworkPlugin(%q) version = %q, want %q", tt.input, selection.Version, tt.want)
		}
	}
}

func TestOpenStackNetworkPluginInstallCiliumUsesHelmOCIChartAndReadiness(t *testing.T) {
	cfg, clusterDir, kubeconfigPath := openStackNetworkPluginTestConfig(t, "cilium-demo")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium = &v2.CiliumConfig{
		Enabled:       true,
		Hubble:        true,
		NetworkPolicy: true,
	}
	fakeRunner := &fakeLifecycleRunner{}
	provider := &openstackBootstrapProvider{runner: fakeRunner}

	step := findBootstrapStep(t, provider, cfg, clusterDir, kubeconfigPath, "openstack-install-network-plugin")
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("install step failed: %v", err)
	}

	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "upgrade --install cilium oci://quay.io/cilium/charts/cilium --namespace kube-system --version 1.19.3")
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" -n kube-system rollout status ds/cilium --timeout=10m")
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" -n kube-system rollout status deploy/cilium-operator --timeout=10m")
}

func TestOpenStackNetworkPluginInstallKubeOVNUsesHelmOCIChartAndReadiness(t *testing.T) {
	cfg, clusterDir, kubeconfigPath := openStackNetworkPluginTestConfig(t, "kubeovn-demo")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN = &v2.KubeOVNConfig{
		Enabled:       true,
		NetworkPolicy: true,
	}
	fakeRunner := &fakeLifecycleRunner{}
	provider := &openstackBootstrapProvider{runner: fakeRunner}

	step := findBootstrapStep(t, provider, cfg, clusterDir, kubeconfigPath, "openstack-install-network-plugin")
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("install step failed: %v", err)
	}

	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "upgrade --install kube-ovn oci://ghcr.io/kubeovn/charts/kube-ovn-v2 --namespace kube-system --version v1.17.0")
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" -n kube-system wait --for=condition=Ready pods -l app.kubernetes.io/part-of=kube-ovn --timeout=10m")
}

func TestOpenStackNetworkPluginInstallCiliumSupportsKustomizeHelm(t *testing.T) {
	cfg, clusterDir, kubeconfigPath := openStackNetworkPluginTestConfig(t, "cilium-kustomize-demo")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium = &v2.CiliumConfig{
		Enabled:       true,
		InstallMethod: "kustomize-helm",
	}
	fakeRunner := &fakeLifecycleRunner{}
	provider := &openstackBootstrapProvider{runner: fakeRunner}

	step := findBootstrapStep(t, provider, cfg, clusterDir, kubeconfigPath, "openstack-install-network-plugin")
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("install step failed: %v", err)
	}

	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" kustomize --enable-helm")
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" apply -f")
}

func openStackNetworkPluginTestConfig(t *testing.T, clusterName string) (*v2.Config, string, string) {
	t.Helper()

	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(t.TempDir(), "repo")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
	}

	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	return &cfg, clusterDir, kubeconfigPath
}

func findBootstrapStep(t *testing.T, provider *openstackBootstrapProvider, cfg *v2.Config, clusterDir, kubeconfigPath, stepID string) bootstrapStep {
	t.Helper()

	steps, err := provider.BuildSteps(cfg, nil, &BootstrapOptions{KubeconfigPath: kubeconfigPath})
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}
	for _, step := range steps {
		if step.ID == stepID {
			return step
		}
	}
	t.Fatalf("step %q not found in %v for %s", stepID, bootstrapStepIDs(steps), clusterDir)
	return bootstrapStep{}
}

func bootstrapStepIDs(steps []bootstrapStep) []string {
	ids := make([]string, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.ID)
	}
	return ids
}

func assertRecordedCommandContains(t *testing.T, calls []recordedLifecycleCommand, name, argsSubstring string) {
	t.Helper()
	for _, call := range calls {
		if call.name == name && strings.Contains(strings.Join(call.args, " "), argsSubstring) {
			return
		}
	}
	t.Fatalf("expected command %s containing %q, got:\n%s", name, argsSubstring, renderRecordedCommands(calls))
}

func renderRecordedCommands(calls []recordedLifecycleCommand) string {
	var b strings.Builder
	for _, call := range calls {
		b.WriteString(call.name)
		if len(call.args) > 0 {
			b.WriteByte(' ')
			b.WriteString(strings.Join(call.args, " "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestReplaceLocalhostInKubeconfig(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		apiEndpointIP  string
		wantContains   string
		wantNotContain string
	}{
		{
			name: "replaces 127.0.0.1 with VIP",
			input: `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test
`,
			apiEndpointIP:  "10.2.128.5",
			wantContains:   "https://10.2.128.5:6443",
			wantNotContain: "127.0.0.1",
		},
		{
			name: "replaces localhost with VIP",
			input: `apiVersion: v1
clusters:
- cluster:
    server: https://localhost:6443
  name: test
`,
			apiEndpointIP:  "10.2.128.5",
			wantContains:   "https://10.2.128.5:6443",
			wantNotContain: "localhost",
		},
		{
			name: "replaces IPv6 loopback with VIP",
			input: `apiVersion: v1
clusters:
- cluster:
    server: https://[::1]:6443
  name: test
`,
			apiEndpointIP:  "10.2.128.5",
			wantContains:   "https://10.2.128.5:6443",
			wantNotContain: "[::1]",
		},
		{
			name: "preserves non-localhost server",
			input: `apiVersion: v1
clusters:
- cluster:
    server: https://10.0.0.1:6443
  name: test
`,
			apiEndpointIP: "10.2.128.5",
			wantContains:  "https://10.0.0.1:6443",
		},
		{
			name: "empty VIP returns data unchanged",
			input: `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test
`,
			apiEndpointIP: "",
			wantContains:  "https://127.0.0.1:6443",
		},
		{
			name: "preserves port when replacing host",
			input: `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:8443
  name: test
`,
			apiEndpointIP:  "192.168.1.100",
			wantContains:   "https://192.168.1.100:8443",
			wantNotContain: "127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(replaceLocalhostInKubeconfig([]byte(tt.input), tt.apiEndpointIP))
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantContains, got)
			}
			if tt.wantNotContain != "" && strings.Contains(got, tt.wantNotContain) {
				t.Errorf("expected output to NOT contain %q, got:\n%s", tt.wantNotContain, got)
			}
		})
	}
}

func TestResolveAPIEndpointIP(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(cfg *v2.Config)
		expected string
	}{
		{
			name: "prefers k8s_api_ip over VRRP IP",
			setup: func(cfg *v2.Config) {
				cfg.OpenCenter.Infrastructure.K8sAPIIP = "10.0.0.99"
				cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = true
				cfg.OpenCenter.Infrastructure.Networking.VRRPIP = "10.2.128.5"
			},
			expected: "10.0.0.99",
		},
		{
			name: "falls back to VRRP IP when k8s_api_ip is empty",
			setup: func(cfg *v2.Config) {
				cfg.OpenCenter.Infrastructure.K8sAPIIP = ""
				cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = true
				cfg.OpenCenter.Infrastructure.Networking.VRRPIP = "10.2.128.5"
			},
			expected: "10.2.128.5",
		},
		{
			name: "returns empty when VRRP is disabled and no k8s_api_ip",
			setup: func(cfg *v2.Config) {
				cfg.OpenCenter.Infrastructure.K8sAPIIP = ""
				cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = false
				cfg.OpenCenter.Infrastructure.Networking.VRRPIP = "10.2.128.5"
			},
			expected: "",
		},
		{
			name: "returns empty when nothing is configured",
			setup: func(cfg *v2.Config) {
				cfg.OpenCenter.Infrastructure.K8sAPIIP = ""
				cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = false
				cfg.OpenCenter.Infrastructure.Networking.VRRPIP = ""
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustNewClusterTestConfig("test", "openstack")
			tt.setup(&cfg)
			got := resolveAPIEndpointIP(&cfg)
			if got != tt.expected {
				t.Errorf("resolveAPIEndpointIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNormalizeOpenStackKubeconfigReplacesLocalhostWithVIP(t *testing.T) {
	clusterDir := t.TempDir()
	targetPath := filepath.Join(t.TempDir(), "kubeconfig.yaml")

	sourceContent := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
    certificate-authority-data: LS0tLS1...
  name: my-cluster
contexts:
- context:
    cluster: my-cluster
    user: admin
  name: my-cluster
current-context: my-cluster
kind: Config
users:
- name: admin
  user:
    client-certificate-data: LS0tLS1...
    client-key-data: LS0tLS1...
`
	sourcePath := filepath.Join(clusterDir, "kubeconfig.yaml")
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o600); err != nil {
		t.Fatalf("write source kubeconfig: %v", err)
	}

	if err := normalizeOpenStackKubeconfig(clusterDir, targetPath, "10.2.128.5"); err != nil {
		t.Fatalf("normalizeOpenStackKubeconfig() error = %v", err)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target kubeconfig: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "127.0.0.1") {
		t.Errorf("target kubeconfig still contains 127.0.0.1:\n%s", content)
	}
	if !strings.Contains(content, "https://10.2.128.5:6443") {
		t.Errorf("target kubeconfig missing VIP endpoint:\n%s", content)
	}
	// Verify the rest of the kubeconfig is preserved.
	if !strings.Contains(content, "certificate-authority-data") {
		t.Errorf("target kubeconfig lost certificate-authority-data:\n%s", content)
	}
}

func TestNormalizeOpenStackKubeconfigNoReplacementWhenVIPEmpty(t *testing.T) {
	clusterDir := t.TempDir()
	targetPath := filepath.Join(t.TempDir(), "kubeconfig.yaml")

	sourceContent := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: my-cluster
`
	sourcePath := filepath.Join(clusterDir, "kubeconfig.yaml")
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o600); err != nil {
		t.Fatalf("write source kubeconfig: %v", err)
	}

	if err := normalizeOpenStackKubeconfig(clusterDir, targetPath, ""); err != nil {
		t.Fatalf("normalizeOpenStackKubeconfig() error = %v", err)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target kubeconfig: %v", err)
	}

	// With empty VIP, localhost should be preserved.
	if !strings.Contains(string(data), "https://127.0.0.1:6443") {
		t.Errorf("expected localhost to be preserved when VIP is empty:\n%s", string(data))
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("target kubeconfig mode = %o, want 600", info.Mode().Perm())
	}
}

func TestNormalizeOpenStackKubeconfigRejectsInvalidYAMLBeforeReplacement(t *testing.T) {
	clusterDir := t.TempDir()
	targetPath := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	if err := os.WriteFile(filepath.Join(clusterDir, "kubeconfig.yaml"), []byte("clusters: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeOpenStackKubeconfig(clusterDir, targetPath, "10.2.128.5"); err == nil {
		t.Fatal("normalizeOpenStackKubeconfig() accepted invalid YAML")
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("invalid kubeconfig created target, stat error = %v", err)
	}
}
