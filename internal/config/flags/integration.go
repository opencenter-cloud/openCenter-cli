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

	if err := parser.RegisterHandler("^subnet$|^subnet-", NewSubnetFlagHandler()); err != nil {
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

	// Register file flag handler
	if err := parser.RegisterHandler("base-config|merge-config|config-stack", NewFileFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register file flag handler: %w", err)
	}

	// Register output flag handler
	if err := parser.RegisterHandler("output-format|dry-run|quiet", NewOutputFlagHandler()); err != nil {
		return nil, fmt.Errorf("failed to register output flag handler: %w", err)
	}

	return &CLIIntegration{
		parser: parser,
	}, nil
}

// ProcessFlags processes command line arguments and applies them to configuration
func (c *CLIIntegration) ProcessFlags(args []string, configStruct interface{}, configMap map[string]interface{}) error {
	return c.ProcessFlagsWithValidation(args, configStruct, configMap, ValidationModeNormal)
}

// ProcessFlagsWithValidation processes command line arguments with validation mode
func (c *CLIIntegration) ProcessFlagsWithValidation(args []string, configStruct interface{}, configMap map[string]interface{}, validationMode ValidationMode) error {
	// Parse flags using enhanced parser
	parsed, err := c.parser.ParseFlags(args)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// Validate flags if requested
	if validationMode == ValidationModeValidateOnly {
		return c.validateOnly(parsed, configStruct, configMap)
	}

	// Validate flags before applying
	validator := NewDefaultConfigurationValidator()
	validationResult, err := validator.ValidateFlags(parsed)
	if err != nil {
		return fmt.Errorf("failed to validate flags: %w", err)
	}

	if !validationResult.Valid {
		return c.formatValidationErrors(validationResult)
	}

	// Apply flags to configuration
	return c.applyFlags(parsed, configStruct, configMap)
}

// validateOnly performs validation without applying changes
func (c *CLIIntegration) validateOnly(parsed *ParsedFlags, configStruct interface{}, configMap map[string]interface{}) error {
	validator := NewDefaultConfigurationValidator()

	// Validate flags
	flagValidation, err := validator.ValidateFlags(parsed)
	if err != nil {
		return fmt.Errorf("failed to validate flags: %w", err)
	}

	// Create a temporary configuration for validation
	tempConfig := make(map[string]interface{})
	for key, value := range configMap {
		tempConfig[key] = value
	}

	// Apply flags to temporary configuration
	if err := c.applyFlags(parsed, nil, tempConfig); err != nil {
		return fmt.Errorf("failed to apply flags for validation: %w", err)
	}

	// Create configuration object for validation
	config := &Configuration{
		Data: tempConfig,
		Sources: []ConfigSource{
			{Type: SourceCLI, Path: "validation", Priority: 1},
		},
	}

	// Validate the resulting configuration
	configValidation, err := validator.ValidateConfiguration(config)
	if err != nil {
		return fmt.Errorf("failed to validate configuration: %w", err)
	}

	// Report validation results
	fmt.Printf("Validation Results:\n")
	fmt.Printf("==================\n\n")

	if flagValidation.Valid && configValidation.Valid {
		fmt.Printf("✓ Configuration is valid\n")
		fmt.Printf("  - Flags processed: %d\n", flagValidation.Summary.FlagsProcessed)
		fmt.Printf("  - Configuration paths: %d\n", configValidation.Summary.ConfigPaths)
	} else {
		fmt.Printf("✗ Configuration has errors\n")

		// Report flag validation errors
		if !flagValidation.Valid {
			fmt.Printf("\nFlag Validation Errors:\n")
			for _, err := range flagValidation.Errors {
				fmt.Printf("  - %s: %s\n", err.Path, err.Message)
				if err.Suggestion != "" {
					fmt.Printf("    Suggestion: %s\n", err.Suggestion)
				}
			}
		}

		// Report configuration validation errors
		if !configValidation.Valid {
			fmt.Printf("\nConfiguration Validation Errors:\n")
			for _, err := range configValidation.Errors {
				fmt.Printf("  - %s: %s\n", err.Path, err.Message)
				if err.Suggestion != "" {
					fmt.Printf("    Suggestion: %s\n", err.Suggestion)
				}
			}
		}

		return fmt.Errorf("validation failed")
	}

	return nil
}

