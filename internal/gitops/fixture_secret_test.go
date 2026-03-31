package gitops

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var fixturePrivateKeyMarkers = []string{
	"BEGIN PRIVATE KEY",
	"BEGIN OPENSSH PRIVATE KEY",
	"BEGIN RSA PRIVATE KEY",
	"BEGIN EC PRIVATE KEY",
	"AGE-SECRET-KEY-",
}

func TestRelayPointFixturesDoNotContainPrivateKeyMaterial(t *testing.T) {
	t.Parallel()

	fixtureRoot := filepath.Join("..", "..", "testdata", "relaypoint-logistics-shared")
	var findings []string

	err := filepath.WalkDir(fixtureRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := strings.ToLower(filepath.Ext(path)); ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixtureRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if marker := findPrivateKeyMarker(string(data)); marker != "" {
			findings = append(findings, fmt.Sprintf("%s contains literal marker %q", rel, marker))
		}

		decoder := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var doc map[string]any
			if err := decoder.Decode(&doc); err != nil {
				if err == io.EOF {
					break
				}
				break
			}
			if kind, _ := doc["kind"].(string); kind != "Secret" {
				continue
			}

			for key, value := range flattenSecretData(doc) {
				decoded := decodeFixtureSecretValue(value)
				if marker := findPrivateKeyMarker(decoded); marker != "" {
					findings = append(findings, fmt.Sprintf("%s secret key %q decodes to %q", rel, key, marker))
				}
				if rel == "applications/overlays/k8s-uat/customer-managed/sources/customer-repository-rpl-apps-flux-k8s-secret.yaml" {
					if key == "identity" || key == "identity.pub" || key == "known_hosts" {
						if !strings.HasPrefix(decoded, "PLACEHOLDER-") {
							findings = append(findings, fmt.Sprintf("%s key %q must decode to a PLACEHOLDER value", rel, key))
						}
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("scan fixtures: %v", err)
	}

	if len(findings) > 0 {
		slices.Sort(findings)
		t.Fatalf("fixture secret policy violations:\n%s", strings.Join(findings, "\n"))
	}
}

func flattenSecretData(doc map[string]any) map[string]string {
	values := make(map[string]string)
	for _, field := range []string{"data", "stringData"} {
		raw, ok := doc[field]
		if !ok {
			continue
		}
		entries, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for key, value := range entries {
			text, ok := value.(string)
			if ok {
				values[key] = text
			}
		}
	}
	return values
}

func decodeFixtureSecretValue(value string) string {
	trimmed := strings.TrimSpace(value)
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return trimmed
	}
	return string(decoded)
}

func findPrivateKeyMarker(value string) string {
	upper := strings.ToUpper(value)
	for _, marker := range fixturePrivateKeyMarkers {
		if strings.Contains(upper, marker) {
			return marker
		}
	}
	return ""
}
