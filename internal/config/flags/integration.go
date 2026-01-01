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
	"reflect"
	"strconv"
	"strings"
)

// CLIIntegration provides integration between the enhanced flag parser and existing CLI commands
type CLIIntegration struct {
	parser *EnhancedFlagParser
}

// NewCLIIntegration creates a new CLI integration with registered handlers
func NewCLIIntegration() (*CLIIntegration, error) {
	parser := NewEnhancedFlagParser()
	
	// Register dedicated array handlers
	if err := parser.RegisterHandler("server-pool", NewServerPoolFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register server-pool handler: %w", err)
	}
	
	if err := parser.RegisterHandler("ssh-key", NewSSHKeyFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register ssh-key handler: %w", err)
	}
	
	if err := parser.RegisterHandler("dns-server", NewDNSServerFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register dns-server handler: %w", err)
	}
	
	if err := parser.RegisterHandler("subnet", NewSubnetFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register subnet handler: %w", err)
	}
	
	// Register JSON flag handler
	if err := parser.RegisterHandler("json-set", NewJSONFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register json-set handler: %w", err)
	}
	
	// Register YAML flag handler
	if err := parser.RegisterHandler("yaml-set|yaml-data|yaml-file", NewYAMLFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register yaml handler: %w", err)
	}
	
	// Register template flag handler
	if err := parser.RegisterHandler("template-var-.*", NewTemplateFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register template handler: %w", err)
	}
	
	// Register array operation handlers
	if err := parser.RegisterHandler("array-append|array-insert|array-remove", NewArrayFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register array operation handler: %w", err)
	}
	
	// Register map operation handlers
	if err := parser.RegisterHandler("map-set|map-merge|map-remove", NewMapFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register map operation handler: %w", err)
	}
	
	return &CLIIntegration{
		parser: parser,
	}, nil
}

// ProcessFlags processes command line arguments and applies them to configuration
func (c *CLIIntegration) ProcessFlags(args []string, configStruct interface{}, configMap map[string]interface{}) error {
	// Parse flags using enhanced parser
	parsed, err := c.parser.ParseFlags(args)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}
	
	// Apply dot notation flags (backward compatibility)
	for key, value := range parsed.DotNotation {
		// Only apply to struct if it's provided
		if configStruct != nil {
			if err := c.setField(configStruct, key, value); err != nil {
				return fmt.Errorf("error setting config from flag '%s': %w", key, err)
			}
		}
		// Always apply to map
		if err := c.setMapField(configMap, key, value); err != nil {
			return fmt.Errorf("error setting config map from flag '%s': %w", key, err)
		}
	}
	
	// Apply array flags
	for _, arrayFlag := range parsed.ArrayFlags {
		if err := c.applyArrayFlag(arrayFlag, configStruct, configMap); err != nil {
			return fmt.Errorf("error applying array flag: %w", err)
		}
	}
	
	// Apply JSON flags
	for _, jsonFlag := range parsed.JSONFlags {
		if err := c.applyJSONFlag(jsonFlag, configStruct, configMap); err != nil {
			return fmt.Errorf("error applying JSON flag: %w", err)
		}
	}
	
	// Apply YAML flags
	for _, yamlFlag := range parsed.YAMLFlags {
		if err := c.applyYAMLFlag(yamlFlag, configStruct, configMap); err != nil {
			return fmt.Errorf("error applying YAML flag: %w", err)
		}
	}
	
	// Apply template variables (process templates after all other flags)
	if len(parsed.TemplateVars) > 0 {
		if err := c.applyTemplateVariables(parsed.TemplateVars, configStruct, configMap); err != nil {
			return fmt.Errorf("error applying template variables: %w", err)
		}
	}
	
	// Apply array operations
	for _, arrayOp := range parsed.ArrayOperations {
		if err := c.applyArrayOperation(arrayOp, configStruct, configMap); err != nil {
			return fmt.Errorf("error applying array operation: %w", err)
		}
	}
	
	// Apply map operations
	for _, mapOp := range parsed.MapOperations {
		if err := c.applyMapOperation(mapOp, configStruct, configMap); err != nil {
			return fmt.Errorf("error applying map operation: %w", err)
		}
	}
	
	return nil
}

