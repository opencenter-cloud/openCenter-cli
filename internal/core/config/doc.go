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

// Package config provides unified configuration management for opencenter-cli.
//
// This package consolidates configuration loading, version handling, and migration
// into a single, well-structured system that supports v2.0.0 and provides clear
// error messages for v1 configurations.
//
// # Architecture
//
// The package is organized into focused modules:
//
//   - manager.go: ConfigManager with strategy pattern for version handling
//   - types.go: Core configuration types (Config, Metadata, etc.)
//   - defaults.go: Default value generation and application
//   - persistence.go: File I/O operations (Load, Save, path resolution)
//   - strategies/: Version-specific loading strategies (v2, legacy)
//   - migration/: Version migration logic
//   - errors.go: Custom error types for configuration issues
//
// # Key Features
//
//   - Auto-detection of configuration version from YAML
//   - Strategy-based loading for version-specific logic
//   - Integrated migration pipeline for version upgrades
//   - Thread-safe caching with invalidation
//   - Single Load() method handles all versions
//   - Organization-aware path resolution
//   - V2-only enforcement with clear upgrade guidance
//
// # Usage
//
// Basic configuration loading:
//
//	manager := config.NewConfigManager()
//	cfg, err := manager.Load("/path/to/config.yaml", config.LoadOptions{
//	    AutoMigrate: true,
//	    Validate: true,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Creating a new configuration with defaults:
//
//	cfg := config.NewDefault("my-cluster")
//	err := manager.Save("/path/to/config.yaml", cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Loading with cache bypass:
//
//	cfg, err := manager.Load(path, config.LoadOptions{
//	    SkipCache: true,
//	})
//
// Invalidating cache after external changes:
//
//	manager.InvalidateCache(path)
//
// Registering custom strategies:
//
//	manager.RegisterStrategy(myCustomStrategy)
//
// # Version Detection and Migration
//
// The ConfigManager automatically detects the configuration version by examining
// the YAML structure. It tries each registered strategy in order until one
// reports it can load the configuration.
//
// In v2.0.0, v1 configurations are rejected with a clear error message:
//
//	cfg, err := manager.Load("old-v1-config.yaml", config.LoadOptions{})
//	// Returns V1ConfigError with upgrade instructions
//
// Users must upgrade to v1.x and run migration before using v2.0.0:
//
//	# Using v1.x
//	opencenter cluster migrate-config my-cluster
//	# Then upgrade to v2.0.0
//
// # Implementation Status
//
// This package provides a unified configuration management system with:
//   - V2 configuration format (current)
//   - V1 configuration format (with auto-migration)
//   - Legacy flat-file format (with auto-migration)
//   - Thread-safe caching
//   - Validation integration
//
// The ConfigManager is fully functional and production-ready, supporting
// all configuration operations including loading, saving, migration, and validation.
//
// # Design Principles
//
//   - Single Responsibility: Each file has one clear purpose
//   - Strategy Pattern: Version-specific logic is isolated
//   - Dependency Inversion: High-level code depends on abstractions
//   - Open/Closed: Open for extension (new versions), closed for modification
//   - Fail Fast: Reject unsupported versions immediately with clear guidance
//
// # Thread Safety
//
// ConfigManager is thread-safe. Multiple goroutines can safely call Load()
// and Save() concurrently. The internal cache is protected by a RWMutex.
//
// # Performance
//
// Target performance characteristics:
//
//   - Config loading: <100ms (50% improvement over legacy)
//   - Cache hit: <1ms
//   - Memory usage: <100MB peak (33% reduction)
//
// # Related Packages
//
//   - internal/core/paths: Path resolution and organization structure
//   - internal/core/validation: Configuration validation
//   - internal/config: Legacy configuration package (being phased out)
//
// # References
//
//   - Design Document: .kiro/specs/v2-breaking-changes/design.md
//   - Requirements: .kiro/specs/v2-breaking-changes/requirements.md
//   - Tasks: .kiro/specs/v2-breaking-changes/tasks.md
package config
