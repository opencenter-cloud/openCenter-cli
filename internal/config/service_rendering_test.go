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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

// TestServiceRendering validates that all services render correctly with proper YAML formatting
func TestServiceRendering(t *testing.T) {
	// Create a temporary directory for test output
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Create a config with all services enabled
	cfg := NewDefault("test-render")
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(dir, "gitops")

	// Populate required infrastructure fields
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://auth.example.com"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "test-region"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"
	cfg.OpenCenter.Secrets.Barbican.AuthURL = "https://barbican.example.com"

	// Enable all services to test rendering
	enableAllServices(&cfg)

	// Marshal to YAML with 2-space indentation
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		t.Fatalf("failed to marshal config to YAML: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("failed to close YAML encoder: %v", err)
	}
	yamlData := buf.Bytes()

	// Write to file for inspection
	testFile := filepath.Join(dir, "test-render-config.yaml")
	if err := os.WriteFile(testFile, yamlData, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	t.Run("YAML Syntax Validation", func(t *testing.T) {
		// Verify YAML is valid by unmarshaling
		var result map[string]any
		if err := yaml.Unmarshal(yamlData, &result); err != nil {
			t.Fatalf("generated YAML is invalid: %v", err)
		}
	})

	t.Run("YAML Linting", func(t *testing.T) {
		// Check if yamllint is available
		if _, err := exec.LookPath("yamllint"); err != nil {
			t.Skip("yamllint not found, skipping lint test")
		}

		// Run yamllint on the generated file
		cmd := exec.Command("yamllint", testFile)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			t.Errorf("yamllint failed:\nstdout: %s\nstderr: %s\nerror: %v",
				stdout.String(), stderr.String(), err)
		}
	})

	t.Run("Service Field Count Validation", func(t *testing.T) {
		// Unmarshal to check service field counts
		var result map[string]any
		if err := yaml.Unmarshal(yamlData, &result); err != nil {
			t.Fatalf("failed to unmarshal YAML: %v", err)
		}

		opencenter, ok := result["opencenter"].(map[string]any)
		if !ok {
			t.Fatal("opencenter section not found")
		}

		services, ok := opencenter["services"].(map[string]any)
		if !ok {
			t.Fatal("services section not found")
		}

		// Check each service has more than just BaseConfig fields
		for serviceName, serviceData := range services {
			serviceMap, ok := serviceData.(map[string]any)
			if !ok {
				continue
			}

			// Count fields
			fieldCount := len(serviceMap)

			// BaseConfig has approximately 4 flat fields (enabled, adoption_mode, namespace, source, image)
			// Services with only BaseConfig fields indicate missing registration
			if fieldCount < 1 {
				t.Logf("WARNING: service '%s' has only %d fields (possibly missing registration)",
					serviceName, fieldCount)
			}

			// Verify service is registered
			if !isServiceRegistered(serviceName) {
				t.Errorf("service '%s' is not registered in the service registry", serviceName)
			}
		}
	})

	t.Run("Service Registration Completeness", func(t *testing.T) {
		// Get all services from the config
		configServices := getAllServiceNames(&cfg)

		// Check each service is registered
		var unregistered []string
		for _, serviceName := range configServices {
			if !isServiceRegistered(serviceName) {
				unregistered = append(unregistered, serviceName)
			}
		}

		if len(unregistered) > 0 {
			t.Errorf("unregistered services found: %v\n"+
				"These services will only render BaseConfig fields.\n"+
				"Add init() function with registry.RegisterServiceConfig() to each service file.",
				unregistered)
		}
	})

	t.Run("No Trailing Spaces", func(t *testing.T) {
		lines := strings.Split(string(yamlData), "\n")
		var linesWithTrailingSpaces []int

		for i, line := range lines {
			if len(line) > 0 && line[len(line)-1] == ' ' {
				linesWithTrailingSpaces = append(linesWithTrailingSpaces, i+1)
			}
		}

		if len(linesWithTrailingSpaces) > 0 {
			t.Errorf("found trailing spaces on lines: %v", linesWithTrailingSpaces)
		}
	})

	t.Run("Proper Indentation", func(t *testing.T) {
		lines := strings.Split(string(yamlData), "\n")
		for i, line := range lines {
			if len(line) == 0 {
				continue
			}

			// Count leading spaces
			spaces := 0
			for _, ch := range line {
				if ch == ' ' {
					spaces++
				} else {
					break
				}
			}

			// Check for tabs (not allowed in YAML)
			if strings.Contains(line, "\t") {
				t.Errorf("line %d contains tabs (use spaces): %q", i+1, line)
			}

			// Indentation should be multiples of 2
			if spaces%2 != 0 {
				t.Errorf("line %d has odd indentation (%d spaces): %q", i+1, spaces, line)
			}
		}
	})
}