// applyArrayFlag applies an array flag to the configuration
func (c *CLIIntegration) applyArrayFlag(arrayFlag ArrayFlag, configStruct interface{}, configMap map[string]interface{}) error {
	switch arrayFlag.Type {
	case "server-pool":
		return c.applyServerPoolFlag(arrayFlag.Config, configStruct, configMap)
	case "ssh-key":
		return c.applySSHKeyFlag(arrayFlag.Config, configStruct, configMap)
	case "dns-server":
		return c.applyDNSServerFlag(arrayFlag.Config, configStruct, configMap)
	case "subnet":
		return c.applySubnetFlag(arrayFlag.Config, configStruct, configMap)
	default:
		return fmt.Errorf("unsupported array flag type: %s", arrayFlag.Type)
	}
}

// applyServerPoolFlag applies a server pool configuration
func (c *CLIIntegration) applyServerPoolFlag(config *ArrayConfig, configStruct interface{}, configMap map[string]interface{}) error {
	// For now, we'll add server pool configurations to a custom field
	// In a full implementation, this would integrate with the actual config structure
	
	// Apply to map
	if err := c.appendToMapArray(configMap, "opencenter.infrastructure.server_pools", config.Fields); err != nil {
		return err
	}
	
	return nil
}

// applySSHKeyFlag applies an SSH key configuration
func (c *CLIIntegration) applySSHKeyFlag(config *ArrayConfig, configStruct interface{}, configMap map[string]interface{}) error {
	// Apply to map
	if err := c.appendToMapArray(configMap, "opencenter.infrastructure.ssh_keys", config.Fields); err != nil {
		return err
	}
	
	return nil
}

// applyDNSServerFlag applies a DNS server configuration
func (c *CLIIntegration) applyDNSServerFlag(config *ArrayConfig, configStruct interface{}, configMap map[string]interface{}) error {
	// Apply to map
	if err := c.appendToMapArray(configMap, "opencenter.networking.dns_servers", config.Fields); err != nil {
		return err
	}
	
	return nil
}

// applySubnetFlag applies a subnet configuration
func (c *CLIIntegration) applySubnetFlag(config *ArrayConfig, configStruct interface{}, configMap map[string]interface{}) error {
	// Apply to map
	if err := c.appendToMapArray(configMap, "opencenter.networking.subnets", config.Fields); err != nil {
		return err
	}
	
	return nil
}

// applyJSONFlag applies a JSON flag to the configuration
func (c *CLIIntegration) applyJSONFlag(jsonFlag JSONFlag, configStruct interface{}, configMap map[string]interface{}) error {
	// Create a JSON handler to merge the configuration
	handler := NewJSONFlagHandler()
	
	// Apply to map
	if err := handler.MergeIntoConfiguration(&jsonFlag, configMap); err != nil {
		return fmt.Errorf("failed to merge JSON flag into configuration map: %w", err)
	}
	
	// TODO: Apply to struct in future implementation
	// For now, we focus on map-based configuration
	
	return nil
}

// applyYAMLFlag applies a YAML flag to the configuration
func (c *CLIIntegration) applyYAMLFlag(yamlFlag YAMLFlag, configStruct interface{}, configMap map[string]interface{}) error {
	// Create a YAML handler to merge the configuration
	handler := NewYAMLFlagHandler()
	
	// Apply to map
	if err := handler.MergeIntoConfiguration(&yamlFlag, configMap); err != nil {
		return fmt.Errorf("failed to merge YAML flag into configuration map: %w", err)
	}
	
	// TODO: Apply to struct in future implementation
	// For now, we focus on map-based configuration
	
	return nil
}

// applyTemplateVariables applies template variables to the configuration
func (c *CLIIntegration) applyTemplateVariables(templateVars map[string]string, configStruct interface{}, configMap map[string]interface{}) error {
	// Create a template processor
	processor := NewDefaultTemplateProcessor()
	
	// Create a configuration object for template processing
	config := &Configuration{
		Data: configMap,
	}
	
	// Process templates with the provided variables
	if err := processor.ProcessTemplates(config, templateVars); err != nil {
		return fmt.Errorf("failed to process templates: %w", err)
	}
	
	// Update the configuration map with processed data
	for key, value := range config.Data {
		configMap[key] = value
	}
	
	// TODO: Apply to struct in future implementation
	// For now, we focus on map-based configuration
	
	return nil
}

// applyArrayOperation applies an array operation to the configuration
func (c *CLIIntegration) applyArrayOperation(arrayOp ArrayOperationFlag, configStruct interface{}, configMap map[string]interface{}) error {
	// Create an array handler to apply the operation
	handler := NewArrayFlagHandler()
	
	// Apply to map
	if err := handler.MergeIntoConfiguration(&arrayOp, configMap); err != nil {
		return fmt.Errorf("failed to apply array operation to configuration map: %w", err)
	}
	
	// TODO: Apply to struct in future implementation
	// For now, we focus on map-based configuration
	
	return nil
}

