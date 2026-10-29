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
	"strings"
	"testing"
	"time"
)

// recordingRunner answers the kubectl calls the CCM gate makes, so the gate's
// scheduling logic is exercised without a live cluster.
type recordingRunner struct {
	responder func(args []string) (string, error)
	calls     [][]string
}

func (r *recordingRunner) Run(_ context.Context, _ string, _ map[string]string, name string, args ...string) ([]byte, error) {
	full := append([]string{name}, args...)
	r.calls = append(r.calls, full)
	if r.responder == nil {
		return nil, nil
	}
	out, err := r.responder(full)
	return []byte(out), err
}

// stripKubectlPrefix strips the leading "kubectl --kubeconfig <path>" so a test can
// match on the resource being queried.
func stripKubectlPrefix(args []string) []string {
	if len(args) >= 4 && args[0] == "kubectl" && args[1] == "--kubeconfig" {
		return args[4:]
	}
	return args
}

// nodesJSONPathOutput renders the gate's jsonpath record format:
// name|providerID|key,key;.
func nodesJSONPathOutput(records ...string) string {
	return strings.Join(records, ";")
}

// taintedNodeRecord renders a node that has no providerID and still carries
// the cloud-provider-uninitialized taint — the state a fresh cluster is left
// in until the CCM runs.
func taintedNodeRecord(name string) string {
	return name + "||" + cloudProviderInitTaintKey + ","
}

// initializedNodeRecord renders a node the CCM has fully initialized.
func initializedNodeRecord(name string) string {
	return name + "|openstack:///" + name + "|"
}

func TestParseNodeCloudState(t *testing.T) {
	raw := nodesJSONPathOutput(
		"cp0|openstack:///uuid-cp0|node-role.kubernetes.io/control-plane,"+cloudProviderInitTaintKey+",",
		"wn0||"+cloudProviderInitTaintKey+",",
		"wn1|openstack:///uuid-wn1|",
	)
	nodes, err := parseNodeCloudState(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("parsed %d nodes, want 3: %+v", len(nodes), nodes)
	}
	if !nodes[0].Tainted || nodes[0].ProviderID == "" {
		t.Errorf("cp0 = %+v, want providerID set and tainted", nodes[0])
	}
	if nodes[1].ProviderID != "" || !nodes[1].Tainted {
		t.Errorf("wn0 = %+v, want no providerID and tainted", nodes[1])
	}
	if nodes[2].ProviderID != "openstack:///uuid-wn1" || nodes[2].Tainted {
		t.Errorf("wn1 = %+v, want initialized", nodes[2])
	}
}

func TestParseNodeCloudStateIgnoresNoise(t *testing.T) {
	nodes, err := parseNodeCloudState("  ;;\n;;")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected no nodes, got %+v", nodes)
	}
}

func TestUnschedulableReason(t *testing.T) {
	tests := []struct {
		name  string
		nodes []nodeCloudState
		want  string
	}{
		{"empty cluster is not initialized", nil, "no nodes are registered yet"},
		{
			"missing providerID",
			[]nodeCloudState{{Name: "wn0"}, {Name: "wn1", ProviderID: "openstack:///x"}},
			"1/2 node(s) still have no spec.providerID: wn0",
		},
		{
			"providerID set but taint remains",
			[]nodeCloudState{{Name: "wn0", ProviderID: "openstack:///x", Tainted: true}},
			"1/1 node(s) still carry the " + cloudProviderInitTaintKey + " taint: wn0",
		},
		{
			"fully initialized",
			[]nodeCloudState{{Name: "wn0", ProviderID: "openstack:///x"}, {Name: "cp0", ProviderID: "openstack:///y"}},
			"",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := unschedulableReason(test.nodes)
			if got != test.want {
				t.Errorf("unschedulableReason = %q, want %q", got, test.want)
			}
		})
	}
}

// The gate must block while nodes are tainted, which is the state a fresh
// kubespray cluster is left in until the CCM runs.
func TestWaitForCloudControllerManagerBlocksWhileTainted(t *testing.T) {
	runner := &recordingRunner{responder: func(args []string) (string, error) {
		return nodesJSONPathOutput(taintedNodeRecord("wn0")), nil
	}}
	provider := &openstackBootstrapProvider{runner: runner}

	err := provider.waitForCloudControllerManager(context.Background(), "/tmp/kc", 40*time.Millisecond, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected the gate to fail while nodes stay tainted")
	}
	if !strings.Contains(err.Error(), cloudProviderInitTaintKey) {
		t.Errorf("error should name the blocking taint: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "within 40ms") {
		t.Errorf("error should report the timeout: %s", err.Error())
	}
	if len(runner.calls) < 2 {
		t.Errorf("expected repeated polling, got %d calls", len(runner.calls))
	}
}

func TestWaitForCloudControllerManagerSucceedsWhenInitialized(t *testing.T) {
	runner := &recordingRunner{responder: func(args []string) (string, error) {
		return nodesJSONPathOutput(initializedNodeRecord("cp0"), initializedNodeRecord("wn0")), nil
	}}
	provider := &openstackBootstrapProvider{runner: runner}

	if err := provider.waitForCloudControllerManager(context.Background(), "/tmp/kc", time.Minute, time.Millisecond); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(runner.calls) != 1 {
		t.Errorf("expected a single poll on success, got %d", len(runner.calls))
	}
}

// Recovery must be observed, not assumed: a cluster whose CCM comes up on the
// third poll has to pass.
func TestWaitForCloudControllerManagerRecovers(t *testing.T) {
	polls := 0
	runner := &recordingRunner{responder: func(args []string) (string, error) {
		polls++
		if polls < 3 {
			return nodesJSONPathOutput(taintedNodeRecord("wn0")), nil
		}
		return nodesJSONPathOutput(initializedNodeRecord("wn0")), nil
	}}
	provider := &openstackBootstrapProvider{runner: runner}

	if err := provider.waitForCloudControllerManager(context.Background(), "/tmp/kc", 5*time.Second, time.Millisecond); err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if polls != 3 {
		t.Errorf("polls = %d, want 3", polls)
	}
}

// An unreachable API must not be mistaken for an initialized cluster.
func TestWaitForCloudControllerManagerReportsAPIFailure(t *testing.T) {
	runner := &recordingRunner{responder: func(args []string) (string, error) {
		return "", errors.New("connection refused")
	}}
	provider := &openstackBootstrapProvider{runner: runner}

	err := provider.waitForCloudControllerManager(context.Background(), "/tmp/kc", 30*time.Millisecond, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected an error when the API is unreachable")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error should carry the API failure: %s", err.Error())
	}
}

