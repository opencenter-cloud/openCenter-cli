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
	"net/url"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
)

// ProviderValidator validates cloud provider configurations.
//
// Requirements (from Phase 2 Validation Consolidation):
//   - Validates provider-specific configuration
//   - Checks required fields for each provider
//   - Validates credentials format
//   - Provides actionable suggestions for fixing issues
//
// Validates: Requirements 2.8, 2.10
type ProviderValidator struct {
	supportedProviders map[string]bool
}

// NewProviderValidator creates a new provider validator.
func NewProviderValidator() *ProviderValidator {
	return &ProviderValidator{
		supportedProviders: map[string]bool{
			"openstack": true,
			"aws":       true,
			"gcp":       true,
			"azure":     true,
			"baremetal": true,
			"vsphere":   true,
			"vmware":    true,
		},
	}
}

// Name returns the validator name.
func (v *ProviderValidator) Name() string {
	return "provider"
}

// Priority returns the validator priority.
// Provider validation involves business logic checks, so it has normal priority.
func (v *ProviderValidator) Priority() int {
	return validation.PriorityNormal
}

// Validate validates cloud provider configuration.
//
// The value should be a map with the following keys:
//   - "provider": Provider name (required)
//   - "config": Provider-specific configuration (required)
//
// Returns a ValidationResult with errors and actionable suggestions.
func (v *ProviderValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := validation.NewValidationResult()

	providerMap, ok := value.(map[string]interface{})
	if !ok {
		result.AddError("provider", "value must be a map with 'provider' and 'config' keys",
			"Provide a map with provider configuration")
		return result, nil
	}

	// Validate provider name
	providerVal, ok := providerMap["provider"]
	if !ok {
		result.AddError("provider", "provider name is required",
			"Specify the cloud provider",
			"Supported providers: openstack, aws, gcp, azure, baremetal, vsphere, vmware")
		return result, nil
	}

	provider, ok := providerVal.(string)
	if !ok {
		result.AddError("provider", "provider must be a string")
		return result, nil
	}

	provider = strings.ToLower(provider)

	// Check if provider is supported
	if !v.supportedProviders[provider] {
		var supportedList []string
		for p := range v.supportedProviders {
			supportedList = append(supportedList, p)
		}

		result.AddError("provider",
			fmt.Sprintf("unsupported provider: %s", provider),
			fmt.Sprintf("Supported providers: %s", strings.Join(supportedList, ", ")),
			"Check for typos in the provider name")
		return result, nil
	}

	// Validate provider-specific configuration
	configVal, ok := providerMap["config"]
	if !ok {
		result.AddError("provider.config", "provider configuration is required",
			fmt.Sprintf("Provide %s configuration", provider))
		return result, nil
	}

	config, ok := configVal.(map[string]interface{})
	if !ok {
		result.AddError("provider.config", "provider configuration must be a map")
		return result, nil
	}

	// Validate based on provider type
	switch provider {
	case "openstack":
		v.validateOpenStackConfig(result, config)
	case "aws":
		v.validateAWSConfig(result, config)
	case "gcp":
		v.validateGCPConfig(result, config)
	case "azure":
		v.validateAzureConfig(result, config)
	case "vsphere":
		v.validateVSphereConfig(result, config)
	case "baremetal":
		v.validateBaremetalConfig(result, config)
	case "vmware":
		v.validateVMwareConfig(result, config)
	}

	return result, nil
}

