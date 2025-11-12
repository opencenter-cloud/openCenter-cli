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
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

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
