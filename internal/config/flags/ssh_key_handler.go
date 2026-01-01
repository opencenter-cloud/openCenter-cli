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
	"strings"
)

// SSHKeyFlagHandler handles --ssh-key flags
type SSHKeyFlagHandler struct{}

// NewSSHKeyFlagHandler creates a new SSH key flag handler
func NewSSHKeyFlagHandler() *SSHKeyFlagHandler {
	return &SSHKeyFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *SSHKeyFlagHandler) CanHandle(flagName string) bool {
	return strings.Contains(flagName, "ssh-key")
}

// ParseFlag processes a single flag and returns the parsed result
func (h *SSHKeyFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return h.ParseArrayFlag(flagName, value)
}

// GetFlagType returns the type of flags this handler processes
func (h *SSHKeyFlagHandler) GetFlagType() FlagType {
	return FlagTypeArray
}

// ParseArrayFlag converts string to array configuration
func (h *SSHKeyFlagHandler) ParseArrayFlag(flagName, value string) (*ArrayConfig, error) {
	if value == "" {
		return nil, fmt.Errorf("ssh-key flag value cannot be empty")
	}
	
	// SSH key can be either a file path or the key content itself
	fields := map[string]interface{}{
		"key": strings.TrimSpace(value),
	}
	
	// Determine if it's a file path or key content
	if strings.HasPrefix(value, "ssh-") || strings.Contains(value, " ") {
		fields["type"] = "content"
	} else {
		fields["type"] = "file"
	}
	
	config := &ArrayConfig{
		Path:   "opencenter.infrastructure.ssh_keys",
		Index:  -1, // Will be determined during merging
		Fields: fields,
		Type:   "ssh-key",
	}
	
	return config, nil
}

// SupportedTypes returns array types this handler supports
func (h *SSHKeyFlagHandler) SupportedTypes() []string {
	return []string{"ssh-key"}
}

// ValidateArrayConfig ensures array configuration is valid
func (h *SSHKeyFlagHandler) ValidateArrayConfig(config *ArrayConfig) error {
	if config == nil {
		return fmt.Errorf("array config cannot be nil")
	}
	
	if config.Type != "ssh-key" {
		return fmt.Errorf("invalid array config type: expected 'ssh-key', got '%s'", config.Type)
	}
	
	if _, exists := config.Fields["key"]; !exists {
		return fmt.Errorf("missing required field 'key' in ssh-key configuration")
	}
	
	return nil
}