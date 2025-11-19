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
	"testing"
	"text/template"
)

// TestTemplateValidationFrameworkDemo demonstrates the validation framework
func TestTemplateValidationFrameworkDemo(t *testing.T) {
	// This test demonstrates the validation framework capabilities
	engine, err := CreateTemplateEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Create a template that requires specific variables
	templateText := "Hello {{.User.Name}}, your cluster {{.ClusterName}} is {{.Status}}"
	tmpl, err := template.New("demo").Parse(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	err = engine.InitWithTemplates(tmpl)
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}

	// Debug: check available templates
	templates := engine.ListTemplates()
	t.Logf("Available templates: %v", templates)

	// Test 1: Complete data should pass validation
	completeData := map[string]interface{}{
		"User": map[string]interface{}{
			"Name": "John",
		},
		"ClusterName": "production",
		"Status":      "ready",
	}

	result := engine.ValidateTemplateWithData("demo", completeData)
	if !result.Valid {
		t.Errorf("Expected validation to pass with complete data, got errors: %v", result.Errors)
	}

	// Test 2: Missing data should fail validation
	incompleteData := map[string]interface{}{
		"User": map[string]interface{}{
			"Name": "John",
		},
		// Missing ClusterName and Status
	}

	result = engine.ValidateTemplateWithData("demo", incompleteData)
	if result.Valid {
		t.Error("Expected validation to fail with incomplete data")
	}

	if len(result.MissingVariables) == 0 {
		t.Error("Expected missing variables to be detected")
	}

	// Test 3: Variable extraction should work
	variables, err := engine.ExtractTemplateVariables("demo")
	if err != nil {
		t.Fatalf("Failed to extract variables: %v", err)
	}

	expectedVars := []string{"User.Name", "ClusterName", "Status"}
	if len(variables) != len(expectedVars) {
		t.Errorf("Expected %d variables, got %d", len(expectedVars), len(variables))
	}

	// Test 4: Syntax validation should work
	err = engine.ValidateTemplateSyntax("demo")
	if err != nil {
		t.Errorf("Expected syntax validation to pass: %v", err)
	}

	// Test 5: Variable substitution validation should work
	err = engine.ValidateVariableSubstitution("demo", completeData)
	if err != nil {
		t.Errorf("Expected variable substitution validation to pass: %v", err)
	}

	err = engine.ValidateVariableSubstitution("demo", incompleteData)
	if err == nil {
		t.Error("Expected variable substitution validation to fail with incomplete data")
	}

	t.Logf("Template validation framework demo completed successfully")
}

// TestNetworkPluginValidationDemo demonstrates network plugin validation
func TestNetworkPluginValidationDemo(t *testing.T) {
	engine, err := CreateTemplateEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Test various network plugin configurations
	testCases := []struct {
		name       string
		pluginType string
		config     map[string]interface{}
		expectErr  bool
	}{
		{
			name:       "valid calico",
			pluginType: "calico",
			config: map[string]interface{}{
				"ipv4_pool": "10.244.0.0/16",
				"mtu":       1440,
			},
			expectErr: false,
		},
		{
			name:       "invalid calico CIDR",
			pluginType: "calico",
			config: map[string]interface{}{
				"ipv4_pool": "invalid-cidr",
			},
			expectErr: true,
		},
		{
			name:       "valid cilium",
			pluginType: "cilium",
			config: map[string]interface{}{
				"cluster_pool_ipv4_cidr":      "10.0.0.0/8",
				"cluster_pool_ipv4_mask_size": 24,
			},
			expectErr: false,
		},
		{
			name:       "invalid cilium mask size",
			pluginType: "cilium",
			config: map[string]interface{}{
				"cluster_pool_ipv4_mask_size": 5, // Too small
			},
			expectErr: true,
		},
		{
			name:       "unsupported plugin",
			pluginType: "unsupported",
			config:     map[string]interface{}{},
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.ValidateNetworkPluginConfig(tc.pluginType, tc.config)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tc.name)
				} else {
					t.Logf("Expected error caught: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got: %v", tc.name, err)
				} else {
					t.Logf("Validation passed for %s", tc.name)
				}
			}
		})
	}
}
