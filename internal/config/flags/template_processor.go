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
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"
)

// TemplateProcessor handles template variable substitution
type TemplateProcessor interface {
	// ProcessTemplates substitutes variables in configuration
	ProcessTemplates(config *Configuration, vars map[string]string) error
	
	// ValidateTemplates checks for undefined variables
	ValidateTemplates(config *Configuration) ([]string, error)
	
	// RegisterFunction adds custom template functions
	RegisterFunction(name string, fn interface{}) error
}

// TemplateContext provides context for template processing
type TemplateContext struct {
	Variables map[string]string      `json:"variables"`
	Functions map[string]interface{} `json:"functions"`
	Config    *Configuration         `json:"config"`
}

// DefaultTemplateProcessor implements the TemplateProcessor interface
type DefaultTemplateProcessor struct {
	functions map[string]interface{}
}

// NewDefaultTemplateProcessor creates a new template processor with built-in functions
func NewDefaultTemplateProcessor() *DefaultTemplateProcessor {
	processor := &DefaultTemplateProcessor{
		functions: make(map[string]interface{}),
	}
	
	// Register built-in template functions
	processor.registerBuiltinFunctions()
	
	return processor
}

// RegisterFunction adds custom template functions
func (p *DefaultTemplateProcessor) RegisterFunction(name string, fn interface{}) error {
	if name == "" {
		return fmt.Errorf("function name cannot be empty")
	}
	
	if fn == nil {
		return fmt.Errorf("function cannot be nil")
	}
	
	// Validate that fn is actually a function
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("provided value is not a function: %T", fn)
	}
	
	p.functions[name] = fn
	return nil
}

// ProcessTemplates substitutes variables in configuration
func (p *DefaultTemplateProcessor) ProcessTemplates(config *Configuration, vars map[string]string) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	
	if vars == nil {
		vars = make(map[string]string)
	}
	
	// Create template context
	context := TemplateContext{
		Variables: vars,
		Functions: p.functions,
		Config:    config,
	}
	
	// Process the configuration data recursively
	processedData, err := p.processValue(config.Data, context)
	if err != nil {
		return fmt.Errorf("failed to process configuration templates: %w", err)
	}
	
	// Update the configuration with processed data
	if processedMap, ok := processedData.(map[string]interface{}); ok {
		config.Data = processedMap
	} else {
		return fmt.Errorf("processed configuration is not a map: %T", processedData)
	}
	
	return nil
}

// ValidateTemplates checks for undefined variables
func (p *DefaultTemplateProcessor) ValidateTemplates(config *Configuration) ([]string, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	
	var undefinedVars []string
	
	// Find all template variables in the configuration
	templateVars, err := p.findTemplateVariables(config.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to find template variables: %w", err)
	}
	
	// Check which variables are undefined (this would require actual variable context)
	// For now, we'll return all found variables as potentially undefined
	undefinedVars = append(undefinedVars, templateVars...)
	
	return undefinedVars, nil
}

// processValue processes a single value, handling templates recursively
func (p *DefaultTemplateProcessor) processValue(value interface{}, context TemplateContext) (interface{}, error) {
	switch v := value.(type) {
	case string:
		// Process string templates
		return p.processStringTemplate(v, context)
	case map[string]interface{}:
		// Process object recursively
		result := make(map[string]interface{})
		for key, val := range v {
			processedKey, err := p.processStringTemplate(key, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in key '%s': %w", key, err)
			}
			
			processedVal, err := p.processValue(val, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in value for key '%s': %w", key, err)
			}
			
			if keyStr, ok := processedKey.(string); ok {
				result[keyStr] = processedVal
			} else {
				return nil, fmt.Errorf("processed key is not a string: %T", processedKey)
			}
		}
		return result, nil
	case []interface{}:
		// Process array recursively
		result := make([]interface{}, len(v))
		for i, val := range v {
			processedVal, err := p.processValue(val, context)
			if err != nil {
				return nil, fmt.Errorf("failed to process template in array element %d: %w", i, err)
			}
			result[i] = processedVal
		}
		return result, nil
	default:
		// Non-string values are returned as-is
		return value, nil
	}
}

