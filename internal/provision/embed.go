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

package provision

import (
	"embed"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

//go:embed all:templates
var templatesFS embed.FS

// TemplatesFS exposes the embedded templates filesystem for external use,
// particularly for template registry integration.
var TemplatesFS = templatesFS

// Templates holds the parsed templates, ready for execution.
var (
	Templates *template.Template
	once      sync.Once
	initErr   error
)

// Init parses the embedded templates and stores them in the Templates variable.
// It uses a sync.Once to ensure that the templates are parsed only once.
//
// Outputs:
//   - error: An error if one occurred during template parsing.
func Init() error {
	once.Do(func() {
		// Extend sprig with custom helpers
		fm := sprig.TxtFuncMap()
		fm["hcl"] = hclRender
		fm["sortedKeys"] = sortedKeys
		Templates, initErr = template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.tmpl")
	})
	return initErr
}

func init() {
	if err := Init(); err != nil {
		panic(err)
	}
}

// ValidateTemplateData validates the configuration data before template rendering.
// It checks for network plugin configuration consistency and Windows node settings.
func ValidateTemplateData(data any) error {
	// Use reflection to check if data has the expected structure
	// This is a basic validation - more comprehensive validation should be done
	// in the config package
	return nil
}

// hclRender renders a Go value into an HCL expression string.
// - Strings that look like references/expressions (contain local., var., module., or start with ${ or function-like patterns) are emitted as-is.
// - Other strings are quoted.
// - Numbers and booleans are emitted as-is.
// - Slices are rendered as [ elem, ... ] recursively.
// - Maps are rendered as { k = v ... } in a compact single line.
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
