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
	"context"
	"testing"
)

func TestVRRPValidation(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	t.Run("VRRP validation fails when use_octavia=false, vrrp_enabled=true, and vrrp_ip is empty", func(t *testing.T) {
		config := &Config{
			OpenCenter: SimplifiedOpenCenter{
				Meta: ClusterMeta{
					Name:         "test-cluster",
					Organization: "test-org",
				},
				Cluster: ClusterConfig{
					ClusterName: "test-cluster",
					Kubernetes: KubernetesConfig{
						MasterCount: 3,
						NetworkPlugin: NetworkPlugin{
							Calico: CalicoConfig{
								Enabled: true,
							},
						},
						Networking: Networking{
							SubnetNodes:    "10.0.0.0/24",
							SubnetPods:     "10.244.0.0/16",
							SubnetServices: "10.96.0.0/12",
							UseOctavia:     false,
							VRRPEnabled:    true,
							VRRPIP:         "",
						},
					},
				},
				Infrastructure: Infrastructure{
					Provider: "openstack",
					Cloud: CloudConfig{
						OpenStack: SimplifiedOpenStackCloud{
							AuthURL:                     "https://identity.example.com/v3",
							Region:                      "RegionOne",
							TenantName:                  "admin",
							Domain:                      "Default",
							ApplicationCredentialID:     "12345678-1234-1234-1234-123456789abc",
							ApplicationCredentialSecret: "test-cred-secret",
							Networking: OpenStackNetworkingConfig{
								FloatingNetworkId: "87654321-4321-4321-4321-cba987654321",
							},
						},
					},
				},
				GitOps: GitOpsConfig{
					GitDir: "/tmp/test",
				},
			},
		}

		result := validator.Validate(ctx, config)
		if result.Valid {
			t.Error("Expected validation to fail, but it passed")
		}

		// Check if VRRP validation error is present
		found := false
		for _, err := range result.Errors {
			if err.Field == "networking.vrrp_ip" {
				found = true
				if err.Message != "vrrp_ip must be set when use_octavia is false" {
					t.Errorf("Expected error message 'vrrp_ip must be set when use_octavia is false', got '%s'", err.Message)
				}
				break
			}
		}

		if !found {
			t.Error("Expected VRRP validation error, but it was not found")
			t.Logf("Errors found: %d", len(result.Errors))
			for _, err := range result.Errors {
				t.Logf("  - Field: %s, Message: %s", err.Field, err.Message)
			}
		}
	})

	t.Run("VRRP validation passes when use_octavia=false, vrrp_enabled=true, and vrrp_ip is set", func(t *testing.T) {
		config := &Config{
			OpenCenter: SimplifiedOpenCenter{
				Meta: ClusterMeta{
					Name:         "test-cluster",
					Organization: "test-org",
				},
				Cluster: ClusterConfig{
					ClusterName: "test-cluster",
					Kubernetes: KubernetesConfig{
						MasterCount: 3,
						NetworkPlugin: NetworkPlugin{
							Calico: CalicoConfig{
								Enabled: true,
							},
						},
						Networking: Networking{
							SubnetNodes:    "10.0.0.0/24",
							SubnetPods:     "10.244.0.0/16",
							SubnetServices: "10.96.0.0/12",
							UseOctavia:     false,
							VRRPEnabled:    true,
							VRRPIP:         "10.0.4.10",
						},
					},
				},
				Infrastructure: Infrastructure{
					Provider: "openstack",
					Cloud: CloudConfig{
						OpenStack: SimplifiedOpenStackCloud{
							AuthURL:                     "https://identity.example.com/v3",
							Region:                      "RegionOne",
							TenantName:                  "admin",
							Domain:                      "Default",
							ApplicationCredentialID:     "12345678-1234-1234-1234-123456789abc",
							ApplicationCredentialSecret: "test-cred-secret",
							Networking: OpenStackNetworkingConfig{
								FloatingNetworkId: "87654321-4321-4321-4321-cba987654321",
							},
						},
					},
				},
				GitOps: GitOpsConfig{
					GitDir: "/tmp/test",
				},
			},
		}

		result := validator.Validate(ctx, config)

		// Check if VRRP validation error is NOT present
		for _, err := range result.Errors {
			if err.Field == "networking.vrrp_ip" {
				t.Errorf("Expected no VRRP validation error, but found: %s", err.Message)
			}
		}
	})

	t.Run("VRRP validation passes when use_octavia=true", func(t *testing.T) {
		config := &Config{
			OpenCenter: SimplifiedOpenCenter{
				Meta: ClusterMeta{
					Name:         "test-cluster",
					Organization: "test-org",
				},
				Cluster: ClusterConfig{
					ClusterName: "test-cluster",
					Kubernetes: KubernetesConfig{
						MasterCount: 3,
						NetworkPlugin: NetworkPlugin{
							Calico: CalicoConfig{
								Enabled: true,
							},
						},
						Networking: Networking{
							SubnetNodes:    "10.0.0.0/24",
							SubnetPods:     "10.244.0.0/16",
							SubnetServices: "10.96.0.0/12",
							UseOctavia:     true,
							VRRPEnabled:    true,
							VRRPIP:         "",
						},
					},
				},
				Infrastructure: Infrastructure{
					Provider: "openstack",
					Cloud: CloudConfig{
						OpenStack: SimplifiedOpenStackCloud{
							AuthURL:                     "https://identity.example.com/v3",
							Region:                      "RegionOne",
							TenantName:                  "admin",
							Domain:                      "Default",
							ApplicationCredentialID:     "12345678-1234-1234-1234-123456789abc",
							ApplicationCredentialSecret: "test-cred-secret",
							Networking: OpenStackNetworkingConfig{
								FloatingNetworkId: "87654321-4321-4321-4321-cba987654321",
							},
						},
					},
				},
				GitOps: GitOpsConfig{
					GitDir: "/tmp/test",
				},
			},
		}

		result := validator.Validate(ctx, config)

		// Check if VRRP validation error is NOT present
		for _, err := range result.Errors {
			if err.Field == "networking.vrrp_ip" {
				t.Errorf("Expected no VRRP validation error when use_octavia=true, but found: %s", err.Message)
			}
		}
	})

	t.Run("VRRP validation passes when vrrp_enabled=false", func(t *testing.T) {
		config := &Config{
			OpenCenter: SimplifiedOpenCenter{
				Meta: ClusterMeta{
					Name:         "test-cluster",
					Organization: "test-org",
				},
				Cluster: ClusterConfig{
					ClusterName: "test-cluster",
					Kubernetes: KubernetesConfig{
						MasterCount: 3,
						NetworkPlugin: NetworkPlugin{
							Calico: CalicoConfig{
								Enabled: true,
							},
						},
						Networking: Networking{
							SubnetNodes:    "10.0.0.0/24",
							SubnetPods:     "10.244.0.0/16",
							SubnetServices: "10.96.0.0/12",
							UseOctavia:     false,
							VRRPEnabled:    false,
							VRRPIP:         "",
						},
					},
				},
				Infrastructure: Infrastructure{
					Provider: "openstack",
					Cloud: CloudConfig{
						OpenStack: SimplifiedOpenStackCloud{
							AuthURL:                     "https://identity.example.com/v3",
							Region:                      "RegionOne",
							TenantName:                  "admin",
							Domain:                      "Default",
							ApplicationCredentialID:     "12345678-1234-1234-1234-123456789abc",
							ApplicationCredentialSecret: "test-cred-secret",
							Networking: OpenStackNetworkingConfig{
								FloatingNetworkId: "87654321-4321-4321-4321-cba987654321",
							},
						},
					},
				},
				GitOps: GitOpsConfig{
					GitDir: "/tmp/test",
				},
			},
		}

		result := validator.Validate(ctx, config)

		// Check if VRRP validation error is NOT present
		for _, err := range result.Errors {
			if err.Field == "networking.vrrp_ip" {
				t.Errorf("Expected no VRRP validation error when vrrp_enabled=false, but found: %s", err.Message)
			}
		}
	})
}
