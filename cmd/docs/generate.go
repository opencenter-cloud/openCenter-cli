// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

const defaultDocsAudience = "operators, developers"

func currentGenerationDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

func main() {
	if err := GenerateDocs("docs/reference/opencenter", cmd.NewBuiltinRootCmd()); err != nil {
		panic(fmt.Errorf("generate documentation: %w", err))
	}
}

// GenerateDocs writes the visible built-in Cobra command tree to outputDir.
// The old directory is retained until generation succeeds, then replaced as a
// whole so stale generated pages cannot survive and failed generation is safe.
func GenerateDocs(outputDir string, root *cobra.Command) error {
	if root == nil {
		return fmt.Errorf("root command is nil")
	}

	parent := filepath.Dir(outputDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create documentation parent: %w", err)
	}
	stage, err := os.MkdirTemp(parent, ".opencenter-reference-")
	if err != nil {
		return fmt.Errorf("create documentation staging directory: %w", err)
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stage)
		}
	}()

	preserved, err := existingFrontmatter(outputDir)
	if err != nil {
		return err
	}
	generationDate := currentGenerationDate()
	metadata := commandMetadata(root, generationDate)
	// Curated frontmatter is preserved verbatim. last_updated is corrected later,
	// in normalizeGeneratedPages, once the new body exists and can be compared
	// against the body it replaced.
	prepender := func(filename string) string {
		base := filepath.Base(filename)
		if frontmatter, ok := preserved[base]; ok {
			return frontmatter
		}
		return metadata[base]
	}
	identity := func(link string) string { return link }
	if err := doc.GenMarkdownTreeCustom(root, stage, prepender, identity); err != nil {
		return fmt.Errorf("generate markdown tree: %w", err)
	}
	if err := normalizeGeneratedPages(stage, previousBodies(outputDir), generationDate); err != nil {
		return err
	}

	if err := replaceGeneratedDirectory(stage, outputDir); err != nil {
		return err
	}
	keepStage = true
	return nil
}

// commandMetadata builds the frontmatter block for every visible command, dated
// generationDate. The date is a parameter rather than a time.Now() call inside so
// a single generation run cannot straddle midnight and emit two dates.
func commandMetadata(root *cobra.Command, generationDate string) map[string]string {
	metadata := make(map[string]string)
	walkVisibleCommands(root, func(command *cobra.Command) {
		filename := commandFilename(command)
		id := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
		id = strings.ReplaceAll(id, "_", "-")
		displayName := commandDisplayName(command.CommandPath())
		metadata[filename] = fmt.Sprintf(
			"---\nid: %s\ntitle: %s\nsidebar_label: %s\ndescription: %s\ndoc_type: reference\naudience: %s\ntags: [cli, reference]\nlast_updated: %s\n---\n",
			yamlQuote(id), yamlQuote(displayName), yamlQuote(displayName), yamlQuote(command.Short), yamlQuote(defaultDocsAudience), yamlQuote(generationDate),
		)
	})
	return metadata
}

func walkVisibleCommands(root *cobra.Command, visit func(*cobra.Command)) {
	if root.IsAvailableCommand() || root.Parent() == nil {
		visit(root)
	}
	for _, child := range root.Commands() {
		if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		walkVisibleCommands(child, visit)
	}
}

func commandFilename(command *cobra.Command) string {
	name := strings.ReplaceAll(command.CommandPath(), " ", "_")
	return name + ".md"
}

func commandDisplayName(commandPath string) string {
	parts := strings.Fields(commandPath)
	for i, part := range parts {
		words := strings.Fields(strings.ReplaceAll(part, "-", " "))
		for j, word := range words {
			if len(word) == 0 {
				continue
			}
			words[j] = strings.ToUpper(word[:1]) + word[1:]
		}
		parts[i] = strings.Join(words, " ")
	}
	return strings.Join(parts, "_")
}

func yamlQuote(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\t", "\\t")
	return "\"" + value + "\""
}

func existingFrontmatter(outputDir string) (map[string]string, error) {
	result := make(map[string]string)
	entries, err := os.ReadDir(outputDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read existing documentation directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(outputDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read existing documentation page %s: %w", entry.Name(), err)
		}
		if frontmatter, ok := extractFrontmatter(data); ok {
			result[entry.Name()] = frontmatter
		}
	}
	return result, nil
}

// previousBodies returns the body (the text after the frontmatter block) of every
// page currently in outputDir, keyed by filename. A missing directory yields an
// empty map, which makes every generated page look new and therefore current-dated.
func previousBodies(outputDir string) map[string]string {
	result := make(map[string]string)
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(outputDir, entry.Name()))
		if err != nil {
			continue
		}
		if block, ok := extractFrontmatter(data); ok {
			result[entry.Name()] = normalizeBody(string(data[len(block):]))
		}
	}
	return result
}

