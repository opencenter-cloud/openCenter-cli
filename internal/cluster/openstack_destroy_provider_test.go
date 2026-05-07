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
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

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

func TestOpenStackDestroyProvider_BuildSteps(t *testing.T) {
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

	runner := &fakeDestroyCommandRunner{}
	provider := newOpenStackDestroyProvider(runner)

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	// Verify step IDs
	if steps[0].ID != "opentofu-init" {
		t.Errorf("expected first step ID 'opentofu-init', got %q", steps[0].ID)
	}
	if steps[1].ID != "opentofu-destroy" {
		t.Errorf("expected second step ID 'opentofu-destroy', got %q", steps[1].ID)
	}

	// Execute steps
	ctx := context.Background()
	for _, step := range steps {
		if err := step.Run(ctx); err != nil {
			t.Fatalf("step %q failed: %v", step.ID, err)
		}
	}

	// Verify commands were called
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 command calls, got %d", len(runner.calls))
	}

	// Verify init command
	if runner.calls[0].name != "tofu" || runner.calls[0].args[0] != "init" {
		t.Errorf("expected first command to be 'tofu init', got %s %v", runner.calls[0].name, runner.calls[0].args)
	}

	// Verify destroy command with -auto-approve
	if runner.calls[1].name != "tofu" || runner.calls[1].args[0] != "destroy" {
		t.Errorf("expected second command to be 'tofu destroy', got %s %v", runner.calls[1].name, runner.calls[1].args)
	}
	if len(runner.calls[1].args) < 2 || runner.calls[1].args[1] != "-auto-approve" {
		t.Errorf("expected -auto-approve flag, got args: %v", runner.calls[1].args)
	}

	// Verify environment contains OpenStack credentials
	if runner.calls[0].env["OS_AUTH_URL"] != "https://keystone.example.com/v3" {
		t.Errorf("expected OS_AUTH_URL in environment, got: %v", runner.calls[0].env)
	}
}

func TestOpenStackDestroyProvider_BuildSteps_NoAutoApprove(t *testing.T) {
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

	runner := &fakeDestroyCommandRunner{}
	provider := newOpenStackDestroyProvider(runner)

	// AutoApprove = false
	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: false})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	// Execute destroy step
	ctx := context.Background()
	if err := steps[1].Run(ctx); err != nil {
		t.Fatalf("destroy step failed: %v", err)
	}

	// Verify destroy command does NOT have -auto-approve
	destroyCall := runner.calls[0]
	for _, arg := range destroyCall.args {
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
	cfg.OpenTofu.Enabled = false // Disabled

	runner := &fakeDestroyCommandRunner{}
	provider := newOpenStackDestroyProvider(runner)

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

	runner := &fakeDestroyCommandRunner{}
	provider := newOpenStackDestroyProvider(runner)

	_, err := provider.BuildSteps(cfg, &DestroyInfraOptions{})
	if err == nil {
		t.Fatal("expected error when cluster directory doesn't exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestOpenStackDestroyProvider_BuildSteps_CustomTofuPath(t *testing.T) {
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
	cfg.OpenTofu.Path = "/custom/path/tofu"

	runner := &fakeDestroyCommandRunner{}
	provider := newOpenStackDestroyProvider(runner)

	steps, err := provider.BuildSteps(cfg, &DestroyInfraOptions{AutoApprove: true})
	if err != nil {
		t.Fatalf("BuildSteps failed: %v", err)
	}

	// Execute init step
	ctx := context.Background()
	if err := steps[0].Run(ctx); err != nil {
		t.Fatalf("init step failed: %v", err)
	}

	// Verify custom path was used
	if runner.calls[0].name != "/custom/path/tofu" {
		t.Errorf("expected custom tofu path, got %q", runner.calls[0].name)
	}
}
