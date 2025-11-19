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
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

func TestConfig(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	t.Run("Save and Load", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.OpenCenter.GitOps.GitDir = ""
		if err := Save(cfg); err != nil {
			t.Fatal(err)
		}

		loaded, err := Load("test")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cfg, loaded) {
			t.Errorf("loaded config does not match saved config")
		}
	})

	t.Run("List", func(t *testing.T) {
		names, err := List()
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 1 || names[0] != "test" {
			t.Errorf("unexpected list result: %v", names)
		}
	})

	t.Run("SetActive and GetActive", func(t *testing.T) {
		if err := SetActive("test"); err != nil {
			t.Fatal(err)
		}
		active, err := GetActive()
		if err != nil {
			t.Fatal(err)
		}
		if active != "test" {
			t.Errorf("unexpected active cluster: %s", active)
		}
	})

	t.Run("Validate", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.OpenCenter.GitOps.GitDir = ""
		// Missing git_dir should produce a validation error
		errs := Validate(cfg)
		if len(errs) == 0 {
			t.Error("expected validation error for missing opencenter.gitops.git_dir")
		}
		// Provide minimal required fields
		cfg.OpenCenter.GitOps.GitDir = "testdata/gitops"
		errs = Validate(cfg)
		if len(errs) != 0 {
			t.Errorf("unexpected validation errors: %v", errs)
		}
	})

	// New: OpenTofu S3 backend requires opencenter AWS credentials
	t.Run("Validate OpenTofu S3 requires credentials", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.OpenCenter.GitOps.GitDir = "testdata/gitops"
		cfg.OpenTofu.Enabled = true
		cfg.OpenTofu.Backend.Type = "s3"
		cfg.OpenTofu.Backend.S3.Bucket = "my-bucket"
		cfg.OpenTofu.Backend.S3.Key = "state.tfstate"
		cfg.OpenTofu.Backend.S3.Region = "us-east-1"

		errs := Validate(cfg)
		if len(errs) == 0 {
			t.Fatal("expected validation error for missing opencenter AWS credentials with s3 backend")
		}
		// Provide credentials, expect no error from this rule (other rules already satisfied)
		cfg.OpenCenter.Cluster.AWSAccessKey = "AKIA..."
		cfg.OpenCenter.Cluster.AWSSecretAccessKey = "secret"
		errs = Validate(cfg)
		if len(errs) != 0 {
			t.Errorf("unexpected validation errors with credentials set: %v", errs)
		}
	})
}

func TestResolveConfigDir(t *testing.T) {
	// Unset env var to test default behavior
	os.Unsetenv("OPENCENTER_CONFIG_DIR")

	dir, err := ResolveConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "openCenter")
	if dir != expected {
		t.Errorf("expected config dir %s, but got %s", expected, dir)
	}

	// Set env var to test override (use repo testdata)
	testDir := "testdata/openCenter-test"
	os.Setenv("OPENCENTER_CONFIG_DIR", testDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	dir, err = ResolveConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	// ResolveConfigDir returns an absolute path; compare absolute forms.
	absExpected, _ := filepath.Abs(testDir)
	if dir != absExpected {
		t.Errorf("expected config dir %s, but got %s", absExpected, dir)
	}
}
func TestConfigPath(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create a flat config file to test path resolution
	flatConfigPath := filepath.Join(dir, "test-cluster.yaml")
	if err := os.WriteFile(flatConfigPath, []byte("opencenter:\n  cluster:\n    cluster_name: test-cluster\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// ConfigPath should find the flat config file
	path, err := ConfigPath("test-cluster")
	if err != nil {
		t.Fatal(err)
	}

	if path != flatConfigPath {
		t.Errorf("expected path %s, got %s", flatConfigPath, path)
	}

	// Test that ConfigPath returns error for non-existent cluster
	_, err = ConfigPath("non-existent-cluster")
	if err == nil {
		t.Error("expected error for non-existent cluster, got nil")
	}
}

func TestConfigHelperMethods(t *testing.T) {
	cfg := NewDefault("test-cluster")

	// Test ClusterName method
	if cfg.ClusterName() != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %s", cfg.ClusterName())
	}

	// Test GitOps method
	gitops := cfg.GitOps()
	if gitops.GitBranch != "main" {
		t.Errorf("expected git branch 'main', got %s", gitops.GitBranch)
	}
}

func TestConfigToJSON(t *testing.T) {
	cfg := NewDefault("test-cluster")
	cfg.OpenCenter.GitOps.GitDir = "test-dir"

	jsonData, err := cfg.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's valid JSON by unmarshaling
	var result map[string]any
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("generated JSON is invalid: %v", err)
	}

	// Check that some expected fields are present
	if result["opencenter"] == nil {
		t.Error("expected 'opencenter' field in JSON output")
	}
	if result["opentofu"] == nil {
		t.Error("expected 'opentofu' field in JSON output")
	}
	if result["secrets"] == nil {
		t.Error("expected 'secrets' field in JSON output")
	}
}

func TestSaveWithEmptyClusterName(t *testing.T) {
	cfg := NewDefault("")
	cfg.OpenCenter.Cluster.ClusterName = ""

	err := Save(cfg)
	if err == nil {
		t.Error("expected error when saving config with empty cluster name")
	}
	if err.Error() != "cluster_name must not be empty" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	_, err := Load("nonexistent")
	if err == nil {
		t.Error("expected error when loading non-existent config")
	}
}

