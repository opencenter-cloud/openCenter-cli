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
	"path/filepath"
	"strings"
	"sync"

	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// DefaultEncryptor implements Encryptor interface
type DefaultEncryptor struct {
	ageKeys    []string
	pgpKeys    []string
	fileSystem fs.FileSystem
	runner     security.CommandRunner
}

// NewDefaultEncryptor creates a new SOPS encryptor
func NewDefaultEncryptor(ageKeys, pgpKeys []string) *DefaultEncryptor {
	// Create FileSystem instance
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	return &DefaultEncryptor{
		ageKeys:    ageKeys,
		pgpKeys:    pgpKeys,
		fileSystem: fileSystem,
		runner:     security.GetDefaultCommandRunner(),
	}
}

func (e *DefaultEncryptor) prepareSOPSCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	cmd, err := e.runner.PrepareCommandContext(ctx, "sops", args...)
	if err != nil {
		return nil, err
	}
	cmd.Env = os.Environ()
	return cmd, nil
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

	// If the file extension is not recognized by SOPS as YAML (e.g. .yaml.enc),
	// explicitly tell SOPS to treat it as YAML.
	if needsExplicitYAMLType(filePath) {
		args = append(args, "--input-type", "yaml", "--output-type", "yaml")
	}

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
	cmd, err := e.prepareSOPSCommand(ctx, args...)
	if err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "Failed to prepare SOPS command",
			Cause:   err,
		}
	}

	if config.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if config.DryRun {
		fmt.Printf("Would execute: sops %s\n", strings.Join(args, " "))
		return nil
	}

	// Capture stderr for diagnostic output
	var stderrBuf strings.Builder
	if !config.Verbose {
		cmd.Stderr = &stderrBuf
	}

	if err := cmd.Run(); err != nil {
		stderrOutput := strings.TrimSpace(stderrBuf.String())

		// Build a diagnostic message with the actual sops output
		message := "SOPS encryption failed"
		if stderrOutput != "" {
			message = fmt.Sprintf("SOPS encryption failed: %s", stderrOutput)
		}

		suggestions := []string{
			fmt.Sprintf("Command: sops %s", strings.Join(args, " ")),
		}

		// Check for missing SOPS_AGE_KEY_FILE
		if os.Getenv("SOPS_AGE_KEY_FILE") == "" {
			suggestions = append(suggestions,
				"SOPS_AGE_KEY_FILE is not set. Export it before running:",
				fmt.Sprintf("  export SOPS_AGE_KEY_FILE=<path-to-age-key>"),
				"Or activate the cluster environment first:",
				fmt.Sprintf("  eval $(opencenter cluster env %s)", filepath.Base(filepath.Dir(filepath.Dir(filePath)))),
			)
		} else {
			keyFile := os.Getenv("SOPS_AGE_KEY_FILE")
			if _, statErr := os.Stat(keyFile); os.IsNotExist(statErr) {
				suggestions = append(suggestions,
					fmt.Sprintf("SOPS_AGE_KEY_FILE points to a missing file: %s", keyFile),
				)
			}
		}

		// Check for missing .sops.yaml
		if config.ConfigFile != "" {
			if _, statErr := os.Stat(config.ConfigFile); os.IsNotExist(statErr) {
				suggestions = append(suggestions,
					fmt.Sprintf("SOPS config file not found: %s", config.ConfigFile),
				)
			}
		}

		return &errors.StructuredError{
			Type:        errors.SOPSError,
			Field:       filePath,
			Message:     message,
			Cause:       err,
			Suggestions: suggestions,
		}
	}

	return nil
}

// EncryptFiles encrypts multiple files with SOPS sequentially
func (e *DefaultEncryptor) EncryptFiles(ctx context.Context, filePaths []string, config EncryptionConfig) error {
	for _, filePath := range filePaths {
		if err := e.EncryptFile(ctx, filePath, config); err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", filePath, err)
		}
	}
	return nil
}