// validateOpenStackConfig validates OpenStack provider configuration.
func (v *ProviderValidator) validateOpenStackConfig(result *validation.ValidationResult, config map[string]interface{}) {
	// Required fields
	requiredFields := []string{"auth_url", "region"}

	for _, field := range requiredFields {
		if val, ok := config[field]; !ok || val == "" {
			result.AddError(fmt.Sprintf("provider.openstack.%s", field),
				fmt.Sprintf("%s is required for OpenStack", field),
				fmt.Sprintf("Provide the %s value", field),
				"Example: auth_url: https://openstack.example.com:5000/v3")
		}
	}

	// Validate auth_url format
	if authURLVal, ok := config["auth_url"]; ok {
		if authURL, ok := authURLVal.(string); ok && authURL != "" {
			if _, err := url.Parse(authURL); err != nil {
				result.AddError("provider.openstack.auth_url",
					fmt.Sprintf("invalid auth_url format: %s", authURL),
					"Provide a valid URL",
					"Example: https://openstack.example.com:5000/v3")
			} else {
				// Check for common mistakes
				if !strings.HasPrefix(authURL, "http://") && !strings.HasPrefix(authURL, "https://") {
					result.AddError("provider.openstack.auth_url",
						"auth_url must start with http:// or https://",
						"Add the protocol to the URL",
						"Example: https://openstack.example.com:5000/v3")
				}

				// Warn about insecure http
				if strings.HasPrefix(authURL, "http://") {
					result.AddWarning("provider.openstack.auth_url",
						"auth_url uses insecure http protocol",
						"Consider using https for secure communication")
				}

				// Check for version suffix
				if !strings.Contains(authURL, "/v3") && !strings.Contains(authURL, "/v2") {
					result.AddWarning("provider.openstack.auth_url",
						"auth_url should include API version (e.g., /v3)",
						"Add version suffix to the URL",
						"Example: https://openstack.example.com:5000/v3")
				}
			}
		}
	}

	// Check for authentication method
	hasAppCred := config["application_credential_id"] != nil && config["application_credential_id"] != ""
	hasUserPass := config["user_name"] != nil && config["user_name"] != "" &&
		config["user_password"] != nil && config["user_password"] != ""

	if !hasAppCred && !hasUserPass {
		result.AddError("provider.openstack.auth",
			"OpenStack authentication credentials are required",
			"Provide either application credentials (application_credential_id + application_credential_secret)",
			"Or user credentials (user_name + user_password + domain)")
	}

	// Validate region format
	if regionVal, ok := config["region"]; ok {
		if region, ok := regionVal.(string); ok && region != "" {
			// Check for common region format issues
			if strings.Contains(region, " ") {
				result.AddWarning("provider.openstack.region",
					"region name contains spaces, which may cause issues",
					"Remove spaces from the region name")
			}
		}
	}

	// Validate networking configuration if present
	if networkingVal, ok := config["networking"]; ok {
		if networking, ok := networkingVal.(map[string]interface{}); ok {
			v.validateOpenStackNetworking(result, networking)
		}
	}
}

// validateOpenStackNetworking validates OpenStack networking configuration.
func (v *ProviderValidator) validateOpenStackNetworking(result *validation.ValidationResult, networking map[string]interface{}) {
	// Validate floating_network_id if present
	if floatingNetVal, ok := networking["floating_network_id"]; ok {
		if floatingNet, ok := floatingNetVal.(string); ok && floatingNet != "" {
			// Check if it looks like a UUID
			if !isUUID(floatingNet) {
				result.AddWarning("provider.openstack.networking.floating_network_id",
					"floating_network_id should be a UUID",
					"Verify the network ID is correct",
					"Example: 12345678-1234-1234-1234-123456789abc")
			}
		}
	}

	// Validate k8s_api_port_acl if present
	if aclVal, ok := networking["k8s_api_port_acl"]; ok {
		if acl, ok := aclVal.([]interface{}); ok {
			for i, cidrVal := range acl {
				if cidr, ok := cidrVal.(string); ok {
					if _, _, err := net.ParseCIDR(cidr); err != nil {
						result.AddError(fmt.Sprintf("provider.openstack.networking.k8s_api_port_acl[%d]", i),
							fmt.Sprintf("invalid CIDR format: %s", cidr),
							"Use CIDR notation: <ip>/<prefix>",
							"Example: 10.0.0.0/8 or 192.168.1.0/24")
					}
				}
			}
		}
	}
}

// validateAWSConfig validates AWS provider configuration.
func (v *ProviderValidator) validateAWSConfig(result *validation.ValidationResult, config map[string]interface{}) {
	// Required fields
	requiredFields := []string{"region"}

	for _, field := range requiredFields {
		if val, ok := config[field]; !ok || val == "" {
			result.AddError(fmt.Sprintf("provider.aws.%s", field),
				fmt.Sprintf("%s is required for AWS", field),
				fmt.Sprintf("Provide the %s value", field),
				"Example: region: us-east-1")
		}
	}

	// Validate region format
	if regionVal, ok := config["region"]; ok {
		if region, ok := regionVal.(string); ok && region != "" {
			// Check for valid AWS region format
			if !isValidAWSRegion(region) {
				result.AddWarning("provider.aws.region",
					fmt.Sprintf("region '%s' does not match standard AWS region format", region),
					"Use standard AWS region format",
					"Examples: us-east-1, eu-west-2, ap-southeast-1")
			}
		}
	}

	// Validate VPC ID if present
	if vpcIDVal, ok := config["vpc_id"]; ok {
		if vpcID, ok := vpcIDVal.(string); ok && vpcID != "" {
			if !strings.HasPrefix(vpcID, "vpc-") {
				result.AddError("provider.aws.vpc_id",
					fmt.Sprintf("invalid VPC ID format: %s", vpcID),
					"VPC ID must start with 'vpc-'",
					"Example: vpc-1234567890abcdef0")
			}
		}
	}

	// Validate subnets if present
	if subnetsVal, ok := config["private_subnets"]; ok {
		if subnets, ok := subnetsVal.([]interface{}); ok {
			for i, subnetVal := range subnets {
				if subnet, ok := subnetVal.(string); ok {
					if !strings.HasPrefix(subnet, "subnet-") {
						result.AddError(fmt.Sprintf("provider.aws.private_subnets[%d]", i),
							fmt.Sprintf("invalid subnet ID format: %s", subnet),
							"Subnet ID must start with 'subnet-'",
							"Example: subnet-1234567890abcdef0")
					}
				}
			}
		}
	}
}