func TestValidateExtended(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() Config
		expectErrs  []string
	}{
		{
			name: "missing cluster name",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.ClusterName = ""
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				return cfg
			},
			expectErrs: []string{"opencenter.cluster.cluster_name must be set"},
		},
		{
			name: "missing git dir",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = ""
				return cfg
			},
			expectErrs: []string{"opencenter.gitops.git_dir must be set"},
		},
		{
			name: "opentofu enabled without path",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenTofu.Enabled = true
				cfg.OpenTofu.Path = ""
				return cfg
			},
			expectErrs: []string{"opentofu.path must be set when opentofu.enabled=true"},
		},
		{
			name: "s3 backend without credentials",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenTofu.Enabled = true
				cfg.OpenTofu.Backend.Type = "s3"
				cfg.OpenTofu.Backend.S3.Bucket = "test-bucket"
				cfg.OpenTofu.Backend.S3.Key = "test.tfstate"
				cfg.OpenTofu.Backend.S3.Region = "us-east-1"
				cfg.OpenCenter.Cluster.AWSAccessKey = ""
				cfg.OpenCenter.Cluster.AWSSecretAccessKey = ""
				return cfg
			},
			expectErrs: []string{"opencenter.cluster.aws_access_key and opencenter.cluster.aws_secret_access_key must be set when opentofu.backend.type=s3"},
		},
		{
			name: "s3 backend missing bucket info",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenTofu.Enabled = true
				cfg.OpenTofu.Backend.Type = "s3"
				cfg.OpenTofu.Backend.S3.Bucket = ""
				// Provide AWS credentials to avoid that validation error
				cfg.OpenCenter.Cluster.AWSAccessKey = "AKIA..."
				cfg.OpenCenter.Cluster.AWSSecretAccessKey = "secret"
				return cfg
			},
			expectErrs: []string{"opentofu.backend.s3 requires bucket, key, and region"},
		},
		{
			name: "invalid backend type",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenTofu.Enabled = true
				cfg.OpenTofu.Backend.Type = "invalid"
				return cfg
			},
			expectErrs: []string{"opentofu.backend.type must be 'local' or 's3'"},
		},
		{
			name: "local backend without path",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenTofu.Enabled = true
				cfg.OpenTofu.Backend.Type = "local"
				cfg.OpenTofu.Backend.Local.Path = ""
				return cfg
			},
			expectErrs: []string{"opentofu.backend.local.path must be set for local backend"},
		},
		{
			name: "valid config",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				return cfg
			},
			expectErrs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			errs := Validate(cfg)

			if len(errs) != len(tt.expectErrs) {
				t.Errorf("expected %d errors, got %d: %v", len(tt.expectErrs), len(errs), errs)
				return
			}

			for i, expectedErr := range tt.expectErrs {
				if i >= len(errs) || errs[i] != expectedErr {
					t.Errorf("expected error %q, got %q", expectedErr, errs[i])
				}
			}
		})
	}
}

func TestListEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	names, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

func TestListMultipleConfigs(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create multiple config files
	configs := []string{"cluster-a", "cluster-b", "cluster-c"}
	for _, name := range configs {
		cfg := NewDefault(name)
		cfg.OpenCenter.GitOps.GitDir = "test-dir"
		if err := Save(cfg); err != nil {
			t.Fatal(err)
		}
	}

	// Create a non-yaml file that should be ignored
	nonYamlFile := filepath.Join(dir, "not-a-config.txt")
	if err := os.WriteFile(nonYamlFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	names, err := List()
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != 3 {
		t.Errorf("expected 3 configs, got %d: %v", len(names), names)
	}

	// Verify sorting
	expected := []string{"cluster-a", "cluster-b", "cluster-c"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("expected %s at position %d, got %s", expected[i], i, name)
		}
	}
}

func TestActiveClusterOperations(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Test getting active when no active cluster is set
	active, err := GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if active != "" {
		t.Errorf("expected empty active cluster, got %s", active)
	}

	// Test setting active cluster
	if err := SetActive("test-cluster"); err != nil {
		t.Fatal(err)
	}

	// Test getting active cluster
	active, err = GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if active != "test-cluster" {
		t.Errorf("expected active cluster 'test-cluster', got %s", active)
	}

	// Test clearing active cluster
	if err := SetActive(""); err != nil {
		t.Fatal(err)
	}

	active, err = GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if active != "" {
		t.Errorf("expected empty active cluster after clearing, got %s", active)
	}
}

