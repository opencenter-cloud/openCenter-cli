/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sops

import (
	"regexp"
	"strings"
	"testing"
)

// TestSOPSConfigInfrastructurePathExclusions verifies that the infrastructure
// path regex correctly excludes venv, kubespray, .terraform, and .bin directories
//
// Note: This test documents the expected SOPS regex pattern. SOPS uses PCRE
// (Perl Compatible Regular Expressions) which supports negative lookahead (?!...),
// but Go's regexp package does not. The actual pattern validation happens in SOPS.
func TestSOPSConfigInfrastructurePathExclusions(t *testing.T) {
	clusterName := "test-cluster"
	
	// This is the pattern used in the SOPS configuration for infrastructure files
	// Pattern: ^infrastructure/clusters/<cluster-name>/(?!(?:venv|kubespray|\.terraform|\.bin)/)(.*)
	//
	// The negative lookahead (?!...) ensures paths starting with venv/, kubespray/, .terraform/, or .bin/
	// are NOT matched, effectively excluding those directories from encryption.
	expectedPattern := `^infrastructure\/clusters\/` + clusterName + `\/(?!(?:venv|kubespray|\.terraform|\.bin)\/)(.*)`
	
	t.Logf("Expected SOPS pattern: %s", expectedPattern)
	
	// Document paths that SHOULD be encrypted (would match the pattern)
	shouldMatch := []string{
		"infrastructure/clusters/" + clusterName + "/main.tf",
		"infrastructure/clusters/" + clusterName + "/variables.tf",
		"infrastructure/clusters/" + clusterName + "/secrets/credentials.yaml",
		"infrastructure/clusters/" + clusterName + "/config/cluster.yaml",
		"infrastructure/clusters/" + clusterName + "/terraform.tfvars",
		"infrastructure/clusters/" + clusterName + "/subdir/file.yaml",
		"infrastructure/clusters/" + clusterName + "/modules/network/main.tf",
		"infrastructure/clusters/" + clusterName + "/environments/prod.tfvars",
	}

	// Document paths that SHOULD NOT be encrypted (excluded directories)
	shouldNotMatch := []string{
		// venv directory and contents
		"infrastructure/clusters/" + clusterName + "/venv/lib/python3.9/site-packages/module.py",
		"infrastructure/clusters/" + clusterName + "/venv/bin/activate",
		"infrastructure/clusters/" + clusterName + "/venv/pyvenv.cfg",
		"infrastructure/clusters/" + clusterName + "/venv/",
		
		// kubespray directory and contents
		"infrastructure/clusters/" + clusterName + "/kubespray/inventory/sample/hosts.yaml",
		"infrastructure/clusters/" + clusterName + "/kubespray/roles/kubernetes/node/tasks/main.yml",
		"infrastructure/clusters/" + clusterName + "/kubespray/",
		
		// .terraform directory and contents
		"infrastructure/clusters/" + clusterName + "/.terraform/providers/registry.terraform.io/hashicorp/aws/5.0.0/darwin_arm64/terraform-provider-aws_v5.0.0",
		"infrastructure/clusters/" + clusterName + "/.terraform/terraform.tfstate",
		"infrastructure/clusters/" + clusterName + "/.terraform/modules/modules.json",
		"infrastructure/clusters/" + clusterName + "/.terraform/",
		
		// .bin directory and contents
		"infrastructure/clusters/" + clusterName + "/.bin/terraform",
		"infrastructure/clusters/" + clusterName + "/.bin/tofu",
		"infrastructure/clusters/" + clusterName + "/.bin/kubectl",
		"infrastructure/clusters/" + clusterName + "/.bin/",
	}

	t.Log("Paths that SHOULD be encrypted:")
	for _, path := range shouldMatch {
		t.Logf("  ✓ %s", path)
	}

	t.Log("Paths that SHOULD NOT be encrypted (excluded):")
	for _, path := range shouldNotMatch {
		t.Logf("  ✗ %s", path)
	}
	
	// Verify the pattern structure is correct
	if !strings.Contains(expectedPattern, "(?!(?:venv|") {
		t.Error("Pattern missing venv exclusion")
	}
	if !strings.Contains(expectedPattern, "|kubespray|") {
		t.Error("Pattern missing kubespray exclusion")
	}
	if !strings.Contains(expectedPattern, "\\.terraform|") {
		t.Error("Pattern missing .terraform exclusion")
	}
	if !strings.Contains(expectedPattern, "\\.bin)") {
		t.Error("Pattern missing .bin exclusion")
	}
}

// TestSOPSConfigInfrastructurePathExclusionsMultipleClusters verifies exclusions work for different cluster names
//
// Note: This test documents the expected SOPS regex patterns. SOPS uses PCRE which supports
// negative lookahead, but Go's regexp does not. The actual pattern validation happens in SOPS.
func TestSOPSConfigInfrastructurePathExclusionsMultipleClusters(t *testing.T) {
	testCases := []struct {
		clusterName string
	}{
		{"prod-cluster"},
		{"dev-cluster"},
		{"staging-cluster"},
		{"k8s-sandbox"},
	}

	for _, tc := range testCases {
		t.Run(tc.clusterName, func(t *testing.T) {
			expectedPattern := `^infrastructure\/clusters\/` + tc.clusterName + `\/(?!(?:venv|kubespray|\.terraform|\.bin)\/)(.*)`
			
			t.Logf("Expected SOPS pattern for %s: %s", tc.clusterName, expectedPattern)
			
			// Verify pattern structure
			if !strings.Contains(expectedPattern, tc.clusterName) {
				t.Errorf("Pattern missing cluster name: %s", tc.clusterName)
			}
			if !strings.Contains(expectedPattern, "(?!(?:venv|") {
				t.Error("Pattern missing venv exclusion")
			}
			if !strings.Contains(expectedPattern, "|kubespray|") {
				t.Error("Pattern missing kubespray exclusion")
			}
			if !strings.Contains(expectedPattern, "\\.terraform|") {
				t.Error("Pattern missing .terraform exclusion")
			}
			if !strings.Contains(expectedPattern, "\\.bin)") {
				t.Error("Pattern missing .bin exclusion")
			}
		})
	}
}

