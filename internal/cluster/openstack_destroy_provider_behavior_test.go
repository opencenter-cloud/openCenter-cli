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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
)

// failingPVRunner returns an error for kubectl calls (unreachable cluster)
// and success for everything else.
type failingPVRunner struct {
	kubectlCalls int
	tofuCalls    int
	tofuErr      error
}

func (r *failingPVRunner) Run(_ context.Context, _ string, _ map[string]string, name string, _ ...string) ([]byte, error) {
	if name == "kubectl" {
		r.kubectlCalls++
		return nil, errors.New("connection refused")
	}
	r.tofuCalls++
	return nil, r.tofuErr
}

// TestOpenStackDestroyProvider_CaptureRequireClusterAccessAborts asserts the
// capture step is fatal under --require-cluster-access when the kubeconfig is
// missing, so destroy stops BEFORE tearing down infrastructure it cannot
// fully clean up.
func TestOpenStackDestroyProvider_CaptureRequireClusterAccessAborts(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	// No kubeconfig anywhere: absent-cluster case.
	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true, RequireClusterAccess: true})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}

	capture := findStep(t, steps, "capture-csi-volumes")
	if !capture.Fatal {
		t.Fatal("capture-csi-volumes must be Fatal when RequireClusterAccess is set")
	}
	if err := capture.Run(context.Background()); err == nil {
		t.Fatal("expected capture step to fail with no kubeconfig under RequireClusterAccess")
	}
}

// TestOpenStackDestroyProvider_CaptureSkipsWhenKubeconfigAbsent asserts the
// default (non-strict) behavior remains a warning + continue, and that the
// warning is recorded on the result instead of vanishing.
func TestOpenStackDestroyProvider_CaptureSkipsWhenKubeconfigAbsent(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	runner := &fakeDestroyCommandRunner{}
	var out bytes.Buffer
	p := newTestOpenStackDestroyProvider(runner, &out)
	result := &DestroyResult{}
	p.result = result

	steps, err := p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	capture := findStep(t, steps, "capture-csi-volumes")
	if capture.Fatal {
		t.Fatal("capture-csi-volumes must not be Fatal by default")
	}
	if err := capture.Run(context.Background()); err != nil {
		t.Fatalf("capture step must not fail by default: %v", err)
	}
	if len(result.CleanupWarnings) != 1 {
		t.Fatalf("expected 1 cleanup warning, got %d: %v", len(result.CleanupWarnings), result.CleanupWarnings)
	}
	if !strings.Contains(out.String(), "skipping CSI volume capture") {
		t.Fatalf("expected skip warning in output, got: %s", out.String())
	}
}

// TestOpenStackDestroyProvider_CaptureUnreachableRecordsWarning asserts a
// present-but-unreachable cluster (kubectl fails) is recorded as a warning
// by default and as a fatal error under RequireClusterAccess.
func TestOpenStackDestroyProvider_CaptureUnreachableRecordsWarning(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	clusterDir := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName())
	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	// Default: warning, no error.
	runner := &failingPVRunner{}
	var out bytes.Buffer
	p := newTestOpenStackDestroyProvider(runner, &out)
	result := &DestroyResult{}
	p.result = result

	steps, err := p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	if err := findStep(t, steps, "capture-csi-volumes").Run(context.Background()); err != nil {
		t.Fatalf("capture step must warn, not fail, by default: %v", err)
	}
	if runner.kubectlCalls != 1 {
		t.Fatalf("expected kubectl to be attempted, calls=%d", runner.kubectlCalls)
	}
	if len(result.CleanupWarnings) != 1 {
		t.Fatalf("expected 1 cleanup warning, got %v", result.CleanupWarnings)
	}

	// Strict: fatal.
	steps, err = p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true, RequireClusterAccess: true})
	if err != nil {
		t.Fatalf("BuildSteps strict: %v", err)
	}
	if err := findStep(t, steps, "capture-csi-volumes").Run(context.Background()); err == nil {
		t.Fatal("expected capture failure under RequireClusterAccess when cluster unreachable")
	}
}

// TestOpenStackDestroyProvider_NetworkingSeverity asserts the pre-destroy
// Octavia sweep is fatal by default (leaked LBs are not silent) and only
// downgrades under AllowCleanupFailures.
func TestOpenStackDestroyProvider_NetworkingSeverity(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	runner := &fakeDestroyCommandRunner{}
	p := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	if !findStep(t, steps, "cleanup-openstack-networking").Fatal {
		t.Fatal("cleanup-openstack-networking must be Fatal by default")
	}

	steps, err = p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true, AllowCleanupFailures: true})
	if err != nil {
		t.Fatalf("BuildSteps lenient: %v", err)
	}
	if findStep(t, steps, "cleanup-openstack-networking").Fatal {
		t.Fatal("cleanup-openstack-networking must not be Fatal under AllowCleanupFailures")
	}
}

