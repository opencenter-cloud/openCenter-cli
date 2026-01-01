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

func TestMapFlagHandler_ParseFlag(t *testing.T) {
	handler := NewMapFlagHandler()
	
	tests := []struct {
		name      string
		flagName  string
		flagValue string
		wantErr   bool
		wantOp    MapOperation
		wantPath  string
		wantKey   string
		wantValue interface{}
	}{
		{
			name:      "map set operation",
			flagName:  "map-set",
			flagValue: "config.name=test-cluster",
			wantErr:   false,
			wantOp:    MapOpSet,
			wantPath:  "config",
			wantKey:   "name",
			wantValue: "test-cluster",
		},
		{
			name:      "map set with JSON value",
			flagName:  "map-set",
			flagValue: "config.metadata={\"version\": \"1.0\"}",
			wantErr:   false,
			wantOp:    MapOpSet,
			wantPath:  "config",
			wantKey:   "metadata",
			wantValue: map[string]interface{}{"version": "1.0"},
		},
		{
			name:      "map merge operation",
			flagName:  "map-merge",
			flagValue: "config={\"name\": \"test\", \"version\": \"1.0\"}",
			wantErr:   false,
			wantOp:    MapOpMerge,
			wantPath:  "config",
			wantKey:   "",
			wantValue: map[string]interface{}{"name": "test", "version": "1.0"},
		},
		{
			name:      "map remove operation",
			flagName:  "map-remove",
			flagValue: "config.old_field",
			wantErr:   false,
			wantOp:    MapOpRemove,
			wantPath:  "config",
			wantKey:   "old_field",
			wantValue: nil,
		},
		{
			name:      "nested path set",
			flagName:  "map-set",
			flagValue: "cluster.infrastructure.provider=openstack",
			wantErr:   false,
			wantOp:    MapOpSet,
			wantPath:  "cluster.infrastructure",
			wantKey:   "provider",
			wantValue: "openstack",
		},
		{
			name:      "integer value",
			flagName:  "map-set",
			flagValue: "config.port=8080",
			wantErr:   false,
			wantOp:    MapOpSet,
			wantPath:  "config",
			wantKey:   "port",
			wantValue: float64(8080), // JSON numbers are float64
		},
		{
			name:      "boolean value",
			flagName:  "map-set",
			flagValue: "config.enabled=true",
			wantErr:   false,
			wantOp:    MapOpSet,
			wantPath:  "config",
			wantKey:   "enabled",
			wantValue: true,
		},
		{
			name:      "invalid flag name",
			flagName:  "invalid-flag",
			flagValue: "path.key=value",
			wantErr:   true,
		},
		{
			name:      "invalid set format - no equals",
			flagName:  "map-set",
			flagValue: "path.key",
			wantErr:   true,
		},
		{
			name:      "invalid set format - no dot",
			flagName:  "map-set",
			flagValue: "pathkey=value",
			wantErr:   true,
		},
		{
			name:      "invalid merge format - bad JSON",
			flagName:  "map-merge",
			flagValue: "path={invalid json}",
			wantErr:   true,
		},
		{
			name:      "invalid remove format - no dot",
			flagName:  "map-remove",
			flagValue: "pathkey",
			wantErr:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.ParseFlag(tt.flagName, tt.flagValue)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseFlag() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("ParseFlag() unexpected error: %v", err)
				return
			}
			
			mapFlag, ok := result.(*MapFlag)
			if !ok {
				t.Errorf("ParseFlag() returned %T, expected *MapFlag", result)
				return
			}
			
			if mapFlag.Operation != tt.wantOp {
				t.Errorf("ParseFlag() operation = %v, want %v", mapFlag.Operation, tt.wantOp)
			}
			
			if mapFlag.Path != tt.wantPath {
				t.Errorf("ParseFlag() path = %v, want %v", mapFlag.Path, tt.wantPath)
			}
			
			if mapFlag.Key != tt.wantKey {
				t.Errorf("ParseFlag() key = %v, want %v", mapFlag.Key, tt.wantKey)
			}
			
			if !compareValues(mapFlag.Value, tt.wantValue) {
				t.Errorf("ParseFlag() value = %v, want %v", mapFlag.Value, tt.wantValue)
			}
		})
	}
}

