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
	
	"gopkg.in/yaml.v3"
)

// TestOpenTofuLocalBackendParsing tests that local backend with path can be parsed
func TestOpenTofuLocalBackendParsing(t *testing.T) {
	configYAML := `schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    organization: test-org
  infrastructure:
    provider: openstack
  cluster:
    cluster_name: test-cluster
    admin_email: admin@example.com
    base_domain: k8s.opencenter.cloud
    cluster_fqdn: test-cluster.k8s.opencenter.cloud
    ssh_authorized_keys:
      - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ test@example.com
    kubernetes:
      version: "1.29.0"
      network_plugin:
        calico:
          enabled: true
      storage_plugin:
        cinder:
          enabled: true
    networking:
      ntp_servers:
        - time.example.com
      dns_nameservers:
        - 8.8.8.8
      subnet_nodes: 10.2.128.0/22
  gitops:
    git_dir: ~/test
    git_url: git@github.com:test/test.git
    git_branch: main
  storage:
    default_storage_class: test-sc
opentofu:
  enabled: true
  path: opentofu
  backend:
    type: local
    local:
      path: .opentofu-local-utils/terraform.tfstate
secrets:
  backend: sops
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(configYAML), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	// Verify the config was parsed correctly
	if cfg.OpenTofu.Backend.Type != "local" {
		t.Errorf("expected backend type 'local', got '%s'", cfg.OpenTofu.Backend.Type)
	}

	if cfg.OpenTofu.Backend.Local.Path != ".opentofu-local-utils/terraform.tfstate" {
		t.Errorf("expected local path '.opentofu-local-utils/terraform.tfstate', got '%s'", cfg.OpenTofu.Backend.Local.Path)
	}
}

// TestOpenTofuS3BackendParsing tests that S3 backend doesn't require local.path
func TestOpenTofuS3BackendParsing(t *testing.T) {
	configYAML := `schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    organization: test-org
  infrastructure:
    provider: openstack
  cluster:
    cluster_name: test-cluster
    admin_email: admin@example.com
    base_domain: k8s.opencenter.cloud
    cluster_fqdn: test-cluster.k8s.opencenter.cloud
    ssh_authorized_keys:
      - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ test@example.com
    kubernetes:
      version: "1.29.0"
      network_plugin:
        calico:
          enabled: true
      storage_plugin:
        cinder:
          enabled: true
    networking:
      ntp_servers:
        - time.example.com
      dns_nameservers:
        - 8.8.8.8
      subnet_nodes: 10.2.128.0/22
  gitops:
    git_dir: ~/test
    git_url: git@github.com:test/test.git
    git_branch: main
  storage:
    default_storage_class: test-sc
opentofu:
  enabled: true
  path: opentofu
  backend:
    type: s3
    s3:
      bucket: my-bucket
      key: terraform.tfstate
      region: us-east-1
secrets:
  backend: sops
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(configYAML), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	// Verify the config was parsed correctly
	if cfg.OpenTofu.Backend.Type != "s3" {
		t.Errorf("expected backend type 's3', got '%s'", cfg.OpenTofu.Backend.Type)
	}

	// Local.Path should be empty for S3 backend
	if cfg.OpenTofu.Backend.Local.Path != "" {
		t.Errorf("expected empty local path for S3 backend, got '%s'", cfg.OpenTofu.Backend.Local.Path)
	}

	// S3 fields should be populated
	if cfg.OpenTofu.Backend.S3.Bucket != "my-bucket" {
		t.Errorf("expected S3 bucket 'my-bucket', got '%s'", cfg.OpenTofu.Backend.S3.Bucket)
	}
}

// TestOpenTofuLocalBackendWithoutPath tests that local backend without path can be parsed
// (the path will be validated at runtime when OpenTofu is actually used)
func TestOpenTofuLocalBackendWithoutPath(t *testing.T) {
	configYAML := `schema_version: "2.0"
opencenter:
  meta:
    name: test-cluster
    organization: test-org
  infrastructure:
    provider: openstack
  cluster:
    cluster_name: test-cluster
    admin_email: admin@example.com
    base_domain: k8s.opencenter.cloud
    cluster_fqdn: test-cluster.k8s.opencenter.cloud
    ssh_authorized_keys:
      - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ test@example.com
    kubernetes:
      version: "1.29.0"
      network_plugin:
        calico:
          enabled: true
      storage_plugin:
        cinder:
          enabled: true
    networking:
      ntp_servers:
        - time.example.com
      dns_nameservers:
        - 8.8.8.8
      subnet_nodes: 10.2.128.0/22
  gitops:
    git_dir: ~/test
    git_url: git@github.com:test/test.git
    git_branch: main
  storage:
    default_storage_class: test-sc
opentofu:
  enabled: true
  path: opentofu
  backend:
    type: local
secrets:
  backend: sops
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(configYAML), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	// Verify the config was parsed correctly
	if cfg.OpenTofu.Backend.Type != "local" {
		t.Errorf("expected backend type 'local', got '%s'", cfg.OpenTofu.Backend.Type)
	}

	// Local.Path will be empty, which is fine - it will be validated at runtime
	if cfg.OpenTofu.Backend.Local.Path != "" {
		t.Logf("local path is '%s'", cfg.OpenTofu.Backend.Local.Path)
	}
}
