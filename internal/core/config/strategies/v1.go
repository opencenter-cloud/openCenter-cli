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

// V1Strategy implements loading for v1.0 configuration schema.
// It uses the existing config package's loading logic.
type V1Strategy struct{}

// NewV1Strategy creates a new V1 loading strategy.
func NewV1Strategy() *V1Strategy {
	return &V1Strategy{}
}

// CanLoad determines if this strategy can load the given configuration data.
// It checks for schema_version: "1.0" or missing schema_version (backward compatibility).
func (s *V1Strategy) CanLoad(data []byte) (bool, error) {
	// Parse just the schema_version field
	var versionCheck struct {
		SchemaVersion string `yaml:"schema_version"`
	}

	if err := yaml.Unmarshal(data, &versionCheck); err != nil {
		return false, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// V1 handles both explicit "1.0" and missing version (backward compatibility)
	return versionCheck.SchemaVersion == "1.0" || versionCheck.SchemaVersion == "", nil
}

// Load loads and parses v1 configuration data.
// It uses the existing config.defaultConfig and YAML unmarshaling logic.
func (s *V1Strategy) Load(data []byte, clusterName string) (*config.Config, error) {
	// Start with default configuration
	cfg := config.NewDefault(clusterName)

	// Unmarshal YAML data onto the default configuration
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse v1 YAML configuration: %w", err)
	}

	// Resolve references after parsing
	resolver := config.NewReferenceResolver()
	if err := resolver.Resolve(&cfg); err != nil {
		return nil, fmt.Errorf("failed to resolve configuration references: %w", err)
	}

	return &cfg, nil
}

// Version returns the version identifier for this strategy.
func (s *V1Strategy) Version() string {
	return "1.0"
}
