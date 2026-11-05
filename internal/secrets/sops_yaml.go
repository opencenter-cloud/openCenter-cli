package secrets

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"gopkg.in/yaml.v3"
)

// rewriteSOPSAgeValues changes only existing creation-rule age scalar values.
// The YAML node tree preserves all unrelated fields, comments, and scalar styles.
func rewriteSOPSAgeValues(data []byte, update func([]string) ([]string, error)) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return data, nil
	}

	rules := findSOPSCreationRules(document.Content[0])
	var ageNodes []*yaml.Node
	for _, rule := range rules {
		if rule.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(rule.Content); i += 2 {
			if rule.Content[i].Value != "age" {
				continue
			}
			value := rule.Content[i+1]
			if value.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("creation rule age value is not a scalar")
			}
			existing := splitSOPSRecipients(value.Value)
			recipients, err := update(existing)
			if err != nil {
				return nil, err
			}
			if len(recipients) == 0 {
				return nil, fmt.Errorf("computed SOPS age recipient set is empty")
			}
			value.Value = strings.Join(recipients, ",")
			ageNodes = append(ageNodes, value)
			break
		}
	}
	if len(ageNodes) == 0 {
		return data, nil
	}

	return marshalSOPSDocument(&document)
}

// marshalSOPSDocument serializes the node tree at two-space indentation.
// yaml.Marshal defaults to four, which would reindent every sequence in the
// user's committed .sops.yaml on each rotation or revocation and produce pure
// diff noise in their GitOps repository.
func marshalSOPSDocument(document *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		_ = encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func findSOPSCreationRules(root *yaml.Node) []*yaml.Node {
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "creation_rules" && root.Content[i+1].Kind == yaml.SequenceNode {
			return root.Content[i+1].Content
		}
	}
	return nil
}

func splitSOPSRecipients(value string) []string {
	parts := strings.Split(value, ",")
	recipients := make([]string, 0, len(parts))
	for _, part := range parts {
		if recipient := strings.TrimSpace(part); recipient != "" {
			recipients = append(recipients, recipient)
		}
	}
	return recipients
}

// reencryptManifestUsingCreationRule decrypts a manifest and lets the
// post-mutation .sops.yaml creation rule choose its recipients. The temporary
// path is never used for rule matching; SOPS receives the original manifest
// path through --filename-override.
func reencryptManifestUsingCreationRule(ctx context.Context, encryptor sops.Encryptor, manifestPath, configPath string) error {
	if err := validateSOPSRuleMatch(configPath, manifestPath); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(manifestPath), "."+filepath.Base(manifestPath)+".decrypted-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temporary manifest: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary manifest: %w", err)
	}
	if err := encryptor.DecryptFile(ctx, manifestPath, tmpPath); err != nil {
		return fmt.Errorf("failed to decrypt %s: %w", manifestPath, err)
	}
	if err := encryptor.EncryptFile(ctx, tmpPath, sops.EncryptionConfig{
		ConfigFile:       configPath,
		FilenameOverride: manifestPath,
		InPlace:          true,
	}); err != nil {
		return fmt.Errorf("failed to re-encrypt %s: %w", manifestPath, err)
	}
	if err := os.Rename(tmpPath, manifestPath); err != nil {
		return fmt.Errorf("failed to replace %s: %w", manifestPath, err)
	}
	return nil
}

func validateSOPSRuleMatch(configPath, manifestPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read SOPS config %s: %w", configPath, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("failed to parse SOPS config %s: %w", configPath, err)
	}
	if len(document.Content) == 0 {
		return fmt.Errorf("SOPS config %s has no creation rules", configPath)
	}
	manifestCandidates := []string{manifestPath}
	if relative, err := filepath.Rel(filepath.Dir(configPath), manifestPath); err == nil {
		manifestCandidates = append(manifestCandidates, filepath.ToSlash(relative))
	}
	for _, rule := range findSOPSCreationRules(document.Content[0]) {
		if rule.Kind != yaml.MappingNode {
			continue
		}
		pathRegex := ""
		age := ""
		for i := 0; i+1 < len(rule.Content); i += 2 {
			switch rule.Content[i].Value {
			case "path_regex":
				pathRegex = rule.Content[i+1].Value
			case "age":
				age = rule.Content[i+1].Value
			}
		}
		if strings.TrimSpace(age) == "" {
			continue
		}
		matched := pathRegex == ""
		if pathRegex != "" {
			expression, err := regexp.Compile(pathRegex)
			if err != nil {
				return fmt.Errorf("invalid SOPS creation rule path_regex %q: %w", pathRegex, err)
			}
			matched = false
			for _, candidate := range manifestCandidates {
				if expression.MatchString(candidate) {
					matched = true
					break
				}
			}
		}
		if matched {
			return nil
		}
	}
	return fmt.Errorf("no SOPS creation rule with Age recipients matches manifest %s", manifestPath)
}
