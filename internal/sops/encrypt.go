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
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/util/errors"
)

// DefaultEncryptor implements Encryptor interface
type DefaultEncryptor struct {
	ageKeys []string
	pgpKeys []string
}

// NewDefaultEncryptor creates a new SOPS encryptor
func NewDefaultEncryptor(ageKeys, pgpKeys []string) *DefaultEncryptor {
	return &DefaultEncryptor{
		ageKeys: ageKeys,
		pgpKeys: pgpKeys,
	}
}

// EncryptFile encrypts a single file with SOPS
func (e *DefaultEncryptor) EncryptFile(ctx context.Context, filePath string, config EncryptionConfig) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &errors.StructuredError{
			Type:    errors.FileError,
			Field:   filePath,
			Message: "File does not exist",
			Suggestions: []string{
				"Check the file path is correct",
				"Ensure the file was created successfully",
				"Verify file permissions",
			},
		}
	}

	// Check if file is already encrypted
	if isEncrypted, err := e.IsFileEncrypted(filePath); err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "Failed to check if file is encrypted",
			Cause:   err,
			Suggestions: []string{
				"Check file permissions",
				"Ensure the file is readable",
				"Verify file format",
			},
		}
	} else if isEncrypted {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "File is already encrypted",
			Suggestions: []string{
				"Skip encryption for already encrypted files",
				"Use decrypt operation if you need to modify the file",
				"Check if this is the intended file",
			},
		}
	}

	// Build SOPS command
	args := []string{"-e"}

	// Add encryption keys
	if len(config.AgeKeys) > 0 {
		args = append(args, "--age", strings.Join(config.AgeKeys, ","))
	}
	if len(config.PGPKeys) > 0 {
		args = append(args, "--pgp", strings.Join(config.PGPKeys, ","))
	}

	// Use default keys if none specified
	if len(config.AgeKeys) == 0 && len(config.PGPKeys) == 0 {
		if len(e.ageKeys) > 0 {
			args = append(args, "--age", strings.Join(e.ageKeys, ","))
		}
		if len(e.pgpKeys) > 0 {
			args = append(args, "--pgp", strings.Join(e.pgpKeys, ","))
		}
	}

	// Add config file if specified
	if config.ConfigFile != "" {
		args = append(args, "--config", config.ConfigFile)
	}

	// Add in-place flag
	if config.InPlace {
		args = append(args, "-i")
	}

	// Add file path
	args = append(args, filePath)

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)

	if config.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if config.DryRun {
		fmt.Printf("Would execute: sops %s\n", strings.Join(args, " "))
		return nil
	}

	if err := cmd.Run(); err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "SOPS encryption failed",
			Cause:   err,
			Suggestions: []string{
				"Check that SOPS is installed and accessible",
				"Verify the age/PGP keys are valid",
				"Ensure the file format is supported",
				"Check SOPS configuration",
			},
		}
	}

	return nil
}

// EncryptFiles encrypts multiple files with SOPS
func (e *DefaultEncryptor) EncryptFiles(ctx context.Context, filePaths []string, config EncryptionConfig) error {
	for _, filePath := range filePaths {
		if err := e.EncryptFile(ctx, filePath, config); err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", filePath, err)
		}
	}
	return nil
}

// DecryptFile decrypts a SOPS-encrypted file
func (e *DefaultEncryptor) DecryptFile(ctx context.Context, filePath string, outputPath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &errors.StructuredError{
			Type:    errors.FileError,
			Field:   filePath,
			Message: "File does not exist",
			Suggestions: []string{
				"Check the file path is correct",
				"Ensure the file exists",
				"Verify file permissions",
			},
		}
	}

	// Check if file is encrypted
	if isEncrypted, err := e.IsFileEncrypted(filePath); err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "Failed to check if file is encrypted",
			Cause:   err,
			Suggestions: []string{
				"Check file permissions",
				"Ensure the file is readable",
				"Verify file format",
			},
		}
	} else if !isEncrypted {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "File is not encrypted",
			Suggestions: []string{
				"Encrypt the file first using SOPS",
				"Check if this is the correct file",
				"Verify the file contains SOPS metadata",
			},
		}
	}

	// Build SOPS command
	args := []string{"-d"}

	// Add output path if specified
	if outputPath != "" {
		args = append(args, "--output", outputPath)
	}

	args = append(args, filePath)

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)

	if outputPath == "" {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "SOPS decryption failed",
			Cause:   err,
			Suggestions: []string{
				"Check that SOPS is installed and accessible",
				"Verify you have access to the decryption keys",
				"Ensure the file is properly encrypted",
				"Check SOPS configuration and key files",
			},
		}
	}

	return nil
}

// IsFileEncrypted checks if a file is encrypted with SOPS
func (e *DefaultEncryptor) IsFileEncrypted(filePath string) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %w", err)
	}

	// Check for SOPS metadata
	contentStr := string(content)
	return strings.Contains(contentStr, "sops:") &&
		(strings.Contains(contentStr, "age:") || strings.Contains(contentStr, "pgp:")), nil
}

// RotateKeys rotates SOPS encryption keys
func (e *DefaultEncryptor) RotateKeys(ctx context.Context, filePath string, newAgeKeys, newPGPKeys []string) error {
	// Build SOPS command for key rotation
	args := []string{"-r"}

	// Add new encryption keys
	if len(newAgeKeys) > 0 {
		args = append(args, "--age", strings.Join(newAgeKeys, ","))
	}
	if len(newPGPKeys) > 0 {
		args = append(args, "--pgp", strings.Join(newPGPKeys, ","))
	}

	// Add in-place flag
	args = append(args, "-i", filePath)

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "SOPS key rotation failed",
			Cause:   err,
			Suggestions: []string{
				"Check that SOPS is installed and accessible",
				"Verify the new keys are valid",
				"Ensure you have access to the current decryption keys",
				"Check file permissions",
			},
		}
	}

	return nil
}

// GetEncryptedContent returns the encrypted content of a file without decrypting
func (e *DefaultEncryptor) GetEncryptedContent(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", &errors.StructuredError{
			Type:    errors.FileError,
			Field:   filePath,
			Message: "Failed to read file",
			Cause:   err,
			Suggestions: []string{
				"Check file permissions",
				"Ensure the file exists",
				"Verify file path is correct",
			},
		}
	}

	return string(content), nil
}

// EditEncryptedFile opens an encrypted file for editing with SOPS
func (e *DefaultEncryptor) EditEncryptedFile(ctx context.Context, filePath string) error {
	// Build SOPS command for editing
	args := []string{filePath}

	// Execute SOPS command
	cmd := exec.CommandContext(ctx, "sops", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "SOPS edit failed",
			Cause:   err,
			Suggestions: []string{
				"Check that SOPS is installed and accessible",
				"Verify you have access to the decryption keys",
				"Ensure the file is properly encrypted",
				"Check your default editor is set",
			},
		}
	}

	return nil
}

// Helper functions

// checkSOPSVersion checks if SOPS is available and returns version info
func checkSOPSVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "sops", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("SOPS not found or not executable: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
