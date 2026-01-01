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
)

func TestDefaultTemplateProcessor_ProcessTemplates(t *testing.T) {
	processor := NewDefaultTemplateProcessor()
	
	tests := []struct {
		name      string
		config    *Configuration
		vars      map[string]string
		expected  map[string]interface{}
		expectErr bool
	}{
		{
			name: "simple variable substitution",
			config: &Configuration{
				Data: map[string]interface{}{
					"cluster": map[string]interface{}{
						"name": "{{.CLUSTER_NAME}}",
					},
				},
			},
			vars: map[string]string{
				"CLUSTER_NAME": "test-cluster",
			},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "test-cluster",
				},
			},
		},
		{
			name: "multiple variables",
			config: &Configuration{
				Data: map[string]interface{}{
					"cluster": map[string]interface{}{
						"name":    "{{.CLUSTER_NAME}}",
						"version": "{{.K8S_VERSION}}",
					},
				},
			},
			vars: map[string]string{
				"CLUSTER_NAME": "prod-cluster",
				"K8S_VERSION":  "1.28",
			},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name":    "prod-cluster",
					"version": "1.28",
				},
			},
		},
		{
			name: "nested object templates",
			config: &Configuration{
				Data: map[string]interface{}{
					"infrastructure": map[string]interface{}{
						"provider": "{{.PROVIDER}}",
						"region":   "{{.REGION}}",
						"servers": map[string]interface{}{
							"count": "{{.SERVER_COUNT}}",
						},
					},
				},
			},
			vars: map[string]string{
				"PROVIDER":     "openstack",
				"REGION":       "us-west-1",
				"SERVER_COUNT": "3",
			},
			expected: map[string]interface{}{
				"infrastructure": map[string]interface{}{
					"provider": "openstack",
					"region":   "us-west-1",
					"servers": map[string]interface{}{
						"count": "3",
					},
				},
			},
		},
		{
			name: "array templates",
			config: &Configuration{
				Data: map[string]interface{}{
					"dns": []interface{}{
						"{{.DNS1}}",
						"{{.DNS2}}",
					},
				},
			},
			vars: map[string]string{
				"DNS1": "8.8.8.8",
				"DNS2": "8.8.4.4",
			},
			expected: map[string]interface{}{
				"dns": []interface{}{
					"8.8.8.8",
					"8.8.4.4",
				},
			},
		},
		{
			name: "no templates",
			config: &Configuration{
				Data: map[string]interface{}{
					"cluster": map[string]interface{}{
						"name": "static-cluster",
					},
				},
			},
			vars: map[string]string{},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "static-cluster",
				},
			},
		},
		{
			name: "undefined variable renders as no value",
			config: &Configuration{
				Data: map[string]interface{}{
					"cluster": map[string]interface{}{
						"name": "{{.UNDEFINED_VAR}}",
					},
				},
			},
			vars: map[string]string{},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "<no value>",
				},
			},
		},
		{
			name:      "nil config",
			config:    nil,
			vars:      map[string]string{},
			expectErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ProcessTemplates(tt.config, tt.vars)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("ProcessTemplates() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("ProcessTemplates() unexpected error: %v", err)
				return
			}
			
			if !compareTemplateValues(tt.config.Data, tt.expected) {
				t.Errorf("ProcessTemplates() result = %v, want %v", tt.config.Data, tt.expected)
			}
		})
	}
}

func TestDefaultTemplateProcessor_RegisterFunction(t *testing.T) {
	processor := NewDefaultTemplateProcessor()
	
	tests := []struct {
		name      string
		funcName  string
		function  interface{}
		expectErr bool
	}{
		{
			name:     "valid function",
			funcName: "customFunc",
			function: func(s string) string { return strings.ToUpper(s) },
		},
		{
			name:      "empty name",
			funcName:  "",
			function:  func() string { return "test" },
			expectErr: true,
		},
		{
			name:      "nil function",
			funcName:  "nilFunc",
			function:  nil,
			expectErr: true,
		},
		{
			name:      "non-function",
			funcName:  "notFunc",
			function:  "not a function",
			expectErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.RegisterFunction(tt.funcName, tt.function)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("RegisterFunction() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("RegisterFunction() unexpected error: %v", err)
				return
			}
		})
	}
}

