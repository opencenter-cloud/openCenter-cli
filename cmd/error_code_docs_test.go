/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
)

// fixCommandPlaceholder matches the {cluster}, {path}, {host} and {service}
// placeholders that the error registry substitutes at render time.
var fixCommandPlaceholder = regexp.MustCompile(`\{[a-z]+\}`)

// TestErrorRegistryFixCommandsResolveToRealCommands walks the built-in Cobra tree
// and asserts that every error-registry FixCommand invoking the CLI names a
// command that actually exists.
//
// This is the check that would have caught the registry recommending
// "opencenter cluster lock break", a subcommand the CLI never implemented: a user
// following the printed remediation gets "unknown command", which turns a
// recoverable error into a dead end.
func TestErrorRegistryFixCommandsResolveToRealCommands(t *testing.T) {
	paths := collectCommandPaths(NewBuiltinRootCmd())
	formatter := ui.NewDefaultErrorFormatter()

	checked := 0
	for _, code := range registryCodes(t, formatter) {
		info, ok := formatter.GetErrorInfo(code)
		if !ok {
			t.Fatalf("GetErrorInfo(%q) reported a missing entry", code)
		}
		cmd := strings.TrimSpace(info.FixCommand)
		if !strings.HasPrefix(cmd, "opencenter ") {
			continue // shell diagnostics are not CLI invocations
		}
		verbs := commandVerbs(cmd)
		checked++
		if len(verbs) == 0 {
			t.Errorf("FixCommand for %s = %q names no subcommand", code, info.FixCommand)
			continue
		}
		if paths[strings.Join(verbs, " ")] {
			continue
		}
		// The full verb run is not a command. It is still acceptable when the
		// leading verbs resolve and the remainder is a positional argument, which
		// is how "cluster init my-cluster" and "cluster set {cluster} key=value"
		// read. Anything else means a recommended subcommand is missing.
		matched := longestCommandPrefix(paths, verbs)
		if matched == 0 {
			t.Errorf("FixCommand for %s recommends %q; %q is not a command in the built-in tree",
				code, info.FixCommand, strings.Join(verbs, " "))
			continue
		}
		trailing := verbs[matched:]
		if !allArguments(trailing) {
			t.Errorf("FixCommand for %s recommends %q; %q is not a subcommand of %q",
				code, info.FixCommand, strings.Join(trailing, " "), strings.Join(verbs[:matched], " "))
		}
	}

	if checked == 0 {
		t.Fatal("no error-registry FixCommand invoked the CLI; the test is not exercising anything")
	}
	t.Logf("verified %d error-registry fix commands against the built-in command tree", checked)
}

// TestDocumentationBaseURLIsReachableShape asserts the base URL the CLI prints is
// an absolute URL into the repository's docs tree. The project publishes no hosted
// documentation site, so a bare domain here would send users nowhere.
func TestDocumentationBaseURLIsReachableShape(t *testing.T) {
	const want = "https://github.com/opencenter-cloud/opencenter-cli/blob/main/docs"
	if ui.DocumentationBaseURL != want {
		t.Errorf("ui.DocumentationBaseURL = %q, want %q", ui.DocumentationBaseURL, want)
	}
	if strings.Contains(ui.DocumentationBaseURL, "docs.opencenter.cloud") {
		t.Error("documentation URL points at the unpublished hosted site")
	}
	// The root long description interpolates the constant, so the printed help
	// must not still contain the unpublished domain.
	long := NewBuiltinRootCmd().Long
	if strings.Contains(long, "docs.opencenter.cloud") {
		t.Error("root help text still advertises the unpublished docs.opencenter.cloud site")
	}
	if !strings.Contains(long, ui.DocumentationBaseURL) {
		t.Error("root help text does not advertise ui.DocumentationBaseURL")
	}
}

// commandVerbs returns the leading non-flag tokens of an "opencenter ..." command
// line, with {placeholder} arguments removed.
func commandVerbs(cmd string) []string {
	rest := fixCommandPlaceholder.ReplaceAllString(strings.TrimPrefix(cmd, "opencenter "), "")
	var verbs []string
	for _, field := range strings.Fields(rest) {
		if strings.HasPrefix(field, "-") {
			break
		}
		verbs = append(verbs, field)
	}
	return verbs
}

// longestCommandPrefix returns how many leading verbs name a command in paths.
func longestCommandPrefix(paths map[string]bool, verbs []string) int {
	for n := len(verbs); n > 0; n-- {
		if paths[strings.Join(verbs[:n], " ")] {
			return n
		}
	}
	return 0
}