func TestSaveDebugConfig(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create a test config
	cfg := NewDefault("test-cluster")
	cfg.OpenCenter.GitOps.GitDir = "test-dir"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Test saving debug config
	gitDir := filepath.Join(dir, "git-repo")
	err := SaveDebugConfig("test-cluster", gitDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify debug config file was created
	debugPath := filepath.Join(gitDir, ".openCenter.yaml")
	if _, err := os.Stat(debugPath); os.IsNotExist(err) {
		t.Error("debug config file was not created")
	}

	// Verify content is valid YAML
	content, err := os.ReadFile(debugPath)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := yaml.Unmarshal(content, &result); err != nil {
		t.Fatalf("debug config is not valid YAML: %v", err)
	}
}

func TestSaveDebugConfigEmptyGitDir(t *testing.T) {
	err := SaveDebugConfig("test", "")
	if err == nil {
		t.Error("expected error when git directory is empty")
	}
	if err.Error() != "git directory is empty" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestGenerateCompleteConfig(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create a test config
	cfg := NewDefault("test-cluster")
	cfg.OpenCenter.GitOps.GitDir = "test-dir"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Test generating complete config
	complete, err := GenerateCompleteConfig("test-cluster")
	if err != nil {
		t.Fatal(err)
	}

	if complete.ClusterName() != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %s", complete.ClusterName())
	}
}

func TestGenerateCompleteConfigYAML(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create a test config
	cfg := NewDefault("test-cluster")
	cfg.OpenCenter.GitOps.GitDir = "test-dir"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Test generating complete config YAML
	yamlData, err := GenerateCompleteConfigYAML("test-cluster")
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's valid YAML
	var result map[string]any
	if err := yaml.Unmarshal(yamlData, &result); err != nil {
		t.Fatalf("generated YAML is invalid: %v", err)
	}

	// Check that expected fields are present
	if result["opencenter"] == nil {
		t.Error("expected 'opencenter' field in YAML output")
	}
}

func TestMergeYAMLMaps(t *testing.T) {
	base := map[string]any{
		"key1": "base_value1",
		"key2": map[string]any{
			"nested1": "base_nested1",
			"nested2": "base_nested2",
		},
		"key3": "base_value3",
	}

	override := map[string]any{
		"key1": "override_value1",
		"key2": map[string]any{
			"nested1": "override_nested1",
			"nested3": "override_nested3",
		},
		"key4": "override_value4",
	}

	result := mergeYAMLMaps(base, override)

	// Check that override values take precedence
	if result["key1"] != "override_value1" {
		t.Errorf("expected key1 to be overridden, got %v", result["key1"])
	}

	// Check that new keys are added
	if result["key4"] != "override_value4" {
		t.Errorf("expected key4 to be added, got %v", result["key4"])
	}

	// Check that base-only keys are preserved
	if result["key3"] != "base_value3" {
		t.Errorf("expected key3 to be preserved, got %v", result["key3"])
	}

	// Check nested map merging
	nested, ok := result["key2"].(map[string]any)
	if !ok {
		t.Fatal("expected key2 to be a map")
	}

	if nested["nested1"] != "override_nested1" {
		t.Errorf("expected nested1 to be overridden, got %v", nested["nested1"])
	}

	if nested["nested2"] != "base_nested2" {
		t.Errorf("expected nested2 to be preserved, got %v", nested["nested2"])
	}

	if nested["nested3"] != "override_nested3" {
		t.Errorf("expected nested3 to be added, got %v", nested["nested3"])
	}
}

func TestSortStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "already sorted",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "reverse order",
			input:    []string{"c", "b", "a"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "random order",
			input:    []string{"b", "a", "d", "c"},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "single element",
			input:    []string{"a"},
			expected: []string{"a"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the test data
			input := make([]string, len(tt.input))
			copy(input, tt.input)

			sortStrings(input)

			if len(input) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(input))
				return
			}

			for i, expected := range tt.expected {
				if input[i] != expected {
					t.Errorf("at position %d: expected %s, got %s", i, expected, input[i])
				}
			}
		})
	}
}

func TestValidateClusterName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid simple name",
			clusterName: "test-cluster",
			expectError: false,
		},
		{
			name:        "valid name with underscores",
			clusterName: "test_cluster_01",
			expectError: false,
		},
		{
			name:        "valid name with dots",
			clusterName: "test.cluster.dev",
			expectError: false,
		},
		{
			name:        "valid alphanumeric",
			clusterName: "cluster123",
			expectError: false,
		},
		{
			name:        "empty name",
			clusterName: "",
			expectError: true,
			errorMsg:    "cluster name cannot be empty for directory creation",
		},
		{
			name:        "name with forward slash",
			clusterName: "test/cluster",
			expectError: true,
			errorMsg:    "cluster name cannot contain path separators (/ or \\) for directory structure",
		},
		{
			name:        "name with backslash",
			clusterName: "test\\cluster",
			expectError: true,
			errorMsg:    "cluster name cannot contain path separators (/ or \\) for directory structure",
		},
		{
			name:        "relative path current dir",
			clusterName: ".",
			expectError: true,
			errorMsg:    "cluster name cannot be a relative path component for security reasons",
		},
		{
			name:        "relative path parent dir",
			clusterName: "..",
			expectError: true,
			errorMsg:    "cluster name cannot be a relative path component for security reasons",
		},
		{
			name:        "name starting with special char",
			clusterName: "-cluster",
			expectError: true,
			errorMsg:    "cluster name must start with alphanumeric character and contain only alphanumeric characters, dots, hyphens, and underscores for directory naming",
		},
		{
			name:        "name with invalid characters",
			clusterName: "test@cluster",
			expectError: true,
			errorMsg:    "cluster name must start with alphanumeric character and contain only alphanumeric characters, dots, hyphens, and underscores for directory naming",
		},
		{
			name:        "name starting with dot",
			clusterName: ".hidden",
			expectError: true,
			errorMsg:    "cluster name must start with alphanumeric character and contain only alphanumeric characters, dots, hyphens, and underscores for directory naming",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClusterName(tt.clusterName)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for cluster name %q, but got none", tt.clusterName)
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("expected error message %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for cluster name %q, but got: %v", tt.clusterName, err)
				}
			}
		})
	}
}

func TestClusterDirectoryPath(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	tests := []struct {
		name        string
		clusterName string
		expectError bool
	}{
		{
			name:        "valid cluster name",
			clusterName: "test-cluster",
			expectError: false,
		},
		{
			name:        "invalid cluster name",
			clusterName: "test/cluster",
			expectError: true,
		},
		{
			name:        "empty cluster name",
			clusterName: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ClusterDirectoryPath(tt.clusterName)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for cluster name %q, but got none", tt.clusterName)
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error for cluster name %q, but got: %v", tt.clusterName, err)
				return
			}

			expected := filepath.Join(dir, "clusters", tt.clusterName)
			if path != expected {
				t.Errorf("expected path %s, got %s", expected, path)
			}
		})
	}
}

