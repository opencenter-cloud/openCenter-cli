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

// CalicoHandler implements SpecificNetworkPluginHandler for Calico
type CalicoHandler struct{}

// NewCalicoHandler creates a new Calico plugin handler
func NewCalicoHandler() *CalicoHandler {
	return &CalicoHandler{}
}

// GetPluginName returns the plugin name
func (h *CalicoHandler) GetPluginName() string {
	return "calico"
}

// GetRequiredFields returns the required fields for Calico configuration
func (h *CalicoHandler) GetRequiredFields() []string {
	return []string{} // Calico has no strictly required fields
}

// GetDefaultConfig returns the default configuration for Calico
func (h *CalicoHandler) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":                        true,
		"cni_iface":                     "enp3s0",
		"calico_interface_autodetect":   "interface",
		"ipv4_pool":                     "192.168.0.0/16",
		"ipv6_pool":                     "",
		"mtu":                           1440,
		"calico_network_backend":        "bird",
		"calico_rr_enabled":             false,
		"calico_node_mesh_enabled":      true,
		"calico_felix_log_level":        "info",
		"calico_cni_log_level":          "info",
	}
}

// ValidateConfiguration validates Calico-specific configuration
func (h *CalicoHandler) ValidateConfiguration(config map[string]interface{}) error {
	// Validate enabled field
	if enabled, exists := config["enabled"]; exists {
		if enabledBool, ok := enabled.(bool); !ok {
			return fmt.Errorf("calico.enabled must be a boolean value")
		} else if !enabledBool {
			return fmt.Errorf("calico plugin is disabled")
		}
	}

	// Validate IPv4 pool
	if ipv4Pool, exists := config["ipv4_pool"]; exists {
		if ipv4PoolStr, ok := ipv4Pool.(string); ok && ipv4PoolStr != "" {
			if !strings.Contains(ipv4PoolStr, "/") {
				return fmt.Errorf("invalid IPv4 pool format: %s, must be in CIDR notation", ipv4PoolStr)
			}
		}
	}

	// Validate MTU
	if mtu, exists := config["mtu"]; exists {
		if mtuInt, ok := mtu.(int); ok {
			if mtuInt < 68 || mtuInt > 9000 {
				return fmt.Errorf("invalid MTU value: %d, must be between 68 and 9000", mtuInt)
			}
		} else if mtuFloat, ok := mtu.(float64); ok {
			mtuInt := int(mtuFloat)
			if mtuInt < 68 || mtuInt > 9000 {
				return fmt.Errorf("invalid MTU value: %d, must be between 68 and 9000", mtuInt)
			}
		}
	}

	// Validate CNI interface
	if cniIface, exists := config["cni_iface"]; exists {
		if cniIfaceStr, ok := cniIface.(string); ok && cniIfaceStr != "" {
			// Basic validation - interface name should not contain invalid characters
			if strings.ContainsAny(cniIfaceStr, " \t\n\r/\\") {
				return fmt.Errorf("invalid CNI interface name: %s", cniIfaceStr)
			}
		}
	}

	// Validate interface autodetect method
	if autodetect, exists := config["calico_interface_autodetect"]; exists {
		if autodetectStr, ok := autodetect.(string); ok && autodetectStr != "" {
			validMethods := []string{"interface", "can-reach", "first-found", "skip-default-route"}
			isValid := false
			for _, method := range validMethods {
				if autodetectStr == method {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid calico interface autodetect method: %s, valid methods: %s", 
					autodetectStr, strings.Join(validMethods, ", "))
			}
		}
	}

	// Validate network backend
	if backend, exists := config["calico_network_backend"]; exists {
		if backendStr, ok := backend.(string); ok && backendStr != "" {
			validBackends := []string{"bird", "vxlan", "none"}
			isValid := false
			for _, validBackend := range validBackends {
				if backendStr == validBackend {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid calico network backend: %s, valid backends: %s", 
					backendStr, strings.Join(validBackends, ", "))
			}
		}
	}

	// Validate log levels
	validLogLevels := []string{"debug", "info", "warning", "error", "fatal"}
	
	if logLevel, exists := config["calico_felix_log_level"]; exists {
		if logLevelStr, ok := logLevel.(string); ok && logLevelStr != "" {
			isValid := false
			for _, level := range validLogLevels {
				if logLevelStr == level {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid calico felix log level: %s, valid levels: %s", 
					logLevelStr, strings.Join(validLogLevels, ", "))
			}
		}
	}

	if logLevel, exists := config["calico_cni_log_level"]; exists {
		if logLevelStr, ok := logLevel.(string); ok && logLevelStr != "" {
			isValid := false
			for _, level := range validLogLevels {
				if logLevelStr == level {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid calico CNI log level: %s, valid levels: %s", 
					logLevelStr, strings.Join(validLogLevels, ", "))
			}
		}
	}

	return nil
}

// RenderConfiguration renders Calico configuration for Terraform templates
func (h *CalicoHandler) RenderConfiguration(config map[string]interface{}) (string, error) {
	// Validate configuration first
	if err := h.ValidateConfiguration(config); err != nil {
		return "", fmt.Errorf("calico configuration validation failed: %w", err)
	}

	var parts []string
	parts = append(parts, "network_plugin = \"calico\"")
	
	// Render CNI interface
	if cniIface, exists := config["cni_iface"]; exists {
		parts = append(parts, fmt.Sprintf("cni_iface = \"%v\"", cniIface))
	}
	
	// Render interface autodetect method
	if autodetect, exists := config["calico_interface_autodetect"]; exists {
		parts = append(parts, fmt.Sprintf("calico_interface_autodetect = \"%v\"", autodetect))
	}
	
	// Render IPv4 pool
	if ipv4Pool, exists := config["ipv4_pool"]; exists {
		parts = append(parts, fmt.Sprintf("calico_ipv4_pool = \"%v\"", ipv4Pool))
	}
	
	// Render IPv6 pool if specified
	if ipv6Pool, exists := config["ipv6_pool"]; exists && ipv6Pool != "" {
		parts = append(parts, fmt.Sprintf("calico_ipv6_pool = \"%v\"", ipv6Pool))
	}
	
	// Render MTU
	if mtu, exists := config["mtu"]; exists {
		parts = append(parts, fmt.Sprintf("calico_mtu = %v", mtu))
	}
	
	// Render network backend
	if backend, exists := config["calico_network_backend"]; exists {
		parts = append(parts, fmt.Sprintf("calico_network_backend = \"%v\"", backend))
	}
	
	// Render route reflector settings
	if rrEnabled, exists := config["calico_rr_enabled"]; exists {
		parts = append(parts, fmt.Sprintf("calico_rr_enabled = %v", rrEnabled))
	}
	
	// Render node mesh settings
	if nodeMesh, exists := config["calico_node_mesh_enabled"]; exists {
		parts = append(parts, fmt.Sprintf("calico_node_mesh_enabled = %v", nodeMesh))
	}
	
	// Render log levels
	if felixLogLevel, exists := config["calico_felix_log_level"]; exists {
		parts = append(parts, fmt.Sprintf("calico_felix_log_level = \"%v\"", felixLogLevel))
	}
	
	if cniLogLevel, exists := config["calico_cni_log_level"]; exists {
		parts = append(parts, fmt.Sprintf("calico_cni_log_level = \"%v\"", cniLogLevel))
	}
	
	return strings.Join(parts, "\n"), nil
}