// EncryptFilesParallel encrypts multiple files with SOPS in parallel
func (e *DefaultEncryptor) EncryptFilesParallel(ctx context.Context, filePaths []string, config EncryptionConfig, maxConcurrency int) error {
	if maxConcurrency <= 0 {
		maxConcurrency = 4 // Default to 4 concurrent operations
	}

	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, maxConcurrency)
	errChan := make(chan error, len(filePaths))

	// Use a wait group to track completion
	var wg sync.WaitGroup

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Encrypt the file
			if err := e.EncryptFile(ctx, path, config); err != nil {
				errChan <- fmt.Errorf("failed to encrypt %s: %w", path, err)
			}
		}(filePath)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to encrypt %d files: %v", len(errors), errors)
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

	// If the file extension is not recognized by SOPS as YAML (e.g. .yaml.enc),
	// explicitly tell SOPS to treat it as YAML.
	if needsExplicitYAMLType(filePath) {
		args = append(args, "--input-type", "yaml", "--output-type", "yaml")
	}

	// Add output path if specified
	if outputPath != "" {
		args = append(args, "--output", outputPath)
	}

	args = append(args, filePath)

	// Execute SOPS command
	cmd, err := e.prepareSOPSCommand(ctx, args...)
	if err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "Failed to prepare SOPS command",
			Cause:   err,
		}
	}

	// Capture stderr for diagnostic output
	var stderrBuf strings.Builder
	if outputPath == "" {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		stderrOutput := strings.TrimSpace(stderrBuf.String())

		message := "SOPS decryption failed"
		if stderrOutput != "" {
			message = fmt.Sprintf("SOPS decryption failed: %s", stderrOutput)
		}

		suggestions := []string{
			fmt.Sprintf("Command: sops %s", strings.Join(args, " ")),
		}

		if os.Getenv("SOPS_AGE_KEY_FILE") == "" {
			suggestions = append(suggestions,
				"SOPS_AGE_KEY_FILE is not set. Export it before running:",
				"  export SOPS_AGE_KEY_FILE=<path-to-age-key>",
				"Or activate the cluster environment first:",
				"  eval $(opencenter cluster env <org/cluster>)",
			)
		} else {
			keyFile := os.Getenv("SOPS_AGE_KEY_FILE")
			if _, statErr := os.Stat(keyFile); os.IsNotExist(statErr) {
				suggestions = append(suggestions,
					fmt.Sprintf("SOPS_AGE_KEY_FILE points to a missing file: %s", keyFile),
				)
			}
		}

		return &errors.StructuredError{
			Type:        errors.SOPSError,
			Field:       filePath,
			Message:     message,
			Cause:       err,
			Suggestions: suggestions,
		}
	}

	return nil
}

// IsFileEncrypted checks if a file is encrypted with SOPS
func (e *DefaultEncryptor) IsFileEncrypted(filePath string) (bool, error) {
	content, err := e.fileSystem.ReadFile(filePath)
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

	// If the file extension is not recognized by SOPS as YAML (e.g. .yaml.enc),
	// explicitly tell SOPS to treat it as YAML.
	if needsExplicitYAMLType(filePath) {
		args = append(args, "--input-type", "yaml", "--output-type", "yaml")
	}

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
	cmd, err := e.prepareSOPSCommand(ctx, args...)
	if err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "Failed to prepare SOPS command",
			Cause:   err,
		}
	}

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
	content, err := e.fileSystem.ReadFile(filePath)
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
	args := []string{}

	// If the file extension is not recognized by SOPS as YAML (e.g. .yaml.enc),
	// explicitly tell SOPS to treat it as YAML.
	if needsExplicitYAMLType(filePath) {
		args = append(args, "--input-type", "yaml", "--output-type", "yaml")
	}

	args = append(args, filePath)

	// Execute SOPS command
	cmd, err := e.prepareSOPSCommand(ctx, args...)
	if err != nil {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Field:   filePath,
			Message: "Failed to prepare SOPS command",
			Cause:   err,
		}
	}

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

// DecryptFilesParallel decrypts multiple SOPS-encrypted files in parallel
func (e *DefaultEncryptor) DecryptFilesParallel(ctx context.Context, filePaths []string, outputDir string, maxConcurrency int) error {
	if maxConcurrency <= 0 {
		maxConcurrency = 4 // Default to 4 concurrent operations
	}

	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, maxConcurrency)
	errChan := make(chan error, len(filePaths))

	// Use a wait group to track completion
	var wg sync.WaitGroup

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Determine output path
			outputPath := ""
			if outputDir != "" {
				outputPath = filepath.Join(outputDir, filepath.Base(path))
			}

			// Decrypt the file
			if err := e.DecryptFile(ctx, path, outputPath); err != nil {
				errChan <- fmt.Errorf("failed to decrypt %s: %w", path, err)
			}
		}(filePath)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to decrypt %d files: %v", len(errors), errors)
	}

	return nil
}

// Helper functions

// needsExplicitYAMLType returns true when the file extension is not one that
// SOPS auto-detects as YAML (e.g. .yaml.enc, .yml.enc). In those cases the
// caller must pass --input-type yaml --output-type yaml to the sops binary.
func needsExplicitYAMLType(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	// SOPS natively recognises .yaml, .yml, .json, .env, .ini
	// Anything else (like .enc) needs an explicit type hint.
	switch ext {
	case ".yaml", ".yml", ".json", ".env", ".ini":
		return false
	default:
		return true
	}
}

// checkSOPSVersion checks if SOPS is available and returns version info
func checkSOPSVersion(ctx context.Context) (string, error) {
	cmd, err := security.GetDefaultCommandRunner().PrepareCommandContext(ctx, "sops", "--version")
	if err != nil {
		return "", fmt.Errorf("failed to prepare SOPS version command: %w", err)
	}
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("SOPS not found or not executable: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
