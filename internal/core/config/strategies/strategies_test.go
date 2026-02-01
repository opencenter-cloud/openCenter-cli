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
	"testing"
)

// TestV2StrategyCanLoad tests V2Strategy version detection
func TestV2StrategyCanLoad(t *testing.T) {
	strategy := NewV2Strategy()

	tests := []struct {
		name     string
		data     string
		expected bool
		wantErr  bool
	}{
		{
			name: "v2 configuration",
			data: `schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster`,
			expected: true,
			wantErr:  false,
		},
		{
			name: "v1 configuration",
			data: `schema_version: "1.0"
opencenter:
  meta:
    name: test-cluster`,
			expected: false,
			wantErr:  false,
		},
		{
			name: "no version",
			data: `opencenter:
  meta:
    name: test-cluster`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "invalid YAML",
			data:     `invalid: yaml: [`,
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canLoad, err := strategy.CanLoad([]byte(tt.data))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if canLoad != tt.expected {
				t.Errorf("CanLoad() = %v, want %v", canLoad, tt.expected)
			}
		})
	}
}

// TestV1StrategyCanLoad tests V1Strategy version detection
func TestV1StrategyCanLoad(t *testing.T) {
	strategy := NewV1Strategy()

	tests := []struct {
		name     string
		data     string
		expected bool
		wantErr  bool
	}{
		{
			name: "v1 explicit",
			data: `schema_version: "1.0"
opencenter:
  cluster:
    cluster_name: test-cluster`,
			expected: true,
			wantErr:  false,
		},
		{
			name: "v1 implicit (no version)",
			data: `opencenter:
  cluster:
    cluster_name: test-cluster`,
			expected: true,
			wantErr:  false,
		},
		{
			name: "v2 configuration",
			data: `schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster`,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "invalid YAML",
			data:     `invalid: yaml: [`,
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canLoad, err := strategy.CanLoad([]byte(tt.data))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if canLoad != tt.expected {
				t.Errorf("CanLoad() = %v, want %v", canLoad, tt.expected)
			}
		})
	}
}

// TestLegacyStrategyCanLoad tests LegacyStrategy version detection
func TestLegacyStrategyCanLoad(t *testing.T) {
	strategy := NewLegacyStrategy()

	tests := []struct {
		name     string
		data     string
		expected bool
		wantErr  bool
	}{
		{
			name: "legacy flat structure",
			data: `cluster_name: test-cluster
provider: openstack
region: sjc3`,
			expected: true,
			wantErr:  false,
		},
		{
			name: "legacy with provider only",
			data: `provider: openstack
environment: dev`,
			expected: true,
			wantErr:  false,
		},
		{
			name: "v1 configuration",
			data: `schema_version: "1.0"
opencenter:
  cluster:
    cluster_name: test-cluster`,
			expected: false,
			wantErr:  false,
		},
		{
			name: "v2 configuration",
			data: `schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster`,
			expected: false,
			wantErr:  false,
		},
		{
			name: "empty opencenter section with sibling fields",
			data: `opencenter:
cluster_name: test-cluster`,
			expected: true, // This is actually a legacy config with empty opencenter
			wantErr:  false,
		},
		{
			name:     "invalid YAML",
			data:     `invalid: yaml: [`,
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canLoad, err := strategy.CanLoad([]byte(tt.data))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if canLoad != tt.expected {
				t.Errorf("CanLoad() = %v, want %v", canLoad, tt.expected)
			}
		})
	}
}

// TestV1StrategyLoad tests V1Strategy loading
func TestV1StrategyLoad(t *testing.T) {
	strategy := NewV1Strategy()

	data := `schema_version: "1.0"
opencenter:
  meta:
    name: test-cluster
    region: sjc3
  cluster:
    cluster_name: test-cluster
    base_domain: k8s.opencenter.cloud`

	config, err := strategy.Load([]byte(data), "test-cluster")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if config == nil {
		t.Fatal("Load() returned nil config")
	}

	if config.ClusterName() != "test-cluster" {
		t.Errorf("ClusterName() = %v, want test-cluster", config.ClusterName())
	}
}

// TestV1StrategyVersion tests V1Strategy version identifier
func TestV1StrategyVersion(t *testing.T) {
	strategy := NewV1Strategy()

	if version := strategy.Version(); version != "1.0" {
		t.Errorf("Version() = %v, want 1.0", version)
	}
}

// TestV2StrategyVersion tests V2Strategy version identifier
func TestV2StrategyVersion(t *testing.T) {
	strategy := NewV2Strategy()

	if version := strategy.Version(); version != "2.0" {
		t.Errorf("Version() = %v, want 2.0", version)
	}
}

// TestLegacyStrategyVersion tests LegacyStrategy version identifier
func TestLegacyStrategyVersion(t *testing.T) {
	strategy := NewLegacyStrategy()

	if version := strategy.Version(); version != "legacy" {
		t.Errorf("Version() = %v, want legacy", version)
	}
}

// TestLegacyStrategyLoad tests LegacyStrategy loading
func TestLegacyStrategyLoad(t *testing.T) {
	strategy := NewLegacyStrategy()

	data := `cluster_name: test-cluster
provider: openstack
region: sjc3
environment: dev`

	config, err := strategy.Load([]byte(data), "test-cluster")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if config == nil {
		t.Fatal("Load() returned nil config")
	}

	// Verify basic field mapping
	if config.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Errorf("Provider = %v, want openstack", config.OpenCenter.Infrastructure.Provider)
	}

	if config.OpenCenter.Meta.Region != "sjc3" {
		t.Errorf("Region = %v, want sjc3", config.OpenCenter.Meta.Region)
	}

	if config.OpenCenter.Meta.Env != "dev" {
		t.Errorf("Environment = %v, want dev", config.OpenCenter.Meta.Env)
	}
}

// TestStrategySelection tests that strategies correctly identify their configs
func TestStrategySelection(t *testing.T) {
	v1Strategy := NewV1Strategy()
	v2Strategy := NewV2Strategy()
	legacyStrategy := NewLegacyStrategy()

	tests := []struct {
		name           string
		data           string
		expectedV1     bool
		expectedV2     bool
		expectedLegacy bool
	}{
		{
			name: "v1 explicit",
			data: `schema_version: "1.0"
opencenter:
  cluster:
    cluster_name: test`,
			expectedV1:     true,
			expectedV2:     false,
			expectedLegacy: false,
		},
		{
			name: "v2 explicit",
			data: `schema_version: "2.0"
opencenter:
  meta:
    name: test`,
			expectedV1:     false,
			expectedV2:     true,
			expectedLegacy: false,
		},
		{
			name: "legacy flat",
			data: `cluster_name: test
provider: openstack`,
			expectedV1:     true, // V1 also accepts no version
			expectedV2:     false,
			expectedLegacy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1Can, _ := v1Strategy.CanLoad([]byte(tt.data))
			v2Can, _ := v2Strategy.CanLoad([]byte(tt.data))
			legacyCan, _ := legacyStrategy.CanLoad([]byte(tt.data))

			if v1Can != tt.expectedV1 {
				t.Errorf("V1 CanLoad() = %v, want %v", v1Can, tt.expectedV1)
			}
			if v2Can != tt.expectedV2 {
				t.Errorf("V2 CanLoad() = %v, want %v", v2Can, tt.expectedV2)
			}
			if legacyCan != tt.expectedLegacy {
				t.Errorf("Legacy CanLoad() = %v, want %v", legacyCan, tt.expectedLegacy)
			}
		})
	}
}
