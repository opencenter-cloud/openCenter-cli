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

// DNSServerFlagHandler handles --dns-server flags
type DNSServerFlagHandler struct{}

// NewDNSServerFlagHandler creates a new DNS server flag handler
func NewDNSServerFlagHandler() *DNSServerFlagHandler {
	return &DNSServerFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *DNSServerFlagHandler) CanHandle(flagName string) bool {
	return strings.Contains(flagName, "dns-server")
}

// ParseFlag processes a single flag and returns the parsed result
func (h *DNSServerFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return h.ParseArrayFlag(flagName, value)
}

// GetFlagType returns the type of flags this handler processes
func (h *DNSServerFlagHandler) GetFlagType() FlagType {
	return FlagTypeArray
}

// ParseArrayFlag converts string to array configuration
func (h *DNSServerFlagHandler) ParseArrayFlag(flagName, value string) (*ArrayConfig, error) {
	if value == "" {
		return nil, fmt.Errorf("dns-server flag value cannot be empty")
	}
	
	// Validate IP address format
	ip := strings.TrimSpace(value)
	if net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("invalid IP address format for dns-server: '%s'", ip)
	}
	
	fields := map[string]interface{}{
		"ip": ip,
	}
	
	config := &ArrayConfig{
		Path:   "opencenter.networking.dns_servers",
		Index:  -1, // Will be determined during merging
		Fields: fields,
		Type:   "dns-server",
	}
	
	return config, nil
}

// SupportedTypes returns array types this handler supports
func (h *DNSServerFlagHandler) SupportedTypes() []string {
	return []string{"dns-server"}
}

// ValidateArrayConfig ensures array configuration is valid
func (h *DNSServerFlagHandler) ValidateArrayConfig(config *ArrayConfig) error {
	if config == nil {
		return fmt.Errorf("array config cannot be nil")
	}
	
	if config.Type != "dns-server" {
		return fmt.Errorf("invalid array config type: expected 'dns-server', got '%s'", config.Type)
	}
	
	ip, exists := config.Fields["ip"]
	if !exists {
		return fmt.Errorf("missing required field 'ip' in dns-server configuration")
	}
	
	if ipStr, ok := ip.(string); ok {
		if net.ParseIP(ipStr) == nil {
			return fmt.Errorf("invalid IP address in dns-server configuration: '%s'", ipStr)
		}
	} else {
		return fmt.Errorf("ip field must be a string in dns-server configuration")
	}
	
	return nil
}