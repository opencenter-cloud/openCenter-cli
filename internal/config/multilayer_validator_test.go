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
	"testing"
)

// TestMultiLayerValidator_ValidateSchema tests schema validation
func TestMultiLayerValidator_ValidateSchema(t *testing.T) {
	validator := NewMultiLayerValidator()

	tests := []struct {
		name      string
		cfg       *Config
		wantError bool
		errorCode string
	}{
		{
			name: "valid config",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Meta: ClusterMeta{
						Name:         "test-cluster",
						Organization: "opencenter",
						Region:       "us-east-1",
					},
					Infrastructure: Infrastructure{
						Provider:  "openstack",
						SSHUser:   "ubuntu",
						OSVersion: "24",
						NodeNaming: NodeNaming{
							Worker: "wn",
							Master: "cp",
						},
						Bastion: BastionConfig{
							Address: "localhost",
						},
						Cloud: CloudConfig{},
					},
					Cluster: ClusterConfig{
						ClusterName:       "test-cluster",
						SSHAuthorizedKeys: []string{"ssh-rsa AAAA..."},
						BaseDomain:        "k8s.example.com",
						ClusterFQDN:       "test.k8s.example.com",
						Kubernetes: KubernetesConfig{
							Version:              "1.33.5",
							KubesprayVersion:     "v2.29.1",
							APIPort:              443,
							FlavorBastion:        "gp.0.2.2",
							SubnetPods:           "10.42.0.0/16",
							SubnetServices:       "10.43.0.0/16",
							LoadbalancerProvider: "ovn",
							NetworkPlugin:        NetworkPlugin{},
						},
						Networking: ClusterNetworkingConfig{
							SubnetNodes:          "10.0.0.0/24",
							NTPServers:           []string{"time.example.com"},
							DNSNameservers:       []string{"8.8.8.8"},
							LoadbalancerProvider: "ovn",
						},
					},
					GitOps: GitOpsConfig{
						GitDir:            "./testdata/test-git-repo",
						GitOpsBaseRepo:    "ssh://git@github.com/example/repo.git",
						GitOpsBaseRelease: "v0.1.0",
						GitOpsBranch:      "main",
					},
					Storage: StorageConfig{
						DefaultStorageClass:         "csi-cinder-sc-delete",
						WorkerVolumeSize:            40,
						WorkerVolumeDestinationType: "volume",
						WorkerVolumeSourceType:      "image",
						WorkerVolumeType:            "HA-Standard",
					},
				},
				OpenTofu: SimplifiedOpenTofu{
					Path: "opentofu",
					Backend: SimplifiedTofuBackend{
						Type: "local",
						Local: SimplifiedTofuLocal{
							Path: "./terraform.tfstate",
						},
					},
				},
				Secrets: Secrets{},
			},
			wantError: false,
		},
		{
			name: "missing required field",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Meta: ClusterMeta{
						// Missing Name
						Organization: "opencenter",
						Region:       "us-east-1",
					},
				},
			},
			wantError: true,
			errorCode: "E001",
		},
		{
			name: "invalid CIDR",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Meta: ClusterMeta{
						Name:         "test-cluster",
						Organization: "opencenter",
						Region:       "us-east-1",
					},
					Cluster: ClusterConfig{
						Networking: ClusterNetworkingConfig{
							SubnetNodes: "invalid-cidr",
						},
					},
				},
			},
			wantError: true,
			errorCode: "E003",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateSchema(tt.cfg)

			if tt.wantError && len(errors) == 0 {
				t.Error("ValidateSchema() expected errors but got none")
			}

			if !tt.wantError && len(errors) > 0 {
				t.Errorf("ValidateSchema() unexpected errors: %v", errors)
			}

			if tt.wantError && tt.errorCode != "" {
				found := false
				for _, err := range errors {
					if err.Code == tt.errorCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateSchema() expected error code %s but got: %v", tt.errorCode, errors)
				}
			}
		})
	}
}

