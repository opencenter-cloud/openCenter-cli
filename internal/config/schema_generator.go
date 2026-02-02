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
	"fmt"
	"os"
	"reflect"

	"github.com/invopop/jsonschema"
)

// SchemaGenerator generates JSON schemas from Go struct definitions
type SchemaGenerator interface {
	// Generate creates a JSON schema for the specified version
	Generate(version string) (*jsonschema.Schema, error)

	// WriteToFile writes the generated schema to a file
	WriteToFile(schema *jsonschema.Schema, path string) error
}

// schemaGenerator implements SchemaGenerator interface
type schemaGenerator struct {
	reflector *jsonschema.Reflector
}

// NewSchemaGenerator creates a new schema generator
func NewSchemaGenerator() SchemaGenerator {
	reflector := &jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            false,
		ExpandedStruct:            true,
	}

	return &schemaGenerator{
		reflector: reflector,
	}
}

// Generate creates a JSON schema for the specified version
// In v2.0.0, only v2 schema generation is supported
func (g *schemaGenerator) Generate(version string) (*jsonschema.Schema, error) {
	// Only support v2 schema generation
	if version != "2.0" && version != "v2" && version != "v2.0" {
		return nil, fmt.Errorf("unsupported schema version: %s (only v2.0 is supported in v2.0.0)", version)
	}

	// Generate schema for v2 Config struct
	schema := g.reflector.Reflect(&Config{})

	// Add schema metadata
	schema.Version = "https://json-schema.org/draft/2020-12/schema"
	schema.Title = "OpenCenter Cluster Configuration Schema v2.0"
	schema.Description = "JSON schema for OpenCenter cluster configuration files (v2.0)"

	// Add custom properties
	if schema.Extras == nil {
		schema.Extras = make(map[string]any)
	}
	schema.Extras["$id"] = "https://opencenter.io/schemas/cluster-config-v2.json"
	schema.Extras["schemaVersion"] = "2.0"

	return schema, nil
}

// WriteToFile writes the generated schema to a file
func (g *schemaGenerator) WriteToFile(schema *jsonschema.Schema, path string) error {
	// Marshal schema to JSON with indentation
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	return nil
}

// GenerateSchemaFromStruct generates a JSON schema from a Go struct using reflection
func GenerateSchemaFromStruct(v interface{}) (*jsonschema.Schema, error) {
	reflector := &jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            false,
		ExpandedStruct:            true,
	}

	// Use reflection to get the type
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := reflector.Reflect(v)

	// Add schema metadata
	schema.Version = "https://json-schema.org/draft/2020-12/schema"
	schema.Title = fmt.Sprintf("%s Schema", t.Name())
	schema.Description = fmt.Sprintf("JSON schema for %s", t.Name())

	return schema, nil
}

// ValidateSchemaOutput validates that the generated schema is valid JSON schema
func ValidateSchemaOutput(schema *jsonschema.Schema) error {
	// Marshal to JSON to ensure it's valid
	data, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("schema is not valid JSON: %w", err)
	}

	// Unmarshal back to verify structure
	var test map[string]interface{}
	if err := json.Unmarshal(data, &test); err != nil {
		return fmt.Errorf("schema structure is invalid: %w", err)
	}

	// Check for required fields
	if schema.Version == "" {
		return fmt.Errorf("schema missing version field")
	}

	return nil
}
