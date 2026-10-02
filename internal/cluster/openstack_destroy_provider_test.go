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
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

type fakeDestroyCommandRunner struct {
	calls []commandCall
}

type commandCall struct {
	dir  string
	env  map[string]string
	name string
	args []string
}

func (r *fakeDestroyCommandRunner) Run(_ context.Context, dir string, env map[string]string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, commandCall{
		dir:  dir,
		env:  env,
		name: name,
		args: args,
	})
	return nil, nil
}

// fakeDestroyCommandRunnerPV returns configurable PV JSON for kubectl calls.
type fakeDestroyCommandRunnerPV struct {
	pvJSON []byte
	calls  []commandCall
}

func (r *fakeDestroyCommandRunnerPV) Run(_ context.Context, dir string, env map[string]string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, commandCall{dir: dir, env: env, name: name, args: args})
	if name == "kubectl" {
		return r.pvJSON, nil
	}
	return nil, nil
}

// buildPVJSON constructs a minimal `kubectl get pv -o json` payload.
func buildPVJSON(ids []string) []byte {
	items := make([]string, len(ids))
	for i, id := range ids {
		items[i] = `{"spec":{"csi":{"driver":"cinder.csi.openstack.org","volumeHandle":"` + id + `"}}}`
	}
	return []byte(`{"items":[` + strings.Join(items, ",") + `]}`)
}

// fakeCinderForDestroy is the cinder service double for provider tests.
type fakeCinderForDestroy struct {
	volumes map[string]struct {
		status string
		size   int
	}
	deleted []string
}

func (f *fakeCinderForDestroy) GetStatus(_ context.Context, id string) (string, int, error) {
	if v, ok := f.volumes[id]; ok {
		return v.status, v.size, nil
	}
	return "", 0, context.DeadlineExceeded
}

func (f *fakeCinderForDestroy) Delete(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

// newTestOpenStackDestroyProvider creates a provider with a no-op cinder
// factory. Use this for tests that focus on step structure or tofu commands,
// not on the volume cleanup path.
func newTestOpenStackDestroyProvider(runner lifecycleCommandRunner, output *bytes.Buffer) *openstackDestroyProvider {
	p := newOpenStackDestroyProvider(runner, output).(*openstackDestroyProvider)
	p.cinderFactory = func(_ *credentials.OpenStackCredentials) (cinderVolumeService, error) {
		return &fakeCinderForDestroy{}, nil
	}
	return p
}

// makeOpenStackCfg builds a minimal openstack v2.Config for provider tests.
func makeOpenStackCfg(t *testing.T) *v2.Config {
	t.Helper()
	dir := t.TempDir()
	gitopsDir := filepath.Join(dir, "gitops")
	clusterDir := filepath.Join(gitopsDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("create cluster dir: %v", err)
	}

	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		AuthURL:                     "https://keystone.example.com/v3",
		ApplicationCredentialID:     "app-cred-id",
		ApplicationCredentialSecret: "app-cred-secret",
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitopsDir
	cfg.OpenTofu.Enabled = true
	cfg.OpenTofu.Path = "tofu"
	return cfg
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestOpenStackDestroyProvider_BuildSteps verifies the 4-step OpenStack
// pipeline: capture-csi-volumes → opentofu-init → opentofu-destroy → cleanup-csi-volumes.
func TestOpenStackDestroyProvider_BuildSteps(t *testing.T) {
	cfg := makeOpenStackCfg(t)

	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	if len(steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(steps))
	}

	wantIDs := []string{"capture-csi-volumes", "opentofu-init", "opentofu-destroy", "cleanup-csi-volumes"}
	for i, want := range wantIDs {
		if steps[i].ID != want {
			t.Errorf("step[%d].ID = %q, want %q", i, steps[i].ID, want)
		}
	}

	// Execute all steps. The capture step warns (no kubeconfig) but returns nil.
	// The cleanup step is a no-op (empty handles). Only the two tofu commands
	// should appear in runner.calls.
	ctx := context.Background()
	for _, step := range steps {
		if err := step.Run(ctx); err != nil {
			t.Fatalf("step %q failed: %v", step.ID, err)
		}
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 runner calls (init+destroy), got %d", len(runner.calls))
	}

	if runner.calls[0].name != "tofu" || runner.calls[0].args[0] != "init" {
		t.Errorf("expected runner.calls[0]='tofu init', got %s %v", runner.calls[0].name, runner.calls[0].args)
	}
	if runner.calls[1].name != "tofu" || runner.calls[1].args[0] != "destroy" {
		t.Errorf("expected runner.calls[1]='tofu destroy', got %s %v", runner.calls[1].name, runner.calls[1].args)
	}
	if len(runner.calls[1].args) < 2 || runner.calls[1].args[1] != "-auto-approve" {
		t.Errorf("expected -auto-approve flag, got args: %v", runner.calls[1].args)
	}

	if runner.calls[0].env["OS_AUTH_URL"] != "https://keystone.example.com/v3" {
		t.Errorf("expected OS_AUTH_URL in environment, got: %v", runner.calls[0].env)
	}
}

func TestOpenStackDestroyProvider_BuildSteps_NoAutoApprove(t *testing.T) {
	cfg := makeOpenStackCfg(t)

	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: false})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	// Destroy step is now at index 2 (after capture and init).
	ctx := context.Background()
	if err := steps[2].Run(ctx); err != nil {
		t.Fatalf("destroy step failed: %v", err)
	}

	// runner.calls[0] is the destroy command (init was not run in this test).
	if len(runner.calls) == 0 {
		t.Fatal("expected at least one runner call")
	}
	for _, arg := range runner.calls[0].args {
		if arg == "-auto-approve" {
			t.Error("expected no -auto-approve flag when AutoApprove is false")
		}
	}
}

