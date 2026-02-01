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

package di

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

func TestProvideLogger(t *testing.T) {
	logger, err := ProvideLogger()
	if err != nil {
		t.Fatalf("ProvideLogger() failed: %v", err)
	}
	if logger == nil {
		t.Error("ProvideLogger() returned nil logger")
	}
}

func TestProvidePathResolver(t *testing.T) {
	tmpDir := t.TempDir()
	resolver, err := ProvidePathResolver(tmpDir)
	if err != nil {
		t.Fatalf("ProvidePathResolver() failed: %v", err)
	}
	if resolver == nil {
		t.Error("ProvidePathResolver() returned nil resolver")
	}
}

func TestProvideConfigManager(t *testing.T) {
	configMgr, err := ProvideConfigManager()
	if err != nil {
		t.Fatalf("ProvideConfigManager() failed: %v", err)
	}
	if configMgr == nil {
		t.Error("ProvideConfigManager() returned nil config manager")
	}
}

func TestProvideValidationEngine(t *testing.T) {
	engine, err := ProvideValidationEngine()
	if err != nil {
		t.Fatalf("ProvideValidationEngine() failed: %v", err)
	}
	if engine == nil {
		t.Error("ProvideValidationEngine() returned nil engine")
	}

	// Verify validators are registered
	validators := []string{"cluster-name", "config", "file", "security"}
	for _, name := range validators {
		result, err := engine.Validate(nil, name, "test-value")
		// We expect validation to run (may pass or fail depending on value)
		// but should not error due to missing validator
		if err != nil && err.Error() == "validator not found: "+name {
			t.Errorf("Validator '%s' not registered", name)
		}
		_ = result // Ignore validation result, just checking registration
	}
}

func TestProvideErrorFormatter(t *testing.T) {
	formatter, err := ProvideErrorFormatter()
	if err != nil {
		t.Fatalf("ProvideErrorFormatter() failed: %v", err)
	}
	if formatter == nil {
		t.Error("ProvideErrorFormatter() returned nil formatter")
	}
}

func TestProvideInitService(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dependencies
	pathResolver, err := ProvidePathResolver(tmpDir)
	if err != nil {
		t.Fatalf("ProvidePathResolver() failed: %v", err)
	}

	validator, err := ProvideValidationEngine()
	if err != nil {
		t.Fatalf("ProvideValidationEngine() failed: %v", err)
	}

	// Create service
	service, err := ProvideInitService(pathResolver, validator)
	if err != nil {
		t.Fatalf("ProvideInitService() failed: %v", err)
	}
	if service == nil {
		t.Error("ProvideInitService() returned nil service")
	}
}

func TestProvideValidateService(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dependencies
	pathResolver, err := ProvidePathResolver(tmpDir)
	if err != nil {
		t.Fatalf("ProvidePathResolver() failed: %v", err)
	}

	validator, err := ProvideValidationEngine()
	if err != nil {
		t.Fatalf("ProvideValidationEngine() failed: %v", err)
	}

	configManager, err := ProvideConfigManager()
	if err != nil {
		t.Fatalf("ProvideConfigManager() failed: %v", err)
	}

	// Create service
	service, err := ProvideValidateService(pathResolver, validator, configManager)
	if err != nil {
		t.Fatalf("ProvideValidateService() failed: %v", err)
	}
	if service == nil {
		t.Error("ProvideValidateService() returned nil service")
	}
}

func TestProvideSetupService(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dependencies
	pathResolver, err := ProvidePathResolver(tmpDir)
	if err != nil {
		t.Fatalf("ProvidePathResolver() failed: %v", err)
	}

	validator, err := ProvideValidationEngine()
	if err != nil {
		t.Fatalf("ProvideValidationEngine() failed: %v", err)
	}

	// Create service
	service, err := ProvideSetupService(pathResolver, validator)
	if err != nil {
		t.Fatalf("ProvideSetupService() failed: %v", err)
	}
	if service == nil {
		t.Error("ProvideSetupService() returned nil service")
	}
}

func TestProvideBootstrapService(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dependencies
	pathResolver, err := ProvidePathResolver(tmpDir)
	if err != nil {
		t.Fatalf("ProvidePathResolver() failed: %v", err)
	}

	validator, err := ProvideValidationEngine()
	if err != nil {
		t.Fatalf("ProvideValidationEngine() failed: %v", err)
	}

	// Create service
	service, err := ProvideBootstrapService(pathResolver, validator)
	if err != nil {
		t.Fatalf("ProvideBootstrapService() failed: %v", err)
	}
	if service == nil {
		t.Error("ProvideBootstrapService() returned nil service")
	}
}

func TestProviderIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "clusters")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create base directory: %v", err)
	}

	// Test that all providers work together
	logger, err := ProvideLogger()
	if err != nil {
		t.Fatalf("ProvideLogger() failed: %v", err)
	}

	pathResolver, err := ProvidePathResolver(baseDir)
	if err != nil {
		t.Fatalf("ProvidePathResolver() failed: %v", err)
	}

	configMgr, err := ProvideConfigManager()
	if err != nil {
		t.Fatalf("ProvideConfigManager() failed: %v", err)
	}

	validator, err := ProvideValidationEngine()
	if err != nil {
		t.Fatalf("ProvideValidationEngine() failed: %v", err)
	}

	errorFormatter, err := ProvideErrorFormatter()
	if err != nil {
		t.Fatalf("ProvideErrorFormatter() failed: %v", err)
	}

	// Create all services
	initService, err := ProvideInitService(pathResolver, validator)
	if err != nil {
		t.Fatalf("ProvideInitService() failed: %v", err)
	}

	validateService, err := ProvideValidateService(pathResolver, validator, configMgr)
	if err != nil {
		t.Fatalf("ProvideValidateService() failed: %v", err)
	}

	setupService, err := ProvideSetupService(pathResolver, validator)
	if err != nil {
		t.Fatalf("ProvideSetupService() failed: %v", err)
	}

	bootstrapService, err := ProvideBootstrapService(pathResolver, validator)
	if err != nil {
		t.Fatalf("ProvideBootstrapService() failed: %v", err)
	}

	// Verify all components are non-nil
	if logger == nil {
		t.Error("Logger is nil")
	}
	if pathResolver == nil {
		t.Error("PathResolver is nil")
	}
	if configMgr == nil {
		t.Error("ConfigManager is nil")
	}
	if validator == nil {
		t.Error("ValidationEngine is nil")
	}
	if errorFormatter == nil {
		t.Error("ErrorFormatter is nil")
	}
	if initService == nil {
		t.Error("InitService is nil")
	}
	if validateService == nil {
		t.Error("ValidateService is nil")
	}
	if setupService == nil {
		t.Error("SetupService is nil")
	}
	if bootstrapService == nil {
		t.Error("BootstrapService is nil")
	}
}

func TestProviderWithContainer(t *testing.T) {
	// Test that providers can be used with the DI container
	container := NewContainer()
	tmpDir := t.TempDir()

	// Register providers
	if err := container.Register("logger", ProvideLogger); err != nil {
		t.Fatalf("Failed to register logger: %v", err)
	}

	if err := container.Register("pathResolver", func() (*paths.PathResolver, error) {
		return ProvidePathResolver(tmpDir)
	}); err != nil {
		t.Fatalf("Failed to register pathResolver: %v", err)
	}

	if err := container.Register("configManager", ProvideConfigManager); err != nil {
		t.Fatalf("Failed to register configManager: %v", err)
	}

	if err := container.Register("validationEngine", ProvideValidationEngine); err != nil {
		t.Fatalf("Failed to register validationEngine: %v", err)
	}

	if err := container.Register("errorFormatter", ProvideErrorFormatter); err != nil {
		t.Fatalf("Failed to register errorFormatter: %v", err)
	}

	// Register services with dependencies
	if err := container.Register("initService", func(
		pr *paths.PathResolver,
		ve *validation.ValidationEngine,
	) (*interface{}, error) {
		svc, err := ProvideInitService(pr, ve)
		var result interface{} = svc
		return &result, err
	}); err != nil {
		t.Fatalf("Failed to register initService: %v", err)
	}

	// Resolve components
	logger, err := container.Resolve("logger")
	if err != nil {
		t.Errorf("Failed to resolve logger: %v", err)
	}
	if logger == nil {
		t.Error("Resolved logger is nil")
	}

	pathResolver, err := container.Resolve("pathResolver")
	if err != nil {
		t.Errorf("Failed to resolve pathResolver: %v", err)
	}
	if pathResolver == nil {
		t.Error("Resolved pathResolver is nil")
	}

	configMgr, err := container.Resolve("configManager")
	if err != nil {
		t.Errorf("Failed to resolve configManager: %v", err)
	}
	if configMgr == nil {
		t.Error("Resolved configManager is nil")
	}
}