// allArguments reports whether every token is a positional argument rather than a
// candidate subcommand name. Placeholders were already stripped, so an argument is
// recognised by being a key=value assignment or a name that cannot be a command
// word, such as "my-cluster" or "{cluster}"-shaped input.
func allArguments(tokens []string) bool {
	for _, tok := range tokens {
		if strings.Contains(tok, "=") {
			continue
		}
		// A bare word that is not a known command is treated as an argument only
		// when it looks like a user-supplied name rather than a CLI verb: verbs are
		// single lowercase words without hyphens, names are not.
		if strings.Contains(tok, "-") || !isLowerWord(tok) {
			continue
		}
		return false
	}
	return true
}

// isLowerWord reports whether s is a single lowercase alphabetic word.
func isLowerWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// collectCommandPaths returns every command path reachable from root, keyed by the
// space-separated verb sequence a user types (for example "cluster backup create").
func collectCommandPaths(root *cobra.Command) map[string]bool {
	paths := map[string]bool{}
	var walk func(c *cobra.Command, prefix []string)
	walk = func(c *cobra.Command, prefix []string) {
		if len(prefix) > 0 {
			paths[strings.Join(prefix, " ")] = true
		}
		for _, sub := range c.Commands() {
			if sub.Hidden {
				continue
			}
			walk(sub, append(append([]string{}, prefix...), sub.Name()))
		}
	}
	walk(root, nil)
	return paths
}

// registryCodes returns the codes the error formatter knows about, sorted for
// deterministic failure output.
func registryCodes(t *testing.T, f *ui.DefaultErrorFormatter) []string {
	t.Helper()
	// The registry map is unexported, so enumerate through GetErrorInfo over the
	// code space rather than reaching into the struct. Codes are "E" + a group
	// digit + a three-digit sequence, e.g. E1001.
	var codes []string
	for group := range 6 {
		for seq := range 1000 {
			code := fmt.Sprintf("E%d%03d", group+1, seq)
			if _, ok := f.GetErrorInfo(code); ok {
				codes = append(codes, code)
			}
		}
	}
	sort.Strings(codes)
	if len(codes) == 0 {
		t.Fatal("error registry appears empty")
	}
	return codes
}

// TestEmittedDocURLAnchorsExist checks that every documentation URL the CLI can
// print resolves to a real page and, when it carries a fragment, to a heading in
// that page. A constant assembled by hand can name an anchor that no heading
// defines, and the link only fails when a user clicks it during an error.
func TestEmittedDocURLAnchorsExist(t *testing.T) {
	const base = "https://github.com/opencenter-cloud/opencenter-cli/blob/main/docs/"

	urls := map[string]string{
		"ui.SopsEncryptionDocsURL": ui.SopsEncryptionDocsURL,
	}
	for _, code := range registryCodes(t, ui.NewDefaultErrorFormatter()) {
		info, ok := ui.NewDefaultErrorFormatter().GetErrorInfo(code)
		if !ok {
			t.Fatalf("GetErrorInfo(%q) reported a missing entry", code)
		}
		urls["error registry "+code] = info.DocsURL
	}

	for name, raw := range urls {
		if !strings.HasPrefix(raw, base) {
			t.Errorf("%s = %q is not a blob URL into the docs tree", name, raw)
			continue
		}
		relative := strings.TrimPrefix(raw, base)
		path, fragment, _ := strings.Cut(relative, "#")
		// Tests run with the package directory as the working directory.
		page := filepath.Join("..", "docs", filepath.FromSlash(path))
		data, err := os.ReadFile(page)
		if err != nil {
			t.Errorf("%s points at %s which cannot be read: %v", name, page, err)
			continue
		}
		if fragment == "" {
			continue
		}
		if !markdownHasAnchor(string(data), fragment) {
			t.Errorf("%s = %q anchors to #%q, which no heading in %s defines", name, raw, fragment, page)
		}
	}
}

// markdownHasAnchor reports whether the document yields the wanted anchor. GitHub
// builds an anchor from a heading by lower-casing it, dropping punctuation, and
// replacing runs of whitespace with a single hyphen.
func markdownHasAnchor(document, want string) bool {
	for _, line := range strings.Split(document, "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		var b strings.Builder
		lastWasDash := false
		for _, r := range strings.ToLower(heading) {
			switch {
			case r == ' ' || r == '\t' || r == '-' || r == '_' || r == '.' || r == ',' || r == ':' || r == ';':
				if !lastWasDash && b.Len() > 0 {
					b.WriteByte('-')
					lastWasDash = true
				}
			case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
				b.WriteRune(r)
				lastWasDash = false
			}
		}
		if strings.TrimSuffix(b.String(), "-") == want {
			return true
		}
	}
	return false
}
