/*
Copyright 2025.

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

package secrets

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// DefaultHookManager implements the HookManager interface.
// It provides methods for installing and managing Git pre-commit hooks
// that validate secrets before commits.
type DefaultHookManager struct {
	secretsManager SecretsManager
	logger         *slog.Logger
}

// NewDefaultHookManager creates a new DefaultHookManager with the given dependencies.
//
// Parameters:
//   - secretsManager: Manager for secrets validation operations
//   - logger: Logger for operation tracking
//
// Returns:
//   - *DefaultHookManager: A new hook manager instance
func NewDefaultHookManager(
	secretsManager SecretsManager,
	logger *slog.Logger,
) *DefaultHookManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultHookManager{
		secretsManager: secretsManager,
		logger:         logger,
	}
}

// InstallHooks installs pre-commit hooks in the repository.
// The hooks validate staged files for unencrypted secrets and drift.
func (h *DefaultHookManager) InstallHooks(ctx context.Context, repoPath string, cluster string) error {
	h.logger.Info("Installing pre-commit hooks", "repo_path", repoPath, "cluster", cluster)

	// Resolve absolute path
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve repository path: %w", err)
	}

	// Check if .git directory exists
	gitDir := filepath.Join(absRepoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", absRepoPath)
	}

	// Create hooks directory if it doesn't exist
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Generate hook script
	hookScript := h.generateHookScript(cluster)

	// Write pre-commit hook
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		return fmt.Errorf("failed to write pre-commit hook: %w", err)
	}

	h.logger.Info("Pre-commit hooks installed successfully", "hook_path", hookPath)
	return nil
}

// ValidatePreCommit runs pre-commit validation on staged files.
// Returns a HookResult indicating whether the commit should proceed.
func (h *DefaultHookManager) ValidatePreCommit(ctx context.Context, stagedFiles []string) (*HookResult, error) {
	h.logger.Info("Running pre-commit validation", "staged_files_count", len(stagedFiles))

	result := &HookResult{
		Passed:           true,
		UnencryptedFiles: []string{},
		DriftDetected:    []string{},
		PlaintextKeys:    []string{},
		Warnings:         []string{},
	}

	// Check for plaintext key files and unencrypted secrets
	for _, file := range stagedFiles {
		// Normalize path for pattern matching (use relative path if absolute)
		normalizedFile := file
		if filepath.IsAbs(file) {
			// For absolute paths, we still need to check the pattern
			// Extract the relative portion for pattern matching
			normalizedFile = file
		}

		if h.isPlaintextKeyFile(normalizedFile) {
			result.PlaintextKeys = append(result.PlaintextKeys, file)
			result.Passed = false
		}

		// Check for unencrypted secrets in manifest files
		if h.isManifestFile(normalizedFile) {
			isEncrypted, err := h.checkFileEncryption(file)
			if err != nil {
				h.logger.Warn("Failed to check encryption status", "file", file, "error", err)
				result.Warnings = append(result.Warnings, fmt.Sprintf("Could not verify encryption for %s: %v", file, err))
				continue
			}

			if !isEncrypted {
				result.UnencryptedFiles = append(result.UnencryptedFiles, file)
				result.Passed = false
			}
		}
	}

	h.logger.Info("Pre-commit validation completed",
		"passed", result.Passed,
		"unencrypted_files", len(result.UnencryptedFiles),
		"plaintext_keys", len(result.PlaintextKeys),
		"warnings", len(result.Warnings))

	return result, nil
}

// UninstallHooks removes installed hooks from the repository.
func (h *DefaultHookManager) UninstallHooks(ctx context.Context, repoPath string) error {
	h.logger.Info("Uninstalling pre-commit hooks", "repo_path", repoPath)

	// Resolve absolute path
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve repository path: %w", err)
	}

	// Remove pre-commit hook
	hookPath := filepath.Join(absRepoPath, ".git", "hooks", "pre-commit")
	if err := os.Remove(hookPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove pre-commit hook: %w", err)
	}

	h.logger.Info("Pre-commit hooks uninstalled successfully")
	return nil
}

// Helper methods

// generateHookScript generates the pre-commit hook script content.
// The script validates staged files for unencrypted secrets and plaintext keys.
func (h *DefaultHookManager) generateHookScript(cluster string) string {
	return fmt.Sprintf(`#!/bin/bash
# openCenter pre-commit hook for secrets validation
# Generated by openCenter secrets hook support
# Cluster: %s

# Check if hook bypass is enabled
if [ -n "$OPENCENTER_SKIP_HOOKS" ]; then
    echo "⚠️  WARNING: Pre-commit hooks bypassed via OPENCENTER_SKIP_HOOKS"
    exit 0
fi

# Get list of staged files
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM)

# Check for plaintext Age keys
if echo "$STAGED_FILES" | grep -q "secrets/age/.*\.txt$"; then
    echo "❌ ERROR: Plaintext Age key file detected in staged changes"
    echo "   Age keys should never be committed to Git"
    echo "   Files:"
    echo "$STAGED_FILES" | grep "secrets/age/.*\.txt$" | sed 's/^/     - /'
    echo ""
    echo "   To bypass this check (NOT RECOMMENDED):"
    echo "   OPENCENTER_SKIP_HOOKS=1 git commit"
    exit 1
fi

# Check for plaintext SSH keys
if echo "$STAGED_FILES" | grep -q "secrets/ssh/.*_rsa$\|secrets/ssh/.*_ed25519$"; then
    echo "❌ ERROR: Plaintext SSH key file detected in staged changes"
    echo "   SSH keys should never be committed to Git"
    echo "   Files:"
    echo "$STAGED_FILES" | grep "secrets/ssh/.*_rsa$\|secrets/ssh/.*_ed25519$" | sed 's/^/     - /'
    echo ""
    echo "   To bypass this check (NOT RECOMMENDED):"
    echo "   OPENCENTER_SKIP_HOOKS=1 git commit"
    exit 1
fi

# Check for unencrypted secrets in manifest files
UNENCRYPTED_FILES=""
for file in $STAGED_FILES; do
    # Check if file is a secret manifest
    if [[ "$file" == applications/overlays/*/services/*/secret.yaml ]]; then
        # Check if file contains SOPS metadata
        if ! git show ":$file" | grep -q "sops:"; then
            UNENCRYPTED_FILES="$UNENCRYPTED_FILES\n     - $file"
        fi
    fi
done

if [ -n "$UNENCRYPTED_FILES" ]; then
    echo "❌ ERROR: Unencrypted secret files detected in staged changes"
    echo "   All secret manifests must be SOPS-encrypted"
    echo "   Files:"
    echo -e "$UNENCRYPTED_FILES"
    echo ""
    echo "   To encrypt secrets, run:"
    echo "   opencenter secrets sync %s"
    echo ""
    echo "   To bypass this check (NOT RECOMMENDED):"
    echo "   OPENCENTER_SKIP_HOOKS=1 git commit"
    exit 1
fi

# Check for configuration drift in staged manifests
# Only check if opencenter CLI is available
if command -v opencenter &> /dev/null; then
    # Check if any secret manifests are staged
    SECRET_MANIFESTS=$(echo "$STAGED_FILES" | grep "applications/overlays/.*/services/.*/secret.yaml" || true)
    
    if [ -n "$SECRET_MANIFESTS" ]; then
        # Run drift detection (suppress output, only check exit code)
        if ! opencenter secrets validate %s --quiet &> /dev/null; then
            echo "❌ ERROR: Configuration drift detected in staged secret manifests"
            echo "   Staged manifests do not match the cluster configuration file"
            echo ""
            echo "   To fix drift, run:"
            echo "   opencenter secrets sync %s"
            echo ""
            echo "   Or to see detailed drift report:"
            echo "   opencenter secrets validate %s"
            echo ""
            echo "   To bypass this check (NOT RECOMMENDED):"
            echo "   OPENCENTER_SKIP_HOOKS=1 git commit"
            exit 1
        fi
    fi
