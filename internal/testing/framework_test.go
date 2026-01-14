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
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTestFramework(t *testing.T) {
	fw := NewTestFramework(t)

	// Verify temporary directory exists
	if _, err := os.Stat(fw.TempDir); os.IsNotExist(err) {
		t.Errorf("expected temp directory to exist at %s", fw.TempDir)
	}

	// Verify config directory exists
	if _, err := os.Stat(fw.ConfigDir); os.IsNotExist(err) {
		t.Errorf("expected config directory to exist at %s", fw.ConfigDir)
	}

	// Verify template directory exists
	if _, err := os.Stat(fw.TemplateDir); os.IsNotExist(err) {
		t.Errorf("expected template directory to exist at %s", fw.TemplateDir)
	}

	// Verify template engine is initialized
	if fw.TemplateEngine == nil {
		t.Error("expected template engine to be initialized")
	}

	// Verify generators are initialized
	if fw.ConfigGenerator == nil {
		t.Error("expected config generator to be initialized")
	}

	if fw.TemplateDataGenerator == nil {
		t.Error("expected template data generator to be initialized")
	}

	if fw.ServiceDataGenerator == nil {
		t.Error("expected service data generator to be initialized")
	}

	if fw.GitOpsDataGenerator == nil {
		t.Error("expected gitops data generator to be initialized")
	}
}

func TestNewTestFrameworkWithSeed(t *testing.T) {
	seed := int64(12345)
	fw1 := NewTestFrameworkWithSeed(t, seed)
	fw2 := NewTestFrameworkWithSeed(t, seed)

	// Verify framework is initialized
	if fw1 == nil || fw2 == nil {
		t.Fatal("expected frameworks to be initialized")
	}

	// Verify generators use the custom seed by generating data
	config1 := fw1.CreateTestConfig("openstack")
	config2 := fw2.CreateTestConfig("openstack")

	// With the same seed, generated configs should be identical
	if config1.OpenCenter.Meta.Name != config2.OpenCenter.Meta.Name {
		t.Errorf("expected identical configs with same seed, got different cluster names: %s vs %s",
			config1.OpenCenter.Meta.Name, config2.OpenCenter.Meta.Name)
	}
}

func TestWriteTemplate(t *testing.T) {
	fw := NewTestFramework(t)

	templateContent := "Hello {{ .Name }}!"
	path := fw.WriteTemplate(t, "test.tmpl", templateContent)

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected template file to exist at %s", path)
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read template file: %v", err)
	}

	if string(content) != templateContent {
		t.Errorf("expected template content %q, got %q", templateContent, string(content))
	}

	// Verify path is in template directory
	expectedPath := filepath.Join(fw.TemplateDir, "test.tmpl")
	if path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, path)
	}
}

func TestCreateTestConfig(t *testing.T) {
	fw := NewTestFramework(t)

	providers := []string{"openstack", "aws", "baremetal"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			cfg := fw.CreateTestConfig(provider)

			// Verify basic config structure
			if cfg.OpenCenter.Infrastructure.Provider != provider {
				t.Errorf("expected provider %s, got %s", provider, cfg.OpenCenter.Infrastructure.Provider)
			}

			if cfg.OpenCenter.Meta.Name == "" {
				t.Error("expected cluster name to be set")
			}

			if cfg.OpenCenter.Meta.Organization == "" {
				t.Error("expected organization to be set")
			}
		})
	}
}

func TestCreateTestTemplateData(t *testing.T) {
	fw := NewTestFramework(t)

	data := fw.CreateTestTemplateData()

	// Verify required fields exist
	requiredFields := []string{
		"ClusterName",
		"Namespace",
		"Version",
		"Replicas",
		"Image",
		"Port",
		"Environment",
		"Labels",
		"Annotations",
		"Resources",
	}

	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			t.Errorf("expected field %s to exist in template data", field)
		}
	}
}

func TestCreateTestServiceDefinition(t *testing.T) {
	fw := NewTestFramework(t)

	service := fw.CreateTestServiceDefinition()

	// Verify required fields exist
	requiredFields := []string{"name", "type", "enabled", "version", "dependencies", "config"}

	for _, field := range requiredFields {
		if _, ok := service[field]; !ok {
			t.Errorf("expected field %s to exist in service definition", field)
		}
	}
}

