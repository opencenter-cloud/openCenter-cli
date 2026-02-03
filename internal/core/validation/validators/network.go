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

package validators

import (
	"context"
	"fmt"
	"net"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// NetworkConfig represents network configuration for validation.
// This is a simplified version to avoid import cycles with internal/config.
//
// Note: This type mirrors the fields from internal/config.Networking that are
// relevant for network validation. When using this validator with actual config
// objects, you'll need to convert the config.Networking to NetworkConfig.
//
// Example conversion:
//
//	networkConfig := &validators.NetworkConfig{
//	    SubnetNodes:    cfg.Networking.SubnetNodes,
//	    SubnetPods:     cfg.Networking.SubnetPods,
//	    SubnetServices: cfg.Networking.SubnetServices,
//	    DNSNameservers: cfg.Networking.DNSNameservers,
//	    VRRPEnabled:    cfg.Networking.VRRPEnabled,
//	    VRRPIP:         cfg.Networking.VRRPIP,
//	}
type NetworkConfig struct {
	SubnetNodes    string
	SubnetPods     string
	SubnetServices string
	DNSNameservers []string
	VRRPEnabled    bool
	VRRPIP         string
}

// NetworkValidator validates network configuration including CIDR ranges and IP addresses.
//
// Requirements (from Phase 2 Validation Consolidation):
//   - Validate CIDR format for pod and service networks
//   - Check for CIDR overlap between pod and service networks
//   - Validate DNS server IP addresses
//   - Provide network configuration suggestions
//
// Validates: Requirements 2.5, 2.10
type NetworkValidator struct{}

// NewNetworkValidator creates a new network validator.
func NewNetworkValidator() *NetworkValidator {
	return &NetworkValidator{}
}

// Name returns the validator name.
func (v *NetworkValidator) Name() string {
	return "network"
}

// Priority returns the validator priority.
// Network validation involves CIDR parsing and overlap checks, which are
// moderately complex, so it has normal priority.
func (v *NetworkValidator) Priority() int {
	return validation.PriorityNormal
}

// Validate validates network configuration including CIDR ranges and IP addresses.
//
// The validator checks:
//   - Pod CIDR format (if specified)
//   - Service CIDR format (if specified)
//   - CIDR overlap between pod and service networks
//   - DNS server IP address validity
//
// Returns a ValidationResult with errors and actionable suggestions.
func (v *NetworkValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := validation.NewValidationResult()

	networkConfig, ok := value.(*NetworkConfig)
	if !ok {
		result.AddError("network", "value must be a NetworkConfig",
			"Provide a valid NetworkConfig object")
		return result, nil
	}

	// Validate pod CIDR
	var podNet *net.IPNet
	if networkConfig.SubnetPods != "" {
		if err := v.validateCIDR(networkConfig.SubnetPods, "subnet_pods", result); err == nil {
			_, podNet, _ = net.ParseCIDR(networkConfig.SubnetPods)
		}
	}

	// Validate service CIDR
	var serviceNet *net.IPNet
	if networkConfig.SubnetServices != "" {
		if err := v.validateCIDR(networkConfig.SubnetServices, "subnet_services", result); err == nil {
			_, serviceNet, _ = net.ParseCIDR(networkConfig.SubnetServices)
		}
	}

	// Check for CIDR overlap if both are specified and valid
	if podNet != nil && serviceNet != nil {
		if v.cidrsOverlap(podNet, serviceNet) {
			result.AddError("network",
				"pod CIDR and service CIDR overlap",
				"Use non-overlapping CIDR ranges",
				"Example: subnet_pods: 10.244.0.0/16, subnet_services: 10.96.0.0/12",
				"Ensure the IP ranges do not conflict")
		}
	}

	// Validate DNS nameservers
	for i, dnsServer := range networkConfig.DNSNameservers {
		if dnsServer == "" {
			continue // Skip empty entries
		}
		if net.ParseIP(dnsServer) == nil {
			result.AddError(fmt.Sprintf("dns_nameservers[%d]", i),
				fmt.Sprintf("invalid DNS server IP address: %s", dnsServer),
				"Provide a valid IPv4 or IPv6 address",
				"Example: 8.8.8.8 or 2001:4860:4860::8888")
		}
	}

	// Validate node subnet CIDR if specified
	if networkConfig.SubnetNodes != "" {
		v.validateCIDR(networkConfig.SubnetNodes, "subnet_nodes", result)
	}

	// Validate VRRP IP if VRRP is enabled
	if networkConfig.VRRPEnabled && networkConfig.VRRPIP != "" {
		if net.ParseIP(networkConfig.VRRPIP) == nil {
			result.AddError("vrrp_ip",
				fmt.Sprintf("invalid VRRP IP address: %s", networkConfig.VRRPIP),
				"Provide a valid IPv4 or IPv6 address",
				"Example: 192.168.1.100")
		}
	}

	// Add warnings for common misconfigurations
	if networkConfig.SubnetPods == "" {
		result.AddWarning("subnet_pods",
			"pod subnet not specified, cluster may use default values",
			"Consider explicitly setting subnet_pods for better control",
			"Example: subnet_pods: 10.244.0.0/16")
	}

	if networkConfig.SubnetServices == "" {
		result.AddWarning("subnet_services",
			"service subnet not specified, cluster may use default values",
			"Consider explicitly setting subnet_services for better control",
			"Example: subnet_services: 10.96.0.0/12")
	}

	if len(networkConfig.DNSNameservers) == 0 {
		result.AddWarning("dns_nameservers",
			"no DNS nameservers specified, cluster may use default values",
			"Consider explicitly setting DNS nameservers",
			"Example: dns_nameservers: [8.8.8.8, 8.8.4.4]")
	}

	return result, nil
}

// validateCIDR validates a CIDR notation string.
//
// Parameters:
//   - cidr: CIDR string to validate (e.g., "10.244.0.0/16")
//   - field: Field name for error reporting
//   - result: ValidationResult to add errors to
//
// Returns:
//   - error: Validation error if CIDR is invalid, nil otherwise
func (v *NetworkValidator) validateCIDR(cidr, field string, result *validation.ValidationResult) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		result.AddError(field,
			fmt.Sprintf("invalid CIDR format: %s", cidr),
			"Use CIDR notation: <ip>/<prefix>",
			"Example: 10.244.0.0/16 or 2001:db8::/32",
			"Ensure the IP address and prefix length are valid")
		return err
	}
	return nil
}

// cidrsOverlap checks if two CIDR ranges overlap.
//
// Two CIDRs overlap if either network contains the other's IP address.
//
// Parameters:
//   - net1: First network
//   - net2: Second network
//
// Returns:
//   - bool: True if the networks overlap
func (v *NetworkValidator) cidrsOverlap(net1, net2 *net.IPNet) bool {
	// Check if net1 contains net2's IP or net2 contains net1's IP
	return net1.Contains(net2.IP) || net2.Contains(net1.IP)
}
