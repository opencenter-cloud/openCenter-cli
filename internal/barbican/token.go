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

package barbican

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
	"github.com/zalando/go-keyring"
)

const (
	serviceName = "opencenter-cli"
)

var (
	// Global FileSystem instance for token operations
	tokenFileSystem fs.FileSystem
)

func init() {
	// Initialize FileSystem for token operations
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	tokenFileSystem = fs.NewDefaultFileSystem(errorHandler)
}

func getUser() string {
	user := os.Getenv("USER")
	if user == "" {
		return "default-user"
	}
	return user
}

// Fallback path for headless environments
func getTokenCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencenter", "barbican", "token"), nil
}

func StoreToken(token string) error {
	err := keyring.Set(serviceName, getUser(), token)
	if err == nil {
		return nil
	}

	// Fallback to file storage if keyring fails (e.g., headless environment)
	path, pathErr := getTokenCachePath()
	if pathErr != nil {
		return fmt.Errorf("keyring failed (%v) and unable to get cache path: %w", err, pathErr)
	}

	if mkdirErr := tokenFileSystem.MkdirAll(filepath.Dir(path), 0700); mkdirErr != nil {
		return fmt.Errorf("keyring failed (%v) and unable to create cache dir: %w", err, mkdirErr)
	}

	if writeErr := tokenFileSystem.WriteFileAtomic(path, []byte(token), 0600); writeErr != nil {
		return fmt.Errorf("keyring failed (%v) and unable to write token file: %w", err, writeErr)
	}

	return nil
}

func LoadToken() (string, error) {
	token, err := keyring.Get(serviceName, getUser())
	if err == nil {
		return token, nil
	}

	// Fallback to file storage
	path, pathErr := getTokenCachePath()
	if pathErr != nil {
		return "", fmt.Errorf("keyring failed (%v) and unable to get cache path: %w", err, pathErr)
	}

	fileToken, readErr := tokenFileSystem.ReadFile(path)
	if readErr != nil {
		// If both fail, return the keyring error as primary, but hint at file error
		return "", fmt.Errorf("failed to retrieve token from keyring (%v) and file (%v)", err, readErr)
	}

	return string(fileToken), nil
}
