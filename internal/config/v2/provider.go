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

package v2

import (
	"fmt"
)

// Provider interface defines provider-specific validation.
// Requirements: 4.3, 4.4, 4.5, 4.6, 4.8
type Provider interface {
	ValidateConfig(cfg *InfrastructureConfig) error
	GetProviderName() string
}

// OpenStackProvider implements provider validation for OpenStack.
// Requirements: 4.3
type OpenStackProvider struct{}

// ValidateConfig validates OpenStack-specific configuration.
func (p *OpenStackProvider) ValidateConfig(cfg *InfrastructureConfig) error {
	if cfg.Provider != "openstack" {
		return fmt.Errorf("provider mismatch: expected openstack, got %s", cfg.Provider)
	}

	if cfg.Cloud.OpenStack == nil {
		return fmt.Errorf("infrastructure.cloud.openstack is required when provider is openstack")
	}

	os := cfg.Cloud.OpenStack

	// Validate required fields
	if os.AuthURL == "" {
		return fmt.Errorf("infrastructure.cloud.openstack.auth_url is required")
	}
	if os.Region == "" {
		return fmt.Errorf("infrastructure.cloud.openstack.region is required")
	}
	if os.ProjectID == "" {
		return fmt.Errorf("infrastructure.cloud.openstack.project_id is required")
	}
	if os.ImageID == "" {
		return fmt.Errorf("infrastructure.cloud.openstack.image_id is required")
	}
	if os.NetworkID == "" {
		return fmt.Errorf("infrastructure.cloud.openstack.network_id is required")
	}

	// Validate that only OpenStack section is populated
	if cfg.Cloud.AWS != nil && !isEmptyAWSConfig(cfg.Cloud.AWS) {
		return fmt.Errorf("infrastructure.cloud.aws must be empty when provider is openstack")
	}
	if cfg.Cloud.GCP != nil && !isEmptyGCPConfig(cfg.Cloud.GCP) {
		return fmt.Errorf("infrastructure.cloud.gcp must be empty when provider is openstack")
	}
	if cfg.Cloud.Azure != nil && !isEmptyAzureConfig(cfg.Cloud.Azure) {
		return fmt.Errorf("infrastructure.cloud.azure must be empty when provider is openstack")
	}
	if cfg.Cloud.VMware != nil && !isEmptyVMwareConfig(cfg.Cloud.VMware) {
		return fmt.Errorf("infrastructure.cloud.vmware must be empty when provider is openstack")
	}

	return nil
}

// GetProviderName returns the provider name.
func (p *OpenStackProvider) GetProviderName() string {
	return "openstack"
}

// AWSProvider implements provider validation for AWS.
// Requirements: 4.4
type AWSProvider struct{}

// ValidateConfig validates AWS-specific configuration.
func (p *AWSProvider) ValidateConfig(cfg *InfrastructureConfig) error {
	if cfg.Provider != "aws" {
		return fmt.Errorf("provider mismatch: expected aws, got %s", cfg.Provider)
	}

	if cfg.Cloud.AWS == nil {
		return fmt.Errorf("infrastructure.cloud.aws is required when provider is aws")
	}

	aws := cfg.Cloud.AWS

	// Validate required fields
	if aws.Region == "" {
		return fmt.Errorf("infrastructure.cloud.aws.region is required")
	}
	if aws.VPCID == "" {
		return fmt.Errorf("infrastructure.cloud.aws.vpc_id is required")
	}
	if len(aws.SubnetIDs) == 0 {
		return fmt.Errorf("infrastructure.cloud.aws.subnet_ids is required")
	}
	if aws.AMIID == "" {
		return fmt.Errorf("infrastructure.cloud.aws.ami_id is required")
	}

	// Validate that only AWS section is populated
	if cfg.Cloud.OpenStack != nil && !isEmptyOpenStackConfig(cfg.Cloud.OpenStack) {
		return fmt.Errorf("infrastructure.cloud.openstack must be empty when provider is aws")
	}
	if cfg.Cloud.GCP != nil && !isEmptyGCPConfig(cfg.Cloud.GCP) {
		return fmt.Errorf("infrastructure.cloud.gcp must be empty when provider is aws")
	}
	if cfg.Cloud.Azure != nil && !isEmptyAzureConfig(cfg.Cloud.Azure) {
		return fmt.Errorf("infrastructure.cloud.azure must be empty when provider is aws")
	}
	if cfg.Cloud.VMware != nil && !isEmptyVMwareConfig(cfg.Cloud.VMware) {
		return fmt.Errorf("infrastructure.cloud.vmware must be empty when provider is aws")
	}

	return nil
}

// GetProviderName returns the provider name.
func (p *AWSProvider) GetProviderName() string {
	return "aws"
}

// GCPProvider implements provider validation for GCP.
// Requirements: 4.5
type GCPProvider struct{}

