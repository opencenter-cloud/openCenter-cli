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

package gitops

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// ManifestValidator validates generated GitOps manifests for common issues
type ManifestValidator struct {
	gitDir           string
	errors           []string
	validationEngine *validation.ValidationEngine
	fileValidator    *validators.FileValidator
	fileSystem       fs.FileSystem
}

// NewManifestValidator creates a new manifest validator
func NewManifestValidator(gitDir string) *ManifestValidator {
	engine := validation.NewValidationEngine()
	fileValidator := validators.NewFileValidator()
	engine.MustRegister(fileValidator)

	// Create FileSystem instance
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	return &ManifestValidator{
		gitDir:           gitDir,
		errors:           []string{},
		validationEngine: engine,
		fileValidator:    fileValidator,
		fileSystem:       fileSystem,
	}
}

// Validate runs all validation checks on generated manifests
func (v *ManifestValidator) Validate() error {
	v.errors = []string{}

	// Run all validation checks
	v.validateFluxCDManifests()
	v.validateGitRepositories()
	v.validateCertManager()
	v.validateGateway()
	v.validateVSphereCSI()
	v.validateMetalLB()
	v.validateHeadlamp()
	v.validateSecurityPolicy()

	if len(v.errors) > 0 {
		return fmt.Errorf("manifest validation failed with %d errors:\n%s",
			len(v.errors), strings.Join(v.errors, "\n"))
	}

	return nil
}

func (v *ManifestValidator) validateSecurityPolicy() {
	findings, err := ScanGitOpsSecrets(v.gitDir)
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("security scan failed: %v", err))
		return
	}
	for _, finding := range findings {
		// Skip encryption-related findings — secrets are encrypted separately
		// via 'opencenter secrets encrypt' before deploy.
		switch finding.Rule {
		case "unencrypted-kubernetes-secret", "plaintext-secret-field", "invalid-sops-metadata":
			continue
		}
		v.errors = append(v.errors, fmt.Sprintf("%s: %s: %s", finding.Path, finding.Rule, finding.Message))
	}
}

// validateFile validates a file using the ValidationEngine
func (v *ManifestValidator) validateFile(file string, operation string) {
	ctx := context.Background()
	result, err := v.validationEngine.Validate(ctx, "file", map[string]interface{}{
		"path":      file,
		"operation": operation,
	})

	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s: validation error: %v", file, err))
		return
	}

	// Add errors from validation result
	for _, issue := range result.Errors {
		v.errors = append(v.errors, fmt.Sprintf("%s: %s", file, issue.Message))
	}
}

// validateFluxCDManifests validates FluxCD Kustomization manifests
func (v *ManifestValidator) validateFluxCDManifests() {
	pattern := filepath.Join(v.gitDir, "applications/overlays/*/services/fluxcd/*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("failed to glob FluxCD manifests: %v", err))
		return
	}

	for _, file := range files {
		v.validateFluxCDKustomization(file)
	}
}

// validateFluxCDKustomization validates a single FluxCD Kustomization manifest
func (v *ManifestValidator) validateFluxCDKustomization(file string) {
	// Use ValidationEngine for basic file validation
	v.validateFile(file, "read")

	data, err := v.fileSystem.ReadFile(file)
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s: failed to read: %v", file, err))
		return
	}

	var manifest map[string]interface{}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s: invalid YAML: %v", file, err))
		return
	}

	// Check for proper indentation (2 spaces)
	if !v.hasProperIndentation(string(data)) {
		v.errors = append(v.errors, fmt.Sprintf("%s: improper indentation (must use 2 spaces)", file))
	}

	// Check interval is valid for kustomizations (5m or 15m are acceptable)
	if spec, ok := manifest["spec"].(map[string]interface{}); ok {
		if interval, ok := spec["interval"].(string); ok {
			if interval != "5m" && interval != "15m" {
				v.errors = append(v.errors, fmt.Sprintf("%s: interval should be 5m or 15m, got %s", file, interval))
			}
		}
	}

	// Check for hardcoded cluster names
	content := string(data)
	if strings.Contains(content, "dev-cluster") || strings.Contains(content, "stage-cluster") {
		v.errors = append(v.errors, fmt.Sprintf("%s: contains hardcoded cluster name (dev-cluster or stage-cluster)", file))
	}

	// Check dependsOn is properly indented
	if strings.Contains(content, "dependsOn:") {
		if !v.hasProperdependsOnIndentation(content) {
			v.errors = append(v.errors, fmt.Sprintf("%s: dependsOn block has improper indentation", file))
		}
	}

	// Check decryption block is properly indented
	if strings.Contains(content, "decryption:") {
		if !v.hasProperDecryptionIndentation(content) {
			v.errors = append(v.errors, fmt.Sprintf("%s: decryption block has improper indentation", file))
		}
	}
}

