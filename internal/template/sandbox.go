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
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// TemplateSandbox provides a secure template rendering environment with restricted functions.
// It prevents template injection attacks by disabling dangerous functions and enforcing timeouts.
type TemplateSandbox interface {
	// GetSafeFunctions returns a function map containing only safe template functions
	GetSafeFunctions() template.FuncMap

	// ValidateTemplate validates that a template only uses safe functions
	ValidateTemplate(tmpl string) error

	// RenderWithTimeout renders a template with a timeout to prevent denial-of-service
	RenderWithTimeout(tmpl string, data interface{}, timeout time.Duration) (string, error)
}

// DefaultTemplateSandbox implements TemplateSandbox with a safe subset of template functions.
type DefaultTemplateSandbox struct {
	safeFunctions template.FuncMap
	timeout       time.Duration
	auditLogger   interface{} // Will be *security.AuditLogger but using interface to avoid circular import
	actor         string
}

// NewTemplateSandbox creates a new template sandbox with default safe functions and 30-second timeout.
func NewTemplateSandbox() *DefaultTemplateSandbox {
	sandbox := &DefaultTemplateSandbox{
		safeFunctions: make(template.FuncMap),
		timeout:       30 * time.Second,
		actor:         "system",
	}

	// Register safe string manipulation functions
	sandbox.registerSafeStringFunctions()

	// Register safe formatting functions
	sandbox.registerSafeFormattingFunctions()

	// Register safe logic functions (built-in to text/template)
	// These are: if, range, with, eq, ne, lt, le, gt, ge, and, or, not, index, len

	return sandbox
}

// SetAuditLogger sets the audit logger for logging template validation failures
func (s *DefaultTemplateSandbox) SetAuditLogger(logger interface{}) {
	s.auditLogger = logger
}

// SetActor sets the actor (user/system) performing template operations
func (s *DefaultTemplateSandbox) SetActor(actor string) {
	s.actor = actor
}

// NewTemplateSandboxWithTimeout creates a new template sandbox with a custom timeout.
func NewTemplateSandboxWithTimeout(timeout time.Duration) *DefaultTemplateSandbox {
	sandbox := NewTemplateSandbox()
	sandbox.timeout = timeout
	return sandbox
}

// registerSafeStringFunctions registers safe string manipulation functions.
// These functions only manipulate strings and cannot access system resources.
func (s *DefaultTemplateSandbox) registerSafeStringFunctions() {
	s.safeFunctions["upper"] = strings.ToUpper
	s.safeFunctions["lower"] = strings.ToLower
	s.safeFunctions["trim"] = strings.TrimSpace
	s.safeFunctions["trimPrefix"] = strings.TrimPrefix
	s.safeFunctions["trimSuffix"] = strings.TrimSuffix
	s.safeFunctions["replace"] = strings.ReplaceAll
	s.safeFunctions["split"] = strings.Split
	s.safeFunctions["join"] = strings.Join
	s.safeFunctions["contains"] = strings.Contains
	s.safeFunctions["hasPrefix"] = strings.HasPrefix
	s.safeFunctions["hasSuffix"] = strings.HasSuffix
	s.safeFunctions["repeat"] = strings.Repeat
}

// registerSafeFormattingFunctions registers safe formatting functions.
// These functions format data but cannot access system resources.
func (s *DefaultTemplateSandbox) registerSafeFormattingFunctions() {
	// printf is safe - it only formats strings
	s.safeFunctions["printf"] = fmt.Sprintf

	// println is safe - it only formats strings (we'll return the string, not print it)
	s.safeFunctions["println"] = func(args ...interface{}) string {
		return fmt.Sprintln(args...)
	}

	// quote adds quotes around a string
	s.safeFunctions["quote"] = func(s string) string {
		return fmt.Sprintf("%q", s)
	}

	// indent adds indentation to each line
	s.safeFunctions["indent"] = func(spaces int, s string) string {
		indent := strings.Repeat(" ", spaces)
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = indent + line
			}
		}
		return strings.Join(lines, "\n")
	}

	// default returns a default value if the input is empty/zero
	s.safeFunctions["default"] = func(defaultValue, value interface{}) interface{} {
		if value == nil || value == "" || value == 0 || value == false {
			return defaultValue
		}
		return value
	}

	// until generates a slice of integers from 0 to n-1 (safe for iteration)
	s.safeFunctions["until"] = func(count int) []int {
		result := make([]int, count)
		for i := 0; i < count; i++ {
			result[i] = i
		}
		return result
	}
}

