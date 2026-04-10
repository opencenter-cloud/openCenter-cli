package gitops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

type parityInventory struct {
	Defaults parityClusterRules            `yaml:"defaults"`
	Clusters map[string]parityClusterRules `yaml:"clusters"`
}

type parityClusterRules struct {
	Aliases        []parityAlias    `yaml:"aliases"`
	IgnoreActual   []string         `yaml:"ignore_actual"`
	IgnoreExpected []string         `yaml:"ignore_expected"`
	CompareModes   []parityCompare  `yaml:"compare_modes"`
	YAMLRules      []parityYAMLRule `yaml:"yaml_rules"`
}

type parityAlias struct {
	Actual   string `yaml:"actual"`
	Expected string `yaml:"expected"`
}

type parityCompare struct {
	Path string `yaml:"path"`
	Mode string `yaml:"mode"`
}

type parityYAMLRule struct {
	Path   string   `yaml:"path,omitempty"`
	Kind   string   `yaml:"kind,omitempty"`
	Remove []string `yaml:"remove,omitempty"`
}

type resolvedParityRules struct {
	aliases        map[string]string
	ignoreActual   map[string]struct{}
	ignoreExpected map[string]struct{}
	compareModes   map[string]string
	compareGlobs   []parityCompare
	yamlRules      []parityYAMLRule
}

func TestRelayPointRenderingParity(t *testing.T) {
	inventory := loadParityInventory(t)
	fixtureRoot := filepath.Join("..", "..", "testdata", "relaypoint-logistics-shared")

	clusters := []string{"k8s-dev", "k8s-dr", "k8s-prod", "k8s-qa", "k8s-uat"}
	for _, cluster := range clusters {
		t.Run(cluster, func(t *testing.T) {
			cfg := loadParityConfig(t, filepath.Join(fixtureRoot, "."+cluster+"-config.yaml"))
			cfg.OpenCenter.GitOps.GitDir = t.TempDir()

			actions, err := planClusterAppActions(cfg)
			if err != nil {
				t.Fatalf("planClusterAppActions(%s): %v", cluster, err)
			}

			if err := RenderClusterApps(cfg); err != nil {
				t.Fatalf("RenderClusterApps(%s): %v", cluster, err)
			}

			actualRoot := filepath.Join(cfg.OpenCenter.GitOps.GitDir, "applications", "overlays", cluster)
			expectedRoot := filepath.Join(fixtureRoot, "applications", "overlays", cluster)
			rules := inventory.rulesFor(cluster)

			actualOwned := make(map[string]string)
			for _, path := range ownedOutputPaths(actions) {
				if _, ignored := rules.ignoreActual[path]; ignored {
					continue
				}
				expectedPath := rules.aliases[path]
				if expectedPath == "" {
					expectedPath = path
				}
				if _, ignored := rules.ignoreExpected[expectedPath]; ignored {
					continue
				}
				if _, exists := actualOwned[expectedPath]; exists {
					t.Fatalf("canonical output collision for %s", expectedPath)
				}
				actualOwned[expectedPath] = path
			}

			expectedOwned, err := scanRendererOwnedFixturePaths(expectedRoot)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			for ignored := range rules.ignoreExpected {
				delete(expectedOwned, ignored)
			}

			var missing []string
			for expectedPath := range expectedOwned {
				if _, ok := actualOwned[expectedPath]; !ok {
					missing = append(missing, expectedPath)
				}
			}

			var unexpected []string
			for expectedPath := range actualOwned {
				if _, ok := expectedOwned[expectedPath]; !ok {
					unexpected = append(unexpected, expectedPath)
				}
			}

			sort.Strings(missing)
			sort.Strings(unexpected)
			if len(missing) > 0 || len(unexpected) > 0 {
				t.Fatalf("path parity mismatch: missing=%v unexpected=%v", missing, unexpected)
			}

			for expectedPath, actualPath := range actualOwned {
				actualData, err := os.ReadFile(filepath.Join(actualRoot, actualPath))
				if err != nil {
					t.Fatalf("read actual %s: %v", actualPath, err)
				}
				expectedData, err := os.ReadFile(filepath.Join(expectedRoot, expectedPath))
				if err != nil {
					t.Fatalf("read expected %s: %v", expectedPath, err)
				}

				actualCanonical, err := canonicalizeParityContent(actualData, expectedPath, rules)
				if err != nil {
					t.Fatalf("canonicalize actual %s: %v", actualPath, err)
				}
				expectedCanonical, err := canonicalizeParityContent(expectedData, expectedPath, rules)
				if err != nil {
					t.Fatalf("canonicalize expected %s: %v", expectedPath, err)
				}

				if !bytes.Equal(actualCanonical, expectedCanonical) {
					t.Fatalf("content mismatch for %s\nactual:\n%s\nexpected:\n%s", expectedPath, actualCanonical, expectedCanonical)
				}
			}
		})
	}
}

func loadParityConfig(t *testing.T, path string) v2.Config {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config %s: %v", path, err)
	}

	var cfg v2.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config %s: %v", path, err)
	}

	return cfg
}

