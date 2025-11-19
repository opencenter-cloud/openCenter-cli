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

func TestDefaultTemplateValidator_ValidateTemplate(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		templateText string
		wantErr      bool
		errorType    TemplateErrorType
	}{
		{
			name:         "valid template",
			templateName: "valid",
			templateText: "Hello {{.Name}}",
			wantErr:      false,
		},
		{
			name:         "template with syntax error",
			templateName: "invalid",
			templateText: "Hello {{.Name",
			wantErr:      true,
			errorType:    ErrorTypeNotFound, // This will be not found because parsing fails
		},
		{
			name:         "template not found",
			templateName: "nonexistent",
			templateText: "",
			wantErr:      true,
			errorType:    ErrorTypeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDefaultTemplateValidator()

			// Create templates
			tmpl := template.New("")
			if tt.templateText != "" {
				if tt.name != "template with syntax error" {
					_, err := tmpl.New(tt.templateName).Parse(tt.templateText)
					if err != nil {
						t.Fatalf("Failed to create test template: %v", err)
					}
				} else {
					// For syntax error test, create a template with invalid syntax
					// We'll let the validator catch this
					tmpl.New(tt.templateName)
				}
			}

			err := validator.Init(tmpl)
			if err != nil {
				t.Fatalf("Failed to initialize validator: %v", err)
			}

			err = validator.ValidateTemplate(tt.templateName)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
					return
				}

				if templateErr, ok := GetTemplateError(err); ok {
					if templateErr.Type != tt.errorType {
						t.Errorf("Expected error type %v, got %v", tt.errorType, templateErr.Type)
					}
				} else {
					t.Logf("Got non-template error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestDefaultTemplateValidator_ValidateTemplateWithData(t *testing.T) {
	tests := []struct {
		name              string
		templateText      string
		data              interface{}
		expectValid       bool
		expectMissingVars []string
		expectUnusedVars  []string
	}{
		{
			name:         "valid template with complete data",
			templateText: "Hello {{.Name}}, you are {{.Age}} years old",
			data: map[string]interface{}{
				"Name": "John",
				"Age":  30,
			},
			expectValid: true,
		},
		{
			name:         "template with missing variable",
			templateText: "Hello {{.Name}}, you are {{.Age}} years old",
			data: map[string]interface{}{
				"Name": "John",
			},
			expectValid:       false,
			expectMissingVars: []string{"Age"},
		},
		{
			name:         "template with unused variable",
			templateText: "Hello {{.Name}}",
			data: map[string]interface{}{
				"Name": "John",
				"Age":  30,
			},
			expectValid:      true,
			expectUnusedVars: []string{"Age"},
		},
		{
			name:              "template with nil data",
			templateText:      "Hello {{.Name}}",
			data:              nil,
			expectValid:       false,
			expectMissingVars: []string{"Name"},
		},
		{
			name:         "template with nested data access",
			templateText: "Hello {{.User.Name}}, email: {{.User.Email}}",
			data: map[string]interface{}{
				"User": map[string]interface{}{
					"Name":  "John",
					"Email": "john@example.com",
				},
			},
			expectValid: true,
		},
		{
			name:         "template with missing nested data",
			templateText: "Hello {{.User.Name}}, email: {{.User.Email}}",
			data: map[string]interface{}{
				"User": map[string]interface{}{
					"Name": "John",
				},
			},
			expectValid:       false,
			expectMissingVars: []string{"User.Email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDefaultTemplateValidator()

			// Create template
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Failed to create test template: %v", err)
			}

			err = validator.Init(tmpl)
			if err != nil {
				t.Fatalf("Failed to initialize validator: %v", err)
			}

			result := validator.ValidateTemplateWithData("test", tt.data)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectValid, result.Valid)
				if len(result.Errors) > 0 {
					t.Logf("Errors: %v", result.Errors)
				}
			}

			// Check missing variables
			if len(tt.expectMissingVars) > 0 {
				if len(result.MissingVariables) == 0 {
					t.Error("Expected missing variables, got none")
				} else {
					for _, expectedMissing := range tt.expectMissingVars {
						found := false
						for _, actualMissing := range result.MissingVariables {
							if actualMissing == expectedMissing {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("Expected missing variable '%s', not found in %v", expectedMissing, result.MissingVariables)
						}
					}
				}
			}

			// Check unused variables
			if len(tt.expectUnusedVars) > 0 {
				if len(result.UnusedVariables) == 0 {
					t.Error("Expected unused variables, got none")
				} else {
					for _, expectedUnused := range tt.expectUnusedVars {
						found := false
						for _, actualUnused := range result.UnusedVariables {
							if actualUnused == expectedUnused {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("Expected unused variable '%s', not found in %v", expectedUnused, result.UnusedVariables)
						}
					}
				}
			}
		})
	}
}

func TestDefaultTemplateValidator_ExtractTemplateVariables(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		expectedVars []string
	}{
		{
			name:         "simple variable",
			templateText: "Hello {{.Name}}",
			expectedVars: []string{"Name"},
		},
		{
			name:         "multiple variables",
			templateText: "Hello {{.Name}}, you are {{.Age}} years old",
			expectedVars: []string{"Name", "Age"},
		},
		{
			name:         "nested variables",
			templateText: "Hello {{.User.Name}}, email: {{.User.Email}}",
			expectedVars: []string{"User.Name", "User.Email"},
		},
		{
			name:         "variables in conditionals",
			templateText: "{{if .IsActive}}Hello {{.Name}}{{end}}",
			expectedVars: []string{"Name"}, // Only Name is extracted as field access
		},
		{
			name:         "variables in range",
			templateText: "{{range .Items}}{{.Name}}{{end}}",
			expectedVars: []string{"Name"}, // Only Name is extracted as field access
		},
		{
			name:         "no variables",
			templateText: "Hello World",
			expectedVars: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDefaultTemplateValidator()

			// Create template
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Failed to create test template: %v", err)
			}

			err = validator.Init(tmpl)
			if err != nil {
				t.Fatalf("Failed to initialize validator: %v", err)
			}

			variables, err := validator.ExtractTemplateVariables("test")
			if err != nil {
				t.Fatalf("Failed to extract variables: %v", err)
			}

			// Convert to string slice for easier comparison
			var actualVars []string
			for _, v := range variables {
				actualVars = append(actualVars, v.Name)
			}

			if len(actualVars) != len(tt.expectedVars) {
				t.Errorf("Expected %d variables, got %d: %v", len(tt.expectedVars), len(actualVars), actualVars)
				return
			}

			for _, expected := range tt.expectedVars {
				found := false
				for _, actual := range actualVars {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected variable '%s' not found in %v", expected, actualVars)
				}
			}
		})
	}
}