fi

# All checks passed
exit 0
`, cluster, cluster, cluster, cluster, cluster)
}

// isPlaintextKeyFile checks if a file path represents a plaintext key file.
// Returns true for Age private keys (.txt) and SSH private keys.
func (h *DefaultHookManager) isPlaintextKeyFile(filePath string) bool {
	// Check for Age private key files
	if strings.Contains(filePath, "secrets/age/") && strings.HasSuffix(filePath, ".txt") {
		return true
	}

	// Check for SSH private key files (without .pub extension)
	if strings.Contains(filePath, "secrets/ssh/") {
		// SSH private keys typically don't have extensions or have _rsa, _ed25519, etc.
		if !strings.HasSuffix(filePath, ".pub") {
			base := filepath.Base(filePath)
			// Check for common SSH key patterns
			if strings.Contains(base, "_rsa") || strings.Contains(base, "_ed25519") ||
				strings.Contains(base, "_ecdsa") || strings.Contains(base, "_dsa") ||
				base == "id_rsa" || base == "id_ed25519" || base == "id_ecdsa" || base == "id_dsa" {
				return true
			}
		}
	}

	return false
}

// isManifestFile checks if a file path represents a secret manifest file.
// Returns true for files matching the pattern: applications/overlays/*/services/*/secret.yaml
// Handles both relative and absolute paths.
func (h *DefaultHookManager) isManifestFile(filePath string) bool {
	// Normalize path separators
	normalizedPath := filepath.ToSlash(filePath)

	// Check if path contains the manifest pattern
	// Pattern: applications/overlays/<cluster>/services/<service>/secret.yaml
	// We need to find this pattern anywhere in the path (for absolute paths)

	// Split the path and look for the pattern
	parts := strings.Split(normalizedPath, "/")

	// Find "applications" in the path
	appIndex := -1
	for i, part := range parts {
		if part == "applications" {
			appIndex = i
			break
		}
	}

	if appIndex == -1 {
		return false
	}

	// Check if we have enough parts after "applications"
	// Need: applications/overlays/<cluster>/services/<service>/secret.yaml (6 parts total)
	if len(parts) < appIndex+6 {
		return false
	}

	// Verify the pattern from the applications index
	return parts[appIndex] == "applications" &&
		parts[appIndex+1] == "overlays" &&
		parts[appIndex+3] == "services" &&
		parts[len(parts)-1] == "secret.yaml"
}

// checkFileEncryption checks if a file is SOPS-encrypted.
// Returns true if the file contains SOPS metadata, false otherwise.
func (h *DefaultHookManager) checkFileEncryption(filePath string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %w", err)
	}

	// Check for SOPS metadata in the file
	// SOPS-encrypted files contain a "sops:" section with metadata
	content := string(data)
	return strings.Contains(content, "sops:") &&
		(strings.Contains(content, "mac:") || strings.Contains(content, "lastmodified:")), nil
}
