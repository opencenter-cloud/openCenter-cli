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

func TestArrayFlagHandler_ParseFlag(t *testing.T) {
	handler := NewArrayFlagHandler()
	
	tests := []struct {
		name      string
		flagName  string
		flagValue string
		wantErr   bool
		wantOp    ArrayOperation
		wantPath  string
		wantIndex int
		wantValue interface{}
	}{
		{
			name:      "array append operation",
			flagName:  "array-append",
			flagValue: "servers=web-server",
			wantErr:   false,
			wantOp:    ArrayOpAppend,
			wantPath:  "servers",
			wantIndex: -1,
			wantValue: "web-server",
		},
		{
			name:      "array insert operation",
			flagName:  "array-insert",
			flagValue: "servers[1]=database-server",
			wantErr:   false,
			wantOp:    ArrayOpInsert,
			wantPath:  "servers",
			wantIndex: 1,
			wantValue: "database-server",
		},
		{
			name:      "array remove operation",
			flagName:  "array-remove",
			flagValue: "servers=old-server",
			wantErr:   false,
			wantOp:    ArrayOpRemove,
			wantPath:  "servers",
			wantIndex: -1,
			wantValue: "old-server",
		},
		{
			name:      "nested path append",
			flagName:  "array-append",
			flagValue: "cluster.nodes=node1",
			wantErr:   false,
			wantOp:    ArrayOpAppend,
			wantPath:  "cluster.nodes",
			wantIndex: -1,
			wantValue: "node1",
		},
		{
			name:      "integer value",
			flagName:  "array-append",
			flagValue: "ports=8080",
			wantErr:   false,
			wantOp:    ArrayOpAppend,
			wantPath:  "ports",
			wantIndex: -1,
			wantValue: 8080,
		},
		{
			name:      "boolean value",
			flagName:  "array-append",
			flagValue: "flags=true",
			wantErr:   false,
			wantOp:    ArrayOpAppend,
			wantPath:  "flags",
			wantIndex: -1,
			wantValue: true,
		},
		{
			name:      "invalid flag name",
			flagName:  "invalid-flag",
			flagValue: "path=value",
			wantErr:   true,
		},
		{
			name:      "invalid append format",
			flagName:  "array-append",
			flagValue: "invalid-format",
			wantErr:   true,
		},
		{
			name:      "invalid insert format - no brackets",
			flagName:  "array-insert",
			flagValue: "path=value",
			wantErr:   true,
		},
		{
			name:      "invalid insert format - bad index",
			flagName:  "array-insert",
			flagValue: "path[abc]=value",
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
			
			arrayFlag, ok := result.(*ArrayOperationFlag)
			if !ok {
				t.Errorf("ParseFlag() returned %T, expected *ArrayOperationFlag", result)
				return
			}
			
			if arrayFlag.Operation != tt.wantOp {
				t.Errorf("ParseFlag() operation = %v, want %v", arrayFlag.Operation, tt.wantOp)
			}
			
			if arrayFlag.Path != tt.wantPath {
				t.Errorf("ParseFlag() path = %v, want %v", arrayFlag.Path, tt.wantPath)
			}
			
			if arrayFlag.Index != tt.wantIndex {
				t.Errorf("ParseFlag() index = %v, want %v", arrayFlag.Index, tt.wantIndex)
			}
			
			if arrayFlag.Value != tt.wantValue {
				t.Errorf("ParseFlag() value = %v, want %v", arrayFlag.Value, tt.wantValue)
			}
		})
	}
}

func TestArrayFlagHandler_MergeIntoConfiguration(t *testing.T) {
	handler := NewArrayFlagHandler()
	
	tests := []struct {
		name       string
		config     map[string]interface{}
		flag       *ArrayOperationFlag
		wantConfig map[string]interface{}
		wantErr    bool
	}{
		{
			name:   "append to existing array",
			config: map[string]interface{}{"servers": []interface{}{"server1", "server2"}},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpAppend,
				Path:      "servers",
				Value:     "server3",
			},
			wantConfig: map[string]interface{}{"servers": []interface{}{"server1", "server2", "server3"}},
			wantErr:    false,
		},
		{
			name:   "append to new array",
			config: map[string]interface{}{},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpAppend,
				Path:      "servers",
				Value:     "server1",
			},
			wantConfig: map[string]interface{}{"servers": []interface{}{"server1"}},
			wantErr:    false,
		},
		{
			name:   "insert at beginning",
			config: map[string]interface{}{"servers": []interface{}{"server2", "server3"}},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpInsert,
				Path:      "servers",
				Index:     0,
				Value:     "server1",
			},
			wantConfig: map[string]interface{}{"servers": []interface{}{"server1", "server2", "server3"}},
			wantErr:    false,
		},
		{
			name:   "insert at middle",
			config: map[string]interface{}{"servers": []interface{}{"server1", "server3"}},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpInsert,
				Path:      "servers",
				Index:     1,
				Value:     "server2",
			},
			wantConfig: map[string]interface{}{"servers": []interface{}{"server1", "server2", "server3"}},
			wantErr:    false,
		},
		{
			name:   "remove existing value",
			config: map[string]interface{}{"servers": []interface{}{"server1", "server2", "server3"}},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpRemove,
				Path:      "servers",
				Value:     "server2",
			},
			wantConfig: map[string]interface{}{"servers": []interface{}{"server1", "server3"}},
			wantErr:    false,
		},
		{
			name:   "remove non-existing value",
			config: map[string]interface{}{"servers": []interface{}{"server1", "server2"}},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpRemove,
				Path:      "servers",
				Value:     "server3",
			},
			wantConfig: map[string]interface{}{"servers": []interface{}{"server1", "server2"}},
			wantErr:    false,
		},
		{
			name:   "nested path creation",
			config: map[string]interface{}{},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpAppend,
				Path:      "cluster.nodes",
				Value:     "node1",
			},
			wantConfig: map[string]interface{}{
				"cluster": map[string]interface{}{
					"nodes": []interface{}{"node1"},
				},
			},
			wantErr: false,
		},
		{
			name:   "insert out of bounds",
			config: map[string]interface{}{"servers": []interface{}{"server1"}},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpInsert,
				Path:      "servers",
				Index:     5,
				Value:     "server2",
			},
			wantErr: true,
		},
		{
			name:   "path conflict - not an array",
			config: map[string]interface{}{"servers": "not-an-array"},
			flag: &ArrayOperationFlag{
				Operation: ArrayOpAppend,
				Path:      "servers",
				Value:     "server1",
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