func TestClusterSecretsPath(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	tests := []struct {
		name        string
		clusterName string
		expectError bool
	}{
		{
			name:        "valid cluster name",
			clusterName: "test-cluster",
			expectError: false,
		},
		{
			name:        "invalid cluster name",
			clusterName: "test/cluster",
			expectError: true,
		},
		{
			name:        "empty cluster name",
			clusterName: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ClusterSecretsPath(tt.clusterName)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for cluster name %q, but got none", tt.clusterName)
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error for cluster name %q, but got: %v", tt.clusterName, err)
				return
			}

			expected := filepath.Join(dir, "clusters", tt.clusterName, "secrets", "age", "keys")
			if path != expected {
				t.Errorf("expected path %s, got %s", expected, path)
			}
		})
	}
}

func TestValidateServiceReleaseAndBranch(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() Config
		expectErrs  []string
	}{
		{
			name: "service with both release and branch",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{
					Enabled: true,
					Release: "v1.0.0",
					Branch:  "main",
				}
				return cfg
			},
			expectErrs: []string{"service 'cert-manager': only one of 'release' or 'branch' can be set, not both"},
		},
		{
			name: "service with only release",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{
					Enabled: true,
					Release: "v1.0.0",
				}
				return cfg
			},
			expectErrs: []string{},
		},
		{
			name: "service with only branch",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{
					Enabled: true,
					Branch:  "main",
				}
				return cfg
			},
			expectErrs: []string{},
		},
		{
			name: "service with neither release nor branch",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{
					Enabled: true,
				}
				return cfg
			},
			expectErrs: []string{},
		},
		{
			name: "managed-service with both release and branch",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.ManagedService["alert-proxy"] = ServiceCfg{
					Enabled: true,
					Release: "v1.0.0",
					Branch:  "main",
				}
				return cfg
			},
			expectErrs: []string{"managed-service 'alert-proxy': only one of 'release' or 'branch' can be set, not both"},
		},
		{
			name: "gitops with both release and branch",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.GitOps.Release = "v1.0.0"
				cfg.OpenCenter.GitOps.Branch = "main"
				return cfg
			},
			expectErrs: []string{"gitops: only one of 'release' or 'branch' can be set, not both"},
		},
		{
			name: "gitops with only release",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.GitOps.Release = "v1.0.0"
				return cfg
			},
			expectErrs: []string{},
		},
		{
			name: "gitops with only branch",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.GitOps.Branch = "develop"
				return cfg
			},
			expectErrs: []string{},
		},
		{
			name: "multiple services with conflicts",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{
					Enabled: true,
					Release: "v1.0.0",
					Branch:  "main",
				}
				cfg.OpenCenter.Services["fluxcd"] = ServiceCfg{
					Enabled: true,
					Release: "v2.0.0",
					Branch:  "develop",
				}
				cfg.OpenCenter.GitOps.Release = "v1.0.0"
				cfg.OpenCenter.GitOps.Branch = "main"
				return cfg
			},
			expectErrs: []string{
				"service 'cert-manager': only one of 'release' or 'branch' can be set, not both",
				"service 'fluxcd': only one of 'release' or 'branch' can be set, not both",
				"gitops: only one of 'release' or 'branch' can be set, not both",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			errs := Validate(cfg)

			if len(errs) != len(tt.expectErrs) {
				t.Errorf("expected %d errors, got %d: %v", len(tt.expectErrs), len(errs), errs)
				return
			}

			// Check that all expected errors are present (order may vary)
			for _, expectedErr := range tt.expectErrs {
				found := false
				for _, err := range errs {
					if err == expectedErr {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error %q not found in: %v", expectedErr, errs)
				}
			}
		})
	}
}

