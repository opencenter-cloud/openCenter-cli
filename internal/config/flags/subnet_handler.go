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

package flags

import (
	"fmt"
	"net"
	"strings"
)

// SubnetFlagHandler handles --subnet flags
type SubnetFlagHandler struct{}

// NewSubnetFlagHandler creates a new subnet flag handler
func NewSubnetFlagHandler() *SubnetFlagHandler {
	return &SubnetFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *SubnetFlagHandler) CanHandle(flagName string) bool {
	return strings.Contains(flagName, "subnet")
}

// ParseFlag processes a single flag and returns the parsed result
func (h *SubnetFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return h.ParseArrayFlag(flagName, value)
}

// GetFlagType returns the type of flags this handler processes
func (h *SubnetFlagHandler) GetFlagType() FlagType {
	return FlagTypeArray
}

// ParseArrayFlag converts string to array configuration
func (h *SubnetFlagHandler) ParseArrayFlag(flagName, value string) (*ArrayConfig, error) {
	if value == "" {
		return nil, fmt.Errorf("subnet flag value cannot be empty")
	}
	
	// Parse comma-separated key=value pairs for subnet configuration
	fields := make(map[string]interface{})
	
	// Split by comma and parse each key=value pair
	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key=value pair in subnet flag: '%s'", pair)
		}
		
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		
		if key == "" {
			return nil, fmt.Errorf("empty key in subnet flag pair: '%s'", pair)
		}
		
		fields[key] = val
	}
	
	// Validate CIDR if provided
	if cidr, exists := fields["cidr"]; exists {
		if cidrStr, ok := cidr.(string); ok {
			if _, _, err := net.ParseCIDR(cidrStr); err != nil {
				return nil, fmt.Errorf("invalid CIDR format in subnet: '%s'", cidrStr)
			}
		}
	}
	
	// Validate required fields
	if err := h.validateRequiredFields(fields); err != nil {
		return nil, err
	}
	
	config := &ArrayConfig{
		Path:   "opencenter.networking.subnets",
		Index:  -1, // Will be determined during merging
		Fields: fields,
		Type:   "subnet",
	}
	
	return config, nil
}

// SupportedTypes returns array types this handler supports
func (h *SubnetFlagHandler) SupportedTypes() []string {
	return []string{"subnet"}
}

// ValidateArrayConfig ensures array configuration is valid
func (h *SubnetFlagHandler) ValidateArrayConfig(config *ArrayConfig) error {
	if config == nil {
		return fmt.Errorf("array config cannot be nil")
	}
	
	if config.Type != "subnet" {
		return fmt.Errorf("invalid array config type: expected 'subnet', got '%s'", config.Type)
	}
	
	return h.validateRequiredFields(config.Fields)
}

// validateRequiredFields checks that required fields are present and valid
func (h *SubnetFlagHandler) validateRequiredFields(fields map[string]interface{}) error {
	// At minimum, we need either a name or CIDR
	if _, hasName := fields["name"]; !hasName {
		if _, hasCIDR := fields["cidr"]; !hasCIDR {
			return fmt.Errorf("subnet must have either 'name' or 'cidr' field")
		}
	}
	
	// Validate CIDR format if present
	if cidr, exists := fields["cidr"]; exists {
		if cidrStr, ok := cidr.(string); ok {
			if _, _, err := net.ParseCIDR(cidrStr); err != nil {
				return fmt.Errorf("invalid CIDR format: '%s'", cidrStr)
			}
		} else {
			return fmt.Errorf("cidr field must be a string")
		}
	}
	
	return nil
}