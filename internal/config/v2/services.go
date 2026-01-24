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

	"github.com/rackerlabs/opencenter-cli/internal/config/registry"
	"gopkg.in/yaml.v3"
)

// UnmarshalYAML implements custom YAML unmarshaling for ServiceMap.
// It uses the service registry to determine the correct type for each service.
// Requirements: 17.1, 17.2, 17.7
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
			// Service not registered, skip or use generic map
			var genericConfig map[string]interface{}
			if err := valueNode.Decode(&genericConfig); err != nil {
				return fmt.Errorf("failed to decode service %s: %w", serviceName, err)
			}
			(*sm)[serviceName] = genericConfig
			continue
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

// BaseServiceConfig represents common fields shared by all services.
// Individual service configurations should embed this struct.
type BaseServiceConfig struct {
	Enabled          bool              `yaml:"enabled" json:"enabled"`
	Status           string            `yaml:"status,omitempty" json:"status,omitempty"`
	Namespace        string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Hostname         string            `yaml:"hostname,omitempty" json:"hostname,omitempty"`
	ImageRepository  string            `yaml:"image_repository,omitempty" json:"image_repository,omitempty"`
	ImageTag         string            `yaml:"image_tag,omitempty" json:"image_tag,omitempty"`
	Release          string            `yaml:"release,omitempty" json:"release,omitempty"`
	Branch           string            `yaml:"branch,omitempty" json:"branch,omitempty"`
	URI              string            `yaml:"uri,omitempty" json:"uri,omitempty"`
	GitOpsSourceType string            `yaml:"gitops_source_type,omitempty" json:"gitops_source_type,omitempty"`
	GitOpsSourceURL  string            `yaml:"gitops_source_url,omitempty" json:"gitops_source_url,omitempty"`
	GitOpsSourceRef  string            `yaml:"gitops_source_ref,omitempty" json:"gitops_source_ref,omitempty"`
	Labels           map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations      map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}
