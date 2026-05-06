package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
	"github.com/spf13/cobra"
)

func saveOpenStackStatusConfig(t *testing.T, dir, clusterName, organization string) (v2.Config, string) {
	t.Helper()

	resolver, clusterPaths := createClusterDirectoriesForTest(t, dir, clusterName, organization)

	cfgPtr, err := v2.NewV2Default(clusterName, "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(dir, "gitops", clusterName)
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
	return cfg, clusterPaths.KubeconfigPath
}

func writeStatusOpenTofuState(t *testing.T, cfg v2.Config, extraOutputs string) string {
	t.Helper()

	infraDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", cfg.ClusterName())
	if err := os.MkdirAll(infraDir, 0o755); err != nil {
		t.Fatalf("mkdir infra dir: %v", err)
	}

	statePath := cfg.OpenTofu.Backend.Local.Path
	if !filepath.IsAbs(statePath) {
		statePath = filepath.Join(infraDir, statePath)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}

	outputs := `"master_nodes":{"value":[{"name":"status-cp-1","access_ip_v4":"10.2.128.11"},{"name":"status-cp-2","access_ip_v4":"10.2.128.12"}]},"worker_nodes":{"value":[{"name":"status-wn-1","access_ip_v4":"10.2.128.21"},{"name":"status-wn-2","access_ip_v4":"10.2.128.22"}]},"k8s_api_ip":{"value":"203.0.113.20"},"k8s_internal_ip":{"value":"10.2.128.5"},"bastion_floating_ip":{"value":"198.51.100.10"}`
	if extraOutputs != "" {
		outputs += "," + extraOutputs
	}

	state := `{"version":4,"outputs":{` + outputs + `},"resources":[]}`
	if err := os.WriteFile(statePath, []byte(state), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}
	return statePath
}

func TestClusterStatusHonorsExplicitClusterArgument(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	if _, err := os.Stat(filepath.Join(dir, "clusters")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error: %v", err)
	}

	saveOpenStackStatusConfig(t, dir, "active-cluster", "opencenter")
	saveOpenStackStatusConfig(t, dir, "requested-cluster", "opencenter")

	manager, err := config.NewConfigurationManager()
	if err != nil {
		t.Fatalf("NewConfigurationManager() error = %v", err)
	}
	if err := manager.SetActive("active-cluster"); err != nil {
		t.Fatalf("SetActive() error = %v", err)
	}

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"requested-cluster"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status failed: %v\nstderr: %s", err, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "Cluster: requested-cluster") {
		t.Fatalf("expected requested cluster in output, got:\n%s", output)
	}
	if strings.Contains(output, "Cluster: active-cluster") {
		t.Fatalf("expected explicit cluster argument to take precedence over active cluster, got:\n%s", output)
	}
}

func TestClusterStatusShowsOpenStackInfrastructureDetails(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, _ := saveOpenStackStatusConfig(t, dir, "status-cluster", "opencenter")
	writeStatusOpenTofuState(t, cfg, "")

	binDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKubectlBinary(t, binDir)
	prependTestPath(t, binDir)

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"status-cluster"})
	cmd.SetContext(context.Background())

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status failed: %v\nstderr: %s", err, errOut.String())
	}

	output := out.String()
	expectedSnippets := []string{
		"OpenStack Status:",
		"GitOps Repo:       ✓ Ready",
		"Infrastructure:    ✓ Rendered",
		"OpenTofu State:    ✓ Present",
		"API Ready:         skipped (use --refresh)",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, output)
		}
	}

	if _, err := os.Stat(filepath.Join(stateDir, "kubectl.log")); err == nil {
		t.Fatalf("default cluster status should not call kubectl, got log at %s", filepath.Join(stateDir, "kubectl.log"))
	}
}