func TestCreateTestGitOpsConfig(t *testing.T) {
	fw := NewTestFramework(t)

	gitops := fw.CreateTestGitOpsConfig()

	// Verify required fields exist
	if _, ok := gitops["enabled"]; !ok {
		t.Error("expected enabled field to exist")
	}

	if _, ok := gitops["repository"]; !ok {
		t.Error("expected repository field to exist")
	}

	if _, ok := gitops["branch"]; !ok {
		t.Error("expected branch field to exist")
	}

	if _, ok := gitops["path"]; !ok {
		t.Error("expected path field to exist")
	}

	if _, ok := gitops["sync"]; !ok {
		t.Error("expected sync field to exist")
	}
}

func TestAssertFileExists(t *testing.T) {
	fw := NewTestFramework(t)

	// Create a test file
	testFile := filepath.Join(fw.TempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// This should not fail
	fw.AssertFileExists(t, testFile)

	// Test with non-existent file - we can't easily test the error case
	// without a mock, so we'll just verify the method exists
}

func TestAssertDirExists(t *testing.T) {
	fw := NewTestFramework(t)

	// Test with existing directory
	fw.AssertDirExists(t, fw.TempDir)

	// Test with non-existent directory - we can't easily test the error case
	// without a mock, so we'll just verify the method exists
}

func TestDeterministicGeneration(t *testing.T) {
	// Test that the same seed produces the same results
	seed := int64(999)

	fw1 := NewTestFrameworkWithSeed(t, seed)
	fw2 := NewTestFrameworkWithSeed(t, seed)

	config1 := fw1.CreateTestConfig("openstack")
	config2 := fw2.CreateTestConfig("openstack")

	// Verify deterministic generation
	if config1.OpenCenter.Meta.Name != config2.OpenCenter.Meta.Name {
		t.Errorf("expected same cluster name with same seed, got %s vs %s",
			config1.OpenCenter.Meta.Name, config2.OpenCenter.Meta.Name)
	}

	if config1.OpenCenter.Meta.Organization != config2.OpenCenter.Meta.Organization {
		t.Errorf("expected same organization with same seed, got %s vs %s",
			config1.OpenCenter.Meta.Organization, config2.OpenCenter.Meta.Organization)
	}
}

func TestMockImplementationsAvailable(t *testing.T) {
	fw := NewTestFramework(t)

	// Verify all mock implementations are initialized
	assert.NotNil(t, fw.MockTemplateEngine, "MockTemplateEngine should be initialized")
	assert.NotNil(t, fw.MockConfigBuilder, "MockConfigBuilder should be initialized")
	assert.NotNil(t, fw.MockConfigValidator, "MockConfigValidator should be initialized")
	assert.NotNil(t, fw.MockTemplateRegistry, "MockTemplateRegistry should be initialized")
	assert.NotNil(t, fw.MockGitOpsGenerator, "MockGitOpsGenerator should be initialized")
	assert.NotNil(t, fw.MockServiceRegistry, "MockServiceRegistry should be initialized")
	assert.NotNil(t, fw.MockMigrationManager, "MockMigrationManager should be initialized")
	assert.NotNil(t, fw.MockMCPServer, "MockMCPServer should be initialized")
	assert.NotNil(t, fw.MockAuthProvider, "MockAuthProvider should be initialized")
	assert.NotNil(t, fw.MockErrorAggregator, "MockErrorAggregator should be initialized")

	// Verify mock implementations are functional
	t.Run("MockTemplateEngine is functional", func(t *testing.T) {
		result, err := fw.MockTemplateEngine.Render(context.Background(), "test.tmpl", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, fw.MockTemplateEngine.RenderCalls, 1)
	})

	t.Run("MockConfigBuilder is functional", func(t *testing.T) {
		builder := fw.MockConfigBuilder.WithProvider("test").WithOrganization("test-org")
		assert.NotNil(t, builder)
		cfg, err := builder.Build()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
	})

	t.Run("MockErrorAggregator is functional", func(t *testing.T) {
		fw.MockErrorAggregator.Add(errors.New("test error"))
		assert.True(t, fw.MockErrorAggregator.HasErrors())
		assert.Len(t, fw.MockErrorAggregator.Errors(), 1)
	})
}