// validateGitRepositories validates GitRepository manifests
func (v *ManifestValidator) validateGitRepositories() {
	pattern := filepath.Join(v.gitDir, "applications/overlays/*/services/sources/*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("failed to glob GitRepository manifests: %v", err))
		return
	}

	for _, file := range files {
		v.validateGitRepository(file)
	}
}

// validateGitRepository validates a single GitRepository manifest
func (v *ManifestValidator) validateGitRepository(file string) {
	// Use ValidationEngine for basic file validation
	v.validateFile(file, "read")

	data, err := v.fileSystem.ReadFile(file)
	if err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s: failed to read: %v", file, err))
		return
	}

	var manifest map[string]interface{}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s: invalid YAML: %v", file, err))
		return
	}

	content := string(data)

	// Check repository URL capitalization (openCenter not opencenter)
	if strings.Contains(content, "opencenter-gitops-base") {
		v.errors = append(v.errors, fmt.Sprintf("%s: repository URL should use 'openCenter-gitops-base' (capital C)", file))
	}

	// Check interval is 15m for GitRepositories
	if spec, ok := manifest["spec"].(map[string]interface{}); ok {
		if interval, ok := spec["interval"].(string); ok {
			if interval != "15m" {
				v.errors = append(v.errors, fmt.Sprintf("%s: interval should be 15m for GitRepository, got %s", file, interval))
			}
		}

		// Check ref uses branch not tag
		if ref, ok := spec["ref"].(map[string]interface{}); ok {
			if _, hasTag := ref["tag"]; hasTag {
				v.errors = append(v.errors, fmt.Sprintf("%s: should use 'branch: main' not 'tag: v0.1.0'", file))
			}
		}
	}

	// Check for proper indentation
	if !v.hasProperIndentation(content) {
		v.errors = append(v.errors, fmt.Sprintf("%s: improper indentation (must use 2 spaces)", file))
	}
}

// validateCertManager validates cert-manager specific issues
func (v *ManifestValidator) validateCertManager() {
	// Check for plaintext secrets
	secretPattern := filepath.Join(v.gitDir, "applications/overlays/*/services/cert-manager/*secret*.yaml")
	files, err := filepath.Glob(secretPattern)
	if err != nil {
		return
	}

	for _, file := range files {
		// Use ValidationEngine for basic file validation
		v.validateFile(file, "read")

		data, err := v.fileSystem.ReadFile(file)
		if err != nil {
			continue
		}

		var secret map[string]interface{}
		if err := yaml.Unmarshal(data, &secret); err != nil {
			continue
		}

		// Check if data fields are base64 encoded
		if dataMap, ok := secret["data"].(map[string]interface{}); ok {
			for key, value := range dataMap {
				if strValue, ok := value.(string); ok {
					if !strings.HasPrefix(strings.TrimSpace(strValue), "ENC[") && !v.isBase64(strValue) {
						v.errors = append(v.errors, fmt.Sprintf("%s: secret field '%s' is not base64 encoded", file, key))
					}
				}
			}
		}
	}

	// Check issuer selectors
	issuerPattern := filepath.Join(v.gitDir, "applications/overlays/*/services/cert-manager/*issuer*.yaml")
	files, err = filepath.Glob(issuerPattern)
	if err != nil {
		return
	}

	for _, file := range files {
		// Use ValidationEngine for basic file validation
		v.validateFile(file, "read")

		data, err := v.fileSystem.ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)
		// Check for incorrect domain patterns
		if strings.Contains(content, ".farmcreditfunding.com") {
			v.errors = append(v.errors, fmt.Sprintf("%s: issuer uses incorrect domain (.farmcreditfunding.com instead of .k8s.opencenter.cloud)", file))
		}
	}

	// Check kustomization.yaml secretGenerator indentation
	kustomizationFile := filepath.Join(v.gitDir, "applications/overlays/*/services/cert-manager/kustomization.yaml")
	files, err = filepath.Glob(kustomizationFile)
	if err != nil {
		return
	}

	for _, file := range files {
		// Use ValidationEngine for basic file validation
		v.validateFile(file, "read")

		data, err := v.fileSystem.ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)
		if strings.Contains(content, "secretGenerator:") {
			if !v.hasProperSecretGeneratorIndentation(content) {
				v.errors = append(v.errors, fmt.Sprintf("%s: secretGenerator options not properly indented", file))
			}
		}
	}
}

