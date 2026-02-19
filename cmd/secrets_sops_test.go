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

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldSkipDirectory(t *testing.T) {
	testCases := []struct {
		name     string
		dirPath  string
		expected bool
	}{
		// Directories that SHOULD be skipped
		{
			name:     "venv directory",
			dirPath:  "/path/to/project/venv",
			expected: true,
		},
		{
			name:     ".venv directory",
			dirPath:  "/path/to/project/.venv",
			expected: true,
		},
		{
			name:     "kubespray directory",
			dirPath:  "/infrastructure/clusters/test-cluster/kubespray",
			expected: true,
		},
		{
			name:     ".terraform directory",
			dirPath:  "/infrastructure/clusters/test-cluster/.terraform",
			expected: true,
		},
		{
			name:     ".bin directory",
			dirPath:  "/infrastructure/clusters/test-cluster/.bin",
			expected: true,
		},
		{
			name:     "node_modules directory",
			dirPath:  "/path/to/project/node_modules",
			expected: true,
		},
		{
			name:     ".git directory",
			dirPath:  "/path/to/project/.git",
			expected: true,
		},
		{
			name:     "__pycache__ directory",
			dirPath:  "/path/to/project/__pycache__",
			expected: true,
		},
		{
			name:     ".pytest_cache directory",
			dirPath:  "/path/to/project/.pytest_cache",
			expected: true,
		},
		{
			name:     ".mypy_cache directory",
			dirPath:  "/path/to/project/.mypy_cache",
			expected: true,
		},
		{
			name:     ".tox directory",
			dirPath:  "/path/to/project/.tox",
			expected: true,
		},
		{
			name:     "vendor directory",
			dirPath:  "/path/to/project/vendor",
			expected: true,
		},
		{
			name:     "target directory",
			dirPath:  "/path/to/project/target",
			expected: true,
		},
		{
			name:     "build directory",
			dirPath:  "/path/to/project/build",
			expected: true,
		},
		{
			name:     "dist directory",
			dirPath:  "/path/to/project/dist",
			expected: true,
		},

		// Directories that SHOULD NOT be skipped
		{
			name:     "secrets directory",
			dirPath:  "/path/to/project/secrets",
			expected: false,
		},
		{
			name:     "infrastructure directory",
			dirPath:  "/infrastructure/clusters/test-cluster",
			expected: false,
		},
		{
			name:     "applications directory",
			dirPath:  "/applications/overlays/prod-cluster",
			expected: false,
		},
		{
			name:     "config directory",
			dirPath:  "/path/to/project/config",
			expected: false,
		},
		{
			name:     "terraform directory (not .terraform)",
			dirPath:  "/path/to/project/terraform",
			expected: false,
		},
		{
			name:     "bin directory (not .bin)",
			dirPath:  "/path/to/project/bin",
			expected: false,
		},
		{
			name:     "modules directory",
			dirPath:  "/infrastructure/clusters/test-cluster/modules",
			expected: false,
		},
		{
			name:     "environments directory",
			dirPath:  "/infrastructure/clusters/test-cluster/environments",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := shouldSkipDirectory(tc.dirPath)
			if result != tc.expected {
				t.Errorf("shouldSkipDirectory(%q) = %v, expected %v", tc.dirPath, result, tc.expected)
			}
		})
	}
}

func TestShouldFileBeEncrypted(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		expected bool
	}{
		// Files that SHOULD be encrypted (based on name patterns)
		{
			name:     "file with 'secret' in name",
			filePath: "/path/to/secret.yaml",
			expected: true,
		},
		{
			name:     "file with 'credential' in name",
			filePath: "/path/to/credentials.yaml",
			expected: true,
		},
		{
			name:     "file with 'password' in name",
			filePath: "/path/to/passwords.yaml",
			expected: true,
		},
		{
			name:     "file with 'token' in name",
			filePath: "/path/to/api-token.yaml",
			expected: true,
		},
		{
			name:     "file with 'key' in name",
			filePath: "/path/to/api-key.yaml",
			expected: true,
		},
		{
			name:     "file with 'cert' in name",
			filePath: "/path/to/cert.yaml",
			expected: true,
		},
		{
			name:     "file with 'tls' in name",
			filePath: "/path/to/tls-config.yaml",
			expected: true,
		},
		{
			name:     "file with 'auth' in name",
			filePath: "/path/to/auth-config.yaml",
			expected: true,
		},

		// Files that SHOULD NOT be encrypted (based on name patterns)
		{
			name:     "regular config file",
			filePath: "/path/to/config.yaml",
			expected: false,
		},
		{
			name:     "kustomization file",
			filePath: "/path/to/kustomization.yaml",
			expected: false,
		},
		{
			name:     "deployment file",
			filePath: "/path/to/deployment.yaml",
			expected: false,
		},
		{
			name:     "service file",
			filePath: "/path/to/service.yaml",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := shouldFileBeEncrypted(tc.filePath)
			if result != tc.expected {
				t.Errorf("shouldFileBeEncrypted(%q) = %v, expected %v", tc.filePath, result, tc.expected)
			}
		})
	}
}

