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

// Package template provides a clean, extensible template engine abstraction
// for rendering Go templates with caching, validation, and error reporting.
package template

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/rackerlabs/openCenter-cli/internal/util/errors"
	"github.com/rackerlabs/openCenter-cli/internal/util/metrics"
)

// TemplateEngine provides a unified interface for template rendering operations.
// It abstracts template processing to support multiple template formats and
// provides caching, validation, and comprehensive error reporting.
type TemplateEngine interface {
	// Render renders a template with the given data and returns the result as bytes.
	// The context can be used for cancellation and timeout control.
	Render(ctx context.Context, templatePath string, data interface{}) ([]byte, error)

	// RenderString renders a template string with the given data and returns the result as bytes.
	RenderString(ctx context.Context, templateName, templateContent string, data interface{}) ([]byte, error)

	// RenderToWriter renders a template with the given data and writes the result to the writer.
	RenderToWriter(ctx context.Context, templatePath string, data interface{}, w io.Writer) error

	// ValidateTemplate validates template syntax before execution.
	// Returns an error if the template has syntax errors or is not found.
	ValidateTemplate(templatePath string) error

	// RegisterFunction registers a custom function for use in templates.
	// The function must have a valid signature for template execution.
	RegisterFunction(name string, fn interface{})

	// RegisterFunctions registers multiple custom functions for use in templates.
	RegisterFunctions(funcs template.FuncMap)

	// SetCacheEnabled enables or disables template caching.
	// When enabled, parsed templates are cached for improved performance.
	SetCacheEnabled(enabled bool)

	// ClearCache clears all cached templates, forcing re-parsing on next render.
	ClearCache()

	// LoadFromFS loads templates from an embedded filesystem.
	LoadFromFS(fsys fs.FS, pattern string) error

	// LoadFromFile loads a single template from a file.
	LoadFromFile(path string) error

	// ExecuteTemplate executes a named template from a collection with the given data.
	// This is useful when multiple templates are loaded together and reference each other.
	ExecuteTemplate(templateName string, data interface{}) ([]byte, error)

	// ExecuteTemplateToWriter executes a named template and writes the result to the writer.
	ExecuteTemplateToWriter(templateName string, data interface{}, w io.Writer) error

	// GetTemplate returns a parsed template by name, useful for direct template.Template access.
	GetTemplate(name string) (*template.Template, error)
}

// GoTemplateEngine implements TemplateEngine using Go's text/template package.
// It provides caching, custom function registration, and comprehensive error reporting.
// By default, it includes all Sprig functions for enhanced template capabilities.
type GoTemplateEngine struct {
	funcMap      template.FuncMap
	cache        map[string]*template.Template
	cacheEnabled bool
	mu           sync.RWMutex
	fsys         fs.FS              // Optional embedded filesystem
	rootTemplate *template.Template // Root template for named template collections
	sandbox      *DefaultTemplateSandbox // Optional sandbox for secure rendering
	sandboxed    bool               // Whether sandboxing is enabled
}

// NewGoTemplateEngine creates a new Go template engine with default settings.
// Caching is enabled by default for optimal performance.
// Sprig functions are automatically registered for enhanced template capabilities.
func NewGoTemplateEngine() *GoTemplateEngine {
	engine := &GoTemplateEngine{
		funcMap:      make(template.FuncMap),
		cache:        make(map[string]*template.Template),
		cacheEnabled: true,
		sandboxed:    false,
	}

	// Register Sprig functions by default for compatibility with existing templates
	for name, fn := range sprig.TxtFuncMap() {
		engine.funcMap[name] = fn
	}

	return engine
}

// EnableSandbox enables template sandboxing for secure rendering.
// When enabled, only safe template functions are available and dangerous functions
// (env, readFile, exec, etc.) are disabled. Templates are also subject to timeout enforcement.
func (e *GoTemplateEngine) EnableSandbox() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.sandboxed = true
	e.sandbox = NewTemplateSandbox()

	// Replace function map with safe functions
	e.funcMap = e.sandbox.GetSafeFunctions()

	// Clear cache to ensure new function map is used
	if e.cacheEnabled {
		e.cache = make(map[string]*template.Template)
	}
}

