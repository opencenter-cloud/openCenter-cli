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
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	// Test that Init() can be called multiple times without error
	err := Init()
	if err != nil {
		t.Errorf("Init() failed: %v", err)
	}
	
	// Call again to test sync.Once behavior
	err = Init()
	if err != nil {
		t.Errorf("Init() failed on second call: %v", err)
	}
	
	// Verify that Templates is not nil
	if Templates == nil {
		t.Error("Templates should not be nil after Init()")
	}
}

func TestTemplatesInitialization(t *testing.T) {
	// Verify that templates are loaded
	if Templates == nil {
		t.Fatal("Templates should be initialized")
	}
	
	// Check that some expected templates exist
	expectedTemplates := []string{
		"ansible.cfg.tmpl",
		"inventory.tmpl",
		"main.tf.tmpl",
		"variables.tf.tmpl",
	}
	
	for _, tmplName := range expectedTemplates {
		tmpl := Templates.Lookup(tmplName)
		if tmpl == nil {
			t.Errorf("Expected template %s not found", tmplName)
		}
	}
}

func TestValidateTemplateData(t *testing.T) {
	tests := []struct {
		name string
		data any
		expectError bool
	}{
		{
			name: "nil data",
			data: nil,
			expectError: false, // Current implementation doesn't validate
		},
		{
			name: "string data",
			data: "test",
			expectError: false,
		},
		{
			name: "map data",
			data: map[string]any{"key": "value"},
			expectError: false,
		},
		{
			name: "struct data",
			data: struct{ Name string }{Name: "test"},
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemplateData(tt.data)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestHclRender(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		// Nil values
		{
			name:     "nil value",
			input:    nil,
			expected: "null",
		},
		
		// String values
		{
			name:     "simple string",
			input:    "hello",
			expected: `"hello"`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: `""`,
		},
		{
			name:     "string with quotes",
			input:    `hello "world"`,
			expected: `"hello \"world\""`,
		},
		{
			name:     "terraform local reference",
			input:    "local.cluster_name",
			expected: "local.cluster_name",
		},
		{
			name:     "terraform variable reference",
			input:    "var.region",
			expected: "var.region",
		},
		{
			name:     "terraform module reference",
			input:    "module.vpc.id",
			expected: "module.vpc.id",
		},
		{
			name:     "terraform interpolation",
			input:    "${local.prefix}-cluster",
			expected: "${local.prefix}-cluster",
		},
		{
			name:     "function call",
			input:    "join(\"-\", [local.prefix, \"cluster\"])",
			expected: "join(\"-\", [local.prefix, \"cluster\"])",
		},
		
		// Boolean values
		{
			name:     "true boolean",
			input:    true,
			expected: "true",
		},
		{
			name:     "false boolean",
			input:    false,
			expected: "false",
		},
		
		// Integer values
		{
			name:     "positive integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "zero integer",
			input:    0,
			expected: "0",
		},
		{
			name:     "negative integer",
			input:    -10,
			expected: "-10",
		},
		{
			name:     "int64",
			input:    int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		
		// Float values
		{
			name:     "float32",
			input:    float32(3.14),
			expected: "3.14",
		},
		{
			name:     "float64",
			input:    3.14159,
			expected: "3.14159",
		},
		
		// Slice values
		{
			name:     "empty slice",
			input:    []any{},
			expected: "[]",
		},
		{
			name:     "string slice",
			input:    []string{"a", "b", "c"},
			expected: `["a", "b", "c"]`,
		},
		{
			name:     "mixed slice",
			input:    []any{"hello", 42, true},
			expected: `["hello", 42, true]`,
		},
		{
			name:     "nested slice",
			input:    []any{[]string{"a", "b"}, []int{1, 2}},
			expected: `[["a", "b"], [1, 2]]`,
		},
		
		// Map values
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: "{}",
		},
		{
			name:     "simple map",
			input:    map[string]any{"name": "test", "count": 3},
			expected: `{ count = 3 name = "test" }`, // Keys are sorted
		},
		{
			name:     "nested map",
			input:    map[string]any{"outer": map[string]any{"inner": "value"}},
			expected: `{ outer = { inner = "value" } }`,
		},
		{
			name:     "map with terraform references",
			input:    map[string]any{"vpc_id": "local.vpc_id", "count": 3},
			expected: `{ count = 3 vpc_id = local.vpc_id }`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hclRender(tt.input)
			if result != tt.expected {
				t.Errorf("hclRender(%v) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsExpr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: false,
		},
		{
			name:     "terraform interpolation",
			input:    "${local.cluster_name}",
			expected: true,
		},
		{
			name:     "local reference",
			input:    "local.vpc_id",
			expected: true,
		},
		{
			name:     "variable reference",
			input:    "var.region",
			expected: true,
		},
		{
			name:     "module reference",
			input:    "module.vpc.subnet_id",
			expected: true,
		},
		{
			name:     "function call",
			input:    "join(\"-\", [\"a\", \"b\"])",
			expected: true,
		},
		{
			name:     "function with no args",
			input:    "timestamp()",
			expected: true,
		},
		{
			name:     "string with parentheses but not function",
			input:    "hello (world)",
			expected: true, // The heuristic considers any string with ( as an expression
		},
		{
			name:     "string starting with parentheses",
			input:    "(not a function)",
			expected: false, // Parentheses at position 0 are not considered expressions
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "string starting with local but not reference",
			input:    "locally grown",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExpr(tt.input)
			if result != tt.expected {
				t.Errorf("isExpr(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEscapeQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no quotes",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single quote",
			input:    `hello "world"`,
			expected: `hello \"world\"`,
		},
		{
			name:     "multiple quotes",
			input:    `"hello" "world"`,
			expected: `\"hello\" \"world\"`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only quotes",
			input:    `"""`,
			expected: `\"\"\"`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeQuotes(tt.input)
			if result != tt.expected {
				t.Errorf("escapeQuotes(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSortedKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected []string
	}{
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: []string{},
		},
		{
			name:     "single key",
			input:    map[string]any{"key": "value"},
			expected: []string{"key"},
		},
		{
			name:     "multiple keys",
			input:    map[string]any{"zebra": 1, "alpha": 2, "beta": 3},
			expected: []string{"alpha", "beta", "zebra"},
		},
		{
			name:     "numeric-like keys",
			input:    map[string]any{"10": "ten", "2": "two", "1": "one"},
			expected: []string{"1", "10", "2"}, // String sort, not numeric
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sortedKeys(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("sortedKeys() returned %d keys, expected %d", len(result), len(tt.expected))
				return
			}
			for i, key := range result {
				if key != tt.expected[i] {
					t.Errorf("sortedKeys() key %d = %q, expected %q", i, key, tt.expected[i])
				}
			}
		})
	}
}

func TestHclRenderComplexStructures(t *testing.T) {
	// Test complex nested structures
	complexData := map[string]any{
		"cluster": map[string]any{
			"name":    "local.cluster_name",
			"version": "1.28.0",
			"nodes": map[string]any{
				"master": map[string]any{
					"count":  3,
					"flavor": "var.master_flavor",
				},
				"worker": map[string]any{
					"count":  5,
					"flavor": "var.worker_flavor",
				},
			},
		},
		"networking": map[string]any{
			"enabled": true,
			"subnets": []string{"10.0.1.0/24", "10.0.2.0/24"},
		},
	}
	
	result := hclRender(complexData)
	
	// Verify the result contains expected elements
	expectedElements := []string{
		"cluster =",
		"networking =",
		"name = local.cluster_name",
		"version = \"1.28.0\"",
		"count = 3",
		"count = 5",
		"enabled = true",
		"[\"10.0.1.0/24\", \"10.0.2.0/24\"]",
	}
	
	for _, element := range expectedElements {
		if !strings.Contains(result, element) {
			t.Errorf("Expected result to contain %q, but it didn't. Result: %s", element, result)
		}
	}
}

func TestTemplateCustomFunctions(t *testing.T) {
	// Test that custom functions are available in templates
	if Templates == nil {
		t.Fatal("Templates not initialized")
	}
	
	// Create a simple template to test custom functions
	tmplText := `{{- hcl .data -}}`
	tmpl, err := Templates.New("test").Parse(tmplText)
	if err != nil {
		t.Fatalf("Failed to parse test template: %v", err)
	}
	
	// Test data
	data := map[string]any{
		"data": map[string]any{
			"name":  "test",
			"count": 3,
		},
	}
	
	var result strings.Builder
	err = tmpl.Execute(&result, data)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}
	
	output := result.String()
	expected := `{ count = 3 name = "test" }`
	if output != expected {
		t.Errorf("Template output = %q, expected %q", output, expected)
	}
}

func TestTemplateSprigFunctions(t *testing.T) {
	// Test that sprig functions are available
	if Templates == nil {
		t.Fatal("Templates not initialized")
	}
	
	// Test upper function from sprig
	tmplText := `{{ upper "hello" }}`
	tmpl, err := Templates.New("test-sprig").Parse(tmplText)
	if err != nil {
		t.Fatalf("Failed to parse test template: %v", err)
	}
	
	var result strings.Builder
	err = tmpl.Execute(&result, nil)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}
	
	output := result.String()
	expected := "HELLO"
	if output != expected {
		t.Errorf("Template output = %q, expected %q", output, expected)
	}
}