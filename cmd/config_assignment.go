package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// configAssignment is the common input understood by the configuration
// assignment engine. Values remain strings at the command boundary so that
// scalar compatibility (for example, the literal string "null") is retained.
type configAssignment struct {
	Path  string
	Value string
}

// applyConfigAssignments applies all assignments transactionally. Both the
// cluster set command and service parameters use this function; neither
// command can expose a partially assigned object when one value is invalid.
func applyConfigAssignments(target any, assignments []configAssignment) error {
	if len(assignments) == 0 {
		return nil
	}

	candidate, err := cloneAssignmentTarget(target)
	if err != nil {
		return fmt.Errorf("prepare configuration assignment: %w", err)
	}
	for _, assignment := range assignments {
		if err := setField(candidate, assignment.Path, assignment.Value); err != nil {
			return fmt.Errorf("failed to set '%s': %w", assignment.Path, err)
		}
	}
	return commitAssignmentTarget(target, candidate)
}

func cloneAssignmentTarget(target any) (any, error) {
	if target == nil {
		return nil, fmt.Errorf("cannot assign fields on nil object")
	}
	typ := reflect.TypeOf(target)
	clone := cloneAssignmentValue(reflect.ValueOf(target))
	if !clone.IsValid() {
		return nil, fmt.Errorf("cannot clone assignment target of type %s", typ)
	}
	return clone.Interface(), nil
}

func cloneAssignmentValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return reflect.Value{}
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clone := cloneAssignmentValue(value.Elem())
		result := reflect.New(value.Type()).Elem()
		result.Set(clone)
		return result
	case reflect.Ptr:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloneAssignmentValue(value.Elem()))
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for i := 0; i < value.NumField(); i++ {
			if result.Field(i).CanSet() && value.Type().Field(i).PkgPath == "" {
				result.Field(i).Set(cloneAssignmentValue(value.Field(i)))
			}
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		for _, key := range value.MapKeys() {
			result.SetMapIndex(cloneAssignmentValue(key), cloneAssignmentValue(value.MapIndex(key)))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			result.Index(i).Set(cloneAssignmentValue(value.Index(i)))
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			result.Index(i).Set(cloneAssignmentValue(value.Index(i)))
		}
		return result
	default:
		return value
	}
}

func commitAssignmentTarget(target, candidate any) error {
	original := reflect.ValueOf(target)
	updated := reflect.ValueOf(candidate)
	if !original.IsValid() || !updated.IsValid() || original.Type() != updated.Type() {
		return fmt.Errorf("assignment target type changed from %T to %T", target, candidate)
	}

	switch original.Kind() {
	case reflect.Ptr:
		if original.IsNil() || updated.IsNil() {
			return fmt.Errorf("assignment target cannot be a nil pointer")
		}
		original.Elem().Set(updated.Elem())
	case reflect.Map:
		if original.IsNil() {
			return fmt.Errorf("assignment target map cannot be nil")
		}
		for _, key := range original.MapKeys() {
			original.SetMapIndex(key, reflect.Value{})
		}
		for _, key := range updated.MapKeys() {
			original.SetMapIndex(key, updated.MapIndex(key))
		}
	default:
		return fmt.Errorf("assignment target must be a pointer or map, got %s", original.Kind())
	}
	return nil
}

// setField sets a field in a struct or map using a dot-notation path.
func setField(obj any, path string, value string) error {
	if obj == nil {
		return fmt.Errorf("cannot set field on nil object")
	}
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("object must be a non-nil pointer to be settable")
		}
	} else if v.Kind() != reflect.Map {
		return fmt.Errorf("object must be a pointer or map to be settable")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("field path cannot be empty")
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return setFieldPath(v, strings.Split(path, "."), value, path)
}

