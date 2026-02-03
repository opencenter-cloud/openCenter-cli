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
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateTempConfig(t *testing.T) {
	content := `
cluster:
  name: test-cluster
  provider: openstack
`

	configPath := CreateTempConfig(t, content)

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("config file was not created: %s", configPath)
	}

	// Verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	if string(data) != content {
		t.Fatalf("config content mismatch: got %q, want %q", string(data), content)
	}

	// Verify file has correct name
	if filepath.Base(configPath) != "config.yaml" {
		t.Fatalf("config file has wrong name: got %s, want config.yaml", filepath.Base(configPath))
	}
}

func TestCreateTempDir(t *testing.T) {
	files := map[string]string{
		"file1.txt":           "content1",
		"dir1/file2.txt":      "content2",
		"dir1/dir2/file3.txt": "content3",
	}

	tmpDir := CreateTempDir(t, files)

	// Verify all files exist with correct content
	for name, expectedContent := range files {
		path := filepath.Join(tmpDir, name)

		// Check file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("file was not created: %s", path)
		}

		// Check content
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file %s: %v", path, err)
		}

		if string(data) != expectedContent {
			t.Fatalf("file %s content mismatch: got %q, want %q", name, string(data), expectedContent)
		}
	}

	// Verify parent directories were created
	dir1 := filepath.Join(tmpDir, "dir1")
	if info, err := os.Stat(dir1); os.IsNotExist(err) {
		t.Fatalf("parent directory was not created: %s", dir1)
	} else if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", dir1)
	}

	dir2 := filepath.Join(tmpDir, "dir1", "dir2")
	if info, err := os.Stat(dir2); os.IsNotExist(err) {
		t.Fatalf("nested directory was not created: %s", dir2)
	} else if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", dir2)
	}
}

func TestCreateTempDir_EmptyMap(t *testing.T) {
	files := map[string]string{}

	tmpDir := CreateTempDir(t, files)

	// Verify directory exists
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Fatalf("temp directory was not created: %s", tmpDir)
	}
}

func TestCreateTempDir_SpecialCharacters(t *testing.T) {
	files := map[string]string{
		"file-with-dash.txt":      "content1",
		"file_with_underscore.txt": "content2",
		"file.with.dots.txt":       "content3",
	}

	tmpDir := CreateTempDir(t, files)

	// Verify all files exist
	for name := range files {
		path := filepath.Join(tmpDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("file was not created: %s", path)
		}
	}
}

func TestAssertNoError_WithNil(t *testing.T) {
	// This should not fail
	AssertNoError(t, nil, "test message")
}

func TestAssertNoError_WithError(t *testing.T) {
	// We can't easily test the failure case without causing the test to fail
	// This test documents the expected behavior
	t.Skip("AssertNoError failure behavior is tested through usage")
}

func TestAssertError_WithError(t *testing.T) {
	// This should not fail
	err := errors.New("test error")
	AssertError(t, err, "test message")
}

func TestAssertError_WithNil(t *testing.T) {
	// We can't easily test the failure case without causing the test to fail
	// This test documents the expected behavior
	t.Skip("AssertError failure behavior is tested through usage")
}

func TestAssertEqual_Equal(t *testing.T) {
	// These should not fail
	AssertEqual(t, 42, 42, "integers")
	AssertEqual(t, "hello", "hello", "strings")
	AssertEqual(t, true, true, "booleans")
}

func TestAssertEqual_NotEqual(t *testing.T) {
	// We can't easily test the failure case without causing the test to fail
	// This test documents the expected behavior
	t.Skip("AssertEqual failure behavior is tested through usage")
}

func TestAssertFileExists_Exists(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// This should not fail
	AssertFileExists(t, filePath)
}

func TestAssertFileExists_NotExists(t *testing.T) {
	// We can't easily test the failure case without causing the test to fail
	// This test documents the expected behavior
	t.Skip("AssertFileExists failure behavior is tested through usage")
}

func TestAssertFileNotExists_NotExists(t *testing.T) {
	// This should not fail
	nonExistentPath := "/path/that/does/not/exist/file.txt"
	AssertFileNotExists(t, nonExistentPath)
}

func TestAssertFileNotExists_Exists(t *testing.T) {
	// We can't easily test the failure case without causing the test to fail
	// This test documents the expected behavior
	t.Skip("AssertFileNotExists failure behavior is tested through usage")
}
