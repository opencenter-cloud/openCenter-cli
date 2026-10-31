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
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// The upstream CCM base ships install remediation that uninstalls the release
// on last failure. With external_cloud_provider: manual the CCM is the only
// component that clears the cloud-provider-uninitialized taint, so uninstalling
// it leaves the cluster with no path back to schedulable. The rendered patch
// must therefore keep the release in place.
func TestOpenstackCCMPatchForbidsUninstallRemediation(t *testing.T) {
	patch, err := openstackCCMBaseKustomizationPatchesRenderer(v2.Config{})
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	if !strings.Contains(patch, "remediateLastFailure: false") {
		t.Error("patch must disable remediateLastFailure so a failed CCM install is not uninstalled")
	}
	if strings.Contains(patch, "remediateLastFailure: true") {
		t.Error("patch must not enable uninstall remediation")
	}
	// Retries are kept: a transient failure should still self-heal.
	if !strings.Contains(patch, "retries: 5") {
		t.Error("patch must keep install retries")
	}
	if !strings.Contains(patch, "kind: HelmRelease") || !strings.Contains(patch, openstackCCMHelmReleaseName) {
		t.Error("patch must target the CCM HelmRelease by name")
	}
	// The patch block must be valid for the base Kustomization: two-space
	// indented `patches:` with a literal block scalar.
	if !strings.HasPrefix(patch, "  patches:\n    - patch: |-\n        apiVersion: helm.toolkit.fluxcd.io/v2") {
		t.Errorf("unexpected patch shape:\n%s", patch)
	}
}

// The CCM catalog entry must wire the remediation patch in and stay
// bootstrap-priority: it cannot wait on the sources gate, because DNS — which
// the sources gate needs to fetch repos — only works once the CCM clears the
// cloud-provider-uninitialized taint.
func TestOpenstackCCMCatalogWiresRemediationPatch(t *testing.T) {
	spec, ok := newBuiltInRenderCatalog().Lookup("openstack-ccm")
	if !ok {
		t.Fatal("openstack-ccm missing from the render catalog")
	}
	if spec.BaseKustomizationPatchesRenderer == nil {
		t.Fatal("openstack-ccm must render the remediation patch")
	}
	if !spec.SkipSourcesDependency {
		t.Error("openstack-ccm must skip the sources dependency or it deadlocks on DNS")
	}
}

// ccmValuesConfig builds a minimal OpenStack config for the CCM override
// renderer with the given use_octavia and networking ids.
func ccmValuesConfig(useOctavia bool) v2.Config {
	return v2.Config{OpenCenter: v2.OpenCenterConfig{
		Infrastructure: v2.InfrastructureConfig{Cloud: v2.CloudConfig{
			OpenStack: &v2.OpenStackCloudConfig{
				AuthURL:     "https://keystone.example.test/v3/",
				Region:      "IAD3",
				ProjectID:   "proj",
				UseOctavia:  useOctavia,
				Networking: &v2.OpenStackNetworkingConfig{
					FloatingNetworkID: "27f1ceeb-50d5-4c72-a9eb-f90502f32ffb",
					SubnetID:          "a127da07-5e5a-4342-aad3-f3108dec8028",
				},
			},
		}},
	}}
}

func renderCCMOverride(t *testing.T, useOctavia bool) string {
	t.Helper()
	out, err := templateRenderer(openstackCCMTemplate)(ccmValuesConfig(useOctavia))
	if err != nil {
		t.Fatalf("render CCM override values: %v", err)
	}
	return out
}

// networking.subnet_id is the cluster's NODE subnet. The CCM's
// [LoadBalancer] subnet-id means "subnet to allocate the VIP from", which is
// only the node subnet when use_octavia=false (the VIP is a Neutron port there).
// Under Octavia the key must be omitted, or the CCM allocates VIPs from the
// node subnet.
func TestOpenstackCCMSubnetIDFollowsOctaviaMode(t *testing.T) {
	vrrp := renderCCMOverride(t, false)
	if !strings.Contains(vrrp, "subnet-id: a127da07-5e5a-4342-aad3-f3108dec8028") {
		t.Errorf("use_octavia=false must emit the node subnet-id for the VRRP VIP port:\n%s", vrrp)
	}
	if !strings.Contains(vrrp, "floating-network-id: 27f1ceeb-50d5-4c72-a9eb-f90502f32ffb") {
		t.Errorf("floating-network-id must always be emitted:\n%s", vrrp)
	}

	octavia := renderCCMOverride(t, true)
	if strings.Contains(octavia, "subnet-id") {
		t.Errorf("use_octavia=true must omit [LoadBalancer] subnet-id (Octavia owns the VIP):\n%s", octavia)
	}
	if !strings.Contains(octavia, "floating-network-id: 27f1ceeb-50d5-4c72-a9eb-f90502f32ffb") {
		t.Errorf("floating-network-id must survive the octavia branch:\n%s", octavia)
	}
}
