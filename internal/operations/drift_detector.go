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

package operations

import (
	"context"
	"fmt"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

// Severity represents the severity level of drift
type Severity int

const (
	// SeverityInfo represents informational drift (metadata, timestamps)
	SeverityInfo Severity = iota
	// SeverityWarning represents warning-level drift (worker nodes, tags, labels)
	SeverityWarning
	// SeverityCritical represents critical drift (control plane, network configuration)
	SeverityCritical
)

// String returns the string representation of severity
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// DriftDetector defines the interface for detecting infrastructure drift
type DriftDetector interface {
	// DetectDrift compares desired configuration with actual infrastructure state
	DetectDrift(ctx context.Context, cluster string) (*DriftReport, error)

	// ReconcileDrift corrects detected drift
	ReconcileDrift(ctx context.Context, cluster string, dryRun bool) error

	// SchedulePeriodicCheck schedules periodic drift detection
	SchedulePeriodicCheck(interval time.Duration, callback func(*DriftReport))
}

// DriftReport represents the result of drift detection
type DriftReport struct {
	ID           string          `json:"id"`
	Cluster      string          `json:"cluster"`
	DetectedAt   time.Time       `json:"detected_at"`
	Drifts       []ResourceDrift `json:"drifts"`
	Severity     Severity        `json:"severity"`
	Reconcilable bool            `json:"reconcilable"`
	Summary      DriftSummary    `json:"summary"`
}

// ResourceDrift represents a single resource drift
type ResourceDrift struct {
	ResourceType string      `json:"resource_type"`
	ResourceID   string      `json:"resource_id"`
	ResourceName string      `json:"resource_name"`
	Field        string      `json:"field"`
	Expected     interface{} `json:"expected"`
	Actual       interface{} `json:"actual"`
	Severity     Severity    `json:"severity"`
	Reconcilable bool        `json:"reconcilable"`
}

// DriftSummary provides aggregate statistics about drift
type DriftSummary struct {
	TotalDrifts       int `json:"total_drifts"`
	CriticalCount     int `json:"critical_count"`
	WarningCount      int `json:"warning_count"`
	InfoCount         int `json:"info_count"`
	ReconcilableCount int `json:"reconcilable_count"`
}

// CloudProvider defines the interface for cloud provider operations
type CloudProvider interface {
	// GetInstances retrieves all instances for the cluster
	GetInstances(ctx context.Context, cluster string) ([]Instance, error)

	// GetNetworks retrieves all networks for the cluster
	GetNetworks(ctx context.Context, cluster string) ([]Network, error)

	// GetSecurityGroups retrieves all security groups for the cluster
	GetSecurityGroups(ctx context.Context, cluster string) ([]SecurityGroup, error)

	// GetLoadBalancers retrieves all load balancers for the cluster
	GetLoadBalancers(ctx context.Context, cluster string) ([]LoadBalancer, error)
}

// Instance represents a compute instance
type Instance struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Flavor string            `json:"flavor"`
	Image  string            `json:"image"`
	Status string            `json:"status"`
	Tags   map[string]string `json:"tags"`
}

// Network represents a network resource
type Network struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	CIDR    string   `json:"cidr"`
	Subnets []string `json:"subnets"`
}

// SecurityGroup represents a security group
type SecurityGroup struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Rules []SecurityRule `json:"rules"`
}

// SecurityRule represents a security group rule
type SecurityRule struct {
	Direction   string `json:"direction"`
	Protocol    string `json:"protocol"`
	PortRange   string `json:"port_range"`
	RemoteIP    string `json:"remote_ip"`
	Description string `json:"description"`
}

// LoadBalancer represents a load balancer
type LoadBalancer struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	VIP      string   `json:"vip"`
	Members  []string `json:"members"`
	Protocol string   `json:"protocol"`
}

// ConfigLoader defines the interface for loading cluster configurations
type ConfigLoader interface {
	LoadConfig(ctx context.Context, cluster string) (*config.Config, error)
}

// driftDetector implements the DriftDetector interface
type driftDetector struct {
	configLoader ConfigLoader
	provider     CloudProvider
}

