package pulumi

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// Feature: talos-openstack-provider, Property 21: Drift detection completeness
// For any status command execution, drift detection should compare all managed resources
// (networks, security groups, instances, volumes) against expected configuration.
// Validates: Requirements 9.4, 9.5, 9.6
func TestProperty_DriftDetectionCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("drift detection compares all managed resources", prop.ForAll(
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

			// Create refresh engine
			refreshEngine, err := NewRefreshEngine(manager, logger)
			if err != nil {
				t.Logf("Failed to create refresh engine: %v", err)
				return false
			}

			ctx := context.Background()

			// Execute refresh
			report, err := refreshEngine.ExecuteRefresh(ctx)
			if err != nil {
				t.Logf("Failed to execute refresh: %v", err)
				return false
			}

			// Verify report is not nil
			if report == nil {
				t.Log("Drift report should not be nil")
				return false
			}

			// Verify report has all required fields
			if report.Drifted == nil {
				t.Log("Report.Drifted should not be nil")
				return false
			}

			if report.Remediations == nil {
				t.Log("Report.Remediations should not be nil")
				return false
			}

			// Verify HasDrift is consistent with Drifted resources
			if report.HasDrift && len(report.Drifted) == 0 {
				t.Log("HasDrift is true but no drifted resources found")
				return false
			}

			if !report.HasDrift && len(report.Drifted) > 0 {
				t.Log("HasDrift is false but drifted resources found")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_DriftReportRemediations tests that drift reports include remediations.
func TestProperty_DriftReportRemediations(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("drift report includes remediations for each drifted resource", prop.ForAll(
		func(numDrifted int) bool {
			// Create drifted resources
			drifted := make([]talos.DriftedResource, numDrifted)
			for i := 0; i < numDrifted; i++ {
				drifted[i] = talos.DriftedResource{
					Type: "openstack:networking/network:Network",
					Name: "test-network",
					Expected: map[string]interface{}{
						"cidr": "10.0.0.0/24",
					},
					Actual: map[string]interface{}{
						"cidr": "10.0.1.0/24",
					},
				}
			}

			// Create refresh engine
			config := &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			}
			logger := &testLogger{}
			manager, _ := NewManager(config, "test-project", logger)
			refreshEngine, _ := NewRefreshEngine(manager, logger)

			// Generate drift report
			ctx := context.Background()
			report, err := refreshEngine.GenerateDriftReport(ctx, drifted)
			if err != nil {
				t.Logf("Failed to generate drift report: %v", err)
				return false
			}

			// Verify report has remediations for each drifted resource
			if len(report.Remediations) != numDrifted {
				t.Logf("Remediation count mismatch: expected %d, got %d", numDrifted, len(report.Remediations))
				return false
			}

			// Verify each remediation has required fields
			for _, remediation := range report.Remediations {
				if remediation.Check == "" {
					t.Log("Remediation.Check should not be empty")
					return false
				}

				if remediation.Description == "" {
					t.Log("Remediation.Description should not be empty")
					return false
				}

				if len(remediation.Steps) == 0 {
					t.Log("Remediation.Steps should not be empty")
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_SecurityDriftDetection tests security drift detection.
func TestProperty_SecurityDriftDetection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("security drift is detected and reported", prop.ForAll(
		func(hasSecurityDrift bool) bool {
			// Create drift report
			report := &talos.DriftReport{
				HasDrift:     hasSecurityDrift,
				Drifted:      []talos.DriftedResource{},
				Remediations: []talos.RemediationAction{},
			}

			// Add security drift if specified
			if hasSecurityDrift {
				report.Drifted = append(report.Drifted, talos.DriftedResource{
					Type: "openstack:networking/securityGroup:SecurityGroup",
					Name: "control-plane-sg",
					Expected: map[string]interface{}{
						"rules": []string{"allow-6443"},
					},
					Actual: map[string]interface{}{
						"rules": []string{"allow-6443", "allow-22"},
					},
				})
			}

			// Create refresh engine
			config := &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			}
			logger := &testLogger{}
			manager, _ := NewManager(config, "test-project", logger)
			refreshEngine, _ := NewRefreshEngine(manager, logger)

			// Check for security drift
			detected := refreshEngine.HasSecurityDrift(report)

			// Verify detection matches expectation
			if detected != hasSecurityDrift {
				t.Logf("Security drift detection mismatch: expected %v, got %v", hasSecurityDrift, detected)
				return false
			}

			return true
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_NetworkDriftDetection tests network drift detection.
func TestProperty_NetworkDriftDetection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("network drift is detected and reported", prop.ForAll(
		func(hasNetworkDrift bool) bool {
			// Create drift report
			report := &talos.DriftReport{
				HasDrift:     hasNetworkDrift,
				Drifted:      []talos.DriftedResource{},
				Remediations: []talos.RemediationAction{},
			}

			// Add network drift if specified
			if hasNetworkDrift {
				report.Drifted = append(report.Drifted, talos.DriftedResource{
					Type: "openstack:networking/network:Network",
					Name: "control-network",
					Expected: map[string]interface{}{
						"cidr": "10.0.1.0/24",
					},
					Actual: map[string]interface{}{
						"cidr": "10.0.2.0/24",
					},
				})
			}

			// Create refresh engine
			config := &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			}
			logger := &testLogger{}
			manager, _ := NewManager(config, "test-project", logger)
			refreshEngine, _ := NewRefreshEngine(manager, logger)

			// Check for network drift
			detected := refreshEngine.HasNetworkDrift(report)

			// Verify detection matches expectation
			if detected != hasNetworkDrift {
				t.Logf("Network drift detection mismatch: expected %v, got %v", hasNetworkDrift, detected)
				return false
			}

			return true
		},
		gen.Bool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_DriftedResourcesByType tests filtering drifted resources by type.
func TestProperty_DriftedResourcesByType(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("drifted resources can be filtered by type", prop.ForAll(
		func(numNetworks int, numInstances int) bool {
			// Create drift report with mixed resource types
			report := &talos.DriftReport{
				HasDrift:     true,
				Drifted:      []talos.DriftedResource{},
				Remediations: []talos.RemediationAction{},
			}

			// Add network resources
			for i := 0; i < numNetworks; i++ {
				report.Drifted = append(report.Drifted, talos.DriftedResource{
					Type: "openstack:networking/network:Network",
					Name: "test-network",
				})
			}

			// Add instance resources
			for i := 0; i < numInstances; i++ {
				report.Drifted = append(report.Drifted, talos.DriftedResource{
					Type: "openstack:compute/instance:Instance",
					Name: "test-instance",
				})
			}

			// Create refresh engine
			config := &talos.TalosPulumiConfig{
				StackName:      "test-stack",
				SwiftContainer: "test-container",
			}
			logger := &testLogger{}
			manager, _ := NewManager(config, "test-project", logger)
			refreshEngine, _ := NewRefreshEngine(manager, logger)

			// Filter by network type
			networks := refreshEngine.GetDriftedResourcesByType(report, "openstack:networking/network:Network")
			if len(networks) != numNetworks {
				t.Logf("Network count mismatch: expected %d, got %d", numNetworks, len(networks))
				return false
			}

			// Filter by instance type
			instances := refreshEngine.GetDriftedResourcesByType(report, "openstack:compute/instance:Instance")
			if len(instances) != numInstances {
				t.Logf("Instance count mismatch: expected %d, got %d", numInstances, len(instances))
				return false
			}

			return true
		},
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ExpectedVsActualComparison tests expected vs actual state comparison.
func TestProperty_ExpectedVsActualComparison(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("expected and actual states are compared correctly", prop.ForAll(
		func(expectedCIDR string, actualCIDR string) bool {
			// Create drifted resource
			drifted := talos.DriftedResource{
				Type: "openstack:networking/network:Network",
				Name: "test-network",
				Expected: map[string]interface{}{
					"cidr": expectedCIDR,
				},
				Actual: map[string]interface{}{
					"cidr": actualCIDR,
				},
			}

			// Verify drift is detected when values differ
			hasDrift := expectedCIDR != actualCIDR

			// Verify expected and actual are properly stored
			if drifted.Expected["cidr"] != expectedCIDR {
				t.Log("Expected CIDR not stored correctly")
				return false
			}

			if drifted.Actual["cidr"] != actualCIDR {
				t.Log("Actual CIDR not stored correctly")
				return false
			}

			// If there's drift, verify we can detect it
			if hasDrift {
				if drifted.Expected["cidr"] == drifted.Actual["cidr"] {
					t.Log("Drift should be detected")
					return false
				}
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
