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

package migration

import (
	"fmt"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

// LegacyToV1Migration creates a migration from legacy flat structure to v1.0 schema.
// This migration handles the transition from the old flat configuration format
// to the structured v1.0 schema with opencenter, opentofu, and secrets sections.
func LegacyToV1Migration() *Migration {
	return &Migration{
		From:        "legacy",
		To:          "1.0",
		Migrate:     migrateLegacyToV1,
		Description: "Migrate legacy flat configuration to v1.0 structured schema",
	}
}

// migrateLegacyToV1 performs the actual migration from legacy to v1.0.
// It transforms the flat structure into the hierarchical v1.0 format.
func migrateLegacyToV1(cfg *config.Config) (*config.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	// Create a new v1.0 configuration with defaults
	migrated := config.NewDefault(cfg.OpenCenter.Meta.Name)

	// Set schema version
	migrated.SchemaVersion = "1.0"

	// Migrate metadata
	now := time.Now()
	migrated.Metadata = config.ConfigMetadata{
		CreatedAt: now,
		UpdatedAt: now,
		Tags:      make(map[string]string),
	}

	// Copy tags if they exist in the original
	if len(cfg.Metadata.Tags) > 0 {
		migrated.Metadata.Tags = cfg.Metadata.Tags
	}

	// Migrate cluster metadata
	if err := migrateLegacyMeta(cfg, &migrated); err != nil {
		return nil, fmt.Errorf("failed to migrate cluster metadata: %w", err)
	}

	// Migrate infrastructure configuration
	if err := migrateLegacyInfrastructure(cfg, &migrated); err != nil {
		return nil, fmt.Errorf("failed to migrate infrastructure: %w", err)
	}

	// Migrate Kubernetes configuration
	if err := migrateLegacyKubernetes(cfg, &migrated); err != nil {
		return nil, fmt.Errorf("failed to migrate Kubernetes configuration: %w", err)
	}

	// Migrate services
	if err := migrateLegacyServices(cfg, &migrated); err != nil {
		return nil, fmt.Errorf("failed to migrate services: %w", err)
	}

	// Migrate GitOps configuration
	if err := migrateLegacyGitOps(cfg, &migrated); err != nil {
		return nil, fmt.Errorf("failed to migrate GitOps configuration: %w", err)
	}

	// Migrate secrets
	if err := migrateLegacySecrets(cfg, &migrated); err != nil {
		return nil, fmt.Errorf("failed to migrate secrets: %w", err)
	}

	// Migrate OpenTofu configuration
	if err := migrateLegacyOpenTofu(cfg, &migrated); err != nil {
		return nil, fmt.Errorf("failed to migrate OpenTofu configuration: %w", err)
	}

	return &migrated, nil
}

// migrateLegacyMeta migrates cluster metadata from legacy to v1.0.
func migrateLegacyMeta(src, dst *config.Config) error {
	// Copy basic metadata
	if src.OpenCenter.Meta.Name != "" {
		dst.OpenCenter.Meta.Name = src.OpenCenter.Meta.Name
	}

	if src.OpenCenter.Meta.Env != "" {
		dst.OpenCenter.Meta.Env = src.OpenCenter.Meta.Env
	}

	if src.OpenCenter.Meta.Region != "" {
		dst.OpenCenter.Meta.Region = src.OpenCenter.Meta.Region
	}

	if src.OpenCenter.Meta.Organization != "" {
		dst.OpenCenter.Meta.Organization = src.OpenCenter.Meta.Organization
	}

	return nil
}

// migrateLegacyInfrastructure migrates infrastructure configuration from legacy to v1.0.
func migrateLegacyInfrastructure(src, dst *config.Config) error {
	// Copy provider
	if src.OpenCenter.Infrastructure.Provider != "" {
		dst.OpenCenter.Infrastructure.Provider = src.OpenCenter.Infrastructure.Provider
	}

	// Copy infrastructure configuration
	dst.OpenCenter.Infrastructure = src.OpenCenter.Infrastructure

	return nil
}

// migrateLegacyKubernetes migrates Kubernetes configuration from legacy to v1.0.
func migrateLegacyKubernetes(src, dst *config.Config) error {
	// Copy Kubernetes configuration
	dst.OpenCenter.Cluster.Kubernetes = src.OpenCenter.Cluster.Kubernetes

	return nil
}

// migrateLegacyServices migrates services configuration from legacy to v1.0.
func migrateLegacyServices(src, dst *config.Config) error {
	// Copy services configuration
	// In legacy format, services might be in a different structure
	// For now, we assume they're already in the correct format
	if len(src.OpenCenter.Services) > 0 {
		dst.OpenCenter.Services = src.OpenCenter.Services
	}

	return nil
}

// migrateLegacyGitOps migrates GitOps configuration from legacy to v1.0.
func migrateLegacyGitOps(src, dst *config.Config) error {
	// Copy GitOps configuration
	dst.OpenCenter.GitOps = src.OpenCenter.GitOps

	return nil
}

// migrateLegacySecrets migrates secrets configuration from legacy to v1.0.
func migrateLegacySecrets(src, dst *config.Config) error {
	// Copy secrets configuration
	dst.Secrets = src.Secrets

	return nil
}

// migrateLegacyOpenTofu migrates OpenTofu configuration from legacy to v1.0.
func migrateLegacyOpenTofu(src, dst *config.Config) error {
	// Copy OpenTofu configuration
	dst.OpenTofu = src.OpenTofu

	return nil
}