// TestMultiLayerValidator_ValidateBusinessRules tests business rule validation
func TestMultiLayerValidator_ValidateBusinessRules(t *testing.T) {
	validator := NewMultiLayerValidator()

	tests := []struct {
		name      string
		cfg       *Config
		wantError bool
		errorCode string
	}{
		{
			name: "allocation pool within subnet",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Cluster: ClusterConfig{
						Networking: ClusterNetworkingConfig{
							SubnetNodes:         "10.0.0.0/24",
							AllocationPoolStart: "10.0.0.10",
							AllocationPoolEnd:   "10.0.0.100",
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "allocation pool outside subnet",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Cluster: ClusterConfig{
						Networking: ClusterNetworkingConfig{
							SubnetNodes:         "10.0.0.0/24",
							AllocationPoolStart: "10.0.1.10",
							AllocationPoolEnd:   "10.0.1.100",
						},
					},
				},
			},
			wantError: true,
			errorCode: "E005",
		},
		{
			name: "VRRP enabled without IP",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Cluster: ClusterConfig{
						Networking: ClusterNetworkingConfig{
							VRRPEnabled: true,
							VRRPIP:      "",
						},
					},
				},
			},
			wantError: true,
			errorCode: "E006",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateBusinessRules(tt.cfg)

			if tt.wantError && len(errors) == 0 {
				t.Error("ValidateBusinessRules() expected errors but got none")
			}

			if !tt.wantError && len(errors) > 0 {
				t.Errorf("ValidateBusinessRules() unexpected errors: %v", errors)
			}

			if tt.wantError && tt.errorCode != "" {
				found := false
				for _, err := range errors {
					if err.Code == tt.errorCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateBusinessRules() expected error code %s but got: %v", tt.errorCode, errors)
				}
			}
		})
	}
}

// TestMultiLayerValidator_ValidateProvider tests provider-specific validation
func TestMultiLayerValidator_ValidateProvider(t *testing.T) {
	validator := NewMultiLayerValidator()

	tests := []struct {
		name      string
		cfg       *Config
		wantError bool
		errorCode string
	}{
		{
			name: "valid OpenStack provider",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Infrastructure: Infrastructure{
						Provider: "openstack",
						Cloud: CloudConfig{
							OpenStack: SimplifiedOpenStackCloud{
								AuthURL: "https://identity.example.com/v3",
								Region:  "us-east-1",
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "OpenStack provider missing auth_url",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Infrastructure: Infrastructure{
						Provider: "openstack",
						Cloud: CloudConfig{
							OpenStack: SimplifiedOpenStackCloud{
								Region: "us-east-1",
							},
						},
					},
				},
			},
			wantError: true,
			errorCode: "E009",
		},
		{
			name: "unsupported provider",
			cfg: &Config{
				OpenCenter: SimplifiedOpenCenter{
					Infrastructure: Infrastructure{
						Provider: "invalid-provider",
					},
				},
			},
			wantError: true,
			errorCode: "E008",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateProvider(tt.cfg)

			if tt.wantError && len(errors) == 0 {
				t.Error("ValidateProvider() expected errors but got none")
			}

			if !tt.wantError && len(errors) > 0 {
				t.Errorf("ValidateProvider() unexpected errors: %v", errors)
			}

			if tt.wantError && tt.errorCode != "" {
				found := false
				for _, err := range errors {
					if err.Code == tt.errorCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateProvider() expected error code %s but got: %v", tt.errorCode, errors)
				}
			}
		})
	}
}

// TestMultiLayerValidator_ErrorAggregation tests that multiple errors are collected
func TestMultiLayerValidator_ErrorAggregation(t *testing.T) {
	validator := NewMultiLayerValidator()

	// Create a config with multiple errors
	cfg := &Config{
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				// Missing Name (E001)
				Organization: "opencenter",
				Region:       "us-east-1",
			},
			Infrastructure: Infrastructure{
				Provider: "invalid-provider", // Invalid provider (E008)
			},
			Cluster: ClusterConfig{
				Networking: ClusterNetworkingConfig{
					SubnetNodes: "invalid-cidr", // Invalid CIDR (E003)
					VRRPEnabled: true,
					VRRPIP:      "", // Missing VRRP IP (E006)
				},
			},
		},
	}

	errors := validator.Validate(cfg)

	// Should have multiple errors
	if len(errors) < 2 {
		t.Errorf("Validate() expected multiple errors but got %d: %v", len(errors), errors)
	}

	// Check that we have different error codes
	errorCodes := make(map[string]bool)
	for _, err := range errors {
		errorCodes[err.Code] = true
	}

	if len(errorCodes) < 2 {
		t.Errorf("Validate() expected multiple error codes but got %d: %v", len(errorCodes), errorCodes)
	}
}

// TestMultiLayerValidator_FieldPathGeneration tests field path generation
func TestMultiLayerValidator_FieldPathGeneration(t *testing.T) {
	validator := NewMultiLayerValidator()

	cfg := &Config{
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				// Missing Name
				Organization: "opencenter",
				Region:       "us-east-1",
			},
		},
	}

	errors := validator.ValidateSchema(cfg)

	// Should have at least one error
	if len(errors) == 0 {
		t.Fatal("ValidateSchema() expected errors but got none")
	}

	// Check that field path is generated
	for _, err := range errors {
		if err.Field == "" {
			t.Error("ValidationError missing field path")
		}
	}
}
