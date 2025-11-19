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

package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultFileValidator implements FileValidator interface
type DefaultFileValidator struct{}

// NewDefaultFileValidator creates a new default file validator
func NewDefaultFileValidator() *DefaultFileValidator {
	return &DefaultFileValidator{}
}

// ValidateFileExists validates that a file exists
func (v *DefaultFileValidator) ValidateFileExists(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filename)
	} else if err != nil {
		return fmt.Errorf("cannot access file %s: %w", filename, err)
	}

	return nil
}

// ValidateFileReadable validates that a file is readable
func (v *DefaultFileValidator) ValidateFileReadable(filename string) error {
	if err := v.ValidateFileExists(filename); err != nil {
		return err
	}

	// Try to open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("file is not readable %s: %w", filename, err)
	}
	file.Close()

	return nil
}

// ValidateFileWritable validates that a file is writable
func (v *DefaultFileValidator) ValidateFileWritable(filename string) error {
	// If file doesn't exist, check if parent directory is writable
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		parentDir := filepath.Dir(filename)
		return v.validateDirectoryWritable(parentDir)
	}

	// File exists, try to open it for writing
	file, err := os.OpenFile(filename, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("file is not writable %s: %w", filename, err)
	}
	file.Close()

	return nil
}

// ValidateFilePermissions validates that a file has expected permissions
func (v *DefaultFileValidator) ValidateFilePermissions(filename string, expectedPerm os.FileMode) error {
	if err := v.ValidateFileExists(filename); err != nil {
		return err
	}

	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("cannot get file permissions for %s: %w", filename, err)
	}

	actualPerm := info.Mode().Perm()
	if actualPerm != expectedPerm {
		return fmt.Errorf("file %s has permissions %o, expected %o", filename, actualPerm, expectedPerm)
	}

	return nil
}

// ValidateFileSize validates that a file is within size limits
func (v *DefaultFileValidator) ValidateFileSize(filename string, maxSize int64) error {
	if err := v.ValidateFileExists(filename); err != nil {
		return err
	}

	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("cannot get file size for %s: %w", filename, err)
	}

	if info.Size() > maxSize {
		return fmt.Errorf("file %s size %d exceeds maximum allowed size %d", filename, info.Size(), maxSize)
	}

	return nil
}

// ValidateFileExtension validates that a file has an allowed extension
func (v *DefaultFileValidator) ValidateFileExtension(filename string, allowedExtensions []string) error {
	if len(allowedExtensions) == 0 {
		return nil // No restrictions
	}

	ext := strings.ToLower(filepath.Ext(filename))

	for _, allowedExt := range allowedExtensions {
		if strings.ToLower(allowedExt) == ext {
			return nil
		}
	}

	return fmt.Errorf("file %s has extension %s, allowed extensions: %v", filename, ext, allowedExtensions)
}

// validateDirectoryWritable validates that a directory is writable
func (v *DefaultFileValidator) validateDirectoryWritable(dirname string) error {
	// Check if directory exists
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dirname)
	} else if err != nil {
		return fmt.Errorf("cannot access directory %s: %w", dirname, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dirname)
	}

	// Try to create a temporary file to test write permissions
	tempFile := filepath.Join(dirname, ".write_test_"+fmt.Sprintf("%d", os.Getpid()))
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("directory is not writable %s: %w", dirname, err)
	}
	file.Close()

	// Clean up test file
	if err := os.Remove(tempFile); err != nil {
		// Log warning but don't fail validation
		fmt.Printf("Warning: failed to remove test file %s: %v\n", tempFile, err)
	}

	return nil
}

// ValidateFilePath validates that a file path is safe and valid
func (v *DefaultFileValidator) ValidateFilePath(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Check for path traversal attempts
	if strings.Contains(filename, "..") {
		return fmt.Errorf("filename contains path traversal elements: %s", filename)
	}

	// Check for invalid characters
	invalidChars := []string{"\x00", "<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		if strings.Contains(filename, char) {
			return fmt.Errorf("filename contains invalid character '%s': %s", char, filename)
		}
	}

	// Check path length
	if len(filename) > 255 {
		return fmt.Errorf("filename is too long (%d characters), maximum is 255", len(filename))
	}

	return nil
}

// ValidateFileNotEmpty validates that a file is not empty
func (v *DefaultFileValidator) ValidateFileNotEmpty(filename string) error {
	if err := v.ValidateFileExists(filename); err != nil {
		return err
	}

	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("cannot get file info for %s: %w", filename, err)
	}

	if info.Size() == 0 {
		return fmt.Errorf("file is empty: %s", filename)
	}

	return nil
}

// ValidateFileIsRegular validates that a path is a regular file
func (v *DefaultFileValidator) ValidateFileIsRegular(filename string) error {
	if err := v.ValidateFileExists(filename); err != nil {
		return err
	}

	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("cannot get file info for %s: %w", filename, err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", filename)
	}

	return nil
}

// ValidateFileIsDirectory validates that a path is a directory
func (v *DefaultFileValidator) ValidateFileIsDirectory(filename string) error {
	if err := v.ValidateFileExists(filename); err != nil {
		return err
	}

	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("cannot get file info for %s: %w", filename, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", filename)
	}

	return nil
}

// ValidateFileAge validates that a file is not older than specified age in seconds
func (v *DefaultFileValidator) ValidateFileAge(filename string, maxAgeSeconds int64) error {
	if err := v.ValidateFileExists(filename); err != nil {
		return err
	}

	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("cannot get file info for %s: %w", filename, err)
	}

	fileAge := info.ModTime().Unix()
	currentTime := time.Now().Unix()

	if currentTime-fileAge > maxAgeSeconds {
		return fmt.Errorf("file %s is too old (age: %d seconds, max: %d seconds)",
			filename, currentTime-fileAge, maxAgeSeconds)
	}

	return nil
}

// ValidateMultipleFiles validates multiple files with the same criteria
func (v *DefaultFileValidator) ValidateMultipleFiles(filenames []string, validator func(string) error) error {
	for _, filename := range filenames {
		if err := validator(filename); err != nil {
			return fmt.Errorf("validation failed for file %s: %w", filename, err)
		}
	}

	return nil
}

// ValidateFilePattern validates files matching a pattern
func (v *DefaultFileValidator) ValidateFilePattern(pattern string, validator func(string) error) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("invalid file pattern %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("no files match pattern: %s", pattern)
	}

	return v.ValidateMultipleFiles(matches, validator)
}
