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

func TestNetworkValidator_Name(t *testing.T) {
	validator := NewNetworkValidator()
	if validator.Name() != "network" {
		t.Errorf("expected name 'network', got %q", validator.Name())
	}
}

func TestNetworkValidator_ValidCIDRFormats(t *testing.T) {
	tests := []struct {
		name        string
		subnetPods  string
		subnetSvcs  string
		wantValid   bool
		description string
	}{
		{
			name:        "valid IPv4 CIDRs",
			subnetPods:  "10.244.0.0/16",
			subnetSvcs:  "10.96.0.0/12",
			wantValid:   true,
			description: "standard non-overlapping IPv4 CIDRs",
		},
		{
			name:        "valid IPv6 CIDRs",
			subnetPods:  "2001:db8:1::/64",
			subnetSvcs:  "2001:db8:2::/64",
			wantValid:   true,
			description: "standard non-overlapping IPv6 CIDRs",
		},
		{
			name:        "valid /32 CIDR",
			subnetPods:  "10.244.0.1/32",
			subnetSvcs:  "10.96.0.1/32",
			wantValid:   true,
			description: "single IP CIDRs",
		},
		{
			name:        "empty CIDRs",
			subnetPods:  "",
			subnetSvcs:  "",
			wantValid:   true,
			description: "empty CIDRs should not cause errors",
		},
	}

	validator := NewNetworkValidator()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkConfig := &NetworkConfig{
				SubnetPods:     tt.subnetPods,
				SubnetServices: tt.subnetSvcs,
			}

			result, err := validator.Validate(ctx, networkConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("expected Valid=%v, got %v", tt.wantValid, result.Valid)
				if len(result.Errors) > 0 {
					t.Logf("Errors: %v", result.Errors)
				}
			}
		})
	}
}

func TestNetworkValidator_InvalidCIDRFormats(t *testing.T) {
	tests := []struct {
		name       string
		subnetPods string
		subnetSvcs string
		wantErrors int
		errorField string
	}{
		{
			name:       "missing prefix length",
			subnetPods: "10.244.0.0",
			wantErrors: 1,
			errorField: "subnet_pods",
		},
		{
			name:       "invalid IP address",
			subnetPods: "10.244.0.256/16",
			wantErrors: 1,
			errorField: "subnet_pods",
		},
		{
			name:       "invalid prefix length",
			subnetPods: "10.244.0.0/33",
			wantErrors: 1,
			errorField: "subnet_pods",
		},
		{
			name:       "not a CIDR",
			subnetPods: "not-a-cidr",
			wantErrors: 1,
			errorField: "subnet_pods",
		},
		{
			name:       "both invalid",
			subnetPods: "invalid",
			subnetSvcs: "also-invalid",
			wantErrors: 2,
		},
	}

	validator := NewNetworkValidator()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkConfig := &NetworkConfig{
				SubnetPods:     tt.subnetPods,
				SubnetServices: tt.subnetSvcs,
			}

			result, err := validator.Validate(ctx, networkConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid {
				t.Error("expected validation to fail, but it passed")
			}

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("expected %d errors, got %d", tt.wantErrors, len(result.Errors))
				for i, e := range result.Errors {
					t.Logf("Error %d: %s - %s", i, e.Field, e.Message)
				}
			}

			if tt.errorField != "" && len(result.Errors) > 0 {
				if result.Errors[0].Field != tt.errorField {
					t.Errorf("expected error field %q, got %q", tt.errorField, result.Errors[0].Field)
				}
			}

			// Verify suggestions are provided
			if len(result.Errors) > 0 && len(result.Errors[0].Suggestions) == 0 {
				t.Error("expected suggestions for error, got none")
			}
		})
	}
}

