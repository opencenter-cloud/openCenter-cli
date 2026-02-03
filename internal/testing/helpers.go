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

package testing

import (
	"os"
	"path/filepath"
	"testing"
)

// CreateTempConfig creates a temporary config file for testing.
// It creates a temporary directory, writes the config content to a file,
// and returns the path to the config file. The directory is automatically
// cleaned up when the test completes.
func CreateTempConfig(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	return configPath
}

// CreateTempDir creates a temporary directory with specified files.
// The files parameter is a map where keys are file paths (relative to the temp dir)
// and values are the file contents. Parent directories are created automatically.
// The directory is automatically cleaned up when the test completes.
func CreateTempDir(t *testing.T, files map[string]string) string {
	t.Helper()

	tmpDir := t.TempDir()

	for name, content := range files {
		path := filepath.Join(tmpDir, name)

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create parent dir for %s: %v", name, err)
		}

		// Write file
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file %s: %v", name, err)
		}
	}

	return tmpDir
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error, message string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", message)
	}
}

// AssertEqual fails the test if got != want.
func AssertEqual(t *testing.T, got, want interface{}, message string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v, want %v", message, got, want)
	}
}

// AssertFileExists fails the test if file doesn't exist.
func AssertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file to exist: %s", path)
	}
}

// AssertFileNotExists fails the test if file exists.
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected file to not exist: %s", path)
	}
}
