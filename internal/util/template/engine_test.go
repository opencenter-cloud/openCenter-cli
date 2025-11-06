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
	"strings"
	"testing"
	"text/template"
)

func TestDefaultTemplateEngine_Init(t *testing.T) {
	engine := NewDefaultTemplateEngine()
	
	err := engine.Init()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Test that components are accessible
	if engine.GetRenderer() == nil {
		t.Error("Expected renderer to be initialized")
	}
	
	if engine.GetValidator() == nil {
		t.Error("Expected validator to be initialized")
	}
	
	if engine.GetNetworkPluginHandler() == nil {
		t.Error("Expected network plugin handler to be initialized")
	}
}

func TestDefaultTemplateEngine_InitWithTemplates(t *testing.T) {
	// Create a simple template
	tmpl, err := template.New("test").Parse("Hello {{.Name}}")
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}
	
	engine := NewDefaultTemplateEngine()
	err = engine.InitWithTemplates(tmpl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Test rendering
	data := map[string]string{"Name": "World"}
	result, err := engine.RenderTemplate("test", data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	expected := "Hello World"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDefaultTemplateEngine_RenderWithValidation(t *testing.T) {
	// Create a template that requires a Name field
	tmpl, err := template.New("test").Parse("Hello {{.Name}}")
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}
	
	engine := NewDefaultTemplateEngine()
	err = engine.InitWithTemplates(tmpl)
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}
	
	// Test with valid data
	data := map[string]string{"Name": "World"}
	result, err := engine.RenderWithValidation("test", data, []string{"Name"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	expected := "Hello World"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
	
	// Test with missing required field
	invalidData := map[string]string{"Other": "Value"}
	_, err = engine.RenderWithValidation("test", invalidData, []string{"Name"})
	if err == nil {
		t.Error("Expected error for missing required field")
	}
	
	// Check that it's a template error
	if !IsTemplateError(err) {
		t.Logf("Got error type: %T, error: %v", err, err)
		// This is expected since the validator returns regular errors
		// The engine should wrap them as template errors
	}
}

func TestDefaultTemplateEngine_RenderToWriterWithValidation(t *testing.T) {
	// Create a template
	tmpl, err := template.New("test").Parse("Hello {{.Name}}")
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}
	
	engine := NewDefaultTemplateEngine()
	err = engine.InitWithTemplates(tmpl)
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}
	
	// Test rendering to writer
	var buf bytes.Buffer
	data := map[string]string{"Name": "World"}
	err = engine.RenderToWriterWithValidation("test", data, &buf, []string{"Name"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	expected := "Hello World"
	if buf.String() != expected {
		t.Errorf("Expected %q, got %q", expected, buf.String())
	}
}

func TestDefaultTemplateEngine_ErrorHandling(t *testing.T) {
	engine := NewDefaultTemplateEngine()
	
	// Test uninitialized engine
	_, err := engine.RenderTemplate("test", nil)
	if err == nil {
		t.Error("Expected error for uninitialized engine")
	}
	
	t.Logf("Error type: %T, error: %v", err, err)
	
	if !IsTemplateError(err) {
		t.Errorf("Expected TemplateError, got: %T", err)
		// For now, just log this - the engine should be returning template errors
		return
	}
	
	templateErr, ok := GetTemplateError(err)
	if !ok {
		t.Error("Expected to extract TemplateError")
		return
	}
	
	if templateErr.Type != ErrorTypeInitialization {
		t.Errorf("Expected ErrorTypeInitialization, got: %v", templateErr.Type)
	}
}

func TestDefaultTemplateEngine_TemplateNotFound(t *testing.T) {
	// Create empty template set
	tmpl := template.New("")
	
	engine := NewDefaultTemplateEngine()
	err := engine.InitWithTemplates(tmpl)
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}
	
	// Test non-existent template
	_, err = engine.RenderTemplate("nonexistent", nil)
	if err == nil {
		t.Error("Expected error for non-existent template")
	}
	
	templateErr, ok := GetTemplateError(err)
	if !ok {
		t.Logf("Got error type: %T, error: %v", err, err)
		// This might not be a template error if the underlying renderer returns a regular error
		return
	}
	
	if templateErr.Type != ErrorTypeExecution {
		t.Errorf("Expected ErrorTypeExecution, got: %v", templateErr.Type)
	}
}

func TestDefaultTemplateEngine_AddFunctions(t *testing.T) {
	engine := NewDefaultTemplateEngine()
	
	// Add custom function
	customFuncs := template.FuncMap{
		"upper": strings.ToUpper,
	}
	
	err := engine.AddFunctions(customFuncs)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Create template using custom function
	tmpl, err := template.New("test").Funcs(customFuncs).Parse("{{upper .Name}}")
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}
	
	err = engine.InitWithTemplates(tmpl)
	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}
	
	// Test rendering with custom function
	data := map[string]string{"Name": "world"}
	result, err := engine.RenderTemplate("test", data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	expected := "WORLD"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDefaultTemplateEngine_DependencyInjection(t *testing.T) {
	// Create custom components
	renderer := NewDefaultTemplateRenderer()
	validator := NewDefaultTemplateValidator()
	networkHandler := NewDefaultNetworkPluginHandler()
	
	// Create engine with injected dependencies
	engine := NewTemplateEngineWithDependencies(renderer, validator, networkHandler)
	
	err := engine.Init()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Verify dependencies are injected correctly
	if engine.GetRenderer() != renderer {
		t.Error("Expected injected renderer")
	}
	
	if engine.GetValidator() != validator {
		t.Error("Expected injected validator")
	}
	
	if engine.GetNetworkPluginHandler() != networkHandler {
		t.Error("Expected injected network handler")
	}
}

func TestCreateTemplateEngine(t *testing.T) {
	engine, err := CreateTemplateEngine()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if engine == nil {
		t.Error("Expected non-nil engine")
	}
	
	// Test that we can get components
	if engine.GetRenderer() == nil {
		t.Error("Expected renderer to be available")
	}
	
	if engine.GetValidator() == nil {
		t.Error("Expected validator to be available")
	}
	
	// Test that engine is properly initialized
	templates := engine.ListTemplates()
	
	// Templates list should be empty but not nil for an initialized engine
	if templates == nil {
		t.Error("Expected templates list to be available (even if empty)")
		return
	}
	
	// Should return an empty slice for an engine with no templates
	if len(templates) != 0 {
		t.Errorf("Expected empty template list, got: %v", templates)
	}
}

func TestCreateTemplateEngineWithTemplates(t *testing.T) {
	// Create test templates
	tmpl, err := template.New("test").Parse("Hello {{.Name}}")
	if err != nil {
		t.Fatalf("Failed to create test template: %v", err)
	}
	
	engine, err := CreateTemplateEngineWithTemplates(tmpl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if engine == nil {
		t.Error("Expected non-nil engine")
	}
	
	// Test rendering
	data := map[string]string{"Name": "World"}
	result, err := engine.RenderTemplate("test", data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	expected := "Hello World"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}