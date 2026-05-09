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

package v2

import (
	"fmt"
	"reflect"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

// UnmarshalYAML implements custom YAML unmarshaling for ServiceMap.
// It uses the service registry to determine the correct type for each service.
func (sm *ServiceMap) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node for services, got %v", node.Kind)
	}

	*sm = make(ServiceMap)

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		serviceName := keyNode.Value

		// Look up the service type in the registry
		serviceType := registry.GetServiceConfigType(serviceName)
		if serviceType == nil {
			// Unregistered service: decode into DefaultServiceConfig
			serviceType = reflect.TypeOf(services.DefaultServiceConfig{})
		}

		// Create a new instance of the registered type
		serviceConfig := reflect.New(serviceType).Interface()

		// Unmarshal into the typed struct
		if err := valueNode.Decode(serviceConfig); err != nil {
			return fmt.Errorf("failed to decode service %s: %w", serviceName, err)
		}

		// Store the pointer to the struct
		(*sm)[serviceName] = serviceConfig
	}

	return nil
}

// MarshalYAML implements custom YAML marshaling for ServiceMap.
func (sm ServiceMap) MarshalYAML() (interface{}, error) {
	return map[string]any(sm), nil
}
