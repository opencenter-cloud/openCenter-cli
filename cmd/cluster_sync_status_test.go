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

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
)

func TestParseFluxResourceStatus(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected string
		wantErr  bool
	}{
		{
			name: "ready true returns success",
			json: `{
				"status": {
					"conditions": [
						{"type": "Ready", "status": "True", "reason": "ReconciliationSucceeded"}
					]
				}
			}`,
			expected: "success",
		},
		{
			name: "ready false with progress returns running",
			json: `{
				"status": {
					"conditions": [
						{"type": "Ready", "status": "False", "reason": "Progressing"}
					]
				}
			}`,
			expected: "running",
		},
		{
			name: "ready false with reconciling returns running",
			json: `{
				"status": {
					"conditions": [
						{"type": "Ready", "status": "False", "reason": "Reconciling"}
					]
				}
			}`,
			expected: "running",
		},
		{
			name: "ready false with failed returns failed",
			json: `{
				"status": {
					"conditions": [
						{"type": "Ready", "status": "False", "reason": "InstallFailed"}
					]
				}
			}`,
			expected: "failed",
		},
		{
			name: "ready false with error returns failed",
			json: `{
				"status": {
					"conditions": [
						{"type": "Ready", "status": "False", "reason": "ValidationError"}
					]
				}
			}`,
			expected: "failed",
		},
		{
			name: "ready false with stalled returns failed",
			json: `{
				"status": {
					"conditions": [
						{"type": "Ready", "status": "False", "reason": "Stalled"}
					]
				}
			}`,
			expected: "failed",
		},
		{
			name: "no ready condition returns pending",
			json: `{
				"status": {
					"conditions": [
						{"type": "Healthy", "status": "True"}
					]
				}
			}`,
			expected: "pending",
		},
		{
			name: "empty conditions returns pending",
			json: `{
				"status": {
					"conditions": []
				}
			}`,
			expected: "pending",
		},
		{
			name:    "invalid json returns error",
			json:    `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFluxResourceStatus([]byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetServiceNamespace(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		expected    string
	}{
		{"cert-manager", "cert-manager", "cert-manager"},
		{"calico", "calico", "calico-system"},
		{"cilium", "cilium", "kube-system"},
		{"gateway", "gateway", "gateway-system"},
		{"keycloak", "keycloak", "keycloak"},
		{"prometheus-stack", "prometheus-stack", "monitoring"},
		{"loki", "loki", "monitoring"},
		{"fluxcd", "fluxcd", "flux-system"},
		{"unknown-service", "unknown-service", "unknown-service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getServiceNamespace(tt.serviceName, nil)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCollectEnabledServices(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Services = make(v2.ServiceMap)
	cfg.OpenCenter.Services["enabled-service"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
	}
	cfg.OpenCenter.Services["disabled-service"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: false},
	}

	enabled := collectEnabledServices(cfg)

	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled service, got %d", len(enabled))
	}

	if _, ok := enabled["enabled-service"]; !ok {
		t.Error("expected enabled-service to be in the result")
	}

	if _, ok := enabled["disabled-service"]; ok {
		t.Error("disabled-service should not be in the result")
	}
}

func TestUpdateServiceStatus(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Services = make(v2.ServiceMap)
	cfg.OpenCenter.Services["test-service"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
	}

	// updateServiceStatus is now a no-op (status removed from config)
	updateServiceStatus(cfg, "test-service", "success")
}

func TestUpdateServiceStatusNonExistent(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Services = make(v2.ServiceMap)

	// Should not panic
	updateServiceStatus(cfg, "non-existent", "success")
}

func TestUpdateServiceStatusNilServices(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Services = nil

	// Should not panic
	updateServiceStatus(cfg, "test-service", "success")
}

func TestSyncServiceStatus(t *testing.T) {
	// Test with a service config that has GetStatus method
	serviceConfig := &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
	}

	// This will fail to query the cluster (no kubeconfig), but should return the old status correctly
	result := syncServiceStatus(context.Background(), "/nonexistent/kubeconfig", "test-service", serviceConfig)

	if result.Name != "test-service" {
		t.Errorf("expected name 'test-service', got %q", result.Name)
	}

	// OldStatus comes from GetStatus() which now always returns ""
	if result.OldStatus != "" {
		t.Errorf("expected old status '', got %q", result.OldStatus)
	}

	// New status should be "pending" since the cluster query will fail
	if result.NewStatus != "pending" {
		t.Errorf("expected new status 'pending', got %q", result.NewStatus)
	}
}

func TestClusterSyncStatusYAMLIsRejectedThroughClusterStatus(t *testing.T) {
	root := newOutputRootForCommandTest()
	root.SetArgs([]string{"cluster", "status", "sync-cluster", "--sync", "--output", "yaml"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected cluster status --sync --output yaml to fail")
	}
	want := "cluster status --sync does not support yaml output yet; use --output text or --output json"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestClusterSyncStatusUsesGlobalJSONOutputThroughClusterStatus(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveSyncStatusConfigForCommandTest(t, dir, "sync-cluster", "unknown")
	installFakeSyncKubectlBinary(t, t.TempDir())

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"cluster", "status", "sync-cluster", "--sync", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster status --sync --output json failed: %v", err)
	}

	var result SyncStatusResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON sync result, got %q: %v", out.String(), err)
	}
	if result.ClusterName != "sync-cluster" {
		t.Fatalf("cluster_name = %q, want sync-cluster", result.ClusterName)
	}
	if result.ServicesTotal != 1 || result.Servicessynced != 1 || result.ServicesFailed != 0 {
		t.Fatalf("unexpected sync counts: total=%d synced=%d failed=%d", result.ServicesTotal, result.Servicessynced, result.ServicesFailed)
	}

	cfg, err := loadConfig(context.Background(), "sync-cluster")
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	// Status is no longer persisted in config (runtime state only).
	// Verify the service still exists and is enabled.
	statusGetter, ok := cfg.OpenCenter.Services["fluxcd"].(interface{ GetStatus() string })
	if !ok {
		t.Fatalf("fluxcd service does not expose GetStatus: %#v", cfg.OpenCenter.Services["fluxcd"])
	}
	// GetStatus() returns "" since status is no longer stored in config
	if statusGetter.GetStatus() != "" {
		t.Fatalf("saved fluxcd status = %q, want empty (status no longer persisted)", statusGetter.GetStatus())
	}
}

func TestClusterSyncStatusJSONWithNoEnabledServicesReturnsEmptyResult(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveSyncStatusConfigForCommandTest(t, dir, "sync-cluster", "unknown", false)

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"cluster", "status", "sync-cluster", "--sync", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster status --sync --output json failed: %v", err)
	}

	var result SyncStatusResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON sync result, got %q: %v", out.String(), err)
	}
	if result.ClusterName != "sync-cluster" {
		t.Fatalf("cluster_name = %q, want sync-cluster", result.ClusterName)
	}
	if result.ServicesTotal != 0 || result.Servicessynced != 0 || result.ServicesFailed != 0 {
		t.Fatalf("unexpected sync counts: total=%d synced=%d failed=%d", result.ServicesTotal, result.Servicessynced, result.ServicesFailed)
	}
	if len(result.Results) != 0 {
		t.Fatalf("expected empty results, got %#v", result.Results)
	}
}

func TestClusterSyncStatusUsesGlobalDryRunThroughClusterStatus(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveSyncStatusConfigForCommandTest(t, dir, "sync-cluster", "unknown")
	installFakeSyncKubectlBinary(t, t.TempDir())

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"cluster", "status", "sync-cluster", "--sync", "--dry-run", "--sync-timeout", "250ms"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster status --sync --dry-run failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Sync Status (dry-run) for cluster: sync-cluster") {
		t.Fatalf("expected dry-run sync output, got:\n%s", output)
	}
	if !strings.Contains(output, "Dry-run mode: no changes saved") {
		t.Fatalf("expected dry-run save notice, got:\n%s", output)
	}
}

func TestClusterSyncStatusJSONDryRunDoesNotPersistThroughClusterStatus(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	saveSyncStatusConfigForCommandTest(t, dir, "sync-cluster", "unknown")
	installFakeSyncKubectlBinary(t, t.TempDir())

	root := newOutputRootForCommandTest()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"cluster", "status", "sync-cluster", "--sync", "--dry-run", "--output", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cluster status --sync --dry-run --output json failed: %v", err)
	}

	var result SyncStatusResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON sync result, got %q: %v", out.String(), err)
	}
	if result.ServicesTotal != 1 || result.Servicessynced != 1 || result.ServicesFailed != 0 {
		t.Fatalf("unexpected sync counts: total=%d synced=%d failed=%d", result.ServicesTotal, result.Servicessynced, result.ServicesFailed)
	}

	cfg, err := loadConfig(context.Background(), "sync-cluster")
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	statusGetter, ok := cfg.OpenCenter.Services["fluxcd"].(interface{ GetStatus() string })
	if !ok {
		t.Fatalf("fluxcd service does not expose GetStatus: %#v", cfg.OpenCenter.Services["fluxcd"])
	}
	// Status is no longer persisted — always returns ""
	if statusGetter.GetStatus() != "" {
		t.Fatalf("saved fluxcd status = %q, want empty (status no longer persisted)", statusGetter.GetStatus())
	}
}

func saveSyncStatusConfigForCommandTest(t *testing.T, dir, clusterName, status string, enabledOverride ...bool) *paths.ClusterPaths {
	t.Helper()

	resolver, clusterPaths := createClusterDirectoriesForTest(t, dir, clusterName, "opencenter")
	cfgPtr, err := v2.NewV2Default(clusterName, "kind")
	if err != nil {
		t.Fatalf("create native v2 kind config: %v", err)
	}
	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Meta.Organization = "opencenter"
	cfg.OpenCenter.GitOps.Repository.LocalDir = clusterPaths.GitOpsDir
	cfg.OpenCenter.Services = make(v2.ServiceMap)
	enabled := true
	if len(enabledOverride) > 0 {
		enabled = enabledOverride[0]
	}
	cfg.OpenCenter.Services["fluxcd"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: enabled},
	}

	kubeconfigPath := filepath.Join(clusterPaths.GitOpsDir, "infrastructure", "clusters", clusterName, "kubeconfig.yaml")
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0o755); err != nil {
		t.Fatalf("mkdir kubeconfig dir: %v", err)
	}
	if err := os.WriteFile(kubeconfigPath, []byte("apiVersion: v1\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	testhelpers.SaveConfigWithPathResolver(t, cfg, resolver)
	return clusterPaths
}

func installFakeSyncKubectlBinary(t *testing.T, binDir string) {
	t.Helper()

	writeFakeExecutable(t, filepath.Join(binDir, "kubectl"), `#!/bin/sh
set -eu
if [ "${1:-}" = "--kubeconfig" ]; then
  shift 2
fi

if [ "${1:-}" = "get" ] && [ "${2:-}" = "helmrelease" ]; then
  cat <<'JSON'
{"status":{"conditions":[{"type":"Ready","status":"True","reason":"ReconciliationSucceeded"}]}}
JSON
  exit 0
fi

echo "unsupported fake kubectl invocation: $*" >&2
exit 1
`)
	prependTestPath(t, binDir)
}
