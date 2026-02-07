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

package flags

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

// FileFlagHandler handles --base-config, --merge-config, and --config-stack flags
type FileFlagHandler struct {
	fileSystem fs.FileSystem
}

// NewFileFlagHandler creates a new file flag handler
func NewFileFlagHandler() *FileFlagHandler {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	return &FileFlagHandler{
		fileSystem: fileSystem,
	}
}

// CanHandle returns true if this handler can process the given flag
func (h *FileFlagHandler) CanHandle(flagName string) bool {
	return strings.HasPrefix(flagName, "base-config") ||
		strings.HasPrefix(flagName, "merge-config") ||
		strings.HasPrefix(flagName, "config-stack")
}

// GetFlagType returns the type of flags this handler processes
func (h *FileFlagHandler) GetFlagType() FlagType {
	return FlagTypeFile
}

// ParseFlag processes a single flag and returns the parsed result
func (h *FileFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	if strings.HasPrefix(flagName, "config-stack") {
		return h.parseConfigStack(value)
	}

	return h.parseConfigFile(flagName, value)
}

// parseConfigFile handles single configuration file flags
func (h *FileFlagHandler) parseConfigFile(flagName, value string) (*ConfigFileFlag, error) {
	if value == "" {
		return nil, fmt.Errorf("empty file path in --%s flag", flagName)
	}

	// Resolve relative paths
	absPath, err := filepath.Abs(value)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path '%s': %w", value, err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file does not exist: %s", absPath)
	}

	// Determine file type
	fileType := h.detectFileType(absPath)

	// Determine merge priority based on flag type
	var priority int
	var mergeType ConfigFileMergeType

	switch {
	case strings.HasPrefix(flagName, "base-config"):
		priority = 1
		mergeType = ConfigFileMergeBase
	case strings.HasPrefix(flagName, "merge-config"):
		priority = 2
		mergeType = ConfigFileMergeOverride
	default:
		priority = 2
		mergeType = ConfigFileMergeOverride
	}

	return &ConfigFileFlag{
		Path:      absPath,
		Type:      fileType,
		Priority:  priority,
		MergeType: mergeType,
	}, nil
}

// parseConfigStack handles --config-stack flags with comma-separated file lists
func (h *FileFlagHandler) parseConfigStack(value string) ([]*ConfigFileFlag, error) {
	if value == "" {
		return nil, fmt.Errorf("empty file list in --config-stack flag")
	}

	filePaths := strings.Split(value, ",")
	configFiles := make([]*ConfigFileFlag, 0, len(filePaths))

	for i, filePath := range filePaths {
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			continue
		}

		// Resolve relative paths
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path '%s' in config stack: %w", filePath, err)
		}

		// Check if file exists
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file does not exist in stack: %s", absPath)
		}

		// Determine file type
		fileType := h.detectFileType(absPath)

		configFiles = append(configFiles, &ConfigFileFlag{
			Path:      absPath,
			Type:      fileType,
			Priority:  i + 1, // Stack order determines priority
			MergeType: ConfigFileMergeStack,
		})
	}

	if len(configFiles) == 0 {
		return nil, fmt.Errorf("no valid files found in config stack")
	}

	return configFiles, nil
}

// detectFileType determines the file type based on extension
func (h *FileFlagHandler) detectFileType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	default:
		return "yaml" // Default to YAML
	}
}

// LoadConfigurationFile loads and parses a configuration file
func (h *FileFlagHandler) LoadConfigurationFile(configFile *ConfigFileFlag) (*Configuration, error) {
	data, err := h.fileSystem.ReadFile(configFile.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file '%s': %w", configFile.Path, err)
	}

	var configData map[string]interface{}

	switch configFile.Type {
	case "yaml":
		if err := yaml.Unmarshal(data, &configData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML configuration file '%s': %w", configFile.Path, err)
		}
	case "json":
		if err := yaml.Unmarshal(data, &configData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON configuration file '%s': %w", configFile.Path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported file type '%s' for file '%s'", configFile.Type, configFile.Path)
	}

	if configData == nil {
		configData = make(map[string]interface{})
	}

	return &Configuration{
		Data: configData,
		Sources: []ConfigSource{
			{
				Type:     SourceFile,
				Path:     configFile.Path,
				Priority: configFile.Priority,
			},
		},
		Metadata: ConfigMetadata{
			Sources: []ConfigSource{
				{
					Type:     SourceFile,
					Path:     configFile.Path,
					Priority: configFile.Priority,
				},
			},
		},
	}, nil
}

// GetPath returns the configuration path this flag affects (root path for file flags)
func (f *ConfigFileFlag) GetPath() string {
	return ""
}

// MergeIntoConfiguration applies this flag to the configuration
func (f *ConfigFileFlag) MergeIntoConfiguration(flag ParsedFlag, config map[string]interface{}) error {
	// File flags are handled differently - they need to be loaded and merged at a higher level
	return fmt.Errorf("file flags should be processed by the configuration merger, not directly merged")
}
