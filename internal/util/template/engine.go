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
	"embed"
	"fmt"
	"io"
	"sync"
	"text/template"
)

// DefaultTemplateEngine implements TemplateEngine interface with dependency injection
type DefaultTemplateEngine struct {
	renderer             TemplateRenderer
	validator            TemplateValidator
	networkPluginHandler NetworkPluginHandler
	templates            *template.Template
	funcMap              template.FuncMap
	once                 sync.Once
	initErr              error
}

// NewDefaultTemplateEngine creates a new default template engine with dependency injection
func NewDefaultTemplateEngine() *DefaultTemplateEngine {
	return &DefaultTemplateEngine{
		renderer:             NewDefaultTemplateRenderer(),
		validator:            NewDefaultTemplateValidator(),
		networkPluginHandler: NewDefaultNetworkPluginHandler(),
		funcMap:              make(template.FuncMap),
	}
}

// NewTemplateEngineWithDependencies creates a template engine with injected dependencies
func NewTemplateEngineWithDependencies(renderer TemplateRenderer, validator TemplateValidator, networkHandler NetworkPluginHandler) *DefaultTemplateEngine {
	return &DefaultTemplateEngine{
		renderer:             renderer,
		validator:            validator,
		networkPluginHandler: networkHandler,
		funcMap:              make(template.FuncMap),
	}
}

// Init initializes the template engine
func (e *DefaultTemplateEngine) Init() error {
	e.once.Do(func() {
		// Initialize with empty templates - will be set later via InitWithFS or InitWithTemplates
		e.templates = template.New("")
		e.initErr = e.initializeDependencies()
	})
	return e.initErr
}

// initializeDependencies initializes all injected dependencies
func (e *DefaultTemplateEngine) initializeDependencies() error {
	// Ensure we have a template instance
	if e.templates == nil {
		e.templates = template.New("")
	}

	// Initialize renderer
	if defaultRenderer, ok := e.renderer.(*DefaultTemplateRenderer); ok {
		if err := defaultRenderer.Init(e.templates); err != nil {
			return fmt.Errorf("failed to initialize renderer: %w", err)
		}
		// Add renderer functions to template
		e.templates = e.templates.Funcs(defaultRenderer.funcMap)
	}

	// Initialize validator
	if defaultValidator, ok := e.validator.(*DefaultTemplateValidator); ok {
		if err := defaultValidator.Init(e.templates); err != nil {
			return fmt.Errorf("failed to initialize validator: %w", err)
		}
	}

	return nil
}

// InitWithFS initializes the template engine with an embedded filesystem
func (e *DefaultTemplateEngine) InitWithFS(fs interface{}, pattern string) error {
	e.once.Do(func() {
		// Type assert the filesystem
		embedFS, ok := fs.(embed.FS)
		if !ok {
			e.initErr = fmt.Errorf("filesystem must be of type embed.FS")
			return
		}

		// Get function map from renderer
		var funcMap template.FuncMap
		if defaultRenderer, ok := e.renderer.(*DefaultTemplateRenderer); ok {
			funcMap = defaultRenderer.funcMap
		} else {
			funcMap = make(template.FuncMap)
		}

		// Merge custom functions
		for name, fn := range e.funcMap {
			funcMap[name] = fn
		}

		// Parse templates from filesystem
		e.templates, e.initErr = template.New("").Funcs(funcMap).ParseFS(embedFS, pattern)
		if e.initErr != nil {
			e.initErr = fmt.Errorf("failed to parse templates from filesystem: %w", e.initErr)
			return
		}

		// Initialize dependencies
		e.initErr = e.initializeDependencies()
	})
	return e.initErr
}

// InitWithTemplates initializes the template engine with pre-parsed templates
func (e *DefaultTemplateEngine) InitWithTemplates(templates *template.Template) error {
	e.once.Do(func() {
		if templates == nil {
			e.initErr = fmt.Errorf("templates cannot be nil")
			return
		}
		e.templates = templates
		e.initErr = e.initializeDependencies()
	})

	// If already initialized, update the templates and re-initialize dependencies
	if e.initErr == nil && e.templates != templates {
		e.templates = templates
		e.initErr = e.initializeDependencies()
	}

	return e.initErr
}

// AddFunctions adds custom functions to the template engine
func (e *DefaultTemplateEngine) AddFunctions(funcMap template.FuncMap) error {
	if e.funcMap == nil {
		e.funcMap = make(template.FuncMap)
	}

	for name, fn := range funcMap {
		e.funcMap[name] = fn
	}

	return e.renderer.AddFunctions(funcMap)
}