// ValidateConfig validates GCP-specific configuration.
func (p *GCPProvider) ValidateConfig(cfg *InfrastructureConfig) error {
	if cfg.Provider != "gcp" {
		return fmt.Errorf("provider mismatch: expected gcp, got %s", cfg.Provider)
	}

	if cfg.Cloud.GCP == nil {
		return fmt.Errorf("infrastructure.cloud.gcp is required when provider is gcp")
	}

	gcp := cfg.Cloud.GCP

	// Validate required fields
	if gcp.Project == "" {
		return fmt.Errorf("infrastructure.cloud.gcp.project is required")
	}
	if gcp.Region == "" {
		return fmt.Errorf("infrastructure.cloud.gcp.region is required")
	}
	if gcp.Network == "" {
		return fmt.Errorf("infrastructure.cloud.gcp.network is required")
	}
	if gcp.Subnetwork == "" {
		return fmt.Errorf("infrastructure.cloud.gcp.subnetwork is required")
	}
	if gcp.ImageFamily == "" {
		return fmt.Errorf("infrastructure.cloud.gcp.image_family is required")
	}

	// Validate that only GCP section is populated
	if cfg.Cloud.OpenStack != nil && !isEmptyOpenStackConfig(cfg.Cloud.OpenStack) {
		return fmt.Errorf("infrastructure.cloud.openstack must be empty when provider is gcp")
	}
	if cfg.Cloud.AWS != nil && !isEmptyAWSConfig(cfg.Cloud.AWS) {
		return fmt.Errorf("infrastructure.cloud.aws must be empty when provider is gcp")
	}
	if cfg.Cloud.Azure != nil && !isEmptyAzureConfig(cfg.Cloud.Azure) {
		return fmt.Errorf("infrastructure.cloud.azure must be empty when provider is gcp")
	}
	if cfg.Cloud.VMware != nil && !isEmptyVMwareConfig(cfg.Cloud.VMware) {
		return fmt.Errorf("infrastructure.cloud.vmware must be empty when provider is gcp")
	}

	return nil
}

// GetProviderName returns the provider name.
func (p *GCPProvider) GetProviderName() string {
	return "gcp"
}

// AzureProvider implements provider validation for Azure.
// Requirements: 4.6
type AzureProvider struct{}

// ValidateConfig validates Azure-specific configuration.
func (p *AzureProvider) ValidateConfig(cfg *InfrastructureConfig) error {
	if cfg.Provider != "azure" {
		return fmt.Errorf("provider mismatch: expected azure, got %s", cfg.Provider)
	}

	if cfg.Cloud.Azure == nil {
		return fmt.Errorf("infrastructure.cloud.azure is required when provider is azure")
	}

	azure := cfg.Cloud.Azure

	// Validate required fields
	if azure.SubscriptionID == "" {
		return fmt.Errorf("infrastructure.cloud.azure.subscription_id is required")
	}
	if azure.ResourceGroup == "" {
		return fmt.Errorf("infrastructure.cloud.azure.resource_group is required")
	}
	if azure.Location == "" {
		return fmt.Errorf("infrastructure.cloud.azure.location is required")
	}
	if azure.VNetName == "" {
		return fmt.Errorf("infrastructure.cloud.azure.vnet_name is required")
	}
	if azure.SubnetName == "" {
		return fmt.Errorf("infrastructure.cloud.azure.subnet_name is required")
	}
	if azure.ImageReference == "" {
		return fmt.Errorf("infrastructure.cloud.azure.image_reference is required")
	}

	// Validate that only Azure section is populated
	if cfg.Cloud.OpenStack != nil && !isEmptyOpenStackConfig(cfg.Cloud.OpenStack) {
		return fmt.Errorf("infrastructure.cloud.openstack must be empty when provider is azure")
	}
	if cfg.Cloud.AWS != nil && !isEmptyAWSConfig(cfg.Cloud.AWS) {
		return fmt.Errorf("infrastructure.cloud.aws must be empty when provider is azure")
	}
	if cfg.Cloud.GCP != nil && !isEmptyGCPConfig(cfg.Cloud.GCP) {
		return fmt.Errorf("infrastructure.cloud.gcp must be empty when provider is azure")
	}
	if cfg.Cloud.VMware != nil && !isEmptyVMwareConfig(cfg.Cloud.VMware) {
		return fmt.Errorf("infrastructure.cloud.vmware must be empty when provider is azure")
	}

	return nil
}

// GetProviderName returns the provider name.
func (p *AzureProvider) GetProviderName() string {
	return "azure"
}

// GetProvider returns the appropriate provider validator for the given provider name.
func GetProvider(providerName string) (Provider, error) {
	switch providerName {
	case "openstack":
		return &OpenStackProvider{}, nil
	case "aws":
		return &AWSProvider{}, nil
	case "gcp":
		return &GCPProvider{}, nil
	case "azure":
		return &AzureProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// Helper functions to check if provider configs are empty

func isEmptyOpenStackConfig(cfg *OpenStackCloudConfig) bool {
	return cfg.AuthURL == "" && cfg.Region == "" && cfg.ProjectID == "" && cfg.ImageID == "" && cfg.NetworkID == ""
}

func isEmptyAWSConfig(cfg *AWSCloudConfig) bool {
	return cfg.Region == "" && cfg.VPCID == "" && len(cfg.SubnetIDs) == 0 && cfg.AMIID == ""
}

func isEmptyGCPConfig(cfg *GCPCloudConfig) bool {
	return cfg.Project == "" && cfg.Region == "" && cfg.Network == "" && cfg.Subnetwork == "" && cfg.ImageFamily == ""
}

func isEmptyAzureConfig(cfg *AzureCloudConfig) bool {
	return cfg.SubscriptionID == "" && cfg.ResourceGroup == "" && cfg.Location == "" && cfg.VNetName == "" && cfg.SubnetName == "" && cfg.ImageReference == ""
}

func isEmptyVMwareConfig(cfg *VMwareCloudConfig) bool {
	return cfg.VCenterServer == "" && cfg.Datacenter == "" && cfg.Datastore == "" && cfg.Network == "" && cfg.Template == ""
}
