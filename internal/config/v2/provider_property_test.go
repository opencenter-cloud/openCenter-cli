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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 13: Multiple Provider Section Rejection
// For any configuration with more than one provider section populated (e.g., both
// `infrastructure.cloud.openstack` and `infrastructure.cloud.aws` have values),
// the validator must reject the configuration.
// **Validates: Requirements 4.7**
func TestProperty_MultipleProviderSectionRejection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("single provider section is valid", prop.ForAll(
		func(provider string) bool {
			cfg := &InfrastructureConfig{
				Provider: provider,
				SSH: SSHConfig{
					AuthorizedKeys: []string{"ssh-rsa AAAAB3NzaC1yc2E..."},
				},
				OSVersion: "24",
				Networking: NetworkingConfig{
					SubnetNodes:          "10.2.128.0/22",
					AllocationPoolStart:  "10.2.128.10",
					AllocationPoolEnd:    "10.2.131.254",
					LoadbalancerProvider: "ovn",
					DNSZoneName:          "cluster.local",
					DNSNameservers:       []string{"8.8.8.8"},
					NTPServers:           []string{"time.google.com"},
				},
				Compute: ComputeConfig{
					FlavorMaster: "m1.medium",
					FlavorWorker: "m1.large",
					MasterCount:  3,
					WorkerCount:  3,
				},
				Storage: StorageConfig{
					DefaultStorageClass:         "standard",
					WorkerVolumeSize:            100,
					WorkerVolumeDestinationType: "volume",
					WorkerVolumeSourceType:      "image",
					WorkerVolumeType:            "ssd",
				},
			}

			// Set only the matching provider section
			switch provider {
			case "openstack":
				cfg.Cloud = CloudConfig{
					OpenStack: &OpenStackCloudConfig{
						AuthURL:   "https://identity.api.rackspacecloud.com/v3",
						Region:    "sjc3",
						ProjectID: "project-123",
						ImageID:   "image-456",
						NetworkID: "network-789",
					},
				}
			case "aws":
				cfg.Cloud = CloudConfig{
					AWS: &AWSCloudConfig{
						Region:    "us-east-1",
						VPCID:     "vpc-123",
						SubnetIDs: []string{"subnet-456"},
						AMIID:     "ami-789",
					},
				}
			case "gcp":
				cfg.Cloud = CloudConfig{
					GCP: &GCPCloudConfig{
						Project:     "project-123",
						Region:      "us-central1",
						Network:     "default",
						Subnetwork:  "default",
						ImageFamily: "ubuntu-2204-lts",
					},
				}
			case "azure":
				cfg.Cloud = CloudConfig{
					Azure: &AzureCloudConfig{
						SubscriptionID: "sub-123",
						ResourceGroup:  "rg-123",
						Location:       "eastus",
						VNetName:       "vnet-123",
						SubnetName:     "subnet-123",
						ImageReference: "ubuntu-22.04",
					},
				}
			}

			// Validate using provider validator
			providerValidator, err := GetProvider(provider)
			if err != nil {
				return false
			}

			err = providerValidator.ValidateConfig(cfg)
			return err == nil
		},
		gen.OneConstOf("openstack", "aws", "gcp", "azure"),
	))

	properties.Property("multiple provider sections are rejected", prop.ForAll(
		func(provider string) bool {
			cfg := &InfrastructureConfig{
				Provider: provider,
				SSH: SSHConfig{
					AuthorizedKeys: []string{"ssh-rsa AAAAB3NzaC1yc2E..."},
				},
				OSVersion: "24",
				Networking: NetworkingConfig{
					SubnetNodes:          "10.2.128.0/22",
					AllocationPoolStart:  "10.2.128.10",
					AllocationPoolEnd:    "10.2.131.254",
					LoadbalancerProvider: "ovn",
					DNSZoneName:          "cluster.local",
					DNSNameservers:       []string{"8.8.8.8"},
					NTPServers:           []string{"time.google.com"},
				},
				Compute: ComputeConfig{
					FlavorMaster: "m1.medium",
					FlavorWorker: "m1.large",
					MasterCount:  3,
					WorkerCount:  3,
				},
				Storage: StorageConfig{
					DefaultStorageClass:         "standard",
					WorkerVolumeSize:            100,
					WorkerVolumeDestinationType: "volume",
					WorkerVolumeSourceType:      "image",
					WorkerVolumeType:            "ssd",
				},
			}

			// Populate multiple provider sections (invalid)
			cfg.Cloud = CloudConfig{
				OpenStack: &OpenStackCloudConfig{
					AuthURL:   "https://identity.api.rackspacecloud.com/v3",
					Region:    "sjc3",
					ProjectID: "project-123",
					ImageID:   "image-456",
					NetworkID: "network-789",
				},
				AWS: &AWSCloudConfig{
					Region:    "us-east-1",
					VPCID:     "vpc-123",
					SubnetIDs: []string{"subnet-456"},
					AMIID:     "ami-789",
				},
			}

			// Validate using provider validator
			providerValidator, err := GetProvider(provider)
			if err != nil {
				return false
			}

			err = providerValidator.ValidateConfig(cfg)
			// Should return error because multiple sections are populated
			return err != nil
		},
		gen.OneConstOf("openstack", "aws", "gcp", "azure"),
	))

	properties.Property("provider mismatch is rejected", prop.ForAll(
		func(declaredProvider, actualProvider string) bool {
			if declaredProvider == actualProvider {
				return true // Skip same provider
			}

			cfg := &InfrastructureConfig{
				Provider: declaredProvider,
				SSH: SSHConfig{
					AuthorizedKeys: []string{"ssh-rsa AAAAB3NzaC1yc2E..."},
				},
				OSVersion: "24",
				Networking: NetworkingConfig{
					SubnetNodes:          "10.2.128.0/22",
					AllocationPoolStart:  "10.2.128.10",
					AllocationPoolEnd:    "10.2.131.254",
					LoadbalancerProvider: "ovn",
					DNSZoneName:          "cluster.local",
					DNSNameservers:       []string{"8.8.8.8"},
					NTPServers:           []string{"time.google.com"},
				},
				Compute: ComputeConfig{
					FlavorMaster: "m1.medium",
					FlavorWorker: "m1.large",
					MasterCount:  3,
					WorkerCount:  3,
				},
				Storage: StorageConfig{
					DefaultStorageClass:         "standard",
					WorkerVolumeSize:            100,
					WorkerVolumeDestinationType: "volume",
					WorkerVolumeSourceType:      "image",
					WorkerVolumeType:            "ssd",
				},
			}

			// Set cloud config for actualProvider (different from declared)
			switch actualProvider {
			case "openstack":
				cfg.Cloud = CloudConfig{
					OpenStack: &OpenStackCloudConfig{
						AuthURL:   "https://identity.api.rackspacecloud.com/v3",
						Region:    "sjc3",
						ProjectID: "project-123",
						ImageID:   "image-456",
						NetworkID: "network-789",
					},
				}
			case "aws":
				cfg.Cloud = CloudConfig{
					AWS: &AWSCloudConfig{
						Region:    "us-east-1",
						VPCID:     "vpc-123",
						SubnetIDs: []string{"subnet-456"},
						AMIID:     "ami-789",
					},
				}
			case "gcp":
				cfg.Cloud = CloudConfig{
					GCP: &GCPCloudConfig{
						Project:     "project-123",
						Region:      "us-central1",
						Network:     "default",
						Subnetwork:  "default",
						ImageFamily: "ubuntu-2204-lts",
					},
				}
			}

			// Validate using declared provider validator
			providerValidator, err := GetProvider(declaredProvider)
			if err != nil {
				return false
			}

			err = providerValidator.ValidateConfig(cfg)
			// Should return error because provider mismatch
			return err != nil
		},
		gen.OneConstOf("openstack", "aws", "gcp"),
		gen.OneConstOf("openstack", "aws", "gcp"),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