// RenderTemplate renders a template with the given data
func (e *DefaultTemplateEngine) RenderTemplate(templateName string, data interface{}) (string, error) {
	if e.templates == nil {
		return "", NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}

	result, err := e.renderer.RenderTemplate(templateName, data)
	if err != nil {
		return "", NewTemplateExecutionError(templateName, err)
	}

	return result, nil
}

// RenderTemplateToWriter renders a template to a writer
func (e *DefaultTemplateEngine) RenderTemplateToWriter(templateName string, data interface{}, writer io.Writer) error {
	if e.templates == nil {
		return NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}

	err := e.renderer.RenderTemplateToWriter(templateName, data, writer)
	if err != nil {
		return NewTemplateExecutionError(templateName, err)
	}

	return nil
}

// GetTemplate returns a specific template
func (e *DefaultTemplateEngine) GetTemplate(templateName string) (*template.Template, error) {
	if e.templates == nil {
		return nil, fmt.Errorf("template engine not initialized")
	}
	return e.renderer.GetTemplate(templateName)
}

// ListTemplates returns a list of available template names
func (e *DefaultTemplateEngine) ListTemplates() []string {
	if e.templates == nil {
		return []string{}
	}

	templates := e.renderer.ListTemplates()
	if templates == nil {
		return []string{}
	}

	return templates
}

// ValidateTemplate validates that a template exists and can be parsed
func (e *DefaultTemplateEngine) ValidateTemplate(templateName string) error {
	if e.templates == nil {
		return fmt.Errorf("template engine not initialized")
	}
	return e.validator.ValidateTemplate(templateName)
}

// ValidateTemplateData validates that data contains required fields for a template
func (e *DefaultTemplateEngine) ValidateTemplateData(templateName string, data interface{}) error {
	if e.templates == nil {
		return fmt.Errorf("template engine not initialized")
	}
	return e.validator.ValidateTemplateData(templateName, data)
}

// ValidateTemplateWithDataAndResult performs comprehensive validation of template with data
func (e *DefaultTemplateEngine) ValidateTemplateWithDataAndResult(templateName string, data interface{}) (*TemplateValidationResult, error) {
	if e.templates == nil {
		return nil, NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}

	result := e.validator.ValidateTemplateWithData(templateName, data)
	if !result.Valid {
		return result, fmt.Errorf("template validation failed with %d errors", len(result.Errors))
	}

	return result, nil
}

// ValidateTemplateSyntaxEngine validates template syntax
func (e *DefaultTemplateEngine) ValidateTemplateSyntaxEngine(templateName string) error {
	if e.templates == nil {
		return NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}
	return e.validator.ValidateTemplateSyntax(templateName)
}

// ValidateVariableSubstitutionEngine validates variable substitution for a template
func (e *DefaultTemplateEngine) ValidateVariableSubstitutionEngine(templateName string, data interface{}) error {
	if e.templates == nil {
		return NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}
	return e.validator.ValidateVariableSubstitution(templateName, data)
}

// ExtractTemplateVariablesEngine extracts variables from a template
func (e *DefaultTemplateEngine) ExtractTemplateVariablesEngine(templateName string) ([]VariableInfo, error) {
	if e.templates == nil {
		return nil, NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}
	return e.validator.ExtractTemplateVariables(templateName)
}

// ValidateNetworkPluginConfig validates network plugin configuration
func (e *DefaultTemplateEngine) ValidateNetworkPluginConfig(pluginType string, config map[string]interface{}) error {
	return e.validator.ValidateNetworkPluginConfig(pluginType, config)
}

// ValidateTemplateWithData performs comprehensive validation of template with data (interface method)
func (e *DefaultTemplateEngine) ValidateTemplateWithData(templateName string, data interface{}) *TemplateValidationResult {
	if e.templates == nil {
		result := &TemplateValidationResult{
			Valid:  false,
			Errors: []*TemplateError{NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))},
		}
		return result
	}

	return e.validator.ValidateTemplateWithData(templateName, data)
}

// ValidateTemplateSyntax validates template syntax (interface method)
func (e *DefaultTemplateEngine) ValidateTemplateSyntax(templateName string) error {
	if e.templates == nil {
		return NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}
	return e.validator.ValidateTemplateSyntax(templateName)
}

