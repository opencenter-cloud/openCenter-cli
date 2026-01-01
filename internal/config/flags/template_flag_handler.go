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
	"strings"
)

// TemplateFlagHandler handles --template-var flags
type TemplateFlagHandler struct{}

// NewTemplateFlagHandler creates a new template flag handler
func NewTemplateFlagHandler() *TemplateFlagHandler {
	return &TemplateFlagHandler{}
}

// CanHandle returns true if this handler can process the given flag
func (h *TemplateFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "template-var")
}

// ParseFlag processes a single flag and returns the parsed result
func (h *TemplateFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	return h.parseTemplateFlag(flagName, value)
}

// GetFlagType returns the type of flags this handler processes
func (h *TemplateFlagHandler) GetFlagType() FlagType {
	return FlagTypeTemplate
}

// parseTemplateFlag parses a template variable flag
func (h *TemplateFlagHandler) parseTemplateFlag(flagName, value string) (*TemplateVariable, error) {
	if value == "" {
		return nil, fmt.Errorf("template variable value cannot be empty")
	}
	
	// Extract variable name from flag name
	varName := h.extractVariableName(flagName)
	if varName == "" {
		return nil, fmt.Errorf("invalid template flag format: expected --template-var-NAME, got %s", flagName)
	}
	
	templateVar := &TemplateVariable{
		Name:  varName,
		Value: value,
	}
	
	return templateVar, nil
}

// extractVariableName extracts the variable name from a template flag name
func (h *TemplateFlagHandler) extractVariableName(flagName string) string {
	// Handle different template flag formats:
	// --template-var-NAME -> NAME
	if strings.HasPrefix(flagName, "template-var-") {
		return strings.TrimPrefix(flagName, "template-var-")
	}
	
	return ""
}

// TemplateVariable represents a parsed template variable
type TemplateVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// String returns a string representation of the template variable
func (v *TemplateVariable) String() string {
	return fmt.Sprintf("TemplateVariable{Name: %s, Value: %s}", v.Name, v.Value)
}