func TestMapFlagHandler_MergeIntoConfiguration(t *testing.T) {
	handler := NewMapFlagHandler()
	
	tests := []struct {
		name       string
		config     map[string]interface{}
		flag       *MapFlag
		wantConfig map[string]interface{}
		wantErr    bool
	}{
		{
			name: "set key in existing map",
			config: map[string]interface{}{
				"config": map[string]interface{}{
					"existing": "value",
				},
			},
			flag: &MapFlag{
				Operation: MapOpSet,
				Path:      "config",
				Key:       "name",
				Value:     "test-cluster",
			},
			wantConfig: map[string]interface{}{
				"config": map[string]interface{}{
					"existing": "value",
					"name":     "test-cluster",
				},
			},
			wantErr: false,
		},
		{
			name:   "set key in new map",
			config: map[string]interface{}{},
			flag: &MapFlag{
				Operation: MapOpSet,
				Path:      "config",
				Key:       "name",
				Value:     "test-cluster",
			},
			wantConfig: map[string]interface{}{
				"config": map[string]interface{}{
					"name": "test-cluster",
				},
			},
			wantErr: false,
		},
		{
			name: "merge into existing map",
			config: map[string]interface{}{
				"config": map[string]interface{}{
					"existing": "value",
				},
			},
			flag: &MapFlag{
				Operation: MapOpMerge,
				Path:      "config",
				Value: map[string]interface{}{
					"name":    "test-cluster",
					"version": "1.0",
				},
			},
			wantConfig: map[string]interface{}{
				"config": map[string]interface{}{
					"existing": "value",
					"name":     "test-cluster",
					"version":  "1.0",
				},
			},
			wantErr: false,
		},
		{
			name: "remove key from map",
			config: map[string]interface{}{
				"config": map[string]interface{}{
					"name":     "test-cluster",
					"old_key":  "old_value",
					"version":  "1.0",
				},
			},
			flag: &MapFlag{
				Operation: MapOpRemove,
				Path:      "config",
				Key:       "old_key",
			},
			wantConfig: map[string]interface{}{
				"config": map[string]interface{}{
					"name":    "test-cluster",
					"version": "1.0",
				},
			},
			wantErr: false,
		},
		{
			name:   "nested path creation",
			config: map[string]interface{}{},
			flag: &MapFlag{
				Operation: MapOpSet,
				Path:      "cluster.infrastructure",
				Key:       "provider",
				Value:     "openstack",
			},
			wantConfig: map[string]interface{}{
				"cluster": map[string]interface{}{
					"infrastructure": map[string]interface{}{
						"provider": "openstack",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "merge with non-map value",
			config: map[string]interface{}{
				"config": map[string]interface{}{
					"existing": "value",
				},
			},
			flag: &MapFlag{
				Operation: MapOpMerge,
				Path:      "config",
				Value:     "not-a-map",
			},
			wantErr: true,
		},
		{
			name: "path conflict - not a map",
			config: map[string]interface{}{
				"config": "not-a-map",
			},
			flag: &MapFlag{
				Operation: MapOpSet,
				Path:      "config",
				Key:       "name",
				Value:     "test",
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.MergeIntoConfiguration(tt.flag, tt.config)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("MergeIntoConfiguration() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("MergeIntoConfiguration() unexpected error: %v", err)
				return
			}
			
			// Compare the resulting configuration
			if !compareConfigValues(tt.config, tt.wantConfig) {
				t.Errorf("MergeIntoConfiguration() config = %v, want %v", tt.config, tt.wantConfig)
			}
		})
	}
}