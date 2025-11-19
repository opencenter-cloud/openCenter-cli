/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"strings"
	"testing"
)

func TestNetworkPluginHandlers(t *testing.T) {
	handler := NewDefaultNetworkPluginHandler()

	tests := []struct {
		name           string
		pluginType     string
		config         map[string]interface{}
		expectError    bool
		expectedRender string
	}{
		{
			name:       "Calico with default config",
			pluginType: "calico",
			config: map[string]interface{}{
				"enabled": true,
			},
			expectError:    false,
			expectedRender: "network_plugin = \"calico\"",
		},
		{
			name:       "Cilium with default config",
			pluginType: "cilium",
			config: map[string]interface{}{
				"enabled": true,
			},
			expectError:    false,
			expectedRender: "network_plugin = \"cilium\"",
		},
		{
			name:       "Kube-OVN with default config",
			pluginType: "kube-ovn",
			config: map[string]interface{}{
				"enabled": true,
			},
			expectError:    false,
			expectedRender: "network_plugin = \"kube-ovn\"",
		},
		{
			name:       "Calico with custom MTU",
			pluginType: "calico",
			config: map[string]interface{}{
				"enabled": true,
				"mtu":     1500,
			},
			expectError:    false,
			expectedRender: "calico_mtu = 1500",
		},
		{
			name:       "Cilium with Hubble enabled",
			pluginType: "cilium",
			config: map[string]interface{}{
				"enabled": true,
				"hubble": map[string]interface{}{
					"enabled": true,
				},
			},
			expectError:    false,
			expectedRender: "cilium_hubble_enabled = true",
		},
		{
			name:       "Invalid plugin type",
			pluginType: "invalid",
			config: map[string]interface{}{
				"enabled": true,
			},
			expectError: true,
		},
		{
			name:       "Calico with invalid MTU",
			pluginType: "calico",
			config: map[string]interface{}{
				"enabled": true,
				"mtu":     10000, // Too high
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation
			err := handler.ValidateNetworkPlugin(tt.pluginType, tt.config)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Test rendering if validation should pass
			if !tt.expectError {
				rendered, err := handler.RenderNetworkPluginConfig(tt.pluginType, tt.config)
				if err != nil {
					t.Errorf("Unexpected rendering error: %v", err)
				}
				if tt.expectedRender != "" && !strings.Contains(rendered, tt.expectedRender) {
					t.Errorf("Expected rendered config to contain '%s', got: %s", tt.expectedRender, rendered)
				}
			}
		})
	}
}

func TestNetworkPluginMutualExclusivity(t *testing.T) {
	handler := NewDefaultNetworkPluginHandler()

	tests := []struct {
		name          string
		pluginConfigs map[string]map[string]interface{}
		expectError   bool
		errorContains string
	}{
		{
			name: "Only Calico enabled - valid",
			pluginConfigs: map[string]map[string]interface{}{
				"calico": {
					"enabled": true,
				},
				"cilium": {
					"enabled": false,
				},
				"kube-ovn": {
					"enabled": false,
				},
			},
			expectError: false,
		},
		{
			name: "Multiple plugins enabled - invalid",
			pluginConfigs: map[string]map[string]interface{}{
				"calico": {
					"enabled": true,
				},
				"cilium": {
					"enabled": true,
				},
				"kube-ovn": {
					"enabled": false,
				},
			},
			expectError:   true,
			errorContains: "only one network plugin can be enabled",
		},
		{
			name: "No plugins enabled - invalid",
			pluginConfigs: map[string]map[string]interface{}{
				"calico": {
					"enabled": false,
				},
				"cilium": {
					"enabled": false,
				},
				"kube-ovn": {
					"enabled": false,
				},
			},
			expectError:   true,
			errorContains: "at least one network plugin must be enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateNetworkPluginMutualExclusivity(tt.pluginConfigs)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectError && tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
			}
		})
	}
}

func TestNetworkPluginCompatibility(t *testing.T) {
	handler := NewDefaultNetworkPluginHandler()

	tests := []struct {
		name          string
		pluginType    string
		config        map[string]interface{}
		globalConfig  map[string]interface{}
		expectError   bool
		errorContains string
	}{
		{
			name:       "Kube-OVN with Cilium integration - valid when Cilium not primary",
			pluginType: "kube-ovn",
			config: map[string]interface{}{
				"enabled":            true,
				"cilium_integration": true,
			},
			globalConfig: map[string]interface{}{
				"cilium": map[string]interface{}{
					"enabled": false,
				},
			},
			expectError: false,
		},
		{
			name:       "Kube-OVN with Cilium integration - invalid when Cilium is primary",
			pluginType: "kube-ovn",
			config: map[string]interface{}{
				"enabled":            true,
				"cilium_integration": true,
			},
			globalConfig: map[string]interface{}{
				"cilium": map[string]interface{}{
					"enabled": true,
				},
			},
			expectError:   true,
			errorContains: "kube-ovn with cilium_integration cannot be used when cilium is also enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateNetworkPluginCompatibility(tt.pluginType, tt.config, tt.globalConfig)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectError && tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
			}
		})
	}
}

func TestGetPluginHandler(t *testing.T) {
	handler := NewDefaultNetworkPluginHandler()

	// Test getting valid handlers
	validPlugins := []string{"calico", "cilium", "kube-ovn"}
	for _, plugin := range validPlugins {
		t.Run("Get "+plugin+" handler", func(t *testing.T) {
			pluginHandler, err := handler.GetPluginHandler(plugin)
			if err != nil {
				t.Errorf("Unexpected error getting %s handler: %v", plugin, err)
			}
			if pluginHandler == nil {
				t.Errorf("Expected non-nil handler for %s", plugin)
			}
			if pluginHandler.GetPluginName() != plugin {
				t.Errorf("Expected plugin name %s, got %s", plugin, pluginHandler.GetPluginName())
			}
		})
	}

	// Test getting invalid handler
	t.Run("Get invalid handler", func(t *testing.T) {
		_, err := handler.GetPluginHandler("invalid")
		if err == nil {
			t.Errorf("Expected error for invalid plugin handler")
		}
	})

	// Test getting all handlers
	t.Run("Get all handlers", func(t *testing.T) {
		allHandlers := handler.GetAllPluginHandlers()
		if len(allHandlers) != 3 {
			t.Errorf("Expected 3 handlers, got %d", len(allHandlers))
		}
		for _, plugin := range validPlugins {
			if _, exists := allHandlers[plugin]; !exists {
				t.Errorf("Expected handler for %s to exist", plugin)
			}
		}
	})
}
