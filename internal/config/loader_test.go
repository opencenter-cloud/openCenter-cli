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

package config

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	utilfs "github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// TestConfigIOHandler_NewConfigIOHandler tests loader creation
func TestConfigIOHandler_NewConfigIOHandler(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)

	if loader == nil {
		t.Fatal("expected non-nil loader")
	}
	if loader.fileSystem == nil {
		t.Fatal("expected non-nil fileSystem")
	}
}

// TestConfigIOHandler_MarshalConfig tests marshaling a config to YAML
func TestConfigIOHandler_MarshalConfig(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &Config{
				SchemaVersion: "2.0",
				OpenCenter: SimplifiedOpenCenter{
					Cluster: ClusterConfig{
						ClusterName: "test-cluster",
					},
				},
			},
			expectError: false,
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "configuration cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := loader.MarshalConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(data) == 0 {
				t.Error("expected non-empty YAML data")
			}

			// Verify it's valid YAML by checking for expected content
			yamlStr := string(data)
			if !strings.Contains(yamlStr, "test-cluster") {
				t.Error("expected YAML to contain cluster name")
			}
		})
	}
}

// TestConfigIOHandler_UnmarshalConfig tests unmarshaling YAML to a config
func TestConfigIOHandler_UnmarshalConfig(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)

	tests := []struct {
		name        string
		data        []byte
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid YAML",
			data: []byte(`
schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: test-cluster
`),
			expectError: false,
		},
		{
			name:        "empty data",
			data:        []byte{},
			expectError: true,
			errorMsg:    "configuration data cannot be empty",
		},
		{
			name: "invalid YAML",
			data: []byte(`
invalid: yaml: content:
  - broken
    indentation
`),
			expectError: true,
			errorMsg:    "failed to unmarshal YAML configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := loader.UnmarshalConfig(tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Error("expected non-nil config")
			}
		})
	}
}

// TestConfigIOHandler_LoadFromBytes tests loading config from byte data
func TestConfigIOHandler_LoadFromBytes(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)
	ctx := context.Background()

	tests := []struct {
		name        string
		data        []byte
		expectError bool
		checkFunc   func(*testing.T, *Config)
	}{
		{
			name: "valid config with environment variable",
			data: []byte(`
schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: test-cluster
`),
			expectError: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.OpenCenter.Cluster.ClusterName != "test-cluster" {
					t.Errorf("expected cluster name 'test-cluster', got %q", cfg.OpenCenter.Cluster.ClusterName)
				}
			},
		},
		{
			name: "config with nested structures",
			data: []byte(`
schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: nested-test
  gitops:
    git_dir: /path/to/gitops
`),
			expectError: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.OpenCenter.GitOps.GitDir != "/path/to/gitops" {
					t.Errorf("expected git_dir '/path/to/gitops', got %q", cfg.OpenCenter.GitOps.GitDir)
				}
			},
		},
		{
			name: "config does not expand arbitrary environment variables",
			data: []byte(`
schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: test-cluster
  gitops:
    git_dir: ${AWS_SECRET_ACCESS_KEY}
`),
			expectError: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.OpenCenter.GitOps.GitDir != "${AWS_SECRET_ACCESS_KEY}" {
					t.Errorf("expected literal git_dir, got %q", cfg.OpenCenter.GitOps.GitDir)
				}
			},
		},
		{
			name:        "invalid YAML",
			data:        []byte(`invalid: yaml: [broken`),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := loader.LoadFromBytes(ctx, tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Fatal("expected non-nil config")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, config)
			}
		})
	}
}

// TestConfigIOHandler_SaveToFile tests saving config to a file
func TestConfigIOHandler_SaveToFile(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)
	ctx := context.Background()

	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		config      *Config
		filename    string
		expectError bool
	}{
		{
			name: "save valid config",
			config: &Config{
				SchemaVersion: "2.0",
				OpenCenter: SimplifiedOpenCenter{
					Cluster: ClusterConfig{
						ClusterName: "save-test",
					},
				},
			},
			filename:    "test-config.yaml",
			expectError: false,
		},
		{
			name:        "save nil config",
			config:      nil,
			filename:    "nil-config.yaml",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.filename)
			err := loader.SaveToFile(ctx, path, tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify file was created
			if _, statErr := os.Stat(path); statErr != nil {
				t.Errorf("expected file to exist at %s: %v", path, statErr)
				return
			}

			// Verify file permissions (should be 0600)
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("failed to stat file: %v", err)
				return
			}

			perm := info.Mode().Perm()
			if perm != 0o600 {
				t.Errorf("expected file permissions 0600, got %o", perm)
			}

			// Verify file content
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("failed to read saved file: %v", err)
				return
			}

			if len(data) == 0 {
				t.Error("expected non-empty file content")
			}

			// Verify content contains expected data
			content := string(data)
			if !strings.Contains(content, "save-test") {
				t.Error("expected file content to contain cluster name")
			}
		})
	}
}

