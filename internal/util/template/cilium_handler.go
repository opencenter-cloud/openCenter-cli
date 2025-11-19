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

// CiliumHandler implements SpecificNetworkPluginHandler for Cilium
type CiliumHandler struct{}

// NewCiliumHandler creates a new Cilium plugin handler
func NewCiliumHandler() *CiliumHandler {
	return &CiliumHandler{}
}

// GetPluginName returns the plugin name
func (h *CiliumHandler) GetPluginName() string {
	return "cilium"
}

// GetRequiredFields returns the required fields for Cilium configuration
func (h *CiliumHandler) GetRequiredFields() []string {
	return []string{} // Cilium has no strictly required fields
}

// GetDefaultConfig returns the default configuration for Cilium
func (h *CiliumHandler) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":                         true,
		"operator_enabled":                true,
		"kubeProxyReplacement":            true,
		"cluster_pool_ipv4_cidr":          "10.0.0.0/8",
		"cluster_pool_ipv4_mask_size":     24,
		"cilium_version":                  "1.14.5",
		"cilium_operator_version":         "1.14.5",
		"cilium_tunnel_mode":              "vxlan",
		"cilium_enable_ipv4":              true,
		"cilium_enable_ipv6":              false,
		"cilium_enable_l7_proxy":          true,
		"cilium_enable_bandwidth_manager": false,
		"hubble": map[string]interface{}{
			"enabled": false,
			"metrics": map[string]interface{}{
				"enabled": false,
			},
			"relay": map[string]interface{}{
				"enabled": false,
			},
			"ui": map[string]interface{}{
				"enabled": false,
			},
		},
	}
}

// ValidateConfiguration validates Cilium-specific configuration
func (h *CiliumHandler) ValidateConfiguration(config map[string]interface{}) error {
	// Validate enabled field
	if enabled, exists := config["enabled"]; exists {
		if enabledBool, ok := enabled.(bool); !ok {
			return fmt.Errorf("cilium.enabled must be a boolean value")
		} else if !enabledBool {
			return fmt.Errorf("cilium plugin is disabled")
		}
	}

	// Validate cluster pool IPv4 CIDR
	if cidr, exists := config["cluster_pool_ipv4_cidr"]; exists {
		if cidrStr, ok := cidr.(string); ok && cidrStr != "" {
			if !strings.Contains(cidrStr, "/") {
				return fmt.Errorf("invalid cluster pool IPv4 CIDR format: %s, must be in CIDR notation", cidrStr)
			}
		}
	}

	// Validate mask size
	if maskSize, exists := config["cluster_pool_ipv4_mask_size"]; exists {
		var maskSizeInt int
		if msi, ok := maskSize.(int); ok {
			maskSizeInt = msi
		} else if msf, ok := maskSize.(float64); ok {
			maskSizeInt = int(msf)
		} else {
			return fmt.Errorf("cluster_pool_ipv4_mask_size must be an integer")
		}

		if maskSizeInt < 8 || maskSizeInt > 30 {
			return fmt.Errorf("invalid cluster pool IPv4 mask size: %d, must be between 8 and 30", maskSizeInt)
		}
	}

	// Validate tunnel mode
	if tunnelMode, exists := config["cilium_tunnel_mode"]; exists {
		if tunnelModeStr, ok := tunnelMode.(string); ok && tunnelModeStr != "" {
			validModes := []string{"vxlan", "geneve", "disabled"}
			isValid := false
			for _, mode := range validModes {
				if tunnelModeStr == mode {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid cilium tunnel mode: %s, valid modes: %s",
					tunnelModeStr, strings.Join(validModes, ", "))
			}
		}
	}

	// Validate version format (basic check)
	if version, exists := config["cilium_version"]; exists {
		if versionStr, ok := version.(string); ok && versionStr != "" {
			if !strings.Contains(versionStr, ".") {
				return fmt.Errorf("invalid cilium version format: %s, expected semantic version", versionStr)
			}
		}
	}

	if operatorVersion, exists := config["cilium_operator_version"]; exists {
		if versionStr, ok := operatorVersion.(string); ok && versionStr != "" {
			if !strings.Contains(versionStr, ".") {
				return fmt.Errorf("invalid cilium operator version format: %s, expected semantic version", versionStr)
			}
		}
	}

	// Validate Hubble configuration if present
	if hubble, exists := config["hubble"]; exists {
		if hubbleMap, ok := hubble.(map[string]interface{}); ok {
			if err := h.validateHubbleConfig(hubbleMap); err != nil {
				return fmt.Errorf("hubble configuration validation failed: %w", err)
			}
		}
	}

	return nil
}