// applyFlags applies parsed flags to configuration
func (c *CLIIntegration) applyFlags(parsed *ParsedFlags, configStruct interface{}, configMap map[string]interface{}) error {
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

	// Apply configuration file merging
	if len(parsed.ConfigFileFlags) > 0 {
		if err := c.applyConfigFileFlags(parsed.ConfigFileFlags, configStruct, configMap); err != nil {
			return fmt.Errorf("error applying configuration file flags: %w", err)
		}
	}

	return nil
}

// formatValidationErrors formats validation errors into a readable error message
func (c *CLIIntegration) formatValidationErrors(result *ValidationResult) error {
	var errorMsg strings.Builder

	fmt.Fprintf(&errorMsg, "Configuration validation failed (%d errors", result.Summary.TotalErrors)
	if result.Summary.TotalWarnings > 0 {
		fmt.Fprintf(&errorMsg, ", %d warnings", result.Summary.TotalWarnings)
	}
	errorMsg.WriteString("):\n\n")

	for _, err := range result.Errors {
		fmt.Fprintf(&errorMsg, "Error: %s", err.Message)
		if err.Path != "" {
			fmt.Fprintf(&errorMsg, " (path: %s)", err.Path)
		}
		errorMsg.WriteString("\n")

		if err.Suggestion != "" {
			fmt.Fprintf(&errorMsg, "  Suggestion: %s\n", err.Suggestion)
		}

		if err.Example != "" {
			fmt.Fprintf(&errorMsg, "  Example: %s\n", err.Example)
		}

		errorMsg.WriteString("\n")
	}

	if len(result.Warnings) > 0 {
		errorMsg.WriteString("Warnings:\n")
		for _, warning := range result.Warnings {
			fmt.Fprintf(&errorMsg, "Warning: %s", warning.Message)
			if warning.Path != "" {
				fmt.Fprintf(&errorMsg, " (path: %s)", warning.Path)
			}
			errorMsg.WriteString("\n")
		}
	}

	return fmt.Errorf("%s", errorMsg.String())
}

// FormatOutput formats the configuration output based on parsed output flags
func (c *CLIIntegration) FormatOutput(config *Configuration, parsed *ParsedFlags) (string, error) {
	formatter := NewDefaultOutputFormatter()

	// Determine output format (default to YAML)
	format := OutputFormatYAML
	if parsed.OutputFormat != nil {
		format = parsed.OutputFormat.Format
	}

	// Determine output mode (default to normal)
	mode := OutputModeNormal
	if parsed.OutputMode != nil {
		mode = parsed.OutputMode.Mode
	}

	return formatter.FormatConfiguration(config, format, mode)
}

// FormatDiff formats a diff between two configurations
func (c *CLIIntegration) FormatDiff(original, updated *Configuration, parsed *ParsedFlags) (string, error) {
	formatter := NewDefaultOutputFormatter()

	// Determine output mode (default to normal)
	mode := OutputModeNormal
	if parsed.OutputMode != nil {
		mode = parsed.OutputMode.Mode
	}

	return formatter.FormatDiff(original, updated, mode)
}

// FormatConflicts formats conflict information
func (c *CLIIntegration) FormatConflicts(conflicts []ConfigConflict, parsed *ParsedFlags) (string, error) {
	formatter := NewDefaultOutputFormatter()

	// Determine output mode (default to normal)
	mode := OutputModeNormal
	if parsed.OutputMode != nil {
		mode = parsed.OutputMode.Mode
	}

	return formatter.FormatConflicts(conflicts, mode)
}

// GetFlagHelp returns help information for CLI flags
func (c *CLIIntegration) GetFlagHelp(flagType FlagType) (string, error) {
	helpSystem := NewDefaultHelpSystem()
	return helpSystem.GetFlagHelp(flagType)
}

// GetAllFlagHelp returns help information for all CLI flags
func (c *CLIIntegration) GetAllFlagHelp() (string, error) {
	helpSystem := NewDefaultHelpSystem()
	return helpSystem.GetAllFlagHelp()
}

// GetExamples returns common usage examples
func (c *CLIIntegration) GetExamples() (string, error) {
	helpSystem := NewDefaultHelpSystem()
	return helpSystem.GetExamples()
}

