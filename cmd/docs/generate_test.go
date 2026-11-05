//go:build tools

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/cmd"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestGenerateDocsPreservesExistingFrontmatterAndDefaultsNewPages(t *testing.T) {
	expectedGenerationDate := currentGenerationDate()
	outputDir := filepath.Join(t.TempDir(), "reference")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	preserved := "---\nid: custom-root\ntitle: \"Custom root\"\nsidebar_label: Custom\ndescription: \"Keep this description: exactly\"\ndoc_type: reference\naudience: \"platform engineers\"\ntags: [reference, custom]\nlast_updated: \"2025-01-02\"\n---\n"
	if err := os.WriteFile(filepath.Join(outputDir, "opencenter.md"), []byte(preserved+"old body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateDocs(outputDir, cmd.NewBuiltinRootCmd()); err != nil {
		t.Fatal(err)
	}
	rootPage, err := os.ReadFile(filepath.Join(outputDir, "opencenter.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Curated frontmatter survives regeneration, with one deliberate exception:
	// last_updated is re-stamped when the generated body changed, so the timestamp
	// keeps meaning "this content was verified on". Everything else must be byte
	// identical, including key order and quoting.
	wantFrontmatter := strings.Replace(preserved,
		"last_updated: \"2025-01-02\"",
		"last_updated: \""+expectedGenerationDate+"\"", 1)
	if !strings.HasPrefix(string(rootPage), wantFrontmatter) {
		t.Fatalf("existing frontmatter was not preserved (apart from the re-dated last_updated):\n%s", rootPage)
	}

	settingsPage, err := os.ReadFile(filepath.Join(outputDir, "opencenter_settings.md"))
	if err != nil {
		t.Fatal(err)
	}
	frontmatter, ok := extractFrontmatter(settingsPage)
	if !ok {
		t.Fatal("new page has no frontmatter")
	}
	fields := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(frontmatter, "---\n"), "---\n")), &fields); err != nil {
		t.Fatalf("new page frontmatter is invalid YAML: %v\n%s", err, frontmatter)
	}
	for _, key := range []string{"id", "title", "sidebar_label", "description", "doc_type", "audience", "tags", "last_updated"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("new page frontmatter is missing %q: %s", key, frontmatter)
		}
	}
	if fields["id"] != "opencenter-settings" {
		t.Fatalf("new page id = %v, want opencenter-settings", fields["id"])
	}
	if fields["title"] != "Opencenter_Settings" || fields["sidebar_label"] != "Opencenter_Settings" {
		t.Fatalf("new page title/sidebar_label = %v/%v, want Opencenter_Settings", fields["title"], fields["sidebar_label"])
	}
	if fields["audience"] != defaultDocsAudience {
		t.Fatalf("new page audience = %v, want %q", fields["audience"], defaultDocsAudience)
	}
	if fields["last_updated"] != expectedGenerationDate {
		t.Fatalf("new page last_updated = %v, want %s", fields["last_updated"], expectedGenerationDate)
	}
}

func TestGenerateDocsRemovesStalePagesAndDoesNotDiscoverPlugins(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("OPENCENTER_CONFIG_DIR", configDir)
	outputDir := filepath.Join(t.TempDir(), "reference")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(outputDir, "opencenter_local.md")
	if err := os.WriteFile(stale, []byte("---\nid: stale\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pluginDir := filepath.Join(configDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "opencenter-external"), []byte("not a command for this generator"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := GenerateDocs(outputDir, cmd.NewBuiltinRootCmd()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale page still exists, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "opencenter_external.md")); !os.IsNotExist(err) {
		t.Fatalf("external plugin page was generated, stat error = %v", err)
	}
}

func TestGenerateDocsIsIdempotent(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "reference")
	if err := GenerateDocs(outputDir, cmd.NewBuiltinRootCmd()); err != nil {
		t.Fatal(err)
	}
	first := readGeneratedPages(t, outputDir)
	if err := GenerateDocs(outputDir, cmd.NewBuiltinRootCmd()); err != nil {
		t.Fatal(err)
	}
	second := readGeneratedPages(t, outputDir)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("generated documentation changed on second run")
	}
}

func TestGeneratedPagesCoverVisibleBuiltinTree(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "reference")
	root := cmd.NewBuiltinRootCmd()
	if err := GenerateDocs(outputDir, root); err != nil {
		t.Fatal(err)
	}
	actual := readGeneratedPages(t, outputDir)
	expected := map[string]bool{}
	walkVisibleCommands(root, func(command *cobra.Command) {
		expected[commandFilename(command)] = true
	})
	if !reflect.DeepEqual(mapKeys(actual), mapKeys(expected)) {
		t.Fatalf("generated page set does not match visible built-in command tree\nwant: %v\ngot:  %v", mapKeys(expected), mapKeys(actual))
	}
}

func readGeneratedPages(t *testing.T, directory string) map[string][]byte {
	t.Helper()
	pages := map[string][]byte{}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		pages[entry.Name()] = data
	}
	return pages
}

func mapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestGeneratedFrontmatterAudit(t *testing.T) {
	expectedGenerationDate := currentGenerationDate()
	outputDir := filepath.Join(t.TempDir(), "reference")
	if err := GenerateDocs(outputDir, cmd.NewBuiltinRootCmd()); err != nil {
		t.Fatal(err)
	}
	slug := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	for filename, page := range readGeneratedPages(t, outputDir) {
		frontmatter, ok := extractFrontmatter(page)
		if !ok {
			t.Errorf("%s: missing frontmatter", filename)
			continue
		}
		fields := map[string]interface{}{}
		contents := strings.TrimSuffix(strings.TrimPrefix(frontmatter, "---\n"), "---\n")
		if err := yaml.Unmarshal([]byte(contents), &fields); err != nil {
			t.Errorf("%s: invalid YAML frontmatter: %v", filename, err)
			continue
		}
		for _, key := range []string{"id", "title", "sidebar_label", "description", "doc_type", "audience", "tags", "last_updated"} {
			if _, ok := fields[key]; !ok {
				t.Errorf("%s: missing frontmatter key %q", filename, key)
			}
		}
		id, _ := fields["id"].(string)
		if !slug.MatchString(id) {
			t.Errorf("%s: id %q is not a lowercase slug", filename, id)
		}
		if fields["doc_type"] != "reference" {
			t.Errorf("%s: doc_type = %v, want reference", filename, fields["doc_type"])
		}
		lastUpdated, _ := fields["last_updated"].(string)
		if lastUpdated != expectedGenerationDate {
			t.Errorf("%s: last_updated = %q, want %s", filename, lastUpdated, expectedGenerationDate)
		}
		if tags, ok := fields["tags"].([]interface{}); !ok || len(tags) == 0 {
			t.Errorf("%s: tags must be a non-empty YAML sequence", filename)
		}
	}
}

// TestGenerateDocsKeepsTimestampWhenBodyUnchanged pins the property that makes
// regeneration idempotent: a page whose generated body is identical to the one
// already on disk keeps its existing last_updated. Without it, every
// `mise run docs-gen` re-dates all command pages and
// `mise run test-docs-idempotency` fails on the second run.
func TestGenerateDocsKeepsTimestampWhenBodyUnchanged(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "reference")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Generate from an empty directory so the body is exactly what the generator
	// produces, then backdate the page.
	if err := GenerateDocs(outputDir, cmd.NewBuiltinRootCmd()); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(outputDir, "opencenter_settings.md")
	before, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	backdated := strings.Replace(string(before),
		"last_updated: \""+currentGenerationDate()+"\"",
		"last_updated: \"2020-01-01\"", 1)
	if backdated == string(before) {
		t.Fatal("fixture page carries no current-generation date to backdate")
	}
	if err := os.WriteFile(page, []byte(backdated), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateDocs(outputDir, cmd.NewBuiltinRootCmd()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != backdated {
		t.Errorf("unchanged body was re-dated; regeneration is not idempotent\nbefore:\n%s\nafter:\n%s",
			backdated, after)
	}
}

// TestEnsurePurposeLineAnchorsOnCobraHeading covers the heading-level detail that
// silently disabled the feature once: Cobra renders command pages with "## ", not
// "# ", so a matcher looking only for an H1 inserts nothing and every generated
// page ships without the mandated line.
func TestEnsurePurposeLineAnchorsOnCobraHeading(t *testing.T) {
	page := []byte("---\nid: x\n---\n## opencenter cluster reconcile\n\nShort description\n")
	got := string(ensurePurposeLine(page, "opencenter_cluster_reconcile.md"))
	if !strings.Contains(got, "**Purpose:**") {
		t.Fatalf("no Purpose line inserted for an H2 page:\n%s", got)
	}
	if !strings.Contains(got, "`opencenter cluster reconcile`") {
		t.Errorf("Purpose line does not name the command derived from the filename:\n%s", got)
	}
	if strings.Index(got, "**Purpose:**") < strings.Index(got, "## opencenter") {
		t.Errorf("Purpose line was inserted before the heading:\n%s", got)
	}
	if again := string(ensurePurposeLine([]byte(got), "opencenter_cluster_reconcile.md")); again != got {
		t.Error("ensurePurposeLine rewrote a page that already had a Purpose line")
	}
}

// TestCommandPathFromFilename covers the root command, which must not gain a
// doubled binary name.
func TestCommandPathFromFilename(t *testing.T) {
	cases := map[string]string{
		"opencenter.md":                         "opencenter",
		"opencenter_cluster_reconcile.md":       "opencenter cluster reconcile",
		"opencenter_cluster_drift_reconcile.md": "opencenter cluster drift reconcile",
	}
	for filename, want := range cases {
		if got := commandPathFromFilename(filename); got != want {
			t.Errorf("commandPathFromFilename(%q) = %q, want %q", filename, got, want)
		}
	}
}
