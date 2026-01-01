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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLFlagHandler_CanHandle(t *testing.T) {
	handler := NewYAMLFlagHandler()
	
	tests := []struct {
		name     string
		flagName string
		expected bool
	}{
		{"yaml-set flag", "yaml-set", true},
		{"yaml-set with path", "yaml-set-path", true},
		{"yaml-data flag", "yaml-data", true},
		{"yaml-data with path", "yaml-data-config", true},
		{"yaml-file flag", "yaml-file", true},
		{"yaml-file with path", "yaml-file-config", true},
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

func TestYAMLFlagHandler_GetFlagType(t *testing.T) {
	handler := NewYAMLFlagHandler()
	
	if handler.GetFlagType() != FlagTypeYAML {
		t.Errorf("GetFlagType() = %v, want %v", handler.GetFlagType(), FlagTypeYAML)
	}
}

func TestYAMLFlagHandler_ParseFlag(t *testing.T) {
	handler := NewYAMLFlagHandler()
	
	tests := []struct {
		name      string
		flagName  string
		value     string
		expectErr bool
		expected  *YAMLFlag
	}{
		{
			name:     "valid YAML object",
			flagName: "yaml-set-infrastructure.servers",
			value:    "count: 3\nflavor: large",
			expected: &YAMLFlag{
				Path:  "infrastructure.servers",
				Value: map[string]interface{}{"count": 3, "flavor": "large"},
			},
		},
		{
			name:     "valid YAML array",
			flagName: "yaml-data-dns.servers",
			value:    "- 8.8.8.8\n- 8.8.4.4",
			expected: &YAMLFlag{
				Path:  "dns.servers",
				Value: []interface{}{"8.8.8.8", "8.8.4.4"},
			},
		},
		{
			name:     "valid YAML string",
			flagName: "yaml-set-cluster.name",
			value:    "my-cluster",
			expected: &YAMLFlag{
				Path:  "cluster.name",
				Value: "my-cluster",
			},
		},
		{
			name:      "invalid YAML",
			flagName:  "yaml-set-test",
			value:     "invalid: yaml: content:",
			expectErr: true,
		},
		{
			name:      "empty value",
			flagName:  "yaml-set-test",
			value:     "",
			expectErr: true,
		},
		{
			name:      "null YAML",
			flagName:  "yaml-set-test",
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
			
			yamlFlag, ok := result.(*YAMLFlag)
			if !ok {
				t.Errorf("ParseFlag() returned wrong type: %T", result)
				return
			}
			
			if yamlFlag.Path != tt.expected.Path {
				t.Errorf("ParseFlag() path = %v, want %v", yamlFlag.Path, tt.expected.Path)
			}
			
			// Compare values based on type
			if !compareYAMLValues(yamlFlag.Value, tt.expected.Value) {
				t.Errorf("ParseFlag() value = %v, want %v", yamlFlag.Value, tt.expected.Value)
			}
		})
	}
}

func TestYAMLFlagHandler_ParseYAMLFile(t *testing.T) {
	handler := NewYAMLFlagHandler()
	
	// Create a temporary YAML file
	tmpDir, err := ioutil.TempDir("", "yaml_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	yamlContent := `
cluster:
  name: test-cluster
  version: "1.28"
infrastructure:
  servers:
    count: 3
    flavor: large
`
	
	tmpFile := filepath.Join(tmpDir, "test.yaml")
	if err := ioutil.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	
	tests := []struct {
		name      string
		flagName  string
		value     string
		expectErr bool
	}{
		{
			name:     "valid YAML file",
			flagName: "yaml-file-config",
			value:    "config=" + tmpFile,
		},
		{
			name:      "invalid file format",
			flagName:  "yaml-file-config",
			value:     "invalid-format",
			expectErr: true,
		},
		{
			name:      "non-existent file",
			flagName:  "yaml-file-config",
			value:     "config=/non/existent/file.yaml",
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
			
			yamlFlag, ok := result.(*YAMLFlag)
			if !ok {
				t.Errorf("ParseFlag() returned wrong type: %T", result)
				return
			}
			
			if yamlFlag.Path != "config" {
				t.Errorf("ParseFlag() path = %v, want %v", yamlFlag.Path, "config")
			}
			
			// Verify the loaded content has the expected structure
			if configMap, ok := yamlFlag.Value.(map[string]interface{}); ok {
				if cluster, exists := configMap["cluster"]; exists {
					if clusterMap, ok := cluster.(map[string]interface{}); ok {
						if name, exists := clusterMap["name"]; !exists || name != "test-cluster" {
							t.Errorf("Expected cluster.name to be 'test-cluster', got %v", name)
						}
					}
				}
			} else {
				t.Errorf("Expected YAML value to be a map, got %T", yamlFlag.Value)
			}
		})
	}
}

func TestYAMLFlagHandler_MergeIntoConfiguration(t *testing.T) {
	handler := NewYAMLFlagHandler()
	
	tests := []struct {
		name     string
		config   *YAMLFlag
		target   map[string]interface{}
		expected map[string]interface{}
		expectErr bool
	}{
		{
			name: "merge simple value",
			config: &YAMLFlag{
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
			config: &YAMLFlag{
				Path:  "infrastructure.servers",
				Value: map[string]interface{}{"count": 3, "flavor": "large"},
			},
			target: make(map[string]interface{}),
			expected: map[string]interface{}{
				"infrastructure": map[string]interface{}{
					"servers": map[string]interface{}{
						"count":  3,
						"flavor": "large",
					},
				},
			},
		},
		{
			name: "merge into existing structure",
			config: &YAMLFlag{
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
			config: &YAMLFlag{
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
			
			if !compareYAMLValues(tt.target, tt.expected) {
				t.Errorf("MergeIntoConfiguration() result = %v, want %v", tt.target, tt.expected)
			}
		})
	}
}

// compareYAMLValues compares two YAML values for equality
func compareYAMLValues(a, b interface{}) bool {
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
			if !compareYAMLValues(value, bVal[key]) {
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
			if !compareYAMLValues(value, bVal[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}