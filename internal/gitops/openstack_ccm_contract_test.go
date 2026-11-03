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
	"time"

	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// ccmPatchHelmRelease parses the rendered base-Kustomization patch block and
// returns the spec of the HelmRelease it targets.
//
// The patch is parsed rather than string-matched: it is a nested YAML document
// embedded in a template, and asserting on its exact indentation pins formatting
// instead of behaviour, so every cosmetic edit to the renderer breaks it without
// catching a real regression.
func ccmPatchHelmRelease(t *testing.T) map[string]any {
	t.Helper()
	patch, err := openstackCCMBaseKustomizationPatchesRenderer(v2.Config{})
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	// The renderer emits `  patches:` for the enclosing Kustomization; dedent the
	// first line so it parses as a standalone document.
	var doc struct {
		Patches []struct {
			Patch  string `yaml:"patch"`
			Target struct {
				Kind string `yaml:"kind"`
				Name string `yaml:"name"`
			} `yaml:"target"`
		} `yaml:"patches"`
	}
	dedented := strings.TrimPrefix(patch, "  ")
	if err := yaml.Unmarshal([]byte(dedented), &doc); err != nil {
		t.Fatalf("patch block is not valid YAML:\n%s\n%v", patch, err)
	}

	for _, p := range doc.Patches {
		if p.Target.Kind != "HelmRelease" || p.Target.Name != openstackCCMHelmReleaseName {
			continue
		}
		var hr struct {
			Spec map[string]any `yaml:"spec"`
		}
		if err := yaml.Unmarshal([]byte(p.Patch), &hr); err != nil {
			t.Fatalf("HelmRelease patch is not valid YAML:\n%s\n%v", p.Patch, err)
		}
		if hr.Spec == nil {
			t.Fatalf("HelmRelease patch has no spec:\n%s", p.Patch)
		}
		return hr.Spec
	}
	t.Fatalf("patch must target the CCM HelmRelease by name:\n%s", patch)
	return nil
}

// remediation digs out install.remediation / upgrade.remediation as a map.
func remediation(t *testing.T, spec map[string]any, action string) map[string]any {
	t.Helper()
	block, ok := spec[action].(map[string]any)
	if !ok {
		t.Fatalf("spec.%s is missing or not a mapping: %v", action, spec[action])
	}
	rem, ok := block["remediation"].(map[string]any)
	if !ok {
		t.Fatalf("spec.%s.remediation is missing or not a mapping: %v", action, block)
	}
	return rem
}

// The upstream CCM base ships install remediation that uninstalls the release
// on last failure. With external_cloud_provider: manual the CCM is the only
// component that clears the cloud-provider-uninitialized taint, so uninstalling
// it leaves the cluster with no path back to schedulable. The rendered patch
// must therefore keep the release in place.
func TestOpenstackCCMPatchForbidsUninstallRemediation(t *testing.T) {
	spec := ccmPatchHelmRelease(t)

	// Both actions must keep the release object. Uninstall-on-failure is the
	// original bug; rollback-on-failure is the same loss of evidence by another
	// route, so guard both rather than only the one that bit first.
	for _, action := range []string{"install", "upgrade"} {
		rem := remediation(t, spec, action)
		if v, ok := rem["remediateLastFailure"].(bool); !ok || v {
			t.Errorf("spec.%s.remediation.remediationLastFailure must be false, got %v", action, rem["remediateLastFailure"])
		}
		// Retries are kept on both: a transient failure should self-heal.
		if n, ok := rem["retries"].(int); !ok || n <= 0 {
			t.Errorf("spec.%s.remediation.retries must stay positive so a transient failure self-heals, got %v", action, rem["retries"])
		}
	}
}

// A CCM upgrade rolls a hostNetwork DaemonSet across every control-plane node
// one at a time (chart default maxUnavailable: 1), so the upgrade needs enough
// time for N sequential delete/create/ready rotations. The upstream 5m default
// was beaten by a healthy 3-node rollout that missed its own deadline by 25
// seconds, which -- with remediateLastFailure: false -- stalled the release
// permanently. The patch must therefore raise spec.timeout.
func TestOpenstackCCMPatchRaisesUpgradeTimeout(t *testing.T) {
	spec := ccmPatchHelmRelease(t)

	raw, ok := spec["timeout"].(string)
	if !ok {
		t.Fatalf("spec.timeout must be a Flux duration string, got %#v", spec["timeout"])
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatalf("spec.timeout %q is not a parseable duration: %v", raw, err)
	}
	// Upstream ships 5m; anything at or below it reproduces the stall.
	if timeout <= 5*time.Minute {
		t.Errorf("spec.timeout must exceed the upstream 5m: got %s", timeout)
	}
	// Roughly 90s per control-plane rotation is the observed budget; this is the
	// floor for a small control plane, not a claim about a specific cluster.
	if timeout < 3*90*time.Second {
		t.Errorf("spec.timeout %s cannot cover 3 control-plane rotations at the observed ~90s each", timeout)
	}
	if timeout != openstackCCMHelmReleaseTimeout {
		t.Errorf("spec.timeout %s must come from openstackCCMHelmReleaseTimeout (%s), not be hardcoded elsewhere", timeout, openstackCCMHelmReleaseTimeout)
	}
}

