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

// Test structures for reflection engine testing
type TestConfig struct {
	Name     string      `yaml:"name"`
	Items    []TestItem  `yaml:"items"`
	Settings TestSettings `yaml:"settings"`
	Metadata map[string]interface{} `yaml:"metadata"`
}

type TestItem struct {
	ID    int    `yaml:"id"`
	Value string `yaml:"value"`
	Tags  []string `yaml:"tags"`
}

type TestSettings struct {
	Enabled bool   `yaml:"enabled"`
	Count   int    `yaml:"count"`
	Nested  *TestNested `yaml:"nested"`
}

type TestNested struct {
	Data []string `yaml:"data"`
}

func TestReflectionEngine_SetField_BasicFields(t *testing.T) {
	engine := NewEnhancedReflectionEngine()
	
	tests := []struct {
		name     string
		path     string
		value    interface{}
		expected interface{}
	}{
		{
			name:     "set string field",
			path:     "name",
			value:    "test-config",
			expected: "test-config",
		},
		{
			name:     "set nested bool field",
			path:     "settings.enabled",
			value:    "true",
			expected: true,
		},
		{
			name:     "set nested int field",
			path:     "settings.count",
			value:    "42",
			expected: 42,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TestConfig{}
			
			err := engine.SetField(config, tt.path, tt.value)
			if err != nil {
				t.Fatalf("SetField failed: %v", err)
			}
			
			// Verify the value was set correctly
			result, err := engine.GetField(config, tt.path)
			if err != nil {
				t.Fatalf("GetField failed: %v", err)
			}
			
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestReflectionEngine_SetField_ArrayAccess(t *testing.T) {
	engine := NewEnhancedReflectionEngine()
	
	tests := []struct {
		name        string
		setupPath   string
		setupValue  interface{}
		testPath    string
		testValue   interface{}
		expectedLen int
		expectedVal interface{}
	}{
		{
			name:        "bracket syntax array access",
			testPath:    "items[0].id",
			testValue:   "123",
			expectedLen: 1,
			expectedVal: 123,
		},
		{
			name:        "dot syntax array access",
			testPath:    "items.0.value",
			testValue:   "test-value",
			expectedLen: 1,
			expectedVal: "test-value",
		},
		{
			name:        "expand array automatically",
			testPath:    "items[2].id",
			testValue:   "456",
			expectedLen: 3,
			expectedVal: 456,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TestConfig{}
			
			// Setup if needed
			if tt.setupPath != "" {
				err := engine.SetField(config, tt.setupPath, tt.setupValue)
				if err != nil {
					t.Fatalf("Setup SetField failed: %v", err)
				}
			}
			
			// Test the main operation
			err := engine.SetField(config, tt.testPath, tt.testValue)
			if err != nil {
				t.Fatalf("SetField failed: %v", err)
			}
			
			// Verify array length
			if len(config.Items) != tt.expectedLen {
				t.Errorf("expected array length %d, got %d", tt.expectedLen, len(config.Items))
			}
			
			// Verify the value was set correctly
			result, err := engine.GetField(config, tt.testPath)
			if err != nil {
				t.Fatalf("GetField failed: %v", err)
			}
			
			if result != tt.expectedVal {
				t.Errorf("expected %v, got %v", tt.expectedVal, result)
			}
		})
	}
}

func TestReflectionEngine_SetField_NestedArrays(t *testing.T) {
	engine := NewEnhancedReflectionEngine()
	config := &TestConfig{}
	
	// Test nested array access
	err := engine.SetField(config, "items[0].tags[1]", "tag2")
	if err != nil {
		t.Fatalf("SetField failed: %v", err)
	}
	
	// Verify the structure was created correctly
	if len(config.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(config.Items))
	}
	
	if len(config.Items[0].Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(config.Items[0].Tags))
	}
	
	if config.Items[0].Tags[1] != "tag2" {
		t.Errorf("expected 'tag2', got %q", config.Items[0].Tags[1])
	}
}

func TestReflectionEngine_SetField_PointerFields(t *testing.T) {
	engine := NewEnhancedReflectionEngine()
	config := &TestConfig{}
	
	// Test setting field in nil pointer struct
	err := engine.SetField(config, "settings.nested.data[0]", "nested-value")
	if err != nil {
		t.Fatalf("SetField failed: %v", err)
	}
	
	// Verify the nested pointer was initialized
	if config.Settings.Nested == nil {
		t.Error("expected nested pointer to be initialized")
	}
	
	if len(config.Settings.Nested.Data) != 1 {
		t.Errorf("expected 1 data item, got %d", len(config.Settings.Nested.Data))
	}
	
	if config.Settings.Nested.Data[0] != "nested-value" {
		t.Errorf("expected 'nested-value', got %q", config.Settings.Nested.Data[0])
	}
}

func TestReflectionEngine_SetField_MapFields(t *testing.T) {
	engine := NewEnhancedReflectionEngine()
	config := &TestConfig{}
	
	// Test setting map values
	err := engine.SetField(config, "metadata.key1", "value1")
	if err != nil {
		t.Fatalf("SetField failed: %v", err)
	}
	
	// Verify the map was initialized and value set
	if config.Metadata == nil {
		t.Error("expected metadata map to be initialized")
	}
	
	if config.Metadata["key1"] != "value1" {
		t.Errorf("expected 'value1', got %v", config.Metadata["key1"])
	}
}

func TestReflectionEngine_ExpandArray(t *testing.T) {
	engine := NewEnhancedReflectionEngine()
	config := &TestConfig{}
	
	// Test expanding array to accommodate index
	err := engine.ExpandArray(config, "items", 5)
	if err != nil {
		t.Fatalf("ExpandArray failed: %v", err)
	}
	
	// Verify array was expanded
	if len(config.Items) != 6 { // index 5 means length 6
		t.Errorf("expected array length 6, got %d", len(config.Items))
	}
}

func TestReflectionEngine_ErrorHandling(t *testing.T) {
	engine := NewEnhancedReflectionEngine()
	
	tests := []struct {
		name    string
		obj     interface{}
		path    string
		value   interface{}
		wantErr bool
	}{
		{
			name:    "nil object",
			obj:     nil,
			path:    "field",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "non-pointer object",
			obj:     TestConfig{},
			path:    "field",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "invalid field name",
			obj:     &TestConfig{},
			path:    "nonexistent",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "invalid path syntax",
			obj:     &TestConfig{},
			path:    "field[invalid]",
			value:   "value",
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.SetField(tt.obj, tt.path, tt.value)
			
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}