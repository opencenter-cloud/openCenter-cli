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
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencenter-cloud/opencenter-cli/internal/resilience"
)

// DefaultAtomicFileWriter writes files atomically with the shared file-operation retry policy.
type DefaultAtomicFileWriter struct {
	retryHandler resilience.RetryHandler
}

// NewDefaultAtomicFileWriter creates a new default atomic file writer
func NewDefaultAtomicFileWriter() *DefaultAtomicFileWriter {
	return &DefaultAtomicFileWriter{
		retryHandler: resilience.NewRetryHandler(resilience.FileOperationConfig),
	}
}

// WriteAtomic writes data to a file atomically
func (w *DefaultAtomicFileWriter) WriteAtomic(filename string, data []byte, perm os.FileMode) error {
	ctx := context.Background()
	return w.retryHandler.Do(ctx, func() error {
		// Create temporary file in the same directory as the target file
		dir := filepath.Dir(filename)
		tempFile, err := w.CreateTempFile(dir, ".tmp-"+filepath.Base(filename)+"-")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}

		tempPath := tempFile.Name()

		// Ensure cleanup on failure
		defer func() {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}()

		// Write data to temporary file
		if _, err := tempFile.Write(data); err != nil {
			return fmt.Errorf("failed to write to temporary file: %w", err)
		}

		// Set proper permissions
		if err := tempFile.Chmod(perm); err != nil {
			return fmt.Errorf("failed to set file permissions: %w", err)
		}

		// Sync to ensure data is written to disk
		if err := tempFile.Sync(); err != nil {
			return fmt.Errorf("failed to sync temporary file: %w", err)
		}

		// Close the temporary file
		if err := tempFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary file: %w", err)
		}

		// Atomically move temporary file to final location
		if err := os.Rename(tempPath, filename); err != nil {
			return fmt.Errorf("failed to move temporary file to final location: %w", err)
		}

		// Don't remove tempPath in defer since rename succeeded
		tempPath = ""
		return nil
	})
}

// CreateTempFile creates a temporary file in the specified directory
func (w *DefaultAtomicFileWriter) CreateTempFile(dir, pattern string) (*os.File, error) {
	if dir == "" {
		dir = os.TempDir()
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tempFile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	return tempFile, nil
}

// WriteFileAtomic is a convenience function for atomic file writing
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	writer := NewDefaultAtomicFileWriter()
	return writer.WriteAtomic(filename, data, perm)
}
