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

package config

import (
	"context"
	"testing"
)

// mockConfigLoader is a simple mock for testing
type mockConfigLoader struct{}

func (m *mockConfigLoader) LoadFromFile(ctx context.Context, filePath string) (*Config, error) {
	return nil, nil
}

func (m *mockConfigLoader) LoadFromBytes(ctx context.Context, data []byte, clusterName string) (*Config, error) {
	return nil, nil
}

func (m *mockConfigLoader) LoadDefault(ctx context.Context, clusterName string) (*Config, error) {
	return nil, nil
}

func (m *mockConfigLoader) GenerateCompleteConfig(ctx context.Context, clusterName string) (*Config, error) {
	return nil, nil
}

func (m *mockConfigLoader) LoadFromPath(ctx context.Context, clusterName string) (*Config, error) {
	return nil, nil
}

// mockPathResolver is a simple mock for testing
type mockPathResolver struct{}

func (m *mockPathResolver) ResolveClusterPaths(ctx context.Context, clusterName, organization string) (*OrganizationClusterPaths, error) {
	return nil, nil
}

func (m *mockPathResolver) CreateClusterDirectories(ctx context.Context, clusterName, organization string) error {
	return nil
}

func (m *mockPathResolver) CreateOrganizationStructure(ctx context.Context, organization string) error {
	return nil
}

func (m *mockPathResolver) ValidatePath(ctx context.Context, path string) error {
	return nil
}

func (m *mockPathResolver) IsLegacyCluster(ctx context.Context, clusterName string) (bool, error) {
	return false, nil
}

func (m *mockPathResolver) GetClusterOrganization(ctx context.Context, clusterName string) (string, error) {
	return "opencenter", nil
}

// mockConfigCache is a simple mock for testing
type mockConfigCache struct{}

func (m *mockConfigCache) Get(ctx context.Context, key string) (*Config, bool) {
	return nil, false
}

func (m *mockConfigCache) Set(ctx context.Context, key string, config *Config) error {
	return nil
}

