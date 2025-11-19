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

// KubeOVNHandler implements SpecificNetworkPluginHandler for Kube-OVN
type KubeOVNHandler struct{}

// NewKubeOVNHandler creates a new Kube-OVN plugin handler
func NewKubeOVNHandler() *KubeOVNHandler {
	return &KubeOVNHandler{}
}

// GetPluginName returns the plugin name
func (h *KubeOVNHandler) GetPluginName() string {
	return "kube-ovn"
}

// GetRequiredFields returns the required fields for Kube-OVN configuration
func (h *KubeOVNHandler) GetRequiredFields() []string {
	return []string{} // Kube-OVN has no strictly required fields
}

// GetDefaultConfig returns the default configuration for Kube-OVN
func (h *KubeOVNHandler) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":                       true,
		"cilium_integration":            true,
		"default_subnet":                "10.16.0.0/16",
		"node_subnet":                   "10.17.0.0/16",
		"service_subnet":                "10.96.0.0/12",
		"kube_ovn_version":              "1.12.0",
		"ovn_nb_port":                   6641,
		"ovn_sb_port":                   6642,
		"ovn_northd_probe_interval":     5000,
		"ovn_controller_probe_interval": 5000,
		"enable_lb":                     true,
		"enable_np":                     true,
		"enable_eip_snat":               true,
		"enable_external_vpc":           false,
		"nic_bridge_mappings":           "",
		"network_type":                  "geneve",
		"default_interface_name":        "",
		"exclude_ips":                   "",
		"enable_ssl":                    false,
		"enable_bind_local_ip":          true,
	}
}

// ValidateConfiguration validates Kube-OVN-specific configuration
func (h *KubeOVNHandler) ValidateConfiguration(config map[string]interface{}) error {
	// Validate enabled field
	if enabled, exists := config["enabled"]; exists {
		if enabledBool, ok := enabled.(bool); !ok {
			return fmt.Errorf("kube-ovn.enabled must be a boolean value")
		} else if !enabledBool {
			return fmt.Errorf("kube-ovn plugin is disabled")
		}
	}

	// Validate default subnet
	if subnet, exists := config["default_subnet"]; exists {
		if subnetStr, ok := subnet.(string); ok && subnetStr != "" {
			if !strings.Contains(subnetStr, "/") {
				return fmt.Errorf("invalid default subnet format: %s, must be in CIDR notation", subnetStr)
			}
		}
	}

	// Validate node subnet
	if nodeSubnet, exists := config["node_subnet"]; exists {
		if nodeSubnetStr, ok := nodeSubnet.(string); ok && nodeSubnetStr != "" {
			if !strings.Contains(nodeSubnetStr, "/") {
				return fmt.Errorf("invalid node subnet format: %s, must be in CIDR notation", nodeSubnetStr)
			}
		}
	}

	// Validate service subnet
	if serviceSubnet, exists := config["service_subnet"]; exists {
		if serviceSubnetStr, ok := serviceSubnet.(string); ok && serviceSubnetStr != "" {
			if !strings.Contains(serviceSubnetStr, "/") {
				return fmt.Errorf("invalid service subnet format: %s, must be in CIDR notation", serviceSubnetStr)
			}
		}
	}

	// Validate version format
	if version, exists := config["kube_ovn_version"]; exists {
		if versionStr, ok := version.(string); ok && versionStr != "" {
			if !strings.Contains(versionStr, ".") {
				return fmt.Errorf("invalid kube-ovn version format: %s, expected semantic version", versionStr)
			}
		}
	}

	// Validate port numbers
	if err := h.validatePort(config, "ovn_nb_port", 1024, 65535); err != nil {
		return err
	}
	if err := h.validatePort(config, "ovn_sb_port", 1024, 65535); err != nil {
		return err
	}

	// Validate probe intervals
	if err := h.validateProbeInterval(config, "ovn_northd_probe_interval"); err != nil {
		return err
	}
	if err := h.validateProbeInterval(config, "ovn_controller_probe_interval"); err != nil {
		return err
	}

	// Validate network type
	if networkType, exists := config["network_type"]; exists {
		if networkTypeStr, ok := networkType.(string); ok && networkTypeStr != "" {
			validTypes := []string{"geneve", "vlan", "stt"}
			isValid := false
			for _, validType := range validTypes {
				if networkTypeStr == validType {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid network type: %s, valid types: %s",
					networkTypeStr, strings.Join(validTypes, ", "))
			}
		}
	}

	// Validate Cilium integration compatibility
	if ciliumIntegration, exists := config["cilium_integration"]; exists {
		if ciliumBool, ok := ciliumIntegration.(bool); ok && ciliumBool {
			// When Cilium integration is enabled, certain features should be compatible
			if enableLB, exists := config["enable_lb"]; exists {
				if enableLBBool, ok := enableLB.(bool); ok && enableLBBool {
					// This is fine - Kube-OVN can work with Cilium for load balancing
				}
			}
		}
	}

	return nil
}