func TestNetworkValidator_CIDROverlap(t *testing.T) {
	tests := []struct {
		name        string
		subnetPods  string
		subnetSvcs  string
		wantOverlap bool
		description string
	}{
		{
			name:        "no overlap - different ranges",
			subnetPods:  "10.244.0.0/16",
			subnetSvcs:  "10.96.0.0/12",
			wantOverlap: false,
			description: "standard non-overlapping ranges",
		},
		{
			name:        "overlap - service contains pod",
			subnetPods:  "10.96.1.0/24",
			subnetSvcs:  "10.96.0.0/12",
			wantOverlap: true,
			description: "service CIDR contains pod CIDR",
		},
		{
			name:        "overlap - pod contains service",
			subnetPods:  "10.0.0.0/8",
			subnetSvcs:  "10.96.0.0/12",
			wantOverlap: true,
			description: "pod CIDR contains service CIDR",
		},
		{
			name:        "overlap - identical ranges",
			subnetPods:  "10.244.0.0/16",
			subnetSvcs:  "10.244.0.0/16",
			wantOverlap: true,
			description: "identical CIDRs",
		},
		{
			name:        "no overlap - adjacent ranges",
			subnetPods:  "10.244.0.0/16",
			subnetSvcs:  "10.245.0.0/16",
			wantOverlap: false,
			description: "adjacent but non-overlapping ranges",
		},
	}

	validator := NewNetworkValidator()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkConfig := &NetworkConfig{
				SubnetPods:     tt.subnetPods,
				SubnetServices: tt.subnetSvcs,
			}

			result, err := validator.Validate(ctx, networkConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			hasOverlapError := false
			for _, e := range result.Errors {
				if e.Field == "network" && e.Message == "pod CIDR and service CIDR overlap" {
					hasOverlapError = true
					break
				}
			}

			if hasOverlapError != tt.wantOverlap {
				t.Errorf("expected overlap=%v, got %v", tt.wantOverlap, hasOverlapError)
				if len(result.Errors) > 0 {
					t.Logf("Errors: %v", result.Errors)
				}
			}

			// Verify suggestions are provided for overlap errors
			if hasOverlapError {
				found := false
				for _, e := range result.Errors {
					if e.Field == "network" && len(e.Suggestions) > 0 {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected suggestions for overlap error, got none")
				}
			}
		})
	}
}

func TestNetworkValidator_DNSServerValidation(t *testing.T) {
	tests := []struct {
		name          string
		dnsServers    []string
		wantValid     bool
		wantErrorsMin int
		description   string
	}{
		{
			name:        "valid IPv4 DNS servers",
			dnsServers:  []string{"8.8.8.8", "8.8.4.4"},
			wantValid:   true,
			description: "Google DNS servers",
		},
		{
			name:        "valid IPv6 DNS servers",
			dnsServers:  []string{"2001:4860:4860::8888", "2001:4860:4860::8844"},
			wantValid:   true,
			description: "Google IPv6 DNS servers",
		},
		{
			name:        "mixed IPv4 and IPv6",
			dnsServers:  []string{"8.8.8.8", "2001:4860:4860::8888"},
			wantValid:   true,
			description: "mixed IP versions",
		},
		{
			name:          "invalid DNS server",
			dnsServers:    []string{"not-an-ip"},
			wantValid:     false,
			wantErrorsMin: 1,
			description:   "invalid IP format",
		},
		{
			name:          "invalid IPv4 address",
			dnsServers:    []string{"256.256.256.256"},
			wantValid:     false,
			wantErrorsMin: 1,
			description:   "out of range IPv4",
		},
		{
			name:          "multiple invalid servers",
			dnsServers:    []string{"invalid1", "invalid2", "8.8.8.8"},
			wantValid:     false,
			wantErrorsMin: 2,
			description:   "multiple invalid with one valid",
		},
		{
			name:        "empty DNS server list",
			dnsServers:  []string{},
			wantValid:   true,
			description: "no DNS servers specified",
		},
		{
			name:        "empty string in list",
			dnsServers:  []string{"8.8.8.8", "", "8.8.4.4"},
			wantValid:   true,
			description: "empty strings should be skipped",
		},
	}

	validator := NewNetworkValidator()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkConfig := &NetworkConfig{
				DNSNameservers: tt.dnsServers,
			}

			result, err := validator.Validate(ctx, networkConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("expected Valid=%v, got %v", tt.wantValid, result.Valid)
				if len(result.Errors) > 0 {
					t.Logf("Errors: %v", result.Errors)
				}
			}

			if tt.wantErrorsMin > 0 && len(result.Errors) < tt.wantErrorsMin {
				t.Errorf("expected at least %d errors, got %d", tt.wantErrorsMin, len(result.Errors))
			}

			// Verify error field names for DNS servers
			for _, e := range result.Errors {
				if e.Field != "network" && e.Field != "subnet_pods" && e.Field != "subnet_services" {
					// Should be dns_nameservers[N]
					if len(e.Field) < 16 || e.Field[:16] != "dns_nameservers[" {
						t.Errorf("unexpected error field: %q", e.Field)
					}
				}
			}

			// Verify suggestions are provided
			if !result.Valid && len(result.Errors) > 0 && len(result.Errors[0].Suggestions) == 0 {
				t.Error("expected suggestions for error, got none")
			}
		})
	}
}