// validateGateway validates gateway configuration
func (v *ManifestValidator) validateGateway() {
	pattern := filepath.Join(v.gitDir, "applications/overlays/*/services/gateway/*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, file := range files {
		// Use ValidationEngine for basic file validation
		v.validateFile(file, "read")

		data, err := v.fileSystem.ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)

		// Check hostname format (should include org prefix)
		hostnameRegex := regexp.MustCompile(`hostname:\s*([^\s]+)`)
		matches := hostnameRegex.FindStringSubmatch(content)
		if len(matches) > 1 {
			hostname := matches[1]
			// Should be like: auth.fcc.k8s-qa.ord1.k8s.opencenter.cloud
			// Not: auth.k8s-qa.ord1.k8s.opencenter.cloud
			parts := strings.Split(hostname, ".")
			if len(parts) >= 3 && parts[1] == "k8s-qa" {
				// Missing org prefix
				v.errors = append(v.errors, fmt.Sprintf("%s: hostname missing organization prefix (should be like auth.fcc.k8s-qa...)", file))
			}
		}
	}

	// Check HTTPRoute for port 80 listener
	httproutePattern := filepath.Join(v.gitDir, "applications/overlays/*/services/*/httproute*.yaml")
	files, err = filepath.Glob(httproutePattern)
	if err != nil {
		return
	}

	for _, file := range files {
		// Use ValidationEngine for basic file validation
		v.validateFile(file, "read")

		data, err := v.fileSystem.ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)
		// Check for both http and https listeners
		hasHTTP := strings.Contains(content, "sectionName: http")
		hasHTTPS := strings.Contains(content, "sectionName: https")

		if hasHTTPS && !hasHTTP {
			v.errors = append(v.errors, fmt.Sprintf("%s: missing port 80 listener with redirect to 443", file))
		}
	}
}

// validateVSphereCSI validates vSphere CSI configuration
func (v *ManifestValidator) validateVSphereCSI() {
	pattern := filepath.Join(v.gitDir, "applications/overlays/*/services/vsphere-csi/*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, file := range files {
		// Use ValidationEngine for basic file validation
		v.validateFile(file, "read")

		data, err := v.fileSystem.ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)

		// Check snapshotter version
		if strings.Contains(content, "csi-snapshotter") {
			if strings.Contains(content, "tag: v3.3.0") {
				v.errors = append(v.errors, fmt.Sprintf("%s: snapshotter version should be v8.2.0, not v3.3.0", file))
			}
		}

		// Check registry
		if strings.Contains(content, "registry.k8s.io/csi-vsphere") {
			v.errors = append(v.errors, fmt.Sprintf("%s: registry should be 'registry.k8s.io' not 'registry.k8s.io/csi-vsphere'", file))
		}

		// Check for datastoreURL in storage class
		if strings.Contains(content, "kind: StorageClass") {
			if !strings.Contains(content, "datastoreurl:") && !strings.Contains(content, "datastoreURL:") {
				v.errors = append(v.errors, fmt.Sprintf("%s: StorageClass missing datastoreURL parameter", file))
			}
		}

		// Check for formatting gaps in storage class
		if strings.Contains(content, "kind: StorageClass") {
			lines := strings.Split(content, "\n")
			for i, line := range lines {
				if strings.TrimSpace(line) == "kind: StorageClass" {
					// Check if there's a blank line before this (between apiVersion and kind)
					if i > 0 && strings.TrimSpace(lines[i-1]) == "" {
						v.errors = append(v.errors, fmt.Sprintf("%s: StorageClass has formatting gap (extra blank line)", file))
					}
				}
			}
		}
	}
}

