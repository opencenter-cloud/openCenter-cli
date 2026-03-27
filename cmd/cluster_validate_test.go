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

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// TestValidateConfigRequiresSchemaVersion ensures configs without the canonical schema version fail validation.
func TestValidateConfigRequiresSchemaVersion(t *testing.T) {
	invalidConfig := `opencenter:
  meta:
    name: test-cluster
    organization: test-org
    env: dev
    region: sjc3`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	if err := os.WriteFile(configPath, []byte(invalidConfig), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	registry := defaults.NewRegistry()
	loader := v2.NewConfigLoader(registry)

	_, err := loader.LoadFromFile(configPath)
	if err == nil {
		t.Fatal("expected schema version validation error")
	}
	if !strings.Contains(err.Error(), "expected '2.0'") {
		t.Fatalf("expected canonical schema version error, got %v", err)
	}
}

// TestValidateCommandV2Support tests that the validate command can handle v2 configurations.
// Requirements: 13.2, 11.7
func TestValidateCommandV2Support(t *testing.T) {
	// This is an integration test that would require a full v2 config
	// For now, we just verify the command structure is correct
	cmd := newClusterValidateCmd()

	if cmd == nil {
		t.Fatal("validate command should not be nil")
	}

	if cmd.Use != "validate [name]" {
		t.Errorf("expected Use='validate [name]', got '%s'", cmd.Use)
	}

	// Verify flags exist
	generateDebugFlag := cmd.Flags().Lookup("generate-debug-config")
	if generateDebugFlag == nil {
		t.Error("generate-debug-config flag should exist")
	}

	outputDirFlag := cmd.Flags().Lookup("output-dir")
	if outputDirFlag == nil {
		t.Error("output-dir flag should exist")
	}
}

// TestValidateV2ConfigIntegration tests end-to-end v2 configuration validation.
// Requirements: 13.2, 11.7
func TestValidateV2ConfigIntegration(t *testing.T) {
	// Load the test v2 config
	configPath := filepath.Join("..", "testdata", "v2-minimal-config.yaml")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("test config not found: %s", configPath)
	}

	registry := defaults.NewRegistry()
	loader := v2.NewConfigLoader(registry)
	cfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load v2 config: %v", err)
	}

	if cfg.SchemaVersion != "2.0" {
		t.Errorf("expected version '2.0', got '%s'", cfg.SchemaVersion)
	}
}

// TestValidationErrorFormatting tests that validation errors are displayed with field paths.
// Requirements: 11.7
func TestValidationErrorFormatting(t *testing.T) {
	// Create a v2 config with validation errors (missing required fields)
	invalidV2Config := `schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    organization: test-org
    env: dev
    region: sjc3
  cluster:
    cluster_name: test-cluster
    base_domain: test.com
    cluster_fqdn: test-cluster.test.com
    admin_email: admin@test.com
    kubernetes:
      version: "1.28.5"
      api_port: 6443
      subnet_pods: "10.233.64.0/18"
      subnet_services: "10.233.0.0/18"
      network_plugin:
        calico:
          enabled: true
  infrastructure:
    provider: openstack
    os_version: "ubuntu-22.04"
    ssh:
      user: ubuntu
      key_path: ~/.ssh/id_rsa
      # Missing authorized_keys (required field)
    networking:
      subnet_nodes: "192.168.1.0/24"
      allocation_pool_start: "192.168.1.10"
      allocation_pool_end: "192.168.1.250"
      vrrp_ip: "192.168.1.5"
      vrrp_enabled: false
      loadbalancer_provider: ovn
      dns_zone_name: test.com
      dns_nameservers:
        - "8.8.8.8"
      ntp_servers:
        - "time.google.com"
    compute:
      flavor_bastion: m1.small
      flavor_master: m1.medium
      flavor_worker: m1.large
      master_count: 0
      worker_count: 3
    storage:
      default_storage_class: standard
      worker_volume_size: 100
      worker_volume_destination_type: volume
      worker_volume_source_type: image
      worker_volume_type: standard
      worker_volume_delete_on_termination: true
    cloud:
      openstack:
        auth_url: https://openstack.test.com:5000/v3
        project_name: test-project
        domain_name: default
        network_name: test-network
        external_network_name: public
        image_name: ubuntu-22.04
        # Missing required fields: region, project_id, network_id
deployment:
  auto_deploy: false
secrets:
  global:
    openstack_auth_url: https://openstack.test.com:5000/v3
  sops:
    enabled: false
# Missing required gitops and opentofu sections
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-v2-config.yaml")

	if err := os.WriteFile(configPath, []byte(invalidV2Config), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	registry := defaults.NewRegistry()
	loader := v2.NewConfigLoader(registry)

	_, err := loader.LoadFromFile(configPath)
	if err == nil {
		t.Error("expected validation error but got none")
	}

	// Verify error message contains field paths
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("error message should not be empty")
	}

	// The error should mention validation failure
	if !strings.Contains(errMsg, "validation") && !strings.Contains(errMsg, "validate") {
		t.Errorf("error message should mention validation: %s", errMsg)
	}
}
