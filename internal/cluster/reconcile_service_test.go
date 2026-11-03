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
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

func TestDetectModifiedServices(t *testing.T) {
	rawStatus := ` M applications/overlays/doritos/services/openstack-ccm/helm-values/override-values.yaml
?? applications/overlays/doritos/services/headlamp/helm-values/override-values.yaml
 M applications/overlays/doritos/services/fluxcd/gateway-api.yaml
 M applications/overlays/doritos/services/fluxcd/openstack-csi-namespace.yaml
 M applications/overlays/doritos/services/sources/opencenter-cert-manager.yaml
 M applications/overlays/kit-kat/services/loki/helm-values/override-values.yaml
 M infrastructure/clusters/doritos/main.tf
`
	got := detectModifiedServices(rawStatus, "doritos")
	want := []string{"gateway-api", "headlamp", "openstack-ccm", "openstack-csi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("detectModifiedServices() = %v, want %v", got, want)
	}
}

func TestBuildScopedGitPaths(t *testing.T) {
	clusterName := "doritos"
	t.Run("scoped to services", func(t *testing.T) {
		got := buildScopedGitPaths(clusterName, []string{"openstack-ccm"})
		want := []string{
			"applications/overlays/doritos/services/openstack-ccm",
			"applications/overlays/doritos/services/fluxcd/openstack-ccm.yaml",
			"applications/overlays/doritos/services/fluxcd/openstack-ccm-namespace.yaml",
			"applications/overlays/doritos/.opencenter-secret-artifacts.json",
			".opencenter/ownership/clusters/doritos.json",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildScopedGitPaths() = %v, want %v", got, want)
		}
	})

	t.Run("whole cluster scope", func(t *testing.T) {
		got := buildScopedGitPaths(clusterName, nil)
		want := []string{
			"applications/overlays/doritos",
			"infrastructure/clusters/doritos",
			".opencenter/ownership/clusters/doritos.json",
			".opencenter/ownership/global.json",
			".gitignore",
			".sops.yaml",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildScopedGitPaths(nil) = %v, want %v", got, want)
		}
	})
}

func TestReconcileDryRunPlan(t *testing.T) {
	cfg := mustNewClusterTestConfig("doritos", "openstack")
	cfg.OpenCenter.GitOps.Repository.LocalDir = "/test/gitops/opensnack"
	clusterPaths := &paths.ClusterPaths{
		GitOpsDir:      "/test/gitops/opensnack",
		KubeconfigPath: "/test/secrets/kubeconfig.yaml",
	}

	service := NewReconcileServiceWithRunner(nil, &fakeLifecycleRunner{})

	t.Run("full reconcile plan", func(t *testing.T) {
		opts := ReconcileOptions{
			ClusterName:    "doritos",
			Services:       []string{"openstack-ccm"},
			DryRun:         true,
			KubeconfigPath: "/test/secrets/kubeconfig.yaml",
		}

		res, err := service.ReconcileWithConfig(context.Background(), opts, &cfg, clusterPaths)
		if err != nil {
			t.Fatalf("Reconcile error: %v", err)
		}
		if res.Plan == nil {
			t.Fatal("expected non-nil Plan")
		}

		stepIDs := make([]string, 0, len(res.Plan.Steps))
		for _, s := range res.Plan.Steps {
			stepIDs = append(stepIDs, s.ID)
		}
		wantStepIDs := []string{
			"gitops-stage-commit",
			"gitops-push",
			"flux-reconcile-source",
			"flux-reconcile-openstack-ccm",
		}
		if !reflect.DeepEqual(stepIDs, wantStepIDs) {
			t.Errorf("step IDs = %v, want %v", stepIDs, wantStepIDs)
		}

		// Verify openstack-ccm commands include both kustomizations and helmrelease
		ccmStep := res.Plan.Steps[3]
		if len(ccmStep.Commands) != 4 {
			t.Fatalf("openstack-ccm step commands len = %d, want 4", len(ccmStep.Commands))
		}
		// 1. namespace kustomization
		if ccmStep.Commands[0].Args[2] != "openstack-ccm-namespace" {
			t.Errorf("command 0 = %v, want openstack-ccm-namespace", ccmStep.Commands[0])
		}
		// 2. override kustomization with-source
		if ccmStep.Commands[1].Args[2] != "openstack-ccm-override" {
			t.Errorf("command 1 = %v, want openstack-ccm-override", ccmStep.Commands[1])
		}
		// 3. base kustomization
		if ccmStep.Commands[2].Args[2] != "openstack-ccm-base" {
			t.Errorf("command 2 = %v, want openstack-ccm-base", ccmStep.Commands[2])
		}
		// 4. helmrelease openstack-cloud-controller-manager in openstack-ccm
		if ccmStep.Commands[3].Args[2] != "openstack-cloud-controller-manager" || ccmStep.Commands[3].Args[4] != "openstack-ccm" {
			t.Errorf("command 3 = %v, want helmrelease openstack-cloud-controller-manager -n openstack-ccm", ccmStep.Commands[3])
		}

		var buf bytes.Buffer
		PrintReconcilePlan(&buf, res.Plan)
		output := buf.String()
		if !strings.Contains(output, "Reconcile Plan for cluster: doritos") {
			t.Errorf("printed plan missing cluster header: %s", output)
		}
		if !strings.Contains(output, "openstack-cloud-controller-manager") {
			t.Errorf("printed plan missing HR name: %s", output)
		}
	})

	t.Run("push-only / skip-flux skips Phase 2", func(t *testing.T) {
		opts := ReconcileOptions{
			ClusterName: "doritos",
			Services:    []string{"openstack-ccm"},
			PushOnly:    true,
			DryRun:      true,
		}

		res, err := service.ReconcileWithConfig(context.Background(), opts, &cfg, clusterPaths)
		if err != nil {
			t.Fatalf("Reconcile error: %v", err)
		}
		stepIDs := make([]string, 0, len(res.Plan.Steps))
		for _, s := range res.Plan.Steps {
			stepIDs = append(stepIDs, s.ID)
		}
		wantStepIDs := []string{
			"gitops-stage-commit",
			"gitops-push",
		}
		if !reflect.DeepEqual(stepIDs, wantStepIDs) {
			t.Errorf("push-only step IDs = %v, want %v", stepIDs, wantStepIDs)
		}
	})

	t.Run("flux-only skips Phase 1", func(t *testing.T) {
		opts := ReconcileOptions{
			ClusterName:    "doritos",
			Services:       []string{"openstack-ccm"},
			FluxOnly:       true,
			DryRun:         true,
			KubeconfigPath: "/test/secrets/kubeconfig.yaml",
		}

		res, err := service.ReconcileWithConfig(context.Background(), opts, &cfg, clusterPaths)
		if err != nil {
			t.Fatalf("Reconcile error: %v", err)
		}
		stepIDs := make([]string, 0, len(res.Plan.Steps))
		for _, s := range res.Plan.Steps {
			stepIDs = append(stepIDs, s.ID)
		}
		wantStepIDs := []string{
			"flux-reconcile-source",
			"flux-reconcile-openstack-ccm",
		}
		if !reflect.DeepEqual(stepIDs, wantStepIDs) {
			t.Errorf("flux-only step IDs = %v, want %v", stepIDs, wantStepIDs)
		}
	})
}