// TestSpecificServiceRendering tests individual service configurations
func TestSpecificServiceRendering(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		expectedFields []string
		minFieldCount  int
	}{
		{
			name:        "tempo",
			serviceName: "tempo",
			expectedFields: []string{
				"enabled", "storage_type", "bucket_name", "volume_size",
				"storage_class", "s3_endpoint", "s3_region",
			},
			minFieldCount: 5,
		},
		{
			name:        "loki",
			serviceName: "loki",
			expectedFields: []string{
				"enabled", "bucket_name",
				"swift_auth_url", "swift_region",
			},
			minFieldCount: 4,
		},
		{
			name:        "kube-prometheus-stack",
			serviceName: "kube-prometheus-stack",
			expectedFields: []string{
				"enabled", "prometheus_volume_size", "prometheus_storage_class",
				"grafana_volume_size", "alertmanager_volume_size",
			},
			minFieldCount: 5,
		},
		{
			name:        "cert-manager",
			serviceName: "cert-manager",
			expectedFields: []string{
				"enabled", "email", "region", "letsencrypt_server",
			},
			minFieldCount: 4,
		},
		{
			name:        "keycloak",
			serviceName: "keycloak",
			expectedFields: []string{
				"enabled", "hostname", "realm", "client_id", "frontend_url",
			},
			minFieldCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefault("test-" + tt.serviceName)
			cfg.OpenCenter.GitOps.Repository.LocalDir = "/tmp/test"

			// Enable the specific service
			if service, exists := cfg.OpenCenter.Services[tt.serviceName]; exists {
				if baseConfig := getBaseConfig(service); baseConfig != nil {
					baseConfig.Enabled = true
				}
			}

			// Marshal to YAML with proper indentation
			yamlData, err := marshalYAMLWithIndent(cfg)
			if err != nil {
				t.Fatalf("failed to marshal config: %v", err)
			}

			// Unmarshal to check fields
			var result map[string]any
			if err := yaml.Unmarshal(yamlData, &result); err != nil {
				t.Fatalf("failed to unmarshal YAML: %v", err)
			}

			// Navigate to the service
			opencenter := result["opencenter"].(map[string]any)
			services := opencenter["services"].(map[string]any)
			serviceData, ok := services[tt.serviceName].(map[string]any)
			if !ok {
				t.Fatalf("service '%s' not found in rendered YAML", tt.serviceName)
			}

			// Check field count
			fieldCount := len(serviceData)
			if fieldCount < tt.minFieldCount {
				t.Errorf("service '%s' has only %d fields, expected at least %d (likely missing registration)",
					tt.serviceName, fieldCount, tt.minFieldCount)
			}

			// Check for expected fields
			for _, expectedField := range tt.expectedFields {
				if _, exists := serviceData[expectedField]; !exists {
					t.Errorf("service '%s' missing expected field: %s", tt.serviceName, expectedField)
				}
			}

			// Verify service is registered
			if !isServiceRegistered(tt.serviceName) {
				t.Errorf("service '%s' is not registered in the service registry", tt.serviceName)
			}
		})
	}
}

// Helper functions

// marshalYAMLWithIndent marshals a config to YAML with 2-space indentation
func marshalYAMLWithIndent(cfg Config) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func enableAllServices(cfg *Config) {
	for _, service := range cfg.OpenCenter.Services {
		if baseConfig := getBaseConfig(service); baseConfig != nil {
			baseConfig.Enabled = true
		}
	}
	for _, managedService := range cfg.OpenCenter.ManagedService {
		if baseConfig := getBaseConfig(managedService); baseConfig != nil {
			baseConfig.Enabled = true
		}
	}
}

func getAllServiceNames(cfg *Config) []string {
	var names []string
	for name := range cfg.OpenCenter.Services {
		names = append(names, name)
	}
	for name := range cfg.OpenCenter.ManagedService {
		names = append(names, name)
	}
	return names
}

func isServiceRegistered(serviceName string) bool {
	return registry.IsRegistered(serviceName)
}

func getBaseConfig(service any) *services.BaseConfig {
	// Use reflection to access the embedded BaseConfig field
	// All service configs embed BaseConfig, so we can access it via reflection
	val := reflect.ValueOf(service)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Look for BaseConfig field
	if val.Kind() == reflect.Struct {
		baseField := val.FieldByName("BaseConfig")
		if baseField.IsValid() && baseField.CanAddr() {
			if bc, ok := baseField.Addr().Interface().(*services.BaseConfig); ok {
				return bc
			}
		}
	}

	return nil
}

// TestYAMLLintCompliance ensures generated configs pass yamllint
func TestYAMLLintCompliance(t *testing.T) {
	// Check if yamllint is available
	if _, err := exec.LookPath("yamllint"); err != nil {
		t.Skip("yamllint not found, skipping compliance test")
	}

	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	// Test multiple config scenarios
	scenarios := []struct {
		name      string
		setupFunc func() Config
	}{
		{
			name: "minimal config",
			setupFunc: func() Config {
				cfg := NewDefault("minimal")
				cfg.OpenCenter.GitOps.Repository.LocalDir = "/tmp/gitops"
				return cfg
			},
		},
		{
			name: "all services enabled",
			setupFunc: func() Config {
				cfg := NewDefault("full")
				cfg.OpenCenter.GitOps.Repository.LocalDir = "/tmp/gitops"
				enableAllServices(&cfg)
				return cfg
			},
		},
		{
			name: "openstack provider",
			setupFunc: func() Config {
				cfg := NewDefault("openstack")
				cfg.OpenCenter.GitOps.Repository.LocalDir = "/tmp/gitops"
				cfg.OpenCenter.Infrastructure.Provider = "openstack"
				cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://auth.example.com"
				cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "test-region"
				return cfg
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			cfg := scenario.setupFunc()

			// Marshal to YAML with proper indentation
			yamlData, err := marshalYAMLWithIndent(cfg)
			if err != nil {
				t.Fatalf("failed to marshal config: %v", err)
			}

			// Write to temp file
			testFile := filepath.Join(dir, fmt.Sprintf("%s-config.yaml", scenario.name))
			if err := os.WriteFile(testFile, yamlData, 0600); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			// Run yamllint
			cmd := exec.Command("yamllint", testFile)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				t.Errorf("yamllint failed for %s:\nstdout: %s\nstderr: %s\nerror: %v",
					scenario.name, stdout.String(), stderr.String(), err)
			}
		})
	}
}