func setFieldPath(v reflect.Value, parts []string, value, path string) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty field path: %s", path)
	}

	for v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
		if v.Kind() == reflect.Interface {
			if v.IsNil() {
				return fmt.Errorf("cannot traverse nil interface at '%s'", path)
			}
			entry := reflect.New(v.Elem().Type()).Elem()
			entry.Set(v.Elem())
			if err := setFieldPath(entry, parts, value, path); err != nil {
				return err
			}
			if !v.CanSet() {
				return fmt.Errorf("cannot set interface while traversing '%s'", path)
			}
			v.Set(entry)
			return nil
		}
		if v.IsNil() {
			if !v.CanSet() {
				return fmt.Errorf("cannot initialize pointer while traversing '%s'", path)
			}
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return fmt.Errorf("invalid value while traversing '%s'", path)
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		// Numeric path components address elements only when the current value
		// is a slice or array. Numeric strings remain valid map keys.
		index, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("expected numeric index %q for %s while setting '%s'", parts[0], v.Type(), path)
		}
		if index < 0 || index >= v.Len() {
			return fmt.Errorf("index %d out of range for %s while setting '%s'", index, v.Type(), path)
		}
		if len(parts) == 1 {
			return setFieldValue(v.Index(index), value)
		}
		return setFieldPath(v.Index(index), parts[1:], value, path)
	case reflect.Struct:
		field := findField(v, parts[0])
		if !field.IsValid() {
			return fmt.Errorf("field not found: '%s' in struct '%s'", parts[0], v.Type())
		}
		if len(parts) == 1 {
			return setFieldValue(field, value)
		}
		return setFieldPath(field, parts[1:], value, path)
	case reflect.Map:
		key, err := mapKeyFromPath(v.Type().Key(), parts[0])
		if err != nil {
			return fmt.Errorf("invalid map key %q for %s: %w", parts[0], v.Type(), err)
		}
		if len(parts) == 1 {
			entry := reflect.New(v.Type().Elem()).Elem()
			if err := setReflectValue(entry, value); err != nil {
				return fmt.Errorf("failed to set map value for key '%s': %w", parts[0], err)
			}
			if v.IsNil() {
				if !v.CanSet() {
					return fmt.Errorf("cannot initialize nil map while setting '%s'", path)
				}
				v.Set(reflect.MakeMap(v.Type()))
			}
			v.SetMapIndex(key, entry)
			return nil
		}
		entry := v.MapIndex(key)
		if !entry.IsValid() {
			return fmt.Errorf("field not found: '%s' in map while setting '%s'", parts[0], path)
		}
		addressable := reflect.New(entry.Type()).Elem()
		addressable.Set(entry)
		if err := setFieldPath(addressable, parts[1:], value, path); err != nil {
			return err
		}
		v.SetMapIndex(key, addressable)
		return nil
	default:
		return fmt.Errorf("field '%s' is not traversable (type %s) while setting '%s'", parts[0], v.Type(), path)
	}
}

// findField handles yaml/json names, Go names, and anonymous embedded fields.
func findField(v reflect.Value, name string) reflect.Value {
	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fieldInfo := t.Field(i)
		if fieldInfo.PkgPath != "" { // unexported
			continue
		}
		if fieldInfo.Name == name || tagName(fieldInfo.Tag.Get("yaml")) == name || tagName(fieldInfo.Tag.Get("json")) == name {
			return v.Field(i)
		}
	}
	for i := 0; i < v.NumField(); i++ {
		fieldInfo := t.Field(i)
		if !fieldInfo.Anonymous || fieldInfo.PkgPath != "" {
			continue
		}
		embedded := v.Field(i)
		if embedded.Kind() == reflect.Ptr {
			if embedded.Type().Elem().Kind() != reflect.Struct {
				continue
			}
			if embedded.IsNil() {
				if !embedded.CanSet() {
					continue
				}
				embedded.Set(reflect.New(embedded.Type().Elem()))
			}
			embedded = embedded.Elem()
		}
		if embedded.Kind() == reflect.Struct {
			if field := findField(embedded, name); field.IsValid() {
				return field
			}
		}
	}
	return reflect.Value{}
}