// The invariant that actually broke, stated where it can be enforced.
//
// openstack-ccm-base runs with wait: true and health-checks the CCM
// HelmRelease, so the HelmRelease must finish inside the Kustomization's own
// timeout. If the pair drifts -- the HelmRelease given more time than the
// Kustomization is willing to wait -- the Kustomization abandons a healthy
// in-flight upgrade and reports a second false failure on top of the first.
//
// The Kustomization side is emitted by a shared template in auto_descriptor.go
// and nothing else connects the two values, which is why this test exists.
func TestOpenstackCCMTimeoutsFitInsideBaseKustomization(t *testing.T) {
	if openstackCCMHelmReleaseTimeout >= openstackCCMBaseKustomizationTimeout {
		t.Errorf("HelmRelease timeout %s must be strictly less than the openstack-ccm-base Kustomization timeout %s, or the health check abandons a healthy upgrade",
			openstackCCMHelmReleaseTimeout, openstackCCMBaseKustomizationTimeout)
	}

	// The Kustomization side comes from autoBaseKustomizationTimeout, so a
	// silent edit there must fail here rather than invalidate the budget above.
	if got := autoBaseKustomizationTimeout; got != openstackCCMBaseKustomizationTimeout {
		t.Errorf("openstackCCMBaseKustomizationTimeout is %s but autoBaseKustomizationTimeout renders %s; re-derive openstackCCMHelmReleaseTimeout",
			openstackCCMBaseKustomizationTimeout, got)
	}
}

// The upgrade path is the one that legitimately changes over a cluster's life
// (credentials rotate, subnet ids get corrected), so it is the one that
// exercises the timeout. With retries: 0 a single slow upgrade marks the release
// failed, Stalled=True blocks reconciliation, and because the rendered manifest
// is already correct by then, drift detection finds no diff and never
// re-attempts: the release stays failed forever with nothing wrong on the node.
func TestOpenstackCCMUpgradeRetriesBeforeStalling(t *testing.T) {
	spec := ccmPatchHelmRelease(t)
	rem := remediation(t, spec, "upgrade")

	n, ok := rem["retries"].(int)
	if !ok {
		t.Fatalf("spec.upgrade.remediation.retries must be an int, got %#v", rem["retries"])
	}
	if n <= 0 {
		t.Errorf("spec.upgrade.remediation.retries must be positive: 0 retries plus a now-stable manifest means drift detection never re-attempts, so one slow upgrade stalls the release permanently (got %d)", n)
	}
	// Evidence preservation is the reason upgrade differs from install; it must
	// survive any future "just retry everything" edit.
	if v, ok := rem["remediateLastFailure"].(bool); !ok || v {
		t.Error("spec.upgrade.remediation.remediateLastFailure must stay false: rolling back the CCM destroys the evidence needed to diagnose the failure")
	}
}

// fluxDuration renders the timeout fields. The naive implementation people reach
// for -- strings.TrimSuffix(d.String(), "0s") -- silently corrupts any duration
// that is not a whole number of minutes: 30s becomes "3" and 90s becomes "1m3",
// both of which look plausible in a diff and are rejected by Flux at apply time.
func TestFluxDurationFormatting(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{9 * time.Minute, "9m"},
		{45 * time.Minute, "45m"},
		{time.Hour, "1h"},
		{90 * time.Minute, "1h30m"},
		{3 * time.Hour, "3h"},
		{3*time.Hour + 15*time.Minute, "3h15m"},
	}
	for _, c := range cases {
		if got := fluxDuration(c.in); got != c.want {
			t.Errorf("fluxDuration(%s) = %q, want %q", c.in, got, c.want)
		}
		// Every emitted value must be something Flux will parse as a duration.
		if _, err := time.ParseDuration(fluxDuration(c.in)); err != nil {
			t.Errorf("fluxDuration(%s) = %q does not parse back: %v", c.in, fluxDuration(c.in), err)
		}
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
				AuthURL:    "https://keystone.example.test/v3/",
				Region:     "IAD3",
				ProjectID:  "proj",
				UseOctavia: useOctavia,
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