func TestNetworkValidator_VRRPIPValidation(t *testing.T) {
	tests := []struct {
		name        string
		vrrpEnabled bool
		vrrpIP      string
		wantValid   bool
		description string
	}{
		{
			name:        "valid VRRP IP",
			vrrpEnabled: true,
			vrrpIP:      "192.168.1.100",
			wantValid:   true,
			description: "valid IPv4 VRRP IP",
		},
		{
			name:        "invalid VRRP IP",
			vrrpEnabled: true,
			vrrpIP:      "not-an-ip",
			wantValid:   false,
			description: "invalid VRRP IP format",
		},
		{
			name:        "VRRP disabled with invalid IP",
			vrrpEnabled: false,
			vrrpIP:      "not-an-ip",
			wantValid:   true,
			description: "VRRP disabled, IP not validated",
		},
		{
			name:        "VRRP enabled with empty IP",
			vrrpEnabled: true,
			vrrpIP:      "",
			wantValid:   true,
			description: "empty VRRP IP is allowed",
		},
	}

	validator := NewNetworkValidator()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkConfig := &NetworkConfig{
				VRRPEnabled: tt.vrrpEnabled,
				VRRPIP:      tt.vrrpIP,
			}

			result, err := validator.Validate(ctx, networkConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("expected Valid=%v, got %v", tt.wantValid, result.Valid)
				if len(result.Errors) > 0 {
					t.Logf("Errors: %v", result.Errors)
				}
			}
		})
	}
}

func TestNetworkValidator_NodeSubnetValidation(t *testing.T) {
	tests := []struct {
		name        string
		subnetNodes string
		wantValid   bool
		description string
	}{
		{
			name:        "valid node subnet",
			subnetNodes: "192.168.1.0/24",
			wantValid:   true,
			description: "valid IPv4 CIDR for nodes",
		},
		{
			name:        "invalid node subnet",
			subnetNodes: "invalid-cidr",
			wantValid:   false,
			description: "invalid CIDR format",
		},
		{
			name:        "empty node subnet",
			subnetNodes: "",
			wantValid:   true,
			description: "empty node subnet is allowed",
		},
	}

	validator := NewNetworkValidator()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkConfig := &NetworkConfig{
				SubnetNodes: tt.subnetNodes,
			}

			result, err := validator.Validate(ctx, networkConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("expected Valid=%v, got %v", tt.wantValid, result.Valid)
			}
		})
	}
}

func TestNetworkValidator_Warnings(t *testing.T) {
	validator := NewNetworkValidator()
	ctx := context.Background()

	// Test with empty configuration
	networkConfig := &NetworkConfig{}

	result, err := validator.Validate(ctx, networkConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have warnings for missing configuration
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for empty configuration, got none")
	}

	// Check for specific warnings
	expectedWarnings := map[string]bool{
		"subnet_pods":     false,
		"subnet_services": false,
		"dns_nameservers": false,
	}

	for _, w := range result.Warnings {
		if _, ok := expectedWarnings[w.Field]; ok {
			expectedWarnings[w.Field] = true
		}
	}

	for field, found := range expectedWarnings {
		if !found {
			t.Errorf("expected warning for field %q, but not found", field)
		}
	}

	// Verify warnings have suggestions
	for _, w := range result.Warnings {
		if len(w.Suggestions) == 0 {
			t.Errorf("warning for field %q has no suggestions", w.Field)
		}
	}
}

func TestNetworkValidator_InvalidType(t *testing.T) {
	validator := NewNetworkValidator()
	ctx := context.Background()

	// Test with invalid type
	result, err := validator.Validate(ctx, "not-a-network-config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected validation to fail for invalid type")
	}

	if len(result.Errors) == 0 {
		t.Error("expected error for invalid type")
	}

	if result.Errors[0].Field != "network" {
		t.Errorf("expected error field 'network', got %q", result.Errors[0].Field)
	}
}

func TestNetworkValidator_CompleteConfiguration(t *testing.T) {
	validator := NewNetworkValidator()
	ctx := context.Background()

	// Test with complete valid configuration
	networkConfig := &NetworkConfig{
		SubnetNodes:    "192.168.1.0/24",
		SubnetPods:     "10.244.0.0/16",
		SubnetServices: "10.96.0.0/12",
		DNSNameservers: []string{"8.8.8.8", "8.8.4.4"},
		VRRPEnabled:    true,
		VRRPIP:         "192.168.1.100",
	}

	result, err := validator.Validate(ctx, networkConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected validation to pass for complete valid configuration")
		for _, e := range result.Errors {
			t.Logf("Error: %s - %s", e.Field, e.Message)
		}
	}

	// Should have no warnings for complete configuration
	if len(result.Warnings) > 0 {
		t.Logf("Got %d warnings (this is acceptable for complete config)", len(result.Warnings))
	}
}