// GetFlagExamples returns examples for a specific flag type
func (c *CLIIntegration) GetFlagExamples(flagType FlagType) ([]string, error) {
	helpSystem := NewDefaultHelpSystem()
	return helpSystem.GetFlagExamples(flagType)
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

	// Apply to struct if provided
	if configStruct != nil {
		if err := c.applyToStruct(configStruct, jsonFlag.Path, jsonFlag.Value); err != nil {
			return fmt.Errorf("failed to apply JSON flag to struct: %w", err)
		}
	}

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

	// Apply to struct if provided
	if configStruct != nil {
		if err := c.applyToStruct(configStruct, yamlFlag.Path, yamlFlag.Value); err != nil {
			return fmt.Errorf("failed to apply YAML flag to struct: %w", err)
		}
	}

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

	// Apply to struct if provided (template variables are already processed in the map)
	// No direct struct application needed for template variables

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

	// Apply to struct if provided
	if configStruct != nil {
		// For array operations, we need to get the current array value and apply the operation
		if err := c.applyArrayOperationToStruct(configStruct, &arrayOp); err != nil {
			return fmt.Errorf("failed to apply array operation to struct: %w", err)
		}
	}

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

	// Apply to struct if provided
	if configStruct != nil {
		if err := c.applyMapOperationToStruct(configStruct, &mapOp); err != nil {
			return fmt.Errorf("failed to apply map operation to struct: %w", err)
		}
	}

	return nil
}

