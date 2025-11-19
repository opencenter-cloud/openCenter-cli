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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultPathValidator implements PathValidator interface
type DefaultPathValidator struct {
	expander PathExpander
}

// NewDefaultPathValidator creates a new default path validator
func NewDefaultPathValidator() *DefaultPathValidator {
	return &DefaultPathValidator{
		expander: NewDefaultPathExpander(),
	}
}

// ValidatePath validates that a path is safe and accessible
func (v *DefaultPathValidator) ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Expand the path first
	expandedPath := v.expander.ExpandPath(path)

	// Check for path traversal attempts
	if strings.Contains(expandedPath, "..") {
		return fmt.Errorf("path contains directory traversal elements: %s", path)
	}

	// Check if the path is absolute after expansion
	if !filepath.IsAbs(expandedPath) {
		return fmt.Errorf("path must be absolute after expansion: %s", expandedPath)
	}

	// Check for invalid characters (platform-specific)
	if err := v.validatePathCharacters(expandedPath); err != nil {
		return fmt.Errorf("path contains invalid characters: %w", err)
	}

	return nil
}

// ValidateDirectoryPermissions validates that a directory has proper read/write permissions
func (v *DefaultPathValidator) ValidateDirectoryPermissions(dir string) error {
	// Check if directory exists
	stat, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("cannot access directory: %w", err)
	}

	if !stat.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
	}

	// Test write permissions by creating a temporary file
	testFile := filepath.Join(dir, ".openCenter_permission_test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory: %w", err)
	}
	file.Close()

	// Clean up test file
	if err := os.Remove(testFile); err != nil {
		// Log warning but don't fail - the directory is writable
		fmt.Printf("Warning: failed to remove test file %s: %v\n", testFile, err)
	}

	return nil
}

// ValidateClusterName validates a cluster name according to openCenter conventions
func (v *DefaultPathValidator) ValidateClusterName(name string) error {
	if name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	// Check length constraints
	if len(name) < 2 {
		return fmt.Errorf("cluster name must be at least 2 characters long")
	}
	if len(name) > 63 {
		return fmt.Errorf("cluster name must be no more than 63 characters long")
	}

	// Check for valid characters (alphanumeric, hyphens, dots)
	validNameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]*[a-zA-Z0-9]$`)
	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("cluster name must start and end with alphanumeric characters and contain only letters, numbers, hyphens, and dots")
	}

	// Check for reserved names
	reservedNames := []string{
		".", "..", "con", "prn", "aux", "nul",
		"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
		"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
	}

	lowerName := strings.ToLower(name)
	for _, reserved := range reservedNames {
		if lowerName == reserved {
			return fmt.Errorf("cluster name '%s' is reserved and cannot be used", name)
		}
	}

	// Check for consecutive hyphens or dots
	if strings.Contains(name, "--") || strings.Contains(name, "..") {
		return fmt.Errorf("cluster name cannot contain consecutive hyphens or dots")
	}

	return nil
}

// ValidateOrganizationName validates an organization name according to openCenter conventions
func (v *DefaultPathValidator) ValidateOrganizationName(name string) error {
	if name == "" {
		return fmt.Errorf("organization name cannot be empty")
	}

	// Use the same validation rules as cluster names for consistency
	return v.ValidateClusterName(name)
}

// ValidatePathIsDirectory validates that a path exists and is a directory
func (v *DefaultPathValidator) ValidatePathIsDirectory(path string) error {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", path)
	} else if err != nil {
		return fmt.Errorf("cannot access directory: %w", err)
	}

	if !stat.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	return nil
}

// ValidatePathIsFile validates that a path exists and is a file
func (v *DefaultPathValidator) ValidatePathIsFile(path string) error {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", path)
	} else if err != nil {
		return fmt.Errorf("cannot access file: %w", err)
	}

	if stat.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}

	return nil
}

// validatePathCharacters validates that a path doesn't contain invalid characters
func (v *DefaultPathValidator) validatePathCharacters(path string) error {
	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null bytes")
	}

	// Check for control characters
	for _, char := range path {
		if char < 32 && char != '\t' && char != '\n' && char != '\r' {
			return fmt.Errorf("path contains control character: %q", char)
		}
	}

	// Platform-specific invalid characters
	invalidChars := getInvalidPathCharacters()
	for _, invalidChar := range invalidChars {
		if strings.ContainsRune(path, invalidChar) {
			return fmt.Errorf("path contains invalid character: %q", invalidChar)
		}
	}

	return nil
}

// getInvalidPathCharacters returns platform-specific invalid path characters
func getInvalidPathCharacters() []rune {
	// Common invalid characters across platforms
	// Windows has more restrictions, but we'll use a conservative set
	return []rune{'<', '>', ':', '"', '|', '?', '*'}
}