// validateMetalLB validates MetalLB configuration
func (v *ManifestValidator) validateMetalLB() {
	pattern := filepath.Join(v.gitDir, "applications/overlays/*/services/metallb/*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, file := range files {
		// Use ValidationEngine for basic file validation
		v.validateFile(file, "read")

		data, err := v.fileSystem.ReadFile(file)
		if err != nil {
			continue
		}

		var manifest map[string]interface{}
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			continue
		}

		// Check IPAddressPool addresses
		if kind, ok := manifest["kind"].(string); ok && kind == "IPAddressPool" {
			if spec, ok := manifest["spec"].(map[string]interface{}); ok {
				if addresses, ok := spec["addresses"].([]interface{}); ok {
					for _, addr := range addresses {
						if addrStr, ok := addr.(string); ok {
							// Validate IP range format
							if !v.isValidIPRange(addrStr) {
								v.errors = append(v.errors, fmt.Sprintf("%s: invalid IP address range: %s", file, addrStr))
							}
						}
					}
				}
			}
		}
	}
}

// validateHeadlamp validates Headlamp configuration
func (v *ManifestValidator) validateHeadlamp() {
	pattern := filepath.Join(v.gitDir, "applications/overlays/*/services/headlamp/*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, file := range files {
		// Use ValidationEngine for basic file validation
		v.validateFile(file, "read")

		data, err := v.fileSystem.ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)

		// Check HTTPRoute URL
		if strings.Contains(content, "kind: HTTPRoute") {
			// Should use dashboard.fcc.k8s-qa... not dashboard.k8s-qa...
			if strings.Contains(content, "dashboard.k8s-qa") && !strings.Contains(content, "dashboard.fcc.k8s-qa") {
				v.errors = append(v.errors, fmt.Sprintf("%s: HTTPRoute URL missing organization prefix", file))
			}
		}
	}
}

// Helper functions

func (v *ManifestValidator) hasProperIndentation(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if len(line) == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		// Check if line starts with spaces (not tabs)
		if strings.HasPrefix(line, "\t") {
			return false
		}
		// Check if indentation is multiple of 2
		spaces := 0
		for _, ch := range line {
			if ch == ' ' {
				spaces++
			} else {
				break
			}
		}
		if spaces > 0 && spaces%2 != 0 {
			return false
		}
	}
	return true
}

func (v *ManifestValidator) hasProperdependsOnIndentation(content string) bool {
	// Check for pattern:
	// dependsOn:
	//   - name: something
	//     namespace: something
	// The list item must be indented with at least 2 spaces
	// and properties under the list item must be indented further
	pattern := regexp.MustCompile(`dependsOn:\s*\n {2,}-\s+name:[^\n]*\n {4,}namespace:`)
	return pattern.MatchString(content)
}

func (v *ManifestValidator) hasProperDecryptionIndentation(content string) bool {
	// Check for pattern:
	// decryption:
	//   provider: sops
	//   secretRef:
	//     name: sops-age
	// Each level must be properly indented (at least 2 spaces per level)
	pattern := regexp.MustCompile(`decryption:\s*\n {2,}provider:\s+sops\s*\n {2,}secretRef:\s*\n {4,}name:`)
	return pattern.MatchString(content)
}

func (v *ManifestValidator) hasProperSecretGeneratorIndentation(content string) bool {
	// Check for pattern:
	// secretGenerator:
	//   - name: something
	//     options:
	//       disableNameSuffixHash: true
	pattern := regexp.MustCompile(`secretGenerator:\s*\n\s+-\s+name:.*\n.*\n\s+options:\s*\n\s+disableNameSuffixHash:`)
	return pattern.MatchString(content)
}

func (v *ManifestValidator) isBase64(s string) bool {
	// Simple check: base64 strings don't contain spaces and are alphanumeric + / + =
	if strings.Contains(s, " ") {
		return false
	}
	base64Regex := regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
	return base64Regex.MatchString(s)
}

func (v *ManifestValidator) isValidIPRange(s string) bool {
	// Check format: IP-IP or CIDR
	if strings.Contains(s, "-") {
		parts := strings.Split(s, "-")
		if len(parts) != 2 {
			return false
		}
		return v.isValidIP(strings.TrimSpace(parts[0])) && v.isValidIP(strings.TrimSpace(parts[1]))
	}
	if strings.Contains(s, "/") {
		// CIDR notation
		parts := strings.Split(s, "/")
		return len(parts) == 2 && v.isValidIP(parts[0])
	}
	return v.isValidIP(s)
}

func (v *ManifestValidator) isValidIP(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		// Simple validation - just check it's numeric
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}