func TestDefaultTemplateProcessor_BuiltinFunctions(t *testing.T) {
	processor := NewDefaultTemplateProcessor()
	
	tests := []struct {
		name     string
		config   *Configuration
		vars     map[string]string
		expected map[string]interface{}
	}{
		{
			name: "upper function",
			config: &Configuration{
				Data: map[string]interface{}{
					"name": "{{.NAME | upper}}",
				},
			},
			vars: map[string]string{
				"NAME": "test-cluster",
			},
			expected: map[string]interface{}{
				"name": "TEST-CLUSTER",
			},
		},
		{
			name: "lower function",
			config: &Configuration{
				Data: map[string]interface{}{
					"name": "{{.NAME | lower}}",
				},
			},
			vars: map[string]string{
				"NAME": "TEST-CLUSTER",
			},
			expected: map[string]interface{}{
				"name": "test-cluster",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ProcessTemplates(tt.config, tt.vars)
			if err != nil {
				t.Errorf("ProcessTemplates() unexpected error: %v", err)
				return
			}
			
			if !compareTemplateValues(tt.config.Data, tt.expected) {
				t.Errorf("ProcessTemplates() result = %v, want %v", tt.config.Data, tt.expected)
			}
		})
	}
}

func TestDefaultTemplateProcessor_ValidateTemplates(t *testing.T) {
	processor := NewDefaultTemplateProcessor()
	
	tests := []struct {
		name      string
		config    *Configuration
		expectErr bool
		minVars   int // Minimum number of variables expected
	}{
		{
			name: "config with templates",
			config: &Configuration{
				Data: map[string]interface{}{
					"cluster": map[string]interface{}{
						"name":    "{{.CLUSTER_NAME}}",
						"version": "{{.K8S_VERSION}}",
					},
				},
			},
			minVars: 2,
		},
		{
			name: "config without templates",
			config: &Configuration{
				Data: map[string]interface{}{
					"cluster": map[string]interface{}{
						"name": "static-cluster",
					},
				},
			},
			minVars: 0,
		},
		{
			name:      "nil config",
			config:    nil,
			expectErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars, err := processor.ValidateTemplates(tt.config)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("ValidateTemplates() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("ValidateTemplates() unexpected error: %v", err)
				return
			}
			
			if len(vars) < tt.minVars {
				t.Errorf("ValidateTemplates() found %d variables, expected at least %d", len(vars), tt.minVars)
			}
		})
	}
}

func TestTemplateFlagHandler_ParseFlag(t *testing.T) {
	handler := NewTemplateFlagHandler()
	
	tests := []struct {
		name      string
		flagName  string
		value     string
		expected  *TemplateVariable
		expectErr bool
	}{
		{
			name:     "valid template variable",
			flagName: "template-var-CLUSTER_NAME",
			value:    "test-cluster",
			expected: &TemplateVariable{
				Name:  "CLUSTER_NAME",
				Value: "test-cluster",
			},
		},
		{
			name:     "template variable with complex value",
			flagName: "template-var-REGION",
			value:    "us-west-1",
			expected: &TemplateVariable{
				Name:  "REGION",
				Value: "us-west-1",
			},
		},
		{
			name:      "empty value",
			flagName:  "template-var-TEST",
			value:     "",
			expectErr: true,
		},
		{
			name:      "invalid flag format",
			flagName:  "invalid-flag",
			value:     "value",
			expectErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.ParseFlag(tt.flagName, tt.value)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("ParseFlag() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("ParseFlag() unexpected error: %v", err)
				return
			}
			
			templateVar, ok := result.(*TemplateVariable)
			if !ok {
				t.Errorf("ParseFlag() returned wrong type: %T", result)
				return
			}
			
			if templateVar.Name != tt.expected.Name {
				t.Errorf("ParseFlag() name = %v, want %v", templateVar.Name, tt.expected.Name)
			}
			
			if templateVar.Value != tt.expected.Value {
				t.Errorf("ParseFlag() value = %v, want %v", templateVar.Value, tt.expected.Value)
			}
		})
	}
}

// Helper function to compare template values
func compareTemplateValues(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	
	for key, aVal := range a {
		bVal, exists := b[key]
		if !exists {
			return false
		}
		
		if !compareTemplateValue(aVal, bVal) {
			return false
		}
	}
	
	return true
}

func compareTemplateValue(a, b interface{}) bool {
	switch aVal := a.(type) {
	case map[string]interface{}:
		if bMap, ok := b.(map[string]interface{}); ok {
			return compareTemplateValues(aVal, bMap)
		}
		return false
	case []interface{}:
		if bArray, ok := b.([]interface{}); ok {
			if len(aVal) != len(bArray) {
				return false
			}
			for i, aItem := range aVal {
				if !compareTemplateValue(aItem, bArray[i]) {
					return false
				}
			}
			return true
		}
		return false
	default:
		return a == b
	}
}