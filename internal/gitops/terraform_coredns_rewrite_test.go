// Copyright 2026 Victor Palma <victor.palma@rackspace.com>
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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTerraformCoreDNSRewriteEmittedAcrossProviders(t *testing.T) {
	providers := []string{"openstack", "baremetal", "vmware"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			dst := t.TempDir()
			cfg := newDefault("tf-coredns-rewrite")
			cfg.OpenCenter.Cluster.ClusterName = "tf-coredns-rewrite"
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst
			cfg.OpenCenter.Infrastructure.Provider = provider
			cfg.OpenCenter.Cluster.ClusterFQDN = "mycluster.example.test"

			require.NoError(t, RenderInfrastructureCluster(cfg))

			mainTf := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
			data, err := os.ReadFile(mainTf)
			require.NoError(t, err, "failed to read rendered %s", mainTf)
			rendered := string(data)

			require.Contains(t, rendered, `cluster_fqdn                            = "mycluster.example.test"`,
				"provider %q: main.tf must define local.cluster_fqdn", provider)
			// The rewrite target must be the Envoy data-plane LoadBalancer
			// Service in envoy-gateway-system; the control-plane Service
			// (envoy-gateway) does not serve traffic and the old
			// "envoy-gateway.gateway.svc" target never resolved (NXDOMAIN),
			// breaking in-cluster OIDC discovery.
			require.Contains(t, rendered, `coredns_rewrite_block                   = "rewrite name exact auth.${local.cluster_fqdn} envoy-rackspace-system-rmpk-gateway-067d088f.envoy-gateway-system.svc.cluster.local"`,
				"provider %q: kubespray-cluster module must rewrite auth.<fqdn> to the envoy data-plane service", provider)
			require.NotContains(t, rendered, `envoy-gateway.gateway.svc.cluster.local`,
				"provider %q: rewrite must not target the non-existent gateway/gateway service", provider)
		})
	}
}
