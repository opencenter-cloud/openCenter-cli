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

package services

import (
	"testing"
)

func TestGetProviderRegistry(t *testing.T) {
	registry := GetProviderRegistry()
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}

	// Verify singleton behavior
	registry2 := GetProviderRegistry()
	if registry != registry2 {
		t.Error("expected same registry instance (singleton)")
	}
}

func TestGetDefaultProvider_CertManagerDNS(t *testing.T) {
	registry := GetProviderRegistry()

	tests := []struct {
		name          string
		infraProvider InfrastructureProvider
		expected      ServiceProviderType
	}{
		{
			name:          "AWS defaults to Route53",
			infraProvider: ProviderAWS,
			expected:      DNSProviderRoute53,
		},
		{
			name:          "GCP defaults to CloudDNS",
			infraProvider: ProviderGCP,
			expected:      DNSProviderCloudDNS,
		},
		{
			name:          "Azure defaults to AzureDNS",
			infraProvider: ProviderAzure,
			expected:      DNSProviderAzureDNS,
		},
		{
			name:          "BareMetal defaults to Cloudflare",
			infraProvider: ProviderBareMetal,
			expected:      DNSProviderCloudflare,
		},
		{
			name:          "VSphere defaults to Cloudflare",
			infraProvider: ProviderVSphere,
			expected:      DNSProviderCloudflare,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := registry.GetDefaultProvider("cert-manager", "dns", tt.infraProvider)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if provider != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, provider)
			}
		})
	}
}

func TestGetDefaultProvider_OpenStackDNS_WithDesignate(t *testing.T) {
	registry := GetProviderRegistry()

	// Set Designate as available
	registry.SetDesignateAvailability("test-cluster", true)

	provider, err := registry.GetDefaultProvider("cert-manager", "dns", ProviderOpenStack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider != DNSProviderDesignate {
		t.Errorf("expected %s when Designate is available, got %s", DNSProviderDesignate, provider)
	}
}

func TestGetDefaultProvider_OpenStackDNS_WithoutDesignate(t *testing.T) {
	registry := GetProviderRegistry()

	// Set Designate as unavailable
	registry.SetDesignateAvailability("test-cluster", false)

	provider, err := registry.GetDefaultProvider("cert-manager", "dns", ProviderOpenStack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider != DNSProviderCloudflare {
		t.Errorf("expected %s when Designate is unavailable, got %s", DNSProviderCloudflare, provider)
	}
}

func TestGetDefaultProvider_LokiStorage(t *testing.T) {
	registry := GetProviderRegistry()

	tests := []struct {
		name          string
		infraProvider InfrastructureProvider
		expected      ServiceProviderType
	}{
		{
			name:          "AWS defaults to S3",
			infraProvider: ProviderAWS,
			expected:      StorageProviderS3,
		},
		{
			name:          "OpenStack defaults to Swift",
			infraProvider: ProviderOpenStack,
			expected:      StorageProviderSwift,
		},
		{
			name:          "GCP defaults to GCS",
			infraProvider: ProviderGCP,
			expected:      StorageProviderGCS,
		},
		{
			name:          "Azure defaults to Azure",
			infraProvider: ProviderAzure,
			expected:      StorageProviderAzure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := registry.GetDefaultProvider("loki", "storage", tt.infraProvider)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if provider != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, provider)
			}
		})
	}
}

