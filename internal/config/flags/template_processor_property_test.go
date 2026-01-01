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
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: cli-configuration-enhancement, Property 5: Template variable resolution
// For any configuration containing template variables, substituting all defined variables should result in a configuration with no remaining unresolved template syntax
// Validates: Requirements 7.1, 7.2, 7.3, 7.4
func TestProperty_TemplateVariableResolution(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("template variables are fully resolved when defined", prop.ForAll(
		func(varName, varValue string) bool {
			// Skip empty variable names
			if varName == "" {
				return true
			}
			
			processor := NewDefaultTemplateProcessor()
			
			// Create a configuration with a template variable
			config := &Configuration{
				Data: map[string]interface{}{
					"test": map[string]interface{}{
						"value": "{{." + varName + "}}",
					},
				},
			}
			
			// Define the variable
			vars := map[string]string{
				varName: varValue,
			}
			
			// Process templates
			err := processor.ProcessTemplates(config, vars)
			if err != nil {
				return false // Template processing should not fail
			}
			
			// Verify the variable was resolved
			if testObj, ok := config.Data["test"].(map[string]interface{}); ok {
				if resolvedValue, ok := testObj["value"].(string); ok {
					// The resolved value should equal the variable value
					return resolvedValue == varValue
				}
			}
			
			return false
		},
		genVariableName(),
		genVariableValue(),
	))

	properties.Property("configurations without templates remain unchanged", prop.ForAll(
		func(configData map[string]interface{}) bool {
			// Skip empty configurations
			if len(configData) == 0 {
				return true
			}
			
			processor := NewDefaultTemplateProcessor()
			
			// Create original configuration
			originalConfig := &Configuration{
				Data: deepCopyMap(configData),
			}
			
			// Create configuration to process
			config := &Configuration{
				Data: deepCopyMap(configData),
			}
			
			// Process templates with empty variables
			err := processor.ProcessTemplates(config, map[string]string{})
			if err != nil {
				return false // Should not fail for configurations without templates
			}
			
			// Configuration should remain unchanged
			return compareTemplateValues(config.Data, originalConfig.Data)
		},
		genConfigWithoutTemplates(),
	))

	properties.Property("multiple template variables are resolved independently", prop.ForAll(
		func(var1Name, var1Value, var2Name, var2Value string) bool {
			// Skip empty variable names or duplicate names
			if var1Name == "" || var2Name == "" || var1Name == var2Name {
				return true
			}
			
			processor := NewDefaultTemplateProcessor()
			
			// Create configuration with multiple template variables
			config := &Configuration{
				Data: map[string]interface{}{
					"config": map[string]interface{}{
						"field1": "{{." + var1Name + "}}",
						"field2": "{{." + var2Name + "}}",
					},
				},
			}
			
			// Define both variables
			vars := map[string]string{
				var1Name: var1Value,
				var2Name: var2Value,
			}
			
			// Process templates
			err := processor.ProcessTemplates(config, vars)
			if err != nil {
				return false
			}
			
			// Verify both variables were resolved correctly
			if configObj, ok := config.Data["config"].(map[string]interface{}); ok {
				field1, ok1 := configObj["field1"].(string)
				field2, ok2 := configObj["field2"].(string)
				
				if ok1 && ok2 {
					return field1 == var1Value && field2 == var2Value
				}
			}
			
			return false
		},
		genVariableName(),
		genVariableValue(),
		genVariableName(),
		genVariableValue(),
	))

	properties.Property("nested template variables are resolved recursively", prop.ForAll(
		func(varName, varValue string) bool {
			// Skip empty variable names
			if varName == "" {
				return true
			}
			
			processor := NewDefaultTemplateProcessor()
			
			// Create configuration with nested template variables
			config := &Configuration{
				Data: map[string]interface{}{
					"level1": map[string]interface{}{
						"level2": map[string]interface{}{
							"level3": []interface{}{
								"{{." + varName + "}}",
								"static-value",
							},
						},
					},
				},
			}
			
			// Define the variable
			vars := map[string]string{
				varName: varValue,
			}
			
			// Process templates
			err := processor.ProcessTemplates(config, vars)
			if err != nil {
				return false
			}
			
			// Verify nested variable was resolved
			if level1, ok := config.Data["level1"].(map[string]interface{}); ok {
				if level2, ok := level1["level2"].(map[string]interface{}); ok {
					if level3, ok := level2["level3"].([]interface{}); ok {
						if len(level3) >= 1 {
							if resolvedValue, ok := level3[0].(string); ok {
								return resolvedValue == varValue
							}
						}
					}
				}
			}
			
			return false
		},
		genVariableName(),
		genVariableValue(),
	))

	properties.Property("template processing is idempotent", prop.ForAll(
		func(varName, varValue string) bool {
			// Skip empty variable names
			if varName == "" {
				return true
			}
			
			processor := NewDefaultTemplateProcessor()
			
			// Create configuration with template variable
			config := &Configuration{
				Data: map[string]interface{}{
					"value": "{{." + varName + "}}",
				},
			}
			
			// Define the variable
			vars := map[string]string{
				varName: varValue,
			}
			
			// Process templates first time
			err1 := processor.ProcessTemplates(config, vars)
			if err1 != nil {
				return false
			}
			
			// Save the result
			firstResult := deepCopyMap(config.Data)
			
			// Process templates second time
			err2 := processor.ProcessTemplates(config, vars)
			if err2 != nil {
				return false
			}
			
			// Results should be identical (idempotent)
			return compareTemplateValues(config.Data, firstResult)
		},
		genVariableName(),
		genVariableValue(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Generators for property-based testing

func genVariableName() gopter.Gen {
	return gen.OneConstOf(
		"CLUSTER_NAME",
		"REGION",
		"ENV",
		"VERSION",
		"COUNT",
		"PROVIDER",
		"SIZE",
		"NAMESPACE",
	)
}

func genVariableValue() gopter.Gen {
	return gen.OneConstOf(
		"test-value",
		"prod-cluster",
		"us-west-1",
		"development",
		"1.28",
		"3",
		"openstack",
		"large",
		"default",
	)
}

func genConfigWithoutTemplates() gopter.Gen {
	return gen.OneConstOf(
		map[string]interface{}{
			"cluster": map[string]interface{}{
				"name": "static-cluster",
			},
		},
		map[string]interface{}{
			"config": map[string]interface{}{
				"enabled": true,
				"count":   5,
			},
		},
		map[string]interface{}{
			"array": []interface{}{"item1", "item2", "item3"},
		},
		map[string]interface{}{
			"nested": map[string]interface{}{
				"deep": map[string]interface{}{
					"value": "no-templates-here",
				},
			},
		},
	)
}

// Helper functions

func deepCopyMap(original map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{})
	for key, value := range original {
		copy[key] = deepCopyValue(value)
	}
	return copy
}

func deepCopyValue(original interface{}) interface{} {
	switch v := original.(type) {
	case map[string]interface{}:
		return deepCopyMap(v)
	case []interface{}:
		copy := make([]interface{}, len(v))
		for i, item := range v {
			copy[i] = deepCopyValue(item)
		}
		return copy
	default:
		return v
	}
}

func containsTemplateVariables(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(v, "{{") && strings.Contains(v, "}}")
	case map[string]interface{}:
		for key, val := range v {
			if containsTemplateVariables(key) || containsTemplateVariables(val) {
				return true
			}
		}
	case []interface{}:
		for _, val := range v {
			if containsTemplateVariables(val) {
				return true
			}
		}
	}
	return false
}