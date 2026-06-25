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

func TestDefaultConfigurationMerger_MergeConfigurations(t *testing.T) {
	merger := NewDefaultConfigurationMerger()

	tests := []struct {
		name      string
		configs   []Configuration
		expected  map[string]interface{}
		expectErr bool
	}{
		{
			name:     "empty configurations",
			configs:  []Configuration{},
			expected: map[string]interface{}{},
		},
		{
			name: "single configuration",
			configs: []Configuration{
				{
					Data: map[string]interface{}{
						"cluster": map[string]interface{}{
							"name": "test-cluster",
						},
					},
					Sources: []ConfigSource{{Type: SourceFile, Path: "config.yaml"}},
				},
			},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "test-cluster",
				},
			},
		},
		{
			name: "merge two configurations with precedence",
			configs: []Configuration{
				{
					Data: map[string]interface{}{
						"cluster": map[string]interface{}{
							"name":    "file-cluster",
							"version": "1.27",
						},
					},
					Sources: []ConfigSource{{Type: SourceFile, Path: "config.yaml"}},
				},
				{
					Data: map[string]interface{}{
						"cluster": map[string]interface{}{
							"name": "cli-cluster",
						},
					},
					Sources: []ConfigSource{{Type: SourceCLI, Path: "command-line"}},
				},
			},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name":    "cli-cluster", // CLI overrides file
					"version": "1.27",        // File value preserved
				},
			},
		},
		{
			name: "merge arrays with append strategy",
			configs: []Configuration{
				{
					Data: map[string]interface{}{
						"dns": []interface{}{"8.8.8.8", "8.8.4.4"},
					},
					Sources: []ConfigSource{{Type: SourceFile, Path: "config.yaml"}},
				},
				{
					Data: map[string]interface{}{
						"dns": []interface{}{"1.1.1.1"},
					},
					Sources: []ConfigSource{{Type: SourceCLI, Path: "command-line"}},
				},
			},
			expected: map[string]interface{}{
				"dns": []interface{}{"8.8.8.8", "8.8.4.4", "1.1.1.1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := merger.MergeConfigurations(tt.configs)

			if tt.expectErr {
				if err == nil {
					t.Errorf("MergeConfigurations() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("MergeConfigurations() unexpected error: %v", err)
				return
			}

			if !compareConfigValues(result.Data, tt.expected) {
				t.Errorf("MergeConfigurations() result = %v, want %v", result.Data, tt.expected)
			}
		})
	}
}

func TestDefaultConfigurationMerger_SetMergeStrategy(t *testing.T) {
	merger := NewDefaultConfigurationMerger()

	tests := []struct {
		name      string
		strategy  MergeStrategy
		expectErr bool
	}{
		{
			name: "valid strategy",
			strategy: MergeStrategy{
				ArrayMergeMode:  ArrayMergeReplace,
				ObjectMergeMode: ObjectMergeShallow,
				Precedence:      []SourceType{SourceDefault, SourceCLI},
			},
		},
		{
			name: "empty precedence",
			strategy: MergeStrategy{
				ArrayMergeMode:  ArrayMergeAppend,
				ObjectMergeMode: ObjectMergeDeep,
				Precedence:      []SourceType{},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := merger.SetMergeStrategy(tt.strategy)

			if tt.expectErr {
				if err == nil {
					t.Errorf("SetMergeStrategy() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SetMergeStrategy() unexpected error: %v", err)
				return
			}
		})
	}
}

func TestDefaultConfigurationMerger_ArrayMergeModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     ArrayMergeMode
		target   []interface{}
		source   []interface{}
		expected []interface{}
	}{
		{
			name:     "append mode",
			mode:     ArrayMergeAppend,
			target:   []interface{}{"a", "b"},
			source:   []interface{}{"c", "d"},
			expected: []interface{}{"a", "b", "c", "d"},
		},
		{
			name:     "replace mode",
			mode:     ArrayMergeReplace,
			target:   []interface{}{"a", "b"},
			source:   []interface{}{"c", "d"},
			expected: []interface{}{"c", "d"},
		},
		{
			name:     "merge mode",
			mode:     ArrayMergeMerge,
			target:   []interface{}{"a", "b", "c"},
			source:   []interface{}{"x", "y"},
			expected: []interface{}{"x", "y", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewDefaultConfigurationMerger()
			merger.SetMergeStrategy(MergeStrategy{
				ArrayMergeMode:  tt.mode,
				ObjectMergeMode: ObjectMergeDeep,
				Precedence:      []SourceType{SourceDefault, SourceCLI},
			})

			result, err := merger.mergeArrays(tt.target, tt.source)
			if err != nil {
				t.Errorf("mergeArrays() unexpected error: %v", err)
				return
			}

			resultArray, ok := result.([]interface{})
			if !ok {
				t.Errorf("mergeArrays() returned non-array: %T", result)
				return
			}

			if !compareArrays(resultArray, tt.expected) {
				t.Errorf("mergeArrays() result = %v, want %v", resultArray, tt.expected)
			}
		})
	}
}

