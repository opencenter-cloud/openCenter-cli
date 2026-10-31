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

// The kubespray module must NOT receive a coredns_rewrite_block. Rewriting
// auth.<cluster_fqdn> to an in-cluster Service was wrong twice over: the
// original target (envoy-gateway.gateway.svc) never existed, so in-cluster
// OIDC discovery returned NXDOMAIN, and the correct target (the Envoy
// data-plane Service, whose name embeds a config hash) drifts whenever the
// Gateway is recreated. auth.<cluster_fqdn> resolves through ordinary DNS like
// every other service hostname, which is what the fleet already does.
func TestTerraformCoreDNSRewriteNeverEmitted(t *testing.T) {
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

			require.NotContains(t, rendered, "coredns_rewrite_block",
				"provider %q: main.tf must not hand kubespray a coredns rewrite block", provider)
			require.NotContains(t, rendered, "rewrite name exact",
				"provider %q: main.tf must not emit a CoreDNS rewrite for auth.<fqdn>", provider)
		})
	}
}
