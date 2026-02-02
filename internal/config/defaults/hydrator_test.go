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

package defaults

import (
	"testing"
)

// TestHydrator_ExplicitValueNotOverridden verifies that explicit values are not overridden by defaults.
func TestHydrator_ExplicitValueNotOverridden(t *testing.T) {
	registry := GetGlobalRegistry()
	hydrator := NewHydrator(registry)

	cfg := &TestConfig{
		ImageID: "my-custom-image",
	}

	err := hydrator.Hydrate(cfg, "openstack", "sjc3")
	if err != nil {
		t.Fatalf("Hydrate failed: %v", err)
	}

	if cfg.ImageID != "my-custom-image" {
		t.Errorf("Expected ImageID to remain 'my-custom-image', got '%s'", cfg.ImageID)
	}
}

// TestHydrator_EmptyFieldsPopulated verifies that empty fields are populated with defaults.
func TestHydrator_EmptyFieldsPopulated(t *testing.T) {
	registry := GetGlobalRegistry()
	hydrator := NewHydrator(registry)

	cfg := &TestConfig{}

	err := hydrator.Hydrate(cfg, "openstack", "sjc3")
	if err != nil {
		t.Fatalf("Hydrate failed: %v", err)
	}

	if cfg.ImageID == "" {
		t.Error("Expected ImageID to be populated with default")
	}

	if cfg.DefaultStorageClass == "" {
		t.Error("Expected DefaultStorageClass to be populated with default")
	}

	if len(cfg.AvailabilityZones) == 0 {
		t.Error("Expected AvailabilityZones to be populated with defaults")
	}
}

// TestHydrator_AppliedDefaultsTracking verifies that applied defaults are tracked correctly.
func TestHydrator_AppliedDefaultsTracking(t *testing.T) {
	registry := GetGlobalRegistry()
	hydrator := NewHydrator(registry)

	cfg := &TestConfig{}

	err := hydrator.Hydrate(cfg, "openstack", "sjc3")
	if err != nil {
		t.Fatalf("Hydrate failed: %v", err)
	}

	appliedDefaults := hydrator.GetAppliedDefaults()

	if len(appliedDefaults) == 0 {
		t.Error("Expected some defaults to be tracked")
	}

	if source, ok := appliedDefaults["ImageID"]; !ok || source != SourceProviderRegion {
		t.Errorf("Expected ImageID to be tracked with source SourceProviderRegion, got %v", source)
	}
}

// TestHydrator_InvalidProviderRegion verifies graceful handling for invalid provider-region combinations.
func TestHydrator_InvalidProviderRegion(t *testing.T) {
	registry := GetGlobalRegistry()
	hydrator := NewHydrator(registry)

	cfg := &TestConfig{}

	// Invalid provider-region combinations should not error, but should not apply defaults
	err := hydrator.Hydrate(cfg, "invalid-provider", "invalid-region")
	if err != nil {
		t.Errorf("Unexpected error for invalid provider-region combination: %v", err)
	}

	// Verify no defaults were applied
	appliedDefaults := hydrator.GetAppliedDefaults()
	if len(appliedDefaults) != 0 {
		t.Errorf("Expected no defaults to be applied for invalid provider-region, got %d", len(appliedDefaults))
	}

	// Verify config fields remain empty
	if cfg.ImageID != "" || cfg.DefaultStorageClass != "" || len(cfg.AvailabilityZones) > 0 {
		t.Error("Expected config fields to remain empty for invalid provider-region")
	}
}