// GetSafeFunctions returns the map of safe template functions.
func (s *DefaultTemplateSandbox) GetSafeFunctions() template.FuncMap {
	// Return a copy to prevent external modification
	funcMapCopy := make(template.FuncMap)
	for name, fn := range s.safeFunctions {
		funcMapCopy[name] = fn
	}
	return funcMapCopy
}

// ValidateTemplate validates that a template only uses safe functions.
// It attempts to parse the template with the safe function map and returns an error
// if the template references any undefined (dangerous) functions.
func (s *DefaultTemplateSandbox) ValidateTemplate(tmpl string) error {
	// Try to parse the template with only safe functions
	_, err := template.New("validation").Funcs(s.safeFunctions).Parse(tmpl)
	if err != nil {
		// Check if the error is due to an undefined function
		if strings.Contains(err.Error(), "function") && strings.Contains(err.Error(), "not defined") {
			// Log template validation failure to audit log
			s.logTemplateValidationFailed("template", fmt.Sprintf("dangerous or undefined function: %v", err))
			return fmt.Errorf("template validation failed: template uses dangerous or undefined function: %w", err)
		}
		s.logTemplateValidationFailed("template", fmt.Sprintf("parse error: %v", err))
		return fmt.Errorf("template validation failed: %w", err)
	}
	return nil
}

// logTemplateValidationFailed logs a template validation failure to the audit log if configured
func (s *DefaultTemplateSandbox) logTemplateValidationFailed(templateName, reason string) {
	if s.auditLogger != nil {
		// Use type assertion to call the method
		// This is safe because we control what gets set via SetAuditLogger
		if logger, ok := s.auditLogger.(interface {
			LogTemplateValidationFailed(ctx context.Context, actor, templateName, reason string) error
		}); ok {
			ctx := context.Background()
			_ = logger.LogTemplateValidationFailed(ctx, s.actor, templateName, reason)
		}
	}
}

// RenderWithTimeout renders a template with the given data and enforces a timeout.
// This prevents denial-of-service attacks from infinite loops or extremely slow templates.
func (s *DefaultTemplateSandbox) RenderWithTimeout(tmpl string, data interface{}, timeout time.Duration) (string, error) {
	// Use the provided timeout, or fall back to the sandbox default
	if timeout == 0 {
		timeout = s.timeout
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Channel to receive the result
	type result struct {
		output string
		err    error
	}
	resultChan := make(chan result, 1)

	// Render in a goroutine
	go func() {
		output, err := s.render(tmpl, data)
		resultChan <- result{output: output, err: err}
	}()

	// Wait for either completion or timeout
	select {
	case res := <-resultChan:
		return res.output, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("template rendering timed out after %v", timeout)
	}
}

// render performs the actual template rendering with safe functions.
func (s *DefaultTemplateSandbox) render(tmpl string, data interface{}) (string, error) {
	// First validate the template
	if err := s.ValidateTemplate(tmpl); err != nil {
		return "", err
	}

	// Parse and execute the template
	t, err := template.New("sandbox").Funcs(s.safeFunctions).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// DangerousFunctions returns a list of dangerous Sprig functions that are disabled in the sandbox.
// This is useful for documentation and error messages.
func DangerousFunctions() []string {
	return []string{
		"env",        // Access environment variables
		"expandenv",  // Expand environment variables in strings
		"readFile",   // Read files from filesystem
		"writeFile",  // Write files to filesystem
		"exec",       // Execute shell commands
		"getHostByName", // Network access
		"base",       // File path manipulation (potential path traversal)
		"dir",        // File path manipulation (potential path traversal)
		"clean",      // File path manipulation (potential path traversal)
		"ext",        // File path manipulation (potential path traversal)
		"isAbs",      // File path manipulation (potential path traversal)
	}
}

// IsDangerousFunction checks if a function name is in the list of dangerous functions.
func IsDangerousFunction(funcName string) bool {
	dangerous := DangerousFunctions()
	for _, df := range dangerous {
		if df == funcName {
			return true
		}
	}
	return false
}