// applyConfigFileFlags applies configuration file flags by loading and merging files
func (c *CLIIntegration) applyConfigFileFlags(configFileFlags []*ConfigFileFlag, configStruct interface{}, configMap map[string]interface{}) error {
	// Create file handler, configuration merger, and conflict detector
	fileHandler := NewFileFlagHandler()
	merger := NewDefaultConfigurationMerger()
	conflictDetector := NewConflictDetector()

	// Load all configuration files
	configurations := make([]Configuration, 0, len(configFileFlags)+1)

	// Add current configuration as base
	currentConfig := Configuration{
		Data: make(map[string]interface{}),
		Sources: []ConfigSource{
			{Type: SourceCLI, Path: "current", Priority: 1000}, // High priority for current config
		},
	}

	// Copy current config map data
	for key, value := range configMap {
		currentConfig.Data[key] = value
	}

	// Load file configurations
	for _, configFileFlag := range configFileFlags {
		fileConfig, err := fileHandler.LoadConfigurationFile(configFileFlag)
		if err != nil {
			return fmt.Errorf("failed to load configuration file '%s': %w", configFileFlag.Path, err)
		}
		configurations = append(configurations, *fileConfig)
	}

	// Add current configuration last (highest precedence)
	configurations = append(configurations, currentConfig)

	// Detect conflicts before merging
	conflicts, err := conflictDetector.DetectConflicts(configurations)
	if err != nil {
		return fmt.Errorf("failed to detect configuration conflicts: %w", err)
	}

	// Report conflicts if any exist
	if len(conflicts) > 0 {
		fmt.Printf("Configuration conflicts detected:\n%s", conflictDetector.GetConflictReport())
	}

	// Merge all configurations
	mergedConfig, err := merger.MergeConfigurations(configurations)
	if err != nil {
		return fmt.Errorf("failed to merge configuration files: %w", err)
	}

	// Update the configuration map with merged data
	for key, value := range mergedConfig.Data {
		configMap[key] = value
	}

	// Apply to struct if provided (merged data is already in the map)
	// No direct struct application needed for config file merging

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
		// Check if current value is a map
		if v.Kind() == reflect.Map {
			if v.Type().Key().Kind() != reflect.String {
				return fmt.Errorf("map key type must be string for path-based setting, got %s", v.Type().Key().Kind())
			}

			// If this is the last part, set the value directly in the map
			if i == len(parts)-1 {
				mapValueType := v.Type().Elem()
				newValue := reflect.New(mapValueType).Elem()
				if err := c.setReflectValue(newValue, value); err != nil {
					return fmt.Errorf("failed to set map value for key '%s': %w", part, err)
				}
				v.SetMapIndex(reflect.ValueOf(part), newValue)
				return nil
			}

			// Not the last part - need to navigate into the map value
			mapValueType := v.Type().Elem()

			// Get existing value or create new one
			existingValue := v.MapIndex(reflect.ValueOf(part))
			var nextValue reflect.Value

			if existingValue.IsValid() {
				// Value exists, use it
				nextValue = existingValue
			} else {
				// Create new value
				if mapValueType.Kind() == reflect.Interface {
					// For interface{} or any, create a map[string]any for nested navigation
					nextValue = reflect.ValueOf(make(map[string]any))
				} else if mapValueType.Kind() == reflect.Struct {
					nextValue = reflect.New(mapValueType).Elem()
				} else if mapValueType.Kind() == reflect.Map {
					nextValue = reflect.MakeMap(mapValueType)
				} else if mapValueType.Kind() == reflect.Ptr {
					// Create a new pointer to the underlying type
					nextValue = reflect.New(mapValueType.Elem())
				} else {
					return fmt.Errorf("cannot navigate into map value of type %s for key '%s'", mapValueType.Kind(), part)
				}
			}

			// For interface{} values, we need to handle them specially
			if nextValue.Kind() == reflect.Interface && !nextValue.IsNil() {
				nextValue = nextValue.Elem()
			}

			// Handle pointers - dereference them
			if nextValue.Kind() == reflect.Ptr {
				if nextValue.IsNil() {
					// Create a new instance
					nextValue.Set(reflect.New(nextValue.Type().Elem()))
				}
				// Now work with the dereferenced value but keep the pointer for later
				ptrValue := nextValue
				nextValue = nextValue.Elem()

				// If the dereferenced value is a struct or map, we can navigate into it
				if nextValue.Kind() == reflect.Struct || nextValue.Kind() == reflect.Map {
					// Recursively set the remaining path
					remainingPath := strings.Join(parts[i+1:], ".")

					if err := c.setField(ptrValue.Interface(), remainingPath, value); err != nil {
						return err
					}

					// Store the modified pointer back in the map
					v.SetMapIndex(reflect.ValueOf(part), ptrValue)
					return nil
				}

				return fmt.Errorf("cannot navigate through pointer to %s for key '%s'", nextValue.Kind(), part)
			}

			// If the next value is a struct or map, we can navigate into it
			if nextValue.Kind() == reflect.Struct || nextValue.Kind() == reflect.Map {
				// Recursively set the remaining path
				remainingPath := strings.Join(parts[i+1:], ".")

				// Create a pointer to the value so we can modify it
				var ptrValue reflect.Value
				if nextValue.CanAddr() {
					ptrValue = nextValue.Addr()
				} else {
					// Create a new addressable copy
					ptrValue = reflect.New(nextValue.Type())
					ptrValue.Elem().Set(nextValue)
				}

				if err := c.setField(ptrValue.Interface(), remainingPath, value); err != nil {
					return err
				}

				// Store the modified value back in the map
				v.SetMapIndex(reflect.ValueOf(part), ptrValue.Elem())
				return nil
			}

			return fmt.Errorf("cannot navigate through map value of type %s for key '%s'", nextValue.Kind(), part)
		}

		field := c.findField(v, part)

		if !field.IsValid() {
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

// findField finds a field by yaml tag or name, including embedded fields
func (c *CLIIntegration) findField(v reflect.Value, name string) reflect.Value {
	t := v.Type()

	// First pass: check direct fields
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

	// Second pass: check embedded/anonymous fields
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			embeddedField := v.Field(i)
			// Handle pointer to embedded struct
			if embeddedField.Kind() == reflect.Ptr {
				if embeddedField.IsNil() {
					continue
				}
				embeddedField = embeddedField.Elem()
			}
			// Recursively search in embedded struct
			if embeddedField.Kind() == reflect.Struct {
				result := c.findField(embeddedField, name)
				if result.IsValid() {
					return result
				}
			}
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

// applyToStruct applies a value to a struct field using a dot-notation path
func (c *CLIIntegration) applyToStruct(configStruct interface{}, path string, value interface{}) error {
	if configStruct == nil {
		return nil
	}

	// Convert flag name to field path
	fieldPath := c.flagNameToFieldPath(path)

	// Navigate to the target field
	field, err := c.navigateToField(configStruct, fieldPath)
	if err != nil {
		return fmt.Errorf("failed to navigate to field '%s': %w", fieldPath, err)
	}

	// Set the field value with type conversion
	if err := c.setFieldValueTyped(field, value); err != nil {
		return fmt.Errorf("failed to set field '%s': %w", fieldPath, err)
	}

	return nil
}

// flagNameToFieldPath converts a flag name to a struct field path
// Example: "infrastructure.cluster.name" -> "Infrastructure.Cluster.Name"
// Example: "nestedvalue" -> "NestedValue" (handles camelCase)
func (c *CLIIntegration) flagNameToFieldPath(flagName string) string {
	parts := strings.Split(flagName, ".")
	for i, part := range parts {
		// Capitalize first letter of each part
		if len(part) > 0 {
			// Handle camelCase by capitalizing each word
			parts[i] = toCamelCase(part)
		}
	}
	return strings.Join(parts, ".")
}

// toCamelCase converts a string to CamelCase (PascalCase)
// Examples: "name" -> "Name", "nestedvalue" -> "NestedValue", "nested_value" -> "NestedValue"
func toCamelCase(s string) string {
	// Split by underscore or dash
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	})

	if len(words) == 0 {
		return strings.ToUpper(s[:1]) + s[1:]
	}

	// Capitalize each word
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, "")
}

