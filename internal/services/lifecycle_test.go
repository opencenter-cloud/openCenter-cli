package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServiceDefinitionExecuteLifecycleHook tests individual lifecycle hook execution
func TestServiceDefinitionExecuteLifecycleHook(t *testing.T) {
	ctx := context.Background()
	config := map[string]interface{}{"test": "config"}

	t.Run("execute defined hook", func(t *testing.T) {
		executed := false
		service := ServiceDefinition{
			Name: "test-service",
			Lifecycle: ServiceLifecycle{
				PreInstall: func(ctx context.Context, cfg interface{}) error {
					executed = true
					assert.Equal(t, config, cfg)
					return nil
				},
			},
		}

		err := service.ExecuteLifecycleHook(ctx, "PreInstall", config)
		assert.NoError(t, err)
		assert.True(t, executed, "PreInstall hook should have been executed")
	})

	t.Run("skip undefined hook", func(t *testing.T) {
		service := ServiceDefinition{
			Name:      "test-service",
			Lifecycle: ServiceLifecycle{}, // No hooks defined
		}

		err := service.ExecuteLifecycleHook(ctx, "PreInstall", config)
		assert.NoError(t, err, "Should not error when hook is undefined")
	})

	t.Run("handle hook error", func(t *testing.T) {
		expectedErr := errors.New("hook failed")
		service := ServiceDefinition{
			Name: "test-service",
			Lifecycle: ServiceLifecycle{
				PostInstall: func(ctx context.Context, cfg interface{}) error {
					return expectedErr
				},
			},
		}

		err := service.ExecuteLifecycleHook(ctx, "PostInstall", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "PostInstall")
		assert.Contains(t, err.Error(), "test-service")
	})

	t.Run("unknown hook name", func(t *testing.T) {
		service := ServiceDefinition{
			Name:      "test-service",
			Lifecycle: ServiceLifecycle{},
		}

		err := service.ExecuteLifecycleHook(ctx, "InvalidHook", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown lifecycle hook")
	})

	t.Run("all lifecycle hooks", func(t *testing.T) {
		hooks := []string{"PreInstall", "PostInstall", "PreUpdate", "PostUpdate", "PreRemove", "PostRemove"}
		
		for _, hookName := range hooks {
			t.Run(hookName, func(t *testing.T) {
				executed := false
				lifecycle := ServiceLifecycle{}
				
				hookFunc := func(ctx context.Context, cfg interface{}) error {
					executed = true
					return nil
				}

				// Set the appropriate hook
				switch hookName {
				case "PreInstall":
					lifecycle.PreInstall = hookFunc
				case "PostInstall":
					lifecycle.PostInstall = hookFunc
				case "PreUpdate":
					lifecycle.PreUpdate = hookFunc
				case "PostUpdate":
					lifecycle.PostUpdate = hookFunc
				case "PreRemove":
					lifecycle.PreRemove = hookFunc
				case "PostRemove":
					lifecycle.PostRemove = hookFunc
				}

				service := ServiceDefinition{
					Name:      "test-service",
					Lifecycle: lifecycle,
				}

				err := service.ExecuteLifecycleHook(ctx, hookName, config)
				assert.NoError(t, err)
				assert.True(t, executed, "%s hook should have been executed", hookName)
			})
		}
	})
}

// TestRegistryExecuteLifecycleHook tests registry-level lifecycle hook execution
func TestRegistryExecuteLifecycleHook(t *testing.T) {
	ctx := context.Background()
	config := map[string]interface{}{"test": "config"}

	t.Run("execute hook for registered service", func(t *testing.T) {
		registry := NewServiceRegistry()
		executed := false

		service := ServiceDefinition{
			Name: "test-service",
			Type: ServiceTypeCore,
			Lifecycle: ServiceLifecycle{
				PreInstall: func(ctx context.Context, cfg interface{}) error {
					executed = true
					return nil
				},
			},
			Plugin: &BasicServicePlugin{
				name:        "test-service",
				serviceType: ServiceTypeCore,
			},
		}

		err := registry.RegisterService(service)
		require.NoError(t, err)

		err = registry.ExecuteLifecycleHook(ctx, "test-service", "PreInstall", config)
		assert.NoError(t, err)
		assert.True(t, executed)
	})

	t.Run("error for non-existent service", func(t *testing.T) {
		registry := NewServiceRegistry()

		err := registry.ExecuteLifecycleHook(ctx, "non-existent", "PreInstall", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non-existent")
	})
}

// TestRegistryExecuteLifecycleHooks tests batch lifecycle hook execution
func TestRegistryExecuteLifecycleHooks(t *testing.T) {
	ctx := context.Background()
	config := map[string]interface{}{"test": "config"}

	t.Run("execute hooks in dependency order", func(t *testing.T) {
		registry := NewServiceRegistry()
		executionOrder := []string{}

		// Create services with dependencies
		core := ServiceDefinition{
			Name: "core",
			Type: ServiceTypeCore,
			Lifecycle: ServiceLifecycle{
				PreInstall: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "core")
					return nil
				},
			},
			Plugin: &BasicServicePlugin{name: "core", serviceType: ServiceTypeCore},
		}

		storage := ServiceDefinition{
			Name:         "storage",
			Type:         ServiceTypeStorage,
			Dependencies: []string{"core"},
			Lifecycle: ServiceLifecycle{
				PreInstall: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "storage")
					return nil
				},
			},
			Plugin: &BasicServicePlugin{name: "storage", serviceType: ServiceTypeStorage},
		}

		monitoring := ServiceDefinition{
			Name:         "monitoring",
			Type:         ServiceTypeMonitoring,
			Dependencies: []string{"core", "storage"},
			Lifecycle: ServiceLifecycle{
				PreInstall: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "monitoring")
					return nil
				},
			},
			Plugin: &BasicServicePlugin{name: "monitoring", serviceType: ServiceTypeMonitoring},
		}

		require.NoError(t, registry.RegisterService(core))
		require.NoError(t, registry.RegisterService(storage))
		require.NoError(t, registry.RegisterService(monitoring))

		// Execute PreInstall hooks
		err := registry.ExecuteLifecycleHooks(ctx, []string{"monitoring"}, "PreInstall", config)
		assert.NoError(t, err)

		// Verify execution order: dependencies before dependents
		require.Len(t, executionOrder, 3)
		assert.Equal(t, "core", executionOrder[0], "Core should execute first")
		
		// Storage should come before monitoring
		storageIdx := -1
		monitoringIdx := -1
		for i, name := range executionOrder {
			if name == "storage" {
				storageIdx = i
			}
			if name == "monitoring" {
				monitoringIdx = i
			}
		}
		assert.True(t, storageIdx < monitoringIdx, "Storage should execute before monitoring")
	})

	t.Run("execute removal hooks in reverse order", func(t *testing.T) {
		registry := NewServiceRegistry()
		executionOrder := []string{}

		// Create services with dependencies
		core := ServiceDefinition{
			Name: "core",
			Type: ServiceTypeCore,
			Lifecycle: ServiceLifecycle{
				PreRemove: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "core")
					return nil
				},
			},
			Plugin: &BasicServicePlugin{name: "core", serviceType: ServiceTypeCore},
		}

		storage := ServiceDefinition{
			Name:         "storage",
			Type:         ServiceTypeStorage,
			Dependencies: []string{"core"},
			Lifecycle: ServiceLifecycle{
				PreRemove: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "storage")
					return nil
				},
			},
			Plugin: &BasicServicePlugin{name: "storage", serviceType: ServiceTypeStorage},
		}

		monitoring := ServiceDefinition{
			Name:         "monitoring",
			Type:         ServiceTypeMonitoring,
			Dependencies: []string{"core", "storage"},
			Lifecycle: ServiceLifecycle{
				PreRemove: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "monitoring")
					return nil
				},
			},
			Plugin: &BasicServicePlugin{name: "monitoring", serviceType: ServiceTypeMonitoring},
		}

		require.NoError(t, registry.RegisterService(core))
		require.NoError(t, registry.RegisterService(storage))
		require.NoError(t, registry.RegisterService(monitoring))

		// Execute PreRemove hooks
		err := registry.ExecuteLifecycleHooks(ctx, []string{"monitoring"}, "PreRemove", config)
		assert.NoError(t, err)

		// Verify execution order: dependents before dependencies (reverse)
		require.Len(t, executionOrder, 3)
		assert.Equal(t, "monitoring", executionOrder[0], "Monitoring should execute first for removal")
		
		// Core should come last
		assert.Equal(t, "core", executionOrder[2], "Core should execute last for removal")
	})

	t.Run("stop on first error", func(t *testing.T) {
		registry := NewServiceRegistry()
		executionOrder := []string{}
		expectedErr := errors.New("hook failed")

		core := ServiceDefinition{
			Name: "core",
			Type: ServiceTypeCore,
			Lifecycle: ServiceLifecycle{
				PreInstall: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "core")
					return nil
				},
			},
			Plugin: &BasicServicePlugin{name: "core", serviceType: ServiceTypeCore},
		}

		storage := ServiceDefinition{
			Name:         "storage",
			Type:         ServiceTypeStorage,
			Dependencies: []string{"core"},
			Lifecycle: ServiceLifecycle{
				PreInstall: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "storage")
					return expectedErr
				},
			},
			Plugin: &BasicServicePlugin{name: "storage", serviceType: ServiceTypeStorage},
		}

		monitoring := ServiceDefinition{
			Name:         "monitoring",
			Type:         ServiceTypeMonitoring,
			Dependencies: []string{"storage"},
			Lifecycle: ServiceLifecycle{
				PreInstall: func(ctx context.Context, cfg interface{}) error {
					executionOrder = append(executionOrder, "monitoring")
					return nil
				},
			},
			Plugin: &BasicServicePlugin{name: "monitoring", serviceType: ServiceTypeMonitoring},
		}

		require.NoError(t, registry.RegisterService(core))
		require.NoError(t, registry.RegisterService(storage))
		require.NoError(t, registry.RegisterService(monitoring))

		// Execute PreInstall hooks
		err := registry.ExecuteLifecycleHooks(ctx, []string{"monitoring"}, "PreInstall", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage")

		// Verify execution stopped after storage failed
		assert.Contains(t, executionOrder, "core")
		assert.Contains(t, executionOrder, "storage")
		assert.NotContains(t, executionOrder, "monitoring", "Monitoring should not execute after storage failed")
	})

	t.Run("skip undefined hooks", func(t *testing.T) {
		registry := NewServiceRegistry()

		service := ServiceDefinition{
			Name:      "test-service",
			Type:      ServiceTypeCore,
			Lifecycle: ServiceLifecycle{}, // No hooks defined
			Plugin:    &BasicServicePlugin{name: "test-service", serviceType: ServiceTypeCore},
		}

		require.NoError(t, registry.RegisterService(service))

		// Should not error when hooks are undefined
		err := registry.ExecuteLifecycleHooks(ctx, []string{"test-service"}, "PreInstall", config)
		assert.NoError(t, err)
	})
}