// processStringTemplate processes template variables in a string
func (p *DefaultTemplateProcessor) processStringTemplate(str string, context TemplateContext) (interface{}, error) {
	// Check if the string contains template syntax
	if !strings.Contains(str, "{{") || !strings.Contains(str, "}}") {
		return str, nil
	}
	
	// Create a new template
	tmpl := template.New("config")
	
	// Add custom functions
	tmpl = tmpl.Funcs(p.functions)
	
	// Parse the template
	tmpl, err := tmpl.Parse(str)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template '%s': %w", str, err)
	}
	
	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context.Variables); err != nil {
		return nil, fmt.Errorf("failed to execute template '%s': %w", str, err)
	}
	
	result := buf.String()
	
	// Try to convert the result to appropriate type
	return p.convertTemplateResult(result), nil
}

// convertTemplateResult attempts to convert template result to appropriate Go type
func (p *DefaultTemplateProcessor) convertTemplateResult(result string) interface{} {
	// For now, just return as string
	// In a more sophisticated implementation, we could try to parse as JSON, numbers, booleans, etc.
	return result
}

// findTemplateVariables finds all template variables in a configuration
func (p *DefaultTemplateProcessor) findTemplateVariables(value interface{}) ([]string, error) {
	var variables []string
	
	switch v := value.(type) {
	case string:
		vars := p.extractVariablesFromString(v)
		variables = append(variables, vars...)
	case map[string]interface{}:
		for key, val := range v {
			// Check key for templates
			keyVars := p.extractVariablesFromString(key)
			variables = append(variables, keyVars...)
			
			// Check value recursively
			valVars, err := p.findTemplateVariables(val)
			if err != nil {
				return nil, err
			}
			variables = append(variables, valVars...)
		}
	case []interface{}:
		for _, val := range v {
			valVars, err := p.findTemplateVariables(val)
			if err != nil {
				return nil, err
			}
			variables = append(variables, valVars...)
		}
	}
	
	// Remove duplicates
	return p.removeDuplicates(variables), nil
}

// extractVariablesFromString extracts template variables from a string
func (p *DefaultTemplateProcessor) extractVariablesFromString(str string) []string {
	var variables []string
	
	// Simple regex-like extraction of {{.VAR}} patterns
	start := 0
	for {
		openIndex := strings.Index(str[start:], "{{")
		if openIndex == -1 {
			break
		}
		openIndex += start
		
		closeIndex := strings.Index(str[openIndex:], "}}")
		if closeIndex == -1 {
			break
		}
		closeIndex += openIndex
		
		// Extract the variable name
		varExpr := str[openIndex+2 : closeIndex]
		varExpr = strings.TrimSpace(varExpr)
		
		// Simple extraction of .VAR pattern
		if strings.HasPrefix(varExpr, ".") {
			varName := strings.TrimPrefix(varExpr, ".")
			if varName != "" {
				variables = append(variables, varName)
			}
		}
		
		start = closeIndex + 2
	}
	
	return variables
}

// removeDuplicates removes duplicate strings from a slice
func (p *DefaultTemplateProcessor) removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// registerBuiltinFunctions registers built-in template functions
func (p *DefaultTemplateProcessor) registerBuiltinFunctions() {
	// String manipulation functions
	p.functions["upper"] = strings.ToUpper
	p.functions["lower"] = strings.ToLower
	p.functions["title"] = strings.Title
	p.functions["trim"] = strings.TrimSpace
	
	// String replacement
	p.functions["replace"] = func(old, new, str string) string {
		return strings.ReplaceAll(str, old, new)
	}
	
	// String joining
	p.functions["join"] = func(sep string, elems []string) string {
		return strings.Join(elems, sep)
	}
	
	// Default value function
	p.functions["default"] = func(defaultVal, val interface{}) interface{} {
		if val == nil || val == "" {
			return defaultVal
		}
		return val
	}
	
	// Environment-like functions (placeholder implementations)
	p.functions["env"] = func(key string) string {
		// In a real implementation, this would read from environment variables
		return fmt.Sprintf("${%s}", key)
	}
}