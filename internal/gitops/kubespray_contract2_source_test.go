package gitops

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

var kubesprayModuleSourceRe = regexp.MustCompile(`(?s)module\s+"kubespray-cluster"\s*\{.*?source\s*=\s*"([^"]*)"`)

// Regression: a config whose deployment.kubespray block predates the module
// source fields (empty sources) must still render the contract v3 pin;
// otherwise `tofu init` fails with "inventory_output_dir: Unsupported argument".
func TestLegacyConfigRendersContract2KubespraySource(t *testing.T) {
	for _, provider := range []string{"openstack", "baremetal", "vmware"} {
		t.Run(provider, func(t *testing.T) {
			dst := t.TempDir()
			cfg := newDefault("legacy-ks-src")
			cfg.OpenCenter.Cluster.ClusterName = "legacy-ks-src"
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst
			cfg.OpenCenter.Infrastructure.Provider = provider
			cfg.Deployment.Kubespray.Modules = nil
			cfg.Deployment.Kubespray.KubesprayCluster.Source = ""
			cfg.Deployment.Kubespray.KubesprayCluster.Enabled = false

			require.NoError(t, RenderInfrastructureCluster(cfg))
			data, err := os.ReadFile(filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf"))
			require.NoError(t, err)
			m := kubesprayModuleSourceRe.FindStringSubmatch(string(data))
			require.NotNil(t, m)
			require.Equal(t, "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/provider/kubespray?ref=2026.03-rc05", m[1])
		})
	}
}
