package pulumi

import (
	"context"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

func TestNewRefreshEngine(t *testing.T) {
	tests := []struct {
		name    string
		manager *Manager
		logger  Logger
		wantErr bool
	}{
		{
			name: "valid refresh engine",
			manager: func() *Manager {
				m, _ := NewManager(&talos.TalosPulumiConfig{
					StackName:      "test-stack",
					SwiftContainer: "test-container",
				}, "test-project", &testLogger{})
				return m
			}(),
			logger:  &testLogger{},
			wantErr: false,
		},
		{
			name:    "nil manager",
			manager: nil,
			logger:  &testLogger{},
			wantErr: true,
		},
		{
			name: "nil logger",
			manager: func() *Manager {
				m, _ := NewManager(&talos.TalosPulumiConfig{
					StackName:      "test-stack",
					SwiftContainer: "test-container",
				}, "test-project", &testLogger{})
				return m
			}(),
			logger:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewRefreshEngine(tt.manager, tt.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRefreshEngine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && engine == nil {
				t.Error("NewRefreshEngine() returned nil engine")
			}
		})
	}
}

func TestRefreshEngine_ExecuteRefresh(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, err := NewRefreshEngine(manager, logger)
	if err != nil {
		t.Fatalf("Failed to create refresh engine: %v", err)
	}

	ctx := context.Background()
	report, err := engine.ExecuteRefresh(ctx)
	if err != nil {
		t.Errorf("ExecuteRefresh() error = %v", err)
	}

	if report == nil {
		t.Error("ExecuteRefresh() returned nil report")
	}

	if report.Drifted == nil {
		t.Error("Report.Drifted should not be nil")
	}

	if report.Remediations == nil {
		t.Error("Report.Remediations should not be nil")
	}
}

func TestRefreshEngine_DetectConfigurationDrift(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewRefreshEngine(manager, logger)

	ctx := context.Background()
	drifted, err := engine.DetectConfigurationDrift(ctx)
	if err != nil {
		t.Errorf("DetectConfigurationDrift() error = %v", err)
	}

	if drifted == nil {
		t.Error("DetectConfigurationDrift() returned nil")
	}
}

func TestRefreshEngine_CompareSecurityPolicies(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewRefreshEngine(manager, logger)

	ctx := context.Background()
	drifted, err := engine.CompareSecurityPolicies(ctx)
	if err != nil {
		t.Errorf("CompareSecurityPolicies() error = %v", err)
	}

	if drifted == nil {
		t.Error("CompareSecurityPolicies() returned nil")
	}
}

func TestRefreshEngine_GenerateDriftReport(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewRefreshEngine(manager, logger)

	tests := []struct {
		name    string
		drifted []talos.DriftedResource
		wantErr bool
	}{
		{
			name:    "no drift",
			drifted: []talos.DriftedResource{},
			wantErr: false,
		},
		{
			name: "single drifted resource",
			drifted: []talos.DriftedResource{
				{
					Type: "openstack:networking/network:Network",
					Name: "test-network",
					Expected: map[string]interface{}{
						"cidr": "10.0.0.0/24",
					},
					Actual: map[string]interface{}{
						"cidr": "10.0.1.0/24",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple drifted resources",
			drifted: []talos.DriftedResource{
				{
					Type: "openstack:networking/network:Network",
					Name: "test-network",
				},
				{
					Type: "openstack:compute/instance:Instance",
					Name: "test-instance",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			report, err := engine.GenerateDriftReport(ctx, tt.drifted)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateDriftReport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if report == nil {
				t.Error("GenerateDriftReport() returned nil")
				return
			}

			if report.HasDrift != (len(tt.drifted) > 0) {
				t.Errorf("HasDrift mismatch: expected %v, got %v", len(tt.drifted) > 0, report.HasDrift)
			}

			if len(report.Remediations) != len(tt.drifted) {
				t.Errorf("Remediation count mismatch: expected %d, got %d", len(tt.drifted), len(report.Remediations))
			}
		})
	}
}

func TestRefreshEngine_GetDriftedResourcesByType(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewRefreshEngine(manager, logger)

	report := &talos.DriftReport{
		HasDrift: true,
		Drifted: []talos.DriftedResource{
			{
				Type: "openstack:networking/network:Network",
				Name: "network-1",
			},
			{
				Type: "openstack:networking/network:Network",
				Name: "network-2",
			},
			{
				Type: "openstack:compute/instance:Instance",
				Name: "instance-1",
			},
		},
	}

	tests := []struct {
		name         string
		resourceType string
		wantCount    int
	}{
		{
			name:         "filter networks",
			resourceType: "openstack:networking/network:Network",
			wantCount:    2,
		},
		{
			name:         "filter instances",
			resourceType: "openstack:compute/instance:Instance",
			wantCount:    1,
		},
		{
			name:         "filter non-existent type",
			resourceType: "openstack:storage/volume:Volume",
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := engine.GetDriftedResourcesByType(report, tt.resourceType)
			if len(filtered) != tt.wantCount {
				t.Errorf("GetDriftedResourcesByType() count = %d, want %d", len(filtered), tt.wantCount)
			}
		})
	}
}

func TestRefreshEngine_HasSecurityDrift(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewRefreshEngine(manager, logger)

	tests := []struct {
		name   string
		report *talos.DriftReport
		want   bool
	}{
		{
			name:   "nil report",
			report: nil,
			want:   false,
		},
		{
			name: "no drift",
			report: &talos.DriftReport{
				HasDrift: false,
				Drifted:  []talos.DriftedResource{},
			},
			want: false,
		},
		{
			name: "security group drift",
			report: &talos.DriftReport{
				HasDrift: true,
				Drifted: []talos.DriftedResource{
					{
						Type: "openstack:networking/securityGroup:SecurityGroup",
						Name: "test-sg",
					},
				},
			},
			want: true,
		},
		{
			name: "non-security drift",
			report: &talos.DriftReport{
				HasDrift: true,
				Drifted: []talos.DriftedResource{
					{
						Type: "openstack:networking/network:Network",
						Name: "test-network",
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.HasSecurityDrift(tt.report)
			if got != tt.want {
				t.Errorf("HasSecurityDrift() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefreshEngine_HasNetworkDrift(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewRefreshEngine(manager, logger)

	tests := []struct {
		name   string
		report *talos.DriftReport
		want   bool
	}{
		{
			name:   "nil report",
			report: nil,
			want:   false,
		},
		{
			name: "no drift",
			report: &talos.DriftReport{
				HasDrift: false,
				Drifted:  []talos.DriftedResource{},
			},
			want: false,
		},
		{
			name: "network drift",
			report: &talos.DriftReport{
				HasDrift: true,
				Drifted: []talos.DriftedResource{
					{
						Type: "openstack:networking/network:Network",
						Name: "test-network",
					},
				},
			},
			want: true,
		},
		{
			name: "non-network drift",
			report: &talos.DriftReport{
				HasDrift: true,
				Drifted: []talos.DriftedResource{
					{
						Type: "openstack:compute/instance:Instance",
						Name: "test-instance",
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.HasNetworkDrift(tt.report)
			if got != tt.want {
				t.Errorf("HasNetworkDrift() = %v, want %v", got, tt.want)
			}
		})
	}
}
