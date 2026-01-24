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

// TestConfigLoaderResolvesReferences tests that the config loader resolves references.
func TestConfigLoaderResolvesReferences(t *testing.T) {
	yamlData := `
schema_version: "1.0"
opencenter:
  meta:
    name: test-cluster
    region: us-west-2
  cluster:
    cluster_name: test-cluster
    base_domain: k8s.example.com
    cluster_fqdn: "${opencenter.cluster.cluster_name}.${opencenter.meta.region}.${opencenter.cluster.base_domain}"
    networking:
      vrrp_ip: "10.2.128.100"
`

	loader := NewConfigLoader(nil)
	cfg, err := loader.LoadFromBytes(context.Background(), []byte(yamlData), "test-cluster")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := "test-cluster.us-west-2.k8s.example.com"
	if cfg.OpenCenter.Cluster.ClusterFQDN != expected {
		t.Errorf("Expected ClusterFQDN to be '%s', got '%s'", expected, cfg.OpenCenter.Cluster.ClusterFQDN)
	}
}

// TestConfigLoaderHandlesReferenceErrors tests that the config loader handles reference errors.
func TestConfigLoaderHandlesReferenceErrors(t *testing.T) {
	yamlData := `
schema_version: "1.0"
opencenter:
  meta:
    name: test-cluster
  cluster:
    cluster_name: test-cluster
    cluster_fqdn: "${opencenter.nonexistent.field}"
`

	loader := NewConfigLoader(nil)
	_, err := loader.LoadFromBytes(context.Background(), []byte(yamlData), "test-cluster")

	if err == nil {
		t.Fatal("Expected error for non-existent reference, got nil")
	}

	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

// TestConfigLoaderHandlesCircularReferences tests that the config loader detects circular references.
func TestConfigLoaderHandlesCircularReferences(t *testing.T) {
	yamlData := `
schema_version: "1.0"
opencenter:
  meta:
    name: test-cluster
  cluster:
    cluster_name: "${opencenter.cluster.base_domain}"
    base_domain: "${opencenter.cluster.cluster_name}"
`

	loader := NewConfigLoader(nil)
	_, err := loader.LoadFromBytes(context.Background(), []byte(yamlData), "test-cluster")

	if err == nil {
		t.Fatal("Expected error for circular reference, got nil")
	}

	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}
