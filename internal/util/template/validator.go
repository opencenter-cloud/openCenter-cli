/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"text/template/parse"
)

// DefaultTemplateValidator implements TemplateValidator interface
type DefaultTemplateValidator struct {
	templates *template.Template
}

// NewDefaultTemplateValidator creates a new default template validator
func NewDefaultTemplateValidator() *DefaultTemplateValidator {
	return &DefaultTemplateValidator{}
}

// Init initializes the template validator with templates
func (v *DefaultTemplateValidator) Init(templates *template.Template) error {
	if templates == nil {
		return fmt.Errorf("templates cannot be nil")
	}
	v.templates = templates
	return nil
}

// ValidateTemplate validates that a template exists and can be parsed
func (v *DefaultTemplateValidator) ValidateTemplate(templateName string) error {
	if v.templates == nil {
		return NewInitializationError("template validator", fmt.Errorf("templates not initialized"))
	}

	tmpl := v.templates.Lookup(templateName)
	if tmpl == nil {
		availableTemplates := v.getAvailableTemplateNames()
		return NewTemplateNotFoundError(templateName, availableTemplates)
	}

	// Validate template syntax
	if err := v.validateTemplateSyntax(tmpl); err != nil {
		return NewTemplateParsingError(templateName, err)
	}

	return nil
}

// ValidateTemplateWithData performs comprehensive validation of template with data
func (v *DefaultTemplateValidator) ValidateTemplateWithData(templateName string, data interface{}) *TemplateValidationResult {
	result := &TemplateValidationResult{
		Valid:    true,
		Errors:   []*TemplateError{},
		Warnings: []*TemplateError{},
	}

	// Basic template existence validation
	if err := v.ValidateTemplate(templateName); err != nil {
		if templateErr, ok := GetTemplateError(err); ok {
			result.Errors = append(result.Errors, templateErr)
		} else {
			result.Errors = append(result.Errors, NewTemplateError(ErrorTypeValidation, templateName, "template validation failed", err))
		}
		result.Valid = false
		return result
	}

	// Get template
	tmpl := v.templates.Lookup(templateName)
	if tmpl == nil {
		result.Errors = append(result.Errors, NewTemplateNotFoundError(templateName, v.getAvailableTemplateNames()))
		result.Valid = false
		return result
	}

	// Extract variables from template
	variables, err := v.extractTemplateVariables(tmpl)
	if err != nil {
		result.Errors = append(result.Errors, NewTemplateError(ErrorTypeValidation, templateName, "failed to extract template variables", err))
		result.Valid = false
		return result
	}

	// Validate variable substitution
	if err := v.validateVariableSubstitution(templateName, data, variables, result); err != nil {
		result.Valid = false
	}

	// Perform dry-run execution to catch runtime errors
	if err := v.validateTemplateExecution(tmpl, data); err != nil {
		result.Errors = append(result.Errors, NewTemplateExecutionError(templateName, err))
		result.Valid = false
	}

	return result
}

// validateTemplateSyntax validates template syntax
func (v *DefaultTemplateValidator) validateTemplateSyntax(tmpl *template.Template) error {
	if tmpl.Tree == nil {
		return fmt.Errorf("template has no parse tree")
	}

	// Check for common syntax issues
	if err := v.validateParseTree(tmpl.Tree.Root); err != nil {
		return fmt.Errorf("syntax validation failed: %w", err)
	}

	return nil
}

// validateParseTree recursively validates the parse tree for syntax issues
func (v *DefaultTemplateValidator) validateParseTree(node parse.Node) error {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				if err := v.validateParseTree(child); err != nil {
					return err
				}
			}
		}
	case *parse.ActionNode:
		if n.Pipe == nil {
			return fmt.Errorf("action node has no pipe")
		}
		return v.validatePipe(n.Pipe)
	case *parse.IfNode:
		if err := v.validateParseTree(n.Pipe); err != nil {
			return err
		}
		if err := v.validateParseTree(n.List); err != nil {
			return err
		}
		if n.ElseList != nil {
			if err := v.validateParseTree(n.ElseList); err != nil {
				return err
			}
		}
	case *parse.RangeNode:
		if err := v.validateParseTree(n.Pipe); err != nil {
			return err
		}
		if err := v.validateParseTree(n.List); err != nil {
			return err
		}
		if n.ElseList != nil {
			if err := v.validateParseTree(n.ElseList); err != nil {
				return err
			}
		}
	case *parse.WithNode:
		if err := v.validateParseTree(n.Pipe); err != nil {
			return err
		}
		if err := v.validateParseTree(n.List); err != nil {
			return err
		}
		if n.ElseList != nil {
			if err := v.validateParseTree(n.ElseList); err != nil {
				return err
			}
		}
	case *parse.TemplateNode:
		if n.Name == "" {
			return fmt.Errorf("template node has empty name")
		}
		if n.Pipe != nil {
			return v.validateParseTree(n.Pipe)
		}
	}

	return nil
}

