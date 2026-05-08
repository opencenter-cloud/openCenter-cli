package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigIDE_DefaultWritesSchemaAndPrintsGenericInstructions(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	cmd := NewSettingsCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"ide"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config ide failed: %v\noutput:\n%s", err, out.String())
	}

	assertFileExists(t, filepath.Join("schema", "opencenter-v2.schema.json"))
	if _, err := os.Stat(filepath.Join(".vscode", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("default config ide should not write VS Code settings, stat err = %v", err)
	}

	output := out.String()
	for _, want := range []string{"schema/opencenter-v2.schema.json", "yaml-language-server"} {
		if !strings.Contains(output, want) {
			t.Fatalf("config ide output missing %q:\n%s", want, output)
		}
	}
}

func TestConfigIDE_PrintWritesSchemaToStdoutOnly(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	cmd := NewSettingsCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"ide", "--print"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config ide --print failed: %v", err)
	}
	if !strings.Contains(out.String(), `"schema_version"`) {
		t.Fatalf("printed schema missing schema_version:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join("schema", "opencenter-v2.schema.json")); !os.IsNotExist(err) {
		t.Fatalf("--print should not write schema file, stat err = %v", err)
	}
}

func TestConfigIDE_SchemaOnlySkipsEditorInstructions(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	cmd := NewSettingsCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"ide", "--schema-only"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config ide --schema-only failed: %v", err)
	}
	assertFileExists(t, filepath.Join("schema", "opencenter-v2.schema.json"))
	if strings.Contains(out.String(), "yaml-language-server") {
		t.Fatalf("--schema-only should skip editor instructions:\n%s", out.String())
	}
}

func TestConfigIDE_CheckDetectsCurrentAndStaleSchema(t *testing.T) {
	t.Chdir(t.TempDir())

	if out, err := executeConfigIDE("ide", "--schema-only"); err != nil {
		t.Fatalf("schema generation failed: %v\n%s", err, out)
	}
	if out, err := executeConfigIDE("ide", "--check"); err != nil {
		t.Fatalf("schema check failed for current schema: %v\n%s", err, out)
	}

	schemaPath := filepath.Join("schema", "opencenter-v2.schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatalf("write stale schema: %v", err)
	}
	out, err := executeConfigIDE("ide", "--check")
	if err == nil {
		t.Fatalf("schema check unexpectedly passed for stale schema:\n%s", out)
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale schema error = %v, want stale", err)
	}
}

func TestConfigIDE_VSCodeWriteMergesSettings(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".vscode", 0o755); err != nil {
		t.Fatalf("create .vscode: %v", err)
	}
	existing := []byte(`{
  "editor.tabSize": 2,
  "yaml.schemas": {
    "./schema/existing.json": ["*.existing.yaml"]
  },
  "files.associations": {
    "*.old": "yaml"
  }
}`)
	if err := os.WriteFile(filepath.Join(".vscode", "settings.json"), existing, 0o644); err != nil {
		t.Fatalf("write existing settings: %v", err)
	}

	out, err := executeConfigIDE("ide", "--ide", "vscode", "--write")
	if err != nil {
		t.Fatalf("config ide --ide vscode --write failed: %v\n%s", err, out)
	}

	settings := readJSONFile(t, filepath.Join(".vscode", "settings.json"))
	if got := settings["editor.tabSize"]; got != float64(2) {
		t.Fatalf("editor.tabSize = %v, want 2", got)
	}
	schemas := objectAt(t, settings, "yaml.schemas")
	if _, ok := schemas["./schema/existing.json"]; !ok {
		t.Fatalf("existing yaml schema association was not preserved: %v", schemas)
	}
	newAssociation, ok := schemas["./schema/opencenter-v2.schema.json"].([]any)
	if !ok {
		t.Fatalf("new schema association missing or wrong type: %v", schemas)
	}
	if !containsAnyString(newAssociation, "**/.opencenter-v2.yaml") || !containsAnyString(newAssociation, "**/*-config.yaml") {
		t.Fatalf("new schema association missing expected patterns: %v", newAssociation)
	}
	associations := objectAt(t, settings, "files.associations")
	if got := associations["*.old"]; got != "yaml" {
		t.Fatalf("existing file association lost: %v", associations)
	}
	if got := associations["*-config.yaml"]; got != "yaml" {
		t.Fatalf("new config file association = %v, want yaml", got)
	}
}

func TestConfigIDE_VSCodeWriteRejectsInvalidExistingSettingsWithoutOverwrite(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".vscode", 0o755); err != nil {
		t.Fatalf("create .vscode: %v", err)
	}
	settingsPath := filepath.Join(".vscode", "settings.json")
	invalid := []byte(`{"yaml.schemas":`)
	if err := os.WriteFile(settingsPath, invalid, 0o644); err != nil {
		t.Fatalf("write invalid settings: %v", err)
	}

	out, err := executeConfigIDE("ide", "--ide", "vscode", "--write")
	if err == nil {
		t.Fatalf("config ide --write unexpectedly passed with invalid settings:\n%s", out)
	}
	data, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatalf("read settings after failure: %v", readErr)
	}
	if !bytes.Equal(data, invalid) {
		t.Fatalf("invalid settings were overwritten:\n%s", string(data))
	}
}

func TestConfigIDE_WriteUnsupportedForJetBrains(t *testing.T) {
	t.Chdir(t.TempDir())

	out, err := executeConfigIDE("ide", "--ide", "jetbrains", "--write")
	if err == nil {
		t.Fatalf("config ide --ide jetbrains --write unexpectedly passed:\n%s", out)
	}
	if !strings.Contains(err.Error(), "automatic editor config writes are only supported for vscode") {
		t.Fatalf("unsupported write error = %v", err)
	}
}

func executeConfigIDE(args ...string) (string, error) {
	var out bytes.Buffer
	cmd := NewSettingsCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("parse %s: %v\n%s", path, err, string(data))
	}
	return value
}

func objectAt(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	object, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %T, want object", key, value[key])
	}
	return object
}

func containsAnyString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
