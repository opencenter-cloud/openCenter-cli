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
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
)

// TestConfigLoader_LoadFromBytes_ValidConfig tests loading a valid v2 configuration.
// Requirements: 16.7
func TestConfigLoader_LoadFromBytes_ValidConfig(t *testing.T) {
	registry := defaults.NewRegistry()
	loader := NewConfigLoader(registry)

	validYAML := `
schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    organization: test-org
    env: dev
    region: sjc3
  cluster:
    cluster_name: test-cluster
    base_domain: example.com
    cluster_fqdn: test-cluster.example.com
    admin_email: admin@example.com
    kubernetes:
      version: "1.28.0"
      api_port: 6443
      subnet_pods: "10.233.64.0/18"
      subnet_services: "10.233.0.0/18"
  infrastructure:
    provider: openstack
    os_version: "24"
    ssh:
      authorized_keys:
        - "ssh-rsa AAAAB3NzaC1yc2E..."
    networking:
      subnet_nodes: "10.2.128.0/22"
      allocation_pool_start: "10.2.128.10"
      allocation_pool_end: "10.2.131.254"
      vrrp_enabled: true
      vrrp_ip: "10.2.128.5"
      loadbalancer_provider: ovn
      dns_zone_name: cluster.local
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
      ntp_servers:
        - "time.google.com"
    compute:
      flavor_master: m1.medium
      flavor_worker: m1.large
      master_count: 3
      worker_count: 2
    storage:
      default_storage_class: standard
      worker_volume_size: 50
      worker_volume_destination_type: volume
      worker_volume_source_type: image
      worker_volume_type: ssd
    cloud:
      openstack:
        auth_url: "https://identity.api.rackspacecloud.com/v3"
        region: sjc3
        tenant_name: test-tenant
        project_id: test-project-id
        network_id: test-network-id
        availability_zones:
          - az1
          - az2
  gitops:
    repository:
      url: "ssh://git@github.com/example/repo.git"
      branch: main
    flux:
      interval: "15m"
      prune: true
deployment:
  auto_deploy: true
  method: kubespray
opentofu:
  backend:
    type: local
    local:
      path: "/tmp/terraform.tfstate"
secrets:
  global:
    aws_access_key: ""
    aws_secret_key: ""
`

	cfg, err := loader.LoadFromBytes([]byte(validYAML))
	if err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	// Verify basic fields
	if cfg.SchemaVersion != "2.0" {
		t.Errorf("Expected schema version '2.0', got '%s'", cfg.SchemaVersion)
	}

	if cfg.OpenCenter.Meta.Name != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", cfg.OpenCenter.Meta.Name)
	}

	if cfg.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Errorf("Expected provider 'openstack', got '%s'", cfg.OpenCenter.Infrastructure.Provider)
	}
}

// TestConfigLoader_LoadFromBytes_InvalidYAML tests error handling for invalid YAML syntax.
// Requirements: 16.7, 16.8
func TestConfigLoader_LoadFromBytes_InvalidYAML(t *testing.T) {
	registry := defaults.NewRegistry()
	loader := NewConfigLoader(registry)

	invalidYAML := `
schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    organization: test-org
    env: dev
    region: sjc3
  cluster:
    cluster_name: test-cluster
    base_domain: example.com
    # Missing closing quote
    cluster_fqdn: "test-cluster.example.com
`

	_, err := loader.LoadFromBytes([]byte(invalidYAML))
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}

	// Verify error message mentions YAML parsing
	if !strings.Contains(err.Error(), "stage 1") && !strings.Contains(err.Error(), "YAML") {
		t.Errorf("Expected YAML parsing error, got: %v", err)
	}
}

