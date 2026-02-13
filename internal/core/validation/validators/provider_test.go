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

package validators

import (
	"context"
	"testing"
)

func TestProviderValidator_Name(t *testing.T) {
	validator := NewProviderValidator()
	if validator.Name() != "provider" {
		t.Errorf("expected name 'provider', got %s", validator.Name())
	}
}

func TestProviderValidator_InvalidInput(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
		errorField  string
	}{
		{
			name:        "not a map",
			value:       "invalid",
			expectError: true,
			errorField:  "provider",
		},
		{
			name:        "missing provider",
			value:       map[string]interface{}{},
			expectError: true,
			errorField:  "provider",
		},
		{
			name: "provider not a string",
			value: map[string]interface{}{
				"provider": 123,
			},
			expectError: true,
			errorField:  "provider",
		},
		{
			name: "unsupported provider",
			value: map[string]interface{}{
				"provider": "unsupported",
			},
			expectError: true,
			errorField:  "provider",
		},
		{
			name: "missing config",
			value: map[string]interface{}{
				"provider": "openstack",
			},
			expectError: true,
			errorField:  "provider.config",
		},
		{
			name: "config not a map",
			value: map[string]interface{}{
				"provider": "openstack",
				"config":   "invalid",
			},
			expectError: true,
			errorField:  "provider.config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if tt.expectError && len(result.Errors) > 0 {
				found := false
				for _, e := range result.Errors {
					if e.Field == tt.errorField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error on field %s, but didn't find it", tt.errorField)
				}
			}
		})
	}
}