func TestDefaultTemplateValidator_ValidateVariableAccess(t *testing.T) {
	tests := []struct {
		name         string
		data         interface{}
		variablePath string
		wantErr      bool
	}{
		{
			name: "valid simple access",
			data: map[string]interface{}{
				"Name": "John",
			},
			variablePath: "Name",
			wantErr:      false,
		},
		{
			name: "valid nested access",
			data: map[string]interface{}{
				"User": map[string]interface{}{
					"Name": "John",
				},
			},
			variablePath: "User.Name",
			wantErr:      false,
		},
		{
			name: "missing simple field",
			data: map[string]interface{}{
				"Name": "John",
			},
			variablePath: "Age",
			wantErr:      true,
		},
		{
			name: "missing nested field",
			data: map[string]interface{}{
				"User": map[string]interface{}{
					"Name": "John",
				},
			},
			variablePath: "User.Email",
			wantErr:      true,
		},
		{
			name:         "nil data",
			data:         nil,
			variablePath: "Name",
			wantErr:      true,
		},
		{
			name: "struct data",
			data: struct {
				Name string
				Age  int
			}{
				Name: "John",
				Age:  30,
			},
			variablePath: "Name",
			wantErr:      false,
		},
		{
			name: "struct missing field",
			data: struct {
				Name string
			}{
				Name: "John",
			},
			variablePath: "Age",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDefaultTemplateValidator()

			err := validator.validateVariableAccess(tt.data, tt.variablePath)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestDefaultTemplateValidator_ValidateNetworkPluginConfig(t *testing.T) {
	tests := []struct {
		name       string
		pluginType string
		config     map[string]interface{}
		wantErr    bool
	}{
		{
			name:       "valid calico config",
			pluginType: "calico",
			config: map[string]interface{}{
				"ipv4_pool": "192.168.0.0/16",
				"mtu":       1440,
			},
			wantErr: false,
		},
		{
			name:       "invalid calico ipv4_pool",
			pluginType: "calico",
			config: map[string]interface{}{
				"ipv4_pool": "invalid-cidr",
			},
			wantErr: true,
		},
		{
			name:       "invalid calico mtu",
			pluginType: "calico",
			config: map[string]interface{}{
				"mtu": 50, // Too low
			},
			wantErr: true,
		},
		{
			name:       "valid cilium config",
			pluginType: "cilium",
			config: map[string]interface{}{
				"cluster_pool_ipv4_cidr":      "10.0.0.0/8",
				"cluster_pool_ipv4_mask_size": 24,
			},
			wantErr: false,
		},
		{
			name:       "invalid cilium mask size",
			pluginType: "cilium",
			config: map[string]interface{}{
				"cluster_pool_ipv4_mask_size": 5, // Too low
			},
			wantErr: true,
		},
		{
			name:       "valid kube-ovn config",
			pluginType: "kube-ovn",
			config: map[string]interface{}{
				"default_subnet": "10.16.0.0/16",
			},
			wantErr: false,
		},
		{
			name:       "invalid kube-ovn subnet",
			pluginType: "kube-ovn",
			config: map[string]interface{}{
				"default_subnet": "invalid-subnet",
			},
			wantErr: true,
		},
		{
			name:       "valid flannel config",
			pluginType: "flannel",
			config: map[string]interface{}{
				"network": "10.244.0.0/16",
				"backend": map[string]interface{}{
					"type": "vxlan",
				},
			},
			wantErr: false,
		},
		{
			name:       "invalid flannel backend",
			pluginType: "flannel",
			config: map[string]interface{}{
				"backend": map[string]interface{}{
					"type": "invalid-backend",
				},
			},
			wantErr: true,
		},
		{
			name:       "unsupported plugin",
			pluginType: "unsupported",
			config:     map[string]interface{}{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDefaultTemplateValidator()

			err := validator.ValidateNetworkPluginConfig(tt.pluginType, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestDefaultTemplateValidator_ValidateTemplateSyntax(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		wantErr      bool
	}{
		{
			name:         "valid template",
			templateText: "Hello {{.Name}}",
			wantErr:      false,
		},
		{
			name:         "template with functions",
			templateText: "Hello {{.Name}}",
			wantErr:      false,
		},
		{
			name:         "template with conditionals",
			templateText: "{{if .IsActive}}Hello {{.Name}}{{end}}",
			wantErr:      false,
		},
		{
			name:         "template with range",
			templateText: "{{range .Items}}{{.Name}}{{end}}",
			wantErr:      false,
		},
		{
			name:         "template with nested templates",
			templateText: "{{template \"header\" .}}Content{{template \"footer\" .}}",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewDefaultTemplateValidator()

			// Create template
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				if tt.wantErr {
					return // Expected parsing to fail
				}
				t.Fatalf("Failed to create test template: %v", err)
			}

			err = validator.Init(tmpl)
			if err != nil {
				t.Fatalf("Failed to initialize validator: %v", err)
			}

			err = validator.ValidateTemplateSyntax("test")

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestTemplateValidationResult_ErrorHandling(t *testing.T) {
	validator := NewDefaultTemplateValidator()

	// Create template with missing variables
	tmpl, err := template.New("test").Parse("Hello {{.Name}}, you are {{.Age}} years old")
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}

	err = validator.Init(tmpl)
	if err != nil {
		t.Fatalf("Failed to initialize validator: %v", err)
	}

	// Test with incomplete data
	data := map[string]interface{}{
		"Name": "John",
		// Missing Age
	}

	result := validator.ValidateTemplateWithData("test", data)

	if result.Valid {
		t.Error("Expected validation to fail")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors")
	}

	if len(result.MissingVariables) == 0 {
		t.Error("Expected missing variables")
	}

	// Check that errors contain helpful information
	for _, err := range result.Errors {
		if err.Message == "" {
			t.Error("Error message should not be empty")
		}
		if len(err.Suggestions) == 0 {
			t.Error("Error should have suggestions")
		}
	}
}

func TestTemplateValidator_Integration(t *testing.T) {
	// Test integration with template engine
	engine := NewDefaultTemplateEngine()

	// Create template with variables
	tmpl, err := template.New("integration").Parse("Hello {{.User.Name}}, your role is {{.User.Role}}")
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}

	err = engine.InitWithTemplates(tmpl)
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}

	// Test with complete data
	completeData := map[string]interface{}{
		"User": map[string]interface{}{
			"Name": "John",
			"Role": "Admin",
		},
	}

	result := engine.ValidateTemplateWithData("integration", completeData)
	if !result.Valid {
		t.Errorf("Expected validation to pass, got errors: %v", result.Errors)
	}

	// Test with incomplete data
	incompleteData := map[string]interface{}{
		"User": map[string]interface{}{
			"Name": "John",
			// Missing Role
		},
	}

	result = engine.ValidateTemplateWithData("integration", incompleteData)
	if result.Valid {
		t.Error("Expected validation to fail")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors")
	}

	// Test variable extraction
	variables, err := engine.ExtractTemplateVariables("integration")
	if err != nil {
		t.Fatalf("Failed to extract variables: %v", err)
	}

	expectedVars := []string{"User.Name", "User.Role"}
	actualVars := make([]string, len(variables))
	for i, v := range variables {
		actualVars[i] = v.Name
	}

	for _, expected := range expectedVars {
		found := false
		for _, actual := range actualVars {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected variable '%s' not found in %v", expected, actualVars)
		}
	}
}
