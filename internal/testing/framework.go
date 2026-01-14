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

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/template"
)

// TestFramework provides a consistent testing environment for openCenter tests.
// It manages temporary directories, mock implementations, and test data generators.
type TestFramework struct {
	// TempDir is the root temporary directory for test artifacts
	TempDir string

	// ConfigDir is the directory for test configuration files
	ConfigDir string

	// TemplateDir is the directory for test template files
	TemplateDir string

	// TemplateEngine is the template engine instance for tests
	TemplateEngine template.TemplateEngine

	// ConfigGenerator generates realistic test configurations
	ConfigGenerator *ConfigGenerator

	// TemplateDataGenerator generates realistic template test data
	TemplateDataGenerator *TemplateDataGenerator

	// ServiceDataGenerator generates realistic service test data
	ServiceDataGenerator *ServiceDataGenerator

	// GitOpsDataGenerator generates realistic GitOps test data
	GitOpsDataGenerator *GitOpsDataGenerator

	// Mock implementations for testing
	MockTemplateEngine    *MockTemplateEngine
	MockConfigBuilder     *MockConfigBuilder
	MockConfigValidator   *MockConfigValidator
	MockTemplateRegistry  *MockTemplateRegistry
	MockGitOpsGenerator   *MockGitOpsGenerator
	MockServiceRegistry   *MockServiceRegistry
	MockMigrationManager  *MockMigrationManager
	MockMCPServer         *MockMCPServer
	MockAuthProvider      *MockAuthProvider
	MockErrorAggregator   *MockErrorAggregator

	// t is the testing.T instance for cleanup
	t *testing.T
}

// NewTestFramework creates a new test framework with a consistent environment.
// It sets up temporary directories, initializes generators, and registers cleanup.
func NewTestFramework(t *testing.T) *TestFramework {
	t.Helper()

	// Create temporary directory
	tempDir := t.TempDir()

	// Create subdirectories
	configDir := filepath.Join(tempDir, "configs")
	templateDir := filepath.Join(tempDir, "templates")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config directory: %v", err)
	}

	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("failed to create template directory: %v", err)
	}

	// Initialize template engine
	engine := template.NewGoTemplateEngine()
	engine.SetCacheEnabled(true)

	// Initialize generators with deterministic seed for reproducibility
	seed := int64(42)

	return &TestFramework{
		TempDir:               tempDir,
		ConfigDir:             configDir,
		TemplateDir:           templateDir,
		TemplateEngine:        engine,
		ConfigGenerator:       NewConfigGenerator(seed),
		TemplateDataGenerator: NewTemplateDataGenerator(seed),
		ServiceDataGenerator:  NewServiceDataGenerator(seed),
		GitOpsDataGenerator:   NewGitOpsDataGenerator(seed),
		MockTemplateEngine:    NewMockTemplateEngine(),
		MockConfigBuilder:     NewMockConfigBuilder(),
		MockConfigValidator:   NewMockConfigValidator(),
		MockTemplateRegistry:  NewMockTemplateRegistry(),
		MockGitOpsGenerator:   NewMockGitOpsGenerator(),
		MockServiceRegistry:   NewMockServiceRegistry(),
		MockMigrationManager:  NewMockMigrationManager(),
		MockMCPServer:         NewMockMCPServer(),
		MockAuthProvider:      NewMockAuthProvider(),
		MockErrorAggregator:   NewMockErrorAggregator(),
		t:                     t,
	}
}