// NewDriftDetector creates a new drift detector
func NewDriftDetector(configLoader ConfigLoader, provider CloudProvider) DriftDetector {
	return &driftDetector{
		configLoader: configLoader,
		provider:     provider,
	}
}

// DetectDrift implements DriftDetector.DetectDrift
func (d *driftDetector) DetectDrift(ctx context.Context, cluster string) (*DriftReport, error) {
	// Load desired configuration
	cfg, err := d.configLoader.LoadConfig(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	report := &DriftReport{
		ID:         fmt.Sprintf("drift-%s-%d", cluster, time.Now().Unix()),
		Cluster:    cluster,
		DetectedAt: time.Now(),
		Drifts:     []ResourceDrift{},
		Summary:    DriftSummary{},
	}

	// Detect instance drift
	if err := d.detectInstanceDrift(ctx, cluster, cfg, report); err != nil {
		return nil, fmt.Errorf("failed to detect instance drift: %w", err)
	}

	// Detect network drift
	if err := d.detectNetworkDrift(ctx, cluster, cfg, report); err != nil {
		return nil, fmt.Errorf("failed to detect network drift: %w", err)
	}

	// Detect security group drift
	if err := d.detectSecurityGroupDrift(ctx, cluster, cfg, report); err != nil {
		return nil, fmt.Errorf("failed to detect security group drift: %w", err)
	}

	// Detect load balancer drift
	if err := d.detectLoadBalancerDrift(ctx, cluster, cfg, report); err != nil {
		return nil, fmt.Errorf("failed to detect load balancer drift: %w", err)
	}

	// Calculate summary and overall severity
	d.calculateSummary(report)

	return report, nil
}

// detectInstanceDrift detects drift in compute instances
func (d *driftDetector) detectInstanceDrift(ctx context.Context, cluster string, cfg *config.Config, report *DriftReport) error {
	instances, err := d.provider.GetInstances(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to get instances: %w", err)
	}

	// Compare actual instances with expected configuration
	// This is a simplified implementation - in reality, you'd need to map
	// configuration to expected instances based on node pools, etc.
	for _, instance := range instances {
		// Check if instance has expected tags
		if clusterTag, ok := instance.Tags["cluster"]; !ok || clusterTag != cluster {
			report.Drifts = append(report.Drifts, ResourceDrift{
				ResourceType: "instance",
				ResourceID:   instance.ID,
				ResourceName: instance.Name,
				Field:        "tags.cluster",
				Expected:     cluster,
				Actual:       clusterTag,
				Severity:     SeverityWarning,
				Reconcilable: true,
			})
		}

		// Check instance status
		if instance.Status != "ACTIVE" {
			report.Drifts = append(report.Drifts, ResourceDrift{
				ResourceType: "instance",
				ResourceID:   instance.ID,
				ResourceName: instance.Name,
				Field:        "status",
				Expected:     "ACTIVE",
				Actual:       instance.Status,
				Severity:     SeverityCritical,
				Reconcilable: false,
			})
		}
	}

	return nil
}

// detectNetworkDrift detects drift in network resources
func (d *driftDetector) detectNetworkDrift(ctx context.Context, cluster string, cfg *config.Config, report *DriftReport) error {
	networks, err := d.provider.GetNetworks(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to get networks: %w", err)
	}

	// Compare actual networks with expected configuration
	expectedCIDR := cfg.OpenCenter.Cluster.Networking.SubnetNodes
	for _, network := range networks {
		if network.CIDR != expectedCIDR {
			report.Drifts = append(report.Drifts, ResourceDrift{
				ResourceType: "network",
				ResourceID:   network.ID,
				ResourceName: network.Name,
				Field:        "cidr",
				Expected:     expectedCIDR,
				Actual:       network.CIDR,
				Severity:     SeverityCritical,
				Reconcilable: false,
			})
		}
	}

	return nil
}

// detectSecurityGroupDrift detects drift in security groups
func (d *driftDetector) detectSecurityGroupDrift(ctx context.Context, cluster string, cfg *config.Config, report *DriftReport) error {
	securityGroups, err := d.provider.GetSecurityGroups(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to get security groups: %w", err)
	}

	// Compare actual security groups with expected configuration
	for _, sg := range securityGroups {
		// Check if security group has expected rules
		// This is simplified - in reality, you'd compare rule sets
		if len(sg.Rules) == 0 {
			report.Drifts = append(report.Drifts, ResourceDrift{
				ResourceType: "security_group",
				ResourceID:   sg.ID,
				ResourceName: sg.Name,
				Field:        "rules",
				Expected:     "non-empty",
				Actual:       "empty",
				Severity:     SeverityCritical,
				Reconcilable: true,
			})
		}
	}

	return nil
}

// detectLoadBalancerDrift detects drift in load balancers
func (d *driftDetector) detectLoadBalancerDrift(ctx context.Context, cluster string, cfg *config.Config, report *DriftReport) error {
	loadBalancers, err := d.provider.GetLoadBalancers(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to get load balancers: %w", err)
	}

	// Compare actual load balancers with expected configuration
	for _, lb := range loadBalancers {
		// Check if load balancer has expected members
		if len(lb.Members) == 0 {
			report.Drifts = append(report.Drifts, ResourceDrift{
				ResourceType: "load_balancer",
				ResourceID:   lb.ID,
				ResourceName: lb.Name,
				Field:        "members",
				Expected:     "non-empty",
				Actual:       "empty",
				Severity:     SeverityWarning,
				Reconcilable: true,
			})
		}
	}

	return nil
}

// calculateSummary calculates the drift summary and overall severity
func (d *driftDetector) calculateSummary(report *DriftReport) {
	report.Summary.TotalDrifts = len(report.Drifts)
	report.Reconcilable = true
	report.Severity = SeverityInfo

	for _, drift := range report.Drifts {
		switch drift.Severity {
		case SeverityCritical:
			report.Summary.CriticalCount++
			report.Severity = SeverityCritical
		case SeverityWarning:
			report.Summary.WarningCount++
			if report.Severity < SeverityWarning {
				report.Severity = SeverityWarning
			}
		case SeverityInfo:
			report.Summary.InfoCount++
		}

		if drift.Reconcilable {
			report.Summary.ReconcilableCount++
		} else {
			report.Reconcilable = false
		}
	}
}

// ReconcileDrift implements DriftDetector.ReconcileDrift
func (d *driftDetector) ReconcileDrift(ctx context.Context, cluster string, dryRun bool) error {
	// Detect drift first
	report, err := d.DetectDrift(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to detect drift: %w", err)
	}

	if len(report.Drifts) == 0 {
		return nil // No drift to reconcile
	}

	if !report.Reconcilable {
		return fmt.Errorf("drift contains non-reconcilable changes")
	}

	if dryRun {
		// In dry-run mode, just return the plan
		fmt.Printf("Drift reconciliation plan for cluster %s:\n", cluster)
		for _, drift := range report.Drifts {
			if drift.Reconcilable {
				fmt.Printf("  - %s %s.%s: %v -> %v\n",
					drift.ResourceType, drift.ResourceName, drift.Field,
					drift.Actual, drift.Expected)
			}
		}
		return nil
	}

	// Apply reconciliation
	// This is a placeholder - actual implementation would call provider APIs
	// to update resources to match desired configuration
	for _, drift := range report.Drifts {
		if !drift.Reconcilable {
			continue
		}

		// Apply the change based on resource type
		switch drift.ResourceType {
		case "instance":
			// Update instance tags, etc.
		case "security_group":
			// Update security group rules
		case "load_balancer":
			// Update load balancer members
		}
	}

	return nil
}

// SchedulePeriodicCheck implements DriftDetector.SchedulePeriodicCheck
func (d *driftDetector) SchedulePeriodicCheck(interval time.Duration, callback func(*DriftReport)) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			// This is a simplified implementation
			// In production, you'd want to:
			// 1. Get list of clusters to check
			// 2. Run drift detection for each
			// 3. Call callback with results
			// 4. Handle errors and retries
			// 5. Support cancellation via context
		}
	}()
}
