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

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SchemaVersionInfo contains information about the detected schema version.
type SchemaVersionInfo struct {
	Version string
	IsV1    bool
	IsV2    bool
}

// DetectSchemaVersionFromFile detects the schema version from a configuration file.
// Requirements: 13.1, 13.2, 13.3
func DetectSchemaVersionFromFile(filePath string) (*SchemaVersionInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return DetectSchemaVersionFromBytes(data)
}

// DetectSchemaVersionFromBytes detects the schema version from configuration data.
// Requirements: 13.1, 13.2, 13.3
func DetectSchemaVersionFromBytes(data []byte) (*SchemaVersionInfo, error) {
	// Parse just the schema_version field
	var versionCheck struct {
		SchemaVersion string `yaml:"schema_version"`
	}

	if err := yaml.Unmarshal(data, &versionCheck); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	info := &SchemaVersionInfo{
		Version: versionCheck.SchemaVersion,
	}

	// Default to v1 when schema_version is missing (backward compatibility)
	// Requirement: 13.3
	if info.Version == "" {
		info.Version = "1.0"
		info.IsV1 = true
		info.IsV2 = false
		return info, nil
	}

	// Determine version type
	switch info.Version {
	case "1.0":
		info.IsV1 = true
		info.IsV2 = false
	case "2.0":
		info.IsV1 = false
		info.IsV2 = true
	default:
		return nil, fmt.Errorf("unsupported schema version: %s (supported: 1.0, 2.0)", info.Version)
	}

	return info, nil
}

// LoadConfigWithVersionDetection loads a configuration file with automatic version detection.
// It routes to the appropriate parser based on the detected schema version.
// Requirements: 13.1, 13.2, 13.3
func LoadConfigWithVersionDetection(filePath string) (interface{}, *SchemaVersionInfo, error) {
	// Detect schema version
	versionInfo, err := DetectSchemaVersionFromFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to detect schema version: %w", err)
	}

	// Read file data
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Route to appropriate parser based on version
	if versionInfo.IsV1 {
		// Parse as v1 configuration
		var v1Config Config
		if err := yaml.Unmarshal(data, &v1Config); err != nil {
			return nil, nil, fmt.Errorf("failed to parse v1 configuration: %w", err)
		}
		return &v1Config, versionInfo, nil
	}

	if versionInfo.IsV2 {
		// Parse as v2 configuration
		// Note: This would use the v2 package when fully implemented
		return nil, nil, fmt.Errorf("v2 configuration parsing not yet implemented")
	}

	return nil, nil, fmt.Errorf("unknown schema version: %s", versionInfo.Version)
}
