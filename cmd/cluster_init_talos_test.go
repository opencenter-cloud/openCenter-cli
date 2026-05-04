package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

func TestClusterInitTalosDeploymentOpenStack(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"talos-init",
		"--org", "opencenter",
		"--type", "openstack",
		"--deployment", "talos",
		"--no-keygen",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Created cluster configuration") {
		t.Fatalf("expected success output, got:\n%s", stdout.String())
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".talos-init-config.yaml")
	cfg := loadTalosInitConfigForTest(t, configPath)

	if cfg.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Fatalf("provider = %q, want openstack", cfg.OpenCenter.Infrastructure.Provider)
	}
	if cfg.Deployment.Method != "talos" {
		t.Fatalf("deployment method = %q, want talos", cfg.Deployment.Method)
	}
	if cfg.Deployment.Kubespray != nil {
		t.Fatalf("expected kubespray config to be omitted for talos, got %#v", cfg.Deployment.Kubespray)
	}
	if cfg.Deployment.Talos == nil {
		t.Fatal("expected deployment.talos defaults")
	}
	if got := cfg.Deployment.Talos.Version; got != "v1.8.0" {
		t.Fatalf("talos version = %q, want v1.8.0", got)
	}
	if got := cfg.Deployment.Talos.KubernetesVersion; got != cfg.OpenCenter.Cluster.Kubernetes.Version {
		t.Fatalf("talos kubernetes version = %q, want %q", got, cfg.OpenCenter.Cluster.Kubernetes.Version)
	}
	if got := cfg.Deployment.Talos.Install.Disk; got != "/dev/sda" {
		t.Fatalf("install disk = %q, want /dev/sda", got)
	}
	if got := cfg.Deployment.Talos.Network.TalosAPIPort; got != 50000 {
		t.Fatalf("talos api port = %d, want 50000", got)
	}
	if got := cfg.OpenCenter.Cluster.Kubernetes.APIPort; got != 443 {
		t.Fatalf("kubernetes api port = %d, want 443", got)
	}
	if got := len(cfg.Deployment.Talos.Network.ManagementCIDRs); got != 0 {
		t.Fatalf("management cidrs len = %d, want 0", got)
	}
	if cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico != nil && cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled {
		t.Fatal("expected Calico to be disabled for Talos defaults")
	}
	if cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium == nil || !cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled {
		t.Fatalf("expected Cilium to be enabled for Talos defaults, got %#v", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium)
	}
}

func TestClusterInitHelpIncludesTalosDeployment(t *testing.T) {
	cmd := newClusterInitCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init help failed: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{
		"--deployment",
		"deployment method: kubespray, talos",
		"Initialize a Talos OpenStack cluster",
		"opencenter cluster init my-cluster --org production --type openstack --deployment talos",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("cluster init help missing %q:\n%s", want, help)
		}
	}
}

func TestClusterInitTalosSOPSConfigEncryptsTalosSecrets(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"talos-sops", "--type", "openstack", "--deployment", "talos"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}

	sopsConfigPath := filepath.Join(dir, "clusters", "opencenter", ".sops.yaml")
	data, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		t.Fatalf("read .sops.yaml: %v", err)
	}
	if !strings.Contains(string(data), `secrets/talos/.*\.ya?ml$`) {
		t.Fatalf(".sops.yaml does not encrypt Talos secrets:\n%s", string(data))
	}
}

func TestClusterInitTalosRejectsProviderType(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"bad-talos", "--type", "talos"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected --type talos to fail")
	}
	if !strings.Contains(err.Error(), talosProviderMisuseMessage) {
		t.Fatalf("error = %q, want clean-break guidance", err.Error())
	}
}

func TestClusterInitTalosDottedOverridesRemainUnderDeploymentTalos(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"talos-overrides",
		"--type", "openstack",
		"--deployment", "talos",
		"--deployment.talos.install.disk", "/dev/vda",
		"--deployment.talos.network.talos_api_port", "50001",
		"--no-keygen",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster init failed: %v\nstderr: %s", err, stderr.String())
	}

	configPath := filepath.Join(dir, "clusters", "opencenter", ".talos-overrides-config.yaml")
	cfg := loadTalosInitConfigForTest(t, configPath)
	if cfg.Deployment.Talos == nil {
		t.Fatal("expected deployment.talos")
	}
	if got := cfg.Deployment.Talos.Install.Disk; got != "/dev/vda" {
		t.Fatalf("install disk = %q, want /dev/vda", got)
	}
	if got := cfg.Deployment.Talos.Network.TalosAPIPort; got != 50001 {
		t.Fatalf("talos api port = %d, want 50001", got)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "\nopencenter:\n  talos:") {
		t.Fatalf("legacy opencenter.talos should not be emitted:\n%s", string(data))
	}
}

func loadTalosInitConfigForTest(t *testing.T, configPath string) *v2.Config {
	t.Helper()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config %s: %v", configPath, err)
	}
	if !strings.Contains(string(data), "management_cidrs: []") {
		t.Fatalf("generated Talos config must include management_cidrs placeholder:\n%s", string(data))
	}
	var cfg v2.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config %s: %v", configPath, err)
	}
	return &cfg
}

func TestClusterInitTalosDeploymentAllowsOnlyOpenStack(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)

	cmd := newClusterInitCmd()
	cmd.SetArgs([]string{"talos-kind", "--type", "kind", "--deployment", "talos"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected talos deployment with kind provider to fail")
	}
	if !strings.Contains(err.Error(), "deployment.method: talos requires opencenter.infrastructure.provider: openstack") {
		t.Fatalf("error = %q, want openstack-only validation", err.Error())
	}
}

func TestV2ValidatorRejectsLegacyOpenCenterTalos(t *testing.T) {
	cfg, err := v2.NewV2Default("legacy-talos", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.LegacyTalos = map[string]any{"version": "v1.8.0"}

	err = v2.NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("expected legacy opencenter.talos to fail validation")
	}
	if !strings.Contains(err.Error(), "opencenter.talos is not supported in v2") {
		t.Fatalf("error = %q, want legacy talos validation", err.Error())
	}
}
