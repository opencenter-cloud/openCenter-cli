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
	"github.com/rackerlabs/opencenter-cli/internal/config/defaults"
	v2 "github.com/rackerlabs/opencenter-cli/internal/config/v2"
)

// V2Strategy implements loading for v2.0 configuration schema.
// It uses the v2 package's ConfigLoader for the complete loading pipeline.
type V2Strategy struct {
	loader *v2.ConfigLoader
}

// NewV2Strategy creates a new V2 loading strategy.
func NewV2Strategy() *V2Strategy {
	// Create default registry for v2 loader
	registry := defaults.NewRegistry()

	return &V2Strategy{
		loader: v2.NewConfigLoader(registry),
	}
}

// CanLoad determines if this strategy can load the given configuration data.
// It checks for schema_version: "2.0" in the YAML.
func (s *V2Strategy) CanLoad(data []byte) (bool, error) {
	// Parse just the schema_version field
	var versionCheck struct {
		SchemaVersion string `yaml:"schema_version"`
	}

	if err := yaml.Unmarshal(data, &versionCheck); err != nil {
		return false, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Check if this is a v2 configuration
	return versionCheck.SchemaVersion == "2.0", nil
}

// Load loads and parses v2 configuration data.
// It uses the v2.ConfigLoader to execute the complete loading pipeline:
// Load → Normalize → Resolve → Hydrate → Validate → Freeze
func (s *V2Strategy) Load(data []byte, clusterName string) (*config.Config, error) {
	// Load v2 configuration using v2 loader
	v2Config, err := s.loader.LoadFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to load v2 configuration: %w", err)
	}

	// Convert v2.Config to config.Config
	// Note: This conversion will be implemented when v2 is fully integrated
	// For now, return an error indicating v2 is not yet fully supported
	_ = v2Config
	return nil, fmt.Errorf("v2 configuration loading is not yet fully integrated (conversion from v2.Config to config.Config pending)")
}

// Version returns the version identifier for this strategy.
func (s *V2Strategy) Version() string {
	return "2.0"
}