func TestReconcileExecution(t *testing.T) {
	cfg := mustNewClusterTestConfig("doritos", "openstack")
	gitDir := t.TempDir()

	// Initialize test git repository
	cmd := exec.Command("git", "init", gitDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	clusterPaths := &paths.ClusterPaths{
		GitOpsDir:      gitDir,
		KubeconfigPath: filepath.Join(gitDir, "kubeconfig.yaml"),
	}

	runner := &fakeLifecycleRunner{
		onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
			if name == "git" {
				if len(args) > 0 && args[0] == "branch" {
					return []byte("main\n"), nil
				}
				if len(args) > 0 && args[0] == "config" {
					return []byte("upstream\n"), nil
				}
				if len(args) > 0 && args[0] == "rev-parse" {
					return []byte("bf883021\n"), nil
				}
			}
			return nil, nil
		},
	}

	service := NewReconcileServiceWithRunner(nil, runner)
	opts := ReconcileOptions{
		ClusterName:    "doritos",
		Services:       []string{"openstack-ccm"},
		KubeconfigPath: clusterPaths.KubeconfigPath,
	}

	res, err := service.ReconcileWithConfig(context.Background(), opts, &cfg, clusterPaths)
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	if !res.Pushed {
		t.Error("expected Pushed = true")
	}
	if !res.FluxReconciled {
		t.Error("expected FluxReconciled = true")
	}
	if res.CommitSHA != "bf883021" {
		t.Errorf("CommitSHA = %q, want bf883021", res.CommitSHA)
	}

	// Verify executed commands
	foundPull := false
	foundPush := false
	foundFluxSrc := false
	foundFluxHR := false

	for _, call := range runner.calls {
		cmdStr := call.name + " " + strings.Join(call.args, " ")
		if strings.Contains(cmdStr, "git pull --rebase upstream main") {
			foundPull = true
		}
		if strings.Contains(cmdStr, "git push upstream main") {
			foundPush = true
		}
		if strings.Contains(cmdStr, "flux reconcile source git flux-system") {
			foundFluxSrc = true
		}
		if strings.Contains(cmdStr, "flux reconcile helmrelease openstack-cloud-controller-manager -n openstack-ccm") {
			foundFluxHR = true
		}
	}

	if !foundPull {
		t.Error("expected git pull --rebase command")
	}
	if !foundPush {
		t.Error("expected git push command")
	}
	if !foundFluxSrc {
		t.Error("expected flux reconcile source git command")
	}
	if !foundFluxHR {
		t.Error("expected flux reconcile helmrelease command")
	}
}