func TestOpenStackDestroyProvider_BuildSteps_TofuDisabled(t *testing.T) {
	dir := t.TempDir()
	gitopsDir := filepath.Join(dir, "gitops")
	clusterDir := filepath.Join(gitopsDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("create cluster dir: %v", err)
	}

	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitopsDir
	cfg.OpenTofu.Enabled = false

	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	_, err := provider.BuildSteps(cfg, &DestroyInfraOptions{})
	if err == nil {
		t.Fatal("expected error when OpenTofu is disabled")
	}
	if !strings.Contains(err.Error(), "opentofu must be enabled") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestOpenStackDestroyProvider_BuildSteps_MissingClusterDir(t *testing.T) {
	dir := t.TempDir()
	gitopsDir := filepath.Join(dir, "gitops")
	// Don't create the cluster directory

	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitopsDir
	cfg.OpenTofu.Enabled = true
	cfg.OpenTofu.Path = "tofu"

	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	_, err := provider.BuildSteps(cfg, &DestroyInfraOptions{})
	if err == nil {
		t.Fatal("expected error when cluster directory doesn't exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestOpenStackDestroyProvider_BuildSteps_CustomTofuPath(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	cfg.OpenTofu.Path = "/custom/path/tofu"

	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	// Init step is now at index 1 (after capture-csi-volumes).
	ctx := context.Background()
	if err := steps[1].Run(ctx); err != nil {
		t.Fatalf("init step failed: %v", err)
	}

	if runner.calls[0].name != "/custom/path/tofu" {
		t.Errorf("expected custom tofu path, got %q", runner.calls[0].name)
	}
}

// TestOpenStackDestroyProvider_StepStructure_WithDeleteVolumes asserts that
// capture-csi-volumes and cleanup-csi-volumes are present when DeleteVolumes=true.
func TestOpenStackDestroyProvider_StepStructure_WithDeleteVolumes(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{DeleteVolumes: true})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	if len(steps) != 4 {
		t.Fatalf("expected 4 steps with DeleteVolumes=true, got %d", len(steps))
	}
	if steps[0].ID != "capture-csi-volumes" {
		t.Errorf("step[0].ID = %q, want 'capture-csi-volumes'", steps[0].ID)
	}
	if steps[3].ID != "cleanup-csi-volumes" {
		t.Errorf("step[3].ID = %q, want 'cleanup-csi-volumes'", steps[3].ID)
	}
}

// TestOpenStackDestroyProvider_StepStructure_WithoutDeleteVolumes asserts that
// both CSI steps are present even when DeleteVolumes=false (report mode).
func TestOpenStackDestroyProvider_StepStructure_WithoutDeleteVolumes(t *testing.T) {
	cfg := makeOpenStackCfg(t)
	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{DeleteVolumes: false})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	if len(steps) != 4 {
		t.Fatalf("expected 4 steps with DeleteVolumes=false, got %d", len(steps))
	}
	if steps[0].ID != "capture-csi-volumes" {
		t.Errorf("step[0].ID = %q, want 'capture-csi-volumes'", steps[0].ID)
	}
	if steps[3].ID != "cleanup-csi-volumes" {
		t.Errorf("step[3].ID = %q, want 'cleanup-csi-volumes'", steps[3].ID)
	}
}

// TestOpenStackDestroyProvider_CSISteps_FullFlow tests capture + cleanup with a
// fake PV response and a fake cinder service.
func TestOpenStackDestroyProvider_CSISteps_FullFlow(t *testing.T) {
	cfg := makeOpenStackCfg(t)

	// Write a placeholder kubeconfig at the path BuildSteps will derive.
	kubeconfigPath := filepath.Join(cfg.GitDir(), "infrastructure", "clusters", cfg.ClusterName(), "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	fake := &fakeCinderForDestroy{
		volumes: map[string]struct {
			status string
			size   int
		}{
			"vol-111": {status: "available", size: 10},
			"vol-222": {status: "in-use", size: 20}, // should be skipped
		},
	}

	buf := &bytes.Buffer{}
	pvRunner := &fakeDestroyCommandRunnerPV{
		pvJSON: buildPVJSON([]string{"vol-111", "vol-222"}),
	}
	provider := newOpenStackDestroyProvider(pvRunner, buf).(*openstackDestroyProvider)
	provider.cinderFactory = func(_ *credentials.OpenStackCredentials) (cinderVolumeService, error) {
		return fake, nil
	}

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{DeleteVolumes: true})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	ctx := context.Background()
	// Run capture step (index 0)
	if err := steps[0].Run(ctx); err != nil {
		t.Fatalf("capture step: %v", err)
	}
	// Run cleanup step (index 3) — skip init/destroy which would need a real tofu
	if err := steps[3].Run(ctx); err != nil {
		t.Fatalf("cleanup step: %v", err)
	}

	// vol-111 is available → should be deleted; vol-222 is in-use → skipped.
	if len(fake.deleted) != 1 || fake.deleted[0] != "vol-111" {
		t.Errorf("expected [vol-111] deleted, got %v", fake.deleted)
	}

	output := buf.String()
	if !strings.Contains(output, "vol-111") {
		t.Errorf("expected vol-111 in output, got: %s", output)
	}
	if !strings.Contains(output, "in-use") {
		t.Errorf("expected in-use status mentioned for vol-222, got: %s", output)
	}
}

