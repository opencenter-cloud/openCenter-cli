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

package openstack

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"

	"github.com/rackerlabs/opencenter-cli/internal/cloud"
	"github.com/rackerlabs/opencenter-cli/internal/config"
)

// Provider implements the CloudProvider interface for OpenStack.
type Provider struct {
	authOpts gophercloud.AuthOptions
}

// NewProvider creates a new OpenStack cloud provider.
func NewProvider(authOpts gophercloud.AuthOptions) *Provider {
	return &Provider{
		authOpts: authOpts,
	}
}

// getProviderClient creates an authenticated OpenStack provider client.
func (p *Provider) getProviderClient() (*gophercloud.ProviderClient, error) {
	client, err := openstack.AuthenticatedClient(p.authOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with OpenStack: %w", err)
	}
	return client, nil
}

// getComputeClient creates a compute service client.
func (p *Provider) getComputeClient() (*gophercloud.ServiceClient, error) {
	provider, err := p.getProviderClient()
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: p.authOpts.DomainName, // Use domain name as region for now
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	return client, nil
}

// getNetworkClient creates a network service client.
func (p *Provider) getNetworkClient() (*gophercloud.ServiceClient, error) {
	provider, err := p.getProviderClient()
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Region: p.authOpts.DomainName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create network client: %w", err)
	}
	return client, nil
}

// GetCurrentState retrieves the current infrastructure state from OpenStack.
func (p *Provider) GetCurrentState(ctx context.Context, cfg config.Config) (*cloud.InfrastructureState, error) {
	state := &cloud.InfrastructureState{
		Servers:        []cloud.Server{},
		Networks:       []cloud.Network{},
		SecurityGroups: []cloud.SecurityGroup{},
		LoadBalancers:  []cloud.LoadBalancer{},
		Volumes:        []cloud.Volume{},
		FloatingIPs:    []cloud.FloatingIP{},
	}

	// Get cluster name for filtering resources
	clusterName := cfg.OpenCenter.Cluster.ClusterName

	// Retrieve servers
	serverList, err := p.listServers(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	state.Servers = serverList

	// Retrieve networks
	networkList, err := p.listNetworks(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}
	state.Networks = networkList

	// TODO: Implement security groups, load balancers, volumes, floating IPs retrieval
	// These will be added as needed for comprehensive drift detection

	return state, nil
}

// listServers retrieves all servers for the cluster.
func (p *Provider) listServers(ctx context.Context, clusterName string) ([]cloud.Server, error) {
	client, err := p.getComputeClient()
	if err != nil {
		return nil, err
	}

	// List all servers with cluster tag
	opts := servers.ListOpts{
		// Filter by cluster name in metadata/tags
		// Note: OpenStack metadata filtering varies by deployment
	}

	allPages, err := servers.List(client, opts).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	serverList, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract servers: %w", err)
	}

	// Convert to our Server type
	result := make([]cloud.Server, 0, len(serverList))
	for _, s := range serverList {
		// Filter by cluster name in metadata
		if clusterTag, ok := s.Metadata["cluster"]; ok && clusterTag == clusterName {
			networkNames := make([]string, 0)
			for netName := range s.Addresses {
				networkNames = append(networkNames, netName)
			}

			result = append(result, cloud.Server{
				ID:       s.ID,
				Name:     s.Name,
				Flavor:   s.Flavor["id"].(string),
				Image:    s.Image["id"].(string),
				Status:   s.Status,
				Networks: networkNames,
				Tags:     s.Metadata,
			})
		}
	}

	return result, nil
}

// listNetworks retrieves all networks for the cluster.
func (p *Provider) listNetworks(ctx context.Context, clusterName string) ([]cloud.Network, error) {
	client, err := p.getNetworkClient()
	if err != nil {
		return nil, err
	}

	// List all networks
	opts := networks.ListOpts{
		// Filter by cluster name in tags
		Tags: clusterName,
	}

	allPages, err := networks.List(client, opts).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	networkList, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract networks: %w", err)
	}

	// Convert to our Network type and fetch subnets
	result := make([]cloud.Network, 0, len(networkList))
	for _, n := range networkList {
		subnetList, err := p.listSubnets(ctx, n.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list subnets for network %s: %w", n.ID, err)
		}

		result = append(result, cloud.Network{
			ID:      n.ID,
			Name:    n.Name,
			CIDR:    "", // Networks don't have CIDR directly, subnets do
			Subnets: subnetList,
		})
	}

	return result, nil
}

// listSubnets retrieves all subnets for a network.
func (p *Provider) listSubnets(ctx context.Context, networkID string) ([]cloud.Subnet, error) {
	client, err := p.getNetworkClient()
	if err != nil {
		return nil, err
	}

	opts := subnets.ListOpts{
		NetworkID: networkID,
	}

	allPages, err := subnets.List(client, opts).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list subnets: %w", err)
	}

	subnetList, err := subnets.ExtractSubnets(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract subnets: %w", err)
	}

	// Convert to our Subnet type
	result := make([]cloud.Subnet, 0, len(subnetList))
	for _, s := range subnetList {
		result = append(result, cloud.Subnet{
			ID:   s.ID,
			Name: s.Name,
			CIDR: s.CIDR,
		})
	}

	return result, nil
}

