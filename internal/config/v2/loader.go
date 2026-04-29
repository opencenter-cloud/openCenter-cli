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
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// YAMLTypeErrors wraps individual YAML type errors so they can be split
// into separate validation messages by the formatter.
type YAMLTypeErrors struct {
	Errors []string
}

func (e *YAMLTypeErrors) Error() string {
	if len(e.Errors) == 0 {
		return "YAML type errors (0)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "YAML type errors (%d):", len(e.Errors))
	for _, msg := range e.Errors {
		b.WriteString("\n  - ")
		b.WriteString(strings.TrimSpace(msg))
	}
	return b.String()
}

// ConfigLoader implements the v2 configuration loading pipeline.
// Pipeline: Load YAML → Normalize → Resolve References → Apply Defaults → Validate → Freeze
// Requirements: 6.3, 15.1
type ConfigLoader struct {
	hydrator   defaults.Hydrator
	validator  Validator
	fileSystem fs.FileSystem
}

// NewConfigLoader creates a new v2 configuration loader with all pipeline components.
func NewConfigLoader(registry defaults.Registry) *ConfigLoader {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	return NewConfigLoaderWithFileSystem(registry, fileSystem)
}

// NewConfigLoaderWithFileSystem creates a new v2 configuration loader with an
// explicit filesystem. This keeps higher-level managers testable while using
// the same native v2 pipeline for every load/save/validate operation.
func NewConfigLoaderWithFileSystem(registry defaults.Registry, fileSystem fs.FileSystem) *ConfigLoader {
	return &ConfigLoader{
		hydrator:   defaults.NewHydrator(registry),
		validator:  NewValidator(),
		fileSystem: fileSystem,
	}
}

// LoadFromFile loads a v2 configuration from a file path.
// It executes the complete pipeline: Load → Normalize → Resolve → Hydrate → Validate → Freeze.
// Requirements: 6.3, 15.1
func (cl *ConfigLoader) LoadFromFile(filePath string) (*Config, error) {
	// Stage 1: Load YAML
	data, err := cl.fileSystem.ReadFile(filePath)
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

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	// Parse YAML with detailed error reporting
	if err := decoder.Decode(&cfg); err != nil {
		// Check if it's a YAML syntax error with line/column info
		if yamlErr, ok := err.(*yaml.TypeError); ok {
			return nil, &YAMLTypeErrors{Errors: yamlErr.Errors}
		}
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Verify schema version
	if cfg.SchemaVersion != "2.0" {
		if cfg.SchemaVersion == "" {
			return nil, fmt.Errorf("invalid schema version: expected '2.0'")
		}
		return nil, fmt.Errorf("invalid schema version: expected '2.0', got '%s'", cfg.SchemaVersion)
	}

	return &cfg, nil
}

// normalize performs type coercion and field canonicalization.
// Requirements: 16.6
func (cl *ConfigLoader) normalize(cfg *Config) error {
	cfg.OpenCenter.Infrastructure.Provider = canonicalInfrastructureProvider(cfg.OpenCenter.Infrastructure.Provider)
	if cfg.Deployment.ClusterAPI != nil {
		cfg.Deployment.ClusterAPI.Providers.Infrastructure = canonicalInfrastructureProvider(cfg.Deployment.ClusterAPI.Providers.Infrastructure)
	}

	if cfg.OpenCenter.Infrastructure.SSH.Username == "" && cfg.OpenCenter.Infrastructure.SSH.User != "" {
		cfg.OpenCenter.Infrastructure.SSH.Username = cfg.OpenCenter.Infrastructure.SSH.User
	}

	// Handle empty string vs null for optional fields
	// Ensure consistent representation

	// Set default values for optional fields if not specified
	if cfg.OpenCenter.GitOps.Repository.Branch == "" {
		cfg.OpenCenter.GitOps.Repository.Branch = "main"
	}

	if cfg.OpenCenter.GitOps.Flux.Interval == "" {
		cfg.OpenCenter.GitOps.Flux.Interval = "5m"
	}

	if len(cfg.OpenCenter.ManagedServices) == 0 && len(cfg.OpenCenter.LegacyManaged) > 0 {
		cfg.OpenCenter.ManagedServices = cfg.OpenCenter.LegacyManaged
	}

	if cfg.Secrets.SOPSConfig.AgeKeyFile == "" && cfg.Secrets.SopsAgeKeyFile != "" {
		cfg.Secrets.SOPSConfig.AgeKeyFile = cfg.Secrets.SopsAgeKeyFile
	}

	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
		if openstack.Domain == "" && openstack.DomainName != "" {
			openstack.Domain = openstack.DomainName
		}
		if openstack.Networking != nil {
			if openstack.FloatingIPPool == "" {
				openstack.FloatingIPPool = openstack.Networking.FloatingIPPool
			}
			if openstack.FloatingNetworkID == "" {
				openstack.FloatingNetworkID = openstack.Networking.FloatingNetworkID
			}
			if openstack.NetworkID == "" {
				openstack.NetworkID = openstack.Networking.NetworkID
			}
			if openstack.RouterExternalNetworkID == "" {
				openstack.RouterExternalNetworkID = openstack.Networking.RouterExternalNetworkID
			}
			if openstack.SubnetID == "" {
				openstack.SubnetID = openstack.Networking.SubnetID
			}
			if openstack.DNSZoneName == "" {
				openstack.DNSZoneName = openstack.Networking.Designate.DNSZoneName
			}
		}
	}

	// Normalize deployment settings
	if cfg.Deployment.AutoDeploy {
		// Ensure deployment configuration is present
		// This is a placeholder for future deployment config normalization
	}

	return nil
}

// resolveReferences resolves ${ref:path}, ${env:VAR}, and ${file:path} references in the configuration.
// Requirements: 6.3, 4.2.9
func (cl *ConfigLoader) resolveReferences(cfg *Config) error {
	resolver := NewReferenceResolver()
	return resolver.Resolve(cfg)
}

// applyDefaults applies provider-region defaults without overwriting explicit values.
// Requirements: 15.1, 15.2, 15.3
func (cl *ConfigLoader) applyDefaults(cfg *Config) error {
	provider := cfg.OpenCenter.Infrastructure.Provider
	region := cfg.OpenCenter.Meta.Region

	// Providers without region-based defaults in the registry (kind, vmware,
	// baremetal) skip registry-based hydration. However, provider-specific
	// behavior defaults (e.g. the Kind block) must still be applied when
	// missing so that configs created outside `cluster init` or manually
	// edited to change the provider are valid.
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "kind":
		if cfg.OpenCenter.Infrastructure.Kind == nil {
			applyProviderBehaviorDefaults(cfg)
		}
		return nil
	case "vmware", "baremetal":
		return nil
	}

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
	data = ensureDocumentStart(data)

	// Write to file with secure permissions using atomic write
	if err := cl.fileSystem.WriteFileAtomic(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}

func ensureDocumentStart(data []byte) []byte {
	trimmed := strings.TrimLeft(string(data), "\ufeff \t\r\n")
	if strings.HasPrefix(trimmed, "---\n") || strings.HasPrefix(trimmed, "---\r\n") {
		return data
	}
	return append([]byte("---\n"), data...)
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