func loadParityInventory(t *testing.T) parityInventory {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "relaypoint-logistics-shared", "parity-canonicalization.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read parity inventory: %v", err)
	}

	var inventory parityInventory
	if err := yaml.Unmarshal(data, &inventory); err != nil {
		t.Fatalf("unmarshal parity inventory: %v", err)
	}

	return inventory
}

func (inventory parityInventory) rulesFor(cluster string) resolvedParityRules {
	resolved := resolvedParityRules{
		aliases:        make(map[string]string),
		ignoreActual:   make(map[string]struct{}),
		ignoreExpected: make(map[string]struct{}),
		compareModes:   make(map[string]string),
	}

	mergeRules := func(rules parityClusterRules) {
		for _, alias := range rules.Aliases {
			resolved.aliases[filepath.ToSlash(alias.Actual)] = filepath.ToSlash(alias.Expected)
		}
		for _, path := range rules.IgnoreActual {
			resolved.ignoreActual[filepath.ToSlash(path)] = struct{}{}
		}
		for _, path := range rules.IgnoreExpected {
			resolved.ignoreExpected[filepath.ToSlash(path)] = struct{}{}
		}
		for _, compare := range rules.CompareModes {
			pattern := filepath.ToSlash(compare.Path)
			if strings.ContainsAny(pattern, "*?[") {
				resolved.compareGlobs = append(resolved.compareGlobs, parityCompare{
					Path: pattern,
					Mode: compare.Mode,
				})
				continue
			}
			resolved.compareModes[pattern] = compare.Mode
		}
		resolved.yamlRules = append(resolved.yamlRules, rules.YAMLRules...)
	}

	mergeRules(inventory.Defaults)
	if clusterRules, ok := inventory.Clusters[cluster]; ok {
		mergeRules(clusterRules)
	}

	return resolved
}

func ownedOutputPaths(actions []clusterAppAction) []string {
	seen := make(map[string]struct{})
	var outputs []string
	for _, action := range actions {
		path := filepath.ToSlash(action.Output)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		outputs = append(outputs, path)
	}
	sort.Strings(outputs)
	return outputs
}

func scanRendererOwnedFixturePaths(root string) (map[string]struct{}, error) {
	owned := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !isRendererOwnedFixturePath(rel) {
			return nil
		}
		owned[rel] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return owned, nil
}

func isRendererOwnedFixturePath(path string) bool {
	switch {
	case path == "kustomization.yaml":
		return true
	case path == ".sops.yaml":
		return true
	case strings.HasPrefix(path, "services/"):
		return true
	case strings.HasPrefix(path, "managed-services/"):
		return true
	case strings.HasPrefix(path, "customer-managed/"):
		return true
	default:
		return false
	}
}

func canonicalizeParityContent(data []byte, path string, rules resolvedParityRules) ([]byte, error) {
	path = filepath.ToSlash(path)
	if mode := rules.compareMode(path); mode == "path_only" {
		return []byte("path-only"), nil
	}

	docs, err := decodeYAMLDocuments(data)
	if err != nil {
		return bytes.TrimSpace(data), nil
	}

	parts := make([]string, 0, len(docs))
	for _, doc := range docs {
		for _, rule := range rules.yamlRules {
			if rule.Path != "" && filepath.ToSlash(rule.Path) != path {
				continue
			}
			if rule.Kind != "" {
				kind, _ := documentKind(doc)
				if kind != rule.Kind {
					continue
				}
			}
			for _, pointer := range rule.Remove {
				removeJSONPointer(doc, pointer)
			}
		}

		part, err := json.Marshal(doc)
		if err != nil {
			return nil, fmt.Errorf("marshal canonical doc for %s: %w", path, err)
		}
		parts = append(parts, string(part))
	}

	return []byte(strings.Join(parts, "\n---\n")), nil
}

func (rules resolvedParityRules) compareMode(path string) string {
	if mode := rules.compareModes[path]; mode != "" {
		return mode
	}

	for _, compare := range rules.compareGlobs {
		matched, err := filepath.Match(compare.Path, path)
		if err != nil {
			continue
		}
		if matched {
			return compare.Mode
		}
	}

	return ""
}

func decodeYAMLDocuments(data []byte) ([]map[string]any, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var docs []map[string]any
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(doc) == 0 {
			continue
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func documentKind(doc map[string]any) (string, bool) {
	kind, ok := doc["kind"].(string)
	return kind, ok
}

func removeJSONPointer(doc map[string]any, pointer string) {
	if pointer == "" || pointer == "/" {
		return
	}

	segments := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	current := map[string]any(doc)
	for idx, segment := range segments {
		segment = strings.ReplaceAll(segment, "~1", "/")
		segment = strings.ReplaceAll(segment, "~0", "~")
		if idx == len(segments)-1 {
			delete(current, segment)
			return
		}
		next, ok := current[segment].(map[string]any)
		if !ok {
			return
		}
		current = next
	}
}