// ValidateVariableSubstitution validates variable substitution for a template (interface method)
func (e *DefaultTemplateEngine) ValidateVariableSubstitution(templateName string, data interface{}) error {
	if e.templates == nil {
		return NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}
	return e.validator.ValidateVariableSubstitution(templateName, data)
}

// ExtractTemplateVariables extracts variables from a template (interface method)
func (e *DefaultTemplateEngine) ExtractTemplateVariables(templateName string) ([]VariableInfo, error) {
	if e.templates == nil {
		return nil, NewInitializationError("template engine", fmt.Errorf("template engine not initialized"))
	}
	return e.validator.ExtractTemplateVariables(templateName)
}

// ValidateTemplateExists validates that a template exists
func (e *DefaultTemplateEngine) ValidateTemplateExists(templateName string) error {
	if e.templates == nil {
		return fmt.Errorf("template engine not initialized")
	}
	return e.validator.ValidateTemplateExists(templateName)
}

// ValidateRequiredFields validates that data contains all required fields
func (e *DefaultTemplateEngine) ValidateRequiredFields(data interface{}, requiredFields []string) error {
	err := e.validator.ValidateRequiredFields(data, requiredFields)
	if err != nil {
		return NewDataValidationError("", "required_fields", err)
	}
	return nil
}

// RenderWithValidation renders a template after validating the data
func (e *DefaultTemplateEngine) RenderWithValidation(templateName string, data interface{}, requiredFields []string) (string, error) {
	// Validate template exists
	if err := e.ValidateTemplateExists(templateName); err != nil {
		return "", fmt.Errorf("template validation failed: %w", err)
	}

	// Validate template data
	if err := e.ValidateTemplateData(templateName, data); err != nil {
		return "", fmt.Errorf("template data validation failed: %w", err)
	}

	// Validate required fields if specified
	if len(requiredFields) > 0 {
		if err := e.ValidateRequiredFields(data, requiredFields); err != nil {
			return "", fmt.Errorf("required fields validation failed: %w", err)
		}
	}

	// Render template
	return e.RenderTemplate(templateName, data)
}

// RenderToWriterWithValidation renders a template to a writer after validating the data
func (e *DefaultTemplateEngine) RenderToWriterWithValidation(templateName string, data interface{}, writer io.Writer, requiredFields []string) error {
	// Validate template exists
	if err := e.ValidateTemplateExists(templateName); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	// Validate template data
	if err := e.ValidateTemplateData(templateName, data); err != nil {
		return fmt.Errorf("template data validation failed: %w", err)
	}

	// Validate required fields if specified
	if len(requiredFields) > 0 {
		if err := e.ValidateRequiredFields(data, requiredFields); err != nil {
			return fmt.Errorf("required fields validation failed: %w", err)
		}
	}

	// Render template
	return e.RenderTemplateToWriter(templateName, data, writer)
}

// GetRenderer returns the template renderer for dependency injection
func (e *DefaultTemplateEngine) GetRenderer() TemplateRenderer {
	return e.renderer
}

// GetValidator returns the template validator for dependency injection
func (e *DefaultTemplateEngine) GetValidator() TemplateValidator {
	return e.validator
}

// GetNetworkPluginHandler returns the network plugin handler for dependency injection
func (e *DefaultTemplateEngine) GetNetworkPluginHandler() NetworkPluginHandler {
	return e.networkPluginHandler
}

// CreateTemplateEngine creates a fully initialized template engine
func CreateTemplateEngine() (TemplateEngine, error) {
	engine := NewDefaultTemplateEngine()
	if err := engine.Init(); err != nil {
		return nil, NewInitializationError("template engine", err)
	}
	return engine, nil
}

// CreateTemplateEngineWithFS creates a template engine initialized with an embedded filesystem
func CreateTemplateEngineWithFS(fs embed.FS, pattern string) (TemplateEngine, error) {
	engine := NewDefaultTemplateEngine()
	if err := engine.InitWithFS(fs, pattern); err != nil {
		return nil, NewInitializationError("template engine with filesystem", err)
	}
	return engine, nil
}

// CreateTemplateEngineWithTemplates creates a template engine initialized with pre-parsed templates
func CreateTemplateEngineWithTemplates(templates *template.Template) (TemplateEngine, error) {
	engine := NewDefaultTemplateEngine()
	if err := engine.InitWithTemplates(templates); err != nil {
		return nil, NewInitializationError("template engine with templates", err)
	}
	return engine, nil
}
