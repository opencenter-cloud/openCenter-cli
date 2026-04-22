package importer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"
)

type PatchResult struct {
	Path    string
	Content []byte
	Diff    string
}

func SelectApprovedFields(cluster ClusterImportResult) ([]FieldInferenceResult, []SkippedField) {
	conflicts := make(map[string]struct{}, len(cluster.Conflicts))
	for _, conflict := range cluster.Conflicts {
		conflicts[strings.TrimSpace(conflict.Path)] = struct{}{}
	}

	approved := make([]FieldInferenceResult, 0, len(cluster.FieldResults))
	skipped := make([]SkippedField, 0)

	collect := func(fields []FieldInferenceResult) {
		for _, field := range fields {
			path := strings.TrimSpace(field.Path)
			if path == "" {
				continue
			}

			if _, ok := conflicts[path]; ok {
				skipped = append(skipped, SkippedField{Path: path, Reason: "conflict requires manual review", Confidence: field.Confidence})
				continue
			}
			if IsProtectedField(path) {
				skipped = append(skipped, SkippedField{Path: path, Reason: "protected field", Confidence: field.Confidence})
				continue
			}
			if field.Confidence != ConfidenceHigh {
				skipped = append(skipped, SkippedField{Path: path, Reason: "requires high confidence", Confidence: field.Confidence})
				continue
			}
			if strings.HasSuffix(path, ".enabled") {
				if enabled, ok := field.Value.(bool); ok && !enabled {
					skipped = append(skipped, SkippedField{Path: path, Reason: "disabled services are inferred by absence", Confidence: field.Confidence})
					continue
				}
			}

			approved = append(approved, field)
		}
	}

	collect(cluster.FieldResults)
	for _, service := range cluster.ServiceResults {
		collect(service.Fields)
	}

	return approved, skipped
}

func PatchYAMLFile(path string, updates []FieldInferenceResult) (*PatchResult, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read yaml file: %w", err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(original, &document); err != nil {
		return nil, fmt.Errorf("decode yaml document: %w", err)
	}

	root := ensureDocumentRoot(&document)
	for _, update := range updates {
		if err := setYAMLPath(root, update.Path, update.Value); err != nil {
			return nil, fmt.Errorf("set yaml path %q: %w", update.Path, err)
		}
	}

	var rendered bytes.Buffer
	encoder := yaml.NewEncoder(&rendered)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return nil, fmt.Errorf("encode patched yaml: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close yaml encoder: %w", err)
	}

	patched := rendered.Bytes()
	if err := os.WriteFile(path, patched, 0o600); err != nil {
		return nil, fmt.Errorf("write patched yaml: %w", err)
	}

	diff, err := buildUnifiedDiff(path, string(original), string(patched))
	if err != nil {
		return nil, fmt.Errorf("build yaml diff: %w", err)
	}

	return &PatchResult{
		Path:    path,
		Content: patched,
		Diff:    diff,
	}, nil
}

func ensureDocumentRoot(document *yaml.Node) *yaml.Node {
	if document.Kind == 0 {
		document.Kind = yaml.DocumentNode
	}
	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := document.Content[0]
	if root.Kind == 0 {
		root.Kind = yaml.MappingNode
		root.Tag = "!!map"
	}
	return root
}

func setYAMLPath(root *yaml.Node, path string, value any) error {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 {
		return fmt.Errorf("path cannot be empty")
	}

	current := root
	for i, part := range parts {
		if current.Kind == 0 {
			current.Kind = yaml.MappingNode
			current.Tag = "!!map"
		}
		if current.Kind != yaml.MappingNode {
			return fmt.Errorf("path %q traverses non-mapping node at %q", path, strings.Join(parts[:i], "."))
		}

		if i == len(parts)-1 {
			valueNode, err := marshalYAMLValue(value)
			if err != nil {
				return err
			}
			setMappingValue(current, part, valueNode)
			return nil
		}

		next := findMappingValue(current, part)
		if next == nil {
			next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			setMappingValue(current, part, next)
		}
		current = next
	}

	return nil
}

func findMappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}

	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

func marshalYAMLValue(value any) (*yaml.Node, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal value: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("decode marshaled value: %w", err)
	}

	if len(node.Content) == 0 {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	}
	return node.Content[0], nil
}

func buildUnifiedDiff(path, before, after string) (string, error) {
	relative := filepath.Base(path)
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(before),
		B:        difflib.SplitLines(after),
		FromFile: relative,
		ToFile:   relative,
		Context:  3,
	})
	if err != nil {
		return "", err
	}
	return diff, nil
}