// DisableSandbox disables template sandboxing and restores full Sprig functions.
func (e *GoTemplateEngine) DisableSandbox() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.sandboxed = false
	e.sandbox = nil

	// Restore Sprig functions
	e.funcMap = make(template.FuncMap)
	for name, fn := range sprig.TxtFuncMap() {
		e.funcMap[name] = fn
	}

	// Clear cache to ensure new function map is used
	if e.cacheEnabled {
		e.cache = make(map[string]*template.Template)
	}
}

// IsSandboxed returns whether sandboxing is currently enabled.
func (e *GoTemplateEngine) IsSandboxed() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sandboxed
}

// Render renders a template with the given data.
// It validates the template, checks the cache, and executes the template.
// Returns the rendered output as bytes or an error if rendering fails.
func (e *GoTemplateEngine) Render(ctx context.Context, templatePath string, data interface{}) ([]byte, error) {
	// Start metrics timer
	startTime := time.Now()
	var renderErr error
	defer func() {
		duration := time.Since(startTime)
		// Record metric using global collector
		metrics.RecordTemplateRender(templatePath, duration, renderErr == nil, renderErr)
	}()

	// Check context cancellation
	select {
	case <-ctx.Done():
		renderErr = fmt.Errorf("template rendering cancelled: %w", ctx.Err())
		return nil, renderErr
	default:
	}

	// Get or parse template
	tmpl, err := e.getTemplate(templatePath)
	if err != nil {
		renderErr = wrapTemplateError(err, templatePath)
		return nil, renderErr
	}

	// If sandboxed, validate the template before rendering
	e.mu.RLock()
	sandboxed := e.sandboxed
	sandbox := e.sandbox
	e.mu.RUnlock()

	if sandboxed && sandbox != nil {
		// Read template content for validation
		content, err := e.readTemplateContent(templatePath)
		if err != nil {
			renderErr = fmt.Errorf("failed to read template for validation: %w", err)
			return nil, renderErr
		}

		if err := sandbox.ValidateTemplate(string(content)); err != nil {
			renderErr = fmt.Errorf("template validation failed: %w", err)
			return nil, renderErr
		}
	}

	// Execute template to buffer
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		renderErr = wrapTemplateError(err, templatePath)
		return nil, renderErr
	}

	return buf.Bytes(), nil
}

// readTemplateContent reads the template content from filesystem or embedded FS.
func (e *GoTemplateEngine) readTemplateContent(templatePath string) ([]byte, error) {
	e.mu.RLock()
	fsys := e.fsys
	e.mu.RUnlock()

	if fsys != nil {
		content, err := fs.ReadFile(fsys, templatePath)
		if err != nil {
			// If not found in embedded FS, try regular file system
			return os.ReadFile(templatePath)
		}
		return content, nil
	}

	// Try regular file system
	return os.ReadFile(templatePath)
}

// RenderString renders a template string with the given data.
// This is useful for rendering templates that are not stored in files.
func (e *GoTemplateEngine) RenderString(ctx context.Context, templateName, templateContent string, data interface{}) ([]byte, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("template rendering cancelled: %w", ctx.Err())
	default:
	}

	// Get function map and sandbox status
	e.mu.RLock()
	funcMap := e.funcMap
	sandboxed := e.sandboxed
	sandbox := e.sandbox
	e.mu.RUnlock()

	// If sandboxed, validate the template before rendering
	if sandboxed && sandbox != nil {
		if err := sandbox.ValidateTemplate(templateContent); err != nil {
			return nil, fmt.Errorf("template validation failed: %w", err)
		}
	}

	// Parse template string
	tmpl, err := template.New(templateName).Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return nil, wrapTemplateError(err, templateName)
	}

	// Execute template to buffer
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, wrapTemplateError(err, templateName)
	}

	return buf.Bytes(), nil
}

