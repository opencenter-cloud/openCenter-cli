package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

func main() {
	// Generate schema from the Config struct using reflection
	cfg := config.Config{}
	schema := generateSchemaFromStruct(reflect.TypeOf(cfg), "Config")
	
	// Add schema metadata
	fullSchema := map[string]interface{}{
		"$id":         "https://opencenter.cloud/schemas/cluster-config.json",
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"title":       "openCenter Cluster Configuration",
		"description": "Complete schema for openCenter cluster configuration with validation rules and constraints",
		"type":        "object",
		"properties":  schema["properties"],
		"required":    schema["required"],
	}
	
	// Pretty print JSON
	output, err := json.MarshalIndent(fullSchema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling schema: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println(string(output))
}

func generateSchemaFromStruct(t reflect.Type, name string) map[string]interface{} {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	
	if t.Kind() != reflect.Struct {
		return map[string]interface{}{
			"type": getJSONType(t),
		}
	}
	
	properties := make(map[string]interface{})
	var required []string
	
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		
		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		
		// Parse JSON tag
		jsonName := field.Name
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				jsonName = parts[0]
			}
			
			// Check if field is required (no omitempty)
			isRequired := true
			for _, part := range parts[1:] {
				if part == "omitempty" {
					isRequired = false
					break
				}
			}
			if isRequired && !strings.Contains(jsonName, "omitempty") {
				required = append(required, jsonName)
			}
		}
		
		// Generate field schema
		fieldSchema := generateFieldSchema(field)
		properties[jsonName] = fieldSchema
	}
	
	result := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	
	if len(required) > 0 {
		result["required"] = required
	}
	
	return result
}

func generateFieldSchema(field reflect.StructField) map[string]interface{} {
	fieldType := field.Type
	
	// Handle pointers
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	
	schema := make(map[string]interface{})
	
	// Parse jsonschema tag for additional metadata
	jsonSchemaTag := field.Tag.Get("jsonschema")
	if jsonSchemaTag != "" {
		parseJSONSchemaTag(jsonSchemaTag, schema)
	}
	
	// Set type based on Go type
	switch fieldType.Kind() {
	case reflect.String:
		schema["type"] = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		schema["type"] = "number"
	case reflect.Bool:
		schema["type"] = "boolean"
	case reflect.Slice, reflect.Array:
		schema["type"] = "array"
		elemType := fieldType.Elem()
		if elemType.Kind() == reflect.String {
			schema["items"] = map[string]interface{}{"type": "string"}
		} else if elemType.Kind() == reflect.Struct {
			schema["items"] = generateSchemaFromStruct(elemType, elemType.Name())
		}
	case reflect.Map:
		schema["type"] = "object"
		if fieldType.Elem().Kind() == reflect.Interface {
			schema["additionalProperties"] = true
		}
	case reflect.Struct:
		// For nested structs, generate their schema
		nestedSchema := generateSchemaFromStruct(fieldType, fieldType.Name())
		for k, v := range nestedSchema {
			schema[k] = v
		}
	default:
		schema["type"] = "string" // fallback
	}
	
	return schema
}

func parseJSONSchemaTag(tag string, schema map[string]interface{}) {
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			
			switch key {
			case "description":
				schema["description"] = value
			case "default":
				schema["default"] = value
			case "minLength":
				if val := parseInt(value); val != nil {
					schema["minLength"] = *val
				}
			case "maxLength":
				if val := parseInt(value); val != nil {
					schema["maxLength"] = *val
				}
			case "minimum":
				if val := parseInt(value); val != nil {
					schema["minimum"] = *val
				}
			case "maximum":
				if val := parseInt(value); val != nil {
					schema["maximum"] = *val
				}
			case "pattern":
				schema["pattern"] = value
			case "format":
				schema["format"] = value
			case "enum":
				// Handle enum values
				enumValues := strings.Split(value, "|")
				schema["enum"] = enumValues
			}
		} else if part == "secret=true" {
			schema["description"] = addSecretNote(schema["description"])
		}
	}
}

func addSecretNote(desc interface{}) string {
	if desc == nil {
		return "Secret field - handle with care"
	}
	return fmt.Sprintf("%s (Secret field)", desc)
}

func parseInt(s string) *int {
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err == nil {
		return &result
	}
	return nil
}

func getJSONType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}