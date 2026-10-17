package v2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

var legacyRenderMetadataSections = []string{"services", "managed_services", "managed-service"}

// SanitizeLegacyRenderMetadata removes recognized legacy render metadata from
// raw YAML before it is decoded with KnownFields enabled. Values are treated as
// opaque and discarded regardless of their YAML type or contents.
func SanitizeLegacyRenderMetadata(data []byte) ([]byte, []services.DeprecatedConfigWarning, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, nil, fmt.Errorf("parse YAML for legacy metadata sanitization: %w", err)
	}

	warnings := services.DeprecatedConfigWarnings(data)
	sortDeprecatedConfigWarnings(warnings)
	root := publicYAMLDocumentRoot(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		encoded, err := encodePublicYAML(&document)
		return encoded, warnings, err
	}

	opencenter := publicYAMLMappingValue(root, "opencenter")
	if opencenter == nil || opencenter.Kind != yaml.MappingNode {
		encoded, err := encodePublicYAML(&document)
		return encoded, warnings, err
	}

	keys := services.DeprecatedServiceConfigKeys()
	for _, section := range legacyRenderMetadataSections {
		sectionNode := publicYAMLMappingValue(opencenter, section)
		if sectionNode == nil || sectionNode.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(sectionNode.Content); i += 2 {
			serviceNode := sectionNode.Content[i+1]
			if serviceNode == nil || serviceNode.Kind != yaml.MappingNode {
				continue
			}
			removePublicYAMLMappingKeys(serviceNode, keys)
		}
	}

	encoded, err := encodePublicYAML(&document)
	if err != nil {
		return nil, nil, err
	}
	return encoded, warnings, nil
}

// SanitizePublicYAML removes all recognized internal render metadata from an
// already serialized public configuration document without emitting warnings.
func SanitizePublicYAML(data []byte) ([]byte, error) {
	sanitized, _, err := SanitizeLegacyRenderMetadata(data)
	return sanitized, err
}

// MarshalPublicConfig serializes a v2 configuration for user-facing YAML.
// Legacy render metadata is stripped from serialized documents for compatibility
// with older in-memory or raw YAML inputs.
func MarshalPublicConfig(cfg *Config) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal public config: %w", err)
	}
	return SanitizePublicYAML(data)
}

// MarshalPublicConfigJSON serializes a v2 configuration for user-facing JSON.
// It starts with encoding/json so public JSON tags, including json:"-", are
// honored before legacy render metadata is removed from service configurations.
func MarshalPublicConfigJSON(cfg *Config) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal public config JSON: %w", err)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode public config JSON: %w", err)
	}
	if err := removeLegacyRenderMetadataFromJSON(document); err != nil {
		return nil, fmt.Errorf("sanitize public config JSON: %w", err)
	}

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("indent public config JSON: %w", err)
	}
	return encoded, nil
}

func removeLegacyRenderMetadataFromJSON(document map[string]json.RawMessage) error {
	opencenter, err := decodeJSONMapping(document["opencenter"])
	if err != nil || opencenter == nil {
		return err
	}

	deprecated := make(map[string]struct{})
	for _, key := range services.DeprecatedServiceConfigKeys() {
		deprecated[key.Key] = struct{}{}
	}

	for _, section := range legacyRenderMetadataSections {
		serviceMap, err := decodeJSONMapping(opencenter[section])
		if err != nil {
			return err
		}
		if serviceMap == nil {
			continue
		}
		for serviceName, serviceJSON := range serviceMap {
			serviceConfig, err := decodeJSONMapping(serviceJSON)
			if err != nil {
				return fmt.Errorf("decode %s.%s: %w", section, serviceName, err)
			}
			if serviceConfig == nil {
				continue
			}
			for key := range deprecated {
				delete(serviceConfig, key)
			}
			encoded, err := json.Marshal(serviceConfig)
			if err != nil {
				return fmt.Errorf("encode %s.%s: %w", section, serviceName, err)
			}
			serviceMap[serviceName] = encoded
		}
		encoded, err := json.Marshal(serviceMap)
		if err != nil {
			return fmt.Errorf("encode %s: %w", section, err)
		}
		opencenter[section] = encoded
	}

	encoded, err := json.Marshal(opencenter)
	if err != nil {
		return fmt.Errorf("encode opencenter: %w", err)
	}
	document["opencenter"] = encoded
	return nil
}

func decodeJSONMapping(data json.RawMessage) (map[string]json.RawMessage, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var mapping map[string]json.RawMessage
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, err
	}
	return mapping, nil
}

func publicYAMLDocumentRoot(document *yaml.Node) *yaml.Node {
	if document == nil {
		return nil
	}
	if document.Kind == yaml.DocumentNode {
		if len(document.Content) == 0 {
			return nil
		}
		return document.Content[0]
	}
	return document
}

func publicYAMLMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func removePublicYAMLMappingKeys(node *yaml.Node, keys []services.DeprecatedConfigKey) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	deprecated := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		deprecated[key.Key] = struct{}{}
	}

	content := make([]*yaml.Node, 0, len(node.Content))
	for i := 0; i+1 < len(node.Content); i += 2 {
		if _, ok := deprecated[node.Content[i].Value]; ok {
			continue
		}
		content = append(content, node.Content[i], node.Content[i+1])
	}
	node.Content = content
}