// validatePipe validates a pipe node
func (v *DefaultTemplateValidator) validatePipe(pipe *parse.PipeNode) error {
	if pipe == nil {
		return fmt.Errorf("pipe is nil")
	}

	for _, cmd := range pipe.Cmds {
		if err := v.validateCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

// validateCommand validates a command node
func (v *DefaultTemplateValidator) validateCommand(cmd *parse.CommandNode) error {
	if cmd == nil || len(cmd.Args) == 0 {
		return fmt.Errorf("command has no arguments")
	}

	// Validate each argument
	for _, arg := range cmd.Args {
		if err := v.validateArgument(arg); err != nil {
			return err
		}
	}

	return nil
}

// validateArgument validates a command argument
func (v *DefaultTemplateValidator) validateArgument(arg parse.Node) error {
	switch a := arg.(type) {
	case *parse.FieldNode:
		if len(a.Ident) == 0 {
			return fmt.Errorf("field node has no identifiers")
		}
	case *parse.VariableNode:
		if len(a.Ident) == 0 {
			return fmt.Errorf("variable node has no identifiers")
		}
	case *parse.DotNode:
		// Dot node is always valid
	case *parse.StringNode:
		// String nodes are always valid
	case *parse.NumberNode:
		// Number nodes are always valid
	case *parse.BoolNode:
		// Bool nodes are always valid
	case *parse.NilNode:
		// Nil nodes are always valid
	}

	return nil
}

// extractTemplateVariables extracts all variables used in a template
func (v *DefaultTemplateValidator) extractTemplateVariables(tmpl *template.Template) ([]VariableInfo, error) {
	if tmpl.Tree == nil {
		return nil, fmt.Errorf("template has no parse tree")
	}

	var variables []VariableInfo
	v.extractVariablesFromNode(tmpl.Tree.Root, "", &variables)

	return variables, nil
}

// extractVariablesFromNode recursively extracts variables from parse nodes
func (v *DefaultTemplateValidator) extractVariablesFromNode(node parse.Node, path string, variables *[]VariableInfo) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				v.extractVariablesFromNode(child, path, variables)
			}
		}
	case *parse.ActionNode:
		if n.Pipe != nil {
			v.extractVariablesFromPipe(n.Pipe, path, variables)
		}
	case *parse.IfNode:
		v.extractVariablesFromNode(n.Pipe, path, variables)
		v.extractVariablesFromNode(n.List, path, variables)
		if n.ElseList != nil {
			v.extractVariablesFromNode(n.ElseList, path, variables)
		}
	case *parse.RangeNode:
		v.extractVariablesFromNode(n.Pipe, path, variables)
		v.extractVariablesFromNode(n.List, path, variables)
		if n.ElseList != nil {
			v.extractVariablesFromNode(n.ElseList, path, variables)
		}
	case *parse.WithNode:
		v.extractVariablesFromNode(n.Pipe, path, variables)
		v.extractVariablesFromNode(n.List, path, variables)
		if n.ElseList != nil {
			v.extractVariablesFromNode(n.ElseList, path, variables)
		}
	case *parse.TemplateNode:
		if n.Pipe != nil {
			v.extractVariablesFromNode(n.Pipe, path, variables)
		}
	}
}

// extractVariablesFromPipe extracts variables from a pipe node
func (v *DefaultTemplateValidator) extractVariablesFromPipe(pipe *parse.PipeNode, path string, variables *[]VariableInfo) {
	if pipe == nil {
		return
	}

	for _, cmd := range pipe.Cmds {
		v.extractVariablesFromCommand(cmd, path, variables)
	}
}

// extractVariablesFromCommand extracts variables from a command node
func (v *DefaultTemplateValidator) extractVariablesFromCommand(cmd *parse.CommandNode, path string, variables *[]VariableInfo) {
	if cmd == nil {
		return
	}

	for _, arg := range cmd.Args {
		v.extractVariablesFromArgument(arg, path, variables)
	}
}

