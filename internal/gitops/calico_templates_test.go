package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func TestCalicoAutodetectionModesUseOneNodeAddressKey(t *testing.T) {
	cases := []struct {
		name      string
		mode      string
		iface     string
		cidr      string
		wantKey   string
		wantValue any
	}{
		{name: "default", wantKey: "firstFound", wantValue: true},
		{name: "blank", mode: "   ", wantKey: "firstFound", wantValue: true},
		{name: "first-found", mode: "first-found", wantKey: "firstFound", wantValue: true},
		{name: "interface", mode: "interface", iface: "ens192", wantKey: "interface", wantValue: "ens192"},
		{name: "cidr", mode: "cidr", cidr: "10.0.0.0/8", wantKey: "cidrs", wantValue: []any{"10.0.0.0/8"}},
		{name: "normalized interface preserves case", mode: " INTERFACE ", iface: " EnS192 ", wantKey: "interface", wantValue: "EnS192"},
		{name: "normalized cidr", mode: " CIDR ", cidr: " 10.0.0.0/8 ", wantKey: "cidrs", wantValue: []any{"10.0.0.0/8"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			cfg := newDefault("calico-autodetect-" + strings.ReplaceAll(tc.name, "-", ""))
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst
			calico := cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico
			calico.CalicoInterfaceAutodetect = tc.mode
			calico.CNIIface = tc.iface
			calico.AutodetectCIDR = tc.cidr

			require.NoError(t, RenderClusterApps(cfg))
			serviceDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "calico")
			values := readYAMLMap(t, filepath.Join(serviceDir, "helm-values", "override_values.yaml"))
			autodetection := mapAt(t, mapAt(t, mapAt(t, values, "installation"), "calicoNetwork"), "nodeAddressAutodetectionV4")

			require.Len(t, autodetection, 1)
			require.Equal(t, tc.wantValue, autodetection[tc.wantKey])

			helmRelease := readYAMLMap(t, filepath.Join(serviceDir, "helmrelease.yaml"))
			spec := mapAt(t, helmRelease, "spec")
			_, hasInlineValues := spec["values"]
			require.False(t, hasInlineValues, "HelmRelease must not have competing inline values")
			valuesFrom, ok := spec["valuesFrom"].([]any)
			require.True(t, ok)
			require.Len(t, valuesFrom, 1)
			require.Equal(t, map[string]any{
				"kind":      "ConfigMap",
				"name":      "calico-values",
				"valuesKey": "values.yaml",
			}, valuesFrom[0])
		})
	}
}

func TestCalicoAutodetectionRejectsUnsupportedMode(t *testing.T) {
	cfg := newDefault("calico-invalid-autodetect")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CalicoInterfaceAutodetect = "route"

	err := RenderClusterApps(cfg)
	require.ErrorContains(t, err, "unsupported calico_interface_autodetect mode")
}

func TestOpenStackCalicoToleratesExternalCloudProviderBootstrapTaint(t *testing.T) {
	dst := t.TempDir()
	cfg := mustNewGitOpsTestConfig("calico-openstack-bootstrap", "openstack")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	require.NoError(t, RenderClusterApps(cfg))
	values := readYAMLMap(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "calico", "helm-values", "override_values.yaml"))
	installation := mapAt(t, values, "installation")
	tolerations, ok := installation["controlPlaneTolerations"].([]any)
	require.True(t, ok)
	require.Equal(t, []any{map[string]any{
		"key":      "node.cloudprovider.kubernetes.io/uninitialized",
		"operator": "Exists",
		"effect":   "NoSchedule",
	}}, tolerations)
}

func TestCalicoAutodetectionRegenerationFromPersistedConfig(t *testing.T) {
	dst := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "cluster.yaml")
	cfg := newDefault("calico-autodetect-regeneration")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	calico := cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico
	calico.CalicoInterfaceAutodetect = "interface"
	calico.CNIIface = " EnS192 "

	loaded := persistAndReloadConfig(t, configPath, &cfg)
	require.NoError(t, RenderClusterApps(*loaded))
	require.NoError(t, RenderClusterApps(*loaded), "unchanged regeneration must succeed")
	assertCalicoAutodetection(t, dst, loaded.ClusterName(), map[string]any{"interface": "EnS192"})

	loaded.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CalicoInterfaceAutodetect = "cidr"
	loaded.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.AutodetectCIDR = " 10.20.0.0/16 "
	loaded = persistAndReloadConfig(t, configPath, loaded)
	require.NoError(t, RenderClusterApps(*loaded))
	assertCalicoAutodetection(t, dst, loaded.ClusterName(), map[string]any{"cidrs": []any{"10.20.0.0/16"}})
}

func persistAndReloadConfig(t *testing.T, path string, cfg *v2.Config) *v2.Config {
	t.Helper()
	data, err := v2.MarshalPublicConfig(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	persisted, err := os.ReadFile(path) //nolint:gosec // test reads its own temporary config
	require.NoError(t, err)
	loaded, err := v2.DecodePublicConfig(persisted)
	require.NoError(t, err)
	return loaded
}

func assertCalicoAutodetection(t *testing.T, dst, clusterName string, want map[string]any) {
	t.Helper()
	path := filepath.Join(dst, "applications", "overlays", clusterName, "services", "calico", "helm-values", "override_values.yaml")
	values := readYAMLMap(t, path)
	got := mapAt(t, mapAt(t, mapAt(t, values, "installation"), "calicoNetwork"), "nodeAddressAutodetectionV4")
	require.Equal(t, want, got)
}

func TestCalicoTerraformModuleRequiresKubesprayInstallMethod(t *testing.T) {
	providers := []string{"openstack", "baremetal", "vmware"}
	modes := []struct {
		name          string
		enabled       bool
		installMethod string
		wantModule    bool
	}{
		{name: "enabled kubespray", enabled: true, installMethod: "kubespray", wantModule: true},
		{name: "enabled helm", enabled: true, installMethod: "helm", wantModule: false},
		{name: "disabled kubespray", enabled: false, installMethod: "kubespray", wantModule: false},
	}

	for _, provider := range providers {
		for _, mode := range modes {
			t.Run(provider+"/"+mode.name, func(t *testing.T) {
				dst := t.TempDir()
				cfg := mustNewGitOpsTestConfig("calico-module-gating", provider)
				cfg.OpenCenter.GitOps.Repository.LocalDir = dst
				calico := cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico
				calico.Enabled = mode.enabled
				calico.InstallMethod = mode.installMethod

				require.NoError(t, RenderInfrastructureCluster(cfg))
				content, err := os.ReadFile(filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf"))
				require.NoError(t, err)
				hasModule := strings.Contains(string(content), `module "calico"`)
				require.Equal(t, mode.wantModule, hasModule)
			})
		}
	}
}
