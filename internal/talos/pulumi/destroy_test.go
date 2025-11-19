package pulumi

import (
	"context"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

func TestNewDestroyEngine(t *testing.T) {
	tests := []struct {
		name    string
		manager *Manager
		logger  Logger
		wantErr bool
	}{
		{
			name: "valid destroy engine",
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
			engine, err := NewDestroyEngine(tt.manager, tt.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDestroyEngine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && engine == nil {
				t.Error("NewDestroyEngine() returned nil engine")
			}
		})
	}
}

func TestDestroyEngine_ExecuteDestroy(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, err := NewDestroyEngine(manager, logger)
	if err != nil {
		t.Fatalf("Failed to create destroy engine: %v", err)
	}

	ctx := context.Background()
	err = engine.ExecuteDestroy(ctx)
	if err != nil {
		t.Errorf("ExecuteDestroy() error = %v", err)
	}
}

func TestDestroyEngine_HandleResourceDependencies(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewDestroyEngine(manager, logger)

	ctx := context.Background()
	err := engine.HandleResourceDependencies(ctx)
	if err != nil {
		t.Errorf("HandleResourceDependencies() error = %v", err)
	}
}

func TestDestroyEngine_CleanupState(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewDestroyEngine(manager, logger)

	ctx := context.Background()
	err := engine.CleanupState(ctx)
	if err != nil {
		t.Errorf("CleanupState() error = %v", err)
	}
}

func TestDestroyEngine_GetDestroyOrder(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewDestroyEngine(manager, logger)

	order := engine.GetDestroyOrder()
	if len(order) == 0 {
		t.Error("GetDestroyOrder() returned empty order")
	}

	// Verify that secrets come last
	lastType := order[len(order)-1]
	if lastType != "openstack:keymanager/container:Container" {
		t.Errorf("Last resource type should be secrets container, got %s", lastType)
	}

	// Verify that compute resources come before network resources
	computeIndex := -1
	networkIndex := -1
	for i, resourceType := range order {
		if resourceType == "openstack:compute/instance:Instance" {
			computeIndex = i
		}
		if resourceType == "openstack:networking/network:Network" {
			networkIndex = i
		}
	}

	if computeIndex >= networkIndex {
		t.Error("Compute resources should be destroyed before network resources")
	}
}

func TestDestroyEngine_ConfirmDestroy(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewDestroyEngine(manager, logger)

	ctx := context.Background()
	confirmed, err := engine.ConfirmDestroy(ctx)
	if err != nil {
		t.Errorf("ConfirmDestroy() error = %v", err)
	}

	// In test mode, should return true
	if !confirmed {
		t.Error("ConfirmDestroy() should return true in test mode")
	}
}

func TestDestroyEngine_GetResourceCount(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewDestroyEngine(manager, logger)

	ctx := context.Background()
	count, err := engine.GetResourceCount(ctx)
	if err != nil {
		t.Errorf("GetResourceCount() error = %v", err)
	}

	if count < 0 {
		t.Error("GetResourceCount() should not return negative count")
	}
}

func TestDestroyEngine_VerifyDestroyCompletion(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewDestroyEngine(manager, logger)

	ctx := context.Background()
	err := engine.VerifyDestroyCompletion(ctx)
	if err != nil {
		t.Errorf("VerifyDestroyCompletion() error = %v", err)
	}
}

func TestDestroyEngine_HandleDestroyProgress(t *testing.T) {
	logger := &testLogger{}
	manager, _ := NewManager(&talos.TalosPulumiConfig{
		StackName:      "test-stack",
		SwiftContainer: "test-container",
	}, "test-project", logger)

	engine, _ := NewDestroyEngine(manager, logger)

	// Create a progress channel
	progressChan := make(chan DestroyProgress, 3)

	// Send some progress updates
	progressChan <- DestroyProgress{
		Type:         DestroyProgressTypeResource,
		ResourceType: "openstack:compute/instance:Instance",
		ResourceName: "test-instance",
		Status:       "destroying",
	}

	progressChan <- DestroyProgress{
		Type:    DestroyProgressTypeMessage,
		Message: "Destroying resources...",
	}

	progressChan <- DestroyProgress{
		Type:  DestroyProgressTypeError,
		Error: nil,
	}

	close(progressChan)

	ctx := context.Background()
	err := engine.HandleDestroyProgress(ctx, progressChan)
	if err != nil {
		t.Errorf("HandleDestroyProgress() error = %v", err)
	}
}

func TestDestroyEngine_validateDestroyConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *talos.TalosPulumiConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			},
			wantErr: false,
		},
		{
			name: "empty stack name",
			config: &talos.TalosPulumiConfig{
				StackName:      "",
				SwiftContainer: "test-container",
			},
			wantErr: true,
		},
		{
			name: "empty swift container",
			config: &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &testLogger{}
			manager, err := NewManager(tt.config, "test-project", logger)

			// If NewManager fails, that's expected for invalid configs
			if err != nil {
				if !tt.wantErr {
					t.Errorf("NewManager() unexpected error = %v", err)
				}
				return
			}

			engine, err := NewDestroyEngine(manager, logger)
			if err != nil {
				t.Fatalf("NewDestroyEngine() error = %v", err)
			}

			err = engine.validateDestroyConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDestroyConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
