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

package cloud

import (
	"context"
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// CloudProvider defines the interface for cloud provider operations needed for drift detection.
// Implementations provide access to infrastructure state and drift reconciliation capabilities.
type CloudProvider interface {
	// GetCurrentState retrieves the current infrastructure state from the cloud provider.
	// It queries the provider's APIs to get the actual state of all resources.
	GetCurrentState(ctx context.Context, cfg v2.Config) (*InfrastructureState, error)

	// DetectDrift compares desired state (from config) with actual state (from provider)
	// and returns a report of all differences found.
	DetectDrift(ctx context.Context, desired, actual *InfrastructureState) (*DriftReport, error)

	// ReconcileDrift applies changes to the infrastructure to bring it back in line
	// with the desired state. Only reconcilable drift items are processed.
	ReconcileDrift(ctx context.Context, drift *DriftReport) error
}

// InfrastructureState represents the complete state of infrastructure resources.
// It captures all resources managed by the cloud provider for a cluster.
type InfrastructureState struct {
	// Servers are the compute instances (VMs) in the cluster
	Servers []Server `json:"servers"`

	// Networks are the network resources (VPCs, subnets)
	Networks []Network `json:"networks"`

	// SecurityGroups are the firewall rules
	SecurityGroups []SecurityGroup `json:"security_groups"`

	// LoadBalancers are the load balancing resources
	LoadBalancers []LoadBalancer `json:"load_balancers"`

	// Volumes are the block storage volumes
	Volumes []Volume `json:"volumes"`

	// FloatingIPs are the public IP addresses
	FloatingIPs []FloatingIP `json:"floating_ips"`
}

// Server represents a compute instance in the infrastructure.
type Server struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Flavor   string            `json:"flavor"`
	Image    string            `json:"image"`
	Status   string            `json:"status"`
	Networks []string          `json:"networks"`
	Tags     map[string]string `json:"tags"`
}

// Network represents a network resource.
type Network struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	CIDR    string   `json:"cidr"`
	Subnets []Subnet `json:"subnets"`
}

// Subnet represents a subnet within a network.
type Subnet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	CIDR string `json:"cidr"`
}

// SecurityGroup represents a security group with its rules.
type SecurityGroup struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Rules []SecurityRule `json:"rules"`
}

// SecurityRule represents a single security group rule.
type SecurityRule struct {
	ID          string `json:"id"`
	Direction   string `json:"direction"`
	Protocol    string `json:"protocol"`
	PortRange   string `json:"port_range"`
	RemoteIP    string `json:"remote_ip"`
	Description string `json:"description"`
}

// LoadBalancer represents a load balancer resource.
type LoadBalancer struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	VIP      string   `json:"vip"`
	Members  []string `json:"members"`
	Protocol string   `json:"protocol"`
	Port     int      `json:"port"`
}

// Volume represents a block storage volume.
type Volume struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Size       int    `json:"size"`
	Status     string `json:"status"`
	AttachedTo string `json:"attached_to"`
}

// FloatingIP represents a public IP address.
type FloatingIP struct {
	ID         string `json:"id"`
	Address    string `json:"address"`
	Status     string `json:"status"`
	AttachedTo string `json:"attached_to"`
}

// DriftReport represents the result of drift detection between desired and actual state.
type DriftReport struct {
	// ClusterName is the name of the cluster being checked
	ClusterName string `json:"cluster_name"`

	// DetectedAt is when the drift was detected
	DetectedAt string `json:"detected_at"`

	// Drifts is the list of all detected drift items
	Drifts []DriftItem `json:"drifts"`

	// Summary provides aggregate statistics
	Summary DriftSummary `json:"summary"`

	// OverallSeverity is the highest severity level found
	OverallSeverity Severity `json:"overall_severity"`

	// Reconcilable indicates if all drift can be automatically fixed
	Reconcilable bool `json:"reconcilable"`
}

// DriftItem represents a single drift between desired and actual state.
type DriftItem struct {
	// ResourceType is the type of resource (server, network, etc.)
	ResourceType string `json:"resource_type"`

	// ResourceID is the unique identifier of the resource
	ResourceID string `json:"resource_id"`

	// ResourceName is the human-readable name of the resource
	ResourceName string `json:"resource_name"`

	// Field is the specific field that has drifted
	Field string `json:"field"`

	// Expected is the desired value from configuration
	Expected interface{} `json:"expected"`

	// Actual is the current value from the provider
	Actual interface{} `json:"actual"`

	// Severity indicates how critical this drift is
	Severity Severity `json:"severity"`

	// Reconcilable indicates if this drift can be automatically fixed
	Reconcilable bool `json:"reconcilable"`

	// Message provides additional context about the drift
	Message string `json:"message"`
}

// DriftSummary provides aggregate statistics about detected drift.
type DriftSummary struct {
	// TotalDrifts is the total number of drift items
	TotalDrifts int `json:"total_drifts"`

	// CriticalCount is the number of critical severity drifts
	CriticalCount int `json:"critical_count"`

	// WarningCount is the number of warning severity drifts
	WarningCount int `json:"warning_count"`

	// InfoCount is the number of info severity drifts
	InfoCount int `json:"info_count"`

	// ReconcilableCount is the number of drifts that can be auto-fixed
	ReconcilableCount int `json:"reconcilable_count"`
}

// Severity represents the severity level of a drift item.
type Severity int

const (
	// SeverityInfo represents informational drift (metadata, timestamps)
	SeverityInfo Severity = iota

	// SeverityWarning represents warning-level drift (worker nodes, tags, labels)
	SeverityWarning

	// SeverityCritical represents critical drift (control plane, network configuration)
	SeverityCritical
)

// String returns the string representation of severity.
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

// CloudProviderFactory creates cloud provider instances for drift-detection workflows.
// It maintains a registry of providers that implement CloudProvider and are queried by
// commands such as cluster drift.
//
// Lifecycle deploy providers are intentionally outside this factory. For example,
// the Kind deploy provider shells out to kind/kubectl and is wired directly into
// cluster deploy/destroy flows rather than the drift-detection registry.
type CloudProviderFactory struct {
	providers map[string]CloudProvider
}

// NewCloudProviderFactory creates a new cloud provider factory with all available providers registered.
func NewCloudProviderFactory() *CloudProviderFactory {
	return &CloudProviderFactory{
		providers: make(map[string]CloudProvider),
	}
}

// RegisterProvider registers a cloud provider with the factory.
// This allows providers to be registered at initialization time.
func (f *CloudProviderFactory) RegisterProvider(name string, provider CloudProvider) {
	f.providers[name] = provider
}

// GetProvider returns a cloud provider instance for the given provider name.
// Returns an error if the provider is not supported.
func (f *CloudProviderFactory) GetProvider(name string) (CloudProvider, error) {
	provider, ok := f.providers[name]
	if !ok {
		return nil, &UnsupportedProviderError{
			Provider:           name,
			SupportedProviders: f.getSupportedProviders(),
		}
	}
	return provider, nil
}

// getSupportedProviders returns a list of all registered provider names.
func (f *CloudProviderFactory) getSupportedProviders() []string {
	providers := make([]string, 0, len(f.providers))
	for name := range f.providers {
		providers = append(providers, name)
	}
	return providers
}

// UnsupportedProviderError is returned when a requested provider is not available.
type UnsupportedProviderError struct {
	Provider           string
	SupportedProviders []string
}

// Error implements the error interface.
func (e *UnsupportedProviderError) Error() string {
	return fmt.Sprintf("unsupported cloud provider: %s (supported: %v)", e.Provider, e.SupportedProviders)
}
