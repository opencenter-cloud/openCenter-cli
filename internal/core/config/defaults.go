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
	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
)

// NewDefault returns a Config initialized with the default values for the given cluster name.
//
// Inputs:
//   - name: The name of the cluster.
//
// Outputs:
//   - Config: A new Config object with default values.
func NewDefault(name string) Config {
	return internalconfig.NewDefault(name)
}

// ApplyDefaults applies default values to a configuration.
// This includes CLI defaults, organization defaults, and provider defaults.
//
// Inputs:
//   - cfg: The configuration to apply defaults to
func ApplyDefaults(cfg *Config) {
	internalconfig.ApplyDefaults(cfg)
}

// DefaultTalosConfig returns a TalosConfig initialized with secure default values.
// This function should be called when enabling Talos for a cluster.
//
// Inputs:
//   - clusterName: The name of the cluster.
//
// Outputs:
//   - *TalosConfig: A new TalosConfig object with default values.
func DefaultTalosConfig(clusterName string) *internalconfig.TalosConfig {
	return internalconfig.DefaultTalosConfig(clusterName)
}
