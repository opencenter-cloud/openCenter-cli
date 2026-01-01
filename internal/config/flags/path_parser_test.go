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
	"testing"
)

func TestPathParser_BasicParsing(t *testing.T) {
	parser := NewEnhancedPathParser()
	
	tests := []struct {
		name     string
		path     string
		wantErr  bool
		expected *StructuredPath
	}{
		{
			name: "simple field",
			path: "field",
			expected: &StructuredPath{
				Parts: []PathPart{
					{Name: "field", Index: -1, IsArray: false, HasIndex: false},
				},
				IsArray:  false,
				HasIndex: false,
				RawPath:  "field",
			},
		},
		{
			name: "nested field",
			path: "field.subfield",
			expected: &StructuredPath{
				Parts: []PathPart{
					{Name: "field", Index: -1, IsArray: false, HasIndex: false},
					{Name: "subfield", Index: -1, IsArray: false, HasIndex: false},
				},
				IsArray:  false,
				HasIndex: false,
				RawPath:  "field.subfield",
			},
		},
		{
			name: "bracket syntax",
			path: "field[0].subfield",
			expected: &StructuredPath{
				Parts: []PathPart{
					{Name: "field", Index: 0, IsArray: true, HasIndex: true},
					{Name: "subfield", Index: -1, IsArray: false, HasIndex: false},
				},
				IsArray:  true,
				HasIndex: true,
				RawPath:  "field[0].subfield",
			},
		},
		{
			name: "dot syntax for array",
			path: "field.0.subfield",
			expected: &StructuredPath{
				Parts: []PathPart{
					{Name: "field", Index: -1, IsArray: false, HasIndex: false},
					{Name: "", Index: 0, IsArray: true, HasIndex: true},
					{Name: "subfield", Index: -1, IsArray: false, HasIndex: false},
				},
				IsArray:  true,
				HasIndex: true,
				RawPath:  "field.0.subfield",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePath(tt.path)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}
			
			// Check basic properties
			if result.IsArray != tt.expected.IsArray {
				t.Errorf("IsArray: got %v, want %v", result.IsArray, tt.expected.IsArray)
			}
			
			if result.HasIndex != tt.expected.HasIndex {
				t.Errorf("HasIndex: got %v, want %v", result.HasIndex, tt.expected.HasIndex)
			}
			
			if len(result.Parts) != len(tt.expected.Parts) {
				t.Errorf("Parts length: got %d, want %d", len(result.Parts), len(tt.expected.Parts))
				return
			}
			
			// Check each part
			for i, part := range result.Parts {
				expected := tt.expected.Parts[i]
				if part.Name != expected.Name {
					t.Errorf("Part[%d].Name: got %q, want %q", i, part.Name, expected.Name)
				}
				if part.Index != expected.Index {
					t.Errorf("Part[%d].Index: got %d, want %d", i, part.Index, expected.Index)
				}
				if part.IsArray != expected.IsArray {
					t.Errorf("Part[%d].IsArray: got %v, want %v", i, part.IsArray, expected.IsArray)
				}
				if part.HasIndex != expected.HasIndex {
					t.Errorf("Part[%d].HasIndex: got %v, want %v", i, part.HasIndex, expected.HasIndex)
				}
			}
		})
	}
}

func TestPathParser_GetFieldNames(t *testing.T) {
	parser := NewEnhancedPathParser()
	
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "bracket syntax",
			path:     "storage[0].compute",
			expected: []string{"storage", "compute"},
		},
		{
			name:     "dot syntax",
			path:     "storage.0.compute",
			expected: []string{"storage", "compute"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePath(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			fields := result.GetFieldNames()
			if len(fields) != len(tt.expected) {
				t.Errorf("field count: got %d, want %d", len(fields), len(tt.expected))
				return
			}
			
			for i, field := range fields {
				if field != tt.expected[i] {
					t.Errorf("field[%d]: got %q, want %q", i, field, tt.expected[i])
				}
			}
		})
	}
}

func TestPathParser_GetIndices(t *testing.T) {
	parser := NewEnhancedPathParser()
	
	tests := []struct {
		name     string
		path     string
		expected []int
	}{
		{
			name:     "bracket syntax",
			path:     "storage[0].compute",
			expected: []int{0},
		},
		{
			name:     "dot syntax",
			path:     "storage.0.compute",
			expected: []int{0},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePath(tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			indices := result.GetIndices()
			if len(indices) != len(tt.expected) {
				t.Errorf("index count: got %d, want %d", len(indices), len(tt.expected))
				return
			}
			
			for i, index := range indices {
				if index != tt.expected[i] {
					t.Errorf("index[%d]: got %d, want %d", i, index, tt.expected[i])
				}
			}
		})
	}
}