// extractVariablesFromArgument extracts variables from command arguments
func (v *DefaultTemplateValidator) extractVariablesFromArgument(arg parse.Node, path string, variables *[]VariableInfo) {
	switch a := arg.(type) {
	case *parse.FieldNode:
		if len(a.Ident) > 0 {
			varPath := path
			if varPath != "" {
				varPath += "."
			}
			varPath += strings.Join(a.Ident, ".")

			*variables = append(*variables, VariableInfo{
				Name:     strings.Join(a.Ident, "."),
				Path:     varPath,
				Required: true, // Assume required unless proven otherwise
			})
		}
	case *parse.VariableNode:
		if len(a.Ident) > 0 {
			varName := strings.Join(a.Ident, ".")
			*variables = append(*variables, VariableInfo{
				Name:     varName,
				Path:     varName,
				Required: true,
			})
		}
	}
}

// validateVariableSubstitution validates that all template variables can be substituted
func (v *DefaultTemplateValidator) validateVariableSubstitution(templateName string, data interface{}, variables []VariableInfo, result *TemplateValidationResult) error {
	if data == nil {
		for _, variable := range variables {
			if variable.Required {
				result.MissingVariables = append(result.MissingVariables, variable.Name)
				result.Errors = append(result.Errors, NewDataValidationError(templateName, variable.Name, fmt.Errorf("required variable '%s' is missing (data is nil)", variable.Name)))
			}
		}
		if len(result.MissingVariables) > 0 {
			return fmt.Errorf("missing required variables")
		}
		return nil
	}

	// Check each variable
	for _, variable := range variables {
		if err := v.validateVariableAccess(data, variable.Name); err != nil {
			result.MissingVariables = append(result.MissingVariables, variable.Name)
			result.Errors = append(result.Errors, NewDataValidationError(templateName, variable.Name, fmt.Errorf("variable '%s' cannot be accessed: %w", variable.Name, err)))
		}
	}

	// Check for unused variables in data (warnings)
	v.findUnusedVariables(data, variables, result)

	if len(result.MissingVariables) > 0 {
		return fmt.Errorf("missing required variables: %s", strings.Join(result.MissingVariables, ", "))
	}

	return nil
}

// validateVariableAccess validates that a variable can be accessed from the data
func (v *DefaultTemplateValidator) validateVariableAccess(data interface{}, variablePath string) error {
	if data == nil {
		return fmt.Errorf("data is nil")
	}

	// Handle dot notation
	parts := strings.Split(variablePath, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return fmt.Errorf("nil value at path segment '%s' (full path: %s)", part, variablePath)
		}

		value := reflect.ValueOf(current)
		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return fmt.Errorf("nil pointer at path segment '%s' (full path: %s)", part, variablePath)
			}
			value = value.Elem()
		}

		switch value.Kind() {
		case reflect.Struct:
			field := value.FieldByName(part)
			if !field.IsValid() {
				return fmt.Errorf("field '%s' not found in struct (full path: %s)", part, variablePath)
			}
			current = field.Interface()
		case reflect.Map:
			mapValue := value.MapIndex(reflect.ValueOf(part))
			if !mapValue.IsValid() {
				return fmt.Errorf("key '%s' not found in map (full path: %s)", part, variablePath)
			}
			current = mapValue.Interface()
		default:
			// For the last part, we might be accessing a method or field that doesn't exist yet
			if i == len(parts)-1 {
				// This is acceptable for the final access
				return nil
			}
			return fmt.Errorf("cannot access '%s' on type %s (full path: %s)", part, value.Kind(), variablePath)
		}
	}

	return nil
}

// findUnusedVariables finds variables in data that are not used in the template
func (v *DefaultTemplateValidator) findUnusedVariables(data interface{}, usedVariables []VariableInfo, result *TemplateValidationResult) {
	if data == nil {
		return
	}

	// Create a map of used variable names
	usedNames := make(map[string]bool)
	for _, variable := range usedVariables {
		usedNames[variable.Name] = true
		// Also mark parent paths as used
		parts := strings.Split(variable.Name, ".")
		for i := 1; i < len(parts); i++ {
			parentPath := strings.Join(parts[:i], ".")
			usedNames[parentPath] = true
		}
	}

	// Find unused variables
	v.findUnusedVariablesRecursive(data, "", usedNames, result)
}

