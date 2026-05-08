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
	"os/exec"
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

	// Create tracked hooks directory if it doesn't exist
	hooksDir := filepath.Join(absRepoPath, ".opencenter", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Generate hook script
	hookScript := h.generateHookScript(cluster)

	// Write pre-commit hook
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte(hookScript), 0o755); err != nil {
		return fmt.Errorf("failed to write pre-commit hook: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "config", "core.hooksPath", ".opencenter/hooks")
	cmd.Dir = absRepoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure core.hooksPath: %w: %s", err, strings.TrimSpace(string(output)))
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
	hookPath := filepath.Join(absRepoPath, ".opencenter", "hooks", "pre-commit")
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
	return fmt.Sprintf(`#!/usr/bin/env sh
# openCenter pre-commit hook for secrets validation
# Generated by openCenter secrets hook support
# Cluster: %s

set -eu

if [ "${OPENCENTER_SKIP_HOOKS:-}" = "1" ]; then
    echo "WARNING: Pre-commit hooks bypassed via OPENCENTER_SKIP_HOOKS" >&2
    exit 0
fi

if ! command -v opencenter >/dev/null 2>&1; then
    echo "ERROR: opencenter CLI not found; cannot run security validation" >&2
    exit 1
fi

repo_root=$(git rev-parse --show-toplevel)
opencenter cluster validate-manifests --repo-path "$repo_root" --staged --security-only

# All checks passed
exit 0
`, cluster)
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
