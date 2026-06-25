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

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// sopsEncryptorAdapter adapts sops.DefaultSOPSManager to secrets.SOPSEncryptor interface
type sopsEncryptorAdapter struct {
	manager *sops.DefaultSOPSManager
}

func (s *sopsEncryptorAdapter) EncryptFile(ctx context.Context, filePath string) error {
	encryptor := s.manager.GetEncryptor()
	if encryptor == nil {
		return fmt.Errorf("encryptor not available")
	}

	// Use empty config to use .sops.yaml configuration
	config := sops.EncryptionConfig{
		InPlace: true,
	}

	return encryptor.EncryptFile(ctx, filePath, config)
}

func (s *sopsEncryptorAdapter) DecryptFile(ctx context.Context, filePath string) ([]byte, error) {
	encryptor := s.manager.GetEncryptor()
	if encryptor == nil {
		return nil, fmt.Errorf("encryptor not available")
	}

	// Create a temporary file for decrypted output
	tmpFile, err := os.CreateTemp("", "sops-decrypt-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Decrypt to temp file
	if err := encryptor.DecryptFile(ctx, filePath, tmpFile.Name()); err != nil {
		return nil, fmt.Errorf("failed to decrypt file: %w", err)
	}

	// Read decrypted content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted content: %w", err)
	}

	return content, nil
}

// createSecretsLogger creates a logger for secrets operations
func createSecretsLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// createSOPSManager creates a SOPS manager instance
func createSOPSManager(logger *slog.Logger) *sops.DefaultSOPSManager {
	encryptor := sops.NewDefaultEncryptor([]string{}, []string{})
	return sops.NewDefaultSOPSManager(nil, encryptor, logger)
}

func createAuditLogger() (*security.AuditLogger, error) {
	return security.NewDefaultAuditLogger()
}

// createConfigLoader creates a config loader instance
func createConfigLoader() *v2.ConfigIOHandler {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	return v2.NewConfigIOHandler(fileSystem)
}

// getSecretsRegistryPath returns the path to the secrets registry directory
func getSecretsRegistryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "opencenter", "secrets"), nil
}
