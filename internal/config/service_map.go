package config

import (
	"encoding/json"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// ServiceMap handles polymorphic unmarshalling of service configurations.
// It maps service names to their specific configuration structs.
type ServiceMap map[string]any

// UnmarshalYAML implements the yaml.Unmarshaler interface.
// It uses the service registry to resolve typed structs per service name.
func (sm *ServiceMap) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected map for Services, got %v", node.Kind)
	}

	if *sm == nil {
		*sm = make(ServiceMap)
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		var serviceName string
		if err := keyNode.Decode(&serviceName); err != nil {
			return err
		}

		// Look up registered type
		configType := registry.GetServiceConfigType(serviceName)
		if configType == nil {
			// Unregistered service: use DefaultServiceConfig as fallback
			configType = reflect.TypeOf(services.DefaultServiceConfig{})
		}

		// Create a new instance of the config type
		configPtr := reflect.New(configType).Interface()

		// Unmarshal into the specific struct
		if err := valNode.Decode(configPtr); err != nil {
			return fmt.Errorf("failed to decode config for service %s: %w", serviceName, err)
		}

		(*sm)[serviceName] = configPtr
	}

	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (sm ServiceMap) MarshalYAML() (interface{}, error) {
	return (map[string]any)(sm), nil
}

// MarshalJSON implements the json.Marshaler interface.
func (sm ServiceMap) MarshalJSON() ([]byte, error) {
	return json.Marshal((map[string]any)(sm))
}
