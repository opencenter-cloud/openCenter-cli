package v2schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestGenerateRootSchemaMetadataAndStrictRoot(t *testing.T) {
	schema := generatedSchemaMap(t)

	if got := schema["$schema"]; got != "http://json-schema.org/draft-07/schema#" {
		t.Fatalf("$schema = %v, want Draft 7", got)
	}
	if got := schema["$id"]; got != "https://opencenter.cloud/schemas/opencenter-v2.schema.json" {
		t.Fatalf("$id = %v, want canonical v2 schema id", got)
	}
	if got := schema["additionalProperties"]; got != false {
		t.Fatalf("root additionalProperties = %v, want false", got)
	}

	required := stringSliceAt(t, schema, "required")
	for _, field := range []string{"schema_version", "opencenter", "opentofu", "secrets"} {
		if !slices.Contains(required, field) {
			t.Fatalf("root required fields missing %q: %v", field, required)
		}
	}

	version := schemaAt(t, schema, "properties", "schema_version")
	enum := stringSliceAt(t, version, "enum")
	if !slices.Equal(enum, []string{"2.0"}) {
		t.Fatalf("schema_version enum = %v, want [2.0]", enum)
	}
}

func TestGenerateValidationTagsForEditorHints(t *testing.T) {
	schema := generatedSchemaMap(t)

	provider := schemaAt(t, schema, "properties", "opencenter", "properties", "infrastructure", "properties", "provider")
	providerEnum := stringSliceAt(t, provider, "enum")
	for _, value := range []string{"openstack", "aws", "gcp", "azure", "baremetal", "vsphere", "vmware", "kind"} {
		if !slices.Contains(providerEnum, value) {
			t.Fatalf("provider enum missing %q: %v", value, providerEnum)
		}
	}

	vlanID := schemaAt(t, schema, "properties", "opencenter", "properties", "infrastructure", "properties", "networking", "properties", "vlan", "properties", "id")
	if got := vlanID["minimum"]; got != float64(1) {
		t.Fatalf("vlan id minimum = %v, want 1", got)
	}
	if got := vlanID["maximum"]; got != float64(4094) {
		t.Fatalf("vlan id maximum = %v, want 4094", got)
	}

	adminEmail := schemaAt(t, schema, "properties", "opencenter", "properties", "cluster", "properties", "admin_email")
	if got := adminEmail["format"]; got != "email" {
		t.Fatalf("admin_email format = %v, want email", got)
	}

	authURL := schemaAt(t, schema, "properties", "opencenter", "properties", "infrastructure", "properties", "cloud", "properties", "openstack", "properties", "auth_url")
	if got := authURL["format"]; got != "uri" {
		t.Fatalf("openstack auth_url format = %v, want uri", got)
	}
}

func TestGenerateRejectsLegacyOpenCenterTalosShape(t *testing.T) {
	schema := generatedSchemaMap(t)

	opencenterProps := schemaAt(t, schema, "properties", "opencenter", "properties")
	if _, ok := opencenterProps["talos"]; ok {
		t.Fatalf("schema should not expose legacy opencenter.talos: %v", opencenterProps["talos"])
	}
}

func TestGenerateHandlesPointersSlicesAndMaps(t *testing.T) {
	schema := generatedSchemaMap(t)

	openstack := schemaAt(t, schema, "properties", "opencenter", "properties", "infrastructure", "properties", "cloud", "properties", "openstack")
	if got := openstack["type"]; got != "object" {
		t.Fatalf("openstack type = %v, want object", got)
	}
	if got := openstack["additionalProperties"]; got != false {
		t.Fatalf("openstack additionalProperties = %v, want false", got)
	}

	nameservers := schemaAt(t, schema, "properties", "opencenter", "properties", "infrastructure", "properties", "networking", "properties", "dns_nameservers")
	if got := nameservers["type"]; got != "array" {
		t.Fatalf("dns_nameservers type = %v, want array", got)
	}
	items := schemaAt(t, nameservers, "items")
	if got := items["format"]; got != "ipv4" {
		t.Fatalf("dns_nameservers item format = %v, want ipv4", got)
	}

	labels := schemaAt(t, schema, "properties", "metadata", "properties", "labels")
	additional := schemaAt(t, labels, "additionalProperties")
	if got := additional["type"]; got != "string" {
		t.Fatalf("metadata.labels additional property type = %v, want string", got)
	}
}

func TestGenerateServiceMapIncludesRegisteredServicesAndAllowsCustomServices(t *testing.T) {
	schema := generatedSchemaMap(t)

	services := schemaAt(t, schema, "properties", "opencenter", "properties", "services")
	properties := schemaAt(t, services, "properties")
	if _, ok := properties["keycloak"]; !ok {
		t.Fatalf("services schema missing registered keycloak service; properties: %v", properties)
	}
	additional := schemaAt(t, services, "additionalProperties")
	if got := additional["type"]; got != "object" {
		t.Fatalf("services additionalProperties type = %v, want object", got)
	}
}

func TestCheckFileDetectsCurrentAndStaleSchema(t *testing.T) {
	current, err := Generate(Options{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "opencenter-v2.schema.json")
	if err := os.WriteFile(path, current, 0o644); err != nil {
		t.Fatalf("write current schema: %v", err)
	}
	if err := CheckFile(path, Options{}); err != nil {
		t.Fatalf("CheckFile() for current schema returned error: %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"stale": true}`), 0o644); err != nil {
		t.Fatalf("write stale schema: %v", err)
	}
	if err := CheckFile(path, Options{}); err == nil {
		t.Fatal("CheckFile() for stale schema returned nil error")
	}
}

func generatedSchemaMap(t *testing.T) map[string]any {
	t.Helper()

	data, err := Generate(Options{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema is not JSON: %v\n%s", err, string(data))
	}
	return schema
}

func schemaAt(t *testing.T, node map[string]any, path ...string) map[string]any {
	t.Helper()

	current := any(node)
	for _, segment := range path {
		mapped, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("path %v reached non-object %T", path, current)
		}
		var exists bool
		current, exists = mapped[segment]
		if !exists {
			t.Fatalf("path %v missing segment %q in %v", path, segment, mapped)
		}
	}
	mapped, ok := current.(map[string]any)
	if !ok {
		t.Fatalf("path %v = %T, want object", path, current)
	}
	return mapped
}

func stringSliceAt(t *testing.T, node map[string]any, key string) []string {
	t.Helper()

	raw, ok := node[key].([]any)
	if !ok {
		t.Fatalf("%s = %T, want []any", key, node[key])
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		value, ok := item.(string)
		if !ok {
			t.Fatalf("%s item = %T, want string", key, item)
		}
		values = append(values, value)
	}
	return values
}
