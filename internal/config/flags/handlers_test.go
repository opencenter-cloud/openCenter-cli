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

package flags

import (
	"testing"
)

// Test ServerPoolFlagHandler

func TestServerPoolFlagHandler_ParseArrayFlag_ValidInput(t *testing.T) {
	handler := NewServerPoolFlagHandler()
	
	testCases := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "basic server pool",
			input: "name=compute,worker_count=3,flavor_worker=large,node_worker=worker",
			expected: map[string]interface{}{
				"name":          "compute",
				"worker_count":  3,
				"flavor_worker": "large",
				"node_worker":   "worker",
			},
		},
		{
			name:  "server pool with optional fields",
			input: "name=storage,worker_count=5,flavor_worker=xlarge,node_worker=storage,volume_size=100,volume_type=ssd",
			expected: map[string]interface{}{
				"name":          "storage",
				"worker_count":  5,
				"flavor_worker": "xlarge",
				"node_worker":   "storage",
				"volume_size":   100,
				"volume_type":   "ssd",
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := handler.ParseArrayFlag("server-pool", tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if config.Type != "server-pool" {
				t.Errorf("expected type 'server-pool', got '%s'", config.Type)
			}
			
			if config.Path != "opencenter.infrastructure.server_pools" {
				t.Errorf("expected path 'opencenter.infrastructure.server_pools', got '%s'", config.Path)
			}
			
			for key, expectedValue := range tc.expected {
				if actualValue, exists := config.Fields[key]; !exists {
					t.Errorf("missing field '%s'", key)
				} else if actualValue != expectedValue {
					t.Errorf("field '%s': expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestServerPoolFlagHandler_ParseArrayFlag_InvalidInput(t *testing.T) {
	handler := NewServerPoolFlagHandler()
	
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "empty input",
			input: "",
		},
		{
			name:  "missing required field",
			input: "name=compute,worker_count=3,flavor_worker=large",
		},
		{
			name:  "invalid worker_count",
			input: "name=compute,worker_count=invalid,flavor_worker=large,node_worker=worker",
		},
		{
			name:  "worker_count out of range",
			input: "name=compute,worker_count=150,flavor_worker=large,node_worker=worker",
		},
		{
			name:  "invalid key=value format",
			input: "name=compute,worker_count=3,invalid_pair,node_worker=worker",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler.ParseArrayFlag("server-pool", tc.input)
			if err == nil {
				t.Errorf("expected error for input '%s', but got none", tc.input)
			}
		})
	}
}

func TestServerPoolFlagHandler_CanHandle(t *testing.T) {
	handler := NewServerPoolFlagHandler()
	
	testCases := []struct {
		flagName string
		expected bool
	}{
		{"server-pool", true},
		{"server-pool-1", true},
		{"my-server-pool", true},
		{"ssh-key", false},
		{"dns-server", false},
		{"subnet", false},
		{"other-flag", false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.flagName, func(t *testing.T) {
			result := handler.CanHandle(tc.flagName)
			if result != tc.expected {
				t.Errorf("CanHandle('%s'): expected %v, got %v", tc.flagName, tc.expected, result)
			}
		})
	}
}

// Test SSHKeyFlagHandler

func TestSSHKeyFlagHandler_ParseArrayFlag_ValidInput(t *testing.T) {
	handler := NewSSHKeyFlagHandler()
	
	testCases := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "ssh key content",
			input: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ... user@host",
			expected: map[string]interface{}{
				"key":  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ... user@host",
				"type": "content",
			},
		},
		{
			name:  "ssh key file path",
			input: "/home/user/.ssh/id_rsa.pub",
			expected: map[string]interface{}{
				"key":  "/home/user/.ssh/id_rsa.pub",
				"type": "file",
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := handler.ParseArrayFlag("ssh-key", tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if config.Type != "ssh-key" {
				t.Errorf("expected type 'ssh-key', got '%s'", config.Type)
			}
			
			if config.Path != "opencenter.infrastructure.ssh_keys" {
				t.Errorf("expected path 'opencenter.infrastructure.ssh_keys', got '%s'", config.Path)
			}
			
			for key, expectedValue := range tc.expected {
				if actualValue, exists := config.Fields[key]; !exists {
					t.Errorf("missing field '%s'", key)
				} else if actualValue != expectedValue {
					t.Errorf("field '%s': expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestSSHKeyFlagHandler_ParseArrayFlag_InvalidInput(t *testing.T) {
	handler := NewSSHKeyFlagHandler()
	
	_, err := handler.ParseArrayFlag("ssh-key", "")
	if err == nil {
		t.Error("expected error for empty input, but got none")
	}
}

// Test DNSServerFlagHandler

func TestDNSServerFlagHandler_ParseArrayFlag_ValidInput(t *testing.T) {
	handler := NewDNSServerFlagHandler()
	
	testCases := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "IPv4 address",
			input: "8.8.8.8",
			expected: map[string]interface{}{
				"ip": "8.8.8.8",
			},
		},
		{
			name:  "IPv6 address",
			input: "2001:4860:4860::8888",
			expected: map[string]interface{}{
				"ip": "2001:4860:4860::8888",
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := handler.ParseArrayFlag("dns-server", tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if config.Type != "dns-server" {
				t.Errorf("expected type 'dns-server', got '%s'", config.Type)
			}
			
			if config.Path != "opencenter.networking.dns_servers" {
				t.Errorf("expected path 'opencenter.networking.dns_servers', got '%s'", config.Path)
			}
			
			for key, expectedValue := range tc.expected {
				if actualValue, exists := config.Fields[key]; !exists {
					t.Errorf("missing field '%s'", key)
				} else if actualValue != expectedValue {
					t.Errorf("field '%s': expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestDNSServerFlagHandler_ParseArrayFlag_InvalidInput(t *testing.T) {
	handler := NewDNSServerFlagHandler()
	
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "empty input",
			input: "",
		},
		{
			name:  "invalid IP address",
			input: "invalid.ip.address",
		},
		{
			name:  "malformed IP",
			input: "256.256.256.256",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler.ParseArrayFlag("dns-server", tc.input)
			if err == nil {
				t.Errorf("expected error for input '%s', but got none", tc.input)
			}
		})
	}
}

// Test SubnetFlagHandler

func TestSubnetFlagHandler_ParseArrayFlag_ValidInput(t *testing.T) {
	handler := NewSubnetFlagHandler()
	
	testCases := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "subnet with name and CIDR",
			input: "name=private,cidr=192.168.1.0/24",
			expected: map[string]interface{}{
				"name": "private",
				"cidr": "192.168.1.0/24",
			},
		},
		{
			name:  "subnet with only name",
			input: "name=public",
			expected: map[string]interface{}{
				"name": "public",
			},
		},
		{
			name:  "subnet with only CIDR",
			input: "cidr=10.0.0.0/16",
			expected: map[string]interface{}{
				"cidr": "10.0.0.0/16",
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := handler.ParseArrayFlag("subnet", tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if config.Type != "subnet" {
				t.Errorf("expected type 'subnet', got '%s'", config.Type)
			}
			
			if config.Path != "opencenter.networking.subnets" {
				t.Errorf("expected path 'opencenter.networking.subnets', got '%s'", config.Path)
			}
			
			for key, expectedValue := range tc.expected {
				if actualValue, exists := config.Fields[key]; !exists {
					t.Errorf("missing field '%s'", key)
				} else if actualValue != expectedValue {
					t.Errorf("field '%s': expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestSubnetFlagHandler_ParseArrayFlag_InvalidInput(t *testing.T) {
	handler := NewSubnetFlagHandler()
	
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "empty input",
			input: "",
		},
		{
			name:  "no name or CIDR",
			input: "gateway=192.168.1.1",
		},
		{
			name:  "invalid CIDR format",
			input: "name=test,cidr=invalid-cidr",
		},
		{
			name:  "invalid key=value format",
			input: "name=test,invalid_pair",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler.ParseArrayFlag("subnet", tc.input)
			if err == nil {
				t.Errorf("expected error for input '%s', but got none", tc.input)
			}
		})
	}
}

// Test multiple flag instances

func TestMultipleFlagInstances(t *testing.T) {
	serverPoolHandler := NewServerPoolFlagHandler()
	dnsHandler := NewDNSServerFlagHandler()
	
	// Test multiple server pool configurations
	config1, err1 := serverPoolHandler.ParseArrayFlag("server-pool", "name=compute,worker_count=3,flavor_worker=large,node_worker=worker")
	config2, err2 := serverPoolHandler.ParseArrayFlag("server-pool", "name=storage,worker_count=5,flavor_worker=xlarge,node_worker=storage")
	
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	
	// Verify configurations are independent
	if config1.Fields["name"] == config2.Fields["name"] {
		t.Error("configurations should be independent")
	}
	
	// Test multiple DNS server configurations
	dns1, err3 := dnsHandler.ParseArrayFlag("dns-server", "8.8.8.8")
	dns2, err4 := dnsHandler.ParseArrayFlag("dns-server", "8.8.4.4")
	
	if err3 != nil || err4 != nil {
		t.Fatalf("unexpected errors: %v, %v", err3, err4)
	}
	
	// Verify DNS configurations are independent
	if dns1.Fields["ip"] == dns2.Fields["ip"] {
		t.Error("DNS configurations should be independent")
	}
}