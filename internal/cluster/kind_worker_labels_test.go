package cluster

import (
	"context"
	"strings"
	"testing"
)

func TestKindWorkerLabelStepLabelsNonControlPlaneNodes(t *testing.T) {
	runner := &recordingLifecycleRunner{}
	step := newKindWorkerLabelStep("/tmp/kubeconfig.yaml", runner)

	if step.ID != kindWorkerLabelStepID {
		t.Fatalf("step ID = %q, want %q", step.ID, kindWorkerLabelStepID)
	}
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	want := "kubectl --kubeconfig /tmp/kubeconfig.yaml label nodes --selector !node-role.kubernetes.io/control-plane node-role.kubernetes.io/worker=worker --overwrite"
	if len(runner.calls) != 1 || runner.calls[0] != want {
		t.Fatalf("calls = %q, want [%q]", runner.calls, want)
	}
}

func TestKindWorkerLabelStepPlanListsCommand(t *testing.T) {
	step := newKindWorkerLabelStep("/tmp/kubeconfig.yaml", &recordingLifecycleRunner{})

	if len(step.Plan.Commands) != 1 || step.Plan.Commands[0].Name != "kubectl" {
		t.Fatalf("plan commands = %#v, want one kubectl command", step.Plan.Commands)
	}
	if !strings.Contains(strings.Join(step.Plan.Commands[0].Args, " "), "node-role.kubernetes.io/worker=worker") {
		t.Fatalf("plan args = %v, want the worker label", step.Plan.Commands[0].Args)
	}
}
