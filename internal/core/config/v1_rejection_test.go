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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigManager_RejectsV1ConfigWithExplicitVersion tests that v1 configs with explicit version are rejected
func TestConfigManager_RejectsV1ConfigWithExplicitVersion(t *testing.T) {
	manager := NewConfigManager()

	// Create v1 config with explicit version
	v1Config := `schema_version: "1.0"
opencenter:
  meta:
    cluster_name: test-cluster
    organization: test-org
`

	// Write to temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(v1Config), 0600)
	require.NoError(t, err)

	// Attempt to load
	_, err = manager.Load(configPath, LoadOptions{})

	// Should reject with V1ConfigError
	require.Error(t, err)
	var v1Err *V1ConfigError
	assert.True(t, errors.As(err, &v1Err), "error should be V1ConfigError")
	assert.Contains(t, err.Error(), "v1 configurations are not supported")
	assert.Contains(t, err.Error(), "Install opencenter v1.x")
	assert.Contains(t, err.Error(), configPath, "error should include file path")
}

// TestConfigManager_RejectsV1ConfigWithMissingVersion tests that v1 configs without version are rejected
func TestConfigManager_RejectsV1ConfigWithMissingVersion(t *testing.T) {
	manager := NewConfigManager()

	// Create v1 config without schema_version (defaults to v1)
	v1Config := `opencenter:
  meta:
    cluster_name: test-cluster
    organization: test-org
`

	// Write to temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(v1Config), 0600)
	require.NoError(t, err)

	// Attempt to load
	_, err = manager.Load(configPath, LoadOptions{})

	// Should reject with V1ConfigError
	require.Error(t, err)
	var v1Err *V1ConfigError
	assert.True(t, errors.As(err, &v1Err), "error should be V1ConfigError")
	assert.Contains(t, err.Error(), "v1 configurations are not supported")
}

// TestConfigManager_AcceptsV2Config tests that v2 configs are accepted
func TestConfigManager_AcceptsV2Config(t *testing.T) {
	manager := NewConfigManager()

	// Create v2 config
	v2Config := `schema_version: "2.0"
opencenter:
  meta:
    cluster_name: test-cluster
    organization: test-org
`

	// Write to temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(v2Config), 0600)
	require.NoError(t, err)

	// Note: This will fail because we don't have v2 strategy registered yet
	// But it should NOT fail with V1ConfigError
	_, err = manager.Load(configPath, LoadOptions{})

	// Should not be a V1ConfigError
	if err != nil {
		var v1Err *V1ConfigError
		assert.False(t, errors.As(err, &v1Err), "error should not be V1ConfigError for v2 config")
	}
}

// TestV1ConfigError_ErrorMessage tests the error message format
func TestV1ConfigError_ErrorMessage(t *testing.T) {
	err := NewV1ConfigError("/path/to/config.yaml")

	errMsg := err.Error()
	assert.Contains(t, errMsg, "v1 configurations are not supported")
	assert.Contains(t, errMsg, "/path/to/config.yaml")
	assert.Contains(t, errMsg, "Install opencenter v1.x")
	assert.Contains(t, errMsg, "opencenter cluster migrate-config")
	assert.Contains(t, errMsg, "https://docs.opencenter.io/migration/v1-to-v2")
}

// TestV1ConfigError_Is tests error matching
func TestV1ConfigError_Is(t *testing.T) {
	err1 := NewV1ConfigError("/path/to/config.yaml")
	err2 := &V1ConfigError{}

	assert.True(t, errors.Is(err1, err2))
	assert.False(t, errors.Is(err1, errors.New("other error")))
}

// TestIsV1Config tests the v1 detection helper
func TestIsV1Config(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected bool
	}{
		{
			name: "explicit v1 version",
			yaml: `schema_version: "1.0"
opencenter:
  meta:
    cluster_name: test
`,
			expected: true,
		},
		{
			name: "missing version (defaults to v1)",
			yaml: `opencenter:
  meta:
    cluster_name: test
`,
			expected: true,
		},
		{
			name: "v2 version",
			yaml: `schema_version: "2.0"
opencenter:
  meta:
    cluster_name: test
`,
			expected: false,
		},
		{
			name: "invalid yaml",
			yaml: `invalid: [yaml: structure`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isV1Config([]byte(tt.yaml))
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConfigManager_V1RejectionWithCache tests that v1 rejection works even with caching
func TestConfigManager_V1RejectionWithCache(t *testing.T) {
	manager := NewConfigManager()

	// Create v1 config
	v1Config := `schema_version: "1.0"
opencenter:
  meta:
    cluster_name: test-cluster
`

	// Write to temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(v1Config), 0600)
	require.NoError(t, err)

	// First load attempt
	_, err1 := manager.Load(configPath, LoadOptions{})
	require.Error(t, err1)

	// Second load attempt (should not use cache for errors)
	_, err2 := manager.Load(configPath, LoadOptions{})
	require.Error(t, err2)

	// Both should be V1ConfigError
	var v1Err1, v1Err2 *V1ConfigError
	assert.True(t, errors.As(err1, &v1Err1))
	assert.True(t, errors.As(err2, &v1Err2))
}

// TestConfigManager_V1RejectionErrorDetails tests that error provides helpful details
func TestConfigManager_V1RejectionErrorDetails(t *testing.T) {
	manager := NewConfigManager()

	v1Config := `schema_version: "1.0"
opencenter:
  meta:
    cluster_name: test-cluster
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(v1Config), 0600)
	require.NoError(t, err)

	_, err = manager.Load(configPath, LoadOptions{})
	require.Error(t, err)

	errMsg := err.Error()

	// Check for key information in error message
	requiredStrings := []string{
		"v1 configurations are not supported",
		"v2.0.0",
		"Install opencenter v1.x",
		"opencenter cluster migrate-config",
		"https://docs.opencenter.io/migration/v1-to-v2",
		configPath,
	}

	for _, required := range requiredStrings {
		assert.True(t, strings.Contains(errMsg, required),
			"error message should contain %q, got: %s", required, errMsg)
	}
}
