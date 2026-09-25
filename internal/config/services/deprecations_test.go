package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDeprecatedServiceConfigKeysAreOrderedAndLookedUp(t *testing.T) {
	entries := DeprecatedServiceConfigKeys()
	if len(entries) == 0 {
		t.Fatal("deprecation registry is empty")
	}
	for i, entry := range entries {
		if entry.Key == "" || entry.Reason == "" || entry.Guidance == "" {
			t.Fatalf("registry entry %d is incomplete: %+v", i, entry)
		}
		got, ok := LookupDeprecatedServiceConfigKey(entry.Key)
		if !ok || got != entry {
			t.Fatalf("lookup(%q) = %+v, %v; want %+v, true", entry.Key, got, ok, entry)
		}
	}
}

func TestDeprecatedConfigWarningsOnlyFindExplicitUserKeys(t *testing.T) {
	data := []byte(`schema_version: "2.0"
opencenter:
  services:
    metallb:
      custom_resources: [secret.yaml]
      edition: enterprise
      enterprise_registry: true
      extra_dependencies: [network]
      conditional_dependencies: [{name: network, when_enabled: cni}]
      override_values: generated
  managed_services:
    olm:
      kustomization_content: generated
`)
	warnings := DeprecatedConfigWarnings(data)
	if len(warnings) != 7 {
		t.Fatalf("got %d warnings, want 7: %+v", len(warnings), warnings)
	}
	wantPaths := []string{
		"opencenter.services.metallb.edition",
		"opencenter.services.metallb.enterprise_registry",
		"opencenter.services.metallb.custom_resources",
		"opencenter.services.metallb.extra_dependencies",
		"opencenter.services.metallb.conditional_dependencies",
		"opencenter.services.metallb.override_values",
		"opencenter.managed_services.olm.kustomization_content",
	}
	for i, want := range wantPaths {
		if warnings[i].Path != want {
			t.Fatalf("warning[%d].Path = %q, want %q", i, warnings[i].Path, want)
		}
	}
}

func TestBuiltInDefaultsAloneProduceNoDeprecatedWarnings(t *testing.T) {
	data := []byte(`schema_version: "2.0"
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
    olm:
      enabled: true
`)
	if warnings := DeprecatedConfigWarnings(data); len(warnings) != 0 {
		t.Fatalf("built-in defaults must not produce warnings: %+v", warnings)
	}
}

func TestDeprecatedRegistryMatchesCheckedInSchema(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(sourceFile), "../../../schema/opencenter-v2.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checked-in schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode checked-in schema: %v", err)
	}
	for _, entry := range DeprecatedServiceConfigKeys() {
		if schemaContainsKey(schema, entry.Key) {
			t.Errorf("removed legacy key %q still occurs in checked-in schema", entry.Key)
		}
	}
}

func schemaContainsKey(node any, key string) bool {
	switch value := node.(type) {
	case map[string]any:
		if _, ok := value[key]; ok {
			return true
		}
		for _, child := range value {
			if schemaContainsKey(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if schemaContainsKey(child, key) {
				return true
			}
		}
	}
	return false
}
