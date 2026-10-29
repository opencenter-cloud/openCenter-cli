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

package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const waitForCCMStepID = "wait-for-cloud-controller-manager"

// cloudProviderInitTaintKey is applied by kubelet at node registration when
// it runs with --cloud-provider=external, which is what Kubespray configures
// via cloud_provider: external. Only the external cloud controller manager
// removes it, after setting spec.providerID.
const cloudProviderInitTaintKey = "node.cloudprovider.kubernetes.io/uninitialized"

// ccmWaitInterval and ccmWaitTimeout bound the gate. The CCM is installed by
// Flux, but the flux-system Kustomization only *applies* the app-of-apps tree;
// the openstack-ccm HelmRelease sits behind a dependsOn chain
// (openstack-ccm-namespace -> openstack-ccm-override -> openstack-ccm-base)
// whose sync and health checks run on 5m intervals, and the override retries a
// failed decryption every 1m. A budget under one full Flux interval expires
// while that chain is still legitimately converging on a cold cluster, so the
// gate reported "cloud controller manager did not initialize the nodes within
// 5m0s" over a cluster whose CCM was about to arrive. 20m covers two full Flux
// intervals plus the Helm install and the node-initializer pass, while still
// failing a permanently broken configuration (missing sops-age secret, Keystone
// error) inside a single deploy.
const (
	ccmWaitInterval = 15 * time.Second
	ccmWaitTimeout  = 20 * time.Minute
)

// nodeCloudState is the per-node evidence the gate inspects.
type nodeCloudState struct {
	Name       string
	ProviderID string
	Tainted    bool
}

// buildWaitForCCMStep returns the bootstrap step that blocks deploy until the
// external cloud controller manager has initialized every node.
//
// This step exists because the cluster is not actually usable when kubespray
// finishes. Nodes come up Ready but carry the cloud-provider-uninitialized
// taint, and with external_cloud_provider: manual Kubespray deliberately
// deploys no CCM — the GitOps CCM is the only thing that can clear it. Until
// it does, every deployment without that toleration is unschedulable, so a
// deploy that stops at "cluster is up" reports success over a cluster where
// nothing can run. Observed as 14 pods stuck Pending for 24 hours behind the
// message "0/6 nodes are available: 6 node(s) had untolerated taint(s)".
//
// providerID plus the taint are the two authoritative signals: providerID is
// set by the CCM node initializer, and the taint is removed by the same pass.
// Checking both avoids passing on a cluster where a stale providerID survived
// a CCM reinstall.
func (p *openstackBootstrapProvider) buildWaitForCCMStep(kubeconfigPath string) bootstrapStep {
	return bootstrapStep{
		ID:          waitForCCMStepID,
		Description: "Wait for the cloud controller manager to initialize nodes and clear the uninitialized taint",
		Plan: BootstrapPlanStep{
			ID:     waitForCCMStepID,
			Action: "Poll node objects until spec.providerID is set and the cloud-provider-uninitialized taint is gone from every node",
			Notes: []string{
				"Plan only; the cluster API was not contacted.",
				"Requires the Flux-installed cloud controller manager to have reconciled.",
			},
		},
		Run: func(ctx context.Context) error {
			return p.waitForCloudControllerManager(ctx, kubeconfigPath, ccmWaitTimeout, ccmWaitInterval)
		},
	}
}