// navigateToField navigates through a struct to find the target field
func (c *CLIIntegration) navigateToField(obj interface{}, path string) (reflect.Value, error) {
	v := reflect.ValueOf(obj)

	// Dereference pointer if needed
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, fmt.Errorf("nil pointer")
		}
		v = v.Elem()
	}

	parts := strings.Split(path, ".")

	for i, part := range parts {
		// Check if current value is a map
		if v.Kind() == reflect.Map {
			if v.Type().Key().Kind() != reflect.String {
				return reflect.Value{}, fmt.Errorf("map key type must be string, got %s", v.Type().Key().Kind())
			}

			// If this is the last part, we'll return a special value that indicates map access
			if i == len(parts)-1 {
				// For maps, we need to handle setting differently
				// Return the map itself and let the caller handle the key
				return v, nil
			}

			// Navigate into the map
			mapKey := reflect.ValueOf(part)
			mapValue := v.MapIndex(mapKey)

			if !mapValue.IsValid() {
				// Create new value for this key
				mapValueType := v.Type().Elem()
				if mapValueType.Kind() == reflect.Interface {
					// For interface{}, create a map for nested navigation
					newMap := make(map[string]interface{})
					v.SetMapIndex(mapKey, reflect.ValueOf(newMap))
					v = reflect.ValueOf(newMap)
				} else if mapValueType.Kind() == reflect.Struct {
					newValue := reflect.New(mapValueType).Elem()
					v.SetMapIndex(mapKey, newValue)
					v = newValue
				} else if mapValueType.Kind() == reflect.Map {
					newValue := reflect.MakeMap(mapValueType)
					v.SetMapIndex(mapKey, newValue)
					v = newValue
				} else {
					return reflect.Value{}, fmt.Errorf("cannot navigate into map value of type %s", mapValueType.Kind())
				}
			} else {
				// Handle interface values
				if mapValue.Kind() == reflect.Interface && !mapValue.IsNil() {
					mapValue = mapValue.Elem()
				}
				v = mapValue
			}
			continue
		}

		// Find the field in the struct
		field := c.findField(v, part)
		if !field.IsValid() {
			return reflect.Value{}, fmt.Errorf("field '%s' not found in struct '%s'", part, v.Type().Name())
		}

		// If this is the last part, return the field
		if i == len(parts)-1 {
			return field, nil
		}

		// Navigate deeper
		if field.Kind() == reflect.Struct {
			v = field
		} else if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				// Initialize nil pointer
				field.Set(reflect.New(field.Type().Elem()))
			}
			v = field.Elem()
		} else if field.Kind() == reflect.Map {
			if field.IsNil() {
				field.Set(reflect.MakeMap(field.Type()))
			}
			v = field
		} else {
			return reflect.Value{}, fmt.Errorf("cannot navigate through field '%s' of type %s", part, field.Kind())
		}
	}

	return v, nil
}

