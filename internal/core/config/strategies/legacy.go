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

package strategies

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

// LegacyStrategy implements loading for legacy flat configuration files.
// These are configurations that predate the schema versioning system and
// use a flat structure without the opencenter/opentofu/secrets hierarchy.
type LegacyStrategy struct{}

// NewLegacyStrategy creates a new Legacy loading strategy.
func NewLegacyStrategy() *LegacyStrategy {
	return &LegacyStrategy{}
}

// CanLoad determines if this strategy can load the given configuration data.
// Legacy configurations are identified by:
// 1. No schema_version field
// 2. No opencenter field (flat structure) OR empty opencenter section
// 3. Presence of legacy top-level fields like cluster_name, provider, etc.
func (s *LegacyStrategy) CanLoad(data []byte) (bool, error) {
	// Parse to check structure
	var structCheck struct {
		SchemaVersion string                 `yaml:"schema_version"`
		OpenCenter    map[string]interface{} `yaml:"opencenter"`
		ClusterName   string                 `yaml:"cluster_name"`
		Provider      string                 `yaml:"provider"`
	}

	if err := yaml.Unmarshal(data, &structCheck); err != nil {
		return false, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Legacy configs have:
	// - No schema_version
	// - No opencenter section OR empty opencenter section
	// - Top-level fields like cluster_name or provider
	hasOpenCenter := structCheck.OpenCenter != nil && len(structCheck.OpenCenter) > 0
	hasLegacyFields := structCheck.ClusterName != "" || structCheck.Provider != ""

	isLegacy := structCheck.SchemaVersion == "" &&
		!hasOpenCenter &&
		hasLegacyFields

	return isLegacy, nil
}

// Load loads and parses legacy configuration data.
// It converts the flat structure to the current Config format.
func (s *LegacyStrategy) Load(data []byte, clusterName string) (*config.Config, error) {
	// Parse legacy structure
	var legacyConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &legacyConfig); err != nil {
		return nil, fmt.Errorf("failed to parse legacy YAML configuration: %w", err)
	}

	// Start with default configuration
	cfg := config.NewDefault(clusterName)

	// Map legacy fields to new structure
	// This is a simplified implementation - a full implementation would
	// handle all legacy field mappings
	if provider, ok := legacyConfig["provider"].(string); ok {
		cfg.OpenCenter.Infrastructure.Provider = provider
	}

	if region, ok := legacyConfig["region"].(string); ok {
		cfg.OpenCenter.Meta.Region = region
	}

	if env, ok := legacyConfig["environment"].(string); ok {
		cfg.OpenCenter.Meta.Env = env
	}

	// Note: Full legacy migration logic would be implemented here
	// For now, we provide basic field mapping and rely on the migration
	// system for complete conversion

	return &cfg, nil
}

// Version returns the version identifier for this strategy.
func (s *LegacyStrategy) Version() string {
	return "legacy"
}
