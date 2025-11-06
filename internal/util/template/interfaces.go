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
	"io"
	"text/template"
)

// TemplateRenderer interface for rendering templates
type TemplateRenderer interface {
	RenderTemplate(templateName string, data interface{}) (string, error)
	RenderTemplateToWriter(templateName string, data interface{}, writer io.Writer) error
	GetTemplate(templateName string) (*template.Template, error)
	ListTemplates() []string
	AddFunctions(funcMap template.FuncMap) error
}

// TemplateValidator interface for validating templates and data
type TemplateValidator interface {
	ValidateTemplate(templateName string) error
	ValidateTemplateData(templateName string, data interface{}) error
	ValidateTemplateExists(templateName string) error
	ValidateRequiredFields(data interface{}, requiredFields []string) error
}

// TemplateEngine interface combining rendering and validation with dependency injection
type TemplateEngine interface {
	TemplateRenderer
	TemplateValidator
	
	// Initialization methods
	Init() error
	InitWithFS(fs interface{}, pattern string) error
	InitWithTemplates(templates *template.Template) error
	
	// Function management
	AddFunctions(funcMap template.FuncMap) error
	
	// Advanced rendering with validation
	RenderWithValidation(templateName string, data interface{}, requiredFields []string) (string, error)
	RenderToWriterWithValidation(templateName string, data interface{}, writer io.Writer, requiredFields []string) error
	
	// Component access for dependency injection
	GetRenderer() TemplateRenderer
	GetValidator() TemplateValidator
	GetNetworkPluginHandler() NetworkPluginHandler
}

// NetworkPluginHandler interface for handling network plugin templates
type NetworkPluginHandler interface {
	ValidateNetworkPlugin(pluginType string, config map[string]interface{}) error
	RenderNetworkPluginConfig(pluginType string, config map[string]interface{}) (string, error)
	GetSupportedPlugins() []string
	GetRequiredFields(pluginType string) []string
	
	// Mutual exclusivity validation
	ValidateNetworkPluginMutualExclusivity(pluginConfigs map[string]map[string]interface{}) error
	ValidateNetworkPluginCompatibility(pluginType string, config map[string]interface{}, globalConfig map[string]interface{}) error
	
	// Access to dedicated handlers
	GetPluginHandler(pluginType string) (SpecificNetworkPluginHandler, error)
	GetAllPluginHandlers() map[string]SpecificNetworkPluginHandler
}

// TemplateData represents data passed to templates
type TemplateData struct {
	Data   interface{}
	Config map[string]interface{}
}

// ValidationError represents a template validation error
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e ValidationError) Error() string {
	return e.Message
}