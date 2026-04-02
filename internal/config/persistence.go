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
	"sync"

	configpersistence "github.com/opencenter-cloud/opencenter-cli/internal/config/persistence"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
)

// globalManager is a singleton ConfigurationManager for backward compatibility
var (
	globalManager     *ConfigurationManager
	globalManagerOnce sync.Once
	globalManagerErr  error
)

// getGlobalManager returns the singleton ConfigurationManager instance
func getGlobalManager() (*ConfigurationManager, error) {
	globalManagerOnce.Do(func() {
		globalManager, globalManagerErr = NewConfigurationManager()
	})
	return globalManager, globalManagerErr
}

// ResolveConfigDir resolves the configuration directory based on the OPENCENTER_CONFIG_DIR
// environment variable. If the variable is not set, it falls back to the user's
// standard config directory (e.g., ~/.config/opencenter on Linux).
// The directory is created if it does not exist.
func ResolveConfigDir() (string, error) {
	return configpersistence.ResolveConfigDir()
}

// ParseClusterIdentifier parses a cluster identifier which can be in one of two formats:
// 1. "cluster" - just the cluster name (uses default "opencenter" organization)
// 2. "organization/cluster" - organization and cluster name
//
// Inputs:
//   - identifier: The cluster identifier to parse.
//
// Outputs:
//   - organization: The organization name (or "opencenter" if not specified).
//   - clusterName: The cluster name.
//   - error: An error if the identifier is invalid.
func ParseClusterIdentifier(identifier string) (organization string, clusterName string, err error) {
	validateClusterName := func(name string) error {
		ctx := context.Background()
		validator := validators.NewClusterNameValidator()

		result, err := validator.Validate(ctx, name)
		if err != nil {
			return fmt.Errorf("cluster name validation failed: %w", err)
		}
		if !result.Valid {
			return fmt.Errorf("invalid cluster name: %s", result.Errors[0].Message)
		}
		return nil
	}

	return configpersistence.ParseClusterIdentifier(identifier, validateClusterName)
}

// List returns a sorted list of cluster names from the configuration directory.