func TestValidateCompatibility_CertManagerDNS(t *testing.T) {
	registry := GetProviderRegistry()

	tests := []struct {
		name            string
		infraProvider   InfrastructureProvider
		serviceProvider ServiceProviderType
		expectError     bool
	}{
		{
			name:            "Route53 compatible with AWS",
			infraProvider:   ProviderAWS,
			serviceProvider: DNSProviderRoute53,
			expectError:     false,
		},
		{
			name:            "Route53 incompatible with OpenStack",
			infraProvider:   ProviderOpenStack,
			serviceProvider: DNSProviderRoute53,
			expectError:     true,
		},
		{
			name:            "Designate compatible with OpenStack",
			infraProvider:   ProviderOpenStack,
			serviceProvider: DNSProviderDesignate,
			expectError:     false,
		},
		{
			name:            "Designate incompatible with AWS",
			infraProvider:   ProviderAWS,
			serviceProvider: DNSProviderDesignate,
			expectError:     true,
		},
		{
			name:            "Cloudflare compatible with AWS",
			infraProvider:   ProviderAWS,
			serviceProvider: DNSProviderCloudflare,
			expectError:     false,
		},
		{
			name:            "Cloudflare compatible with OpenStack",
			infraProvider:   ProviderOpenStack,
			serviceProvider: DNSProviderCloudflare,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.ValidateCompatibility("cert-manager", "dns", tt.infraProvider, tt.serviceProvider)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateCompatibility_LokiStorage(t *testing.T) {
	registry := GetProviderRegistry()

	tests := []struct {
		name            string
		infraProvider   InfrastructureProvider
		serviceProvider ServiceProviderType
		expectError     bool
	}{
		{
			name:            "S3 compatible with AWS",
			infraProvider:   ProviderAWS,
			serviceProvider: StorageProviderS3,
			expectError:     false,
		},
		{
			name:            "Swift incompatible with AWS",
			infraProvider:   ProviderAWS,
			serviceProvider: StorageProviderSwift,
			expectError:     true,
		},
		{
			name:            "Swift compatible with OpenStack",
			infraProvider:   ProviderOpenStack,
			serviceProvider: StorageProviderSwift,
			expectError:     false,
		},
		{
			name:            "S3 compatible with OpenStack",
			infraProvider:   ProviderOpenStack,
			serviceProvider: StorageProviderS3,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.ValidateCompatibility("loki", "storage", tt.infraProvider, tt.serviceProvider)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetCompatibleProviders_CertManagerDNS(t *testing.T) {
	registry := GetProviderRegistry()

	tests := []struct {
		name          string
		infraProvider InfrastructureProvider
		minExpected   int
	}{
		{
			name:          "AWS has multiple compatible DNS providers",
			infraProvider: ProviderAWS,
			minExpected:   2, // route53, cloudflare
		},
		{
			name:          "OpenStack has multiple compatible DNS providers",
			infraProvider: ProviderOpenStack,
			minExpected:   2, // designate, cloudflare
		},
		{
			name:          "GCP has multiple compatible DNS providers",
			infraProvider: ProviderGCP,
			minExpected:   2, // clouddns, cloudflare
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compatible := registry.GetCompatibleProviders("cert-manager", "dns", tt.infraProvider)
			if len(compatible) < tt.minExpected {
				t.Errorf("expected at least %d compatible providers, got %d: %v",
					tt.minExpected, len(compatible), compatible)
			}
		})
	}
}

func TestAutoSelectProvider_OpenStackDNS(t *testing.T) {
	registry := GetProviderRegistry()

	tests := []struct {
		name               string
		designateAvailable bool
		expected           ServiceProviderType
	}{
		{
			name:               "Designate selected when available",
			designateAvailable: true,
			expected:           DNSProviderDesignate,
		},
		{
			name:               "Cloudflare selected when Designate unavailable",
			designateAvailable: false,
			expected:           DNSProviderCloudflare,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterName := "test-cluster-" + tt.name
			registry.SetDesignateAvailability(clusterName, tt.designateAvailable)

			provider, err := registry.AutoSelectProvider("cert-manager", "dns", ProviderOpenStack, clusterName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if provider != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, provider)
			}
		})
	}
}

func TestAutoSelectProvider_NonOpenStack(t *testing.T) {
	registry := GetProviderRegistry()

	tests := []struct {
		name          string
		infraProvider InfrastructureProvider
		expected      ServiceProviderType
	}{
		{
			name:          "AWS auto-selects Route53",
			infraProvider: ProviderAWS,
			expected:      DNSProviderRoute53,
		},
		{
			name:          "GCP auto-selects CloudDNS",
			infraProvider: ProviderGCP,
			expected:      DNSProviderCloudDNS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := registry.AutoSelectProvider("cert-manager", "dns", tt.infraProvider, "test-cluster")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if provider != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, provider)
			}
		})
	}
}