// setFieldValueTyped sets a field value with proper type conversion
func (c *CLIIntegration) setFieldValueTyped(field reflect.Value, value interface{}) error {
	if !field.IsValid() {
		return fmt.Errorf("invalid field")
	}

	if !field.CanSet() {
		return fmt.Errorf("cannot set field")
	}

	// Get the value as a reflect.Value
	valueReflect := reflect.ValueOf(value)

	// Handle different field types
	switch field.Kind() {
	case reflect.String:
		return c.setStringField(field, value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return c.setIntField(field, value)
	case reflect.Bool:
		return c.setBoolField(field, value)
	case reflect.Slice:
		return c.setSliceField(field, value)
	case reflect.Map:
		return c.setMapFieldValue(field, value)
	case reflect.Interface:
		// For interface{} fields, set the value directly
		field.Set(valueReflect)
		return nil
	case reflect.Ptr:
		// Handle pointer fields
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return c.setFieldValueTyped(field.Elem(), value)
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
}

// setStringField sets a string field from various value types
func (c *CLIIntegration) setStringField(field reflect.Value, value interface{}) error {
	switch v := value.(type) {
	case string:
		field.SetString(v)
	case int, int8, int16, int32, int64:
		field.SetString(fmt.Sprintf("%d", v))
	case float32, float64:
		field.SetString(fmt.Sprintf("%f", v))
	case bool:
		field.SetString(fmt.Sprintf("%t", v))
	default:
		field.SetString(fmt.Sprintf("%v", v))
	}
	return nil
}

// setIntField sets an integer field from various value types
func (c *CLIIntegration) setIntField(field reflect.Value, value interface{}) error {
	switch v := value.(type) {
	case int:
		field.SetInt(int64(v))
	case int8:
		field.SetInt(int64(v))
	case int16:
		field.SetInt(int64(v))
	case int32:
		field.SetInt(int64(v))
	case int64:
		field.SetInt(v)
	case float32:
		field.SetInt(int64(v))
	case float64:
		field.SetInt(int64(v))
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot convert string '%s' to int: %w", v, err)
		}
		field.SetInt(i)
	default:
		return fmt.Errorf("cannot convert %T to int", value)
	}
	return nil
}

// setBoolField sets a boolean field from various value types
func (c *CLIIntegration) setBoolField(field reflect.Value, value interface{}) error {
	switch v := value.(type) {
	case bool:
		field.SetBool(v)
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("cannot convert string '%s' to bool: %w", v, err)
		}
		field.SetBool(b)
	case int, int8, int16, int32, int64:
		field.SetBool(reflect.ValueOf(v).Int() != 0)
	default:
		return fmt.Errorf("cannot convert %T to bool", value)
	}
	return nil
}

// setSliceField sets a slice field from various value types
func (c *CLIIntegration) setSliceField(field reflect.Value, value interface{}) error {
	valueReflect := reflect.ValueOf(value)

	// If value is already a slice, convert it
	if valueReflect.Kind() == reflect.Slice {
		// Create a new slice of the correct type
		newSlice := reflect.MakeSlice(field.Type(), valueReflect.Len(), valueReflect.Len())

		// Copy elements with type conversion
		for i := 0; i < valueReflect.Len(); i++ {
			elem := valueReflect.Index(i)
			newElem := newSlice.Index(i)

			if err := c.setFieldValueTyped(newElem, elem.Interface()); err != nil {
				return fmt.Errorf("failed to set slice element %d: %w", i, err)
			}
		}

		field.Set(newSlice)
		return nil
	}

	// If value is a single item, create a slice with one element
	newSlice := reflect.MakeSlice(field.Type(), 1, 1)
	if err := c.setFieldValueTyped(newSlice.Index(0), value); err != nil {
		return fmt.Errorf("failed to set slice element: %w", err)
	}
	field.Set(newSlice)
	return nil
}

// setMapFieldValue sets a map field from various value types
func (c *CLIIntegration) setMapFieldValue(field reflect.Value, value interface{}) error {
	valueReflect := reflect.ValueOf(value)

	// If value is already a map, convert it
	if valueReflect.Kind() == reflect.Map {
		// Create a new map of the correct type
		newMap := reflect.MakeMap(field.Type())

		// Copy key-value pairs with type conversion
		iter := valueReflect.MapRange()
		for iter.Next() {
			key := iter.Key()
			val := iter.Value()

			// Convert key
			newKey := reflect.New(field.Type().Key()).Elem()
			if err := c.setFieldValueTyped(newKey, key.Interface()); err != nil {
				return fmt.Errorf("failed to convert map key: %w", err)
			}

			// Convert value
			newVal := reflect.New(field.Type().Elem()).Elem()
			if err := c.setFieldValueTyped(newVal, val.Interface()); err != nil {
				return fmt.Errorf("failed to convert map value: %w", err)
			}

			newMap.SetMapIndex(newKey, newVal)
		}

		field.Set(newMap)
		return nil
	}

	return fmt.Errorf("cannot convert %T to map", value)
}

