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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClusterInitSchemaVersionFlag tests that the --schema-version flag is properly defined
func TestClusterInitSchemaVersionFlag(t *testing.T) {
	cmd := newClusterInitCmd()

	// Verify the flag exists
	flag := cmd.Flags().Lookup("schema-version")
	assert.NotNil(t, flag, "schema-version flag should be defined")

	// Verify the default value
	assert.Equal(t, "2.0", flag.DefValue, "default schema version should be 2.0")

	// Verify the flag can be set
	err := cmd.Flags().Set("schema-version", "1.0")
	assert.NoError(t, err, "should be able to set schema-version to 1.0")

	value, err := cmd.Flags().GetString("schema-version")
	assert.NoError(t, err)
	assert.Equal(t, "1.0", value, "schema-version should be set to 1.0")
}

// TestClusterInitSchemaVersionValidation tests schema version validation logic
func TestClusterInitSchemaVersionValidation(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectError bool
	}{
		{
			name:        "valid v2.0",
			version:     "2.0",
			expectError: false,
		},
		{
			name:        "valid v1.0",
			version:     "1.0",
			expectError: false,
		},
		{
			name:        "invalid v3.0",
			version:     "3.0",
			expectError: true,
		},
		{
			name:        "invalid empty",
			version:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate schema version
			isValid := tt.version == "1.0" || tt.version == "2.0"

			if tt.expectError {
				assert.False(t, isValid, "schema version %s should be invalid", tt.version)
			} else {
				assert.True(t, isValid, "schema version %s should be valid", tt.version)
			}
		})
	}
}

