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
	"strings"
	"testing"
)

// TestSimpleReferenceResolution tests resolving a simple reference.
func TestSimpleReferenceResolution(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Cluster.Networking.VRRPIP = "10.2.128.100"
	cfg.OpenCenter.Cluster.BaseDomain = "${opencenter.cluster.networking.vrrp_ip}.example.com"

	resolver := NewReferenceResolver()
	err := resolver.Resolve(&cfg)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := "10.2.128.100.example.com"
	if cfg.OpenCenter.Cluster.BaseDomain != expected {
		t.Errorf("Expected BaseDomain to be '%s', got '%s'", expected, cfg.OpenCenter.Cluster.BaseDomain)
	}
}

// TestNestedReferenceResolution tests resolving nested references.
func TestNestedReferenceResolution(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Cluster.Networking.VRRPIP = "10.2.128.100"
	cfg.OpenCenter.Cluster.BaseDomain = "${opencenter.cluster.networking.vrrp_ip}.example.com"
	cfg.OpenCenter.Cluster.ClusterFQDN = "api.${opencenter.cluster.base_domain}"

	resolver := NewReferenceResolver()
	err := resolver.Resolve(&cfg)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expectedBaseDomain := "10.2.128.100.example.com"
	if cfg.OpenCenter.Cluster.BaseDomain != expectedBaseDomain {
		t.Errorf("Expected BaseDomain to be '%s', got '%s'", expectedBaseDomain, cfg.OpenCenter.Cluster.BaseDomain)
	}

	expectedFQDN := "api.10.2.128.100.example.com"
	if cfg.OpenCenter.Cluster.ClusterFQDN != expectedFQDN {
		t.Errorf("Expected ClusterFQDN to be '%s', got '%s'", expectedFQDN, cfg.OpenCenter.Cluster.ClusterFQDN)
	}
}

// TestReferenceToNonExistentPath tests that referencing a non-existent path fails.
func TestReferenceToNonExistentPath(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Cluster.Kubernetes.LoadbalancerProvider = "${opencenter.nonexistent.field}"

	resolver := NewReferenceResolver()
	err := resolver.Resolve(&cfg)

	if err == nil {
		t.Fatal("Expected error for non-existent reference, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected error message to contain 'not found', got: %v", err)
	}
}

// TestCircularReference tests that circular references are detected.
func TestCircularReference(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Cluster.ClusterName = "${opencenter.cluster.base_domain}"
	cfg.OpenCenter.Cluster.BaseDomain = "${opencenter.cluster.cluster_fqdn}"
	cfg.OpenCenter.Cluster.ClusterFQDN = "${opencenter.cluster.cluster_name}"

	resolver := NewReferenceResolver()
	err := resolver.Resolve(&cfg)

	if err == nil {
		t.Fatal("Expected error for circular reference, got nil")
	}

	if !strings.Contains(err.Error(), "circular reference") {
		t.Errorf("Expected error message to contain 'circular reference', got: %v", err)
	}
}