// RenderToWriter renders a template with the given data and writes the result to the writer.
// This is more efficient than Render when writing directly to a file or network connection.
func (e *GoTemplateEngine) RenderToWriter(ctx context.Context, templatePath string, data interface{}, w io.Writer) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("template rendering cancelled: %w", ctx.Err())
	default:
	}

	// Get or parse template
	tmpl, err := e.getTemplate(templatePath)
	if err != nil {
		return wrapTemplateError(err, templatePath)
	}

	// Execute template directly to writer
	if err := tmpl.Execute(w, data); err != nil {
		return wrapTemplateError(err, templatePath)
	}

	return nil
}

// ValidateTemplate validates that a template exists and has valid syntax.
// It attempts to parse the template and returns any syntax errors.
func (e *GoTemplateEngine) ValidateTemplate(templatePath string) error {
	_, err := e.getTemplate(templatePath)
	if err != nil {
		return wrapTemplateError(err, templatePath)
	}
	return nil
}

// RegisterFunction registers a custom function for use in templates.
// The function is added to the function map and will be available
// in all subsequently parsed templates.
func (e *GoTemplateEngine) RegisterFunction(name string, fn interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.funcMap[name] = fn

	// Clear cache to ensure new function is available in all templates
	if e.cacheEnabled {
		e.cache = make(map[string]*template.Template)
	}
}

// RegisterFunctions registers multiple custom functions for use in templates.
// This is more efficient than calling RegisterFunction multiple times.
func (e *GoTemplateEngine) RegisterFunctions(funcs template.FuncMap) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for name, fn := range funcs {
		e.funcMap[name] = fn
	}

	// Clear cache to ensure new functions are available in all templates
	if e.cacheEnabled {
		e.cache = make(map[string]*template.Template)
	}
}

// SetCacheEnabled enables or disables template caching.
// When disabled, templates are re-parsed on every render.
// When enabled, parsed templates are cached for improved performance.
func (e *GoTemplateEngine) SetCacheEnabled(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cacheEnabled = enabled

	// Clear cache when disabling
	if !enabled {
		e.cache = make(map[string]*template.Template)
	}
}

// ClearCache clears all cached templates.
// This forces all templates to be re-parsed on next render.
func (e *GoTemplateEngine) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cache = make(map[string]*template.Template)
}

// LoadFromFS loads templates from an embedded filesystem.
// The pattern follows the same rules as filepath.Match.
// All matching templates are parsed and cached.
func (e *GoTemplateEngine) LoadFromFS(fsys fs.FS, pattern string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Store filesystem for later use
	e.fsys = fsys

	// Parse templates from filesystem
	tmpl, err := template.New("").Funcs(e.funcMap).ParseFS(fsys, pattern)
	if err != nil {
		return wrapTemplateError(err, pattern)
	}

	// Store root template for named template access
	e.rootTemplate = tmpl

	// Cache all parsed templates
	if e.cacheEnabled {
		for _, t := range tmpl.Templates() {
			e.cache[t.Name()] = t
		}
	}

	return nil
}

// LoadFromFile loads a single template from a file.
// The template is parsed and cached if caching is enabled.
func (e *GoTemplateEngine) LoadFromFile(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return wrapTemplateError(err, path)
	}

	// If we don't have a root template yet, create one
	if e.rootTemplate == nil {
		e.rootTemplate = template.New("").Funcs(e.funcMap)
	}

	// Parse template and add to root template
	_, err = e.rootTemplate.New(path).Parse(string(content))
	if err != nil {
		return wrapTemplateError(err, path)
	}

	// Cache if enabled
	if e.cacheEnabled {
		tmpl := e.rootTemplate.Lookup(path)
		if tmpl != nil {
			e.cache[path] = tmpl
		}
	}

	return nil
}