func (m *mockConfigCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (m *mockConfigCache) Clear(ctx context.Context) error {
	return nil
}

func (m *mockConfigCache) InvalidateCluster(ctx context.Context, clusterName string) error {
	return nil
}

// TestConfigurationManager_ValidationEngineIntegration tests that the ValidationEngine
// is properly integrated into the ConfigurationManager.
func TestConfigurationManager_ValidationEngineIntegration(t *testing.T) {
	// Create a configuration manager with all dependencies
	loader := &mockConfigLoader{}
	validator := NewConfigValidator(false)
	pathResolver := &mockPathResolver{}
	cache := &mockConfigCache{}
	
	manager := NewConfigurationManager(loader, validator, pathResolver, cache)
	
	// Verify ValidationEngine is initialized
	if manager.validationEngine == nil {
		t.Fatal("ValidationEngine should be initialized")
	}
	
	// Verify core validators are registered
	validators := manager.validationEngine.List()
	expectedValidators := []string{"cluster-name", "network", "config", "security"}
	
	for _, expected := range expectedValidators {
		found := false
		for _, v := range validators {
			if v == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected validator %q to be registered, but it was not found", expected)
		}
	}
}

// TestConfigurationManager_ValidateConfigWithEngine tests the new ValidateConfigWithEngine method.
func TestConfigurationManager_ValidateConfigWithEngine(t *testing.T) {
	loader := &mockConfigLoader{}
	validator := NewConfigValidator(false)
	pathResolver := &mockPathResolver{}
	cache := &mockConfigCache{}
	
	manager := NewConfigurationManager(loader, validator, pathResolver, cache)
	ctx := context.Background()
	
	t.Run("nil config", func(t *testing.T) {
		result := manager.ValidateConfigWithEngine(ctx, nil)
		if result.Valid {
			t.Error("Expected validation to fail for nil config")
		}
		if len(result.Errors) == 0 {
			t.Error("Expected at least one error for nil config")
		}
	})
	
	t.Run("valid cluster name", func(t *testing.T) {
		config := NewDefault("test-cluster")
		result := manager.ValidateConfigWithEngine(ctx, &config)
		
		// Debug: print all errors
		for _, err := range result.Errors {
			t.Logf("Error field: %s, message: %s", err.Field, err.Message)
		}
		
		// Check if there are any cluster name errors
		hasClusterNameError := false
		for _, err := range result.Errors {
			if err.Field == "opencenter.cluster.cluster_name" {
				hasClusterNameError = true
				t.Errorf("Unexpected cluster name validation error: %s", err.Message)
			}
		}
		
		// Valid cluster name should not have errors
		if hasClusterNameError {
			t.Error("Valid cluster name should not produce validation errors")
		}
	})
	
	t.Run("invalid cluster name with uppercase", func(t *testing.T) {
		config := NewDefault("Invalid-Cluster-Name")
		result := manager.ValidateConfigWithEngine(ctx, &config)
		
		// Should have cluster name validation error
		foundError := false
		for _, err := range result.Errors {
			if err.Field == "opencenter.cluster.cluster_name" {
				foundError = true
				if len(err.Suggestions) == 0 {
					t.Error("Expected suggestions for invalid cluster name")
				}
			}
		}
		if !foundError {
			t.Error("Expected cluster name validation error for invalid name with uppercase")
		}
	})
	
	t.Run("invalid cluster name with underscore", func(t *testing.T) {
		config := NewDefault("invalid_cluster_name")
		result := manager.ValidateConfigWithEngine(ctx, &config)
		
		// Should have cluster name validation error
		foundError := false
		for _, err := range result.Errors {
			if err.Field == "opencenter.cluster.cluster_name" {
				foundError = true
				if len(err.Suggestions) == 0 {
					t.Error("Expected suggestions for invalid cluster name")
				}
			}
		}
		if !foundError {
			t.Error("Expected cluster name validation error for invalid name with underscore")
		}
	})
}

// TestConfigurationManager_ClusterNameValidation tests cluster name validation
// in various ConfigurationManager methods.
func TestConfigurationManager_ClusterNameValidation(t *testing.T) {
	loader := &mockConfigLoader{}
	validator := NewConfigValidator(false)
	pathResolver := &mockPathResolver{}
	cache := &mockConfigCache{}
	
	manager := NewConfigurationManager(loader, validator, pathResolver, cache)
	ctx := context.Background()
	
	t.Run("LoadConfig with invalid name", func(t *testing.T) {
		_, err := manager.LoadConfig(ctx, "Invalid_Name")
		if err == nil {
			t.Error("Expected error for invalid cluster name")
		}
	})
	
	t.Run("DeleteConfig with invalid name", func(t *testing.T) {
		err := manager.DeleteConfig(ctx, "Invalid_Name")
		if err == nil {
			t.Error("Expected error for invalid cluster name")
		}
	})
	
	t.Run("GetConfigPath with invalid name", func(t *testing.T) {
		_, err := manager.GetConfigPath(ctx, "Invalid_Name")
		if err == nil {
			t.Error("Expected error for invalid cluster name")
		}
	})
	
	t.Run("SetActiveConfig with invalid name", func(t *testing.T) {
		err := manager.SetActiveConfig(ctx, "Invalid_Name")
		if err == nil {
			t.Error("Expected error for invalid cluster name")
		}
	})
}

// TestConfigurationManager_GetValidationEngine tests the GetValidationEngine method.
func TestConfigurationManager_GetValidationEngine(t *testing.T) {
	loader := &mockConfigLoader{}
	validator := NewConfigValidator(false)
	pathResolver := &mockPathResolver{}
	cache := &mockConfigCache{}
	
	manager := NewConfigurationManager(loader, validator, pathResolver, cache)
	
	engine := manager.GetValidationEngine()
	if engine == nil {
		t.Fatal("GetValidationEngine should return non-nil engine")
	}
	
	// Verify we can use the engine directly
	ctx := context.Background()
	result, err := engine.Validate(ctx, "cluster-name", "test-cluster")
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}
	if !result.Valid {
		t.Error("Expected valid cluster name to pass validation")
	}
}
