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
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// DefaultTemplateRenderer implements TemplateRenderer interface
type DefaultTemplateRenderer struct {
	templates *template.Template
	funcMap   template.FuncMap
}

// NewDefaultTemplateRenderer creates a new default template renderer
func NewDefaultTemplateRenderer() *DefaultTemplateRenderer {
	funcMap := sprig.TxtFuncMap()

	// Add custom template functions
	funcMap["hcl"] = hclRender
	funcMap["sortedKeys"] = sortedKeys
	funcMap["hasField"] = hasField
	funcMap["getField"] = getField
	funcMap["isNil"] = isNil
	funcMap["isEmpty"] = isEmpty
	funcMap["toYAML"] = toYAML
	funcMap["fromYAML"] = fromYAML

	return &DefaultTemplateRenderer{
		funcMap: funcMap,
	}
}

// Init initializes the template renderer with templates
func (r *DefaultTemplateRenderer) Init(templates *template.Template) error {
	if templates == nil {
		return fmt.Errorf("templates cannot be nil")
	}
	r.templates = templates
	return nil
}

// RenderTemplate renders a template with the given data
func (r *DefaultTemplateRenderer) RenderTemplate(templateName string, data interface{}) (string, error) {
	if r.templates == nil {
		return "", fmt.Errorf("templates not initialized")
	}

	var buf bytes.Buffer
	if err := r.RenderTemplateToWriter(templateName, data, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderTemplateToWriter renders a template to a writer
func (r *DefaultTemplateRenderer) RenderTemplateToWriter(templateName string, data interface{}, writer io.Writer) error {
	if r.templates == nil {
		return fmt.Errorf("templates not initialized")
	}

	tmpl := r.templates.Lookup(templateName)
	if tmpl == nil {
		return fmt.Errorf("template not found: %s", templateName)
	}

	if err := tmpl.Execute(writer, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return nil
}

// GetTemplate returns a specific template
func (r *DefaultTemplateRenderer) GetTemplate(templateName string) (*template.Template, error) {
	if r.templates == nil {
		return nil, fmt.Errorf("templates not initialized")
	}

	tmpl := r.templates.Lookup(templateName)
	if tmpl == nil {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	return tmpl, nil
}

// ListTemplates returns a list of available template names
func (r *DefaultTemplateRenderer) ListTemplates() []string {
	if r.templates == nil {
		return []string{}
	}

	var names []string
	for _, tmpl := range r.templates.Templates() {
		if tmpl.Name() != "" {
			names = append(names, tmpl.Name())
		}
	}
	sort.Strings(names)
	return names
}

// AddFunctions adds custom functions to the template renderer
func (r *DefaultTemplateRenderer) AddFunctions(funcMap template.FuncMap) error {
	if r.funcMap == nil {
		r.funcMap = make(template.FuncMap)
	}

	for name, fn := range funcMap {
		r.funcMap[name] = fn
	}

	return nil
}

// hclRender renders a Go value into an HCL expression string
func hclRender(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		s := strings.TrimSpace(t)
		if isExpr(s) {
			return s
		}
		return fmt.Sprintf("\"%s\"", escapeQuotes(s))
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%v", t)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", t)
	case float32, float64:
		return fmt.Sprintf("%v", t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, hclRender(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []string:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, hclRender(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		// Stable order
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s = %s", k, hclRender(t[k])))
		}
		if len(parts) == 0 {
			return "{}"
		}
		return "{ " + strings.Join(parts, " ") + " }"
	default:
		// Handle maps unmarshaled as map[interface{}]interface{} or other slices
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Map:
			iter := rv.MapRange()
			tmp := map[string]any{}
			for iter.Next() {
				k := fmt.Sprintf("%v", iter.Key().Interface())
				tmp[k] = iter.Value().Interface()
			}
			return hclRender(tmp)
		case reflect.Slice, reflect.Array:
			n := rv.Len()
			parts := make([]string, 0, n)
			for i := 0; i < n; i++ {
				parts = append(parts, hclRender(rv.Index(i).Interface()))
			}
			return "[" + strings.Join(parts, ", ") + "]"
		}
		return fmt.Sprintf("\"%v\"", v)
	}
}

// isExpr checks if a string is a Terraform expression
func isExpr(s string) bool {
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		return true
	}
	if strings.Contains(s, "local.") || strings.Contains(s, "var.") || strings.Contains(s, "module.") {
		return true
	}
	// Heuristic: function-like pattern foo(
	if i := strings.Index(s, "("); i > 0 {
		return true
	}
	return false
}

// escapeQuotes escapes quotes in a string
func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}

// sortedKeys returns the sorted keys of a map[string]any
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// hasField checks if a struct has a specific field
func hasField(obj interface{}, fieldName string) bool {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}
	return v.FieldByName(fieldName).IsValid()
}

// getField gets a field value from a struct
func getField(obj interface{}, fieldName string) interface{} {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	return field.Interface()
}

// isNil checks if a value is nil
func isNil(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

// isEmpty checks if a value is empty
func isEmpty(v interface{}) bool {
	if isNil(v) {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return rv.IsNil()
	}
	return false
}

// toYAML converts a value to YAML string (placeholder implementation)
func toYAML(v interface{}) string {
	// This would require a YAML library like gopkg.in/yaml.v3
	// For now, return a simple string representation
	return fmt.Sprintf("%v", v)
}

// fromYAML parses a YAML string (placeholder implementation)
func fromYAML(s string) interface{} {
	// This would require a YAML library like gopkg.in/yaml.v3
	// For now, return the string as-is
	return s
}