func tagName(tag string) string {
	return strings.Split(tag, ",")[0]
}

func mapKeyFromPath(typ reflect.Type, raw string) (reflect.Value, error) {
	key := reflect.New(typ).Elem()
	switch typ.Kind() {
	case reflect.String:
		key.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, typ.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("expected %s: %w", typ, err)
		}
		key.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, typ.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("expected %s: %w", typ, err)
		}
		key.SetUint(value)
	default:
		return reflect.Value{}, fmt.Errorf("map key type %s is not supported for path-based setting", typ)
	}
	return key, nil
}

func setFieldValue(field reflect.Value, value string) error {
	if !field.IsValid() || !field.CanSet() {
		return fmt.Errorf("cannot set field value")
	}
	return setReflectValue(field, value)
}

// setReflectValue converts command-line text to the destination type. JSON is
// used for composite values; []string additionally accepts comma-delimited
// input for compatibility with existing CLI usage.
func setReflectValue(field reflect.Value, value string) error {
	converted, err := reflectValueFromString(field.Type(), value)
	if err != nil {
		return err
	}
	field.Set(converted)
	return nil
}

func reflectValueFromString(typ reflect.Type, value string) (reflect.Value, error) {
	if typ.Kind() == reflect.Ptr {
		if strings.TrimSpace(value) == "null" {
			return reflect.Zero(typ), nil
		}
		element, err := reflectValueFromString(typ.Elem(), value)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid value for %s: %w", typ, err)
		}
		result := reflect.New(typ.Elem())
		result.Elem().Set(element)
		return result, nil
	}

	result := reflect.New(typ).Elem()
	trimmed := strings.TrimSpace(value)
	switch typ.Kind() {
	case reflect.String:
		result.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, typ.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid integer value: '%s'", value)
		}
		result.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(value, 10, typ.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid unsigned integer value: '%s'", value)
		}
		result.SetUint(i)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, typ.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid float value: '%s'", value)
		}
		result.SetFloat(f)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid boolean value: '%s'", value)
		}
		result.SetBool(b)
	case reflect.Interface:
		parsed, err := parseInterfaceValue(value)
		if err != nil {
			return reflect.Value{}, err
		}
		if parsed == nil {
			return reflect.Zero(typ), nil
		}
		parsedValue := reflect.ValueOf(parsed)
		if !parsedValue.Type().AssignableTo(typ) {
			return reflect.Value{}, fmt.Errorf("value of type %s does not implement interface %s", parsedValue.Type(), typ)
		}
		result.Set(parsedValue)
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 && strings.HasPrefix(trimmed, "[") {
			return reflect.Value{}, fmt.Errorf("invalid JSON array for %s", typ)
		}
		if typ.Elem().Kind() == reflect.String && !strings.HasPrefix(trimmed, "[") {
			items := []string{}
			if value != "" {
				for _, item := range strings.Split(value, ",") {
					items = append(items, strings.TrimSpace(item))
				}
			}
			result.Set(reflect.MakeSlice(typ, len(items), len(items)))
			for i, item := range items {
				result.Index(i).SetString(item)
			}
			break
		}
		if err := json.Unmarshal([]byte(value), result.Addr().Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("invalid JSON value for %s: %w", typ, err)
		}
	case reflect.Map, reflect.Struct, reflect.Array:
		if err := json.Unmarshal([]byte(value), result.Addr().Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("invalid JSON value for %s: %w", typ, err)
		}
	default:
		return reflect.Value{}, fmt.Errorf("unsupported field type: %s", typ)
	}
	return result, nil
}

func parseInterfaceValue(value string) (any, error) {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, fmt.Errorf("invalid JSON value: %w", err)
		}
		return parsed, nil
	}
	if b, err := strconv.ParseBool(value); err == nil {
		return b, nil
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i, nil
	}
	return value, nil
}