func TestClusterStatusShowsOpenTofuInventoryFromState(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, _ := saveOpenStackStatusConfig(t, dir, "status-inventory", "opencenter")
	statePath := writeStatusOpenTofuState(t, cfg, "")

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"status-inventory"})
	cmd.SetContext(context.Background())

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status failed: %v\nstderr: %s", err, errOut.String())
	}

	output := out.String()
	expectedSnippets := []string{
		"Network:",
		"API VIP:              203.0.113.20",
		"Internal VIP:         10.2.128.5",
		"Load balancer:        ovn",
		"Floating IP pool:     PUBLICNET",
		"Bastion Floating IP:  198.51.100.10",
		"Nodes:",
		"Controllers:",
		"status-cp-1  10.2.128.11",
		"Workers:",
		"status-wn-1  10.2.128.21",
		"Inventory:",
		"Source: OpenTofu state",
		"State:  " + statePath,
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, output)
		}
	}
}

func TestClusterStatusMissingStateFallsBackToConfiguredNetwork(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, _ := saveOpenStackStatusConfig(t, dir, "status-missing-state", "opencenter")
	cfg.OpenCenter.Infrastructure.K8sAPIIP = "192.0.2.10"
	cfg.OpenCenter.Infrastructure.Networking.VRRPIP = "10.2.128.5"
	cfg.OpenCenter.Infrastructure.Networking.LoadbalancerProvider = "ovn"
	resolver, _ := createClusterDirectoriesForTest(t, dir, "status-missing-state", "opencenter")
	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"status-missing-state"})
	cmd.SetContext(context.Background())

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status failed: %v\nstderr: %s", err, errOut.String())
	}

	output := out.String()
	expectedSnippets := []string{
		"Network:",
		"API VIP:              192.0.2.10 (configured)",
		"Internal VIP:         10.2.128.5 (configured)",
		"Load balancer:        ovn",
		"Controller IPs: unavailable until OpenTofu provisioning completes",
		"Worker IPs:     unavailable until OpenTofu provisioning completes",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, output)
		}
	}
}

func TestClusterStatusDoesNotRunLiveChecksWithoutRefresh(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, kubeconfigPath := saveOpenStackStatusConfig(t, dir, "status-no-refresh", "opencenter")
	writeStatusOpenTofuState(t, cfg, "")
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0o755); err != nil {
		t.Fatalf("mkdir kubeconfig dir: %v", err)
	}
	if err := os.WriteFile(kubeconfigPath, []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	binDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKubectlBinary(t, binDir)
	prependTestPath(t, binDir)

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"status-no-refresh"})
	cmd.SetContext(context.Background())

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status failed: %v\nstderr: %s", err, errOut.String())
	}

	if _, err := os.Stat(filepath.Join(stateDir, "kubectl.log")); err == nil {
		t.Fatalf("default cluster status should not call kubectl, got log at %s", filepath.Join(stateDir, "kubectl.log"))
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat kubectl log: %v", err)
	}
}

func TestClusterStatusRefreshUsesLiveNodeIPs(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cfg, kubeconfigPath := saveOpenStackStatusConfig(t, dir, "status-refresh", "opencenter")
	writeStatusOpenTofuState(t, cfg, "")
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0o755); err != nil {
		t.Fatalf("mkdir kubeconfig dir: %v", err)
	}
	if err := os.WriteFile(kubeconfigPath, []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	binDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("FAKE_KIND_STATE_DIR", stateDir)
	installFakeKubectlBinary(t, binDir)
	prependTestPath(t, binDir)
	nodesJSON := `{"items":[{"metadata":{"name":"live-cp-1","labels":{"node-role.kubernetes.io/control-plane":""}},"status":{"addresses":[{"type":"InternalIP","address":"10.2.128.111"},{"type":"ExternalIP","address":"203.0.113.111"}],"conditions":[{"type":"Ready","status":"True"}]}},{"metadata":{"name":"live-wn-1","labels":{}},"status":{"addresses":[{"type":"InternalIP","address":"10.2.128.121"}],"conditions":[{"type":"Ready","status":"True"}]}}]}`
	if err := os.WriteFile(filepath.Join(stateDir, "nodes.json"), []byte(nodesJSON), 0o600); err != nil {
		t.Fatalf("write nodes json: %v", err)
	}

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"status-refresh", "--refresh"})
	cmd.SetContext(context.Background())

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status --refresh failed: %v\nstderr: %s", err, errOut.String())
	}

	output := out.String()
	expectedSnippets := []string{
		"API Ready:         ✓ Ready",
		"API Endpoint:      https://127.0.0.1:6443",
		"Source: Kubernetes refresh",
		"live-cp-1  10.2.128.111  203.0.113.111",
		"live-wn-1  10.2.128.121",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got:\n%s", snippet, output)
		}
	}
	if strings.Contains(output, "status-cp-1  10.2.128.11") {
		t.Fatalf("expected refreshed nodes to replace stale state nodes, got:\n%s", output)
	}
	kubectlLog, err := os.ReadFile(filepath.Join(stateDir, "kubectl.log"))
	if err != nil {
		t.Fatalf("read kubectl log: %v", err)
	}
	if !strings.Contains(string(kubectlLog), "get nodes -o json") {
		t.Fatalf("expected refresh to call kubectl get nodes, log:\n%s", string(kubectlLog))
	}
}

