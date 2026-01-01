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
	"strconv"
	"strings"
)

// ServerPoolFlagHandler handles --server-pool flags with comma-separated key=value syntax
type ServerPoolFlagHandler struct{}

// NewServerPoolFlagHandler creates a new server pool flag handler
func NewServerPoolFlagHandler() *ServerPoolFlagHandler {
	return &ServerPoolFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *ServerPoolFlagHandler) CanHandle(flagName string) bool {
	return strings.Contains(flagName, "server-pool")
}

// ParseFlag processes a single flag and returns the parsed result
func (h *ServerPoolFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return h.ParseArrayFlag(flagName, value)
}

// GetFlagType returns the type of flags this handler processes
func (h *ServerPoolFlagHandler) GetFlagType() FlagType {
	return FlagTypeArray
}

// ParseArrayFlag converts string to array configuration
func (h *ServerPoolFlagHandler) ParseArrayFlag(flagName, value string) (*ArrayConfig, error) {
	// Parse comma-separated key=value pairs
	fields := make(map[string]interface{})
	
	if value == "" {
		return nil, fmt.Errorf("server-pool flag value cannot be empty")
	}
	
	// Split by comma and parse each key=value pair
	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key=value pair in server-pool flag: '%s'", pair)
		}
		
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		
		if key == "" {
			return nil, fmt.Errorf("empty key in server-pool flag pair: '%s'", pair)
		}
		
		// Convert numeric values
		if key == "worker_count" || key == "volume_size" {
			if intVal, err := strconv.Atoi(val); err == nil {
				fields[key] = intVal
			} else {
				return nil, fmt.Errorf("invalid integer value for %s: '%s'", key, val)
			}
		} else {
			fields[key] = val
		}
	}
	
	// Validate required fields
	if err := h.validateRequiredFields(fields); err != nil {
		return nil, err
	}
	
	config := &ArrayConfig{
		Path:   "opencenter.infrastructure.server_pools",
		Index:  -1, // Will be determined during merging
		Fields: fields,
		Type:   "server-pool",
	}
	
	return config, nil
}

// SupportedTypes returns array types this handler supports
func (h *ServerPoolFlagHandler) SupportedTypes() []string {
	return []string{"server-pool"}
}

// ValidateArrayConfig ensures array configuration is valid
func (h *ServerPoolFlagHandler) ValidateArrayConfig(config *ArrayConfig) error {
	if config == nil {
		return fmt.Errorf("array config cannot be nil")
	}
	
	if config.Type != "server-pool" {
		return fmt.Errorf("invalid array config type: expected 'server-pool', got '%s'", config.Type)
	}
	
	return h.validateRequiredFields(config.Fields)
}

// validateRequiredFields checks that all required fields are present
func (h *ServerPoolFlagHandler) validateRequiredFields(fields map[string]interface{}) error {
	requiredFields := []string{"name", "worker_count", "flavor_worker", "node_worker"}
	
	for _, field := range requiredFields {
		if _, exists := fields[field]; !exists {
			return fmt.Errorf("missing required field '%s' in server-pool configuration", field)
		}
	}
	
	// Validate worker_count range
	if workerCount, ok := fields["worker_count"].(int); ok {
		if workerCount < 0 || workerCount > 100 {
			return fmt.Errorf("worker_count must be between 0 and 100, got %d", workerCount)
		}
	}
	
	// Validate volume_size if present
	if volumeSize, ok := fields["volume_size"].(int); ok {
		if volumeSize < 1 {
			return fmt.Errorf("volume_size must be at least 1, got %d", volumeSize)
		}
	}
	
	return nil
}

// ServerPoolConfig represents the structure of a server pool configuration
// This matches the design document specification
type ServerPoolConfig struct {
	Name                string            `yaml:"name" validate:"required"`
	WorkerCount         int               `yaml:"worker_count" validate:"min=0,max=100"`
	FlavorWorker        string            `yaml:"flavor_worker" validate:"required"`
	NodeWorker          string            `yaml:"node_worker" validate:"required"`
	ServerGroupAffinity string            `yaml:"server_group_affinity,omitempty"`
	ImageID             string            `yaml:"image_id,omitempty"`
	ImageName           string            `yaml:"image_name,omitempty"`
	VolumeSize          int               `yaml:"volume_size,omitempty"`
	VolumeType          string            `yaml:"volume_type,omitempty"`
	CustomFields        map[string]string `yaml:"custom_fields,omitempty"`
}

// ToServerPoolConfig converts the parsed fields to a ServerPoolConfig struct
func (h *ServerPoolFlagHandler) ToServerPoolConfig(fields map[string]interface{}) (*ServerPoolConfig, error) {
	config := &ServerPoolConfig{
		CustomFields: make(map[string]string),
	}
	
	for key, value := range fields {
		switch key {
		case "name":
			if str, ok := value.(string); ok {
				config.Name = str
			} else {
				return nil, fmt.Errorf("name must be a string")
			}
		case "worker_count":
			if intVal, ok := value.(int); ok {
				config.WorkerCount = intVal
			} else {
				return nil, fmt.Errorf("worker_count must be an integer")
			}
		case "flavor_worker":
			if str, ok := value.(string); ok {
				config.FlavorWorker = str
			} else {
				return nil, fmt.Errorf("flavor_worker must be a string")
			}
		case "node_worker":
			if str, ok := value.(string); ok {
				config.NodeWorker = str
			} else {
				return nil, fmt.Errorf("node_worker must be a string")
			}
		case "server_group_affinity":
			if str, ok := value.(string); ok {
				config.ServerGroupAffinity = str
			}
		case "image_id":
			if str, ok := value.(string); ok {
				config.ImageID = str
			}
		case "image_name":
			if str, ok := value.(string); ok {
				config.ImageName = str
			}
		case "volume_size":
			if intVal, ok := value.(int); ok {
				config.VolumeSize = intVal
			}
		case "volume_type":
			if str, ok := value.(string); ok {
				config.VolumeType = str
			}
		default:
			// Store unknown fields in CustomFields
			if str, ok := value.(string); ok {
				config.CustomFields[key] = str
			}
		}
	}
	
	return config, nil
}