// TestOpenStackDestroyProvider_BuildSteps_VMware_NoCSISteps asserts that the
// VMware provider (which reuses openstackDestroyProvider) has exactly 2 steps
// and does not include any CSI volume handling.
func TestOpenStackDestroyProvider_BuildSteps_VMware_NoCSISteps(t *testing.T) {
	dir := t.TempDir()
	gitopsDir := filepath.Join(dir, "gitops")
	clusterDir := filepath.Join(gitopsDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("create cluster dir: %v", err)
	}

	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Name = "test-cluster"
	cfg.OpenCenter.Infrastructure.Provider = "vmware"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{
		AuthURL:                     "https://keystone.example.com/v3",
		ApplicationCredentialID:     "app-cred-id",
		ApplicationCredentialSecret: "app-cred-secret",
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitopsDir
	cfg.OpenTofu.Enabled = true
	cfg.OpenTofu.Path = "tofu"

	runner := &fakeDestroyCommandRunner{}
	provider := newTestOpenStackDestroyProvider(runner, &bytes.Buffer{})

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps for VMware, got %d", len(steps))
	}
	if steps[0].ID != "opentofu-init" {
		t.Errorf("step[0].ID = %q, want 'opentofu-init'", steps[0].ID)
	}
	if steps[1].ID != "opentofu-destroy" {
		t.Errorf("step[1].ID = %q, want 'opentofu-destroy'", steps[1].ID)
	}
}
