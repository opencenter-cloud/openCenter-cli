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

func TestJSONFlagHandler_CanHandle(t *testing.T) {
	handler := NewJSONFlagHandler()
	
	tests := []struct {
		name     string
		flagName string
		expected bool
	}{
		{"json-set flag", "json-set", true},
		{"json-set with path", "json-set-path", true},
		{"other flag", "server-pool", false},
		{"empty flag", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.CanHandle(tt.flagName)
			if result != tt.expected {
				t.Errorf("CanHandle(%s) = %v, want %v", tt.flagName, result, tt.expected)
			}
		})
	}
}

func TestJSONFlagHandler_GetFlagType(t *testing.T) {
	handler := NewJSONFlagHandler()
	
	if handler.GetFlagType() != FlagTypeJSON {
		t.Errorf("GetFlagType() = %v, want %v", handler.GetFlagType(), FlagTypeJSON)
	}
}

func TestJSONFlagHandler_ParseFlag(t *testing.T) {
	handler := NewJSONFlagHandler()
	
	tests := []struct {
		name      string
		flagName  string
		value     string
		expectErr bool
		expected  *JSONFlag
	}{
		{
			name:     "valid JSON object",
			flagName: "json-set-infrastructure.servers",
			value:    `{"count": 3, "flavor": "large"}`,
			expected: &JSONFlag{
				Path:  "infrastructure.servers",
				Value: map[string]interface{}{"count": float64(3), "flavor": "large"},
			},
		},
		{
			name:     "valid JSON array",
			flagName: "json-set-dns.servers",
			value:    `["8.8.8.8", "8.8.4.4"]`,
			expected: &JSONFlag{
				Path:  "dns.servers",
				Value: []interface{}{"8.8.8.8", "8.8.4.4"},
			},
		},
		{
			name:     "valid JSON string",
			flagName: "json-set-cluster.name",
			value:    `"my-cluster"`,
			expected: &JSONFlag{
				Path:  "cluster.name",
				Value: "my-cluster",
			},
		},
		{
			name:      "invalid JSON",
			flagName:  "json-set-test",
			value:     `{"invalid": json}`,
			expectErr: true,
		},
		{
			name:      "empty value",
			flagName:  "json-set-test",
			value:     "",
			expectErr: true,
		},
		{
			name:      "null JSON",
			flagName:  "json-set-test",
			value:     "null",
			expectErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.ParseFlag(tt.flagName, tt.value)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("ParseFlag() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("ParseFlag() unexpected error: %v", err)
				return
			}
			
			jsonFlag, ok := result.(*JSONFlag)
			if !ok {
				t.Errorf("ParseFlag() returned wrong type: %T", result)
				return
			}
			
			if jsonFlag.Path != tt.expected.Path {
				t.Errorf("ParseFlag() path = %v, want %v", jsonFlag.Path, tt.expected.Path)
			}
			
			// Compare values based on type
			if !compareJSONValues(jsonFlag.Value, tt.expected.Value) {
				t.Errorf("ParseFlag() value = %v, want %v", jsonFlag.Value, tt.expected.Value)
			}
		})
	}
}

func TestJSONFlagHandler_MergeIntoConfiguration(t *testing.T) {
	handler := NewJSONFlagHandler()
	
	tests := []struct {
		name     string
		config   *JSONFlag
		target   map[string]interface{}
		expected map[string]interface{}
		expectErr bool
	}{
		{
			name: "merge simple value",
			config: &JSONFlag{
				Path:  "cluster.name",
				Value: "test-cluster",
			},
			target: make(map[string]interface{}),
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "test-cluster",
				},
			},
		},
		{
			name: "merge object",
			config: &JSONFlag{
				Path:  "infrastructure.servers",
				Value: map[string]interface{}{"count": float64(3), "flavor": "large"},
			},
			target: make(map[string]interface{}),
			expected: map[string]interface{}{
				"infrastructure": map[string]interface{}{
					"servers": map[string]interface{}{
						"count":  float64(3),
						"flavor": "large",
					},
				},
			},
		},
		{
			name: "merge into existing structure",
			config: &JSONFlag{
				Path:  "cluster.version",
				Value: "1.28",
			},
			target: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "existing-cluster",
				},
			},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name":    "existing-cluster",
					"version": "1.28",
				},
			},
		},
		{
			name:      "nil config",
			config:    nil,
			target:    make(map[string]interface{}),
			expectErr: true,
		},
		{
			name: "nil target",
			config: &JSONFlag{
				Path:  "test",
				Value: "value",
			},
			target:    nil,
			expectErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.MergeIntoConfiguration(tt.config, tt.target)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("MergeIntoConfiguration() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("MergeIntoConfiguration() unexpected error: %v", err)
				return
			}
			
			if !compareJSONValues(tt.target, tt.expected) {
				t.Errorf("MergeIntoConfiguration() result = %v, want %v", tt.target, tt.expected)
			}
		})
	}
}

// compareJSONValues compares two JSON values for equality
func compareJSONValues(a, b interface{}) bool {
	switch aVal := a.(type) {
	case map[string]interface{}:
		bVal, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		if len(aVal) != len(bVal) {
			return false
		}
		for key, value := range aVal {
			if !compareJSONValues(value, bVal[key]) {
				return false
			}
		}
		return true
	case []interface{}:
		bVal, ok := b.([]interface{})
		if !ok {
			return false
		}
		if len(aVal) != len(bVal) {
			return false
		}
		for i, value := range aVal {
			if !compareJSONValues(value, bVal[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}