// TestDefaultConfigNewFields tests that NewDefault populates all new configuration fields correctly
func TestDefaultConfigNewFields(t *testing.T) {
	cfg := NewDefault("test-cluster")

	// Test ClusterConfig new fields
	t.Run("ClusterConfig fields", func(t *testing.T) {
		if cfg.OpenCenter.Cluster.BaseDomain != "k8s.opencenter.cloud" {
			t.Errorf("expected BaseDomain 'k8s.opencenter.cloud', got %s", cfg.OpenCenter.Cluster.BaseDomain)
		}

		expectedFQDN := "test-cluster.sjc3.k8s.opencenter.cloud"
		if cfg.OpenCenter.Cluster.ClusterFQDN != expectedFQDN {
			t.Errorf("expected ClusterFQDN '%s', got %s", expectedFQDN, cfg.OpenCenter.Cluster.ClusterFQDN)
		}

		if cfg.OpenCenter.Cluster.AdminEmail != "admin@example.com" {
			t.Errorf("expected AdminEmail 'admin@example.com', got %s", cfg.OpenCenter.Cluster.AdminEmail)
		}
	})

	// Test GitOpsConfig new fields
	t.Run("GitOpsConfig fields", func(t *testing.T) {
		if cfg.OpenCenter.GitOps.GitOpsBaseRepo != "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git" {
			t.Errorf("expected GitOpsBaseRepo 'ssh://git@github.com/rackerlabs/openCenter-gitops-base.git', got %s", cfg.OpenCenter.GitOps.GitOpsBaseRepo)
		}

		if cfg.OpenCenter.GitOps.GitOpsBaseRelease != "v0.1.0" {
			t.Errorf("expected GitOpsBaseRelease 'v0.1.0', got %s", cfg.OpenCenter.GitOps.GitOpsBaseRelease)
		}

		if cfg.OpenCenter.GitOps.GitOpsBranch != "main" {
			t.Errorf("expected GitOpsBranch 'main', got %s", cfg.OpenCenter.GitOps.GitOpsBranch)
		}
	})

	// Test StorageConfig
	t.Run("StorageConfig fields", func(t *testing.T) {
		if cfg.OpenCenter.Storage.DefaultStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected DefaultStorageClass 'csi-cinder-sc-delete', got %s", cfg.OpenCenter.Storage.DefaultStorageClass)
		}
	})

	// Test ServiceCfg new fields
	t.Run("ServiceCfg fields", func(t *testing.T) {
		// Test cert-manager
		certManager, ok := cfg.OpenCenter.Services["cert-manager"]
		if !ok {
			t.Fatal("cert-manager service not found")
		}
		if certManager.LetsEncryptServer != "https://acme-v02.api.letsencrypt.org/directory" {
			t.Errorf("expected LetsEncryptServer 'https://acme-v02.api.letsencrypt.org/directory', got %s", certManager.LetsEncryptServer)
		}
		if certManager.Region != "us-east-1" {
			t.Errorf("expected cert-manager Region 'us-east-1', got %s", certManager.Region)
		}

		// Test loki
		loki, ok := cfg.OpenCenter.Services["loki"]
		if !ok {
			t.Fatal("loki service not found")
		}
		if loki.LokiVolumeSize != 20 {
			t.Errorf("expected LokiVolumeSize 20, got %d", loki.LokiVolumeSize)
		}
		if loki.LokiStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected LokiStorageClass 'csi-cinder-sc-delete', got %s", loki.LokiStorageClass)
		}

		// Test kube-prometheus-stack
		promStack, ok := cfg.OpenCenter.Services["kube-prometheus-stack"]
		if !ok {
			t.Fatal("kube-prometheus-stack service not found")
		}
		if promStack.PrometheusVolumeSize != 50 {
			t.Errorf("expected PrometheusVolumeSize 50, got %d", promStack.PrometheusVolumeSize)
		}
		if promStack.PrometheusStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected PrometheusStorageClass 'csi-cinder-sc-delete', got %s", promStack.PrometheusStorageClass)
		}
		if promStack.GrafanaVolumeSize != 10 {
			t.Errorf("expected GrafanaVolumeSize 10, got %d", promStack.GrafanaVolumeSize)
		}
		if promStack.GrafanaStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected GrafanaStorageClass 'csi-cinder-sc-delete', got %s", promStack.GrafanaStorageClass)
		}
		if promStack.AlertmanagerVolumeSize != 10 {
			t.Errorf("expected AlertmanagerVolumeSize 10, got %d", promStack.AlertmanagerVolumeSize)
		}
		if promStack.AlertmanagerStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected AlertmanagerStorageClass 'csi-cinder-sc-delete', got %s", promStack.AlertmanagerStorageClass)
		}
	})

	// Test Secrets section initialization
	t.Run("Secrets section", func(t *testing.T) {
		// All service-specific secrets should be empty (must be provided by user)
		if cfg.Secrets.CertManager.AWSAccessKey != "" {
			t.Error("expected CertManager.AWSAccessKey to be empty")
		}
		if cfg.Secrets.CertManager.AWSSecretAccessKey != "" {
			t.Error("expected CertManager.AWSSecretAccessKey to be empty")
		}
		if cfg.Secrets.Loki.SwiftPassword != "" {
			t.Error("expected Loki.SwiftPassword to be empty")
		}
		if cfg.Secrets.Keycloak.ClientSecret != "" {
			t.Error("expected Keycloak.ClientSecret to be empty")
		}
		if cfg.Secrets.Keycloak.AdminPassword != "" {
			t.Error("expected Keycloak.AdminPassword to be empty")
		}
		if cfg.Secrets.Headlamp.OIDCClientSecret != "" {
			t.Error("expected Headlamp.OIDCClientSecret to be empty")
		}
		if cfg.Secrets.WeaveGitOps.Password != "" {
			t.Error("expected WeaveGitOps.Password to be empty")
		}
		if cfg.Secrets.WeaveGitOps.PasswordHash != "" {
			t.Error("expected WeaveGitOps.PasswordHash to be empty")
		}
		if cfg.Secrets.Grafana.AdminPassword != "" {
			t.Error("expected Grafana.AdminPassword to be empty")
		}
		if cfg.Secrets.AlertProxy.CoreDeviceId != "" {
			t.Error("expected AlertProxy.CoreDeviceId to be empty")
		}
		if cfg.Secrets.AlertProxy.AccountServiceToken != "" {
			t.Error("expected AlertProxy.AccountServiceToken to be empty")
		}
		if cfg.Secrets.AlertProxy.CoreAccountNumber != "" {
			t.Error("expected AlertProxy.CoreAccountNumber to be empty")
		}
	})
}

