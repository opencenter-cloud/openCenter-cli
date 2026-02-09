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

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
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
	// Initialize FileSystem for reading
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	data, err := fileSystem.ReadFile(filePath)
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
