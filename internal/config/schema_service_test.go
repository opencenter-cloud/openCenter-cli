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
	"encoding/json"
	"testing"
)

// TestLokiSchema tests that the loki schema includes all loki-specific fields
func TestLokiSchema(t *testing.T) {
	// Generate the schema
	schemaBytes, err := GenerateSchema(false)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	// Navigate to services.loki
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing properties field")
	}

	opencenter, ok := properties["opencenter"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter field")
	}

	opencenterProps, ok := opencenter["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.properties field")
	}

	services, ok := opencenterProps["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.services field")
	}

	servicesProps, ok := services["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.services.properties field")
	}

	loki, ok := servicesProps["loki"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing loki service")
	}

	lokiProps, ok := loki["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Loki schema missing properties field")
	}

	// Test that loki-specific fields are present
	requiredFields := []string{
		"enabled",
		"status",
		"release",
		"branch",
		"uri",
		"loki_storage_type",
		"loki_bucket_name",
		"loki_volume_size",
		"loki_storage_class",
		"swift_auth_url",
		"swift_region",
		"swift_auth_version",
		"swift_application_credential_id",
		"swift_container_name",
		"swift_user_domain_name",
		"loki_s3_endpoint",
		"loki_s3_region",
		"loki_s3_force_path_style",
		"loki_s3_insecure",
	}

	for _, field := range requiredFields {
		if _, ok := lokiProps[field]; !ok {
			t.Errorf("Loki schema missing field: %s", field)
		}
	}

	// Verify storage_type has correct enum values
	storageType, ok := lokiProps["loki_storage_type"].(map[string]interface{})
	if !ok {
		t.Fatal("loki_storage_type field is not an object")
	}

	enum, ok := storageType["enum"].([]interface{})
	if !ok {
		t.Fatal("loki_storage_type missing enum field")
	}

	expectedEnums := []string{"s3", "swift"}
	if len(enum) != len(expectedEnums) {
		t.Errorf("loki_storage_type enum has %d values, expected %d", len(enum), len(expectedEnums))
	}

	for _, expected := range expectedEnums {
		found := false
		for _, val := range enum {
			if val.(string) == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("loki_storage_type enum missing value: %s", expected)
		}
	}
}

// TestVeleroSchema tests that the velero schema includes all velero-specific fields
func TestVeleroSchema(t *testing.T) {
	// Generate the schema
	schemaBytes, err := GenerateSchema(false)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	// Navigate to services.velero
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing properties field")
	}

	opencenter, ok := properties["opencenter"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter field")
	}

	opencenterProps, ok := opencenter["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.properties field")
	}

	services, ok := opencenterProps["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.services field")
	}

	servicesProps, ok := services["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.services.properties field")
	}

	velero, ok := servicesProps["velero"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing velero service")
	}

	veleroProps, ok := velero["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Velero schema missing properties field")
	}

	// Test that velero-specific fields are present
	requiredFields := []string{
		"enabled",
		"status",
		"release",
		"branch",
		"uri",
		"velero_backup_bucket",
		"velero_region",
		"storage_type",
	}

	for _, field := range requiredFields {
		if _, ok := veleroProps[field]; !ok {
			t.Errorf("Velero schema missing field: %s", field)
		}
	}

	// Verify storage_type has correct enum values
	storageType, ok := veleroProps["storage_type"].(map[string]interface{})
	if !ok {
		t.Fatal("storage_type field is not an object")
	}

	enum, ok := storageType["enum"].([]interface{})
	if !ok {
		t.Fatal("storage_type missing enum field")
	}

	expectedEnums := []string{"s3", "swift", "gcs", "azure"}
	if len(enum) != len(expectedEnums) {
		t.Errorf("storage_type enum has %d values, expected %d", len(enum), len(expectedEnums))
	}

	for _, expected := range expectedEnums {
		found := false
		for _, val := range enum {
			if val.(string) == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("storage_type enum missing value: %s", expected)
		}
	}
}

// TestServiceSchemasNotUsingBaseSchema tests that loki and velero don't use baseServiceSchema
func TestServiceSchemasNotUsingBaseSchema(t *testing.T) {
	// Generate the schema
	schemaBytes, err := GenerateSchema(false)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	// Navigate to services
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing properties field")
	}

	opencenter, ok := properties["opencenter"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter field")
	}

	opencenterProps, ok := opencenter["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.properties field")
	}

	services, ok := opencenterProps["services"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.services field")
	}

	servicesProps, ok := services["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing opencenter.services.properties field")
	}

	// Check that loki has more than just base fields
	loki, ok := servicesProps["loki"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing loki service")
	}

	lokiProps, ok := loki["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Loki schema missing properties field")
	}

	// Base schema has 5 fields (enabled, status, release, branch, uri)
	// Loki should have significantly more
	if len(lokiProps) <= 5 {
		t.Errorf("Loki schema has only %d fields, expected more than 5 (appears to be using baseServiceSchema)", len(lokiProps))
	}

	// Check that velero has more than just base fields
	velero, ok := servicesProps["velero"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing velero service")
	}

	veleroProps, ok := velero["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Velero schema missing properties field")
	}

	// Velero should have more than base fields
	if len(veleroProps) <= 5 {
		t.Errorf("Velero schema has only %d fields, expected more than 5 (appears to be using baseServiceSchema)", len(veleroProps))
	}
}