// TestConfigIOHandler_LoadFromFile tests loading config from a file
func TestConfigIOHandler_LoadFromFile(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)
	ctx := context.Background()

	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create a valid test config file
	validConfigPath := filepath.Join(tmpDir, "valid-config.yaml")
	validConfigData := []byte(`
schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: load-test
  gitops:
    git_dir: /test/path
`)
	if err := os.WriteFile(validConfigPath, validConfigData, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create an invalid YAML file
	invalidConfigPath := filepath.Join(tmpDir, "invalid-config.yaml")
	invalidConfigData := []byte(`invalid: yaml: [broken`)
	if err := os.WriteFile(invalidConfigPath, invalidConfigData, 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		expectError bool
		checkFunc   func(*testing.T, *Config)
	}{
		{
			name:        "load valid config",
			path:        validConfigPath,
			expectError: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.OpenCenter.Cluster.ClusterName != "load-test" {
					t.Errorf("expected cluster name 'load-test', got %q", cfg.OpenCenter.Cluster.ClusterName)
				}
				if cfg.OpenCenter.GitOps.GitDir != "/test/path" {
					t.Errorf("expected git_dir '/test/path', got %q", cfg.OpenCenter.GitOps.GitDir)
				}
			},
		},
		{
			name:        "load non-existent file",
			path:        filepath.Join(tmpDir, "non-existent.yaml"),
			expectError: true,
		},
		{
			name:        "load invalid YAML",
			path:        invalidConfigPath,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := loader.LoadFromFile(ctx, tt.path)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Fatal("expected non-nil config")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, config)
			}
		})
	}
}

// TestConfigIOHandler_RoundTrip tests marshaling and unmarshaling preserves data
func TestConfigIOHandler_RoundTrip(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)

	originalConfig := &Config{
		SchemaVersion: "2.0",
		OpenCenter: SimplifiedOpenCenter{
			Cluster: ClusterConfig{
				ClusterName: "roundtrip-test",
			},
			GitOps: GitOpsConfig{
				GitDir: "/test/gitops",
			},
		},
	}

	// Marshal to YAML
	data, err := loader.MarshalConfig(originalConfig)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	// Unmarshal back to config
	restoredConfig, err := loader.UnmarshalConfig(data)
	if err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	// Verify key fields are preserved
	if restoredConfig.SchemaVersion != originalConfig.SchemaVersion {
		t.Errorf("schema version mismatch: expected %q, got %q",
			originalConfig.SchemaVersion, restoredConfig.SchemaVersion)
	}

	if restoredConfig.OpenCenter.Cluster.ClusterName != originalConfig.OpenCenter.Cluster.ClusterName {
		t.Errorf("cluster name mismatch: expected %q, got %q",
			originalConfig.OpenCenter.Cluster.ClusterName, restoredConfig.OpenCenter.Cluster.ClusterName)
	}

	if restoredConfig.OpenCenter.GitOps.GitDir != originalConfig.OpenCenter.GitOps.GitDir {
		t.Errorf("git dir mismatch: expected %q, got %q",
			originalConfig.OpenCenter.GitOps.GitDir, restoredConfig.OpenCenter.GitOps.GitDir)
	}
}

// TestConfigIOHandler_SaveAndLoad tests the complete save and load cycle
func TestConfigIOHandler_SaveAndLoad(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)
	ctx := context.Background()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cycle-test.yaml")

	originalConfig := &Config{
		SchemaVersion: "2.0",
		OpenCenter: SimplifiedOpenCenter{
			Cluster: ClusterConfig{
				ClusterName: "cycle-test",
			},
			GitOps: GitOpsConfig{
				GitDir: "/cycle/test/path",
			},
		},
	}

	// Save config
	if err := loader.SaveToFile(ctx, configPath, originalConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Load config
	loadedConfig, err := loader.LoadFromFile(ctx, configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify loaded config matches original
	if loadedConfig.OpenCenter.Cluster.ClusterName != originalConfig.OpenCenter.Cluster.ClusterName {
		t.Errorf("cluster name mismatch after save/load cycle: expected %q, got %q",
			originalConfig.OpenCenter.Cluster.ClusterName, loadedConfig.OpenCenter.Cluster.ClusterName)
	}

	if loadedConfig.OpenCenter.GitOps.GitDir != originalConfig.OpenCenter.GitOps.GitDir {
		t.Errorf("git dir mismatch after save/load cycle: expected %q, got %q",
			originalConfig.OpenCenter.GitOps.GitDir, loadedConfig.OpenCenter.GitOps.GitDir)
	}
}

// TestConfigIOHandler_EnvironmentVariablesRemainLiteral verifies that raw config
// loading does not expand arbitrary environment variables.
func TestConfigIOHandler_EnvironmentVariablesRemainLiteral(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)
	ctx := context.Background()

	testValue := "expanded-value"
	t.Setenv("TEST_CONFIG_VAR", testValue)

	configData := []byte(`
schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: ${TEST_CONFIG_VAR}
`)

	config, err := loader.LoadFromBytes(ctx, configData)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if config.OpenCenter.Cluster.ClusterName != "${TEST_CONFIG_VAR}" {
		t.Errorf("expected literal cluster name, got %q",
			config.OpenCenter.Cluster.ClusterName)
	}
}