// NewTestFrameworkWithSeed creates a test framework with a custom seed for generators.
// This is useful when you need different random data in different tests.
func NewTestFrameworkWithSeed(t *testing.T, seed int64) *TestFramework {
	t.Helper()

	// Create temporary directory
	tempDir := t.TempDir()

	// Create subdirectories
	configDir := filepath.Join(tempDir, "configs")
	templateDir := filepath.Join(tempDir, "templates")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config directory: %v", err)
	}

	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("failed to create template directory: %v", err)
	}

	// Initialize template engine
	engine := template.NewGoTemplateEngine()
	engine.SetCacheEnabled(true)

	// Initialize generators with custom seed
	return &TestFramework{
		TempDir:               tempDir,
		ConfigDir:             configDir,
		TemplateDir:           templateDir,
		TemplateEngine:        engine,
		ConfigGenerator:       NewConfigGenerator(seed),
		TemplateDataGenerator: NewTemplateDataGenerator(seed),
		ServiceDataGenerator:  NewServiceDataGenerator(seed),
		GitOpsDataGenerator:   NewGitOpsDataGenerator(seed),
		MockTemplateEngine:    NewMockTemplateEngine(),
		MockConfigBuilder:     NewMockConfigBuilder(),
		MockConfigValidator:   NewMockConfigValidator(),
		MockTemplateRegistry:  NewMockTemplateRegistry(),
		MockGitOpsGenerator:   NewMockGitOpsGenerator(),
		MockServiceRegistry:   NewMockServiceRegistry(),
		MockMigrationManager:  NewMockMigrationManager(),
		MockMCPServer:         NewMockMCPServer(),
		MockAuthProvider:      NewMockAuthProvider(),
		MockErrorAggregator:   NewMockErrorAggregator(),
		t:                     t,
	}
}

// WriteTemplate writes a template to a file in the test template directory.
// Returns the path to the written file.
func (fw *TestFramework) WriteTemplate(t *testing.T, filename string, content string) string {
	t.Helper()

	path := filepath.Join(fw.TemplateDir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write template to %s: %v", path, err)
	}

	return path
}

// WriteFile writes arbitrary content to a file in the temp directory.
// Returns the path to the written file.
func (fw *TestFramework) WriteFile(t *testing.T, filename string, content []byte) string {
	t.Helper()

	path := filepath.Join(fw.TempDir, filename)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write file to %s: %v", path, err)
	}

	return path
}

// CreateTestConfig creates a test configuration with the given provider.
func (fw *TestFramework) CreateTestConfig(provider string) config.Config {
	return fw.ConfigGenerator.GenerateConfig(provider)
}

// CreateTestTemplateData creates test data for template rendering.
func (fw *TestFramework) CreateTestTemplateData() map[string]interface{} {
	return fw.TemplateDataGenerator.GenerateTemplateData()
}

// CreateTestServiceDefinition creates a test service definition.
func (fw *TestFramework) CreateTestServiceDefinition() map[string]interface{} {
	return fw.ServiceDataGenerator.GenerateServiceDefinition()
}

// CreateTestGitOpsConfig creates a test GitOps configuration.
func (fw *TestFramework) CreateTestGitOpsConfig() map[string]interface{} {
	return fw.GitOpsDataGenerator.GenerateGitOpsConfig()
}

// AssertFileExists asserts that a file exists at the given path.
func (fw *TestFramework) AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist at %s, but it does not", path)
	}
}

// AssertFileNotExists asserts that a file does not exist at the given path.
func (fw *TestFramework) AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file not to exist at %s, but it does", path)
	}
}

// AssertDirExists asserts that a directory exists at the given path.
func (fw *TestFramework) AssertDirExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("expected directory to exist at %s, but it does not", path)
		return
	}

	if !info.IsDir() {
		t.Errorf("expected %s to be a directory, but it is a file", path)
	}
}

// AssertDirNotExists asserts that a directory does not exist at the given path.
func (fw *TestFramework) AssertDirNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected directory not to exist at %s, but it does", path)
	}
}

// Cleanup performs cleanup operations for the test framework.
// This is automatically called by testing.T.Cleanup() when using t.TempDir().
func (fw *TestFramework) Cleanup() {
	// Cleanup is handled automatically by t.TempDir()
	// This method is provided for explicit cleanup if needed
}