func TestProviderValidator_OpenStack(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		expectWarn  bool
	}{
		{
			name: "missing required fields",
			config: map[string]interface{}{
				"provider": "openstack",
				"config":   map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "missing auth_url",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"region": "RegionOne",
				},
			},
			expectError: true,
		},
		{
			name: "missing region",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url": "https://openstack.example.com:5000/v3",
				},
			},
			expectError: true,
		},
		{
			name: "invalid auth_url format",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url": "not a url",
					"region":   "RegionOne",
				},
			},
			expectError: true,
		},
		{
			name: "auth_url without protocol",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url": "openstack.example.com:5000/v3",
					"region":   "RegionOne",
				},
			},
			expectError: true,
		},
		{
			name: "insecure http auth_url",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url":                  "http://openstack.example.com:5000/v3",
					"region":                    "RegionOne",
					"application_credential_id": "test-id",
				},
			},
			expectWarn: true,
		},
		{
			name: "auth_url without version",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url":                  "https://openstack.example.com:5000",
					"region":                    "RegionOne",
					"application_credential_id": "test-id",
				},
			},
			expectWarn: true,
		},
		{
			name: "missing authentication credentials",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url": "https://openstack.example.com:5000/v3",
					"region":   "RegionOne",
				},
			},
			expectError: true,
		},
		{
			name: "valid with application credentials",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url":                      "https://openstack.example.com:5000/v3",
					"region":                        "RegionOne",
					"application_credential_id":     "test-id",
					"application_credential_secret": "test-secret",
				},
			},
		},
		{
			name: "valid with user credentials",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url":      "https://openstack.example.com:5000/v3",
					"region":        "RegionOne",
					"user_name":     "test-user",
					"user_password": "test-password",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if tt.expectWarn && len(result.Warnings) == 0 {
				t.Error("expected warnings, but got none")
			}

			if !tt.expectError && !tt.expectWarn && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProviderValidator_AWS(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		expectWarn  bool
	}{
		{
			name: "missing required fields",
			config: map[string]interface{}{
				"provider": "aws",
				"config":   map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "invalid region format",
			config: map[string]interface{}{
				"provider": "aws",
				"config": map[string]interface{}{
					"region": "invalid-region",
				},
			},
			expectWarn: true,
		},
		{
			name: "invalid VPC ID format",
			config: map[string]interface{}{
				"provider": "aws",
				"config": map[string]interface{}{
					"region": "us-east-1",
					"vpc_id": "invalid-vpc-id",
				},
			},
			expectError: true,
		},
		{
			name: "invalid subnet ID format",
			config: map[string]interface{}{
				"provider": "aws",
				"config": map[string]interface{}{
					"region":          "us-east-1",
					"private_subnets": []interface{}{"invalid-subnet-id"},
				},
			},
			expectError: true,
		},
		{
			name: "valid configuration",
			config: map[string]interface{}{
				"provider": "aws",
				"config": map[string]interface{}{
					"region":          "us-east-1",
					"vpc_id":          "vpc-1234567890abcdef0",
					"private_subnets": []interface{}{"subnet-1234567890abcdef0"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if tt.expectWarn && len(result.Warnings) == 0 {
				t.Error("expected warnings, but got none")
			}

			if !tt.expectError && !tt.expectWarn && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProviderValidator_GCP(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "missing required fields",
			config: map[string]interface{}{
				"provider": "gcp",
				"config":   map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "project ID not lowercase",
			config: map[string]interface{}{
				"provider": "gcp",
				"config": map[string]interface{}{
					"project": "MyProject",
					"region":  "us-central1",
				},
			},
			expectError: true,
		},
		{
			name: "valid configuration",
			config: map[string]interface{}{
				"provider": "gcp",
				"config": map[string]interface{}{
					"project": "my-project",
					"region":  "us-central1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if !tt.expectError && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProviderValidator_Azure(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "missing required fields",
			config: map[string]interface{}{
				"provider": "azure",
				"config":   map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "invalid subscription_id format",
			config: map[string]interface{}{
				"provider": "azure",
				"config": map[string]interface{}{
					"subscription_id": "not-a-uuid",
					"resource_group":  "my-rg",
					"location":        "eastus",
				},
			},
			expectError: true,
		},
		{
			name: "valid configuration",
			config: map[string]interface{}{
				"provider": "azure",
				"config": map[string]interface{}{
					"subscription_id": "12345678-1234-1234-1234-123456789abc",
					"resource_group":  "my-rg",
					"location":        "eastus",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if !tt.expectError && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProviderValidator_VSphere(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "missing required fields",
			config: map[string]interface{}{
				"provider": "vsphere",
				"config":   map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "valid with hostname",
			config: map[string]interface{}{
				"provider": "vsphere",
				"config": map[string]interface{}{
					"server":     "vcenter.example.com",
					"datacenter": "DC1",
				},
			},
		},
		{
			name: "valid with IP",
			config: map[string]interface{}{
				"provider": "vsphere",
				"config": map[string]interface{}{
					"server":     "192.168.1.100",
					"datacenter": "DC1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if !tt.expectError && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProviderValidator_Baremetal(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "missing nodes",
			config: map[string]interface{}{
				"provider": "baremetal",
				"config":   map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "empty nodes list",
			config: map[string]interface{}{
				"provider": "baremetal",
				"config": map[string]interface{}{
					"nodes": []interface{}{},
				},
			},
			expectError: true,
		},
		{
			name: "node missing required fields",
			config: map[string]interface{}{
				"provider": "baremetal",
				"config": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"name": "node1",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "node with invalid IP",
			config: map[string]interface{}{
				"provider": "baremetal",
				"config": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"name": "node1",
							"ip":   "invalid-ip",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "valid configuration",
			config: map[string]interface{}{
				"provider": "baremetal",
				"config": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"name": "node1",
							"ip":   "192.168.1.10",
						},
						map[string]interface{}{
							"name": "node2",
							"ip":   "192.168.1.11",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if !tt.expectError && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProviderValidator_SetSupportedProviders(t *testing.T) {
	validator := NewProviderValidator()

	// Set custom supported providers
	customProviders := []string{"custom1", "custom2"}
	validator.SetSupportedProviders(customProviders)

	ctx := context.Background()

	// Test that custom provider is now supported
	value := map[string]interface{}{
		"provider": "custom1",
		"config":   map[string]interface{}{},
	}

	result, err := validator.Validate(ctx, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not have error about unsupported provider
	for _, e := range result.Errors {
		if e.Field == "provider" && e.Message != "" {
			if e.Message == "unsupported provider: custom1" {
				t.Error("custom provider should be supported after SetSupportedProviders")
			}
		}
	}

	// Test that original providers are no longer supported
	value2 := map[string]interface{}{
		"provider": "openstack",
		"config":   map[string]interface{}{},
	}

	result2, err := validator.Validate(ctx, value2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have error about unsupported provider
	found := false
	for _, e := range result2.Errors {
		if e.Field == "provider" && e.Message != "" {
			found = true
			break
		}
	}

	if !found {
		t.Error("openstack should not be supported after SetSupportedProviders with custom list")
	}
}

func TestProviderValidator_OpenStackNetworking(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		expectWarn  bool
	}{
		{
			name: "invalid floating_network_id",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url":                  "https://openstack.example.com:5000/v3",
					"region":                    "RegionOne",
					"application_credential_id": "test-id",
					"networking": map[string]interface{}{
						"floating_network_id": "not-a-uuid",
					},
				},
			},
			expectWarn: true,
		},
		{
			name: "invalid CIDR in k8s_api_port_acl",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url":                  "https://openstack.example.com:5000/v3",
					"region":                    "RegionOne",
					"application_credential_id": "test-id",
					"networking": map[string]interface{}{
						"k8s_api_port_acl": []interface{}{"invalid-cidr"},
					},
				},
			},
			expectError: true,
		},
		{
			name: "valid networking configuration",
			config: map[string]interface{}{
				"provider": "openstack",
				"config": map[string]interface{}{
					"auth_url":                  "https://openstack.example.com:5000/v3",
					"region":                    "RegionOne",
					"application_credential_id": "test-id",
					"networking": map[string]interface{}{
						"floating_network_id": "12345678-1234-1234-1234-123456789abc",
						"k8s_api_port_acl":    []interface{}{"10.0.0.0/8", "192.168.1.0/24"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectError && result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if tt.expectError && len(result.Errors) == 0 {
				t.Error("expected errors, but got none")
			}

			if tt.expectWarn && len(result.Warnings) == 0 {
				t.Error("expected warnings, but got none")
			}

			if !tt.expectError && !tt.expectWarn && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}


func TestProviderValidator_VMware(t *testing.T) {
	validator := NewProviderValidator()
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "missing nodes",
			config: map[string]interface{}{
				"provider": "vmware",
				"config":   map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "empty nodes list",
			config: map[string]interface{}{
				"provider": "vmware",
				"config": map[string]interface{}{
					"nodes": []interface{}{},
				},
			},
			expectError: true,
		},
		{
			name: "node missing required fields",
			config: map[string]interface{}{
				"provider": "vmware",
				"config": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"name": "node1",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "node with invalid IP",
			config: map[string]interface{}{
				"provider": "vmware",
				"config": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"name": "node1",
							"ip":   "invalid-ip",
							"role": "master",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "node with invalid role",
			config: map[string]interface{}{
				"provider": "vmware",
				"config": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"name": "node1",
							"ip":   "192.168.1.10",
							"role": "invalid",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "valid configuration",
			config: map[string]interface{}{
				"provider": "vmware",
				"config": map[string]interface{}{
					"vcenter_server": "vcenter.example.com",
					"datacenter":     "DC1",
					"datastore":      "datastore1",
					"nodes": []interface{}{
						map[string]interface{}{
							"name":        "master-1",
							"ip":          "192.168.1.10",
							"role":        "master",
							"uuid":        "12345678-1234-1234-1234-123456789abc",
							"mac_address": "00:50:56:12:34:56",
						},
						map[string]interface{}{
							"name": "worker-1",
							"ip":   "192.168.1.20",
							"role": "worker",
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			hasErrors := len(result.Errors) > 0
			if hasErrors != tt.expectError {
				t.Errorf("expected error: %v, got errors: %v", tt.expectError, result.Errors)
			}
		})
	}
}