// TestLifecycleHookContextPropagation tests that context is properly propagated
func TestLifecycleHookContextPropagation(t *testing.T) {
	type contextKey string
	const testKey contextKey = "test-key"
	const testValue = "test-value"

	ctx := context.WithValue(context.Background(), testKey, testValue)
	config := map[string]interface{}{}

	service := ServiceDefinition{
		Name: "test-service",
		Lifecycle: ServiceLifecycle{
			PreInstall: func(ctx context.Context, cfg interface{}) error {
				value := ctx.Value(testKey)
				assert.Equal(t, testValue, value, "Context value should be propagated")
				return nil
			},
		},
	}

	err := service.ExecuteLifecycleHook(ctx, "PreInstall", config)
	assert.NoError(t, err)
}

// TestLifecycleHookConfigPropagation tests that config is properly passed to hooks
func TestLifecycleHookConfigPropagation(t *testing.T) {
	ctx := context.Background()
	config := map[string]interface{}{
		"cluster":  "test-cluster",
		"provider": "openstack",
		"replicas": 3,
	}

	service := ServiceDefinition{
		Name: "test-service",
		Lifecycle: ServiceLifecycle{
			PreInstall: func(ctx context.Context, cfg interface{}) error {
				receivedConfig, ok := cfg.(map[string]interface{})
				require.True(t, ok, "Config should be a map")
				assert.Equal(t, "test-cluster", receivedConfig["cluster"])
				assert.Equal(t, "openstack", receivedConfig["provider"])
				assert.Equal(t, 3, receivedConfig["replicas"])
				return nil
			},
		},
	}

	err := service.ExecuteLifecycleHook(ctx, "PreInstall", config)
	assert.NoError(t, err)
}