// The failure message must carry the evidence that explains the stall, since
// the most common cause is a Flux-remediated CCM release with no pods left.
func TestWaitForCloudControllerManagerIncludesEvidence(t *testing.T) {
	runner := &recordingRunner{responder: func(args []string) (string, error) {
		switch call := stripKubectlPrefix(args); {
		case len(call) > 0 && call[0] == "nodes":
			return nodesJSONPathOutput(taintedNodeRecord("wn0")), nil
		case len(call) > 0 && call[0] == "helmrelease":
			return "openstack-ccm/openstack-cloud-controller-manager=Helm install failed: timeout waiting for DaemonSet\n", nil
		case len(call) > 0 && call[0] == "pods":
			return `""`, nil
		}
		return "", nil
	}}
	provider := &openstackBootstrapProvider{runner: runner}

	err := provider.waitForCloudControllerManager(context.Background(), "/tmp/kc", 30*time.Millisecond, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !strings.Contains(err.Error(), "helmrelease: openstack-ccm/openstack-cloud-controller-manager") {
		t.Errorf("error should include the HelmRelease state: %s", err.Error())
	}
}

func TestWaitForCloudControllerManagerHonoursContextCancellation(t *testing.T) {
	runner := &recordingRunner{responder: func(args []string) (string, error) {
		return nodesJSONPathOutput(taintedNodeRecord("wn0")), nil
	}}
	provider := &openstackBootstrapProvider{runner: runner}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := provider.waitForCloudControllerManager(ctx, "/tmp/kc", time.Hour, time.Millisecond); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestSelectCCMRelease(t *testing.T) {
	dump := "observability/loki=Ready\nopenstack-ccm/openstack-cloud-controller-manager=Stalled\n"
	got := selectCCMRelease(dump)
	if !strings.Contains(got, "openstack-cloud-controller-manager=Stalled") {
		t.Errorf("selectCCMRelease = %q", got)
	}
	if selectCCMRelease("observability/loki=Ready") != "" {
		t.Error("expected no match when no CCM release is present")
	}
}

// The CCM is applied through a Flux dependsOn chain, so the usual reason the
// gate times out is a Kustomization upstream of the CCM that never became
// Ready — and at that point no CCM HelmRelease exists to blame. The decryption
// failure has to reach the operator's error message.
func TestWaitForCloudControllerManagerIncludesBlockedKustomization(t *testing.T) {
	runner := &recordingRunner{responder: func(args []string) (string, error) {
		switch call := stripKubectlPrefix(args); {
		case len(call) > 0 && call[0] == "nodes":
			return nodesJSONPathOutput(taintedNodeRecord("wn0")), nil
		case len(call) > 0 && call[0] == "kustomization":
			return "openstack-ccm-namespace=\n" +
				"openstack-ccm-override=secrets \"sops-age\" not found\n" +
				"openstack-ccm-base=dependency 'flux-system/openstack-ccm-override' is not ready\n" +
				"sources=\n", nil
		}
		return "", nil
	}}
	provider := &openstackBootstrapProvider{runner: runner}

	err := provider.waitForCloudControllerManager(context.Background(), "/tmp/kc", 30*time.Millisecond, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `openstack-ccm-override=secrets "sops-age" not found`) {
		t.Errorf("error should name the failing Kustomization and why: %s", msg)
	}
	if !strings.Contains(msg, "openstack-ccm-base=dependency") {
		t.Errorf("error should include the downstream blocked Kustomization: %s", msg)
	}
	// Converged Kustomizations in the same namespace are not the blocker and
	// must not be reported as one.
	if strings.Contains(msg, "openstack-ccm-namespace=") {
		t.Errorf("ready Kustomization should not be reported as blocked: %s", msg)
	}
}

func TestSelectBlockedKustomizations(t *testing.T) {
	dump := "openstack-ccm-namespace=\n" +
		"openstack-ccm-override=secrets \"sops-age\" not found\n" +
		"kyverno-base=dependency 'flux-system/sources' is not ready\n"
	got := selectBlockedKustomizations(dump)
	if got != `openstack-ccm-override=secrets "sops-age" not found` {
		t.Errorf("selectBlockedKustomizations = %q", got)
	}
	if selectBlockedKustomizations("openstack-ccm-namespace=\nsources=\n") != "" {
		t.Error("expected no blocked Kustomization when the CCM chain is ready")
	}
}

func TestWaitForCCMStepIsRegistered(t *testing.T) {
	step := (&openstackBootstrapProvider{}).buildWaitForCCMStep("/tmp/kc")
	if step.ID != waitForCCMStepID {
		t.Errorf("step ID = %q", step.ID)
	}
	if step.Run == nil || step.Plan.ID != waitForCCMStepID {
		t.Error("step must be runnable and planned")
	}
}