// getTemplate retrieves a template from cache or parses it.
// It handles caching logic and ensures thread-safe access.
func (e *GoTemplateEngine) getTemplate(templatePath string) (*template.Template, error) {
	// Check cache first (read lock)
	if e.cacheEnabled {
		e.mu.RLock()
		if tmpl, ok := e.cache[templatePath]; ok {
			e.mu.RUnlock()
			return tmpl, nil
		}
		e.mu.RUnlock()
	}

	// Parse template (write lock)
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check cache after acquiring write lock
	if e.cacheEnabled {
		if tmpl, ok := e.cache[templatePath]; ok {
			return tmpl, nil
		}
	}

	// Try to read from filesystem first (if available)
	var content []byte
	var err error

	if e.fsys != nil {
		content, err = fs.ReadFile(e.fsys, templatePath)
		if err != nil {
			// If not found in embedded FS, try regular file system
			content, err = os.ReadFile(templatePath)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Try regular file system
		content, err = os.ReadFile(templatePath)
		if err != nil {
			return nil, err
		}
	}

	// Parse template
	tmpl, err := template.New(templatePath).Funcs(e.funcMap).Parse(string(content))
	if err != nil {
		return nil, err
	}

	// Cache if enabled
	if e.cacheEnabled {
		e.cache[templatePath] = tmpl
	}

	return tmpl, nil
}

// ExecuteTemplate executes a named template from a collection with the given data.
// This is useful when multiple templates are loaded together and reference each other.
// The template must have been previously loaded via LoadFromFS or LoadFromFile.
func (e *GoTemplateEngine) ExecuteTemplate(templateName string, data interface{}) ([]byte, error) {
	e.mu.RLock()
	rootTemplate := e.rootTemplate
	e.mu.RUnlock()

	if rootTemplate == nil {
		return nil, fmt.Errorf("no templates loaded, call LoadFromFS or LoadFromFile first")
	}

	// Execute the named template
	var buf bytes.Buffer
	if err := rootTemplate.ExecuteTemplate(&buf, templateName, data); err != nil {
		return nil, wrapTemplateError(err, templateName)
	}

	return buf.Bytes(), nil
}

// ExecuteTemplateToWriter executes a named template and writes the result to the writer.
// This is more efficient than ExecuteTemplate when writing directly to a file or network connection.
func (e *GoTemplateEngine) ExecuteTemplateToWriter(templateName string, data interface{}, w io.Writer) error {
	e.mu.RLock()
	rootTemplate := e.rootTemplate
	e.mu.RUnlock()

	if rootTemplate == nil {
		return fmt.Errorf("no templates loaded, call LoadFromFS or LoadFromFile first")
	}

	// Execute the named template directly to writer
	if err := rootTemplate.ExecuteTemplate(w, templateName, data); err != nil {
		return wrapTemplateError(err, templateName)
	}

	return nil
}

// GetTemplate returns a parsed template by name.
// This is useful for direct access to the template.Template object.
// The template must have been previously loaded via LoadFromFS or LoadFromFile.
func (e *GoTemplateEngine) GetTemplate(name string) (*template.Template, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check cache first
	if tmpl, ok := e.cache[name]; ok {
		return tmpl, nil
	}

	// Check root template
	if e.rootTemplate != nil {
		tmpl := e.rootTemplate.Lookup(name)
		if tmpl != nil {
			return tmpl, nil
		}
	}

	return nil, fmt.Errorf("template %s not found", name)
}

// TemplateContext provides context information for template rendering.
// It includes configuration data, metadata, and custom functions.
type TemplateContext struct {
	// Config contains the main configuration data for template rendering
	Config interface{}

	// Metadata contains additional metadata that may be used in templates
	Metadata map[string]interface{}

	// Functions contains custom template functions specific to this context
	Functions template.FuncMap
}

// NewTemplateContext creates a new template context with the given configuration.
func NewTemplateContext(config interface{}) *TemplateContext {
	return &TemplateContext{
		Config:    config,
		Metadata:  make(map[string]interface{}),
		Functions: make(template.FuncMap),
	}
}

// WithMetadata adds metadata to the template context.
func (tc *TemplateContext) WithMetadata(key string, value interface{}) *TemplateContext {
	tc.Metadata[key] = value
	return tc
}

// WithFunction adds a custom function to the template context.
func (tc *TemplateContext) WithFunction(name string, fn interface{}) *TemplateContext {
	tc.Functions[name] = fn
	return tc
}

// ToMap converts the template context to a map suitable for template rendering.
func (tc *TemplateContext) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"Config":   tc.Config,
		"Metadata": tc.Metadata,
	}
}