// TestBuildDependencyGraph tests building a dependency graph.
func TestBuildDependencyGraph(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Cluster.Networking.VRRPIP = "10.2.128.100"
	cfg.OpenCenter.Cluster.BaseDomain = "${opencenter.cluster.networking.vrrp_ip}.example.com"

	resolver := NewReferenceResolver()
	graph, err := resolver.BuildDependencyGraph(&cfg)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	// Check that the node exists
	nodePath := "opencenter.cluster.base_domain"
	if _, exists := graph.Nodes[nodePath]; !exists {
		t.Errorf("Expected node '%s' to exist in graph", nodePath)
	}

	// Check that the edge exists
	expectedDep := "opencenter.cluster.networking.vrrp_ip"
	edges := graph.Edges[nodePath]
	found := false
	for _, edge := range edges {
		if edge == expectedDep {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected edge from '%s' to '%s'", nodePath, expectedDep)
	}
}

// TestTopologicalSort tests the topological sort functionality.
func TestTopologicalSort(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Cluster.Networking.VRRPIP = "10.2.128.100"
	cfg.OpenCenter.Cluster.BaseDomain = "${opencenter.cluster.networking.vrrp_ip}.example.com"
	cfg.OpenCenter.Cluster.ClusterFQDN = "api.${opencenter.cluster.base_domain}"

	resolver := NewReferenceResolver()
	graph, err := resolver.BuildDependencyGraph(&cfg)
	if err != nil {
		t.Fatalf("Failed to build dependency graph: %v", err)
	}

	// Access the private topologicalSort method via the resolver
	r := resolver.(*referenceResolver)
	order, err := r.topologicalSort(graph)
	if err != nil {
		t.Fatalf("Failed to perform topological sort: %v", err)
	}

	// Verify that dependencies come before dependents
	baseDomainPath := "opencenter.cluster.base_domain"
	clusterFQDNPath := "opencenter.cluster.cluster_fqdn"

	baseIndex := -1
	fqdnIndex := -1
	for i, path := range order {
		if path == baseDomainPath {
			baseIndex = i
		}
		if path == clusterFQDNPath {
			fqdnIndex = i
		}
	}

	if baseIndex == -1 || fqdnIndex == -1 {
		t.Fatal("Expected both paths to be in the topological order")
	}

	if baseIndex >= fqdnIndex {
		t.Errorf("Expected '%s' (index %d) to come before '%s' (index %d) in topological order",
			baseDomainPath, baseIndex, clusterFQDNPath, fqdnIndex)
	}
}

// TestDetectCycles tests cycle detection in the dependency graph.
func TestDetectCycles(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Cluster.ClusterName = "${opencenter.cluster.base_domain}"
	cfg.OpenCenter.Cluster.BaseDomain = "${opencenter.cluster.cluster_name}"

	resolver := NewReferenceResolver()
	graph, err := resolver.BuildDependencyGraph(&cfg)
	if err != nil {
		t.Fatalf("Failed to build dependency graph: %v", err)
	}

	err = resolver.DetectCycles(graph)
	if err == nil {
		t.Fatal("Expected error for circular dependency, got nil")
	}

	if !strings.Contains(err.Error(), "circular reference") {
		t.Errorf("Expected error message to contain 'circular reference', got: %v", err)
	}
}

// TestMultipleReferencesInSingleString tests resolving multiple references in one string.
func TestMultipleReferencesInSingleString(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Meta.Name = "my-cluster"
	cfg.OpenCenter.Meta.Region = "us-west-2"
	cfg.OpenCenter.Cluster.ClusterFQDN = "${opencenter.meta.name}.${opencenter.meta.region}.example.com"

	resolver := NewReferenceResolver()
	err := resolver.Resolve(&cfg)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := "my-cluster.us-west-2.example.com"
	if cfg.OpenCenter.Cluster.ClusterFQDN != expected {
		t.Errorf("Expected ClusterFQDN to be '%s', got '%s'", expected, cfg.OpenCenter.Cluster.ClusterFQDN)
	}
}

// TestNoReferences tests that configuration without references is unchanged.
func TestNoReferences(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	originalFQDN := cfg.OpenCenter.Cluster.ClusterFQDN

	resolver := NewReferenceResolver()
	err := resolver.Resolve(&cfg)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cfg.OpenCenter.Cluster.ClusterFQDN != originalFQDN {
		t.Errorf("Expected ClusterFQDN to remain '%s', got '%s'", originalFQDN, cfg.OpenCenter.Cluster.ClusterFQDN)
	}
}

// TestNilConfigHandling tests that nil configuration is handled gracefully.
func TestNilConfigHandling(t *testing.T) {
	resolver := NewReferenceResolver()
	err := resolver.Resolve(nil)

	if err == nil {
		t.Fatal("Expected error for nil configuration, got nil")
	}

	if !strings.Contains(err.Error(), "cannot be nil") {
		t.Errorf("Expected error message to contain 'cannot be nil', got: %v", err)
	}
}

// TestReferenceWithSpecialCharacters tests references with special characters in paths.
func TestReferenceWithSpecialCharacters(t *testing.T) {
	cfg := defaultConfig("test-cluster")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "us-west-2"
	cfg.OpenCenter.Cluster.ClusterFQDN = "api.${opencenter.infrastructure.cloud.openstack.region}.example.com"

	resolver := NewReferenceResolver()
	err := resolver.Resolve(&cfg)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := "api.us-west-2.example.com"
	if cfg.OpenCenter.Cluster.ClusterFQDN != expected {
		t.Errorf("Expected ClusterFQDN to be '%s', got '%s'", expected, cfg.OpenCenter.Cluster.ClusterFQDN)
	}
}