// validateGCPConfig validates GCP provider configuration.
func (v *ProviderValidator) validateGCPConfig(result *validation.ValidationResult, config map[string]interface{}) {
	// Required fields
	requiredFields := []string{"project", "region"}

	for _, field := range requiredFields {
		if val, ok := config[field]; !ok || val == "" {
			result.AddError(fmt.Sprintf("provider.gcp.%s", field),
				fmt.Sprintf("%s is required for GCP", field),
				fmt.Sprintf("Provide the %s value", field))
		}
	}

	// Validate project ID format
	if projectVal, ok := config["project"]; ok {
		if project, ok := projectVal.(string); ok && project != "" {
			// GCP project IDs must be lowercase and can contain hyphens
			if strings.ToLower(project) != project {
				result.AddError("provider.gcp.project",
					"GCP project ID must be lowercase",
					"Convert project ID to lowercase")
			}
		}
	}
}

// validateAzureConfig validates Azure provider configuration.
func (v *ProviderValidator) validateAzureConfig(result *validation.ValidationResult, config map[string]interface{}) {
	// Required fields
	requiredFields := []string{"subscription_id", "resource_group", "location"}

	for _, field := range requiredFields {
		if val, ok := config[field]; !ok || val == "" {
			result.AddError(fmt.Sprintf("provider.azure.%s", field),
				fmt.Sprintf("%s is required for Azure", field),
				fmt.Sprintf("Provide the %s value", field))
		}
	}

	// Validate subscription_id format (should be a UUID)
	if subIDVal, ok := config["subscription_id"]; ok {
		if subID, ok := subIDVal.(string); ok && subID != "" {
			if !isUUID(subID) {
				result.AddError("provider.azure.subscription_id",
					"subscription_id should be a UUID",
					"Verify the subscription ID is correct",
					"Example: 12345678-1234-1234-1234-123456789abc")
			}
		}
	}
}

// validateVSphereConfig validates VSphere provider configuration.
func (v *ProviderValidator) validateVSphereConfig(result *validation.ValidationResult, config map[string]interface{}) {
	// Required fields
	requiredFields := []string{"server", "datacenter"}

	for _, field := range requiredFields {
		if val, ok := config[field]; !ok || val == "" {
			result.AddError(fmt.Sprintf("provider.vsphere.%s", field),
				fmt.Sprintf("%s is required for VSphere", field),
				fmt.Sprintf("Provide the %s value", field))
		}
	}

	// Validate server format
	if serverVal, ok := config["server"]; ok {
		if server, ok := serverVal.(string); ok && server != "" {
			// Check if it's a valid hostname or IP
			if net.ParseIP(server) == nil {
				// Not an IP, check if it's a valid hostname
				if _, err := url.Parse("https://" + server); err != nil {
					result.AddError("provider.vsphere.server",
						fmt.Sprintf("invalid server format: %s", server),
						"Provide a valid hostname or IP address",
						"Example: vcenter.example.com or 192.168.1.100")
				}
			}
		}
	}
}

// validateBaremetalConfig validates baremetal provider configuration.
func (v *ProviderValidator) validateBaremetalConfig(result *validation.ValidationResult, config map[string]interface{}) {
	// Baremetal requires node definitions
	if nodesVal, ok := config["nodes"]; !ok || nodesVal == nil {
		result.AddError("provider.baremetal.nodes",
			"nodes configuration is required for baremetal provider",
			"Provide a list of nodes with their configuration",
			"Example: nodes: [{name: node1, ip: 192.168.1.10}]")
		return
	}

	// Validate nodes if present
	if nodes, ok := config["nodes"].([]interface{}); ok {
		if len(nodes) == 0 {
			result.AddError("provider.baremetal.nodes",
				"at least one node is required for baremetal provider",
				"Add node definitions to the configuration")
			return
		}

		for i, nodeVal := range nodes {
			if node, ok := nodeVal.(map[string]interface{}); ok {
				v.validateBaremetalNode(result, node, i)
			}
		}
	}
}

