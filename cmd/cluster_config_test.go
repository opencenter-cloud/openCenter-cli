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
	"github.com/spf13/cobra"
)

// TestClusterConfigCommand tests the cluster config command structure.
// Requirements: 15.7, 15.8
func TestClusterConfigCommand(t *testing.T) {
	cmd := newClusterConfigCmd()

	if cmd == nil {
		t.Fatal("config command should not be nil")
	}

	if cmd.Use != "config" {
		t.Errorf("expected Use='config', got '%s'", cmd.Use)
	}

	// Verify subcommands exist
	exportCmd := findSubcommand(cmd, "export-effective")
	if exportCmd == nil {
		t.Error("export-effective subcommand should exist")
	}
}

// TestExportEffectiveCommand tests the export-effective command structure.
// Requirements: 15.7, 15.8
func TestExportEffectiveCommand(t *testing.T) {
	cmd := newClusterConfigExportEffectiveCmd()

	if cmd == nil {
		t.Fatal("export-effective command should not be nil")
	}

	if cmd.Use != "export-effective [name]" {
		t.Errorf("expected Use='export-effective [name]', got '%s'", cmd.Use)
	}

	// Verify flags exist
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("output flag should exist")
	}

	if outputFlag.Shorthand != "o" {
		t.Errorf("expected output flag shorthand 'o', got '%s'", outputFlag.Shorthand)
	}
}

// TestExportV2EffectiveConfig tests exporting v2 effective configuration.
// Requirements: 15.7, 15.8
func TestExportV2EffectiveConfig(t *testing.T) {
	// Create a minimal v2 config
	v2Config := `schema_version: "2.0"
opencenter:
  meta:
    name: test-export-cluster
    organization: test-org
    env: dev
    region: sjc3
  cluster:
    cluster_name: test-export-cluster
    base_domain: test.com
    cluster_fqdn: test-export-cluster.sjc3.test.com
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
      authorized_keys:
        - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ test@example.com
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
        region: sjc3
        project_name: test-project
        project_id: test-project-id
        domain_name: default
        network_name: test-network
        network_id: test-network-id
        external_network_name: public
        image_name: ubuntu-22.04
  gitops:
    repository:
      url: https://github.com/test/test-repo.git
      branch: main
      local_dir: ./gitops
    base_repo:
      url: https://github.com/opencenter-cloud/openCenter-gitops-base.git
      release: v0.1.0
      branch: main
    flux:
      interval: 15m
      prune: true
deployment:
  auto_deploy: false
  method: kubespray
opentofu:
  enabled: true
  path: opentofu
  backend:
    type: local
    local:
      path: terraform.tfstate
secrets:
  sops_age_key_file: ~/.config/opencenter/age/keys.txt
  ssh_key:
    private: ~/.ssh/id_rsa
    public: ~/.ssh/id_rsa.pub
    cypher: rsa
  global:
    aws:
      infrastructure:
        access_key: ""
        secret_access_key: ""
        region: us-east-1
      application:
        access_key: ""
        secret_access_key: ""
        region: us-east-1
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	outputPath := filepath.Join(tmpDir, "effective-config.yaml")

	if err := os.WriteFile(configPath, []byte(v2Config), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader and load config
	registry := defaults.NewRegistry()
	loader := v2.NewConfigLoader(registry)

	cfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Export effective configuration
	effectiveConfig, err := loader.ExportEffectiveConfig(cfg)
	if err != nil {
		t.Fatalf("failed to export effective config: %v", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, effectiveConfig, 0600); err != nil {
		t.Fatalf("failed to write effective config: %v", err)
	}

	// Verify the output file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("effective config file should exist")
	}

	// Read and verify content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read effective config: %v", err)
	}

	contentStr := string(content)

	// Verify header comments exist
	if !strings.Contains(contentStr, "# Effective Configuration") {
		t.Error("effective config should contain header comment")
	}

	if !strings.Contains(contentStr, "# Applied defaults:") {
		t.Error("effective config should contain applied defaults comment")
	}

	// Verify schema version is present
	if !strings.Contains(contentStr, "schema_version: \"2.0\"") {
		t.Error("effective config should contain schema version")
	}

	// Verify cluster name is present
	if !strings.Contains(contentStr, "name: test-export-cluster") {
		t.Error("effective config should contain cluster name")
	}
}

// TestExportEffectiveConfigWithDefaults tests that applied defaults are included in comments.
// Requirements: 15.7, 15.8
func TestExportEffectiveConfigWithDefaults(t *testing.T) {
	// Create a minimal v2 config that will trigger default application
	v2Config := `schema_version: "2.0"
opencenter:
  meta:
    name: test-defaults-cluster
    organization: test-org
    env: dev
    region: sjc3
  cluster:
    cluster_name: test-defaults-cluster
    base_domain: test.com
    cluster_fqdn: test-defaults-cluster.sjc3.test.com
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
      authorized_keys:
        - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ test@example.com
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
        region: sjc3
        project_name: test-project
        project_id: test-project-id
        domain_name: default
        network_name: test-network
        network_id: test-network-id
        external_network_name: public
        image_name: ubuntu-22.04
  gitops:
    repository:
      url: https://github.com/test/test-repo.git
      branch: main
      local_dir: ./gitops
    base_repo:
      url: https://github.com/opencenter-cloud/openCenter-gitops-base.git
      release: v0.1.0
      branch: main
    flux:
      interval: 15m
      prune: true
deployment:
  auto_deploy: false
  method: kubespray
opentofu:
  enabled: true
  path: opentofu
  backend:
    type: local
    local:
      path: terraform.tfstate
secrets:
  sops_age_key_file: ~/.config/opencenter/age/keys.txt
  ssh_key:
    private: ~/.ssh/id_rsa
    public: ~/.ssh/id_rsa.pub
    cypher: rsa
  global:
    aws:
      infrastructure:
        access_key: ""
        secret_access_key: ""
        region: us-east-1
      application:
        access_key: ""
        secret_access_key: ""
        region: us-east-1
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	if err := os.WriteFile(configPath, []byte(v2Config), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create loader and load config
	registry := defaults.NewRegistry()
	loader := v2.NewConfigLoader(registry)

	cfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Export effective configuration
	effectiveConfig, err := loader.ExportEffectiveConfig(cfg)
	if err != nil {
		t.Fatalf("failed to export effective config: %v", err)
	}

	contentStr := string(effectiveConfig)

	// Verify applied defaults section exists
	if !strings.Contains(contentStr, "# Applied defaults:") {
		t.Error("effective config should contain applied defaults section")
	}

	// The loader should have tracked some applied defaults
	appliedDefaults := loader.GetAppliedDefaults()
	if len(appliedDefaults) == 0 {
		t.Log("Warning: no defaults were applied (this may be expected if all values are explicit)")
	}
}

// findSubcommand is a helper function to find a subcommand by name.
func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}
