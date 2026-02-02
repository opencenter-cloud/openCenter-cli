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

// Schema version constants
const (
	SchemaVersion1_0_0   = "1.0.0"
	SchemaVersion1_1_0   = "v1.1.0"
	SchemaVersion1_2_0   = "v1.2.0"
	SchemaVersion2_0_0   = "v2.0.0"
	CurrentSchemaVersion = SchemaVersion2_0_0
)

// DetectSchemaVersion attempts to detect the schema version of a configuration.
// If no version is specified, it attempts to infer based on structure.
// This is used to identify v1 configurations so they can be rejected in v2.0.0.
func DetectSchemaVersion(config Config) string {
	// If schema version is explicitly set, use it
	if config.SchemaVersion != "" {
		return config.SchemaVersion
	}

	// Attempt to infer version based on structure
	// v1.1.0+ has Metadata field
	if !config.Metadata.CreatedAt.IsZero() || len(config.Metadata.Tags) > 0 || len(config.Metadata.Annotations) > 0 {
		// Could be v1.1.0, v1.2.0, or v2.0.0
		// Check for v2.0.0 specific features
		if config.Metadata.Tags != nil {
			if _, hasSchemaTag := config.Metadata.Tags["schema_version"]; hasSchemaTag {
				return SchemaVersion2_0_0
			}
		}
		// Default to v1.1.0 if metadata exists but no v2.0.0 markers
		return SchemaVersion1_1_0
	}

	// No metadata field, must be 1.0.0
	return SchemaVersion1_0_0
}