func TestSOPSPathMatcher(t *testing.T) {
	// Create a temporary SOPS config for testing
	tmpDir := t.TempDir()
	sopsConfigPath := filepath.Join(tmpDir, ".sops.yaml")

	sopsConfig := `creation_rules:
  - path_regex: '^infrastructure\/clusters\/test-cluster\/(?!(?:venv|kubespray|\.terraform|\.bin)\/)(.*)'
    encrypted_regex: "^(secret)$"
    age: age1test123
  - path_regex: 'secrets/ssh/(?!.*\.pub$).*'
    age: age1test123
  - path_regex: 'secrets/age/keys/.*-key\.txt'
    age: age1test123
  - path_regex: 'applications/overlays/[^/]+/(managed-services|services)/.*/.*\.ya?ml'
    encrypted_regex: "^(secret)$"
    age: age1test123
`

	if err := os.WriteFile(sopsConfigPath, []byte(sopsConfig), 0644); err != nil {
		t.Fatalf("Failed to create test SOPS config: %v", err)
	}

	// Change to temp directory for testing
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	matcher, err := NewSOPSPathMatcher(".sops.yaml")
	if err != nil {
		t.Fatalf("Failed to create SOPS path matcher: %v", err)
	}

	if len(matcher.rules) != 4 {
		t.Errorf("Expected 4 rules, got %d", len(matcher.rules))
	}

	testCases := []struct {
		name          string
		path          string
		shouldEncrypt bool
		shouldSkip    bool
		description   string
	}{
		// Infrastructure paths that SHOULD be encrypted
		{
			name:          "infrastructure main.tf",
			path:          "infrastructure/clusters/test-cluster/main.tf",
			shouldEncrypt: true,
			shouldSkip:    false,
			description:   "Main terraform file should be encrypted",
		},
		{
			name:          "infrastructure secrets",
			path:          "infrastructure/clusters/test-cluster/secrets/credentials.yaml",
			shouldEncrypt: true,
			shouldSkip:    false,
			description:   "Secrets in infrastructure should be encrypted",
		},

		// Infrastructure paths that SHOULD be skipped (excluded directories)
		{
			name:          "venv file",
			path:          "infrastructure/clusters/test-cluster/venv/lib/python3.9/site.py",
			shouldEncrypt: false,
			shouldSkip:    true,
			description:   "Files in venv should be skipped",
		},
		{
			name:          "kubespray file",
			path:          "infrastructure/clusters/test-cluster/kubespray/inventory/hosts.yaml",
			shouldEncrypt: false,
			shouldSkip:    true,
			description:   "Files in kubespray should be skipped",
		},
		{
			name:          ".terraform file",
			path:          "infrastructure/clusters/test-cluster/.terraform/providers/aws.json",
			shouldEncrypt: false,
			shouldSkip:    true,
			description:   "Files in .terraform should be skipped",
		},
		{
			name:          ".bin file",
			path:          "infrastructure/clusters/test-cluster/.bin/terraform",
			shouldEncrypt: false,
			shouldSkip:    true,
			description:   "Files in .bin should be skipped",
		},

		// SSH keys
		{
			name:          "SSH private key",
			path:          "secrets/ssh/id_rsa",
			shouldEncrypt: true,
			shouldSkip:    false,
			description:   "SSH private keys should be encrypted",
		},
		{
			name:          "SSH public key",
			path:          "secrets/ssh/id_rsa.pub",
			shouldEncrypt: false,
			shouldSkip:    true, // Public keys are excluded by negative lookahead
			description:   "SSH public keys should not be encrypted and should be skipped",
		},

		// Age keys
		{
			name:          "Age key file",
			path:          "secrets/age/keys/cluster-key.txt",
			shouldEncrypt: true,
			shouldSkip:    false,
			description:   "Age key files should be encrypted",
		},

		// Application overlays
		{
			name:          "Service secret",
			path:          "applications/overlays/prod-cluster/services/harbor/secret.yaml",
			shouldEncrypt: true,
			shouldSkip:    false,
			description:   "Service secrets should be encrypted",
		},
		{
			name:          "Managed service config",
			path:          "applications/overlays/dev-cluster/managed-services/loki/config.yml",
			shouldEncrypt: true,
			shouldSkip:    false,
			description:   "Managed service configs should be encrypted",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shouldEncrypt := matcher.ShouldEncryptPath(tc.path)
			if shouldEncrypt != tc.shouldEncrypt {
				t.Errorf("%s: ShouldEncryptPath(%q) = %v, expected %v",
					tc.description, tc.path, shouldEncrypt, tc.shouldEncrypt)
			}

			shouldSkip := matcher.ShouldSkipPath(tc.path)
			if shouldSkip != tc.shouldSkip {
				t.Errorf("%s: ShouldSkipPath(%q) = %v, expected %v",
					tc.description, tc.path, shouldSkip, tc.shouldSkip)
			}
		})
	}
}

func TestExtractBasePattern(t *testing.T) {
	testCases := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "infrastructure pattern with exclusions",
			pattern:  `^infrastructure\/clusters\/test-cluster\/(?!(?:venv|kubespray|\.terraform|\.bin)\/)(.*)`,
			expected: "infrastructure/clusters/test-cluster/",
		},
		{
			name:     "SSH pattern with exclusions",
			pattern:  `secrets/ssh/(?!.*\.pub$).*`,
			expected: "secrets/ssh/",
		},
		{
			name:     "pattern without exclusions",
			pattern:  `secrets/age/keys/.*-key\.txt`,
			expected: "",
		},
		{
			name:     "empty pattern",
			pattern:  "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractBasePattern(tc.pattern)
			if result != tc.expected {
				t.Errorf("extractBasePattern(%q) = %q, expected %q", tc.pattern, result, tc.expected)
			}
		})
	}
}

func TestNewSOPSPathMatcherNoConfig(t *testing.T) {
	// Test with non-existent config file
	matcher, err := NewSOPSPathMatcher("/nonexistent/.sops.yaml")
	if err != nil {
		t.Errorf("Expected no error for non-existent config, got: %v", err)
	}

	if matcher == nil {
		t.Error("Expected non-nil matcher")
	}

	if len(matcher.rules) != 0 {
		t.Errorf("Expected 0 rules for non-existent config, got %d", len(matcher.rules))
	}
}
