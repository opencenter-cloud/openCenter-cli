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

package config

import (
	"testing"
	
	"gopkg.in/yaml.v3"
)

// TestAdminEmailDefault verifies that admin_email has a default value
func TestAdminEmailDefault(t *testing.T) {
	cfg := NewDefault("test-cluster")
	
	if cfg.OpenCenter.Cluster.AdminEmail == "" {
		t.Error("AdminEmail should not be empty - it should have a default value")
	}
	
	if cfg.OpenCenter.Cluster.AdminEmail != "admin@example.com" {
		t.Errorf("expected AdminEmail 'admin@example.com', got '%s'", cfg.OpenCenter.Cluster.AdminEmail)
	}
}

// TestAdminEmailInSchemaDefaults verifies that admin_email is included in schema-generated defaults
func TestAdminEmailInSchemaDefaults(t *testing.T) {
	yamlBytes, err := GenerateDefaultFromSchema("test-cluster")
	if err != nil {
		t.Fatalf("GenerateDefaultFromSchema failed: %v", err)
	}
	
	// Parse the YAML
	var cfg Config
	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		t.Fatalf("failed to unmarshal schema defaults: %v", err)
	}
	
	if cfg.OpenCenter.Cluster.AdminEmail == "" {
		t.Error("AdminEmail should not be empty in schema-generated defaults")
	}
	
	if cfg.OpenCenter.Cluster.AdminEmail != "admin@example.com" {
		t.Errorf("expected AdminEmail 'admin@example.com' in schema defaults, got '%s'", cfg.OpenCenter.Cluster.AdminEmail)
	}
}

// TestAdminEmailInFullSchemaDefaults verifies that admin_email is included in full schema defaults
func TestAdminEmailInFullSchemaDefaults(t *testing.T) {
	yamlBytes, err := GenerateFullSchemaDefaults("test-cluster")
	if err != nil {
		t.Fatalf("GenerateFullSchemaDefaults failed: %v", err)
	}
	
	// Parse the YAML
	var cfg Config
	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		t.Fatalf("failed to unmarshal full schema defaults: %v", err)
	}
	
	if cfg.OpenCenter.Cluster.AdminEmail == "" {
		t.Error("AdminEmail should not be empty in full schema defaults")
	}
	
	if cfg.OpenCenter.Cluster.AdminEmail != "admin@example.com" {
		t.Errorf("expected AdminEmail 'admin@example.com' in full schema defaults, got '%s'", cfg.OpenCenter.Cluster.AdminEmail)
	}
}