// validateBaremetalNode validates a single baremetal node configuration.
func (v *ProviderValidator) validateBaremetalNode(result *validation.ValidationResult, node map[string]interface{}, index int) {
	// Required fields for each node
	requiredFields := []string{"name", "ip"}

	for _, field := range requiredFields {
		if val, ok := node[field]; !ok || val == "" {
			result.AddError(fmt.Sprintf("provider.baremetal.nodes[%d].%s", index, field),
				fmt.Sprintf("%s is required for baremetal node", field),
				fmt.Sprintf("Provide the %s value for node %d", field, index))
		}
	}

	// Validate IP address
	if ipVal, ok := node["ip"]; ok {
		if ip, ok := ipVal.(string); ok && ip != "" {
			if net.ParseIP(ip) == nil {
				result.AddError(fmt.Sprintf("provider.baremetal.nodes[%d].ip", index),
					fmt.Sprintf("invalid IP address: %s", ip),
					"Provide a valid IPv4 or IPv6 address",
					"Example: 192.168.1.10 or 2001:db8::1")
			}
		}
	}
}

// isUUID checks if a string is a valid UUID.
func isUUID(s string) bool {
	// Simple UUID validation (8-4-4-4-12 format)
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 ||
		len(parts[3]) != 4 || len(parts[4]) != 12 {
		return false
	}
	return true
}

// isValidAWSRegion checks if a string matches AWS region format.
func isValidAWSRegion(region string) bool {
	// AWS regions follow pattern: <geo>-<direction>-<number>
	// Examples: us-east-1, eu-west-2, ap-southeast-1
	parts := strings.Split(region, "-")
	if len(parts) < 3 {
		return false
	}

	// Check if last part is a number
	lastPart := parts[len(parts)-1]
	if len(lastPart) != 1 || lastPart[0] < '0' || lastPart[0] > '9' {
		return false
	}

	return true
}

// SetSupportedProviders sets the list of supported providers.
func (v *ProviderValidator) SetSupportedProviders(providers []string) {
	v.supportedProviders = make(map[string]bool)
	for _, provider := range providers {
		v.supportedProviders[strings.ToLower(provider)] = true
	}
}

// validateVMwareConfig validates VMware provider configuration.
// VMware is treated as baremetal - requires pre-provisioned nodes in compute section.
func (v *ProviderValidator) validateVMwareConfig(result *validation.ValidationResult, config map[string]interface{}) {
	// Validate vCenter configuration (optional but recommended)
	if vcenterVal, ok := config["vcenter_server"]; ok {
		if vcenter, ok := vcenterVal.(string); ok && vcenter != "" {
			// Check if it's a valid hostname or IP
			if net.ParseIP(vcenter) == nil {
				// Not an IP, check if it's a valid hostname
				if _, err := url.Parse("https://" + vcenter); err != nil {
					result.AddWarning("provider.vmware.vcenter_server",
						fmt.Sprintf("vcenter_server may not be a valid hostname: %s", vcenter),
						"Verify the vCenter server address is correct",
						"Example: vcenter.example.com or 192.168.1.100")
				}
			}
		}
	}

	// Validate datacenter if present
	if datacenterVal, ok := config["datacenter"]; ok {
		if datacenter, ok := datacenterVal.(string); ok && datacenter == "" {
			result.AddWarning("provider.vmware.datacenter",
				"datacenter is empty",
				"Provide the VMware datacenter name for CSI driver configuration")
		}
	}

	// Validate datastore if present
	if datastoreVal, ok := config["datastore"]; ok {
		if datastore, ok := datastoreVal.(string); ok && datastore == "" {
			result.AddWarning("provider.vmware.datastore",
				"datastore is empty",
				"Provide the default datastore name for persistent volumes")
		}
	}
}


// isValidMACAddress checks if a string is a valid MAC address.
func isValidMACAddress(mac string) bool {
	// Simple validation for common MAC address formats
	// Supports: XX:XX:XX:XX:XX:XX, XX-XX-XX-XX-XX-XX, XXXXXXXXXXXX
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	if len(mac) != 12 {
		return false
	}
	for _, c := range mac {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