// validateHubbleConfig validates Hubble-specific configuration
func (h *CiliumHandler) validateHubbleConfig(hubbleConfig map[string]interface{}) error {
	// Validate hubble enabled
	if enabled, exists := hubbleConfig["enabled"]; exists {
		if _, ok := enabled.(bool); !ok {
			return fmt.Errorf("hubble.enabled must be a boolean value")
		}
	}

	// Validate metrics configuration
	if metrics, exists := hubbleConfig["metrics"]; exists {
		if metricsMap, ok := metrics.(map[string]interface{}); ok {
			if enabled, exists := metricsMap["enabled"]; exists {
				if _, ok := enabled.(bool); !ok {
					return fmt.Errorf("hubble.metrics.enabled must be a boolean value")
				}
			}
		}
	}

	// Validate relay configuration
	if relay, exists := hubbleConfig["relay"]; exists {
		if relayMap, ok := relay.(map[string]interface{}); ok {
			if enabled, exists := relayMap["enabled"]; exists {
				if _, ok := enabled.(bool); !ok {
					return fmt.Errorf("hubble.relay.enabled must be a boolean value")
				}
			}
		}
	}

	// Validate UI configuration
	if ui, exists := hubbleConfig["ui"]; exists {
		if uiMap, ok := ui.(map[string]interface{}); ok {
			if enabled, exists := uiMap["enabled"]; exists {
				if _, ok := enabled.(bool); !ok {
					return fmt.Errorf("hubble.ui.enabled must be a boolean value")
				}
			}
		}
	}

	return nil
}

// RenderConfiguration renders Cilium configuration for Terraform templates
func (h *CiliumHandler) RenderConfiguration(config map[string]interface{}) (string, error) {
	// Validate configuration first
	if err := h.ValidateConfiguration(config); err != nil {
		return "", fmt.Errorf("cilium configuration validation failed: %w", err)
	}

	var parts []string
	parts = append(parts, "network_plugin = \"cilium\"")

	// Render operator enabled
	if operatorEnabled, exists := config["operator_enabled"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_operator_enabled = %v", operatorEnabled))
	}

	// Render kube-proxy replacement
	if kubeProxyReplacement, exists := config["kubeProxyReplacement"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_kube_proxy_replacement = %v", kubeProxyReplacement))
	}

	// Render cluster pool IPv4 CIDR
	if cidr, exists := config["cluster_pool_ipv4_cidr"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_cluster_pool_ipv4_cidr = \"%v\"", cidr))
	}

	// Render cluster pool IPv4 mask size
	if maskSize, exists := config["cluster_pool_ipv4_mask_size"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_cluster_pool_ipv4_mask_size = %v", maskSize))
	}

	// Render versions
	if version, exists := config["cilium_version"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_version = \"%v\"", version))
	}

	if operatorVersion, exists := config["cilium_operator_version"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_operator_version = \"%v\"", operatorVersion))
	}

	// Render tunnel mode
	if tunnelMode, exists := config["cilium_tunnel_mode"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_tunnel_mode = \"%v\"", tunnelMode))
	}

	// Render IPv4/IPv6 settings
	if enableIPv4, exists := config["cilium_enable_ipv4"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_enable_ipv4 = %v", enableIPv4))
	}

	if enableIPv6, exists := config["cilium_enable_ipv6"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_enable_ipv6 = %v", enableIPv6))
	}

	// Render L7 proxy
	if enableL7Proxy, exists := config["cilium_enable_l7_proxy"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_enable_l7_proxy = %v", enableL7Proxy))
	}

	// Render bandwidth manager
	if enableBandwidthManager, exists := config["cilium_enable_bandwidth_manager"]; exists {
		parts = append(parts, fmt.Sprintf("cilium_enable_bandwidth_manager = %v", enableBandwidthManager))
	}

	// Render Hubble configuration
	if hubble, exists := config["hubble"]; exists {
		if hubbleMap, ok := hubble.(map[string]interface{}); ok {
			if hubbleEnabled, exists := hubbleMap["enabled"]; exists {
				parts = append(parts, fmt.Sprintf("cilium_hubble_enabled = %v", hubbleEnabled))
			}

			if metrics, exists := hubbleMap["metrics"]; exists {
				if metricsMap, ok := metrics.(map[string]interface{}); ok {
					if metricsEnabled, exists := metricsMap["enabled"]; exists {
						parts = append(parts, fmt.Sprintf("cilium_hubble_metrics_enabled = %v", metricsEnabled))
					}
				}
			}

			if relay, exists := hubbleMap["relay"]; exists {
				if relayMap, ok := relay.(map[string]interface{}); ok {
					if relayEnabled, exists := relayMap["enabled"]; exists {
						parts = append(parts, fmt.Sprintf("cilium_hubble_relay_enabled = %v", relayEnabled))
					}
				}
			}

			if ui, exists := hubbleMap["ui"]; exists {
				if uiMap, ok := ui.(map[string]interface{}); ok {
					if uiEnabled, exists := uiMap["enabled"]; exists {
						parts = append(parts, fmt.Sprintf("cilium_hubble_ui_enabled = %v", uiEnabled))
					}
				}
			}
		}
	}

	return strings.Join(parts, "\n"), nil
}
