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

// Package template provides legacy compatibility functions for existing template calls.
// This allows gradual migration from direct text/template usage to the new TemplateEngine interface.
package template

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// Feature flag environment variable for controlling template engine selection
const (
	// EnvUseNewTemplateEngine controls whether to use the new template engine (true)
	// or the legacy direct text/template implementation (false/unset).
	// This allows gradual migration and rollback capability.
	EnvUseNewTemplateEngine = "OPENCENTER_USE_NEW_TEMPLATE_ENGINE"
)

// UseNewTemplateEngine returns true if the new template engine should be used.
// This checks the OPENCENTER_USE_NEW_TEMPLATE_ENGINE environment variable.
// Valid values for enabling: "true", "1", "yes", "on" (case-insensitive)
// Any other value or unset means use legacy system.
func UseNewTemplateEngine() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(EnvUseNewTemplateEngine)))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

// LegacyTemplateRenderer provides backward compatibility for existing template rendering code.
// It wraps the new TemplateEngine interface while maintaining the same API as the old code.
type LegacyTemplateRenderer struct {
	engine TemplateEngine
}

// NewLegacyTemplateRenderer creates a new legacy template renderer that uses the new engine.
func NewLegacyTemplateRenderer() *LegacyTemplateRenderer {
	return &LegacyTemplateRenderer{
		engine: NewGoTemplateEngine(),
	}
}

// RenderTemplateToFile renders a template from embedded filesystem to a file.
// This maintains compatibility with the existing renderTemplate function in gitops/copy.go.
//
// The function respects the OPENCENTER_USE_NEW_TEMPLATE_ENGINE feature flag:
//   - If enabled: Uses the new GoTemplateEngine with caching and enhanced error handling
//   - If disabled (default): Uses the legacy direct text/template implementation
//
// Parameters:
//   - fsys: The embedded filesystem containing the template
//   - templatePath: Path to the template file within the filesystem
//   - outputPath: Destination file path for the rendered output
//   - data: Data to pass to the template
//
// Returns an error if template rendering or file writing fails.
func RenderTemplateToFile(fsys fs.FS, templatePath, outputPath string, data interface{}) error {
	// Check feature flag to determine which engine to use
	if UseNewTemplateEngine() {
		// Use new template engine with caching and enhanced features
		return RenderWithEngine(defaultEngine, fsys, templatePath, outputPath, data)
	}

	// Legacy implementation - direct text/template usage
	return renderLegacyTemplateToFile(fsys, templatePath, outputPath, data)
}

// renderLegacyTemplateToFile is the original legacy implementation.
// This is kept separate to ensure exact backward compatibility.
func renderLegacyTemplateToFile(fsys fs.FS, templatePath, outputPath string, data interface{}) error {
	// Read template content from embedded filesystem
	content, err := fs.ReadFile(fsys, templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// Handle special cases for files that contain conflicting template syntax
	templateContent := string(content)
	filename := filepath.Base(templatePath)

	// For Makefile.tpl, escape Helm template syntax to prevent Go template parsing conflicts
	if filename == "Makefile.tpl" {
		templateContent = strings.ReplaceAll(templateContent, `--template="{{.Version}}"`, `--template="{{"{{"}}.Version{{"}}"}}"`)
	}

	// Create a new template with Sprig functions
	tmpl, err := template.New(filename).Funcs(sprig.TxtFuncMap()).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer f.Close()

	// Execute template to file
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	return nil
}

// RenderTemplateString renders a template string with the given data.
// This is useful for inline template rendering without file I/O.
func RenderTemplateString(templateName, templateContent string, data interface{}) (string, error) {
	engine := NewGoTemplateEngine()
	result, err := engine.RenderString(context.Background(), templateName, templateContent, data)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// RenderTemplateToWriter renders a template to an io.Writer.
// This is more efficient than rendering to a string when writing to files or network connections.
//
// The function respects the OPENCENTER_USE_NEW_TEMPLATE_ENGINE feature flag:
//   - If enabled: Uses the new GoTemplateEngine
//   - If disabled (default): Uses the legacy direct text/template implementation
func RenderTemplateToWriter(fsys fs.FS, templatePath string, data interface{}, w io.Writer) error {
	// Check feature flag to determine which engine to use
	if UseNewTemplateEngine() {
		// Use new template engine
		content, err := fs.ReadFile(fsys, templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templatePath, err)
		}

		result, err := defaultEngine.RenderString(context.Background(), filepath.Base(templatePath), string(content), data)
		if err != nil {
			return err
		}

		_, err = w.Write(result)
		return err
	}

	// Legacy implementation
	return renderLegacyTemplateToWriter(fsys, templatePath, data, w)
}

// renderLegacyTemplateToWriter is the original legacy implementation.
func renderLegacyTemplateToWriter(fsys fs.FS, templatePath string, data interface{}, w io.Writer) error {
	// Read template content from embedded filesystem
	content, err := fs.ReadFile(fsys, templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// Handle special cases for files that contain conflicting template syntax
	templateContent := string(content)
	filename := filepath.Base(templatePath)

	// For Makefile.tpl, escape Helm template syntax to prevent Go template parsing conflicts
	if filename == "Makefile.tpl" {
		templateContent = strings.ReplaceAll(templateContent, `--template="{{.Version}}"`, `--template="{{"{{"}}.Version{{"}}"}}"`)
	}

	// Create a new template with Sprig functions
	tmpl, err := template.New(filename).Funcs(sprig.TxtFuncMap()).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	// Execute template to writer
	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	return nil
}

// CopyFileFromFS copies a file from an embedded filesystem to a destination path.
// This maintains compatibility with the existing copyFile function in gitops/copy.go.
func CopyFileFromFS(fsys fs.FS, sourcePath, destPath string) error {
	// Read file content from embedded filesystem
	data, err := fs.ReadFile(fsys, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", sourcePath, err)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Write file to destination
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}

	return nil
}

// GetDefaultEngine returns a default template engine instance for backward compatibility.
// This allows existing code to gradually migrate to using the engine directly.
var defaultEngine *GoTemplateEngine

func init() {
	defaultEngine = NewGoTemplateEngine()
}

// GetDefaultEngine returns the default template engine instance.
// This is useful for code that needs direct access to the engine.
func GetDefaultEngine() TemplateEngine {
	return defaultEngine
}

// RenderWithEngine renders a template using the provided engine.
// This is a helper function for code that wants to use a custom engine instance.
func RenderWithEngine(engine TemplateEngine, fsys fs.FS, templatePath, outputPath string, data interface{}) error {
	// Read template content from embedded filesystem
	content, err := fs.ReadFile(fsys, templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// Render using the engine
	result, err := engine.RenderString(context.Background(), filepath.Base(templatePath), string(content), data)
	if err != nil {
		return err
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write result to file
	if err := os.WriteFile(outputPath, result, 0o644); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
	}

	return nil
}
