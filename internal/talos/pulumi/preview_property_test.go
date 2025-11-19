package pulumi

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// Feature: talos-openstack-provider, Property 20: Pulumi preview before apply
// For any apply operation, a Pulumi preview showing all planned changes
// (creates, updates, deletes, replaces) should be generated and displayed before execution.
// Validates: Requirements 9.1, 9.2
func TestProperty_PulumiPreviewBeforeApply(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("preview is generated before apply", prop.ForAll(
		func(stackName string, containerName string) bool {
			// Create Pulumi configuration
			config := &talos.TalosPulumiConfig{
				StackName:      stackName,
				SwiftContainer: containerName,
				SwiftPrefix:    "test/",
			}

			// Create logger and manager
			logger := &testLogger{}
			manager, err := NewManager(config, "test-project", logger)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Create preview engine
			previewEngine, err := NewPreviewEngine(manager, logger)
			if err != nil {
				t.Logf("Failed to create preview engine: %v", err)
				return false
			}

			ctx := context.Background()

			// Execute preview
			preview, err := previewEngine.ExecutePreview(ctx)
			if err != nil {
				t.Logf("Failed to execute preview: %v", err)
				return false
			}

			// Verify preview is not nil
			if preview == nil {
				t.Log("Preview should not be nil")
				return false
			}

			// Verify preview has all required fields
			if preview.Creates == nil {
				t.Log("Preview.Creates should not be nil")
				return false
			}

			if preview.Updates == nil {
				t.Log("Preview.Updates should not be nil")
				return false
			}

			if preview.Deletes == nil {
				t.Log("Preview.Deletes should not be nil")
				return false
			}

			if preview.Replaces == nil {
				t.Log("Preview.Replaces should not be nil")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_PreviewDisplaysAllChangeTypes tests that preview displays all change types.
func TestProperty_PreviewDisplaysAllChangeTypes(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("preview displays creates, updates, deletes, and replaces", prop.ForAll(
		func(numCreates int, numUpdates int, numDeletes int, numReplaces int) bool {
			// Create a preview with various changes
			preview := &talos.PulumiPreview{
				Creates:  make([]talos.ResourceChange, numCreates),
				Updates:  make([]talos.ResourceChange, numUpdates),
				Deletes:  make([]talos.ResourceChange, numDeletes),
				Replaces: make([]talos.ResourceChange, numReplaces),
			}

			// Populate changes
			for i := 0; i < numCreates; i++ {
				preview.Creates[i] = talos.ResourceChange{
					Type: "openstack:compute/instance:Instance",
					Name: "test-instance",
				}
			}

			for i := 0; i < numUpdates; i++ {
				preview.Updates[i] = talos.ResourceChange{
					Type:   "openstack:networking/network:Network",
					Name:   "test-network",
					Reason: "configuration changed",
				}
			}

			for i := 0; i < numDeletes; i++ {
				preview.Deletes[i] = talos.ResourceChange{
					Type: "openstack:networking/subnet:Subnet",
					Name: "test-subnet",
				}
			}

			for i := 0; i < numReplaces; i++ {
				preview.Replaces[i] = talos.ResourceChange{
					Type:   "openstack:compute/instance:Instance",
					Name:   "test-node",
					Reason: "image version changed",
				}
			}

			// Create preview engine
			config := &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			}
			logger := &testLogger{}
			manager, _ := NewManager(config, "test-project", logger)
			previewEngine, _ := NewPreviewEngine(manager, logger)

			// Display planned changes
			ctx := context.Background()
			err := previewEngine.DisplayPlannedChanges(ctx, preview)
			if err != nil {
				t.Logf("Failed to display planned changes: %v", err)
				return false
			}

			// Verify change count
			totalChanges := previewEngine.GetChangeCount(preview)
			expectedChanges := numCreates + numUpdates + numDeletes + numReplaces
			if totalChanges != expectedChanges {
				t.Logf("Change count mismatch: expected %d, got %d", expectedChanges, totalChanges)
				return false
			}

			return true
		},
		gen.IntRange(0, 10),
		gen.IntRange(0, 10),
		gen.IntRange(0, 10),
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_NodeReplacementDetection tests node replacement detection.
func TestProperty_NodeReplacementDetection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("node replacements are detected in preview", prop.ForAll(
		func(hasNodeReplacement bool) bool {
			// Create a preview
			preview := &talos.PulumiPreview{
				Creates:  []talos.ResourceChange{},
				Updates:  []talos.ResourceChange{},
				Deletes:  []talos.ResourceChange{},
				Replaces: []talos.ResourceChange{},
			}

			// Add node replacement if specified
			if hasNodeReplacement {
				preview.Replaces = append(preview.Replaces, talos.ResourceChange{
					Type:   "openstack:compute/instance:Instance",
					Name:   "control-plane-1",
					Reason: "machine config changed",
				})
			}

			// Create preview engine
			config := &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			}
			logger := &testLogger{}
			manager, _ := NewManager(config, "test-project", logger)
			previewEngine, _ := NewPreviewEngine(manager, logger)

			// Check for node replacements
			detected := previewEngine.HasNodeReplacements(preview)

			// Verify detection matches expectation
			if detected != hasNodeReplacement {
				t.Logf("Node replacement detection mismatch: expected %v, got %v", hasNodeReplacement, detected)
				return false
			}

			// If there are node replacements, verify we can get them
			if hasNodeReplacement {
				nodeReplacements := previewEngine.GetNodeReplacements(preview)
				if len(nodeReplacements) == 0 {
					t.Log("Should have found node replacements")
					return false
				}
			}

			return true
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_SecurityPolicyChangeDetection tests security policy change detection.
func TestProperty_SecurityPolicyChangeDetection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("security policy changes are detected", prop.ForAll(
		func(hasSecurityChange bool) bool {
			// Create a preview
			preview := &talos.PulumiPreview{
				Creates:  []talos.ResourceChange{},
				Updates:  []talos.ResourceChange{},
				Deletes:  []talos.ResourceChange{},
				Replaces: []talos.ResourceChange{},
			}

			// Add security change if specified
			if hasSecurityChange {
				preview.Updates = append(preview.Updates, talos.ResourceChange{
					Type:   "openstack:networking/securityGroup:SecurityGroup",
					Name:   "control-plane-sg",
					Reason: "security rules updated",
				})
			}

			// Create preview engine
			config := &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			}
			logger := &testLogger{}
			manager, _ := NewManager(config, "test-project", logger)
			previewEngine, _ := NewPreviewEngine(manager, logger)

			// Get security policy changes
			securityChanges := previewEngine.GetSecurityPolicyChanges(preview)

			// Verify detection matches expectation
			if hasSecurityChange && len(securityChanges) == 0 {
				t.Log("Should have found security policy changes")
				return false
			}

			if !hasSecurityChange && len(securityChanges) > 0 {
				t.Log("Should not have found security policy changes")
				return false
			}

			return true
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
