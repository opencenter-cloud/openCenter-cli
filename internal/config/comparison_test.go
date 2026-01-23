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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
)

func TestCompareConfigs_NoChanges(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Set the same timestamps to ensure they're identical
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	config1.Metadata.CreatedAt = baseTime
	config1.Metadata.UpdatedAt = baseTime
	config2.Metadata.CreatedAt = baseTime
	config2.Metadata.UpdatedAt = baseTime

	diff := CompareConfigs(config1, config2)

	if diff.HasChanges {
		t.Errorf("Expected no changes, but got %d changes", len(diff.Changes))
		for _, change := range diff.Changes {
			t.Logf("Unexpected change: Path=%s, Type=%s", change.Path, change.Type)
		}
	}

	if diff.Summary.Added != 0 || diff.Summary.Removed != 0 || diff.Summary.Modified != 0 {
		t.Errorf("Expected zero changes in summary, got Added=%d, Removed=%d, Modified=%d",
			diff.Summary.Added, diff.Summary.Removed, diff.Summary.Modified)
	}
}

func TestCompareConfigs_SimpleFieldChange(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Modify a simple field
	config2.OpenCenter.Cluster.ClusterName = "modified-cluster"

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected")
	}

	if diff.Summary.Modified == 0 {
		t.Error("Expected at least one modified field")
	}

	// Check that the specific change was detected
	found := false
	for _, change := range diff.Changes {
		if change.Path == "OpenCenter.Cluster.ClusterName" && change.Type == ChangeTypeModified {
			found = true
			if change.OldValue != "test-cluster" {
				t.Errorf("Expected old value 'test-cluster', got '%v'", change.OldValue)
			}
			if change.NewValue != "modified-cluster" {
				t.Errorf("Expected new value 'modified-cluster', got '%v'", change.NewValue)
			}
		}
	}

	if !found {
		t.Error("Expected to find ClusterName change in diff")
	}
}

func TestCompareConfigs_NestedFieldChange(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Modify a nested field
	config2.OpenCenter.Cluster.Kubernetes.Version = "1.34.0"

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected")
	}

	// Check that the nested change was detected
	found := false
	for _, change := range diff.Changes {
		if change.Path == "OpenCenter.Cluster.Kubernetes.Version" && change.Type == ChangeTypeModified {
			found = true
		}
	}

	if !found {
		t.Error("Expected to find Kubernetes.Version change in diff")
	}
}

func TestCompareConfigs_MapChanges(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Add a new tag
	config2.Metadata.Tags["environment"] = "production"

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected")
	}

	// Check that the map addition was detected
	found := false
	for _, change := range diff.Changes {
		if change.Path == "Metadata.Tags[environment]" && change.Type == ChangeTypeAdded {
			found = true
			if change.NewValue != "production" {
				t.Errorf("Expected new value 'production', got '%v'", change.NewValue)
			}
		}
	}

	if !found {
		t.Error("Expected to find Tags[environment] addition in diff")
	}
}

func TestCompareConfigs_SliceChanges(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Modify slice length
	config2.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL = append(config2.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL, "10.0.0.0/8")

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected")
	}

	// Check that slice length change was detected
	foundLengthChange := false
	foundElementAdd := false
	for _, change := range diff.Changes {
		if change.Path == "OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL.length" && change.Type == ChangeTypeModified {
			foundLengthChange = true
		}
		if change.Path == "OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL[1]" && change.Type == ChangeTypeAdded {
			foundElementAdd = true
		}
	}

	if !foundLengthChange {
		t.Error("Expected to find slice length change in diff")
	}
	if !foundElementAdd {
		t.Error("Expected to find new element addition in diff")
	}
}

func TestCompareConfigs_ServiceMapChanges(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Modify a service configuration
	if certManager, ok := config2.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig); ok {
		certManager.Enabled = true
		certManager.Email = "test@example.com"
	}

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected")
	}

	// Debug: print all changes
	t.Logf("Total changes detected: %d", len(diff.Changes))
	for _, change := range diff.Changes {
		t.Logf("Change: Path=%s, Type=%s, Old=%v, New=%v", change.Path, change.Type, change.OldValue, change.NewValue)
	}

	// Check that service changes were detected
	hasServiceChanges := false
	for _, change := range diff.Changes {
		if strings.Contains(change.Path, "Services[cert-manager]") && strings.Contains(change.Path, "Enabled") {
			hasServiceChanges = true
		}
	}

	if !hasServiceChanges {
		t.Error("Expected to find service configuration changes in diff")
	}
}