// DetectDrift compares desired state with actual state and returns a drift report.
func (p *Provider) DetectDrift(ctx context.Context, desired, actual *cloud.InfrastructureState) (*cloud.DriftReport, error) {
	report := &cloud.DriftReport{
		Drifts:  []cloud.DriftItem{},
		Summary: cloud.DriftSummary{},
	}

	// Detect server drift
	p.detectServerDrift(desired, actual, report)

	// Detect network drift
	p.detectNetworkDrift(desired, actual, report)

	// Calculate summary
	p.calculateSummary(report)

	return report, nil
}

// detectServerDrift compares desired and actual servers.
func (p *Provider) detectServerDrift(desired, actual *cloud.InfrastructureState, report *cloud.DriftReport) {
	// Create maps for easier lookup
	desiredServers := make(map[string]cloud.Server)
	for _, s := range desired.Servers {
		desiredServers[s.Name] = s
	}

	actualServers := make(map[string]cloud.Server)
	for _, s := range actual.Servers {
		actualServers[s.Name] = s
	}

	// Check for missing servers (in desired but not in actual)
	for name, desiredServer := range desiredServers {
		if _, exists := actualServers[name]; !exists {
			report.Drifts = append(report.Drifts, cloud.DriftItem{
				ResourceType: "server",
				ResourceID:   desiredServer.ID,
				ResourceName: name,
				Field:        "existence",
				Expected:     "exists",
				Actual:       "missing",
				Severity:     cloud.SeverityCritical,
				Reconcilable: true,
				Message:      fmt.Sprintf("Server %s is missing from infrastructure", name),
			})
		}
	}

	// Check for extra servers (in actual but not in desired)
	for name, actualServer := range actualServers {
		if _, exists := desiredServers[name]; !exists {
			report.Drifts = append(report.Drifts, cloud.DriftItem{
				ResourceType: "server",
				ResourceID:   actualServer.ID,
				ResourceName: name,
				Field:        "existence",
				Expected:     "not exists",
				Actual:       "exists",
				Severity:     cloud.SeverityWarning,
				Reconcilable: false,
				Message:      fmt.Sprintf("Unexpected server %s found in infrastructure", name),
			})
		}
	}

	// Check for configuration drift in existing servers
	for name, desiredServer := range desiredServers {
		actualServer, exists := actualServers[name]
		if !exists {
			continue
		}

		// Check flavor
		if desiredServer.Flavor != actualServer.Flavor {
			report.Drifts = append(report.Drifts, cloud.DriftItem{
				ResourceType: "server",
				ResourceID:   actualServer.ID,
				ResourceName: name,
				Field:        "flavor",
				Expected:     desiredServer.Flavor,
				Actual:       actualServer.Flavor,
				Severity:     cloud.SeverityCritical,
				Reconcilable: false,
				Message:      fmt.Sprintf("Server %s has wrong flavor", name),
			})
		}

		// Check status
		if actualServer.Status != "ACTIVE" {
			report.Drifts = append(report.Drifts, cloud.DriftItem{
				ResourceType: "server",
				ResourceID:   actualServer.ID,
				ResourceName: name,
				Field:        "status",
				Expected:     "ACTIVE",
				Actual:       actualServer.Status,
				Severity:     cloud.SeverityCritical,
				Reconcilable: false,
				Message:      fmt.Sprintf("Server %s is not active", name),
			})
		}

		// Check tags
		for key, expectedValue := range desiredServer.Tags {
			if actualValue, ok := actualServer.Tags[key]; !ok || actualValue != expectedValue {
				report.Drifts = append(report.Drifts, cloud.DriftItem{
					ResourceType: "server",
					ResourceID:   actualServer.ID,
					ResourceName: name,
					Field:        fmt.Sprintf("tags.%s", key),
					Expected:     expectedValue,
					Actual:       actualValue,
					Severity:     cloud.SeverityInfo,
					Reconcilable: true,
					Message:      fmt.Sprintf("Server %s has incorrect tag %s", name, key),
				})
			}
		}
	}
}

