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
	"fmt"
	"strings"
)

// DefaultNetworkPluginHandler implements NetworkPluginHandler interface
type DefaultNetworkPluginHandler struct {
	supportedPlugins map[string]NetworkPluginConfig
	pluginHandlers   map[string]SpecificNetworkPluginHandler
}

// SpecificNetworkPluginHandler interface for individual network plugin handlers
type SpecificNetworkPluginHandler interface {
	ValidateConfiguration(config map[string]interface{}) error
	RenderConfiguration(config map[string]interface{}) (string, error)
	GetRequiredFields() []string
	GetDefaultConfig() map[string]interface{}
	GetPluginName() string
}

// NetworkPluginConfig represents configuration for a network plugin
type NetworkPluginConfig struct {
	Name           string
	RequiredFields []string
	DefaultConfig  map[string]interface{}
	Validator      func(config map[string]interface{}) error
}

// NewDefaultNetworkPluginHandler creates a new default network plugin handler
func NewDefaultNetworkPluginHandler() *DefaultNetworkPluginHandler {
	handler := &DefaultNetworkPluginHandler{
		supportedPlugins: make(map[string]NetworkPluginConfig),
		pluginHandlers:   make(map[string]SpecificNetworkPluginHandler),
	}

	// Initialize specific plugin handlers
	handler.initializePluginHandlers()

	// Initialize supported plugins
	handler.initializeSupportedPlugins()

	return handler
}

// initializePluginHandlers initializes the specific plugin handlers
func (h *DefaultNetworkPluginHandler) initializePluginHandlers() {
	h.pluginHandlers["calico"] = NewCalicoHandler()
	h.pluginHandlers["cilium"] = NewCiliumHandler()
	h.pluginHandlers["kube-ovn"] = NewKubeOVNHandler()
}

// initializeSupportedPlugins initializes the supported network plugins using dedicated handlers
func (h *DefaultNetworkPluginHandler) initializeSupportedPlugins() {
	// Initialize plugins using dedicated handlers
	for pluginName, handler := range h.pluginHandlers {
		h.supportedPlugins[pluginName] = NetworkPluginConfig{
			Name:           handler.GetPluginName(),
			RequiredFields: handler.GetRequiredFields(),
			DefaultConfig:  handler.GetDefaultConfig(),
			Validator: func(config map[string]interface{}) error {
				return handler.ValidateConfiguration(config)
			},
		}
	}

	// Keep Flannel for backward compatibility (no dedicated handler yet)
	h.supportedPlugins["flannel"] = NetworkPluginConfig{
		Name:           "flannel",
		RequiredFields: []string{},
		DefaultConfig: map[string]interface{}{
			"network": "10.244.0.0/16",
			"backend": map[string]interface{}{
				"type": "vxlan",
			},
		},
		Validator: h.validateFlannelConfig,
	}
}

// ValidateNetworkPlugin validates a network plugin configuration
func (h *DefaultNetworkPluginHandler) ValidateNetworkPlugin(pluginType string, config map[string]interface{}) error {
	pluginConfig, exists := h.supportedPlugins[pluginType]
	if !exists {
		return fmt.Errorf("unsupported network plugin: %s, supported plugins: %s",
			pluginType, strings.Join(h.GetSupportedPlugins(), ", "))
	}

	// Validate required fields
	for _, field := range pluginConfig.RequiredFields {
		if _, exists := config[field]; !exists {
			return fmt.Errorf("missing required field '%s' for network plugin '%s'", field, pluginType)
		}
	}

	// Run plugin-specific validation
	if pluginConfig.Validator != nil {
		if err := pluginConfig.Validator(config); err != nil {
			return fmt.Errorf("validation failed for network plugin '%s': %w", pluginType, err)
		}
	}

	return nil
}

// ValidateNetworkPluginMutualExclusivity validates that only one network plugin is enabled
// This method takes a map of plugin configurations and ensures mutual exclusivity
func (h *DefaultNetworkPluginHandler) ValidateNetworkPluginMutualExclusivity(pluginConfigs map[string]map[string]interface{}) error {
	var enabledPlugins []string

	// Check each plugin configuration for enabled status
	for pluginName, config := range pluginConfigs {
		if config == nil {
			continue
		}

		// Check if plugin is enabled
		if enabled, exists := config["enabled"]; exists {
			if enabledBool, ok := enabled.(bool); ok && enabledBool {
				enabledPlugins = append(enabledPlugins, pluginName)
			}
		}
	}

	// Validate mutual exclusivity
	if len(enabledPlugins) == 0 {
		return fmt.Errorf("at least one network plugin must be enabled")
	}

	if len(enabledPlugins) > 1 {
		return fmt.Errorf("only one network plugin can be enabled at a time, but found enabled: %s",
			strings.Join(enabledPlugins, ", "))
	}

	return nil
}