// TestConfigIOHandler_AtomicWrite tests that SaveToFile uses atomic writes
func TestConfigIOHandler_AtomicWrite(t *testing.T) {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := utilfs.NewDefaultFileSystem(errorHandler)
	loader := NewConfigIOHandler(fileSystem)
	ctx := context.Background()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "atomic-test.yaml")

	// Create initial config
	initialConfig := &Config{
		SchemaVersion: "2.0",
		OpenCenter: SimplifiedOpenCenter{
			Cluster: ClusterConfig{
				ClusterName: "initial",
			},
		},
	}

	// Save initial config
	if err := loader.SaveToFile(ctx, configPath, initialConfig); err != nil {
		t.Fatalf("failed to save initial config: %v", err)
	}

	// Verify initial config exists
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read initial config: %v", err)
	}

	if !strings.Contains(string(data), "initial") {
		t.Error("expected initial config to contain 'initial'")
	}

	// Save updated config (should overwrite atomically)
	updatedConfig := &Config{
		SchemaVersion: "2.0",
		OpenCenter: SimplifiedOpenCenter{
			Cluster: ClusterConfig{
				ClusterName: "updated",
			},
		},
	}

	if err := loader.SaveToFile(ctx, configPath, updatedConfig); err != nil {
		t.Fatalf("failed to save updated config: %v", err)
	}

	// Verify updated config
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read updated config: %v", err)
	}

	if !strings.Contains(string(data), "updated") {
		t.Error("expected updated config to contain 'updated'")
	}

	if strings.Contains(string(data), "initial") {
		t.Error("expected updated config to not contain 'initial'")
	}
}

// mockFileSystem is a mock implementation for testing error conditions
type mockFileSystem struct {
	readFileFunc        func(path string) ([]byte, error)
	writeFileAtomicFunc func(path string, data []byte, perm os.FileMode) error
}

func (m *mockFileSystem) ReadFile(path string) ([]byte, error) {
	if m.readFileFunc != nil {
		return m.readFileFunc(path)
	}
	return nil, fmt.Errorf("mock read error")
}

func (m *mockFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return fmt.Errorf("mock write error")
}

func (m *mockFileSystem) WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if m.writeFileAtomicFunc != nil {
		return m.writeFileAtomicFunc(path, data, perm)
	}
	return fmt.Errorf("mock atomic write error")
}

func (m *mockFileSystem) Exists(path string) bool {
	return false
}

func (m *mockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return fmt.Errorf("mock mkdir error")
}

func (m *mockFileSystem) Remove(path string) error {
	return fmt.Errorf("mock remove error")
}

func (m *mockFileSystem) Stat(path string) (fs.FileInfo, error) {
	return nil, fmt.Errorf("mock stat error")
}

// TestConfigIOHandler_LoadFromFileError tests error handling in LoadFromFile
func TestConfigIOHandler_LoadFromFileError(t *testing.T) {
	mockFS := &mockFileSystem{
		readFileFunc: func(path string) ([]byte, error) {
			return nil, fmt.Errorf("simulated read error")
		},
	}
	loader := NewConfigIOHandler(mockFS)
	ctx := context.Background()

	_, err := loader.LoadFromFile(ctx, "/test/path")
	if err == nil {
		t.Error("expected error from LoadFromFile with failing FileSystem")
	}

	if !strings.Contains(err.Error(), "file operation failed: read") {
		t.Errorf("expected error message about reading file, got: %v", err)
	}
}

// TestConfigIOHandler_SaveToFileError tests error handling in SaveToFile
func TestConfigIOHandler_SaveToFileError(t *testing.T) {
	mockFS := &mockFileSystem{
		writeFileAtomicFunc: func(path string, data []byte, perm os.FileMode) error {
			return fmt.Errorf("simulated write error")
		},
	}
	loader := NewConfigIOHandler(mockFS)
	ctx := context.Background()

	config := &Config{
		SchemaVersion: "2.0",
		OpenCenter: SimplifiedOpenCenter{
			Cluster: ClusterConfig{
				ClusterName: "test",
			},
		},
	}

	err := loader.SaveToFile(ctx, "/test/path", config)
	if err == nil {
		t.Error("expected error from SaveToFile with failing FileSystem")
	}

	if !strings.Contains(err.Error(), "file operation failed: write") {
		t.Errorf("expected error message about writing file, got: %v", err)
	}
}
