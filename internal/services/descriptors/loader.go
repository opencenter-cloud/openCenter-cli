package descriptors

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"gopkg.in/yaml.v3"
)

//go:embed data/*.yaml
var embeddedFiles embed.FS

var (
	conditionFieldPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)
	validationRootOnce    sync.Once
	validationRootValue   reflect.Value
	validationRootErr     error
)

// Registry holds the validated descriptor set used by the renderer.
type Registry struct {
	descriptors []Descriptor
	byName      map[string]Descriptor
}

// LoadEmbedded loads all embedded descriptor YAML files.
func LoadEmbedded() (*Registry, error) {
	return loadRegistry(embeddedFiles, "data")
}

func loadRegistry(fsys fs.FS, root string) (*Registry, error) {
	registry := &Registry{
		byName: make(map[string]Descriptor),
	}

	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read descriptor %s: %w", path, err)
		}

		var descriptor Descriptor
		if err := yaml.Unmarshal(data, &descriptor); err != nil {
			return fmt.Errorf("parse descriptor %s: %w", path, err)
		}
		if err := validateDescriptor(descriptor); err != nil {
			return fmt.Errorf("invalid descriptor %s: %w", path, err)
		}
		if _, exists := registry.byName[descriptor.Name]; exists {
			return fmt.Errorf("duplicate descriptor name %q", descriptor.Name)
		}

		registry.descriptors = append(registry.descriptors, descriptor)
		registry.byName[descriptor.Name] = descriptor
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, descriptor := range registry.descriptors {
		for _, target := range descriptor.AggregateTargets {
			if _, ok := registry.byName[target]; !ok {
				return nil, fmt.Errorf("descriptor %q references unknown aggregate target %q", descriptor.Name, target)
			}
		}
	}

	return registry, nil
}

// Descriptors returns the descriptors in registration order.
func (r *Registry) Descriptors() []Descriptor {
	if r == nil {
		return nil
	}
	return slices.Clone(r.descriptors)
}

// Get looks up one descriptor by name.
func (r *Registry) Get(name string) (Descriptor, bool) {
	if r == nil {
		return Descriptor{}, false
	}
	descriptor, ok := r.byName[name]
	return descriptor, ok
}

func validateDescriptor(descriptor Descriptor) error {
	if strings.TrimSpace(descriptor.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(descriptor.Layer) == "" {
		return fmt.Errorf("layer is required")
	}
	if len(descriptor.Roots) == 0 && len(descriptor.Files) == 0 {
		return fmt.Errorf("at least one root or file is required")
	}
	if descriptor.Service != "" && descriptor.ManagedService != "" {
		return fmt.Errorf("service and managed_service are mutually exclusive")
	}
	if err := validateCondition(descriptor.EnabledWhen); err != nil {
		return fmt.Errorf("enabled_when: %w", err)
	}
	for _, root := range descriptor.Roots {
		if strings.TrimSpace(root.Path) == "" {
			return fmt.Errorf("root path is required")
		}
		if err := validateCondition(root.When); err != nil {
			return fmt.Errorf("root %q: %w", root.Path, err)
		}
	}
	for _, file := range descriptor.Files {
		if strings.TrimSpace(file.Template) == "" {
			return fmt.Errorf("file template is required")
		}
		if err := validateCondition(file.When); err != nil {
			return fmt.Errorf("file %q: %w", file.Template, err)
		}
	}
	return nil
}

func validateCondition(condition *Condition) error {
	if condition == nil {
		return nil
	}
	if strings.TrimSpace(condition.Field) == "" {
		return fmt.Errorf("field is required")
	}
	if err := validateFieldPath(condition.Field); err != nil {
		return err
	}
	switch condition.Operator {
	case ConditionOperatorEquals:
		if strings.TrimSpace(condition.Value) == "" {
			return fmt.Errorf("equals requires a value")
		}
	case ConditionOperatorExists, ConditionOperatorTrue, ConditionOperatorFalse:
		if strings.TrimSpace(condition.Value) != "" {
			return fmt.Errorf("%s does not accept a value", condition.Operator)
		}
	default:
		return fmt.Errorf("unsupported operator %q", condition.Operator)
	}
	return nil
}

func validateFieldPath(field string) error {
	if !conditionFieldPattern.MatchString(field) {
		return fmt.Errorf("invalid field path %q", field)
	}

	root, err := defaultValidationRoot()
	if err != nil {
		return err
	}

	if _, ok := lookupFieldValue(root, strings.Split(field, ".")); !ok {
		return fmt.Errorf("unknown field path %q", field)
	}

	return nil
}

func defaultValidationRoot() (reflect.Value, error) {
	validationRootOnce.Do(func() {
		validationRootValue = reflect.ValueOf(config.NewDefault("descriptor-validation"))
	})

	if validationRootErr != nil {
		return reflect.Value{}, validationRootErr
	}

	return validationRootValue, nil
}

func lookupFieldValue(value reflect.Value, segments []string) (reflect.Value, bool) {
	current := dereferenceValue(value)
	for _, segment := range segments {
		if !current.IsValid() {
			return reflect.Value{}, false
		}

		switch current.Kind() {
		case reflect.Struct:
			fieldValue, ok := lookupStructField(current, segment)
			if !ok {
				return reflect.Value{}, false
			}
			current = dereferenceValue(fieldValue)
		case reflect.Map:
			next := current.MapIndex(reflect.ValueOf(segment))
			if !next.IsValid() {
				return reflect.Value{}, false
			}
			current = dereferenceValue(next)
		default:
			return reflect.Value{}, false
		}
	}

	return current, current.IsValid()
}

func dereferenceValue(value reflect.Value) reflect.Value {
	current := value
	for current.IsValid() && (current.Kind() == reflect.Pointer || current.Kind() == reflect.Interface) {
		if current.IsNil() {
			return reflect.Value{}
		}
		current = current.Elem()
	}
	return current
}

func lookupStructField(value reflect.Value, segment string) (reflect.Value, bool) {
	value = dereferenceValue(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	valueType := value.Type()
	for idx := 0; idx < value.NumField(); idx++ {
		fieldType := valueType.Field(idx)
		if !fieldType.IsExported() {
			continue
		}
		if fieldNameMatches(fieldType, segment) {
			return value.Field(idx), true
		}
	}

	return reflect.Value{}, false
}

func fieldNameMatches(field reflect.StructField, segment string) bool {
	if normalizeTag(field.Tag.Get("json")) == segment {
		return true
	}
	if normalizeTag(field.Tag.Get("yaml")) == segment {
		return true
	}
	return strings.EqualFold(field.Name, segment)
}

func normalizeTag(tag string) string {
	if tag == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(tag, ",")[0])
}