// applyMapOperation applies a map operation to the configuration
func (c *CLIIntegration) applyMapOperation(mapOp MapFlag, configStruct interface{}, configMap map[string]interface{}) error {
	// Create a map handler to apply the operation
	handler := NewMapFlagHandler()
	
	// Apply to map
	if err := handler.MergeIntoConfiguration(&mapOp, configMap); err != nil {
		return fmt.Errorf("failed to apply map operation to configuration map: %w", err)
	}
	
	// TODO: Apply to struct in future implementation
	// For now, we focus on map-based configuration
	
	return nil
}

// appendToMapArray appends a value to an array in a nested map structure
func (c *CLIIntegration) appendToMapArray(configMap map[string]interface{}, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	current := configMap
	
	// Navigate to the parent of the target array
	for i, part := range parts[:len(parts)-1] {
		if next, exists := current[part]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return fmt.Errorf("field '%s' at path '%s' is not a map", part, strings.Join(parts[:i+1], "."))
			}
		} else {
			// Create new map
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
	
	// Handle the final array field
	arrayField := parts[len(parts)-1]
	if existing, exists := current[arrayField]; exists {
		if existingArray, ok := existing.([]interface{}); ok {
			current[arrayField] = append(existingArray, value)
		} else {
			// Convert existing value to array
			current[arrayField] = []interface{}{existing, value}
		}
	} else {
		// Create new array
		current[arrayField] = []interface{}{value}
	}
	
	return nil
}

// setField sets a field in a struct using dot notation (backward compatibility)
func (c *CLIIntegration) setField(obj interface{}, path string, value string) error {
	v := reflect.ValueOf(obj).Elem()
	parts := strings.Split(path, ".")
	
	for i, part := range parts {
		field := c.findField(v, part)
		
		if !field.IsValid() {
			if v.Kind() == reflect.Map {
				if i != len(parts)-1 {
					return fmt.Errorf("setting nested fields in maps is not supported: %s", path)
				}
				
				if v.Type().Key().Kind() != reflect.String {
					return fmt.Errorf("map key type must be string for path-based setting, got %s", v.Type().Key().Kind())
				}
				
				mapValueType := v.Type().Elem()
				newValue := reflect.New(mapValueType).Elem()
				if err := c.setReflectValue(newValue, value); err != nil {
					return fmt.Errorf("failed to set map value for key '%s': %w", part, err)
				}
				
				v.SetMapIndex(reflect.ValueOf(part), newValue)
				return nil
			}
			return fmt.Errorf("field not found: '%s' in struct '%s'", part, v.Type().Name())
		}
		
		if i == len(parts)-1 {
			return c.setFieldValue(field, value)
		}
		
		if field.Kind() == reflect.Struct {
			v = field
		} else if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			v = field.Elem()
		} else if field.Kind() == reflect.Map {
			if field.IsNil() {
				field.Set(reflect.MakeMap(field.Type()))
			}
			v = field
		} else {
			return fmt.Errorf("field '%s' is not a struct or map, cannot traverse further", part)
		}
	}
	return nil
}

// findField finds a field by yaml tag or name
func (c *CLIIntegration) findField(v reflect.Value, name string) reflect.Value {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag != "" {
			yamlName := strings.Split(yamlTag, ",")[0]
			if yamlName == name {
				return v.Field(i)
			}
		}
		if field.Name == name {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// setFieldValue sets a reflect.Value from a string
func (c *CLIIntegration) setFieldValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("cannot set field value")
	}
	return c.setReflectValue(field, value)
}

// setReflectValue converts string value to the field's type and sets it
func (c *CLIIntegration) setReflectValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: '%s'", value)
		}
		field.SetInt(i)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: '%s'", value)
		}
		field.SetBool(b)
	case reflect.Interface:
		if b, err := strconv.ParseBool(value); err == nil {
			field.Set(reflect.ValueOf(b))
		} else if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			field.Set(reflect.ValueOf(i))
		} else {
			field.Set(reflect.ValueOf(value))
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Type())
	}
	return nil
}

// setMapField sets a field in a map using dot notation (backward compatibility)
func (c *CLIIntegration) setMapField(obj map[string]interface{}, path string, value string) error {
	parts := strings.Split(path, ".")
	current := obj
	
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = c.convertStringValue(value)
			return nil
		}
		
		if next, exists := current[part]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return fmt.Errorf("field '%s' is not a map, cannot traverse further", part)
			}
		} else {
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
	return nil
}

// convertStringValue converts a string to the appropriate type
func (c *CLIIntegration) convertStringValue(value string) interface{} {
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	return value
}