// findUnusedVariablesRecursive recursively finds unused variables
func (v *DefaultTemplateValidator) findUnusedVariablesRecursive(data interface{}, path string, usedNames map[string]bool, result *TemplateValidationResult) {
	if data == nil {
		return
	}

	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}

			fieldPath := field.Name
			if path != "" {
				fieldPath = path + "." + field.Name
			}

			if !usedNames[fieldPath] && !usedNames[field.Name] {
				result.UnusedVariables = append(result.UnusedVariables, fieldPath)
			}

			// Recurse into nested structures
			fieldValue := value.Field(i)
			if fieldValue.CanInterface() {
				v.findUnusedVariablesRecursive(fieldValue.Interface(), fieldPath, usedNames, result)
			}
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			keyStr := fmt.Sprintf("%v", key.Interface())
			keyPath := keyStr
			if path != "" {
				keyPath = path + "." + keyStr
			}

			if !usedNames[keyPath] && !usedNames[keyStr] {
				result.UnusedVariables = append(result.UnusedVariables, keyPath)
			}

			// Recurse into map values
			mapValue := value.MapIndex(key)
			if mapValue.CanInterface() {
				v.findUnusedVariablesRecursive(mapValue.Interface(), keyPath, usedNames, result)
			}
		}
	}
}

// validateTemplateExecution performs a dry-run execution to catch runtime errors
func (v *DefaultTemplateValidator) validateTemplateExecution(tmpl *template.Template, data interface{}) error {
	var buf bytes.Buffer

	// Try to execute the template
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("template execution failed: %w", err)
	}

	return nil
}

// getAvailableTemplateNames returns a list of available template names
func (v *DefaultTemplateValidator) getAvailableTemplateNames() []string {
	if v.templates == nil {
		return []string{}
	}

	var names []string
	for _, tmpl := range v.templates.Templates() {
		if tmpl.Name() != "" {
			names = append(names, tmpl.Name())
		}
	}
	return names
}

// ValidateTemplateData validates that data contains required fields for a template
func (v *DefaultTemplateValidator) ValidateTemplateData(templateName string, data interface{}) error {
	if err := v.ValidateTemplateExists(templateName); err != nil {
		return err
	}

	// Basic validation - check if data is not nil
	if data == nil {
		return fmt.Errorf("template data cannot be nil for template: %s", templateName)
	}

	// Additional validation could be added here based on template requirements
	return nil
}

// ValidateTemplateExists validates that a template exists
func (v *DefaultTemplateValidator) ValidateTemplateExists(templateName string) error {
	if v.templates == nil {
		return fmt.Errorf("templates not initialized")
	}

	tmpl := v.templates.Lookup(templateName)
	if tmpl == nil {
		return fmt.Errorf("template not found: %s", templateName)
	}

	return nil
}

// ValidateRequiredFields validates that data contains all required fields
func (v *DefaultTemplateValidator) ValidateRequiredFields(data interface{}, requiredFields []string) error {
	if data == nil {
		return fmt.Errorf("data cannot be nil when validating required fields")
	}

	if len(requiredFields) == 0 {
		return nil // No required fields to validate
	}

	// Use reflection to check for required fields
	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	var missingFields []string

	switch value.Kind() {
	case reflect.Struct:
		// Validate struct fields
		for _, fieldName := range requiredFields {
			field := value.FieldByName(fieldName)
			if !field.IsValid() || isFieldEmpty(field) {
				missingFields = append(missingFields, fieldName)
			}
		}
	case reflect.Map:
		// Validate map keys
		for _, fieldName := range requiredFields {
			mapValue := value.MapIndex(reflect.ValueOf(fieldName))
			if !mapValue.IsValid() || isFieldEmpty(mapValue) {
				missingFields = append(missingFields, fieldName)
			}
		}
	default:
		return fmt.Errorf("unsupported data type for field validation: %s", value.Kind())
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missingFields, ", "))
	}

	return nil
}

