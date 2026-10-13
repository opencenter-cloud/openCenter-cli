package cluster

import (
	"context"
	"fmt"
)

const kindWorkerLabelStepID = "kind-label-workers"

// kindWorkerLabelArgs labels every node that is not a control-plane node.
// Generated manifests select workers with node-role.kubernetes.io/worker=worker,
// which the kubespray-based providers set on their workers. The kubelet cannot
// apply node-role.kubernetes.io/* labels to itself, so Kind has to label the
// nodes after they register.
func kindWorkerLabelArgs(kubeconfigPath string) []string {
	return []string{
		"--kubeconfig", kubeconfigPath,
		"label", "nodes",
		"--selector", "!node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/worker=worker",
		"--overwrite",
	}
}

func newKindWorkerLabelStep(kubeconfigPath string, runner lifecycleCommandRunner) bootstrapStep {
	return bootstrapStep{
		ID:          kindWorkerLabelStepID,
		Description: "Label Kind worker nodes",
		Plan: BootstrapPlanStep{
			ID:          kindWorkerLabelStepID,
			Action:      "Label Kind worker nodes with node-role.kubernetes.io/worker=worker",
			Commands:    []BootstrapPlanCommand{commandPlan("kubectl", kindWorkerLabelArgs(kubeconfigPath)...)},
			Environment: []BootstrapPlanEnv{{Name: "KUBECONFIG", Value: kubeconfigPath}},
			Reads:       []string{kubeconfigPath},
			Writes:      []string{"node labels on the Kind worker nodes"},
			Notes:       []string{"Plan only; node registration was not checked."},
		},
		Run: func(ctx context.Context) error {
			if _, err := runner.Run(ctx, "", nil, "kubectl", kindWorkerLabelArgs(kubeconfigPath)...); err != nil {
				return fmt.Errorf("label Kind worker nodes: %w", err)
			}
			return nil
		},
	}
}
