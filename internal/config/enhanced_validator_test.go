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

func TestEnhancedConfigValidator_ValidateComprehensive(t *testing.T) {
	validator := NewEnhancedConfigValidator(false)
	ctx := context.Background()

	tests := []struct {
		name           string
		config         *Config
		expectValid    bool
		expectErrors   int
		expectWarnings int
	}{
		{
			name:           "nil config",
			config:         nil,
			expectValid:    false,
			expectErrors:   1,
			expectWarnings: 0,
		},
		{
			name: "valid basic config",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Meta: ClusterMeta{
						Name: "test-cluster",
					},
					Infrastructure: Infrastructure{
						Provider: "kind",
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
			},
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 1, // Unknown provider warning for kind
		},
		{
			name: "invalid config - empty cluster name",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Infrastructure: Infrastructure{
						Provider: "openstack",
					},
					Cluster: ClusterConfig{
						ClusterName: "", // Invalid
						Kubernetes: KubernetesConfig{
							Version:     "1.31.4",
							MasterCount: 1,
							WorkerCount: 1,
						},
					},
					GitOps: GitOpsConfig{
						GitDir: "/tmp/test-gitops",
					},
				},
			},
			expectValid:    false,
			expectErrors:   8, // Comprehensive validation includes OpenStack config errors
			expectWarnings: 3, // SSH keys, subnets warnings
		},
		{
			name: "invalid config - multiple network plugins",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Infrastructure: Infrastructure{
						Provider: "openstack",
					},
					Cluster: ClusterConfig{
						ClusterName: "test-cluster",
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
			},
			expectValid:    false,
			expectErrors:   7, // Multiple network plugins + OpenStack config errors
			expectWarnings: 4, // SSH keys, subnets, Calico CNI interface warnings
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateComprehensive(ctx, tt.config)

			if result.Valid != tt.expectValid {
				t.Errorf("ValidateComprehensive() valid = %v, want %v", result.Valid, tt.expectValid)
			}

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("ValidateComprehensive() errors = %d, want %d", len(result.Errors), tt.expectErrors)
				for i, err := range result.Errors {
					t.Logf("Error %d: %s - %s", i, err.Field, err.Message)
				}
			}

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("ValidateComprehensive() warnings = %d, want %d", len(result.Warnings), tt.expectWarnings)
				for i, warning := range result.Warnings {
					t.Logf("Warning %d: %s - %s", i, warning.Field, warning.Message)
				}
			}
		})
	}
}

func TestEnhancedConfigValidator_ValidateCredentials(t *testing.T) {
	validator := NewEnhancedConfigValidator(false)
	ctx := context.Background()

	tests := []struct {
		name         string
		config       *Config
		expectErrors int
	}{
		{
			name: "valid OpenStack credentials",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Infrastructure: Infrastructure{
						Provider: "openstack",
						Cloud: CloudConfig{
							OpenStack: SimplifiedOpenStackCloud{
								ApplicationCredentialID:     "12345678-1234-1234-1234-123456789012",
								ApplicationCredentialSecret: "valid-secret",
							},
						},
					},
				},
			},
			expectErrors: 0,
		},
		{
			name: "invalid OpenStack credentials - missing secret",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Infrastructure: Infrastructure{
						Provider: "openstack",
						Cloud: CloudConfig{
							OpenStack: SimplifiedOpenStackCloud{
								ApplicationCredentialID:     "12345678-1234-1234-1234-123456789012",
								ApplicationCredentialSecret: "", // Missing
							},
						},
					},
				},
			},
			expectErrors: 1,
		},
		{
			name: "invalid OpenStack credentials - invalid UUID",
			config: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Infrastructure: Infrastructure{
						Provider: "openstack",
						Cloud: CloudConfig{
							OpenStack: SimplifiedOpenStackCloud{
								ApplicationCredentialID:     "invalid-uuid", // Invalid format
								ApplicationCredentialSecret: "valid-secret",
							},
						},
					},
				},
			},
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test credential validation specifically
			if validator, ok := validator.cloudValidators["openstack"]; ok {
				errors := validator.ValidateCredentials(ctx, tt.config)
				if len(errors) != tt.expectErrors {
					t.Errorf("ValidateCredentials() errors = %d, want %d", len(errors), tt.expectErrors)
					for i, err := range errors {
						t.Logf("Error %d: %s - %s", i, err.Field, err.Message)
					}
				}
			}
		})
	}
}

func TestEnhancedConfigValidator_ConfigValidatorInterface(t *testing.T) {
	validator := NewEnhancedConfigValidator(false)
	ctx := context.Background()

	// Test that EnhancedConfigValidator implements ConfigValidatorInterface
	var _ ConfigValidatorInterface = validator

	config := &Config{
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name: "test-cluster",
			},
			Infrastructure: Infrastructure{
				Provider: "kind",
			},
			Cluster: ClusterConfig{
				ClusterName: "test-cluster",
				Kubernetes: KubernetesConfig{
					Version:     "1.31.4",
					MasterCount: 1,
					WorkerCount: 1,
					NetworkPlugin: NetworkPlugin{
						Calico: CalicoConfig{
							Enabled: true,
						},
					},
				},
			},
			GitOps: GitOpsConfig{
				GitDir: "/tmp/test-gitops",
			},
		},
	}

	// Test all interface methods
	result := validator.Validate(ctx, config)
	if result == nil {
		t.Error("Validate() returned nil")
	}

	structureResult := validator.ValidateStructure(ctx, config)
	if structureResult == nil {
		t.Error("ValidateStructure() returned nil")
	}

	semanticsResult := validator.ValidateSemantics(ctx, config)
	if semanticsResult == nil {
		t.Error("ValidateSemantics() returned nil")
	}

	networkingResult := validator.ValidateNetworking(ctx, config)
	if networkingResult == nil {
		t.Error("ValidateNetworking() returned nil")
	}

	cloudResult := validator.ValidateCloudProvider(ctx, config)
	if cloudResult == nil {
		t.Error("ValidateCloudProvider() returned nil")
	}
}