// TestDefaultConfigMatchesSpecifications tests that default values match the specifications
func TestDefaultConfigMatchesSpecifications(t *testing.T) {
	tests := []struct {
		name     string
		getValue func(Config) any
		expected any
	}{
		{
			name:     "BaseDomain default",
			getValue: func(c Config) any { return c.OpenCenter.Cluster.BaseDomain },
			expected: "k8s.opencenter.cloud",
		},
		{
			name:     "AdminEmail default",
			getValue: func(c Config) any { return c.OpenCenter.Cluster.AdminEmail },
			expected: "admin@example.com",
		},
		{
			name:     "GitOpsBaseRepo default",
			getValue: func(c Config) any { return c.OpenCenter.GitOps.GitOpsBaseRepo },
			expected: "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
		},
		{
			name:     "GitOpsBaseRelease default",
			getValue: func(c Config) any { return c.OpenCenter.GitOps.GitOpsBaseRelease },
			expected: "v0.1.0",
		},
		{
			name:     "GitOpsBranch default",
			getValue: func(c Config) any { return c.OpenCenter.GitOps.GitOpsBranch },
			expected: "main",
		},
		{
			name:     "DefaultStorageClass default",
			getValue: func(c Config) any { return c.OpenCenter.Storage.DefaultStorageClass },
			expected: "csi-cinder-sc-delete",
		},
		{
			name:     "LetsEncryptServer default",
			getValue: func(c Config) any { return c.OpenCenter.Services["cert-manager"].LetsEncryptServer },
			expected: "https://acme-v02.api.letsencrypt.org/directory",
		},
		{
			name:     "LokiVolumeSize default",
			getValue: func(c Config) any { return c.OpenCenter.Services["loki"].LokiVolumeSize },
			expected: 20,
		},
		{
			name:     "PrometheusVolumeSize default",
			getValue: func(c Config) any { return c.OpenCenter.Services["kube-prometheus-stack"].PrometheusVolumeSize },
			expected: 50,
		},
		{
			name:     "GrafanaVolumeSize default",
			getValue: func(c Config) any { return c.OpenCenter.Services["kube-prometheus-stack"].GrafanaVolumeSize },
			expected: 10,
		},
	}

	cfg := NewDefault("test-cluster")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.getValue(cfg)
			if actual != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

// TestValidateEmailFormat tests email validation
func TestValidateEmailFormat(t *testing.T) {
	validator := NewConfigValidator(false)

	tests := []struct {
		name        string
		email       string
		expectError bool
	}{
		{
			name:        "valid email",
			email:       "admin@example.com",
			expectError: false,
		},
		{
			name:        "valid email with subdomain",
			email:       "user@mail.example.com",
			expectError: false,
		},
		{
			name:        "valid email with plus",
			email:       "user+tag@example.com",
			expectError: false,
		},
		{
			name:        "valid email with dots",
			email:       "first.last@example.com",
			expectError: false,
		},
		{
			name:        "invalid email no at",
			email:       "adminexample.com",
			expectError: true,
		},
		{
			name:        "invalid email no domain",
			email:       "admin@",
			expectError: true,
		},
		{
			name:        "invalid email no local part",
			email:       "@example.com",
			expectError: true,
		},
		{
			name:        "invalid email no TLD",
			email:       "admin@example",
			expectError: true,
		},
		{
			name:        "empty email",
			email:       "",
			expectError: false, // Empty email is allowed, just not validated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefault("test")
			cfg.OpenCenter.GitOps.GitDir = "test-dir"
			cfg.OpenCenter.Cluster.AdminEmail = tt.email

			result := validator.ValidateStructure(context.Background(), &cfg)

			hasEmailError := false
			for _, err := range result.Errors {
				if err.Field == "opencenter.cluster.admin_email" {
					hasEmailError = true
					break
				}
			}

			if tt.expectError && !hasEmailError {
				t.Errorf("expected email validation error for %q, but got none", tt.email)
			}
			if !tt.expectError && hasEmailError {
				t.Errorf("expected no email validation error for %q, but got one", tt.email)
			}
		})
	}
}

// TestValidateDomainFormat tests domain validation
func TestValidateDomainFormat(t *testing.T) {
	validator := NewConfigValidator(false)

	tests := []struct {
		name        string
		domain      string
		field       string
		expectError bool
	}{
		{
			name:        "valid domain",
			domain:      "k8s.opencenter.cloud",
			field:       "base_domain",
			expectError: false,
		},
		{
			name:        "valid FQDN",
			domain:      "my-cluster.sjc3.k8s.opencenter.cloud",
			field:       "cluster_fqdn",
			expectError: false,
		},
		{
			name:        "valid domain with hyphens",
			domain:      "my-cluster.example-domain.com",
			field:       "base_domain",
			expectError: false,
		},
		{
			name:        "invalid domain no TLD",
			domain:      "example",
			field:       "base_domain",
			expectError: true,
		},
		{
			name:        "invalid domain no dot",
			domain:      "examplecom",
			field:       "base_domain",
			expectError: true,
		},
		{
			name:        "invalid domain starts with dot",
			domain:      ".example.com",
			field:       "base_domain",
			expectError: true,
		},
		{
			name:        "invalid domain ends with dot",
			domain:      "example.com.",
			field:       "base_domain",
			expectError: true,
		},
		{
			name:        "empty domain",
			domain:      "",
			field:       "base_domain",
			expectError: false, // Empty is allowed, just not validated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefault("test")
			cfg.OpenCenter.GitOps.GitDir = "test-dir"

			if tt.field == "base_domain" {
				cfg.OpenCenter.Cluster.BaseDomain = tt.domain
			} else {
				cfg.OpenCenter.Cluster.ClusterFQDN = tt.domain
			}

			result := validator.ValidateStructure(context.Background(), &cfg)

			hasDomainError := false
			expectedField := "opencenter.cluster." + tt.field
			for _, err := range result.Errors {
				if err.Field == expectedField {
					hasDomainError = true
					break
				}
			}

			if tt.expectError && !hasDomainError {
				t.Errorf("expected domain validation error for %q, but got none", tt.domain)
			}
			if !tt.expectError && hasDomainError {
				t.Errorf("expected no domain validation error for %q, but got one", tt.domain)
			}
		})
	}
}