func TestClusterStatusHonorsJSONOutput(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	cfg, _ := saveOpenStackStatusConfig(t, dir, "status-json", "opencenter")
	writeStatusOpenTofuState(t, cfg, "")

	root := &cobra.Command{
		Use:           "opencenter",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return applyGlobalOptions(cmd, args)
		},
	}
	addGlobalFlags(root)
	root.AddCommand(NewClusterCmd())

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"cluster", "status", "status-json", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster status --output json failed: %v\nstderr: %s", err, errOut.String())
	}

	if strings.Contains(out.String(), "Cluster: status-json") {
		t.Fatalf("expected structured JSON output, got text:\n%s", out.String())
	}

	var payload struct {
		Cluster      string `json:"cluster"`
		Name         string `json:"name"`
		Environment  string `json:"environment"`
		Organization string `json:"organization"`
		Provider     string `json:"provider"`
		Inventory    struct {
			Source    string `json:"source"`
			StatePath string `json:"state_path"`
			Network   struct {
				APIVIP            string `json:"api_vip"`
				InternalVIP       string `json:"internal_vip"`
				LoadBalancer      string `json:"load_balancer"`
				FloatingIPPool    string `json:"floating_ip_pool"`
				BastionFloatingIP string `json:"bastion_floating_ip"`
			} `json:"network"`
			Nodes []struct {
				Name       string `json:"name"`
				Role       string `json:"role"`
				InternalIP string `json:"internal_ip"`
				ExternalIP string `json:"external_ip"`
				Source     string `json:"source"`
			} `json:"nodes"`
			Warnings []string `json:"warnings"`
		} `json:"inventory"`
		NextSteps []string `json:"next_steps"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json output, got error %v and output:\n%s", err, out.String())
	}
	if payload.Cluster != "status-json" {
		t.Fatalf("expected cluster status-json, got %#v", payload)
	}
	if payload.Provider != "openstack" {
		t.Fatalf("expected provider openstack, got %#v", payload)
	}
	if payload.Inventory.Source != "opentofu_state" {
		t.Fatalf("expected inventory source opentofu_state, got %#v", payload.Inventory)
	}
	if payload.Inventory.Network.APIVIP != "203.0.113.20" {
		t.Fatalf("expected API VIP from inventory, got %#v", payload.Inventory.Network)
	}
	if len(payload.Inventory.Nodes) != 4 {
		t.Fatalf("expected 4 inventory nodes, got %#v", payload.Inventory.Nodes)
	}
	if len(payload.NextSteps) == 0 {
		t.Fatalf("expected next steps in json payload, got %#v", payload)
	}
}

func TestClusterStatusQuietUnchanged(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveOpenStackStatusConfig(t, dir, "status-quiet", "opencenter")

	cmd := newClusterStatusCmd()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"status-quiet", "--quiet"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster status --quiet failed: %v\nstderr: %s", err, errOut.String())
	}

	if got := out.String(); got != "status-quiet\n" {
		t.Fatalf("quiet output = %q, want cluster name only", got)
	}
}
