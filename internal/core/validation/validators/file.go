// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validators

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// FileValidator validates file paths and file system operations.
type FileValidator struct {
	allowedExtensions map[string]bool
	maxPathLength     int
}

// NewFileValidator creates a new file validator.
func NewFileValidator() *FileValidator {
	return &FileValidator{
		allowedExtensions: map[string]bool{
			".yaml": true,
			".yml":  true,
			".json": true,
			".txt":  true,
			".md":   true,
			".sh":   true,
			".key":  true,
			".pub":  true,
			".pem":  true,
			".crt":  true,
			".conf": true,
			".toml": true,
		},
		maxPathLength: 4096, // Maximum path length on most systems
	}
}

// Name returns the validator name.
func (v *FileValidator) Name() string {
	return "file"
}

// Validate validates a file path or file operation.
// The value should be a map with "operation" and "path" keys.
func (v *FileValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := &validation.ValidationResult{
		Valid:    true,
		Errors:   []*validation.ValidationIssue{},
		Warnings: []*validation.ValidationIssue{},
		Info:     []*validation.ValidationIssue{},
	}

	// Handle both string (simple path) and map (operation + path)
	var path string
	var operation string

	switch v := value.(type) {
	case string:
		path = v
		operation = "validate"
	case map[string]interface{}:
		pathVal, ok := v["path"]
		if !ok {
			result.AddError("file", "missing 'path' field")
			return result, nil
		}
		path, ok = pathVal.(string)
		if !ok {
			result.AddError("file", "path must be a string")
			return result, nil
		}

		if opVal, ok := v["operation"]; ok {
			operation, _ = opVal.(string)
		} else {
			operation = "validate"
		}
	default:
		result.AddError("file", "value must be a string or map with 'path' field")
		return result, nil
	}

	// Validate the path
	v.validatePath(result, path)

	// Perform operation-specific validation
	switch operation {
	case "read":
		v.validateReadOperation(result, path)
	case "write":
		v.validateWriteOperation(result, path)
	case "delete":
		v.validateDeleteOperation(result, path)
	case "validate":
		// Basic validation already done
	default:
		result.AddWarning("file", fmt.Sprintf("unknown operation '%s', performing basic validation only", operation))
	}

	return result, nil
}

// validatePath validates a file path for security issues.
func (v *FileValidator) validatePath(result *validation.ValidationResult, path string) {
	if path == "" {
		result.AddError("path", "path cannot be empty")
		return
	}

	// Check path length
	if len(path) > v.maxPathLength {
		result.AddError("path",
			fmt.Sprintf("path is too long (%d characters, maximum is %d)", len(path), v.maxPathLength),
			fmt.Sprintf("Shorten the path to %d characters or less", v.maxPathLength))
		return
	}

	// Check for path traversal sequences
	if strings.Contains(path, "..") {
		result.AddError("path", "path cannot contain path traversal sequences (..)",
			"Remove '..' from the path",
			"Use absolute paths or paths relative to a known base directory")
		return
	}

	// Clean the path and check if it changed significantly
	cleanPath := filepath.Clean(path)
	if cleanPath != path && strings.Contains(path, "..") {
		result.AddError("path", "path contains suspicious sequences that resolve differently",
			fmt.Sprintf("Path '%s' resolves to '%s'", path, cleanPath),
			"Use the cleaned path directly")
		return
	}

	// Check for null bytes (security issue)
	if strings.Contains(path, "\x00") {
		result.AddError("path", "path contains null bytes",
			"Remove null bytes from the path")
		return
	}

	// Warn about absolute paths
	if filepath.IsAbs(path) {
		result.AddWarning("path", "using absolute path",
			"Consider using relative paths for portability")
	}

	// Check file extension
	ext := filepath.Ext(path)
	if ext != "" && !v.allowedExtensions[ext] {
		result.AddWarning("path",
			fmt.Sprintf("file extension '%s' is not in the common allowed list", ext),
			"Ensure this file type is expected for your use case")
	}

	// Warn about hidden files
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != "." && base != ".." {
		result.AddInfo("path", "path refers to a hidden file (starts with '.')")
	}
}

// validateReadOperation validates a file read operation.
func (v *FileValidator) validateReadOperation(result *validation.ValidationResult, path string) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddError("read", fmt.Sprintf("file does not exist: %s", path),
				"Ensure the file exists before attempting to read it",
				"Check the file path for typos")
		} else if os.IsPermission(err) {
			result.AddError("read", fmt.Sprintf("permission denied: %s", path),
				"Ensure you have read permissions for this file",
				"Check file permissions with 'ls -l'")
		} else {
			result.AddError("read", fmt.Sprintf("cannot access file: %v", err))
		}
		return
	}

	// Check if it's a directory
	if info.IsDir() {
		result.AddError("read", "path is a directory, not a file",
			"Specify a file path, not a directory")
		return
	}

	// Check file size
	const maxFileSize = 100 * 1024 * 1024 // 100 MB
	if info.Size() > maxFileSize {
		result.AddWarning("read",
			fmt.Sprintf("file is very large (%d bytes, %.2f MB)", info.Size(), float64(info.Size())/(1024*1024)),
			"Reading large files may consume significant memory",
			"Consider streaming or processing the file in chunks")
	}

	// Check if file is readable
	file, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			result.AddError("read", "file is not readable (permission denied)",
				"Ensure you have read permissions for this file")
		} else {
			result.AddError("read", fmt.Sprintf("cannot open file: %v", err))
		}
		return
	}
	file.Close()
}