// TestValidateServiceSpecificRequirements tests service-specific validation
func TestValidateServiceSpecificRequirements(t *testing.T) {
	validator := NewConfigValidator(false)

	tests := []struct {
		name          string
		setupConfig   func() Config
		expectedField string
		expectError   bool
	}{
		{
			name: "cert-manager missing AWS access key",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{Enabled: true}
				cfg.Secrets.CertManager.AWSAccessKey = ""
				cfg.Secrets.CertManager.AWSSecretAccessKey = "secret"
				return cfg
			},
			expectedField: "secrets.cert_manager.aws_access_key",
			expectError:   true,
		},
		{
			name: "cert-manager missing AWS secret key",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{Enabled: true}
				cfg.Secrets.CertManager.AWSAccessKey = "AKIA..."
				cfg.Secrets.CertManager.AWSSecretAccessKey = ""
				return cfg
			},
			expectedField: "secrets.cert_manager.aws_secret_access_key",
			expectError:   true,
		},
		{
			name: "cert-manager with valid secrets",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{Enabled: true}
				cfg.Secrets.CertManager.AWSAccessKey = "AKIA..."
				cfg.Secrets.CertManager.AWSSecretAccessKey = "secret"
				return cfg
			},
			expectedField: "secrets.cert_manager.aws_access_key",
			expectError:   false,
		},
		{
			name: "loki missing Swift auth URL",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["loki"] = ServiceCfg{
					Enabled:          true,
					SwiftAuthURL:     "",
					SwiftUsername:    "user",
					SwiftProjectName: "project",
				}
				cfg.Secrets.Loki.SwiftPassword = "password"
				return cfg
			},
			expectedField: "opencenter.services.loki.swift_auth_url",
			expectError:   true,
		},
		{
			name: "loki missing Swift password",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["loki"] = ServiceCfg{
					Enabled:          true,
					SwiftAuthURL:     "https://keystone.api.example.com/v3/",
					SwiftUsername:    "user",
					SwiftProjectName: "project",
				}
				cfg.Secrets.Loki.SwiftPassword = ""
				return cfg
			},
			expectedField: "secrets.loki.swift_password",
			expectError:   true,
		},
		{
			name: "keycloak missing admin password",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["keycloak"] = ServiceCfg{Enabled: true}
				cfg.Secrets.Keycloak.AdminPassword = ""
				return cfg
			},
			expectedField: "secrets.keycloak.admin_password",
			expectError:   true,
		},
		{
			name: "weave-gitops missing password hash",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["weave-gitops"] = ServiceCfg{Enabled: true}
				cfg.Secrets.WeaveGitOps.PasswordHash = ""
				return cfg
			},
			expectedField: "secrets.weave_gitops.password_hash",
			expectError:   true,
		},
		{
			name: "kube-prometheus-stack missing Grafana password",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				cfg.OpenCenter.Services["kube-prometheus-stack"] = ServiceCfg{Enabled: true}
				cfg.Secrets.Grafana.AdminPassword = ""
				return cfg
			},
			expectedField: "secrets.grafana.admin_password",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			result := validator.ValidateSemantics(context.Background(), &cfg)

			hasExpectedError := false
			for _, err := range result.Errors {
				if err.Field == tt.expectedField {
					hasExpectedError = true
					break
				}
			}

			if tt.expectError && !hasExpectedError {
				t.Errorf("expected validation error for field %q, but got none. Errors: %v", tt.expectedField, result.Errors)
			}
			if !tt.expectError && hasExpectedError {
				t.Errorf("expected no validation error for field %q, but got one", tt.expectedField)
			}
		})
	}
}

// TestValidateMissingRequiredFields tests that Validate catches missing required fields
func TestValidateMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() Config
		expectErrs  []string
	}{
		{
			name: "missing cluster name",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.ClusterName = ""
				cfg.OpenCenter.GitOps.GitDir = "test-dir"
				return cfg
			},
			expectErrs: []string{"opencenter.cluster.cluster_name must be set"},
		},
		{
			name: "missing git dir",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitDir = ""
				return cfg
			},
			expectErrs: []string{"opencenter.gitops.git_dir must be set"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			errs := Validate(cfg)

			if len(errs) == 0 {
				t.Errorf("expected validation errors, got none")
				return
			}

			for _, expectedErr := range tt.expectErrs {
				found := false
				for _, err := range errs {
					if strings.Contains(err, expectedErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q not found in: %v", expectedErr, errs)
				}
			}
		})
	}
}

