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
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

// loadConfigV2Only loads a cluster configuration and rejects v1 configs.
// This is a wrapper around config.Load that enforces v2-only support.
//
// Parameters:
//   - clusterName: The cluster name to load
//
// Returns:
//   - config.Config: The loaded configuration
//   - error: An error if the config cannot be loaded or is v1
func loadConfigV2Only(clusterName string) (config.Config, error) {
	cfg, err := config.Load(clusterName)
	if err != nil {
		return cfg, err
	}

	// Check schema version - only v2 is supported
	if cfg.SchemaVersion != "2.0" {
		return cfg, fmt.Errorf(`v1 configurations are not supported in v2.0.0

To upgrade to v2.0.0:
1. Install opencenter v1.x
2. Run: opencenter cluster migrate-config %s
3. Upgrade to opencenter v2.0.0

See: https://docs.opencenter.io/migration/v1-to-v2`, clusterName)
	}

	return cfg, nil
}
