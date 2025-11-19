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

package paths

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultPathExpander implements PathExpander interface
type DefaultPathExpander struct{}

// NewDefaultPathExpander creates a new default path expander
func NewDefaultPathExpander() *DefaultPathExpander {
	return &DefaultPathExpander{}
}

// ExpandPath expands environment variables and user home directory in a path
func (e *DefaultPathExpander) ExpandPath(path string) string {
	// First expand environment variables
	expanded := e.ExpandEnvironmentVariables(path)

	// Then expand user home directory
	expanded = e.ExpandUserHome(expanded)

	return expanded
}

// ExpandEnvironmentVariables expands environment variables in a path
func (e *DefaultPathExpander) ExpandEnvironmentVariables(path string) string {
	return os.ExpandEnv(path)
}

// ExpandUserHome expands tilde (~) to user home directory
func (e *DefaultPathExpander) ExpandUserHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path // Return original path if we can't get home directory
	}

	if path == "~" {
		return homeDir
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}

	return path
}

// ResolvePath resolves a path to its absolute form with all expansions applied
func (e *DefaultPathExpander) ResolvePath(path string) (string, error) {
	// Expand the path
	expanded := e.ExpandPath(path)

	// Convert to absolute path
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}

	// Clean the path to remove any redundant elements
	return filepath.Clean(absPath), nil
}

// ExpandPathWithDefaults expands a path with default fallback
func ExpandPathWithDefaults(path, defaultPath string) string {
	if path == "" {
		path = defaultPath
	}

	expander := NewDefaultPathExpander()
	return expander.ExpandPath(path)
}

// IsAbsolutePath checks if a path is absolute after expansion
func IsAbsolutePath(path string) bool {
	expander := NewDefaultPathExpander()
	expanded := expander.ExpandPath(path)
	return filepath.IsAbs(expanded)
}

// JoinAndExpand joins path elements and expands the result
func JoinAndExpand(elements ...string) string {
	joined := filepath.Join(elements...)
	expander := NewDefaultPathExpander()
	return expander.ExpandPath(joined)
}

// NormalizePath normalizes a path by expanding and cleaning it
func NormalizePath(path string) (string, error) {
	expander := NewDefaultPathExpander()
	resolved, err := expander.ResolvePath(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
