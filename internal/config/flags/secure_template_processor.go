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
	"os"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
	"github.com/rackerlabs/opencenter-cli/internal/util/security"
)

// SecureTemplateProcessor handles secure template variable processing
type SecureTemplateProcessor struct {
	masker          security.CredentialMasker
	secureVars      map[string]string
	warningsEnabled bool
	fileSystem      fs.FileSystem
}

// NewSecureTemplateProcessor creates a new secure template processor
func NewSecureTemplateProcessor() *SecureTemplateProcessor {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	return &SecureTemplateProcessor{
		masker:          security.NewDefaultCredentialMasker(),
		secureVars:      make(map[string]string),
		warningsEnabled: true,
		fileSystem:      fileSystem,
	}
}

// AddSecureVariable adds a secure template variable
func (p *SecureTemplateProcessor) AddSecureVariable(key, value string, isFile bool) error {
	if key == "" {
		return fmt.Errorf("template variable key cannot be empty")
	}

	// Validate that the value is not empty
	if value == "" {
		return fmt.Errorf("template variable value cannot be empty for key: %s", key)
	}

	// Store the variable
	p.secureVars[key] = value

	// Warn about potential security issues if not from file
	if !isFile && p.warningsEnabled {
		p.warnCommandHistoryExposure(key)
	}

	return nil
}

// GetSecureVariable retrieves a secure template variable
func (p *SecureTemplateProcessor) GetSecureVariable(key string) (string, bool) {
	value, exists := p.secureVars[key]
	return value, exists
}

// GetAllSecureVariables returns all secure template variables (masked for logging)
func (p *SecureTemplateProcessor) GetAllSecureVariables() map[string]string {
	masked := make(map[string]string)
	for key := range p.secureVars {
		// For secure variables, always mask the entire value
		masked[key] = security.MaskString
	}
	return masked
}

// ProcessSecureTemplates processes templates with secure variables
func (p *SecureTemplateProcessor) ProcessSecureTemplates(content string) (string, error) {
	result := content

	// Replace secure template variables
	for key, value := range p.secureVars {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// ValidateSecureVariables validates that all required secure variables are provided
func (p *SecureTemplateProcessor) ValidateSecureVariables(content string) []string {
	var missing []string

	// Find all template variables in content
	templateVars := p.extractTemplateVariables(content)

	for _, varName := range templateVars {
		if _, exists := p.secureVars[varName]; !exists {
			missing = append(missing, varName)
		}
	}

	return missing
}

// extractTemplateVariables extracts template variable names from content
func (p *SecureTemplateProcessor) extractTemplateVariables(content string) []string {
	var variables []string

	// Simple regex-like extraction for {{.VAR}} patterns
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		start := 0
		for {
			startIdx := strings.Index(line[start:], "{{.")
			if startIdx == -1 {
				break
			}
			startIdx += start

			endIdx := strings.Index(line[startIdx:], "}}")
			if endIdx == -1 {
				break
			}
			endIdx += startIdx

			// Extract variable name
			varExpr := line[startIdx+3 : endIdx]
			varName := strings.TrimSpace(varExpr)

			// Remove any additional template syntax
			if spaceIdx := strings.Index(varName, " "); spaceIdx != -1 {
				varName = varName[:spaceIdx]
			}

			if varName != "" {
				variables = append(variables, varName)
			}

			start = endIdx + 2
		}
	}

	return variables
}

// LoadSecureVariableFromFile loads a secure variable from a file
func (p *SecureTemplateProcessor) LoadSecureVariableFromFile(key, filePath string) error {
	if key == "" {
		return fmt.Errorf("template variable key cannot be empty")
	}

	if filePath == "" {
		return fmt.Errorf("file path cannot be empty for secure template variable %s", key)
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("secure template variable file does not exist: %s", filePath)
	}

	// Read file content
	content, err := p.fileSystem.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read secure template variable file %s: %w", filePath, err)
	}

	// Store the variable (trim whitespace)
	value := strings.TrimSpace(string(content))
	if value == "" {
		return fmt.Errorf("secure template variable file %s is empty", filePath)
	}

	p.secureVars[key] = value

	return nil
}

// LoadSecureVariablesFromEnv loads secure variables from environment variables
func (p *SecureTemplateProcessor) LoadSecureVariablesFromEnv(prefix string) error {
	if prefix == "" {
		prefix = "OPENCENTER_SECURE_"
	}

	// Ensure prefix ends with underscore
	if !strings.HasSuffix(prefix, "_") {
		prefix += "_"
	}

	// Get all environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		envKey := parts[0]
		envValue := parts[1]

		// Check if this environment variable matches our prefix
		if strings.HasPrefix(envKey, prefix) {
			// Extract the variable name (remove prefix)
			varName := envKey[len(prefix):]
			if varName != "" {
				p.secureVars[varName] = envValue
			}
		}
	}

	return nil
}

// SetWarningsEnabled enables or disables security warnings
func (p *SecureTemplateProcessor) SetWarningsEnabled(enabled bool) {
	p.warningsEnabled = enabled
}

// warnCommandHistoryExposure warns about potential command history exposure
func (p *SecureTemplateProcessor) warnCommandHistoryExposure(key string) {
	if !p.warningsEnabled {
		return
	}

	fmt.Fprintf(os.Stderr, "WARNING: Secure template variable '%s' provided via command line may be exposed in command history.\n", key)
	fmt.Fprintf(os.Stderr, "Consider using --secure-template-var %s=@file or environment variables instead.\n", key)
	fmt.Fprintf(os.Stderr, "To disable this warning, use --security-warnings=false\n")
}

// ClearSecureVariables clears all secure template variables
func (p *SecureTemplateProcessor) ClearSecureVariables() {
	// Clear the map
	for key := range p.secureVars {
		delete(p.secureVars, key)
	}
}

// GetSecureVariableCount returns the number of secure variables
func (p *SecureTemplateProcessor) GetSecureVariableCount() int {
	return len(p.secureVars)
}

// HasSecureVariable checks if a secure variable exists
func (p *SecureTemplateProcessor) HasSecureVariable(key string) bool {
	_, exists := p.secureVars[key]
	return exists
}

// MaskSecureVariablesInString masks secure variables in a string for logging
func (p *SecureTemplateProcessor) MaskSecureVariablesInString(content string) string {
	result := content

	// Replace all secure variable values with masked versions
	for _, value := range p.secureVars {
		if value != "" && len(value) > 4 {
			masked := p.masker.MaskString(value)
			result = strings.ReplaceAll(result, value, masked)
		}
	}

	return result
}
