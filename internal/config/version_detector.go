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
// In v2.0.0, only v2 configurations are supported. V1 configurations are rejected.
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

	// Reject v1 configurations (including missing schema_version which defaults to v1)
	if info.Version == "" || info.Version == "1.0" {
		return nil, fmt.Errorf("v1 configurations are not supported in v2.0.0. Please upgrade to v1.x and run migration before using v2.0.0")
	}

	// Only support v2
	if info.Version == "2.0" {
		info.IsV1 = false
		info.IsV2 = true
		return info, nil
	}

	// Unsupported version
	return nil, fmt.Errorf("unsupported schema version: %s (only 2.0 is supported)", info.Version)
}

// LoadConfigWithVersionDetection loads a configuration file with automatic version detection.
// It routes to the appropriate parser based on the detected schema version.
// Requirements: 13.1, 13.2, 13.3
//
// Deprecated: Use internal/core/config.ConfigManager.Load() which handles version detection automatically.
// This function will be removed in v2.0.0.
// Migration: Replace LoadConfigWithVersionDetection(filePath) with configManager.Load(filePath, LoadOptions{})
func LoadConfigWithVersionDetection(filePath string) (interface{}, *SchemaVersionInfo, error) {
	logDeprecationWarning(
		"config.LoadConfigWithVersionDetection()",
		"internal/core/config.ConfigManager.Load()",
		"v2.0.0",
	)
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
