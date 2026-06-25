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
	cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = true
	cfg.OpenCenter.Infrastructure.Networking.VRRPIP = "10.2.128.5"
	cfg.OpenTofu.Path = "tofu"

	clusterDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir cluster dir: %v", err)
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
			if len(args) > 0 && args[0] == "apply" {
				sourceKubeconfig := filepath.Join(clusterDir, "kubeconfig.yaml")
				if err := os.WriteFile(sourceKubeconfig, []byte(localhostKubeconfig), 0o600); err != nil {
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

	wantIDs := []string{"preflight", "opentofu-init", "opentofu-apply", "openstack-normalize-kubeconfig", "openstack-install-network-plugin"}
	if got := bootstrapStepIDs(steps); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("BuildSteps() IDs = %v, want %v", got, wantIDs)
	}

	for _, step := range steps[:4] {
		if err := step.Run(context.Background()); err != nil {
			t.Fatalf("step %q failed: %v", step.ID, err)
		}
	}

	if len(fakeRunner.calls) != 2 {
		t.Fatalf("expected tofu init/apply lifecycle commands, got %d", len(fakeRunner.calls))
	}
	if fakeRunner.calls[0].name != "tofu" || len(fakeRunner.calls[0].args) == 0 || fakeRunner.calls[0].args[0] != "init" {
		t.Fatalf("expected first command to be tofu init, got %#v", fakeRunner.calls[0])
	}
	if fakeRunner.calls[1].name != "tofu" || len(fakeRunner.calls[1].args) < 2 || fakeRunner.calls[1].args[0] != "apply" || fakeRunner.calls[1].args[1] != "-auto-approve" {
		t.Fatalf("expected second command to be tofu apply -auto-approve, got %#v", fakeRunner.calls[1])
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

	if len(fakeRunner.calls) < 2 {
		t.Fatalf("expected opentofu apply and network plugin commands to run after resuming, got %d calls", len(fakeRunner.calls))
	}
	if len(fakeRunner.calls[0].args) == 0 || fakeRunner.calls[0].args[0] != "apply" {
		t.Fatalf("expected resumed command to be opentofu apply, got %#v", fakeRunner.calls[0])
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

func TestOpenStackNetworkPluginInstallCalicoUsesHelmChart(t *testing.T) {
	cfg, _, kubeconfigPath := openStackNetworkPluginTestConfig(t, "calico-demo")

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

	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "repo add projectcalico https://docs.tigera.io/calico/charts")
	assertRecordedCommandContains(t, fakeRunner.calls, "helm", "upgrade --install calico projectcalico/tigera-operator --namespace tigera-operator --create-namespace -f "+valuesPath)
	assertRecordedCommandContains(t, fakeRunner.calls, "kubectl", "--kubeconfig "+kubeconfigPath+" -n tigera-operator rollout status deployment/tigera-operator --timeout=5m")
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
		{"", "v3.32.0"},
		{"3.32.0", "v3.32.0"},
		{"v3.32.0", "v3.32.0"},
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
}
