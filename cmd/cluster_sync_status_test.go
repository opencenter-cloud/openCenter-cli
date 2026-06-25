package cmd

import (
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

func TestUpdateServiceStatusIsNoOp(t *testing.T) {
	cfg := &v2.Config{}
	cfg.OpenCenter.Services = make(v2.ServiceMap)
	cfg.OpenCenter.Services["test-service"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true},
	}

	// updateServiceStatus is a no-op — should not panic or modify anything
	updateServiceStatus(cfg, "test-service", "success")
	updateServiceStatus(cfg, "non-existent", "success")

	cfg.OpenCenter.Services = nil
	updateServiceStatus(cfg, "test-service", "success")
}
