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

package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docsErrorCodesFile is the reader-facing reference that every registered error
// code must document. The path is repository-relative because the documentation
// tree is the published artefact; ErrorCodeDocsURL points users at it.
const docsErrorCodesFile = "../../docs/reference/error-codes.md"

// TestErrorCodesAreDocumented asserts that every code in the error registry has
// an anchor in docs/reference/error-codes.md.
//
// This matters because each registry entry carries a DocsURL built by
// ErrorCodeDocsURL, so a code added without a matching heading sends the user to
// a documentation anchor that does not exist.
func TestErrorCodesAreDocumented(t *testing.T) {
	f := NewDefaultErrorFormatter()

	raw, err := os.ReadFile(filepath.Clean(docsErrorCodesFile))
	if err != nil {
		t.Fatalf("read %s: %v", docsErrorCodesFile, err)
	}
	content := string(raw)

	// Collect the headings the documentation defines, e.g. "### E1001".
	heading := regexp.MustCompile(`(?m)^### (E\d{4})$`)
	documented := map[string]bool{}
	for _, m := range heading.FindAllStringSubmatch(content, -1) {
		documented[m[1]] = true
	}

	if len(documented) == 0 {
		t.Fatalf("%s defines no error code headings", docsErrorCodesFile)
	}

	for _, code := range sortedRegistryCodes(f) {
		if !documented[code] {
			t.Errorf("error code %s is registered but has no \"### %s\" heading in %s; "+
				"ErrorCodeDocsURL(%q) would link to a missing anchor",
				code, code, docsErrorCodesFile, code)
		}
	}

	// The reverse direction catches a code that was removed from the registry
	// while its documentation page stayed behind.
	registry := map[string]bool{}
	for _, code := range sortedRegistryCodes(f) {
		registry[code] = true
	}
	for code := range documented {
		if !registry[code] {
			t.Errorf("%s documents %s, which the error registry no longer defines", docsErrorCodesFile, code)
		}
	}
}

// TestErrorCodeDocsURLsAreWellFormed checks the shape of every generated
// documentation link: absolute, inside the documentation tree, and anchored to
// the lower-cased code, which is how Markdown anchors resolve the heading.
func TestErrorCodeDocsURLsAreWellFormed(t *testing.T) {
	f := NewDefaultErrorFormatter()

	for _, code := range sortedRegistryCodes(f) {
		info, ok := f.GetErrorInfo(code)
		if !ok {
			t.Fatalf("GetErrorInfo(%q) reported a missing entry", code)
		}
		want := DocumentationBaseURL + "/reference/error-codes.md#" + strings.ToLower(code)
		if info.DocsURL != want {
			t.Errorf("DocsURL for %s = %q, want %q", code, info.DocsURL, want)
		}
		if !strings.HasPrefix(info.DocsURL, "https://") {
			t.Errorf("DocsURL for %s = %q: terminal output needs an absolute URL", code, info.DocsURL)
		}
		if strings.Contains(info.DocsURL, "docs.opencenter.cloud") {
			t.Errorf("DocsURL for %s = %q points at the unpublished hosted site", code, info.DocsURL)
		}
	}
}

// TestErrorCodesHaveRemediation checks that each registered code carries the
// guidance a user needs to recover, rather than only a description of failure.
func TestErrorCodesHaveRemediation(t *testing.T) {
	f := NewDefaultErrorFormatter()

	for _, code := range sortedRegistryCodes(f) {
		info, ok := f.GetErrorInfo(code)
		if !ok {
			t.Fatalf("GetErrorInfo(%q) reported a missing entry", code)
		}
		if strings.TrimSpace(info.Title) == "" {
			t.Errorf("%s has no title", code)
		}
		if strings.TrimSpace(info.Description) == "" {
			t.Errorf("%s has no description", code)
		}
		if strings.TrimSpace(info.Fix) == "" {
			t.Errorf("%s has no fix guidance", code)
		}
	}
}

// TestErrorRegistryOpencenterCommandsAreWellFormed checks the shape of every
// FixCommand that invokes the binary: it must name a subcommand, and its verb
// path must not contain a placeholder. Whether the verb path resolves to a real
// subcommand is verified in cmd/, which owns the Cobra tree; this package must
// not import cmd, because cmd imports ui.
func TestErrorRegistryOpencenterCommandsAreWellFormed(t *testing.T) {
	f := NewDefaultErrorFormatter()

	for _, code := range sortedRegistryCodes(f) {
		info, ok := f.GetErrorInfo(code)
		if !ok {
			t.Fatalf("GetErrorInfo(%q) reported a missing entry", code)
		}
		cmd := strings.TrimSpace(info.FixCommand)
		if !strings.HasPrefix(cmd, "opencenter ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(cmd, "opencenter "))
		if rest == "" {
			t.Errorf("FixCommand for %s = %q names no subcommand", code, info.FixCommand)
			continue
		}
		// The verb path is everything before the first flag or placeholder.
		path := rest
		if i := strings.IndexAny(path, "-{"); i >= 0 {
			path = strings.TrimSpace(path[:i])
		}
		if strings.Fields(path) == nil {
			t.Errorf("FixCommand for %s = %q has an empty verb path", code, info.FixCommand)
		}
		for _, word := range strings.Fields(path) {
			if strings.ContainsAny(word, "{}") {
				t.Errorf("FixCommand for %s = %q puts a placeholder in the verb path", code, info.FixCommand)
			}
		}
	}
}

// sortedRegistryCodes returns every registered code in lexicographic order so
// failures are reported deterministically.
func sortedRegistryCodes(f *DefaultErrorFormatter) []string {
	codes := make([]string, 0, len(f.errorRegistry))
	for code := range f.errorRegistry {
		codes = append(codes, code)
	}
	for i := 1; i < len(codes); i++ {
		for j := i; j > 0 && codes[j] < codes[j-1]; j-- {
			codes[j], codes[j-1] = codes[j-1], codes[j]
		}
	}
	return codes
}
