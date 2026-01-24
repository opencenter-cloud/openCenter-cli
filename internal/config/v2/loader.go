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

package v2

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/rackerlabs/opencenter-cli/internal/config/defaults"
)

// ConfigLoader implements the v2 configuration loading pipeline.
// Pipeline: Load YAML → Normalize → Resolve References → Apply Defaults → Validate → Freeze
// Requirements: 6.3, 15.1
type ConfigLoader struct {
	hydrator  defaults.Hydrator
	validator Validator
}

// NewConfigLoader creates a new v2 configuration loader with all pipeline components.
func NewConfigLoader(registry defaults.Registry) *ConfigLoader {
	return &ConfigLoader{
		hydrator:  defaults.NewHydrator(registry),
		validator: NewValidator(),
	}
}

// LoadFromFile loads a v2 configuration from a file path.
// It executes the complete pipeline: Load → Normalize → Resolve → Hydrate → Validate → Freeze.
// Requirements: 6.3, 15.1
func (cl *ConfigLoader) LoadFromFile(filePath string) (*Config, error) {
	// Stage 1: Load YAML
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	return cl.LoadFromBytes(data)
}

// LoadFromBytes loads a v2 configuration from byte data.
// It executes the complete pipeline: Load → Normalize → Resolve → Hydrate → Validate → Freeze.
// Requirements: 6.3, 15.1
func (cl *ConfigLoader) LoadFromBytes(data []byte) (*Config, error) {
	// Stage 1: Load YAML
	cfg, err := cl.parseYAML(data)
	if err != nil {
		return nil, fmt.Errorf("stage 1 (load): %w", err)
	}

	// Stage 2: Normalize
	if err := cl.normalize(cfg); err != nil {
		return nil, fmt.Errorf("stage 2 (normalize): %w", err)
	}

	// Stage 3: Resolve References
	if err := cl.resolveReferences(cfg); err != nil {
		return nil, fmt.Errorf("stage 3 (resolve references): %w", err)
	}

	// Stage 4: Apply Defaults (Hydration)
	if err := cl.applyDefaults(cfg); err != nil {
		return nil, fmt.Errorf("stage 4 (apply defaults): %w", err)
	}

	// Stage 5: Validate
	if err := cl.validate(cfg); err != nil {
		return nil, fmt.Errorf("stage 5 (validate): %w", err)
	}

	// Stage 6: Freeze (mark as immutable)
	cl.freeze(cfg)

	return cfg, nil
}

// parseYAML parses YAML data into a Config struct.
// Requirements: 16.1, 16.7
func (cl *ConfigLoader) parseYAML(data []byte) (*Config, error) {
	var cfg Config

	// Parse YAML with detailed error reporting
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Check if it's a YAML syntax error with line/column info
		if yamlErr, ok := err.(*yaml.TypeError); ok {
			return nil, fmt.Errorf("YAML syntax error: %v", yamlErr.Errors)
		}
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Verify schema version
	if cfg.SchemaVersion != "2.0" {
		return nil, fmt.Errorf("invalid schema version: expected '2.0', got '%s'", cfg.SchemaVersion)
	}

	return &cfg, nil
}

// normalize performs type coercion and field canonicalization.
// Requirements: 16.6
func (cl *ConfigLoader) normalize(cfg *Config) error {
	// Handle empty string vs null for optional fields
	// Ensure consistent representation

	// Set default values for optional fields if not specified
	if cfg.OpenCenter.GitOps.GitBranch == "" {
		cfg.OpenCenter.GitOps.GitBranch = "main"
	}

	if cfg.OpenCenter.GitOps.FluxInterval == "" {
		cfg.OpenCenter.GitOps.FluxInterval = "15m"
	}

	// Normalize deployment settings
	if cfg.Deployment.AutoDeploy {
		// Ensure deployment configuration is present
		// This is a placeholder for future deployment config normalization
	}

	return nil
}

// resolveReferences resolves ${path.to.value} references in the configuration.
// Requirements: 6.3
// Note: Reference resolution is already implemented and tested in phase 4 (task 6.1-6.5).
// For now, we skip it in the v2 loader to avoid coupling issues.
// This will be integrated properly when the resolver is refactored to work with generic structs.
func (cl *ConfigLoader) resolveReferences(cfg *Config) error {
	// TODO: Integrate reference resolver from phase 4
	// The resolver needs to be refactored to work with interface{} instead of *config.Config
	return nil
}

// applyDefaults applies provider-region defaults without overwriting explicit values.
// Requirements: 15.1, 15.2, 15.3
func (cl *ConfigLoader) applyDefaults(cfg *Config) error {
	// Extract provider and region from configuration
	provider := cfg.OpenCenter.Infrastructure.Provider
	region := cfg.OpenCenter.Meta.Region

	// Apply defaults using hydrator
	return cl.hydrator.Hydrate(cfg, provider, region)
}

// validate performs multi-layered validation.
// Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7
func (cl *ConfigLoader) validate(cfg *Config) error {
	return cl.validator.Validate(cfg)
}

// freeze marks the configuration as immutable (ready for use).
func (cl *ConfigLoader) freeze(cfg *Config) {
	// In Go, we don't have built-in immutability, but we can document
	// that the configuration should not be modified after this point.
	// Future enhancement: could use a wrapper type with read-only methods.
}

// GetAppliedDefaults returns the defaults that were applied during hydration.
// Requirements: 15.6, 15.7
func (cl *ConfigLoader) GetAppliedDefaults() map[string]defaults.DefaultSource {
	return cl.hydrator.GetAppliedDefaults()
}

// SaveToFile saves a v2 configuration to a file.
// Requirements: 16.2
func (cl *ConfigLoader) SaveToFile(cfg *Config, filePath string) error {
	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to file with secure permissions
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}

// ExportEffectiveConfig exports the configuration with applied defaults as comments.
// Requirements: 15.7, 15.8
func (cl *ConfigLoader) ExportEffectiveConfig(cfg *Config) ([]byte, error) {
	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Get applied defaults
	appliedDefaults := cl.hydrator.GetAppliedDefaults()

	// Add comments for applied defaults
	// This is a simplified implementation - a full implementation would
	// parse the YAML and insert comments at the appropriate locations
	header := "# Effective Configuration (includes applied defaults)\n"
	header += "# Applied defaults:\n"
	for field, source := range appliedDefaults {
		header += fmt.Sprintf("#   %s: %s\n", field, source)
	}
	header += "\n"

	return append([]byte(header), data...), nil
}