// detectNetworkDrift compares desired and actual networks.
func (p *Provider) detectNetworkDrift(desired, actual *cloud.InfrastructureState, report *cloud.DriftReport) {
	// Create maps for easier lookup
	desiredNetworks := make(map[string]cloud.Network)
	for _, n := range desired.Networks {
		desiredNetworks[n.Name] = n
	}

	actualNetworks := make(map[string]cloud.Network)
	for _, n := range actual.Networks {
		actualNetworks[n.Name] = n
	}

	// Check for missing networks
	for name, desiredNetwork := range desiredNetworks {
		if _, exists := actualNetworks[name]; !exists {
			report.Drifts = append(report.Drifts, cloud.DriftItem{
				ResourceType: "network",
				ResourceID:   desiredNetwork.ID,
				ResourceName: name,
				Field:        "existence",
				Expected:     "exists",
				Actual:       "missing",
				Severity:     cloud.SeverityCritical,
				Reconcilable: true,
				Message:      fmt.Sprintf("Network %s is missing from infrastructure", name),
			})
		}
	}

	// Check for extra networks
	for name, actualNetwork := range actualNetworks {
		if _, exists := desiredNetworks[name]; !exists {
			report.Drifts = append(report.Drifts, cloud.DriftItem{
				ResourceType: "network",
				ResourceID:   actualNetwork.ID,
				ResourceName: name,
				Field:        "existence",
				Expected:     "not exists",
				Actual:       "exists",
				Severity:     cloud.SeverityWarning,
				Reconcilable: false,
				Message:      fmt.Sprintf("Unexpected network %s found in infrastructure", name),
			})
		}
	}

	// Check subnet drift for existing networks
	for name, desiredNetwork := range desiredNetworks {
		actualNetwork, exists := actualNetworks[name]
		if !exists {
			continue
		}

		// Compare subnet counts
		if len(desiredNetwork.Subnets) != len(actualNetwork.Subnets) {
			report.Drifts = append(report.Drifts, cloud.DriftItem{
				ResourceType: "network",
				ResourceID:   actualNetwork.ID,
				ResourceName: name,
				Field:        "subnet_count",
				Expected:     len(desiredNetwork.Subnets),
				Actual:       len(actualNetwork.Subnets),
				Severity:     cloud.SeverityWarning,
				Reconcilable: true,
				Message:      fmt.Sprintf("Network %s has incorrect number of subnets", name),
			})
		}
	}
}

// calculateSummary calculates the drift summary and overall severity.
func (p *Provider) calculateSummary(report *cloud.DriftReport) {
	report.Summary.TotalDrifts = len(report.Drifts)
	report.Reconcilable = true
	report.OverallSeverity = cloud.SeverityInfo

	for _, drift := range report.Drifts {
		switch drift.Severity {
		case cloud.SeverityCritical:
			report.Summary.CriticalCount++
			report.OverallSeverity = cloud.SeverityCritical
		case cloud.SeverityWarning:
			report.Summary.WarningCount++
			if report.OverallSeverity < cloud.SeverityWarning {
				report.OverallSeverity = cloud.SeverityWarning
			}
		case cloud.SeverityInfo:
			report.Summary.InfoCount++
		}

		if drift.Reconcilable {
			report.Summary.ReconcilableCount++
		} else {
			report.Reconcilable = false
		}
	}
}

// ReconcileDrift applies changes to fix detected drift.
func (p *Provider) ReconcileDrift(ctx context.Context, drift *cloud.DriftReport) error {
	if !drift.Reconcilable {
		return fmt.Errorf("drift report contains non-reconcilable items")
	}

	if len(drift.Drifts) == 0 {
		return nil // No drift to reconcile
	}

	// Process each reconcilable drift item
	for _, item := range drift.Drifts {
		if !item.Reconcilable {
			continue
		}

		if err := p.reconcileDriftItem(ctx, item); err != nil {
			return fmt.Errorf("failed to reconcile %s %s: %w", item.ResourceType, item.ResourceName, err)
		}
	}

	return nil
}

// reconcileDriftItem reconciles a single drift item.
func (p *Provider) reconcileDriftItem(ctx context.Context, item cloud.DriftItem) error {
	switch item.ResourceType {
	case "server":
		return p.reconcileServerDrift(ctx, item)
	case "network":
		return p.reconcileNetworkDrift(ctx, item)
	default:
		return fmt.Errorf("unsupported resource type for reconciliation: %s", item.ResourceType)
	}
}

// reconcileServerDrift reconciles server-specific drift.
func (p *Provider) reconcileServerDrift(ctx context.Context, item cloud.DriftItem) error {
	switch item.Field {
	case "existence":
		if item.Actual == "missing" {
			// Server needs to be created
			return fmt.Errorf("server creation not yet implemented")
		}
		return nil

	case "tags":
		// Update server metadata/tags
		client, err := p.getComputeClient()
		if err != nil {
			return err
		}

		// Extract tag key from field (e.g., "tags.cluster" -> "cluster")
		tagKey := item.Field[5:] // Remove "tags." prefix

		updateOpts := servers.MetadataOpts{
			tagKey: item.Expected.(string),
		}

		_, err = servers.UpdateMetadata(client, item.ResourceID, updateOpts).Extract()
		if err != nil {
			return fmt.Errorf("failed to update server metadata: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("unsupported server field for reconciliation: %s", item.Field)
	}
}

// reconcileNetworkDrift reconciles network-specific drift.
func (p *Provider) reconcileNetworkDrift(ctx context.Context, item cloud.DriftItem) error {
	switch item.Field {
	case "existence":
		if item.Actual == "missing" {
			// Network needs to be created
			return fmt.Errorf("network creation not yet implemented")
		}
		return nil

	case "subnet_count":
		// Subnet reconciliation would require more complex logic
		return fmt.Errorf("subnet reconciliation not yet implemented")

	default:
		return fmt.Errorf("unsupported network field for reconciliation: %s", item.Field)
	}
}