// ValidateNetworkPluginCompatibility validates compatibility between network plugins and other configurations
func (h *DefaultNetworkPluginHandler) ValidateNetworkPluginCompatibility(pluginType string, config map[string]interface{}, globalConfig map[string]interface{}) error {
	// Validate basic plugin configuration first
	if err := h.ValidateNetworkPlugin(pluginType, config); err != nil {
		return err
	}

	// Plugin-specific compatibility checks
	switch pluginType {
	case "kube-ovn":
		// Check Kube-OVN and Cilium integration compatibility
		if ciliumIntegration, exists := config["cilium_integration"]; exists {
			if ciliumBool, ok := ciliumIntegration.(bool); ok && ciliumBool {
				// When Kube-OVN has Cilium integration enabled, ensure Cilium is not also enabled as primary
				if globalConfig != nil {
					if ciliumConfig, exists := globalConfig["cilium"]; exists {
						if ciliumConfigMap, ok := ciliumConfig.(map[string]interface{}); ok {
							if ciliumEnabled, exists := ciliumConfigMap["enabled"]; exists {
								if ciliumEnabledBool, ok := ciliumEnabled.(bool); ok && ciliumEnabledBool {
									return fmt.Errorf("kube-ovn with cilium_integration cannot be used when cilium is also enabled as primary network plugin")
								}
							}
						}
					}
				}
			}
		}
	case "cilium":
		// Check if Cilium is being used with Kube-OVN integration
		if globalConfig != nil {
			if kubeOVNConfig, exists := globalConfig["kube-ovn"]; exists {
				if kubeOVNConfigMap, ok := kubeOVNConfig.(map[string]interface{}); ok {
					if kubeOVNEnabled, exists := kubeOVNConfigMap["enabled"]; exists {
						if kubeOVNEnabledBool, ok := kubeOVNEnabled.(bool); ok && kubeOVNEnabledBool {
							if ciliumIntegration, exists := kubeOVNConfigMap["cilium_integration"]; exists {
								if ciliumIntegrationBool, ok := ciliumIntegration.(bool); ok && ciliumIntegrationBool {
									return fmt.Errorf("cilium cannot be enabled as primary when kube-ovn with cilium_integration is enabled")
								}
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// RenderNetworkPluginConfig renders network plugin configuration
func (h *DefaultNetworkPluginHandler) RenderNetworkPluginConfig(pluginType string, config map[string]interface{}) (string, error) {
	pluginConfig, exists := h.supportedPlugins[pluginType]
	if !exists {
		return "", fmt.Errorf("unsupported network plugin: %s", pluginType)
	}

	// Merge with default configuration
	mergedConfig := make(map[string]interface{})
	for k, v := range pluginConfig.DefaultConfig {
		mergedConfig[k] = v
	}
	for k, v := range config {
		mergedConfig[k] = v
	}

	// Validate the merged configuration
	if err := h.ValidateNetworkPlugin(pluginType, mergedConfig); err != nil {
		return "", fmt.Errorf("configuration validation failed: %w", err)
	}

	// Use dedicated handler if available
	if handler, exists := h.pluginHandlers[pluginType]; exists {
		return handler.RenderConfiguration(mergedConfig)
	}

	// Fallback to legacy rendering for plugins without dedicated handlers
	switch pluginType {
	case "flannel":
		return h.renderFlannelConfig(mergedConfig), nil
	default:
		return "", fmt.Errorf("rendering not implemented for plugin: %s", pluginType)
	}
}

// GetSupportedPlugins returns a list of supported network plugins
func (h *DefaultNetworkPluginHandler) GetSupportedPlugins() []string {
	plugins := make([]string, 0, len(h.supportedPlugins))
	for plugin := range h.supportedPlugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// GetRequiredFields returns the required fields for a network plugin
func (h *DefaultNetworkPluginHandler) GetRequiredFields(pluginType string) []string {
	if pluginConfig, exists := h.supportedPlugins[pluginType]; exists {
		return pluginConfig.RequiredFields
	}
	return []string{}
}

// GetPluginHandler returns the dedicated handler for a specific plugin type
func (h *DefaultNetworkPluginHandler) GetPluginHandler(pluginType string) (SpecificNetworkPluginHandler, error) {
	if handler, exists := h.pluginHandlers[pluginType]; exists {
		return handler, nil
	}
	return nil, fmt.Errorf("no dedicated handler found for plugin: %s", pluginType)
}

// GetAllPluginHandlers returns all available plugin handlers
func (h *DefaultNetworkPluginHandler) GetAllPluginHandlers() map[string]SpecificNetworkPluginHandler {
	handlers := make(map[string]SpecificNetworkPluginHandler)
	for name, handler := range h.pluginHandlers {
		handlers[name] = handler
	}
	return handlers
}

// Plugin-specific validation functions

func (h *DefaultNetworkPluginHandler) validateCalicoConfig(config map[string]interface{}) error {
	// Validate IPv4 pool
	if ipv4Pool, exists := config["ipv4_pool"]; exists {
		if ipv4PoolStr, ok := ipv4Pool.(string); ok && ipv4PoolStr != "" {
			// Basic CIDR validation could be added here
			if !strings.Contains(ipv4PoolStr, "/") {
				return fmt.Errorf("invalid IPv4 pool format: %s", ipv4PoolStr)
			}
		}
	}

	// Validate MTU
	if mtu, exists := config["mtu"]; exists {
		if mtuInt, ok := mtu.(int); ok {
			if mtuInt < 68 || mtuInt > 9000 {
				return fmt.Errorf("invalid MTU value: %d, must be between 68 and 9000", mtuInt)
			}
		}
	}

	return nil
}

func (h *DefaultNetworkPluginHandler) validateCiliumConfig(config map[string]interface{}) error {
	// Validate cluster pool IPv4 CIDR
	if cidr, exists := config["cluster_pool_ipv4_cidr"]; exists {
		if cidrStr, ok := cidr.(string); ok && cidrStr != "" {
			if !strings.Contains(cidrStr, "/") {
				return fmt.Errorf("invalid cluster pool IPv4 CIDR format: %s", cidrStr)
			}
		}
	}

	// Validate mask size
	if maskSize, exists := config["cluster_pool_ipv4_mask_size"]; exists {
		if maskSizeInt, ok := maskSize.(int); ok {
			if maskSizeInt < 8 || maskSizeInt > 30 {
				return fmt.Errorf("invalid cluster pool IPv4 mask size: %d, must be between 8 and 30", maskSizeInt)
			}
		}
	}

	return nil
}

func (h *DefaultNetworkPluginHandler) validateKubeOVNConfig(config map[string]interface{}) error {
	// Validate default subnet
	if subnet, exists := config["default_subnet"]; exists {
		if subnetStr, ok := subnet.(string); ok && subnetStr != "" {
			if !strings.Contains(subnetStr, "/") {
				return fmt.Errorf("invalid default subnet format: %s", subnetStr)
			}
		}
	}

	return nil
}

func (h *DefaultNetworkPluginHandler) validateFlannelConfig(config map[string]interface{}) error {
	// Validate network
	if network, exists := config["network"]; exists {
		if networkStr, ok := network.(string); ok && networkStr != "" {
			if !strings.Contains(networkStr, "/") {
				return fmt.Errorf("invalid network format: %s", networkStr)
			}
		}
	}

	// Validate backend type
	if backend, exists := config["backend"]; exists {
		if backendMap, ok := backend.(map[string]interface{}); ok {
			if backendType, exists := backendMap["type"]; exists {
				validTypes := []string{"vxlan", "host-gw", "udp"}
				isValid := false
				for _, validType := range validTypes {
					if backendType == validType {
						isValid = true
						break
					}
				}
				if !isValid {
					return fmt.Errorf("invalid backend type: %v, valid types: %s",
						backendType, strings.Join(validTypes, ", "))
				}
			}
		}
	}

	return nil
}

// Plugin-specific rendering functions

func (h *DefaultNetworkPluginHandler) renderCalicoConfig(config map[string]interface{}) string {
	var parts []string
	parts = append(parts, "network_plugin = \"calico\"")

	if ipv4Pool, exists := config["ipv4_pool"]; exists {
		parts = append(parts, fmt.Sprintf("calico_ipv4_pool = \"%v\"", ipv4Pool))
	}

	if mtu, exists := config["mtu"]; exists {
		parts = append(parts, fmt.Sprintf("calico_mtu = %v", mtu))
	}

	return strings.Join(parts, "\n")
}

func (h *DefaultNetworkPluginHandler) renderCiliumConfig(config map[string]interface{}) string {
	var parts []string
	parts = append(parts, "network_plugin = \"cilium\"")

	if cidr, exists := config["cluster_pool_ipv4_cidr"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_cluster_pool_ipv4_cidr = \"%v\"", cidr))
	}

	if maskSize, exists := config["cluster_pool_ipv4_mask_size"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_cluster_pool_ipv4_mask_size = %v", maskSize))
	}

	return strings.Join(parts, "\n")
}

func (h *DefaultNetworkPluginHandler) renderKubeOVNConfig(config map[string]interface{}) string {
	var parts []string
	parts = append(parts, "network_plugin = \"kube-ovn\"")

	if subnet, exists := config["default_subnet"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_default_subnet = \"%v\"", subnet))
	}

	return strings.Join(parts, "\n")
}

func (h *DefaultNetworkPluginHandler) renderFlannelConfig(config map[string]interface{}) string {
	var parts []string
	parts = append(parts, "network_plugin = \"flannel\"")

	if network, exists := config["network"]; exists {
		parts = append(parts, fmt.Sprintf("flannel_network = \"%v\"", network))
	}

	if backend, exists := config["backend"]; exists {
		if backendMap, ok := backend.(map[string]interface{}); ok {
			if backendType, exists := backendMap["type"]; exists {
				parts = append(parts, fmt.Sprintf("flannel_backend_type = \"%v\"", backendType))
			}
		}
	}

	return strings.Join(parts, "\n")
}
