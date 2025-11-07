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

package util

import (
	"reflect"
	"testing"
)

// Test structs for reflection testing
type TestStruct struct {
	Name        string `yaml:"name" json:"name"`
	Count       int    `yaml:"count" json:"count"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Internal    string `yaml:"-" json:"-"`
	NoTags      string
	YamlOnly    string `yaml:"yaml_only"`
	JsonOnly    string `json:"json_only"`
}

func TestFindField(t *testing.T) {
	testStruct := TestStruct{
		Name:        "test",
		Count:       42,
		Enabled:     true,
		Description: "test description",
		Internal:    "internal",
		NoTags:      "no tags",
		YamlOnly:    "yaml only",
		JsonOnly:    "json only",
	}
	
	v := reflect.ValueOf(testStruct)
	
	tests := []struct {
		name           string
		fieldName      string
		expectFound    bool
		expectedValue  interface{}
		expectedKind   reflect.Kind
	}{
		{
			name:          "find by yaml tag - name",
			fieldName:     "name",
			expectFound:   true,
			expectedValue: "test",
			expectedKind:  reflect.String,
		},
		{
			name:          "find by yaml tag - count",
			fieldName:     "count",
			expectFound:   true,
			expectedValue: 42,
			expectedKind:  reflect.Int,
		},
		{
			name:          "find by yaml tag - enabled",
			fieldName:     "enabled",
			expectFound:   true,
			expectedValue: true,
			expectedKind:  reflect.Bool,
		},
		{
			name:          "find by yaml tag with omitempty",
			fieldName:     "description",
			expectFound:   true,
			expectedValue: "test description",
			expectedKind:  reflect.String,
		},
		{
			name:          "find yaml-only field",
			fieldName:     "yaml_only",
			expectFound:   true,
			expectedValue: "yaml only",
			expectedKind:  reflect.String,
		},
		{
			name:          "find json-only field",
			fieldName:     "json_only",
			expectFound:   true,
			expectedValue: "json only",
			expectedKind:  reflect.String,
		},
		{
			name:        "field not found",
			fieldName:   "nonexistent",
			expectFound: false,
		},
		{
			name:          "field with dash tag",
			fieldName:     "-",
			expectFound:   true,
			expectedValue: "internal",
			expectedKind:  reflect.String,
		},
		{
			name:        "field without tags",
			fieldName:   "NoTags",
			expectFound: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindField(v, tt.fieldName)
			
			if tt.expectFound {
				if !result.IsValid() {
					t.Errorf("expected to find field %q, but got invalid value", tt.fieldName)
					return
				}
				
				if result.Kind() != tt.expectedKind {
					t.Errorf("expected field kind %v, got %v", tt.expectedKind, result.Kind())
				}
				
				if result.Interface() != tt.expectedValue {
					t.Errorf("expected field value %v, got %v", tt.expectedValue, result.Interface())
				}
			} else {
				if result.IsValid() {
					t.Errorf("expected not to find field %q, but got valid value: %v", tt.fieldName, result.Interface())
				}
			}
		})
	}
}

func TestFindField_NonStruct(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "string value",
			value: "hello",
		},
		{
			name:  "int value",
			value: 42,
		},
		{
			name:  "slice value",
			value: []string{"a", "b", "c"},
		},
		{
			name:  "map value",
			value: map[string]string{"key": "value"},
		},
		{
			name:  "nil value",
			value: nil,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v reflect.Value
			if tt.value != nil {
				v = reflect.ValueOf(tt.value)
			}
			
			result := FindField(v, "any_field")
			if result.IsValid() {
				t.Errorf("expected invalid value for non-struct input, got: %v", result.Interface())
			}
		})
	}
}

func TestFindField_PointerToStruct(t *testing.T) {
	testStruct := &TestStruct{
		Name:    "pointer test",
		Count:   100,
		Enabled: false,
	}
	
	// Test with pointer to struct (should not work directly)
	v := reflect.ValueOf(testStruct)
	result := FindField(v, "name")
	if result.IsValid() {
		t.Error("expected invalid value for pointer to struct without dereferencing")
	}
	
	// Test with dereferenced pointer (should work)
	v = reflect.ValueOf(testStruct).Elem()
	result = FindField(v, "name")
	if !result.IsValid() {
		t.Error("expected valid value for dereferenced pointer to struct")
	} else if result.String() != "pointer test" {
		t.Errorf("expected 'pointer test', got %q", result.String())
	}
}

func TestFindField_EmptyStruct(t *testing.T) {
	type EmptyStruct struct{}
	
	empty := EmptyStruct{}
	v := reflect.ValueOf(empty)
	
	result := FindField(v, "any_field")
	if result.IsValid() {
		t.Error("expected invalid value for empty struct")
	}
}

func TestFindField_NestedStruct(t *testing.T) {
	type NestedStruct struct {
		Inner TestStruct `yaml:"inner" json:"inner"`
	}
	
	nested := NestedStruct{
		Inner: TestStruct{
			Name:  "nested",
			Count: 99,
		},
	}
	
	v := reflect.ValueOf(nested)
	result := FindField(v, "inner")
	
	if !result.IsValid() {
		t.Error("expected to find nested struct field")
		return
	}
	
	if result.Kind() != reflect.Struct {
		t.Errorf("expected struct kind, got %v", result.Kind())
	}
	
	// Test finding field in the nested struct
	innerResult := FindField(result, "name")
	if !innerResult.IsValid() {
		t.Error("expected to find field in nested struct")
	} else if innerResult.String() != "nested" {
		t.Errorf("expected 'nested', got %q", innerResult.String())
	}
}

func TestFindField_TagParsing(t *testing.T) {
	type TagTestStruct struct {
		Field1 string `yaml:"field1,omitempty,flow" json:"field1,omitempty"`
		Field2 string `yaml:"field2" json:"field2,string"`
		Field3 string `yaml:",inline" json:",inline"`
		Field4 string `yaml:"field4," json:"field4,"`
	}
	
	testStruct := TagTestStruct{
		Field1: "value1",
		Field2: "value2",
		Field3: "value3",
		Field4: "value4",
	}
	
	v := reflect.ValueOf(testStruct)
	
	// Test that tag parsing handles additional options correctly
	tests := []struct {
		fieldName     string
		expectedValue string
	}{
		{"field1", "value1"},
		{"field2", "value2"},
		{"field4", "value4"},
	}
	
	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			result := FindField(v, tt.fieldName)
			if !result.IsValid() {
				t.Errorf("expected to find field %q", tt.fieldName)
			} else if result.String() != tt.expectedValue {
				t.Errorf("expected %q, got %q", tt.expectedValue, result.String())
			}
		})
	}
	
	// Test inline tag (empty tag name after comma)
	result := FindField(v, "")
	if !result.IsValid() {
		t.Error("expected to find field with empty tag name (inline)")
	} else if result.String() != "value3" {
		t.Errorf("expected 'value3', got %q", result.String())
	}
}