func encodePublicYAML(document *yaml.Node) ([]byte, error) {
	var encoded bytes.Buffer
	encoder := yaml.NewEncoder(&encoded)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode sanitized YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close sanitized YAML encoder: %w", err)
	}
	return encoded.Bytes(), nil
}

// rejectUnknownPublicServiceKeys compensates for ServiceMap's custom YAML
// unmarshaler, which otherwise prevents yaml.Decoder.KnownFields from seeing
// unknown keys inside polymorphic service configurations.
func rejectUnknownPublicServiceKeys(data []byte) error {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("parse YAML for strict service-key validation: %w", err)
	}
	root := publicYAMLDocumentRoot(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	opencenter := publicYAMLMappingValue(root, "opencenter")
	if opencenter == nil || opencenter.Kind != yaml.MappingNode {
		return nil
	}

	for _, section := range legacyRenderMetadataSections {
		sectionNode := publicYAMLMappingValue(opencenter, section)
		if sectionNode == nil || sectionNode.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(sectionNode.Content); i += 2 {
			serviceName := sectionNode.Content[i].Value
			serviceNode := sectionNode.Content[i+1]
			if serviceNode == nil || serviceNode.Kind != yaml.MappingNode {
				continue
			}

			serviceType := registry.GetServiceConfigType(serviceName)
			if serviceType == nil {
				serviceType = reflect.TypeOf(services.DefaultServiceConfig{})
			}
			allowed := publicYAMLFieldNames(serviceType)
			for j := 0; j+1 < len(serviceNode.Content); j += 2 {
				key := serviceNode.Content[j].Value
				if _, ok := allowed[key]; !ok {
					return fmt.Errorf("unknown field %q at opencenter.%s.%s", key, section, serviceName)
				}
			}
		}
	}
	return nil
}

func publicYAMLFieldNames(typ reflect.Type) map[string]struct{} {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	fields := make(map[string]struct{})
	if typ.Kind() != reflect.Struct {
		return fields
	}
	publicYAMLCollectFieldNames(typ, fields)
	return fields
}

func publicYAMLCollectFieldNames(typ reflect.Type, fields map[string]struct{}) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("yaml")
		name := strings.Split(tag, ",")[0]
		if name == "-" {
			continue
		}
		if name == "" && field.Anonymous {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				publicYAMLCollectFieldNames(embedded, fields)
			}
			continue
		}
		if name != "" {
			fields[name] = struct{}{}
		}
	}
}

// sortDeprecatedConfigWarnings establishes a canonical order independent of
// YAML map insertion order or the order of keys in the registry.
func sortDeprecatedConfigWarnings(warnings []services.DeprecatedConfigWarning) {
	sectionOrder := map[string]int{
		"services":         0,
		"managed_services": 1,
		"managed-service":  2,
	}
	sort.SliceStable(warnings, func(i, j int) bool {
		left := strings.Split(warnings[i].Path, ".")
		right := strings.Split(warnings[j].Path, ".")
		leftOrder := sectionOrder[""]
		rightOrder := sectionOrder[""]
		if len(left) > 1 {
			leftOrder = sectionOrder[left[1]]
		}
		if len(right) > 1 {
			rightOrder = sectionOrder[right[1]]
		}
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		return warnings[i].Path < warnings[j].Path
	})
}

// DecodePublicConfig decodes user-facing v2 YAML using the same strict public
// contract as ConfigLoader: recognized legacy render metadata is warned about
// and removed, unknown service fields are rejected, and all other unknown YAML
// fields are rejected by KnownFields.
func DecodePublicConfig(data []byte) (*Config, error) {
	sanitized, warnings, err := SanitizeLegacyRenderMetadata(data)
	if err != nil {
		return nil, err
	}
	if err := rejectUnknownPublicServiceKeys(sanitized); err != nil {
		return nil, err
	}
	services.WarnDeprecatedConfigWarnings(warnings)

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(sanitized))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		if yamlErr, ok := err.(*yaml.TypeError); ok {
			return nil, &YAMLTypeErrors{Errors: improveLegacyAffinityErrors(yamlErr.Errors)}
		}
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if cfg.SchemaVersion != "2.0" {
		if cfg.SchemaVersion == "" {
			return nil, fmt.Errorf("invalid schema version: expected '2.0'")
		}
		return nil, fmt.Errorf("invalid schema version: expected '2.0', got '%s'", cfg.SchemaVersion)
	}

	return &cfg, nil
}

// improveLegacyAffinityErrors rewrites the opaque KnownFields failure for the
// removed top-level infrastructure.server_group_affinity field into an action-
// oriented message pointing at its compute-level replacements.
func improveLegacyAffinityErrors(errors []string) []string {
	for i, e := range errors {
		if strings.Contains(e, "field server_group_affinity not found in type v2.InfrastructureConfig") {
			errors[i] = fmt.Sprintf(
				"%s -- use opencenter.infrastructure.compute.worker_group_affinity (workers) or opencenter.infrastructure.compute.cp_server_group_affinity (control plane)",
				e)
		}
	}
	return errors
}
