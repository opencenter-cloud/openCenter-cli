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
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

// SaveConfig is a test helper that saves a configuration using ConfigurationManager.
// It handles manager initialization and provides a clean API for tests.
// Note: This skips validation to allow tests to save incomplete configs.
//
// The function uses the organization from the config to determine the base directory.
// If the organization is empty, it defaults to "opencenter".
func SaveConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	
	ctx := context.Background()
	manager, err := config.NewConfigurationManager()
	if err != nil {
		t.Fatalf("failed to create configuration manager: %v", err)
	}
	
	// Ensure organization is set
	if cfg.OpenCenter.Meta.Organization == "" {
		cfg.OpenCenter.Meta.Organization = "opencenter"
	}
	
	// Save without validation for test flexibility
	// Tests can explicitly call ValidateConfig if they need validation
	if err := manager.SaveWithoutValidation(ctx, &cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
}

// SaveConfigWithPathResolver is a test helper that saves a configuration using a custom PathResolver.
// This is useful for tests that use temporary directories.
// Note: This skips validation to allow tests to save incomplete configs.
func SaveConfigWithPathResolver(t *testing.T, cfg config.Config, pathResolver *paths.PathResolver) {
	t.Helper()
	
	ctx := context.Background()
	
	// Create dependencies
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validation.NewValidationEngine()
	cache := config.NewConfigCache()
	loader := config.NewConfigIOHandler(fileSystem)
	
	// Create manager with custom path resolver
	manager := config.NewConfigurationManagerWithDeps(
		loader,
		validator,
		cache,
		pathResolver,
		fileSystem,
	)
	
	// Ensure organization is set
	if cfg.OpenCenter.Meta.Organization == "" {
		cfg.OpenCenter.Meta.Organization = "opencenter"
	}
	
	// Save without validation for test flexibility
	if err := manager.SaveWithoutValidation(ctx, &cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
}

// SaveConfigWithValidation is a test helper that saves and validates a configuration.
// Use this when you want to test validation behavior.
func SaveConfigWithValidation(t *testing.T, cfg config.Config) error {
	t.Helper()
	
	ctx := context.Background()
	manager, err := config.NewConfigurationManager()
	if err != nil {
		t.Fatalf("failed to create configuration manager: %v", err)
	}
	
	return manager.Save(ctx, &cfg)
}

// LoadConfig is a test helper that loads a configuration using ConfigurationManager.
// It handles manager initialization and provides a clean API for tests.
func LoadConfig(t *testing.T, name string) config.Config {
	t.Helper()
	
	ctx := context.Background()
	manager, err := config.NewConfigurationManager()
	if err != nil {
		t.Fatalf("failed to create configuration manager: %v", err)
	}
	
	cfg, err := manager.Load(ctx, name)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	
	if cfg == nil {
		t.Fatalf("configuration not found: %s", name)
	}
	
	return *cfg
}

// ValidateConfig is a test helper that validates a configuration using ConfigurationManager.
// It returns an error if validation fails.
func ValidateConfig(t *testing.T, cfg config.Config) error {
	t.Helper()
	
	ctx := context.Background()
	manager, err := config.NewConfigurationManager()
	if err != nil {
		t.Fatalf("failed to create configuration manager: %v", err)
	}
	
	return manager.Validate(ctx, &cfg)
}