// validatePort validates port number configuration
func (h *KubeOVNHandler) validatePort(config map[string]interface{}, portKey string, minPort, maxPort int) error {
	if port, exists := config[portKey]; exists {
		var portInt int
		if pi, ok := port.(int); ok {
			portInt = pi
		} else if pf, ok := port.(float64); ok {
			portInt = int(pf)
		} else {
			return fmt.Errorf("%s must be an integer", portKey)
		}

		if portInt < minPort || portInt > maxPort {
			return fmt.Errorf("invalid %s: %d, must be between %d and %d", portKey, portInt, minPort, maxPort)
		}
	}
	return nil
}

// validateProbeInterval validates probe interval configuration
func (h *KubeOVNHandler) validateProbeInterval(config map[string]interface{}, intervalKey string) error {
	if interval, exists := config[intervalKey]; exists {
		var intervalInt int
		if ii, ok := interval.(int); ok {
			intervalInt = ii
		} else if if_, ok := interval.(float64); ok {
			intervalInt = int(if_)
		} else {
			return fmt.Errorf("%s must be an integer", intervalKey)
		}

		if intervalInt < 1000 || intervalInt > 60000 {
			return fmt.Errorf("invalid %s: %d, must be between 1000 and 60000 milliseconds", intervalKey, intervalInt)
		}
	}
	return nil
}

// RenderConfiguration renders Kube-OVN configuration for Terraform templates
func (h *KubeOVNHandler) RenderConfiguration(config map[string]interface{}) (string, error) {
	// Validate configuration first
	if err := h.ValidateConfiguration(config); err != nil {
		return "", fmt.Errorf("kube-ovn configuration validation failed: %w", err)
	}

	var parts []string
	parts = append(parts, "network_plugin = \"kube-ovn\"")

	// Render Cilium integration
	if ciliumIntegration, exists := config["cilium_integration"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_cilium_integration = %v", ciliumIntegration))
	}

	// Render subnet configurations
	if defaultSubnet, exists := config["default_subnet"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_default_subnet = \"%v\"", defaultSubnet))
	}

	if nodeSubnet, exists := config["node_subnet"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_node_subnet = \"%v\"", nodeSubnet))
	}

	if serviceSubnet, exists := config["service_subnet"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_service_subnet = \"%v\"", serviceSubnet))
	}

	// Render version
	if version, exists := config["kube_ovn_version"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_version = \"%v\"", version))
	}

	// Render port configurations
	if nbPort, exists := config["ovn_nb_port"]; exists {
		parts = append(parts, fmt.Sprintf("ovn_nb_port = %v", nbPort))
	}

	if sbPort, exists := config["ovn_sb_port"]; exists {
		parts = append(parts, fmt.Sprintf("ovn_sb_port = %v", sbPort))
	}

	// Render probe intervals
	if northdProbe, exists := config["ovn_northd_probe_interval"]; exists {
		parts = append(parts, fmt.Sprintf("ovn_northd_probe_interval = %v", northdProbe))
	}

	if controllerProbe, exists := config["ovn_controller_probe_interval"]; exists {
		parts = append(parts, fmt.Sprintf("ovn_controller_probe_interval = %v", controllerProbe))
	}

	// Render feature flags
	if enableLB, exists := config["enable_lb"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_enable_lb = %v", enableLB))
	}

	if enableNP, exists := config["enable_np"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_enable_np = %v", enableNP))
	}

	if enableEipSnat, exists := config["enable_eip_snat"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_enable_eip_snat = %v", enableEipSnat))
	}

	if enableExternalVPC, exists := config["enable_external_vpc"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_enable_external_vpc = %v", enableExternalVPC))
	}

	if enableSSL, exists := config["enable_ssl"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_enable_ssl = %v", enableSSL))
	}

	if enableBindLocalIP, exists := config["enable_bind_local_ip"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_enable_bind_local_ip = %v", enableBindLocalIP))
	}

	// Render network type
	if networkType, exists := config["network_type"]; exists {
		parts = append(parts, fmt.Sprintf("kube_ovn_network_type = \"%v\"", networkType))
	}

	// Render optional string configurations
	if nicBridgeMappings, exists := config["nic_bridge_mappings"]; exists && nicBridgeMappings != "" {
		parts = append(parts, fmt.Sprintf("kube_ovn_nic_bridge_mappings = \"%v\"", nicBridgeMappings))
	}

	if defaultInterfaceName, exists := config["default_interface_name"]; exists && defaultInterfaceName != "" {
		parts = append(parts, fmt.Sprintf("kube_ovn_default_interface_name = \"%v\"", defaultInterfaceName))
	}

	if excludeIPs, exists := config["exclude_ips"]; exists && excludeIPs != "" {
		parts = append(parts, fmt.Sprintf("kube_ovn_exclude_ips = \"%v\"", excludeIPs))
	}

	return strings.Join(parts, "\n"), nil
}
