package descriptors

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadRegistryRejectsUnsupportedOperator(t *testing.T) {
	t.Parallel()

	_, err := loadRegistry(fstest.MapFS{
		"data/invalid.yaml": {
			Data: []byte(`
name: invalid-operator
layer: services
enabled_when:
  field: opencenter.meta.region
  operator: contains
files:
  - template: services/test.yaml.tpl
`),
		},
	}, "data")
	if err == nil {
		t.Fatal("expected unsupported operator error")
	}
	if !strings.Contains(err.Error(), `unsupported operator "contains"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRegistryRejectsUnknownFieldPath(t *testing.T) {
	t.Parallel()

	_, err := loadRegistry(fstest.MapFS{
		"data/invalid.yaml": {
			Data: []byte(`
name: invalid-field
layer: services
enabled_when:
  field: opencenter.meta.not_a_field
  operator: exists
files:
  - template: services/test.yaml.tpl
`),
		},
	}, "data")
	if err == nil {
		t.Fatal("expected unknown field path error")
	}
	if !strings.Contains(err.Error(), `unknown field path "opencenter.meta.not_a_field"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRegistryRejectsUnknownAggregateTarget(t *testing.T) {
	t.Parallel()

	_, err := loadRegistry(fstest.MapFS{
		"data/service.yaml": {
			Data: []byte(`
name: service-test
layer: services
aggregate_targets:
  - services-missing
files:
  - template: services/test.yaml.tpl
`),
		},
	}, "data")
	if err == nil {
		t.Fatal("expected unknown aggregate target error")
	}
	if !strings.Contains(err.Error(), `references unknown aggregate target "services-missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