// waitForCloudControllerManager polls until every node has a providerID and no
// cloud-provider-uninitialized taint, or the deadline passes.
func (p *openstackBootstrapProvider) waitForCloudControllerManager(ctx context.Context, kubeconfigPath string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		// Checked first: a cancelled deploy must not report success just
		// because the nodes happen to look initialized on this pass.
		if err := ctx.Err(); err != nil {
			return err
		}
		nodes, err := p.readNodeCloudState(ctx, kubeconfigPath)
		if err != nil {
			lastErr = err
		} else if reason := unschedulableReason(nodes); reason == "" {
			return nil
		} else {
			lastErr = errors.New(reason)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("cloud controller manager did not initialize the cluster within %s: %w%s",
				timeout, lastErr, p.ccmEvidence(ctx, kubeconfigPath))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// unschedulableReason returns why the cluster cannot schedule yet, or "" when
// every node is initialized. An empty node list is treated as not-ready: a
// cluster with no nodes has not been initialized either.
func unschedulableReason(nodes []nodeCloudState) string {
	if len(nodes) == 0 {
		return "no nodes are registered yet"
	}
	var missingProviderID, tainted []string
	for _, node := range nodes {
		if strings.TrimSpace(node.ProviderID) == "" {
			missingProviderID = append(missingProviderID, node.Name)
		}
		if node.Tainted {
			tainted = append(tainted, node.Name)
		}
	}
	switch {
	case len(missingProviderID) > 0:
		return fmt.Sprintf("%d/%d node(s) still have no spec.providerID: %s",
			len(missingProviderID), len(nodes), strings.Join(missingProviderID, ", "))
	case len(tainted) > 0:
		return fmt.Sprintf("%d/%d node(s) still carry the %s taint: %s",
			len(tainted), len(nodes), cloudProviderInitTaintKey, strings.Join(tainted, ", "))
	}
	return ""
}

// readNodeCloudState reads providerID and taints for every node.
func (p *openstackBootstrapProvider) readNodeCloudState(ctx context.Context, kubeconfigPath string) ([]nodeCloudState, error) {
	output, err := p.runner.Run(ctx, "", nil, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "nodes", "-o", "jsonpath="+nodeCloudJSONPath)
	if err != nil {
		return nil, fmt.Errorf("read node cloud state: %w", err)
	}
	return parseNodeCloudState(string(output))
}

// nodeCloudJSONPath emits one tab-separated record per node so parsing does
// not depend on a full node object.
const nodeCloudJSONPath = `{range .items[*]}{.metadata.name}{"|"}{.spec.providerID}{"|"}{range .spec.taints[*]}{.key}{","}{end}{";"}{end}`

// parseNodeCloudState decodes the jsonpath records produced above.
func parseNodeCloudState(raw string) ([]nodeCloudState, error) {
	var nodes []nodeCloudState
	for _, record := range strings.Split(raw, ";") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		fields := strings.Split(record, "|")
		if len(fields) < 2 {
			continue
		}
		state := nodeCloudState{Name: strings.TrimSpace(fields[0]), ProviderID: strings.TrimSpace(fields[1])}
		if len(fields) > 2 {
			for _, key := range strings.Split(fields[2], ",") {
				if strings.TrimSpace(key) == cloudProviderInitTaintKey {
					state.Tainted = true
				}
			}
		}
		nodes = append(nodes, state)
	}
	return nodes, nil
}

// ccmEvidence collects what a failed gate should report: the stalled Flux
// Kustomizations, the HelmRelease status and the CCM pods. Without this the
// operator sees only "timed out" and has to rediscover the cause by hand.
//
// The Kustomization line matters most. The CCM is not applied directly: it sits
// behind a dependsOn chain, so a Kustomization that cannot decrypt its overlay
// (missing sops-age secret) leaves every downstream app — CCM, CSI,
// cert-manager, monitoring — blocked, and no CCM HelmRelease exists yet for a
// HelmRelease-shaped report to describe. Observed on kit-kat: the gate timed
// out reporting nothing useful while openstack-ccm-override was failing with
// `secrets "sops-age" not found`.
func (p *openstackBootstrapProvider) ccmEvidence(ctx context.Context, kubeconfigPath string) string {
	var parts []string
	if out, err := p.runner.Run(ctx, "", nil, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "kustomization", "-n", "flux-system",
		"-o", "jsonpath={range .items[*]}{.metadata.name}={.status.conditions[?(@.type==\"Ready\")].message}{\"\\n\"}{end}"); err == nil {
		if line := selectBlockedKustomizations(string(out)); line != "" {
			parts = append(parts, "blocked flux kustomizations: "+line)
		}
	}
	if out, err := p.runner.Run(ctx, "", nil, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "helmrelease", "-A", "-o", "jsonpath={range .items[*]}{.metadata.namespace}/{.metadata.name}={.status.conditions[?(@.type==\"Ready\")].message}{\"\\n\"}{end}"); err == nil {
		if line := selectCCMRelease(string(out)); line != "" {
			parts = append(parts, "helmrelease: "+line)
		}
	}
	if out, err := p.runner.Run(ctx, "", nil, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "pods", "-n", "openstack-ccm", "-o", "json={.items[*].status.phase}"); err == nil {
		parts = append(parts, "ccm pods: "+strings.TrimSpace(strings.Trim(string(out), `"`)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "; " + strings.Join(parts, "; ")
}

// ccmReleaseKeywords identify resources whose name indicates they own the cloud
// controller manager.
var ccmReleaseKeywords = []string{"controller-manager", "ccm"}

// mentionsCCM reports whether a resource name refers to the CCM.
func mentionsCCM(name string) bool {
	lower := strings.ToLower(name)
	for _, keyword := range ccmReleaseKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// ccmKustomizationPrefixes are the Flux Kustomizations in the CCM dependsOn
// chain. Matching is by prefix so a future openstack-ccm-override-v2 line still
// surfaces.
var ccmKustomizationPrefixes = []string{"openstack-ccm-"}

// selectBlockedKustomizations picks the CCM-chain Kustomizations that are not
// Ready, rendering them as "name=message". An empty message means the
// Kustomization converged and is not the blocker.
func selectBlockedKustomizations(dump string) string {
	var blocked []string
	for _, line := range strings.Split(dump, "\n") {
		name, message, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if strings.TrimSpace(message) == "" {
			continue
		}
		for _, prefix := range ccmKustomizationPrefixes {
			if strings.HasPrefix(name, prefix) {
				blocked = append(blocked, name+"="+strings.TrimSpace(message))
				break
			}
		}
	}
	return strings.Join(blocked, ", ")
}

// selectCCMRelease picks the cloud-controller-manager line out of a
// namespace/name=condition dump.
func selectCCMRelease(dump string) string {
	for _, line := range strings.Split(dump, "\n") {
		name, _, _ := strings.Cut(line, "=")
		if mentionsCCM(name) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
