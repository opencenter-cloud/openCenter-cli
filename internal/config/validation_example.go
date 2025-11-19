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
	"fmt"
)

// ExampleEnhancedValidation demonstrates the enhanced validation functionality.
func ExampleEnhancedValidation() {
	// Create an enhanced validator
	validator := NewEnhancedConfigValidator(false)
	ctx := context.Background()

	// Example configuration with some issues
	config := &Config{
		OpenCenter: SimplifiedOpenCenter{
			Infrastructure: Infrastructure{
				Provider: "openstack",
				Cloud: CloudConfig{
					OpenStack: SimplifiedOpenStackCloud{
						AuthURL:                     "https://keystone.example.com/v3/",
						Region:                      "RegionOne",
						TenantName:                  "my-project",
						Domain:                      "Default",
						ApplicationCredentialID:     "invalid-uuid", // Invalid format
						ApplicationCredentialSecret: "",             // Missing
						FloatingNetworkId:           "",             // Missing
					},
				},
			},
			Cluster: ClusterConfig{
				ClusterName: "", // Invalid - empty
				Kubernetes: KubernetesConfig{
					Version:     "1.31.4",
					MasterCount: 1,
					WorkerCount: 1,
					NetworkPlugin: NetworkPlugin{
						Calico: CalicoConfig{
							Enabled: true,
						},
						Cilium: CiliumConfig{
							Enabled: true, // Invalid - multiple plugins
						},
					},
				},
			},
			GitOps: GitOpsConfig{
				GitDir: "/tmp/test-gitops",
			},
		},
	}

	// Perform comprehensive validation
	result := validator.ValidateComprehensive(ctx, config)

	fmt.Printf("Validation Result: Valid = %t\n", result.Valid)
	fmt.Printf("Errors: %d\n", len(result.Errors))
	fmt.Printf("Warnings: %d\n", len(result.Warnings))

	// Display errors with suggestions
	for i, err := range result.Errors {
		fmt.Printf("\nError %d:\n", i+1)
		fmt.Printf("  Type: %s\n", err.Type)
		fmt.Printf("  Field: %s\n", err.Field)
		fmt.Printf("  Message: %s\n", err.Message)
		if len(err.Suggestions) > 0 {
			fmt.Printf("  Suggestions:\n")
			for _, suggestion := range err.Suggestions {
				fmt.Printf("    - %s\n", suggestion)
			}
		}
	}

	// Display warnings
	for i, warning := range result.Warnings {
		fmt.Printf("\nWarning %d:\n", i+1)
		fmt.Printf("  Type: %s\n", warning.Type)
		fmt.Printf("  Field: %s\n", warning.Field)
		fmt.Printf("  Message: %s\n", warning.Message)
		if len(warning.Suggestions) > 0 {
			fmt.Printf("  Suggestions:\n")
			for _, suggestion := range warning.Suggestions {
				fmt.Printf("    - %s\n", suggestion)
			}
		}
	}
}

// ExamplePreFlightValidation demonstrates pre-flight validation with connectivity checks.
func ExamplePreFlightValidation() {
	// Create an enhanced validator
	validator := NewEnhancedConfigValidator(false)
	ctx := context.Background()

	// Example configuration for pre-flight validation
	config := &Config{
		OpenCenter: SimplifiedOpenCenter{
			Infrastructure: Infrastructure{
				Provider: "openstack",
				Cloud: CloudConfig{
					OpenStack: SimplifiedOpenStackCloud{
						AuthURL:                     "https://keystone.example.com/v3/",
						Region:                      "RegionOne",
						TenantName:                  "my-project",
						Domain:                      "Default",
						ApplicationCredentialID:     "12345678-1234-1234-1234-123456789012",
						ApplicationCredentialSecret: "valid-secret",
						FloatingNetworkId:           "87654321-4321-4321-4321-210987654321",
					},
				},
			},
			Cluster: ClusterConfig{
				ClusterName: "test-cluster",
				Kubernetes: KubernetesConfig{
					Version:        "1.31.4",
					MasterCount:    1,
					WorkerCount:    1,
					SubnetPods:     "10.42.0.0/16",
					SubnetServices: "10.43.0.0/16",
					NetworkPlugin: NetworkPlugin{
						Calico: CalicoConfig{
							Enabled:  true,
							CNIIface: "eth0",
						},
					},
				},
				SSHAuthorizedKeys: []string{"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7 test@example.com"},
			},
			GitOps: GitOpsConfig{
				GitDir: "/tmp/test-gitops",
			},
		},
		OpenTofu: SimplifiedOpenTofu{
			Enabled: true,
			Path:    "opentofu",
		},
	}

	// Perform pre-flight validation (includes connectivity checks)
	result := validator.ValidatePreFlight(ctx, config)

	fmt.Printf("Pre-flight Validation Result: Valid = %t\n", result.Valid)
	fmt.Printf("Errors: %d\n", len(result.Errors))
	fmt.Printf("Warnings: %d\n", len(result.Warnings))

	// Display connectivity warnings
	for i, warning := range result.Warnings {
		if warning.Type == "cloud" || warning.Type == "tools" {
			fmt.Printf("\nPre-flight Warning %d:\n", i+1)
			fmt.Printf("  Type: %s\n", warning.Type)
			fmt.Printf("  Field: %s\n", warning.Field)
			fmt.Printf("  Message: %s\n", warning.Message)
		}
	}
}