// validateWriteOperation validates a file write operation.
func (v *FileValidator) validateWriteOperation(result *validation.ValidationResult, path string) {
	// Check if file exists
	info, err := os.Stat(path)
	if err == nil {
		// File exists
		if info.IsDir() {
			result.AddError("write", "path is a directory, not a file",
				"Specify a file path, not a directory")
			return
		}

		// Check if file is writable
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			if os.IsPermission(err) {
				result.AddError("write", "file is not writable (permission denied)",
					"Ensure you have write permissions for this file",
					"Check file permissions with 'ls -l'")
			} else {
				result.AddError("write", fmt.Sprintf("cannot open file for writing: %v", err))
			}
			return
		}
		file.Close()

		result.AddWarning("write", "file already exists and will be overwritten",
			"Ensure you want to overwrite the existing file",
			"Consider backing up the file first")
	} else if os.IsNotExist(err) {
		// File doesn't exist, check if directory is writable
		dir := filepath.Dir(path)
		dirInfo, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				result.AddError("write", fmt.Sprintf("parent directory does not exist: %s", dir),
					"Create the parent directory first",
					fmt.Sprintf("Run: mkdir -p %s", dir))
			} else if os.IsPermission(err) {
				result.AddError("write", fmt.Sprintf("cannot access parent directory: %s", dir),
					"Ensure you have permissions for the parent directory")
			} else {
				result.AddError("write", fmt.Sprintf("cannot access parent directory: %v", err))
			}
			return
		}

		if !dirInfo.IsDir() {
			result.AddError("write", fmt.Sprintf("parent path is not a directory: %s", dir))
			return
		}

		// Try to create a temporary file to check write permissions
		tempFile := filepath.Join(dir, ".opencenter-write-test")
		file, err := os.Create(tempFile)
		if err != nil {
			if os.IsPermission(err) {
				result.AddError("write", fmt.Sprintf("directory is not writable: %s", dir),
					"Ensure you have write permissions for the directory")
			} else {
				result.AddError("write", fmt.Sprintf("cannot write to directory: %v", err))
			}
			return
		}
		file.Close()
		os.Remove(tempFile)
	} else {
		result.AddError("write", fmt.Sprintf("cannot access path: %v", err))
	}
}

// validateDeleteOperation validates a file delete operation.
func (v *FileValidator) validateDeleteOperation(result *validation.ValidationResult, path string) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddError("delete", fmt.Sprintf("file does not exist: %s", path),
				"Cannot delete a file that doesn't exist")
		} else if os.IsPermission(err) {
			result.AddError("delete", fmt.Sprintf("permission denied: %s", path),
				"Ensure you have permissions to access this file")
		} else {
			result.AddError("delete", fmt.Sprintf("cannot access file: %v", err))
		}
		return
	}

	// Check if it's a directory
	if info.IsDir() {
		result.AddWarning("delete", "path is a directory",
			"Deleting directories requires special handling",
			"Use recursive delete if you want to delete the directory and its contents")
		return
	}

	// Check if file is writable (needed for deletion)
	dir := filepath.Dir(path)
	dirInfo, err := os.Stat(dir)
	if err != nil {
		result.AddError("delete", fmt.Sprintf("cannot access parent directory: %v", err))
		return
	}

	if !dirInfo.IsDir() {
		result.AddError("delete", fmt.Sprintf("parent path is not a directory: %s", dir))
		return
	}

	// Try to check write permissions on directory
	tempFile := filepath.Join(dir, ".opencenter-delete-test")
	file, err := os.Create(tempFile)
	if err != nil {
		if os.IsPermission(err) {
			result.AddError("delete", fmt.Sprintf("directory is not writable: %s", dir),
				"Ensure you have write permissions for the directory to delete files")
		} else {
			result.AddError("delete", fmt.Sprintf("cannot write to directory: %v", err))
		}
		return
	}
	file.Close()
	os.Remove(tempFile)

	// Warn about important files
	base := filepath.Base(path)
	importantFiles := []string{".git", ".gitignore", "go.mod", "go.sum", "package.json", "Dockerfile"}
	for _, important := range importantFiles {
		if base == important {
			result.AddWarning("delete",
				fmt.Sprintf("attempting to delete important file: %s", base),
				"Ensure you really want to delete this file")
			break
		}
	}
}

// SetAllowedExtensions sets the allowed file extensions.
func (v *FileValidator) SetAllowedExtensions(extensions []string) {
	v.allowedExtensions = make(map[string]bool)
	for _, ext := range extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		v.allowedExtensions[ext] = true
	}
}

// SetMaxPathLength sets the maximum allowed path length.
func (v *FileValidator) SetMaxPathLength(length int) {
	v.maxPathLength = length
}
