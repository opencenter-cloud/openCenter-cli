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
	"testing"
)

// TestNewDefault verifies that NewDefault returns a properly initialized configuration.
func TestNewDefault(t *testing.T) {
	clusterName := "test-cluster"
	cfg := NewDefault(clusterName)

	// Verify basic fields are set
	if cfg.OpenCenter.Meta.Name != clusterName {
		t.Errorf("expected cluster name %s, got %s", clusterName, cfg.OpenCenter.Meta.Name)
	}

	if cfg.SchemaVersion == "" {
		t.Error("expected schema version to be set")
	}

	if cfg.OpenCenter.Infrastructure.Provider == "" {
		t.Error("expected provider to be set")
	}
}

// TestApplyDefaults verifies that ApplyDefaults applies organization and CLI defaults.
func TestApplyDefaults(t *testing.T) {
	cfg := NewDefault("test-cluster")
	cfg.OpenCenter.Meta.Organization = "test-org"

	// Apply defaults
	ApplyDefaults(&cfg)

	// Verify the function runs without error
	// The actual behavior is tested in internal/config/defaults_test.go
	if cfg.OpenCenter.Meta.Organization != "test-org" {
		t.Error("ApplyDefaults should not modify explicitly set organization")
	}
}

// TestDefaultTalosConfig verifies that DefaultTalosConfig returns a properly initialized configuration.
func TestDefaultTalosConfig(t *testing.T) {
	clusterName := "test-cluster"
	talosConfig := DefaultTalosConfig(clusterName)

	if talosConfig == nil {
		t.Fatal("expected non-nil TalosConfig")
	}

	if !talosConfig.Enabled {
		t.Error("expected Talos to be enabled by default")
	}

	if talosConfig.Version == "" {
		t.Error("expected Talos version to be set")
	}

	if talosConfig.PulumiConfig.StackName == "" {
		t.Error("expected Pulumi stack name to be set")
	}
}
