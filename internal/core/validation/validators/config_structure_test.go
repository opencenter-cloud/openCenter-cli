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

package validators

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigStructureValidator_Name(t *testing.T) {
	validator := NewConfigStructureValidator()
	assert.Equal(t, "config-structure", validator.Name())
}

func TestConfigStructureValidator_ValidV2Structure(t *testing.T) {
	validator := NewConfigStructureValidator()
	ctx := context.Background()

	// Valid v2 structure
	config := map[string]interface{}{
		"schema_version": "2.0",
		"opencenter": map[string]interface{}{
			"infrastructure": map[string]interface{}{
				"networking": map[string]interface{}{
					"vrrp_ip": "10.0.0.1",
				},
				"compute": map[string]interface{}{
					"flavor_control_plane": "m1.large",
					"flavor_worker":        "m1.medium",
				},
				"storage": map[string]interface{}{
					"type": "ceph",
				},
			},
		},
	}

	result, err := validator.Validate(ctx, config)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestConfigStructureValidator_RejectsV1NetworkingLocation(t *testing.T) {
	validator := NewConfigStructureValidator()
	ctx := context.Background()

	// v1 structure with networking under cluster
	config := map[string]interface{}{
		"schema_version": "1.0",
		"opencenter": map[string]interface{}{
			"cluster": map[string]interface{}{
				"networking": map[string]interface{}{
					"vrrp_ip": "10.0.0.1",
				},
			},
		},
	}

	result, err := validator.Validate(ctx, config)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)

	// Check error message
	assert.Contains(t, result.Errors[0].Message, "opencenter.cluster.networking.vrrp_ip")
	assert.Contains(t, result.Errors[0].Suggestions[1], "opencenter.infrastructure.networking.vrrp_ip")
	assert.Contains(t, result.Errors[0].Suggestions[2], "migrate-config")
}

func TestConfigStructureValidator_RejectsV1KubernetesFlavorFields(t *testing.T) {
	validator := NewConfigStructureValidator()
	ctx := context.Background()

	// v1 structure with flavor fields under kubernetes
	config := map[string]interface{}{
		"schema_version": "1.0",
		"opencenter": map[string]interface{}{
			"cluster": map[string]interface{}{
				"kubernetes": map[string]interface{}{
					"flavor_control_plane": "m1.large",
					"flavor_worker":        "m1.medium",
					"flavor_etcd":          "m1.small",
				},
			},
		},
	}

	result, err := validator.Validate(ctx, config)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 3) // Three flavor fields

	// Check that all three fields are reported
	errorMessages := make([]string, len(result.Errors))
	for i, err := range result.Errors {
		errorMessages[i] = err.Message
	}

	assert.Contains(t, errorMessages[0], "flavor_control_plane")
	assert.Contains(t, errorMessages[1], "flavor_worker")
	assert.Contains(t, errorMessages[2], "flavor_etcd")
}

func TestConfigStructureValidator_RejectsV1StorageLocation(t *testing.T) {
	validator := NewConfigStructureValidator()
	ctx := context.Background()

	// v1 structure with storage under opencenter (not infrastructure)
	config := map[string]interface{}{
		"schema_version": "1.0",
		"opencenter": map[string]interface{}{
			"storage": map[string]interface{}{
				"type": "ceph",
				"size": "100Gi",
			},
		},
	}

	result, err := validator.Validate(ctx, config)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)

	// Check error message
	assert.Contains(t, result.Errors[0].Message, "opencenter.storage")
	assert.Contains(t, result.Errors[0].Suggestions[1], "opencenter.infrastructure.storage")
}

func TestConfigStructureValidator_RejectsTopLevelStorageLocation(t *testing.T) {
	validator := NewConfigStructureValidator()
	ctx := context.Background()

	// v1 structure with top-level storage
	config := map[string]interface{}{
		"schema_version": "1.0",
		"storage": map[string]interface{}{
			"type": "ceph",
			"size": "100Gi",
		},
	}

	result, err := validator.Validate(ctx, config)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)

	// Check error message
	assert.Contains(t, result.Errors[0].Message, "storage (top-level)")
	assert.Contains(t, result.Errors[0].Suggestions[1], "opencenter.infrastructure.storage")
}

func TestConfigStructureValidator_IgnoresEmptyV1Fields(t *testing.T) {
	validator := NewConfigStructureValidator()
	ctx := context.Background()

	// v1 structure with empty storage (should not trigger error)
	config := map[string]interface{}{
		"schema_version": "2.0",
		"opencenter": map[string]interface{}{
			"storage": map[string]interface{}{},
		},
	}

	result, err := validator.Validate(ctx, config)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestConfigStructureValidator_MultipleV1Violations(t *testing.T) {
	validator := NewConfigStructureValidator()
	ctx := context.Background()

	// v1 structure with multiple violations
	config := map[string]interface{}{
		"schema_version": "1.0",
		"opencenter": map[string]interface{}{
			"cluster": map[string]interface{}{
				"networking": map[string]interface{}{
					"vrrp_ip": "10.0.0.1",
				},
				"kubernetes": map[string]interface{}{
					"flavor_control_plane": "m1.large",
				},
			},
			"storage": map[string]interface{}{
				"type": "ceph",
			},
		},
	}

	result, err := validator.Validate(ctx, config)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 3) // networking + flavor + storage
}

func TestConfigStructureValidator_InvalidInput(t *testing.T) {
	validator := NewConfigStructureValidator()
	ctx := context.Background()

	// Invalid input (not a map or struct)
	result, err := validator.Validate(ctx, "invalid")
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0].Message, "must be a configuration map or struct")
}
