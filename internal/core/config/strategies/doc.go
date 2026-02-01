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

// Package strategies provides version-specific configuration loading strategies.
//
// The strategies package implements the Strategy pattern for loading different
// versions of opencenter configuration files. Each strategy knows how to:
//   - Detect if it can load a given configuration (CanLoad)
//   - Load and parse the configuration data (Load)
//   - Report its version identifier (Version)
//
// # Supported Strategies
//
// V2Strategy: Loads v2.0 schema configurations
//   - Uses internal/config/v2 package
//   - Executes complete loading pipeline: Load → Normalize → Resolve → Hydrate → Validate
//   - Identified by schema_version: "2.0"
//
// V1Strategy: Loads v1.0 schema configurations
//   - Uses internal/config package
//   - Handles both explicit "1.0" and missing version (backward compatibility)
//   - Applies defaults and resolves references
//
// LegacyStrategy: Loads pre-versioning flat configurations
//   - Converts flat structure to current format
//   - Identified by absence of schema_version and opencenter sections
//   - Provides basic field mapping
//
// # Usage
//
// Strategies are registered with ConfigManager and automatically selected
// based on the configuration content:
//
//	manager := config.NewConfigManager()
//	manager.RegisterStrategy(strategies.NewV2Strategy())
//	manager.RegisterStrategy(strategies.NewV1Strategy())
//	manager.RegisterStrategy(strategies.NewLegacyStrategy())
//
//	config, err := manager.Load(path, config.LoadOptions{})
//
// # Version Detection
//
// Each strategy implements CanLoad() to determine if it can handle a given
// configuration. The ConfigManager tries strategies in registration order
// until one returns true from CanLoad().
//
// Detection logic:
//   - V2: schema_version == "2.0"
//   - V1: schema_version == "1.0" OR schema_version is missing
//   - Legacy: No schema_version AND no opencenter section AND has legacy fields
//
// # Architecture
//
// This package is part of the architectural refactoring (Phase 1, Epic 1.3.2)
// to consolidate configuration loading into a single, extensible system.
// See .kiro/specs/architectural-refactoring/design.md for details.
package strategies