// TestOpenStackDestroyProvider_DestroyRetries asserts the destroy step
// re-attempts a transient tofu failure up to DestroyRetries extra times and
// does not retry after success.
func TestOpenStackDestroyProvider_DestroyRetries(t *testing.T) {
	cfg := makeOpenStackCfg(t)

	runner := &failingPVRunner{tofuErr: errors.New("context deadline exceeded")}
	p := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true, DestroyRetries: 2})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	destroy := findStep(t, steps, "opentofu-destroy")
	if err := destroy.Run(context.Background()); err == nil {
		t.Fatal("expected destroy to fail after exhausting retries")
	}
	if runner.tofuCalls != 3 {
		t.Fatalf("expected 1 initial + 2 retry attempts, got %d", runner.tofuCalls)
	}

	// Zero retries: single attempt.
	runner2 := &failingPVRunner{tofuErr: errors.New("boom")}
	p2 := newTestOpenStackDestroyProvider(runner2, &bytes.Buffer{})
	steps, err = p2.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true, DestroyRetries: 0})
	if err != nil {
		t.Fatalf("BuildSteps no-retry: %v", err)
	}
	if err := findStep(t, steps, "opentofu-destroy").Run(context.Background()); err == nil {
		t.Fatal("expected destroy failure")
	}
	if runner2.tofuCalls != 1 {
		t.Fatalf("expected exactly 1 attempt with DestroyRetries=0, got %d", runner2.tofuCalls)
	}
}

// TestOpenStackDestroyProvider_ParallelismFlag asserts DestroyParallelism
// reaches the tofu command line.
func TestOpenStackDestroyProvider_ParallelismFlag(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	runner := &fakeDestroyCommandRunner{}
	p := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true, DestroyParallelism: 8})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	if err := findStep(t, steps, "opentofu-destroy").Run(context.Background()); err != nil {
		t.Fatalf("destroy step: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 tofu call, got %d", len(runner.calls))
	}
	var found bool
	for _, arg := range runner.calls[0].args {
		if arg == "-parallelism=8" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected -parallelism=8 in args, got %v", runner.calls[0].args)
	}
}

// TestOpenStackDestroyProvider_KubeconfigEnvExported asserts the resolved
// kubeconfig is exported to the OpenTofu environment (bootstrap parity) so
// kubernetes-provider resources resolve the same cluster context.
func TestOpenStackDestroyProvider_KubeconfigEnvExported(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	clusterDir := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName())
	kubeconfigPath := filepath.Join(clusterDir, "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	runner := &fakeDestroyCommandRunner{}
	p := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})
	steps, err := p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	if err := findStep(t, steps, "opentofu-init").Run(context.Background()); err != nil {
		t.Fatalf("init step: %v", err)
	}
	if got := runner.calls[0].env["KUBECONFIG"]; got != kubeconfigPath {
		t.Fatalf("KUBECONFIG env = %q, want %q", got, kubeconfigPath)
	}
}

// TestOpenStackDestroyProvider_LogfReachesActionLog asserts provider warnings
// land in the destroy action log, not just stdout: the kit-kat post-mortem
// was impossible because capture warnings were stdout-only.
func TestOpenStackDestroyProvider_LogfReachesActionLog(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	runner := &fakeDestroyCommandRunner{}
	var stdout, actionLog bytes.Buffer
	p := newTestOpenStackDestroyProvider(runner, &stdout)

	steps, err := p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}

	ctx := logging.WithActionLogWriter(context.Background(), &actionLog)
	if err := findStep(t, steps, "capture-csi-volumes").Run(ctx); err != nil {
		t.Fatalf("capture step: %v", err)
	}
	if !strings.Contains(actionLog.String(), "skipping CSI volume capture") {
		t.Fatalf("expected capture warning in action log, got: %s", actionLog.String())
	}
	if !strings.Contains(stdout.String(), "skipping CSI volume capture") {
		t.Fatalf("expected capture warning in stdout too, got: %s", stdout.String())
	}
}

// TestOpenStackDestroyProvider_PreFlightInputsLogged asserts the pre-flight
// summary names the resolved kubeconfig and network ID so silent skips are
// explainable from the log alone.
func TestOpenStackDestroyProvider_PreFlightInputsLogged(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	runner := &fakeDestroyCommandRunner{}
	var out bytes.Buffer
	p := newTestOpenStackDestroyProvider(runner, &out)

	if _, err := p.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true}); err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	text := out.String()
	for _, want := range []string{"Destroy inputs:", "kubeconfig=", "network_id="} {
		if !strings.Contains(text, want) {
			t.Fatalf("pre-flight line missing %q, got: %s", want, text)
		}
	}
}

func findStep(t *testing.T, steps []destroyStep, id string) destroyStep {
	t.Helper()
	for _, s := range steps {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("step %q not found", id)
	return destroyStep{}
}