// ValidateNetworkPluginConfig validates network plugin configuration
func (v *DefaultTemplateValidator) ValidateNetworkPluginConfig(pluginType string, config map[string]interface{}) error {
	if pluginType == "" {
		return fmt.Errorf("network plugin type cannot be empty")
	}

	supportedPlugins := []string{"calico", "cilium", "kube-ovn", "flannel"}
	isSupported := false
	for _, supported := range supportedPlugins {
		if pluginType == supported {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return fmt.Errorf("unsupported network plugin: %s, supported plugins: %s",
			pluginType, strings.Join(supportedPlugins, ", "))
	}

	// Plugin-specific validation
	switch pluginType {
	case "calico":
		return v.validateCalicoConfigValidator(config)
	case "cilium":
		return v.validateCiliumConfigValidator(config)
	case "kube-ovn":
		return v.validateKubeOVNConfigValidator(config)
	case "flannel":
		return v.validateFlannelConfigValidator(config)
	}

	return nil
}

// validateCalicoConfigValidator validates Calico-specific configuration
func (v *DefaultTemplateValidator) validateCalicoConfigValidator(config map[string]interface{}) error {
	// Calico-specific validation logic
	if config == nil {
		return nil // Default configuration is acceptable
	}

	// Check for conflicting configurations
	if ipv4Pool, exists := config["ipv4_pool"]; exists {
		if ipv4PoolStr, ok := ipv4Pool.(string); ok {
			if ipv4PoolStr == "" {
				return fmt.Errorf("calico ipv4_pool cannot be empty")
			}
			// Basic CIDR validation
			if !strings.Contains(ipv4PoolStr, "/") {
				return fmt.Errorf("invalid IPv4 pool format: %s", ipv4PoolStr)
			}
		}
	}

	// Validate MTU
	if mtu, exists := config["mtu"]; exists {
		if mtuInt, ok := mtu.(int); ok {
			if mtuInt < 68 || mtuInt > 9000 {
				return fmt.Errorf("invalid MTU value: %d, must be between 68 and 9000", mtuInt)
			}
		}
	}

	return nil
}

// validateCiliumConfigValidator validates Cilium-specific configuration
func (v *DefaultTemplateValidator) validateCiliumConfigValidator(config map[string]interface{}) error {
	// Cilium-specific validation logic
	if config == nil {
		return nil // Default configuration is acceptable
	}

	// Validate cluster pool IPv4 CIDR
	if cidr, exists := config["cluster_pool_ipv4_cidr"]; exists {
		if cidrStr, ok := cidr.(string); ok && cidrStr != "" {
			if !strings.Contains(cidrStr, "/") {
				return fmt.Errorf("invalid cluster pool IPv4 CIDR format: %s", cidrStr)
			}
		}
	}

	// Validate mask size
	if maskSize, exists := config["cluster_pool_ipv4_mask_size"]; exists {
		if maskSizeInt, ok := maskSize.(int); ok {
			if maskSizeInt < 8 || maskSizeInt > 30 {
				return fmt.Errorf("invalid cluster pool IPv4 mask size: %d, must be between 8 and 30", maskSizeInt)
			}
		}
	}

	// Check for Cilium-specific requirements
	if hubble, exists := config["hubble"]; exists {
		if hubbleMap, ok := hubble.(map[string]interface{}); ok {
			if enabled, exists := hubbleMap["enabled"]; exists {
				if enabledBool, ok := enabled.(bool); ok && enabledBool {
					// Validate Hubble configuration
					if ui, exists := hubbleMap["ui"]; exists {
						if uiMap, ok := ui.(map[string]interface{}); ok {
							if _, exists := uiMap["enabled"]; !exists {
								return fmt.Errorf("hubble ui configuration missing enabled field")
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// validateKubeOVNConfigValidator validates Kube-OVN-specific configuration
func (v *DefaultTemplateValidator) validateKubeOVNConfigValidator(config map[string]interface{}) error {
	// Kube-OVN-specific validation logic
	if config == nil {
		return nil // Default configuration is acceptable
	}

	// Check for Kube-OVN specific requirements
	if subnet, exists := config["default_subnet"]; exists {
		if subnetStr, ok := subnet.(string); ok {
			if subnetStr == "" {
				return fmt.Errorf("kube-ovn default_subnet cannot be empty")
			}
			if !strings.Contains(subnetStr, "/") {
				return fmt.Errorf("invalid default subnet format: %s", subnetStr)
			}
		}
	}

	return nil
}

// validateFlannelConfigValidator validates Flannel-specific configuration
func (v *DefaultTemplateValidator) validateFlannelConfigValidator(config map[string]interface{}) error {
	// Flannel-specific validation logic
	if config == nil {
		return nil // Default configuration is acceptable
	}

	// Validate network
	if network, exists := config["network"]; exists {
		if networkStr, ok := network.(string); ok && networkStr != "" {
			if !strings.Contains(networkStr, "/") {
				return fmt.Errorf("invalid network format: %s", networkStr)
			}
		}
	}

	// Validate backend type
	if backend, exists := config["backend"]; exists {
		if backendMap, ok := backend.(map[string]interface{}); ok {
			if backendType, exists := backendMap["type"]; exists {
				validTypes := []string{"vxlan", "host-gw", "udp"}
				isValid := false
				for _, validType := range validTypes {
					if backendType == validType {
						isValid = true
						break
					}
				}
				if !isValid {
					return fmt.Errorf("invalid backend type: %v, valid types: %s",
						backendType, strings.Join(validTypes, ", "))
				}
			}
		}
	}

	return nil
}

// validateCalicoConfig validates Calico-specific configuration
func (v *DefaultTemplateValidator) validateCalicoConfig(config map[string]interface{}) error {
	// Calico-specific validation logic
	if config == nil {
		return nil // Default configuration is acceptable
	}

	// Check for conflicting configurations
	if ipv4Pool, exists := config["ipv4_pool"]; exists {
		if ipv4Pool == "" {
			return fmt.Errorf("calico ipv4_pool cannot be empty")
		}
	}

	return nil
}

// validateCiliumConfig validates Cilium-specific configuration
func (v *DefaultTemplateValidator) validateCiliumConfig(config map[string]interface{}) error {
	// Cilium-specific validation logic
	if config == nil {
		return nil // Default configuration is acceptable
	}

	// Check for Cilium-specific requirements
	if hubble, exists := config["hubble"]; exists {
		if hubbleMap, ok := hubble.(map[string]interface{}); ok {
			if enabled, exists := hubbleMap["enabled"]; exists {
				if enabledBool, ok := enabled.(bool); ok && enabledBool {
					// Validate Hubble configuration
					if ui, exists := hubbleMap["ui"]; exists {
						if uiMap, ok := ui.(map[string]interface{}); ok {
							if _, exists := uiMap["enabled"]; !exists {
								return fmt.Errorf("hubble ui configuration missing enabled field")
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// validateKubeOVNConfig validates Kube-OVN-specific configuration
func (v *DefaultTemplateValidator) validateKubeOVNConfig(config map[string]interface{}) error {
	// Kube-OVN-specific validation logic
	if config == nil {
		return nil // Default configuration is acceptable
	}

	// Check for Kube-OVN specific requirements
	if subnet, exists := config["default_subnet"]; exists {
		if subnet == "" {
			return fmt.Errorf("kube-ovn default_subnet cannot be empty")
		}
	}

	return nil
}

// validateFlannelConfig validates Flannel-specific configuration
func (v *DefaultTemplateValidator) validateFlannelConfig(config map[string]interface{}) error {
	// Flannel-specific validation logic
	if config == nil {
		return nil // Default configuration is acceptable
	}

	// Check for Flannel specific requirements
	if backend, exists := config["backend"]; exists {
		if backendMap, ok := backend.(map[string]interface{}); ok {
			if backendType, exists := backendMap["type"]; exists {
				validTypes := []string{"vxlan", "host-gw", "udp"}
				isValid := false
				for _, validType := range validTypes {
					if backendType == validType {
						isValid = true
						break
					}
				}
				if !isValid {
					return fmt.Errorf("invalid flannel backend type: %v, valid types: %s",
						backendType, strings.Join(validTypes, ", "))
				}
			}
		}
	}

	return nil
}

// ValidateTemplateSyntax validates template syntax
func (v *DefaultTemplateValidator) ValidateTemplateSyntax(templateName string) error {
	if v.templates == nil {
		return NewInitializationError("template validator", fmt.Errorf("templates not initialized"))
	}

	tmpl := v.templates.Lookup(templateName)
	if tmpl == nil {
		return NewTemplateNotFoundError(templateName, v.getAvailableTemplateNames())
	}

	return v.validateTemplateSyntax(tmpl)
}

// ValidateVariableSubstitution validates variable substitution for a template
func (v *DefaultTemplateValidator) ValidateVariableSubstitution(templateName string, data interface{}) error {
	if v.templates == nil {
		return NewInitializationError("template validator", fmt.Errorf("templates not initialized"))
	}

	tmpl := v.templates.Lookup(templateName)
	if tmpl == nil {
		return NewTemplateNotFoundError(templateName, v.getAvailableTemplateNames())
	}

	variables, err := v.extractTemplateVariables(tmpl)
	if err != nil {
		return NewTemplateError(ErrorTypeValidation, templateName, "failed to extract template variables", err)
	}

	result := &TemplateValidationResult{
		Valid:    true,
		Errors:   []*TemplateError{},
		Warnings: []*TemplateError{},
	}

	if err := v.validateVariableSubstitution(templateName, data, variables, result); err != nil {
		if len(result.Errors) > 0 {
			return result.Errors[0] // Return the first error
		}
		return NewDataValidationError(templateName, "", err)
	}

	return nil
}

// ExtractTemplateVariables extracts variables from a template
func (v *DefaultTemplateValidator) ExtractTemplateVariables(templateName string) ([]VariableInfo, error) {
	if v.templates == nil {
		return nil, NewInitializationError("template validator", fmt.Errorf("templates not initialized"))
	}

	tmpl := v.templates.Lookup(templateName)
	if tmpl == nil {
		return nil, NewTemplateNotFoundError(templateName, v.getAvailableTemplateNames())
	}

	return v.extractTemplateVariables(tmpl)
}

// ValidateTemplateWithMissingVariableDetection validates template and detects missing variables
func (v *DefaultTemplateValidator) ValidateTemplateWithMissingVariableDetection(templateName string, data interface{}) (*TemplateValidationResult, error) {
	result := v.ValidateTemplateWithData(templateName, data)

	// Add additional missing variable detection using regex patterns
	if v.templates != nil {
		tmpl := v.templates.Lookup(templateName)
		if tmpl != nil {
			additionalMissing := v.detectMissingVariablesWithRegex(tmpl, data)
			for _, missing := range additionalMissing {
				found := false
				for _, existing := range result.MissingVariables {
					if existing == missing {
						found = true
						break
					}
				}
				if !found {
					result.MissingVariables = append(result.MissingVariables, missing)
					result.Errors = append(result.Errors, NewDataValidationError(templateName, missing, fmt.Errorf("variable '%s' detected as missing via regex analysis", missing)))
				}
			}
		}
	}

	if len(result.Errors) > 0 {
		result.Valid = false
		return result, fmt.Errorf("template validation failed with %d errors", len(result.Errors))
	}

	return result, nil
}

// detectMissingVariablesWithRegex uses regex to detect potentially missing variables
func (v *DefaultTemplateValidator) detectMissingVariablesWithRegex(tmpl *template.Template, data interface{}) []string {
	var missing []string

	// Get template text (this is a simplified approach)
	templateText := v.getTemplateText(tmpl)
	if templateText == "" {
		return missing
	}

	// Regex patterns to find template variables
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\{\{\s*\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`),
		regexp.MustCompile(`\{\{\s*\$([A-Za-z][A-Za-z0-9_]*)\s*\}\}`),
		regexp.MustCompile(`\{\{\s*range\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`),
		regexp.MustCompile(`\{\{\s*with\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`),
		regexp.MustCompile(`\{\{\s*if\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`),
	}

	foundVars := make(map[string]bool)

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(templateText, -1)
		for _, match := range matches {
			if len(match) > 1 {
				varName := match[1]
				if !foundVars[varName] {
					foundVars[varName] = true
					// Check if this variable exists in data
					if err := v.validateVariableAccess(data, varName); err != nil {
						missing = append(missing, varName)
					}
				}
			}
		}
	}

	return missing
}

// getTemplateText attempts to get the template text for regex analysis
func (v *DefaultTemplateValidator) getTemplateText(tmpl *template.Template) string {
	// This is a simplified approach - in a real implementation,
	// you might need to traverse the parse tree to reconstruct the template text
	if tmpl.Tree != nil && tmpl.Tree.Root != nil {
		// For now, return empty string as we're using parse tree analysis
		// In a full implementation, you could reconstruct the template text from the parse tree
		return ""
	}
	return ""
}

// isFieldEmpty checks if a reflect.Value represents an empty field
func isFieldEmpty(field reflect.Value) bool {
	if !field.IsValid() {
		return true
	}

	switch field.Kind() {
	case reflect.String:
		return field.String() == ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return field.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return field.IsNil()
	case reflect.Bool:
		return !field.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return field.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return field.Float() == 0
	}

	return false
}
