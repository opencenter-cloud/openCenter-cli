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
	"context"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
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
		BaseConfig: services.BaseConfig{Enabled: true, Status: "unknown"},
	}

	updateServiceStatus(cfg, "test-service", "success")

	svc := cfg.OpenCenter.Services["test-service"].(*services.DefaultServiceConfig)
	if svc.Status != "success" {
		t.Errorf("expected status 'success', got %q", svc.Status)
	}
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
		BaseConfig: services.BaseConfig{Enabled: true, Status: "running"},
	}

	// This will fail to query the cluster (no kubeconfig), but should return the old status correctly
	result := syncServiceStatus(context.Background(), "/nonexistent/kubeconfig", "test-service", serviceConfig)

	if result.Name != "test-service" {
		t.Errorf("expected name 'test-service', got %q", result.Name)
	}

	if result.OldStatus != "running" {
		t.Errorf("expected old status 'running', got %q", result.OldStatus)
	}

	// New status should be "pending" since the cluster query will fail
	if result.NewStatus != "pending" {
		t.Errorf("expected new status 'pending', got %q", result.NewStatus)
	}
}

func TestNewClusterSyncStatusCmd(t *testing.T) {
	cmd := newClusterSyncStatusCmd()

	if cmd.Use != "sync-status [name]" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	// Check flags exist
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Error("expected --dry-run flag")
	}

	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag")
	}

	if cmd.Flags().Lookup("timeout") == nil {
		t.Error("expected --timeout flag")
	}
}