// parseTemplateError extracts line and column information from Go template errors.
// Go template errors typically include location information in formats like:
// - "template: name:line:column: error message"
// - "template: name:line: error message"
// This function parses these formats and returns structured error information.
func parseTemplateError(err error, templatePath string) (lineNum, colNum int, message string) {
	if err == nil {
		return 0, 0, ""
	}

	errMsg := err.Error()
	message = errMsg

	// Pattern 1: "template: name:line:column: message"
	// Example: "template: test.tmpl:5:12: function "nonexistent" not defined"
	pattern1 := regexp.MustCompile(`template:\s+[^:]+:(\d+):(\d+):\s*(.+)`)
	if matches := pattern1.FindStringSubmatch(errMsg); len(matches) == 4 {
		lineNum, _ = strconv.Atoi(matches[1])
		colNum, _ = strconv.Atoi(matches[2])
		message = matches[3]
		return
	}

	// Pattern 2: "template: name:line: message"
	// Example: "template: test.tmpl:5: unexpected EOF"
	pattern2 := regexp.MustCompile(`template:\s+[^:]+:(\d+):\s*(.+)`)
	if matches := pattern2.FindStringSubmatch(errMsg); len(matches) == 3 {
		lineNum, _ = strconv.Atoi(matches[1])
		message = matches[2]
		return
	}

	// Pattern 3: Look for "at line X" or "line X" in the message
	pattern3 := regexp.MustCompile(`(?:at\s+)?line\s+(\d+)`)
	if matches := pattern3.FindStringSubmatch(errMsg); len(matches) == 2 {
		lineNum, _ = strconv.Atoi(matches[1])
		return
	}

	// If no line number found, return 0 and original message
	return 0, 0, errMsg
}

// wrapTemplateError wraps a template error with structured error information including
// line numbers and context. It extracts location information from the error message
// and creates a properly formatted StructuredError.
func wrapTemplateError(err error, templatePath string) error {
	if err == nil {
		return nil
	}

	// Extract line and column information from the error
	lineNum, colNum, message := parseTemplateError(err, templatePath)

	// Read template content to provide context around the error
	var contextLines []string
	if lineNum > 0 {
		contextLines = extractTemplateContext(templatePath, lineNum, 2)
	}

	// Build enhanced error message with context
	enhancedMessage := message
	if len(contextLines) > 0 {
		enhancedMessage += "\n\nTemplate context:\n" + strings.Join(contextLines, "\n")
	}

	// Create structured error with line number information
	if colNum > 0 {
		return errors.CreateTemplateErrorWithColumn(templatePath, lineNum, colNum, enhancedMessage, err)
	} else if lineNum > 0 {
		return errors.CreateTemplateError(templatePath, lineNum, enhancedMessage, err)
	}

	// If no line number could be extracted, create basic template error
	return errors.CreateTemplateError(templatePath, 0, message, err)
}

// extractTemplateContext reads the template file and extracts lines around the error location.
// It returns contextRadius lines before and after the error line for debugging context.
func extractTemplateContext(templatePath string, errorLine, contextRadius int) []string {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	if errorLine < 1 || errorLine > len(lines) {
		return nil
	}

	// Calculate range (1-indexed to 0-indexed)
	startLine := errorLine - contextRadius - 1
	if startLine < 0 {
		startLine = 0
	}

	endLine := errorLine + contextRadius
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Build context with line numbers
	var contextLines []string
	for i := startLine; i < endLine; i++ {
		lineNum := i + 1
		marker := "  "
		if lineNum == errorLine {
			marker = "→ " // Arrow points to error line
		}
		contextLines = append(contextLines, fmt.Sprintf("%s%4d | %s", marker, lineNum, lines[i]))
	}

	return contextLines
}