// TestConfigLoader_LoadFromBytes_InvalidSchemaVersion tests error handling for wrong schema version.
// Requirements: 16.7
func TestConfigLoader_LoadFromBytes_InvalidSchemaVersion(t *testing.T) {
	registry := defaults.NewRegistry()
	loader := NewConfigLoader(registry)

	tests := []struct {
		name    string
		version string
	}{
		{
			name:    "rejects legacy schema version",
			version: strings.Join([]string{"1", "0"}, "."),
		},
		{
			name:    "rejects future schema version",
			version: "3.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalidVersionYAML := fmt.Sprintf(`
schema_version: %q
opencenter:
  meta:
    name: test-cluster
`, tt.version)

			_, err := loader.LoadFromBytes([]byte(invalidVersionYAML))
			if err == nil {
				t.Fatal("Expected error for invalid schema version, got nil")
			}

			if !strings.Contains(err.Error(), "schema version") {
				t.Errorf("Expected schema version error, got: %v", err)
			}
		})
	}
}

// TestConfigLoader_LoadFromBytes_MissingRequiredFields tests validation of required fields.
// Requirements: 16.7
func TestConfigLoader_LoadFromBytes_MissingRequiredFields(t *testing.T) {
	registry := defaults.NewRegistry()
	loader := NewConfigLoader(registry)

	missingFieldsYAML := `
schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    region: sjc3
    # Missing organization, env
  infrastructure:
    provider: openstack
secrets:
  global: {}
`

	_, err := loader.LoadFromBytes([]byte(missingFieldsYAML))
	if err == nil {
		t.Fatal("Expected error for missing required fields, got nil")
	}

	// Verify error mentions validation
	if !strings.Contains(err.Error(), "validate") && !strings.Contains(err.Error(), "required") {
		t.Errorf("Expected validation error for missing fields, got: %v", err)
	}
}

// TestConfigLoader_SaveToFile tests saving configuration to a file.
// Requirements: 16.2
func TestConfigLoader_SaveToFile(t *testing.T) {
	registry := defaults.NewRegistry()
	loader := NewConfigLoader(registry)

	cfg := newValidV2TestConfig("openstack")

	// Save to temporary file
	tmpFile := t.TempDir() + "/test-config.yaml"
	err := loader.SaveToFile(cfg, tmpFile)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load it back
	loadedCfg, err := loader.LoadFromFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	// Verify key fields match
	if loadedCfg.OpenCenter.Meta.Name != cfg.OpenCenter.Meta.Name {
		t.Errorf("Cluster name mismatch after save/load")
	}
}

// TestConfigLoader_ExportEffectiveConfig tests exporting configuration with applied defaults.
// Requirements: 15.7, 15.8
func TestConfigLoader_ExportEffectiveConfig(t *testing.T) {
	registry := defaults.NewRegistry()
	loader := NewConfigLoader(registry)

	validYAML := `
schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    organization: test-org
    env: dev
    region: sjc3
  cluster:
    cluster_name: test-cluster
    base_domain: example.com
    cluster_fqdn: test-cluster.example.com
    admin_email: admin@example.com
    kubernetes:
      version: "1.28.0"
      api_port: 6443
      subnet_pods: "10.233.64.0/18"
      subnet_services: "10.233.0.0/18"
  infrastructure:
    provider: openstack
    os_version: "24"
    ssh:
      authorized_keys:
        - "ssh-rsa AAAAB3NzaC1yc2E..."
    networking:
      subnet_nodes: "10.2.128.0/22"
      allocation_pool_start: "10.2.128.10"
      allocation_pool_end: "10.2.131.254"
      vrrp_enabled: true
      vrrp_ip: "10.2.128.5"
      loadbalancer_provider: ovn
      dns_zone_name: cluster.local
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
      ntp_servers:
        - "time.google.com"
    compute:
      flavor_master: m1.medium
      flavor_worker: m1.large
      master_count: 3
      worker_count: 2
    storage:
      default_storage_class: standard
      worker_volume_size: 50
      worker_volume_destination_type: volume
      worker_volume_source_type: image
      worker_volume_type: ssd
    cloud:
      openstack:
        auth_url: "https://identity.api.rackspacecloud.com/v3"
        region: sjc3
        tenant_name: test-tenant
        project_id: test-project-id
        network_id: test-network-id
  gitops:
    repository:
      url: "ssh://git@github.com/example/repo.git"
      branch: main
    flux:
      interval: "15m"
      prune: true
deployment:
  auto_deploy: true
  method: kubespray
opentofu:
  backend:
    type: local
    local:
      path: "/tmp/terraform.tfstate"
secrets:
  global: {}
`

	cfg, err := loader.LoadFromBytes([]byte(validYAML))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Export effective configuration
	effectiveYAML, err := loader.ExportEffectiveConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to export effective config: %v", err)
	}

	// Verify it contains the header comment
	effectiveStr := string(effectiveYAML)
	if !strings.Contains(effectiveStr, "Effective Configuration") {
		t.Error("Expected effective config to contain header comment")
	}

	// Verify it contains applied defaults information
	if !strings.Contains(effectiveStr, "Applied defaults") {
		t.Error("Expected effective config to contain applied defaults information")
	}
}

// TestConfigLoader_Pipeline_ErrorHandling tests error handling at each pipeline stage.
// Requirements: 16.7, 16.8
func TestConfigLoader_Pipeline_ErrorHandling(t *testing.T) {
	registry := defaults.NewRegistry()
	loader := NewConfigLoader(registry)

	tests := []struct {
		name          string
		yaml          string
		expectedStage string
	}{
		{
			name: "Stage 1: Invalid YAML syntax",
			yaml: `
schema_version: "2.0"
opencenter:
  meta: {invalid yaml
`,
			expectedStage: "stage 1",
		},
		{
			name: "Stage 5: Validation error",
			yaml: `
schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    organization: test-org
    env: invalid-env
    region: sjc3
  infrastructure:
    provider: openstack
secrets:
  global: {}
`,
			expectedStage: "stage 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loader.LoadFromBytes([]byte(tt.yaml))
			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expectedStage) {
				t.Errorf("Expected error from %s, got: %v", tt.expectedStage, err)
			}
		})
	}
}
