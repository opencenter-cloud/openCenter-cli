package pulumi

import (
	"context"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name      string
		config    *talos.TalosPulumiConfig
		projectID string
		logger    Logger
		wantErr   bool
	}{
		{
			name: "valid configuration",
			config: &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
				SwiftPrefix:    "test/",
			},
			projectID: "test-project",
			logger:    &testLogger{},
			wantErr:   false,
		},
		{
			name:      "nil config",
			config:    nil,
			projectID: "test-project",
			logger:    &testLogger{},
			wantErr:   true,
		},
		{
			name: "empty stack name",
			config: &talos.TalosPulumiConfig{
				StackName:      "",
				SwiftContainer: "test-container",
			},
			projectID: "test-project",
			logger:    &testLogger{},
			wantErr:   true,
		},
		{
			name: "empty swift container",
			config: &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "",
			},
			projectID: "test-project",
			logger:    &testLogger{},
			wantErr:   true,
		},
		{
			name: "nil logger",
			config: &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			},
			projectID: "test-project",
			logger:    nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.config, tt.projectID, tt.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && manager == nil {
				t.Error("NewManager() returned nil manager")
			}
		})
	}
}

func TestManager_Initialize(t *testing.T) {
	tests := []struct {
		name         string
		initialConfig *talos.TalosPulumiConfig
		updateConfig  *talos.TalosPulumiConfig
		wantErr      bool
	}{
		{
			name: "valid initialization",
			initialConfig: &talos.TalosPulumiConfig{
				StackName:      "initial-stack",
				SwiftContainer: "initial-container",
			},
			updateConfig: &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
				SwiftPrefix:    "test/",
			},
			wantErr: false,
		},
		{
			name: "update with partial config",
			initialConfig: &talos.TalosPulumiConfig{
				StackName:      "initial-stack",
				SwiftContainer: "initial-container",
			},
			updateConfig: &talos.TalosPulumiConfig{
				SwiftPrefix: "new-prefix/",
			},
			wantErr: false,
		},
		{
			name: "invalid initial config",
			initialConfig: &talos.TalosPulumiConfig{
				StackName:      "",
				SwiftContainer: "initial-container",
			},
			updateConfig: &talos.TalosPulumiConfig{
				SwiftPrefix: "test/",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &testLogger{}
			manager, err := NewManager(tt.initialConfig, "test-project", logger)
			
			// If NewManager fails, that's expected for invalid configs
			if err != nil {
				if !tt.wantErr {
					t.Errorf("NewManager() unexpected error = %v", err)
				}
				return
			}

			ctx := context.Background()
			err = manager.Initialize(ctx, tt.updateConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("Manager.Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_Preview(t *testing.T) {
	logger := &testLogger{}
	config := &talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}
	manager, err := NewManager(config, "test-project", logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	preview, err := manager.Preview(ctx)
	if err != nil {
		t.Errorf("Manager.Preview() error = %v", err)
	}

	if preview == nil {
		t.Error("Manager.Preview() returned nil preview")
	}

	if preview.Creates == nil {
		t.Error("Preview.Creates should not be nil")
	}

	if preview.Updates == nil {
		t.Error("Preview.Updates should not be nil")
	}

	if preview.Deletes == nil {
		t.Error("Preview.Deletes should not be nil")
	}

	if preview.Replaces == nil {
		t.Error("Preview.Replaces should not be nil")
	}
}

func TestManager_Apply(t *testing.T) {
	logger := &testLogger{}
	config := &talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}
	manager, err := NewManager(config, "test-project", logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	result, err := manager.Apply(ctx)
	if err != nil {
		t.Errorf("Manager.Apply() error = %v", err)
	}

	if result == nil {
		t.Error("Manager.Apply() returned nil result")
	}

	if result.Outputs == nil {
		t.Error("Result.Outputs should not be nil")
	}
}

func TestManager_Refresh(t *testing.T) {
	logger := &testLogger{}
	config := &talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}
	manager, err := NewManager(config, "test-project", logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	report, err := manager.Refresh(ctx)
	if err != nil {
		t.Errorf("Manager.Refresh() error = %v", err)
	}

	if report == nil {
		t.Error("Manager.Refresh() returned nil report")
	}

	if report.Drifted == nil {
		t.Error("Report.Drifted should not be nil")
	}

	if report.Remediations == nil {
		t.Error("Report.Remediations should not be nil")
	}
}

func TestManager_Destroy(t *testing.T) {
	logger := &testLogger{}
	config := &talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}
	manager, err := NewManager(config, "test-project", logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	err = manager.Destroy(ctx)
	if err != nil {
		t.Errorf("Manager.Destroy() error = %v", err)
	}
}

func TestManager_GetOutputs(t *testing.T) {
	logger := &testLogger{}
	config := &talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}
	manager, err := NewManager(config, "test-project", logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()
	outputs, err := manager.GetOutputs(ctx)
	if err != nil {
		t.Errorf("Manager.GetOutputs() error = %v", err)
	}

	if outputs == nil {
		t.Error("Manager.GetOutputs() returned nil outputs")
	}
}

func TestManager_GetConfig(t *testing.T) {
	logger := &testLogger{}
	config := &talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
		SwiftPrefix:    "test/",
	}
	manager, err := NewManager(config, "test-project", logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	retrievedConfig := manager.GetConfig()
	if retrievedConfig == nil {
		t.Error("Manager.GetConfig() returned nil")
	}

	if retrievedConfig.StackName != config.StackName {
		t.Errorf("StackName mismatch: expected %s, got %s", config.StackName, retrievedConfig.StackName)
	}

	if retrievedConfig.SwiftContainer != config.SwiftContainer {
		t.Errorf("SwiftContainer mismatch: expected %s, got %s", config.SwiftContainer, retrievedConfig.SwiftContainer)
	}
}

func TestManager_GetProjectID(t *testing.T) {
	logger := &testLogger{}
	config := &talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}
	projectID := "test-project-123"
	manager, err := NewManager(config, projectID, logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	retrievedProjectID := manager.GetProjectID()
	if retrievedProjectID != projectID {
		t.Errorf("ProjectID mismatch: expected %s, got %s", projectID, retrievedProjectID)
	}
}