func TestCompareConfigs_MetadataTimestampChange(t *testing.T) {
	// Test with full default configs to avoid zero-value issues
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Set explicit timestamps
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	config1.Metadata.CreatedAt = baseTime
	config1.Metadata.UpdatedAt = baseTime
	config1.Metadata.CreatedBy = "user1"

	config2.Metadata.CreatedAt = baseTime
	config2.Metadata.UpdatedAt = baseTime.Add(24 * time.Hour)
	config2.Metadata.CreatedBy = "user1"

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected")
	}

	// Debug: print all changes
	t.Logf("Total changes detected: %d", len(diff.Changes))
	for _, change := range diff.Changes {
		t.Logf("Change: Path=%s, Type=%s", change.Path, change.Type)
	}

	// Check that timestamp change was detected
	foundTimestamp := false
	for _, change := range diff.Changes {
		if strings.Contains(change.Path, "UpdatedAt") {
			foundTimestamp = true
		}
	}

	if !foundTimestamp {
		t.Error("Expected to find UpdatedAt change in diff")
	}
}

func TestCompareConfigs_MetadataChanges(t *testing.T) {
	// Create configs with explicitly different metadata from the start
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Set both to the same base time first
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	config1.Metadata.CreatedAt = baseTime
	config1.Metadata.UpdatedAt = baseTime
	config2.Metadata.CreatedAt = baseTime
	config2.Metadata.UpdatedAt = baseTime

	// Now modify config2
	config2.Metadata.UpdatedAt = baseTime.Add(24 * time.Hour)
	config2.Metadata.Tags["version"] = "2.0"

	// Verify timestamps are actually different
	t.Logf("Config1 UpdatedAt: %v", config1.Metadata.UpdatedAt)
	t.Logf("Config2 UpdatedAt: %v", config2.Metadata.UpdatedAt)
	t.Logf("Timestamps equal: %v", config1.Metadata.UpdatedAt.Equal(config2.Metadata.UpdatedAt))
	t.Logf("DeepEqual: %v", reflect.DeepEqual(config1.Metadata.UpdatedAt, config2.Metadata.UpdatedAt))

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected")
	}

	// Debug: print all changes
	t.Logf("Total changes detected: %d", len(diff.Changes))
	for _, change := range diff.Changes {
		t.Logf("Change: Path=%s, Type=%s, Old=%v, New=%v", change.Path, change.Type, change.OldValue, change.NewValue)
	}

	// Check that metadata changes were detected
	foundTimestamp := false
	foundTag := false
	for _, change := range diff.Changes {
		if strings.Contains(change.Path, "UpdatedAt") {
			foundTimestamp = true
			t.Logf("Found UpdatedAt change: %v -> %v", change.OldValue, change.NewValue)
		}
		if strings.Contains(change.Path, "Tags[version]") {
			foundTag = true
		}
	}

	if !foundTimestamp {
		t.Error("Expected to find UpdatedAt change in diff")
	}
	if !foundTag {
		t.Error("Expected to find Tags[version] addition in diff")
	}
}

func TestCompareConfigs_MultipleChanges(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Make multiple changes
	config2.OpenCenter.Cluster.ClusterName = "new-cluster"
	config2.OpenCenter.Cluster.Kubernetes.Version = "1.34.0"
	config2.OpenCenter.Cluster.Kubernetes.MasterCount = 5
	config2.Metadata.Tags["environment"] = "staging"

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected")
	}

	if len(diff.Changes) < 4 {
		t.Errorf("Expected at least 4 changes, got %d", len(diff.Changes))
	}

	if diff.Summary.Modified < 3 {
		t.Errorf("Expected at least 3 modified fields, got %d", diff.Summary.Modified)
	}

	if diff.Summary.Added < 1 {
		t.Errorf("Expected at least 1 added field, got %d", diff.Summary.Added)
	}
}

func TestConfigDiff_FormatDiff(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	config2.OpenCenter.Cluster.ClusterName = "modified-cluster"

	diff := CompareConfigs(config1, config2)

	formatted := diff.FormatDiff()

	if formatted == "" {
		t.Error("Expected non-empty formatted diff")
	}

	if formatted == "No changes detected" {
		t.Error("Expected changes to be formatted")
	}

	// Check that formatted output contains expected sections
	if !strings.Contains(formatted, "Configuration Diff Summary") {
		t.Error("Expected formatted diff to contain summary section")
	}

	if !strings.Contains(formatted, "Detailed Changes") {
		t.Error("Expected formatted diff to contain detailed changes section")
	}
}