func extractFrontmatter(data []byte) (string, bool) {
	if !strings.HasPrefix(string(data), "---") {
		return "", false
	}
	separator := strings.Index(string(data[3:]), "\n---")
	if separator < 0 {
		return "", false
	}
	end := 3 + separator + len("\n---")
	if end < len(data) && data[end] == '\r' {
		end++
	}
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return string(data[:end]), true
}

func replaceGeneratedDirectory(stage, outputDir string) error {
	backup, err := os.MkdirTemp(filepath.Dir(outputDir), ".opencenter-reference-backup-")
	if err != nil {
		return fmt.Errorf("create documentation backup: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("prepare documentation backup: %w", err)
	}
	oldMoved := false
	if _, err := os.Stat(outputDir); err == nil {
		if err := os.Rename(outputDir, backup); err != nil {
			return fmt.Errorf("move existing documentation directory: %w", err)
		}
		oldMoved = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat existing documentation directory: %w", err)
	}

	if err := os.Rename(stage, outputDir); err != nil {
		if oldMoved {
			_ = os.Rename(backup, outputDir)
		}
		return fmt.Errorf("install generated documentation directory: %w", err)
	}
	if oldMoved {
		_ = os.RemoveAll(backup)
	}
	return nil
}

// normalizeGeneratedPages trims trailing whitespace, inserts the mandated
// **Purpose:** line, and re-dates pages whose generated body actually changed.
//
// The re-dating fixes a real defect: the frontmatter of a curated page was
// preserved verbatim on every run, so a page whose command documentation changed
// kept its original last_updated forever. 144 pages read 2026-09-24 despite later
// regeneration. Comparing the body rather than rewriting the date unconditionally
// keeps `mise run test-docs-idempotency` green: a second run over unchanged
// content produces byte-identical output.
func normalizeGeneratedPages(directory string, previous map[string]string, generationDate string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read staged documentation: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read staged documentation page %s: %w", entry.Name(), err)
		}
		trimmed := bytes.TrimRight(data, "\r\n")
		trimmed = append(trimmed, '\n')
		trimmed = ensurePurposeLine(trimmed, entry.Name())
		trimmed = []byte(redateIfChanged(string(trimmed), previous[entry.Name()], generationDate))
		if err := os.WriteFile(path, trimmed, 0o644); err != nil {
			return fmt.Errorf("normalize staged documentation page %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// ensurePurposeLine inserts the "**Purpose:**" sentence that docs/README.md
// mandates, immediately after the H1. Generated pages derive it from the command
// path so the line cannot drift from the command it documents. Pages that already
// carry one are returned untouched, preserving hand-written wording across
// regenerations.
func ensurePurposeLine(data []byte, filename string) []byte {
	text := string(data)
	if strings.Contains(text, "**Purpose:**") {
		return data
	}
	// Cobra renders each command page heading as "## <command path>", not an H1,
	// so the first ATX heading of any level is the anchor.
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		purpose := "**Purpose:** For " + defaultDocsAudience +
			", documents `" + commandPathFromFilename(filename) + "`: what it does, its flags, and its examples."
		out := append([]string{}, lines[:i+1]...)
		out = append(out, "", purpose)
		out = append(out, lines[i+1:]...)
		return []byte(strings.Join(out, "\n"))
	}
	// No heading at all: leave the page exactly as the generator emitted it.
	return data
}

// redateIfChanged rewrites the last_updated key to generationDate when body
// differs from previousBody, and returns text unchanged otherwise. A page with no
// previous body is new, so it is stamped. Only the date value is touched; key
// order and every other frontmatter field are preserved.
func redateIfChanged(text, previousBody, generationDate string) string {
	block, ok := extractFrontmatter([]byte(text))
	if !ok {
		return text
	}
	body := normalizeBody(text[len(block):])
	if previousBody != "" && previousBody == body {
		return text
	}
	updated := setLastUpdated(block, generationDate)
	return updated + text[len(block):]
}

// setLastUpdated replaces the last_updated value inside a frontmatter block. A
// block without the key is returned unchanged rather than having one appended, so
// the generator cannot silently invent metadata the audit does not expect.
func setLastUpdated(block, generationDate string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "last_updated:") {
			continue
		}
		lines[i] = "last_updated: " + yamlQuote(generationDate)
		return strings.Join(lines, "\n")
	}
	return block
}

// commandPathFromFilename recovers the CLI command path from a generated page
// filename. The generator names each page after Cobra's CommandPath with spaces
// replaced by underscores, which round-trips exactly -- unlike the rendered title,
// which is title-cased and hyphen-split and cannot be reversed reliably.
func commandPathFromFilename(filename string) string {
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	return strings.ReplaceAll(stem, "_", " ")
}

// normalizeBody collapses trailing whitespace and CRLF so a body comparison
// reflects content rather than line-ending noise.
func normalizeBody(body string) string {
	return strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
}