func TestDefaultConfigurationMerger_ObjectMergeModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     ObjectMergeMode
		target   map[string]interface{}
		source   map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "deep merge mode",
			mode: ObjectMergeDeep,
			target: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name":    "target-cluster",
					"version": "1.27",
				},
				"other": "value",
			},
			source: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "source-cluster",
					"size": 3,
				},
			},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name":    "source-cluster", // Overridden
					"version": "1.27",           // Preserved
					"size":    3,                // Added
				},
				"other": "value", // Preserved
			},
		},
		{
			name: "shallow merge mode",
			mode: ObjectMergeShallow,
			target: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name":    "target-cluster",
					"version": "1.27",
				},
				"other": "value",
			},
			source: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "source-cluster",
					"size": 3,
				},
			},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "source-cluster", // Completely replaced
					"size": 3,
				},
				"other": "value", // Preserved
			},
		},
		{
			name: "replace mode",
			mode: ObjectMergeReplace,
			target: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name":    "target-cluster",
					"version": "1.27",
				},
				"other": "value",
			},
			source: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "source-cluster",
					"size": 3,
				},
			},
			expected: map[string]interface{}{
				"cluster": map[string]interface{}{
					"name": "source-cluster",
					"size": 3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewDefaultConfigurationMerger()
			merger.SetMergeStrategy(MergeStrategy{
				ArrayMergeMode:  ArrayMergeAppend,
				ObjectMergeMode: tt.mode,
				Precedence:      []SourceType{SourceDefault, SourceCLI},
			})

			result, err := merger.mergeObjects(tt.target, tt.source)
			if err != nil {
				t.Errorf("mergeObjects() unexpected error: %v", err)
				return
			}

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Errorf("mergeObjects() returned non-map: %T", result)
				return
			}

			if !compareConfigValues(resultMap, tt.expected) {
				t.Errorf("mergeObjects() result = %v, want %v", resultMap, tt.expected)
			}
		})
	}
}

func TestDefaultConfigurationMerger_AddConfigSource(t *testing.T) {
	merger := NewDefaultConfigurationMerger()

	tests := []struct {
		name      string
		source    ConfigSource
		expectErr bool
	}{
		{
			name: "valid source",
			source: ConfigSource{
				Type:     SourceFile,
				Path:     "config.yaml",
				Priority: 1,
			},
		},
		{
			name: "empty type",
			source: ConfigSource{
				Type:     "",
				Path:     "config.yaml",
				Priority: 1,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := merger.AddConfigSource(tt.source)

			if tt.expectErr {
				if err == nil {
					t.Errorf("AddConfigSource() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("AddConfigSource() unexpected error: %v", err)
				return
			}
		})
	}
}

func TestDefaultConfigurationMerger_PrecedenceOrdering(t *testing.T) {
	merger := NewDefaultConfigurationMerger()

	// Set custom precedence: File < Template < CLI
	merger.SetMergeStrategy(MergeStrategy{
		ArrayMergeMode:  ArrayMergeAppend,
		ObjectMergeMode: ObjectMergeDeep,
		Precedence:      []SourceType{SourceFile, SourceTemplate, SourceCLI},
	})

	configs := []Configuration{
		{
			Data: map[string]interface{}{
				"value": "cli-value",
			},
			Sources: []ConfigSource{{Type: SourceCLI, Path: "command-line"}},
		},
		{
			Data: map[string]interface{}{
				"value": "file-value",
			},
			Sources: []ConfigSource{{Type: SourceFile, Path: "config.yaml"}},
		},
		{
			Data: map[string]interface{}{
				"value": "template-value",
			},
			Sources: []ConfigSource{{Type: SourceTemplate, Path: "template.yaml"}},
		},
	}

	result, err := merger.MergeConfigurations(configs)
	if err != nil {
		t.Fatalf("MergeConfigurations() unexpected error: %v", err)
	}

	// CLI should have highest precedence
	if result.Data["value"] != "cli-value" {
		t.Errorf("Expected CLI value to have highest precedence, got %v", result.Data["value"])
	}
}

// Helper functions