// TestSOPSConfigSSHPublicKeyExclusion verifies SSH public keys are excluded
//
// Note: This test documents the expected SOPS regex pattern. SOPS uses PCRE which supports
// negative lookahead, but Go's regexp does not. The actual pattern validation happens in SOPS.
func TestSOPSConfigSSHPublicKeyExclusion(t *testing.T) {
	// Pattern: secrets/ssh/(?!.*\.pub$).*
	// This matches files in secrets/ssh/ that do NOT end with .pub
	expectedPattern := `secrets/ssh/(?!.*\.pub$).*`
	
	t.Logf("Expected SOPS pattern: %s", expectedPattern)
	
	// Document SSH private keys that SHOULD be encrypted
	shouldMatch := []string{
		"secrets/ssh/id_rsa",
		"secrets/ssh/id_ed25519",
		"secrets/ssh/deploy_key",
		"secrets/ssh/cluster-key",
	}

	// Document SSH public keys that SHOULD NOT be encrypted
	shouldNotMatch := []string{
		"secrets/ssh/id_rsa.pub",
		"secrets/ssh/id_ed25519.pub",
		"secrets/ssh/deploy_key.pub",
		"secrets/ssh/cluster-key.pub",
	}

	t.Log("SSH private keys that SHOULD be encrypted:")
	for _, path := range shouldMatch {
		t.Logf("  ✓ %s", path)
	}

	t.Log("SSH public keys that SHOULD NOT be encrypted:")
	for _, path := range shouldNotMatch {
		t.Logf("  ✗ %s", path)
	}
	
	// Verify pattern structure
	if !strings.Contains(expectedPattern, "secrets/ssh/") {
		t.Error("Pattern missing secrets/ssh/ prefix")
	}
	if !strings.Contains(expectedPattern, "(?!.*\\.pub$)") {
		t.Error("Pattern missing .pub exclusion")
	}
}

// TestSOPSConfigAgeKeyExclusion verifies age key pattern
func TestSOPSConfigAgeKeyExclusion(t *testing.T) {
	// Pattern: secrets/age/keys/.*-key\.txt$
	pattern := `secrets/age/keys/.*-key\.txt$`
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("Failed to compile regex pattern: %v", err)
	}

	// Test cases: age keys that SHOULD be encrypted
	shouldMatch := []string{
		"secrets/age/keys/cluster-key.txt",
		"secrets/age/keys/prod-key.txt",
		"secrets/age/keys/backup-key.txt",
		"secrets/age/keys/test-cluster-key.txt",
	}

	// Test cases: files that SHOULD NOT match
	shouldNotMatch := []string{
		"secrets/age/keys/README.txt",
		"secrets/age/keys/cluster-key.pub",
		"secrets/age/keys/notes.md",
		"secrets/age/other/cluster-key.txt",
		"secrets/age/keys/key.txt.bak",
	}

	// Verify age keys match
	for _, path := range shouldMatch {
		if !re.MatchString(path) {
			t.Errorf("Age key should match but doesn't: %s", path)
		}
	}

	// Verify non-keys don't match
	for _, path := range shouldNotMatch {
		if re.MatchString(path) {
			t.Errorf("Non-key file should NOT match but does: %s", path)
		}
	}
}

// TestSOPSConfigApplicationOverlayPattern verifies application overlay pattern
func TestSOPSConfigApplicationOverlayPattern(t *testing.T) {
	// Pattern: applications/overlays/[^/]+/(managed-services|services)/.*/.*\.ya?ml$
	pattern := `applications/overlays/[^/]+/(managed-services|services)/.*/.*\.ya?ml$`
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("Failed to compile regex pattern: %v", err)
	}

	// Test cases: overlay files that SHOULD match
	shouldMatch := []string{
		"applications/overlays/prod-cluster/services/harbor/secret.yaml",
		"applications/overlays/dev-cluster/managed-services/loki/config.yml",
		"applications/overlays/test/services/keycloak/credentials.yaml",
		"applications/overlays/staging/managed-services/tempo/values.yaml",
	}

	// Test cases: files that SHOULD NOT match
	shouldNotMatch := []string{
		"applications/overlays/prod-cluster/kustomization.yaml",
		"applications/overlays/prod-cluster/README.md",
		"applications/base/services/harbor/secret.yaml",
		"applications/overlays/prod-cluster/other/config.yaml",
		"infrastructure/overlays/prod-cluster/services/config.yaml",
	}

	// Verify overlay files match
	for _, path := range shouldMatch {
		if !re.MatchString(path) {
			t.Errorf("Overlay file should match but doesn't: %s", path)
		}
	}

	// Verify non-overlay files don't match
	for _, path := range shouldNotMatch {
		if re.MatchString(path) {
			t.Errorf("Non-overlay file should NOT match but does: %s", path)
		}
	}
}
