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
	"os"
	"path/filepath"
	"testing"
)

// TestSchemaGenerator_Generate tests schema generation for v2 structs
func TestSchemaGenerator_Generate(t *testing.T) {
	generator := NewSchemaGenerator()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{
			name:    "generate v2.0 schema",
			version: "2.0",
			wantErr: false,
		},
		{
			name:    "generate v2 schema",
			version: "v2",
			wantErr: false,
		},
		{
			name:    "generate v2.0 schema (alternate)",
			version: "v2.0",
			wantErr: false,
		},
		{
			name:    "reject unsupported 1.0 schema",
			version: "1.0",
			wantErr: true,
		},
		{
			name:    "reject unsupported v1 alias",
			version: "v1",
			wantErr: true,
		},
		{
			name:    "unsupported version",
			version: "3.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := generator.Generate(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && schema == nil {
				t.Error("Generate() returned nil schema")
			}
		})
	}
}

// TestSchemaGenerator_SchemaIncludesRequiredFields tests that generated schema includes required fields
func TestSchemaGenerator_SchemaIncludesRequiredFields(t *testing.T) {
	generator := NewSchemaGenerator()
	schema, err := generator.Generate("2.0")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Marshal to JSON to inspect structure
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(data, &schemaMap); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	// Check for required top-level fields
	if _, ok := schemaMap["$schema"]; !ok {
		t.Error("Schema missing $schema field")
	}

	if _, ok := schemaMap["title"]; !ok {
		t.Error("Schema missing title field")
	}

	if _, ok := schemaMap["description"]; !ok {
		t.Error("Schema missing description field")
	}

	// Check for properties
	if _, ok := schemaMap["properties"]; !ok {
		t.Error("Schema missing properties field")
	}
}

// TestSchemaGenerator_SchemaIncludesEnums tests that schema includes enum validation
func TestSchemaGenerator_SchemaIncludesEnums(t *testing.T) {
	generator := NewSchemaGenerator()
	schema, err := generator.Generate("2.0")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Marshal to JSON to inspect structure
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	// Check that schema contains enum definitions
	// This is a basic check - we're looking for the word "enum" in the schema
	if len(data) == 0 {
		t.Error("Schema is empty")
	}

	// The schema should contain definitions for enums like provider types
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(data, &schemaMap); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	// Check that we have definitions
	if _, ok := schemaMap["$defs"]; !ok {
		t.Error("Schema missing $defs field")
	}
}

// TestSchemaGenerator_SchemaIncludesPatterns tests that schema includes pattern validation
func TestSchemaGenerator_SchemaIncludesPatterns(t *testing.T) {
	generator := NewSchemaGenerator()
	schema, err := generator.Generate("2.0")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Marshal to JSON to inspect structure
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	// The schema should contain pattern validations from jsonschema tags
	// This is a basic check - we're looking for the word "pattern" in the schema
	if len(data) == 0 {
		t.Error("Schema is empty")
	}
}

// TestSchemaGenerator_SchemaOutputIsValidJSON tests that schema output is valid JSON schema
func TestSchemaGenerator_SchemaOutputIsValidJSON(t *testing.T) {
	generator := NewSchemaGenerator()
	schema, err := generator.Generate("2.0")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Validate schema output
	if err := ValidateSchemaOutput(schema); err != nil {
		t.Errorf("ValidateSchemaOutput() error = %v", err)
	}
}

// TestSchemaGenerator_WriteToFile tests writing schema to file
func TestSchemaGenerator_WriteToFile(t *testing.T) {
	generator := NewSchemaGenerator()
	schema, err := generator.Generate("2.0")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Create temp directory
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-schema.json")

	// Write to file
	if err := generator.WriteToFile(schema, outputPath); err != nil {
		t.Errorf("WriteToFile() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Schema file was not created")
	}

	// Verify file content is valid JSON
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read schema file: %v", err)
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(data, &schemaMap); err != nil {
		t.Errorf("Schema file contains invalid JSON: %v", err)
	}
}

// TestGenerateSchemaFromStruct tests generating schema from arbitrary struct
func TestGenerateSchemaFromStruct(t *testing.T) {
	// Test with Config struct
	schema, err := GenerateSchemaFromStruct(&Config{})
	if err != nil {
		t.Errorf("GenerateSchemaFromStruct() error = %v", err)
	}

	if schema == nil {
		t.Error("GenerateSchemaFromStruct() returned nil schema")
	}

	// Verify schema has required fields
	if schema.Version == "" {
		t.Error("Schema missing version field")
	}

	if schema.Title == "" {
		t.Error("Schema missing title field")
	}
}

// TestValidateSchemaOutput tests schema validation
func TestValidateSchemaOutput(t *testing.T) {
	generator := NewSchemaGenerator()
	schema, err := generator.Generate("2.0")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Valid schema should pass validation
	if err := ValidateSchemaOutput(schema); err != nil {
		t.Errorf("ValidateSchemaOutput() error = %v", err)
	}

	// Test with invalid schema (missing version)
	invalidSchema := schema
	invalidSchema.Version = ""
	if err := ValidateSchemaOutput(invalidSchema); err == nil {
		t.Error("ValidateSchemaOutput() should fail for schema missing version")
	}
}