// TestTemplateRenderingWithNewFields tests that new configuration fields can be used in templates
func TestTemplateRenderingWithNewFields(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		setupConfig  func() Config
		expected     string
	}{
		{
			name:         "BaseDomain rendering",
			templateText: "domain: {{ .OpenCenter.Cluster.BaseDomain }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.BaseDomain = "k8s.example.com"
				return cfg
			},
			expected: "domain: k8s.example.com",
		},
		{
			name:         "ClusterFQDN rendering",
			templateText: "fqdn: {{ .OpenCenter.Cluster.ClusterFQDN }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.ClusterFQDN = "test.sjc3.k8s.example.com"
				return cfg
			},
			expected: "fqdn: test.sjc3.k8s.example.com",
		},
		{
			name:         "AdminEmail rendering",
			templateText: "email: {{ .OpenCenter.Cluster.AdminEmail }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.AdminEmail = "admin@example.com"
				return cfg
			},
			expected: "email: admin@example.com",
		},
		{
			name:         "GitOpsBaseRepo rendering",
			templateText: "repo: {{ .OpenCenter.GitOps.GitOpsBaseRepo }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.GitOpsBaseRepo = "ssh://git@github.com/example/repo.git"
				return cfg
			},
			expected: "repo: ssh://git@github.com/example/repo.git",
		},
		{
			name:         "DefaultStorageClass rendering",
			templateText: "storageClass: {{ .OpenCenter.Storage.DefaultStorageClass }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Storage.DefaultStorageClass = "csi-cinder-sc-delete"
				return cfg
			},
			expected: "storageClass: csi-cinder-sc-delete",
		},
		{
			name:         "LetsEncryptServer rendering",
			templateText: `server: {{ (index .OpenCenter.Services "cert-manager").LetsEncryptServer }}`,
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Services["cert-manager"] = ServiceCfg{
					LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
				}
				return cfg
			},
			expected: "server: https://acme-v02.api.letsencrypt.org/directory",
		},
		{
			name:         "LokiVolumeSize rendering",
			templateText: "size: {{ .OpenCenter.Services.loki.LokiVolumeSize }}Gi",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Services["loki"] = ServiceCfg{
					LokiVolumeSize: 20,
				}
				return cfg
			},
			expected: "size: 20Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()

			// Use text/template to render
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var buf strings.Builder
			if err := tmpl.Execute(&buf, cfg); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestTemplateRenderingWithSecrets tests that secret values are properly rendered
func TestTemplateRenderingWithSecrets(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		setupConfig  func() Config
		expected     string
	}{
		{
			name:         "CertManager AWS access key",
			templateText: "key: {{ .Secrets.CertManager.AWSAccessKey }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.CertManager.AWSAccessKey = "AKIA..."
				return cfg
			},
			expected: "key: AKIA...",
		},
		{
			name:         "Loki Swift password",
			templateText: "password: {{ .Secrets.Loki.SwiftPassword }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.Loki.SwiftPassword = "secret-password"
				return cfg
			},
			expected: "password: secret-password",
		},
		{
			name:         "Keycloak admin password",
			templateText: "password: {{ .Secrets.Keycloak.AdminPassword }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.Keycloak.AdminPassword = "admin-secret"
				return cfg
			},
			expected: "password: admin-secret",
		},
		{
			name:         "Grafana admin password",
			templateText: "password: {{ .Secrets.Grafana.AdminPassword }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.Grafana.AdminPassword = "grafana-secret"
				return cfg
			},
			expected: "password: grafana-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()

			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var buf strings.Builder
			if err := tmpl.Execute(&buf, cfg); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestTemplateRenderingWithSprigFunctions tests Sprig function usage
func TestTemplateRenderingWithSprigFunctions(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		setupConfig  func() Config
		expected     string
	}{
		{
			name:         "b64enc function",
			templateText: "encoded: {{ .Secrets.Grafana.AdminPassword | b64enc }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.Grafana.AdminPassword = "password"
				return cfg
			},
			expected: "encoded: cGFzc3dvcmQ=",
		},
		{
			name:         "default function with value",
			templateText: "size: {{ .OpenCenter.Services.loki.LokiVolumeSize | default 20 }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Services["loki"] = ServiceCfg{
					LokiVolumeSize: 50,
				}
				return cfg
			},
			expected: "size: 50",
		},
		{
			name:         "default function with empty value",
			templateText: "size: {{ .OpenCenter.Services.loki.LokiVolumeSize | default 20 }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Services["loki"] = ServiceCfg{
					LokiVolumeSize: 0,
				}
				return cfg
			},
			expected: "size: 20",
		},
		{
			name:         "printf function",
			templateText: "url: {{ printf \"https://auth.%s\" .OpenCenter.Cluster.ClusterFQDN }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.ClusterFQDN = "test.example.com"
				return cfg
			},
			expected: "url: https://auth.test.example.com",
		},
		{
			name:         "nested default with printf",
			templateText: "bucket: {{ .OpenCenter.Services.loki.LokiBucketName | default (printf \"%s-loki\" .OpenCenter.Cluster.ClusterName) }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.ClusterName = "my-cluster"
				cfg.OpenCenter.Services["loki"] = ServiceCfg{
					LokiBucketName: "",
				}
				return cfg
			},
			expected: "bucket: my-cluster-loki",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()

			// Use Sprig functions
			tmpl, err := template.New("test").Funcs(sprig.TxtFuncMap()).Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var buf strings.Builder
			if err := tmpl.Execute(&buf, cfg); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestTemplateRenderingDefaultValues tests that default values work correctly in templates
func TestTemplateRenderingDefaultValues(t *testing.T) {
	cfg := NewDefault("test-cluster")

	tests := []struct {
		name         string
		templateText string
		expected     string
	}{
		{
			name:         "LetsEncryptServer with default",
			templateText: `{{ (index .OpenCenter.Services "cert-manager").LetsEncryptServer | default "https://acme-v02.api.letsencrypt.org/directory" }}`,
			expected:     "https://acme-v02.api.letsencrypt.org/directory",
		},
		{
			name:         "StorageClass with default",
			templateText: "{{ .OpenCenter.Services.loki.LokiStorageClass | default .OpenCenter.Storage.DefaultStorageClass | default \"csi-cinder-sc-delete\" }}",
			expected:     "csi-cinder-sc-delete",
		},
		{
			name:         "VolumeSize with default",
			templateText: "{{ .OpenCenter.Services.loki.LokiVolumeSize | default 20 }}Gi",
			expected:     "20Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Funcs(sprig.TxtFuncMap()).Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var buf strings.Builder
			if err := tmpl.Execute(&buf, cfg); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