// applyArrayOperationToStruct applies an array operation to a struct field
func (c *CLIIntegration) applyArrayOperationToStruct(configStruct interface{}, arrayOp *ArrayOperationFlag) error {
	// Navigate to the array field
	field, err := c.navigateToField(configStruct, c.flagNameToFieldPath(arrayOp.Path))
	if err != nil {
		return err
	}

	if field.Kind() != reflect.Slice {
		return fmt.Errorf("field '%s' is not a slice", arrayOp.Path)
	}

	// Apply the operation based on the operation type
	switch arrayOp.Operation {
	case "append":
		// Append the value to the slice
		newElem := reflect.New(field.Type().Elem()).Elem()
		if err := c.setFieldValueTyped(newElem, arrayOp.Value); err != nil {
			return err
		}
		field.Set(reflect.Append(field, newElem))

	case "insert":
		// Insert at the specified index
		if arrayOp.Index < 0 || arrayOp.Index > field.Len() {
			return fmt.Errorf("index %d out of range for slice of length %d", arrayOp.Index, field.Len())
		}

		newElem := reflect.New(field.Type().Elem()).Elem()
		if err := c.setFieldValueTyped(newElem, arrayOp.Value); err != nil {
			return err
		}

		// Create new slice with room for one more element
		newSlice := reflect.MakeSlice(field.Type(), field.Len()+1, field.Len()+1)

		// Copy elements before insertion point
		reflect.Copy(newSlice, field.Slice(0, arrayOp.Index))

		// Set the new element
		newSlice.Index(arrayOp.Index).Set(newElem)

		// Copy elements after insertion point
		if arrayOp.Index < field.Len() {
			reflect.Copy(newSlice.Slice(arrayOp.Index+1, newSlice.Len()), field.Slice(arrayOp.Index, field.Len()))
		}

		field.Set(newSlice)

	case "remove":
		// Remove the element at the specified index
		if arrayOp.Index < 0 || arrayOp.Index >= field.Len() {
			return fmt.Errorf("index %d out of range for slice of length %d", arrayOp.Index, field.Len())
		}

		// Create new slice without the element
		newSlice := reflect.MakeSlice(field.Type(), field.Len()-1, field.Len()-1)

		// Copy elements before removal point
		reflect.Copy(newSlice, field.Slice(0, arrayOp.Index))

		// Copy elements after removal point
		if arrayOp.Index < field.Len()-1 {
			reflect.Copy(newSlice.Slice(arrayOp.Index, newSlice.Len()), field.Slice(arrayOp.Index+1, field.Len()))
		}

		field.Set(newSlice)

	default:
		return fmt.Errorf("unsupported array operation: %s", arrayOp.Operation)
	}

	return nil
}

// applyMapOperationToStruct applies a map operation to a struct field
func (c *CLIIntegration) applyMapOperationToStruct(configStruct interface{}, mapOp *MapFlag) error {
	// Navigate to the map field
	field, err := c.navigateToField(configStruct, c.flagNameToFieldPath(mapOp.Path))
	if err != nil {
		return err
	}

	if field.Kind() != reflect.Map {
		return fmt.Errorf("field '%s' is not a map", mapOp.Path)
	}

	// Ensure map is initialized
	if field.IsNil() {
		field.Set(reflect.MakeMap(field.Type()))
	}

	// Apply the operation based on the operation type
	switch mapOp.Operation {
	case "set":
		// Set a single key-value pair
		key := reflect.ValueOf(mapOp.Key)
		val := reflect.New(field.Type().Elem()).Elem()
		if err := c.setFieldValueTyped(val, mapOp.Value); err != nil {
			return err
		}
		field.SetMapIndex(key, val)

	case "merge":
		// Merge a map into the existing map
		valueMap, ok := mapOp.Value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("merge operation requires a map value")
		}

		for k, v := range valueMap {
			key := reflect.ValueOf(k)
			val := reflect.New(field.Type().Elem()).Elem()
			if err := c.setFieldValueTyped(val, v); err != nil {
				return fmt.Errorf("failed to set map value for key '%s': %w", k, err)
			}
			field.SetMapIndex(key, val)
		}

	case "remove":
		// Remove a key from the map
		key := reflect.ValueOf(mapOp.Key)
		field.SetMapIndex(key, reflect.Value{})

	default:
		return fmt.Errorf("unsupported map operation: %s", mapOp.Operation)
	}

	return nil
}
