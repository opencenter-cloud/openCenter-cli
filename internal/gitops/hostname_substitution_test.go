// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func hostnameSubTestConfig(enabled bool) v2.Config {
	return v2.Config{OpenCenter: v2.OpenCenterConfig{
		Cluster: v2.ClusterConfig{
			ClusterName: "rackai-dev",
			BaseDomain:  "rax.io",
			ClusterFQDN: "rackai-dev.rax.io",
			HostnameSubstitution: v2.HostnameSubstitutionConfig{Enabled: enabled},
		},
		Services: v2.ServiceMap{"gateway": &services.GatewayConfig{BaseConfig: services.BaseConfig{Enabled: true}}},
	}}
}

func TestHostnameSubstitutionDisabledRendersLiteral(t *testing.T) {
	files, err := gatewayOverlayFilesRenderer(hostnameSubTestConfig(false))
	require.NoError(t, err)
	gw := files["gateway.yaml"]
	require.Contains(t, gw, "auth.rackai-dev.rax.io")
	require.NotContains(t, gw, "${CLUSTER_FQDN}")
}

func TestHostnameSubstitutionEnabledRendersPlaceholder(t *testing.T) {
	files, err := gatewayOverlayFilesRenderer(hostnameSubTestConfig(true))
	require.NoError(t, err)
	gw := files["gateway.yaml"]
	require.Contains(t, gw, "auth.${CLUSTER_FQDN}")
	require.NotContains(t, gw, "auth.rackai-dev.rax.io")
}

func TestPostBuildSubstituteFromBlock(t *testing.T) {
	require.Equal(t, "", postBuildSubstituteFromBlock(hostnameSubTestConfig(false)))

	block := postBuildSubstituteFromBlock(hostnameSubTestConfig(true))
	require.Contains(t, block, "postBuild:")
	require.Contains(t, block, "substituteFrom:")
	require.Contains(t, block, "kind: ConfigMap")
	require.Contains(t, block, "name: cluster-vars")
	// Must not carry a trailing newline (the template adds line handling).
	require.False(t, strings.HasSuffix(block, "\n"))
}

func TestHostnameSubstitutionConfigDefaults(t *testing.T) {
	hs := v2.HostnameSubstitutionConfig{Enabled: true}
	require.Equal(t, "cluster-vars", hs.GetConfigMapName())
	require.Equal(t, "${CLUSTER_FQDN}", hs.FQDNPlaceholder())
	require.Equal(t, "cluster_fqdn", hs.GetVariables()["CLUSTER_FQDN"])

	custom := v2.HostnameSubstitutionConfig{
		Enabled:       true,
		ConfigMapName: "my-vars",
		Variables:     map[string]string{"FQDN": "cluster_fqdn"},
	}
	require.Equal(t, "my-vars", custom.GetConfigMapName())
	require.Equal(t, "${FQDN}", custom.FQDNPlaceholder())

	disabled := v2.HostnameSubstitutionConfig{Enabled: false}
	require.Equal(t, "", disabled.FQDNPlaceholder())
}

// TestClusterFluxBridgeEmitsClusterVarsConfigMap guards against the regression
// where the cluster-vars ConfigMap referenced by every Flux
// postBuild.substituteFrom block was never emitted on the normal generate path
// (it was only written by the ConfigStage no-templates fallback, which is
// overwritten/pruned during a real template-based generate). Without the
// ConfigMap, Flux cannot resolve ${CLUSTER_FQDN} and hostnames stay literal.
// The bridge (clusters/<cluster>/) is the flux-system reconciliation root, so
// the ConfigMap must be emitted there when substitution is enabled.
func TestClusterFluxBridgeEmitsClusterVarsConfigMap(t *testing.T) {
	root := t.TempDir()
	cfg := hostnameSubTestConfig(true)
	workspace := &GitOpsWorkspace{RootDir: root, TempDir: root, Config: cfg}

	require.NoError(t, RenderClusterFluxBridgeAtomic(cfg, workspace))

	cmPath := filepath.Join(root, "clusters", cfg.ClusterName(), "cluster-vars-configmap.yaml")
	data, err := os.ReadFile(cmPath)
	require.NoError(t, err, "cluster-vars ConfigMap must be emitted under clusters/<cluster>/ when hostname substitution is enabled")

	content := string(data)
	require.Contains(t, content, "kind: ConfigMap")
	require.Contains(t, content, "name: cluster-vars")
	require.Contains(t, content, "namespace: flux-system")
	// Values must be the resolved cluster-config fields, not placeholders.
	require.Contains(t, content, `CLUSTER_FQDN: "rackai-dev.rax.io"`)
	require.Contains(t, content, `BASE_DOMAIN: "rax.io"`)
	require.Contains(t, content, `CLUSTER_NAME: "rackai-dev"`)
}

// TestClusterFluxBridgeOmitsClusterVarsWhenDisabled confirms the disabled path
// stays byte-identical: no cluster-vars ConfigMap is written.
func TestClusterFluxBridgeOmitsClusterVarsWhenDisabled(t *testing.T) {
	root := t.TempDir()
	cfg := hostnameSubTestConfig(false)
	workspace := &GitOpsWorkspace{RootDir: root, TempDir: root, Config: cfg}

	require.NoError(t, RenderClusterFluxBridgeAtomic(cfg, workspace))

	cmPath := filepath.Join(root, "clusters", cfg.ClusterName(), "cluster-vars-configmap.yaml")
	_, err := os.Stat(cmPath)
	require.True(t, os.IsNotExist(err), "cluster-vars ConfigMap must NOT be emitted when hostname substitution is disabled")
}
