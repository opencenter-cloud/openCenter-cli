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

// Config represents the root configuration for a cluster.
// This is a placeholder that will be moved from internal/config/config.go
// during Phase 2 of the architectural refactoring.
//
// The Config struct contains:
//   - SchemaVersion: Version of the configuration schema (1.0, 2.0)
//   - Metadata: Configuration metadata (created_at, updated_at, tags, annotations)
//   - OpenCenter: Core cluster configuration (infrastructure, services, gitops)
//   - OpenTofu: Infrastructure as code configuration
//   - Secrets: Sensitive data (keys, passwords, credentials)
//   - Deployment: Deployment settings
//   - Overrides: Custom overrides for advanced use cases
//
// Note: This type definition will be populated with the actual Config struct
// from internal/config/config.go in Epic 2.4.1 (Phase 2: Migration).
// For now, it serves as a placeholder to establish the package structure.
//
// TODO: Move Config struct from internal/config/config.go in Phase 2, Epic 2.4.1
type Config struct {
	// Placeholder fields - will be replaced with actual Config struct
	// from internal/config/config.go during migration
}

// ConfigMetadata holds metadata about the configuration file.
// This will be moved from internal/config/metadata.go during Phase 2.
//
// TODO: Move ConfigMetadata from internal/config/metadata.go in Phase 2, Epic 2.4.1
type ConfigMetadata struct {
	// Placeholder - will be populated during migration
}

// SimplifiedOpenCenter represents the opencenter section of the configuration.
// This will be moved from internal/config/types_opencenter.go during Phase 2.
//
// TODO: Move from internal/config/types_opencenter.go in Phase 2, Epic 2.4.1
type SimplifiedOpenCenter struct {
	// Placeholder - will be populated during migration
}

// SimplifiedOpenTofu represents the opentofu section of the configuration.
// This will be moved from internal/config/types_opentofu.go during Phase 2.
//
// TODO: Move from internal/config/types_opentofu.go in Phase 2, Epic 2.4.1
type SimplifiedOpenTofu struct {
	// Placeholder - will be populated during migration
}

// Secrets holds all sensitive configuration data.
// This will be moved from internal/config/types_secrets.go during Phase 2.
//
// TODO: Move from internal/config/types_secrets.go in Phase 2, Epic 2.4.1
type Secrets struct {
	// Placeholder - will be populated during migration
}

// Deployment holds deployment-related configuration.
// This will be moved from internal/config/types_deployment.go during Phase 2.
//
// TODO: Move from internal/config/types_deployment.go in Phase 2, Epic 2.4.1
type Deployment struct {
	// Placeholder - will be populated during migration
}

// LegacyNetworking holds old networking fields for backward compatibility.
// This will be moved from internal/config/config.go during Phase 2.
//
// TODO: Move from internal/config/config.go in Phase 2, Epic 2.4.1
type LegacyNetworking struct {
	// Placeholder - will be populated during migration
}