func TestConfigDiff_FilterChangesByType(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Make changes of different types
	config2.OpenCenter.Cluster.ClusterName = "modified-cluster" // Modified
	config2.Metadata.Tags["new-tag"] = "value"                  // Added

	diff := CompareConfigs(config1, config2)

	modifiedChanges := diff.FilterChangesByType(ChangeTypeModified)
	addedChanges := diff.FilterChangesByType(ChangeTypeAdded)

	if len(modifiedChanges) == 0 {
		t.Error("Expected to find modified changes")
	}

	if len(addedChanges) == 0 {
		t.Error("Expected to find added changes")
	}

	// Verify that filtered changes are of correct type
	for _, change := range modifiedChanges {
		if change.Type != ChangeTypeModified {
			t.Errorf("Expected ChangeTypeModified, got %s", change.Type)
		}
	}

	for _, change := range addedChanges {
		if change.Type != ChangeTypeAdded {
			t.Errorf("Expected ChangeTypeAdded, got %s", change.Type)
		}
	}
}

func TestConfigDiff_FilterChangesByPath(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Make changes in different paths
	config2.OpenCenter.Cluster.ClusterName = "modified-cluster"
	config2.OpenCenter.Cluster.Kubernetes.Version = "1.34.0"
	config2.Metadata.Tags["tag"] = "value"

	diff := CompareConfigs(config1, config2)

	// Filter by path prefix
	clusterChanges := diff.FilterChangesByPath("OpenCenter.Cluster")
	metadataChanges := diff.FilterChangesByPath("Metadata")

	if len(clusterChanges) == 0 {
		t.Error("Expected to find cluster changes")
	}

	if len(metadataChanges) == 0 {
		t.Error("Expected to find metadata changes")
	}

	// Verify that filtered changes match the path prefix
	for _, change := range clusterChanges {
		if !strings.Contains(change.Path, "OpenCenter.Cluster") {
			t.Errorf("Expected path to contain 'OpenCenter.Cluster', got '%s'", change.Path)
		}
	}
}

func TestConfigDiff_HasChangesInPath(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	config2.OpenCenter.Cluster.ClusterName = "modified-cluster"

	diff := CompareConfigs(config1, config2)

	if !diff.HasChangesInPath("OpenCenter.Cluster") {
		t.Error("Expected to find changes in OpenCenter.Cluster path")
	}

	if diff.HasChangesInPath("NonExistent.Path") {
		t.Error("Expected no changes in non-existent path")
	}
}

func TestCompareConfigs_NilPointerHandling(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Set Talos to nil in config1 and non-nil in config2
	config1.OpenCenter.Talos = nil
	config2.OpenCenter.Talos = DefaultTalosConfig("test-cluster")

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected when Talos is added")
	}

	// Check that the Talos addition was detected
	found := false
	for _, change := range diff.Changes {
		if change.Path == "OpenCenter.Talos" && change.Type == ChangeTypeAdded {
			found = true
		}
	}

	if !found {
		t.Error("Expected to find Talos addition in diff")
	}
}

func TestCompareConfigs_EmptySliceVsNilSlice(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Set one to empty slice and one to nil
	config1.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL = []string{}
	config2.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.K8sAPIPortACL = nil

	diff := CompareConfigs(config1, config2)

	// Empty slice and nil slice should be treated as different
	if !diff.HasChanges {
		t.Error("Expected changes to be detected between empty and nil slice")
	}
}

func TestCompareConfigs_ComplexNestedStructure(t *testing.T) {
	config1 := NewDefault("test-cluster")
	config2 := NewDefault("test-cluster")

	// Modify deeply nested structure
	config2.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CNIIface = "eth1"
	config2.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.EncapsulationType = "IPIP"

	diff := CompareConfigs(config1, config2)

	if !diff.HasChanges {
		t.Fatal("Expected changes to be detected in nested structure")
	}

	// Verify both nested changes were detected
	foundCNIIface := false
	foundEncapsulation := false
	for _, change := range diff.Changes {
		if change.Path == "OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CNIIface" {
			foundCNIIface = true
		}
		if change.Path == "OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.EncapsulationType" {
			foundEncapsulation = true
		}
	}

	if !foundCNIIface {
		t.Error("Expected to find CNIIface change in diff")
	}
	if !foundEncapsulation {
		t.Error("Expected to find EncapsulationType change in diff")
	}
}
