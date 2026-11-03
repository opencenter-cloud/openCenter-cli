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
	"fmt"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// Health-check window of the openstack-ccm-base Kustomization.
//
// This constant must equal autoBaseKustomizationTimeout in
// internal/gitops/auto_descriptor.go, which is what the generator actually
// renders. It is duplicated here rather than imported because that constant is
// unexported and this file is the CCM contract, but
// TestOpenstackCCMTimeoutsFitInsideBaseKustomization fails if the two drift.
//
// The value cannot be raised from inside the CCM renderer: spec.patches on a
// Flux Kustomization are applied to the objects of the upstream base that
// Kustomization is building, and openstack-ccm-base is not one of them, so a
// self-targeting patch silently matches nothing. That makes this the hard
// ceiling on the CCM upgrade budget below, and the two must not drift apart
// independently.
const openstackCCMBaseKustomizationTimeout = 10 * time.Minute

// Upgrade-timeout budget for the CCM HelmRelease.
//
// The upstream base ships spec.timeout: 5m, which is sized for a generic
// HelmRelease. The CCM is not one: it runs as a hostNetwork DaemonSet pinned to
// the control-plane nodes, and the chart renders `updateStrategy:
// {type: RollingUpdate}` with no maxUnavailable, so Kubernetes defaults to
// maxUnavailable: 1. Every control-plane node is therefore rotated
// sequentially: delete, create, wait for Ready, repeat. The upgrade only
// succeeds if all N rotations finish inside spec.timeout.
//
// Observed on a 3 control-plane cluster: the deadline expired at 20:47:08.654
// and the replacement pods were created at 20:47:33, 20:47:34 and 20:47:36 --
// the rollout missed its own deadline by 25 seconds while doing nothing wrong.
// The release was marked failed, and because the CCM deliberately keeps
// evidence instead of rolling back (see openstackCCMUpgradeRemediationRetries),
// that single spurious failure became a permanent stall.
//
// Derived as openstackCCMBaseKustomizationTimeout - 1m rather than picked freely:
// the base Kustomization runs with wait: true and blocks on this HelmRelease, so
// the HelmRelease must be allowed less time than the Kustomization is willing to
// wait, or the Kustomization gives up on a healthy in-flight upgrade and reports
// a second false failure on top of the first. 9m is ~3.6x the observed ~2.5m for
// 3 nodes. Raising it means raising the template's 10m in the same change, which
// TestOpenstackCCMTimeoutsFitInsideBaseKustomization guards.
const openstackCCMHelmReleaseTimeout = openstackCCMBaseKustomizationTimeout - time.Minute

// openstackCCMUpgradeRemediationRetries is how many times helm-controller
// re-attempts a failed CCM upgrade before stalling.
//
// Non-zero on purpose. The CCM override values change over the life of a
// cluster (credentials rotate, subnet ids get corrected), so the upgrade path is
// the one that actually exercises the timeout. With retries: 0 a single slow
// upgrade marks the release failed, Stalled=True blocks further reconciliation,
// and -- because the rendered manifest is already correct by then -- drift
// detection finds no diff and never re-attempts. The cluster keeps running fine
// while the release object stays failed forever, and nothing clears it without
// a human. A transient Keystone slowdown or a cold image pull should not need a
// human.
const openstackCCMUpgradeRemediationRetries = 3

// openstackCCMBaseKustomizationPatchesRenderer emits the inline patch on the
// Flux base Kustomization for the OpenStack cloud-controller-manager.
//
// The upstream base ships install.remediation with retries plus uninstall-on-
// last-failure. That default is wrong for the one component whose job is to
// make the cluster schedulable. Kubespray is configured with
// cloud_provider: external and external_cloud_provider: manual, which means it
// deliberately deploys no CCM: every node boots carrying the
// node.cloudprovider.kubernetes.io/uninitialized:NoSchedule taint that kubelet
// applied at registration, and the CCM is the only component that ever removes
// it. So when a CCM install times out — a bad auth_url, a slow first pull, a
// transient Keystone error — Flux uninstalls the release, and the cluster is
// left with a taint nothing can clear, no CCM pods left to inspect, and no
// path back to Ready without human intervention. Observed on a real cluster:
// 14 pods stuck Pending for 24 hours with 0 of 10 HelmReleases Ready, while
// deploy reported success.
//
// The patch keeps the retries so a transient failure still recovers, but drops
// the uninstall so a persistent failure leaves a stalled release plus its pods
// and events behind — visible, diagnosable, and one `flux reconcile` away from
// working once the cause is fixed.
//
// The timeouts and the upgrade retry count are patched alongside it for the
// reason in openstackCCMHelmReleaseTimeout: the evidence-preservation choice is
// only safe when a slow-but-healthy upgrade cannot be mistaken for a failure,
// and when a genuine failure is retried before it is allowed to go terminal.
func openstackCCMBaseKustomizationPatchesRenderer(cfg v2.Config) (string, error) {
	return fmt.Sprintf(`  patches:
    - patch: |-
        apiVersion: helm.toolkit.fluxcd.io/v2
        kind: HelmRelease
        metadata:
          name: %[1]s
        spec:
          timeout: %[2]s
          install:
            remediation:
              retries: 5
              remediateLastFailure: false
          upgrade:
            remediation:
              # Leave a failed upgrade in place: rolling back the CCM re-taints
              # nothing but removes the evidence needed to diagnose it.
              retries: %[3]d
              remediateLastFailure: false
      target:
        group: helm.toolkit.fluxcd.io
        version: v2
        kind: HelmRelease
        name: %[1]s
`, openstackCCMHelmReleaseName, fluxDuration(openstackCCMHelmReleaseTimeout),
		openstackCCMUpgradeRemediationRetries), nil
}

// fluxDuration renders a Go duration in the compact form Flux uses ("9m",
// "1h30m"), which is what a Kustomization/HelmRelease timeout field expects.
//
// It assembles the value from whole minutes rather than trimming Go's
// time.Duration string: d.String() renders 30s as "30s" and 90s as "1m30s", so a
// naive TrimSuffix of the trailing "0s" silently produces "3" and "1m3" -- both
// plausible-looking, both rejected by Flux at apply time. Callers pass
// whole-minute budgets; a sub-minute duration rounds down to the minute below,
// which is the conservative direction for a timeout.
func fluxDuration(d time.Duration) string {
	minutes := int64(d / time.Minute)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours, rem := minutes/60, minutes%60
	if rem == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, rem)
}

// openstackCCMHelmReleaseName is the metadata.name of the CCM HelmRelease in
// the upstream base repository. It is independent of the opencenter service
// key, which is why it is named separately.
const openstackCCMHelmReleaseName = "openstack-cloud-controller-manager"
