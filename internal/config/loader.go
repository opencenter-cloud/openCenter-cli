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
	"context"
	"fmt"

	configpersistence "github.com/opencenter-cloud/opencenter-cli/internal/config/persistence"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// ConfigIOHandler handles configuration file I/O operations.
// It provides methods for loading and saving configurations using
// the FileSystem interface for atomic operations.
type ConfigIOHandler struct {
	fileSystem fs.FileSystem
}

// NewConfigIOHandler creates a new ConfigIOHandler with the given FileSystem.
func NewConfigIOHandler(fileSystem fs.FileSystem) *ConfigIOHandler {
	return &ConfigIOHandler{
		fileSystem: fileSystem,
	}
}

// LoadFromFile reads and parses a configuration file from the given path.
// It uses the FileSystem interface to read the file and then unmarshals
// the YAML content into a Config struct.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - path: Absolute path to the configuration file
//
// Returns:
//   - *Config: The parsed configuration
//   - error: An error if the file cannot be read or parsed
func (cl *ConfigIOHandler) LoadFromFile(ctx context.Context, path string) (*Config, error) {
	// Read file using FileSystem interface
	data, err := cl.fileSystem.ReadFile(path)
	if err != nil {
		return nil, NewFileError("read", path, err)
	}

	// Parse the YAML data
	config, err := cl.LoadFromBytes(ctx, data)
	if err != nil {
		// Wrap with file context for better error messages
		return nil, WrapParseError(err, path, 0, 0)
	}

	return config, nil
}

// LoadFromBytes parses configuration data from a byte slice.
// It unmarshals the YAML content into a Config struct.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - data: YAML configuration data as bytes
//
// Returns:
//   - *Config: The parsed configuration
//   - error: An error if the data cannot be parsed
func (cl *ConfigIOHandler) LoadFromBytes(ctx context.Context, data []byte) (*Config, error) {
	if config, err, handled := cl.loadNativeV2(data); handled {
		if err != nil {
			return nil, NewParseError("", 0, 0, err)
		}
		return config, nil
	}

	// Unmarshal the YAML data
	config, err := cl.UnmarshalConfig(data)
	if err != nil {
		// Return parse error with context
		return nil, NewParseError("", 0, 0, err)
	}

	return config, nil
}

func (cl *ConfigIOHandler) loadNativeV2(data []byte) (*Config, error, bool) {
	loader := defaultLegacyV2Loader()
	v2Config, err := loader.LoadFromBytes(data)
	if err != nil {
		if isNativeV2ConfigData(data) {
			return nil, fmt.Errorf("failed to load native v2 configuration: %w", err), true
		}
		return nil, nil, false
	}

	legacyConfig, err := convertNativeV2ToLegacyConfig(v2Config)
	if err != nil {
		return nil, fmt.Errorf("convert native v2 configuration: %w", err), true
	}

	return &legacyConfig, nil, true
}

// SaveToFile writes a configuration to a file atomically.
// It uses FileSystem.WriteFileAtomic to ensure the write operation
// is atomic and prevents corruption from partial writes.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - path: Absolute path where the configuration should be saved
//   - config: The configuration to save
//
// Returns:
//   - error: An error if the configuration cannot be marshaled or written
func (cl *ConfigIOHandler) SaveToFile(ctx context.Context, path string, config *Config) error {
	// Marshal the configuration to YAML
	data, err := cl.MarshalConfig(config)
	if err != nil {
		return NewParseError(path, 0, 0, err)
	}

	// Write atomically using FileSystem interface
	// Use 0600 permissions to protect sensitive data
	if err := cl.fileSystem.WriteFileAtomic(path, data, 0o600); err != nil {
		return NewFileError("write", path, err)
	}

	return nil
}

// MarshalConfig converts a Config struct to YAML bytes.
// It uses gopkg.in/yaml.v3 for marshaling.
//
// Parameters:
//   - config: The configuration to marshal
//
// Returns:
//   - []byte: The YAML-encoded configuration
//   - error: An error if marshaling fails
func (cl *ConfigIOHandler) MarshalConfig(config *Config) ([]byte, error) {
	if config == nil {
		return nil, NewValidationError("", "configuration cannot be nil", nil)
	}

	data, err := configpersistence.MarshalYAML(config)
	if err != nil {
		return nil, NewParseError("", 0, 0, fmt.Errorf("failed to marshal configuration to YAML: %w", err))
	}

	return data, nil
}

// UnmarshalConfig parses YAML bytes into a Config struct.
// It uses gopkg.in/yaml.v3 for unmarshaling.
//
// Parameters:
//   - data: YAML configuration data as bytes
//
// Returns:
//   - *Config: The parsed configuration
//   - error: An error if unmarshaling fails
func (cl *ConfigIOHandler) UnmarshalConfig(data []byte) (*Config, error) {
	config, err := configpersistence.UnmarshalYAML[Config](data)
	if err != nil {
		if len(data) == 0 {
			return nil, NewValidationError("", "configuration data cannot be empty", nil)
		}
		return nil, NewParseError("", 0, 0, fmt.Errorf("failed to unmarshal YAML configuration: %w", err))
	}
	return config, nil
}
