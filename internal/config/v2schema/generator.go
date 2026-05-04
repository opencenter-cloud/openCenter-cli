package v2schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	_ "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const (
	defaultSchemaID = "https://opencenter.cloud/schemas/opencenter-v2.schema.json"
	draft7Schema    = "http://json-schema.org/draft-07/schema#"
)

// Options configures v2 JSON Schema generation.
type Options struct {
	ID string
}

// Generate returns an editor-oriented JSON Schema for native v2 cluster config.
func Generate(opts Options) ([]byte, error) {
	id := strings.TrimSpace(opts.ID)
	if id == "" {
		id = defaultSchemaID
	}

	g := generator{
		serviceMapType: reflect.TypeOf(v2.ServiceMap{}),
	}
	schema := g.schemaFor(reflect.TypeOf(v2.Config{}), "")
	schema["$schema"] = draft7Schema
	schema["$id"] = id
	schema["title"] = "openCenter v2 Cluster Configuration"
	schema["description"] = "Editor schema for schema_version 2.0 openCenter cluster configuration. The opencenter CLI remains authoritative for defaults, references, readiness checks, and cross-field validation."

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal v2 schema: %w", err)
	}
	return append(data, '\n'), nil
}

// CheckFile compares the generated v2 schema with an existing file.
func CheckFile(path string, opts Options) error {
	want, err := Generate(opts)
	if err != nil {
		return err
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema file %s: %w", path, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("schema file %s is stale; run opencenter config ide --schema-only", path)
	}
	return nil
}

type generator struct {
	serviceMapType reflect.Type
}

func (g generator) schemaFor(t reflect.Type, validateTag string) map[string]any {
	t = concreteType(t)
	if t == g.serviceMapType {
		return g.serviceMapSchema()
	}

	var schema map[string]any
	switch t.Kind() {
	case reflect.Struct:
		schema = g.structSchema(t)
	case reflect.Map:
		schema = g.mapSchema(t)
	case reflect.Slice, reflect.Array:
		schema = map[string]any{
			"type":  "array",
			"items": g.schemaFor(t.Elem(), itemValidateTag(validateTag)),
		}
		applyValidation(schema, parentValidateTag(validateTag))
		return schema
	case reflect.Bool:
		schema = map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema = map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		schema = map[string]any{"type": "number"}
	case reflect.String:
		schema = map[string]any{"type": "string"}
	case reflect.Interface:
		schema = map[string]any{}
	default:
		schema = map[string]any{}
	}

	applyValidation(schema, validateTag)
	return schema
}

func (g generator) structSchema(t reflect.Type) map[string]any {
	properties := map[string]any{}
	requiredSet := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name, inline, skip := yamlFieldName(field)
		if skip {
			continue
		}

		fieldType := concreteType(field.Type)
		if inline {
			inlined := g.schemaFor(fieldType, field.Tag.Get("validate"))
			if inlinedProps, ok := inlined["properties"].(map[string]any); ok {
				for key, value := range inlinedProps {
					properties[key] = value
				}
			}
			if inlinedRequired, ok := inlined["required"].([]string); ok {
				for _, key := range inlinedRequired {
					requiredSet[key] = true
				}
			}
			continue
		}

		properties[name] = g.schemaFor(field.Type, field.Tag.Get("validate"))
		if hasRequiredValidator(field.Tag.Get("validate")) {
			requiredSet[name] = true
		}
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(requiredSet) > 0 {
		required := make([]string, 0, len(requiredSet))
		for field := range requiredSet {
			required = append(required, field)
		}
		sort.Strings(required)
		schema["required"] = required
	}
	return schema
}

func (g generator) mapSchema(t reflect.Type) map[string]any {
	if t.Key().Kind() != reflect.String {
		return map[string]any{"type": "object"}
	}
	valueType := concreteType(t.Elem())
	if valueType.Kind() == reflect.Interface {
		return map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": g.schemaFor(t.Elem(), ""),
	}
}

func (g generator) serviceMapSchema() map[string]any {
	properties := map[string]any{}
	services := registry.GetRegisteredServices()
	sort.Strings(services)
	for _, name := range services {
		serviceType := registry.GetServiceConfigType(name)
		if serviceType == nil {
			continue
		}
		properties[name] = g.schemaFor(serviceType, "")
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"additionalProperties": map[string]any{
			"type": "object",
		},
	}
}

func concreteType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func yamlFieldName(field reflect.StructField) (name string, inline bool, skip bool) {
	if field.Tag.Get("json") == "-" {
		return "", false, true
	}
	tag := field.Tag.Get("yaml")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	if len(parts) > 0 && parts[0] != "" {
		name = parts[0]
	} else {
		name = lowerCamelToSnake(field.Name)
	}
	for _, part := range parts[1:] {
		switch part {
		case "inline":
			inline = true
		}
	}
	return name, inline, false
}

func lowerCamelToSnake(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func hasRequiredValidator(tag string) bool {
	for _, token := range strings.Split(tag, ",") {
		if token == "required" {
			return true
		}
	}
	return false
}

func itemValidateTag(tag string) string {
	parts := strings.Split(tag, ",")
	for i, part := range parts {
		if part == "dive" && i+1 < len(parts) {
			return strings.Join(parts[i+1:], ",")
		}
	}
	return ""
}

func parentValidateTag(tag string) string {
	parts := strings.Split(tag, ",")
	for i, part := range parts {
		if part == "dive" {
			return strings.Join(parts[:i], ",")
		}
	}
	return tag
}

func applyValidation(schema map[string]any, tag string) {
	if tag == "" {
		return
	}

	var retained []string
	for _, token := range strings.Split(tag, ",") {
		if token == "" || token == "omitempty" || token == "required" || token == "dive" {
			continue
		}
		switch {
		case strings.HasPrefix(token, "eq="):
			schema["enum"] = []string{strings.TrimPrefix(token, "eq=")}
		case strings.HasPrefix(token, "oneof="):
			schema["enum"] = strings.Fields(strings.TrimPrefix(token, "oneof="))
		case strings.HasPrefix(token, "min="):
			applyBound(schema, "min", strings.TrimPrefix(token, "min="))
		case strings.HasPrefix(token, "max="):
			applyBound(schema, "max", strings.TrimPrefix(token, "max="))
		case token == "email":
			schema["format"] = "email"
		case token == "url":
			schema["format"] = "uri"
		case token == "hostname" || token == "fqdn":
			schema["format"] = "hostname"
		case token == "ip":
			schema["format"] = "ip"
		case token == "ipv4":
			schema["format"] = "ipv4"
		case token == "cidrv4":
			schema["pattern"] = `^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`
		case token == "semver":
			schema["pattern"] = `^v?[0-9]+(\.[0-9]+){1,2}([-+][0-9A-Za-z.-]+)?$`
		case token == "dns1123":
			schema["pattern"] = `^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
		default:
			retained = append(retained, token)
		}
	}
	if len(retained) > 0 {
		schema["x-opencenter-validation"] = retained
	}
}

func applyBound(schema map[string]any, kind, raw string) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return
	}
	switch schema["type"] {
	case "integer", "number":
		if kind == "min" {
			schema["minimum"] = value
		} else {
			schema["maximum"] = value
		}
	case "array":
		if kind == "min" {
			schema["minItems"] = value
		} else {
			schema["maxItems"] = value
		}
	case "string":
		if kind == "min" {
			schema["minLength"] = value
		} else {
			schema["maxLength"] = value
		}
	}
}
