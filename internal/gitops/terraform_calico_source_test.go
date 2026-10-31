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
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// defaultGitBaseRepoTag is the immutable gitops-base tag that the default
// templates must resolve CNI/IaC modules from when no explicit override is
// configured (OCTR-808). Keep this in sync with
// internal/config/v2/defaults.go:defaultGitBaseRepoRelease.
const defaultGitBaseRepoTag = "2026.03-rc4"

// kubesprayModuleTag is the gitops-base tag carrying the lifecycle contract v2
// kubespray module (inventory_output_dir and the OIDC/kubespray contract vars).
// Keep in sync with internal/config/v2/defaults.go kubespray module sources.
const kubesprayModuleTag = "2026.03-rc4"

// calicoModuleSource matches the quoted source assignment of the rendered
// Terraform `module "calico"` block, e.g. `source = "github.com/...calico?ref=..."`.
var calicoModuleSource = regexp.MustCompile(`(?s)module\s+"calico"\s*\{.*?source\s*=\s*"([^"]*)"`)

// TestTerraformDefaultCalicoSourceHonorsOverride guards OCTR-808: the
// openstack/default template's Calico module must honor an explicit
// NetworkPlugin.Calico.Modules.Calico.Source override, consistent with the
// baremetal/VMware templates and the sibling Cilium/KubeOVN modules. It
// previously hardcoded the source, silently ignoring the override.
func TestTerraformDefaultCalicoSourceHonorsOverride(t *testing.T) {
	t.Run("no override resolves to gitops-base tag", func(t *testing.T) {
		dst := t.TempDir()
		cfg := newDefault("tf-calico-src")
		cfg.OpenCenter.Cluster.ClusterName = "tf-calico-src"
		cfg.OpenCenter.GitOps.Repository.LocalDir = dst
		cfg.OpenCenter.Infrastructure.Provider = "openstack"
		cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod = "kubespray"

		require.NoError(t, RenderInfrastructureCluster(cfg))

		mainTf := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
		data, err := os.ReadFile(mainTf) //nolint:gosec // test reads a file it just rendered
		require.NoError(t, err, "failed to read rendered %s", mainTf)

		m := calicoModuleSource.FindStringSubmatch(string(data))
		require.NotNil(t, m, "no module \"calico\" source assignment found in rendered main.tf")
		want := "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/calico?ref=" + defaultGitBaseRepoTag
		require.Equal(t, want, m[1],
			"default Calico source must resolve to gitops-base calico at tag %s when no override is set", defaultGitBaseRepoTag)
	})

	t.Run("explicit override wins", func(t *testing.T) {
		dst := t.TempDir()
		const override = "github.com/example/custom-fork.git//iac/cni/calico?ref=my-branch"
		cfg := newDefault("tf-calico-src-ovr")
		cfg.OpenCenter.Cluster.ClusterName = "tf-calico-src-ovr"
		cfg.OpenCenter.GitOps.Repository.LocalDir = dst
		cfg.OpenCenter.Infrastructure.Provider = "openstack"
		cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod = "kubespray"
		cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Modules.Calico.Source = override

		require.NoError(t, RenderInfrastructureCluster(cfg))

		mainTf := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
		data, err := os.ReadFile(mainTf) //nolint:gosec // test reads a file it just rendered
		require.NoError(t, err, "failed to read rendered %s", mainTf)

		m := calicoModuleSource.FindStringSubmatch(string(data))
		require.NotNil(t, m, "no module \"calico\" source assignment found in rendered main.tf")
		require.Equal(t, override, m[1],
			"explicit Calico source override must be rendered verbatim, not replaced by the template default")
	})
}

// TestTerraformTemplatesPinGitopsBaseTag is a source-level backstop asserting no
// embedded cluster template references a stale gitops-base ref (branch or older
// RC) for a gitops-base module source. Every ?ref= to openCenter-gitops-base
// must pin the immutable tag so generated clusters are reproducible (OCTR-808).
func TestTerraformTemplatesPinGitopsBaseTag(t *testing.T) {
	templates := []string{
		"templates/infrastructure-cluster-template/main-default.tf.tpl",
		"templates/infrastructure-cluster-template/main-baremetal.tf.tpl",
		"templates/infrastructure-cluster-template/main-vmware.tf.tpl",
	}

	// Matches a gitops-base source ref, capturing the ref value after `?ref=`.
	gitopsBaseRef := regexp.MustCompile(`openCenter-gitops-base\.git//[^"?]+\?ref=([0-9A-Za-z._-]+)`)

	for _, name := range templates {
		t.Run(filepath.Base(name), func(t *testing.T) {
			data, err := Files.ReadFile(name)
			require.NoError(t, err, "failed to read embedded template %s", name)

			matches := gitopsBaseRef.FindAllStringSubmatch(string(data), -1)
			require.NotEmpty(t, matches, "%s: no gitops-base ?ref= source found", name)

			for _, match := range matches {
				want := defaultGitBaseRepoTag
				if strings.Contains(match[0], "iac/provider/kubespray") {
					want = kubesprayModuleTag
				}
				require.Equal(t, want, match[1],
					"%s: gitops-base module ref must pin the immutable tag %q, got %q",
					name, want, match[1])
			}
		})
	}
}
